// Package resolver implements environment variable resolution with precedence tracking
package resolver

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Layer represents the source layer of an environment variable
type Layer int

const (
	LayerEnvExample Layer = iota
	LayerEnv
	LayerEnvLocal
	LayerEnvOther
	LayerComposeEnvFile
	LayerComposeInline
	LayerOSEnv // New: system environment variables
)

func (l Layer) String() string {
	switch l {
	case LayerEnvExample:
		return ".env.example"
	case LayerEnv:
		return ".env"
	case LayerEnvLocal:
		return ".env.local"
	case LayerEnvOther:
		return ".env.*"
	case LayerComposeEnvFile:
		return "compose env_file"
	case LayerComposeInline:
		return "compose inline"
	case LayerOSEnv:
		return "OS environment"
	default:
		return "unknown"
	}
}

// Precedence returns the precedence order (higher wins)
func (l Layer) Precedence() int {
	switch l {
	case LayerEnvExample:
		return 0
	case LayerEnv:
		return 1
	case LayerEnvLocal:
		return 2
	case LayerEnvOther:
		return 3
	case LayerComposeEnvFile:
		return 4
	case LayerComposeInline:
		return 5
	case LayerOSEnv:
		return 6 // OS env has highest precedence
	default:
		return -1
	}
}

// Source represents where a variable value came from
type Source struct {
	Layer    Layer
	File     string
	Line     int
	Service  string // For compose sources
	Value    string
	IsInline bool
}

// Variable represents a resolved environment variable
type Variable struct {
	Name       string
	FinalValue string
	FinalFrom  Source
	Chain      []Source // All sources in precedence order
	Overridden bool
	Conflicts  []string // Different values from different sources
}

// Resolution is the complete resolution result
type Resolution struct {
	Path         string
	Variables    []*Variable
	ByName       map[string]*Variable
	EnvFiles     []string
	ComposeFiles []string
	Warnings     []string
	Undefined    []string // Variables referenced but not defined anywhere
}

// Options for resolution
type Options struct {
	IncludeOSEnv bool     // Include system environment variables
	ServiceName  string   // Filter to specific service
	StrictMode   bool     // Return error if undefined vars found
	CompareWith  string   // Path to compare environments
}

// Resolve scans and resolves all environment variables
func Resolve(basePath string) (*Resolution, error) {
	return ResolveWithOptions(basePath, Options{})
}

// ResolveWithOptions scans and resolves with configurable options
func ResolveWithOptions(basePath string, opts Options) (*Resolution, error) {
	r := &Resolution{
		Path:     basePath,
		ByName:   make(map[string]*Variable),
		Warnings: []string{},
	}

	// 1. Find and parse .env files (in precedence order)
	envPatterns := []struct {
		pattern string
		layer   Layer
	}{
		{".env.example", LayerEnvExample},
		{".env", LayerEnv},
		{".env.local", LayerEnvLocal},
	}

	for _, ep := range envPatterns {
		envPath := filepath.Join(basePath, ep.pattern)
		if _, err := os.Stat(envPath); err == nil {
			r.EnvFiles = append(r.EnvFiles, envPath)
			if err := r.parseEnvFile(envPath, ep.layer); err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Error parsing %s: %v", ep.pattern, err))
			}
		}
	}

	// Also scan for other .env.* files
	entries, _ := os.ReadDir(basePath)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".env.") &&
			name != ".env.example" &&
			name != ".env.local" {
			envPath := filepath.Join(basePath, name)
			r.EnvFiles = append(r.EnvFiles, envPath)
			if err := r.parseEnvFile(envPath, LayerEnvOther); err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Error parsing %s: %v", name, err))
			}
		}
	}

	// 2. Find and parse compose files
	composePatterns := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, pattern := range composePatterns {
		composePath := filepath.Join(basePath, pattern)
		if _, err := os.Stat(composePath); err == nil {
			r.ComposeFiles = append(r.ComposeFiles, composePath)
			if err := r.parseComposeFile(composePath); err != nil {
				r.Warnings = append(r.Warnings, fmt.Sprintf("Error parsing %s: %v", pattern, err))
			}
		}
	}

	// 3. Build the variables list sorted by name
	for _, v := range r.ByName {
		// Sort chain by precedence
		sort.Slice(v.Chain, func(i, j int) bool {
			return v.Chain[i].Layer.Precedence() < v.Chain[j].Layer.Precedence()
		})

		// Determine final value (highest precedence wins)
		if len(v.Chain) > 0 {
			v.FinalFrom = v.Chain[len(v.Chain)-1]
			v.FinalValue = v.FinalFrom.Value
		}

		// Check for conflicts (different values)
		values := make(map[string]bool)
		for _, src := range v.Chain {
			if src.Value != "" {
				values[src.Value] = true
			}
		}
		if len(values) > 1 {
			v.Overridden = true
			for val := range values {
				if val != v.FinalValue {
					v.Conflicts = append(v.Conflicts, val)
				}
			}
		}

		r.Variables = append(r.Variables, v)
	}

	// Sort by name
	sort.Slice(r.Variables, func(i, j int) bool {
		return r.Variables[i].Name < r.Variables[j].Name
	})

	// Add OS environment variables if requested
	if opts.IncludeOSEnv {
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				// Only add if this var is referenced in the project
				if v, exists := r.ByName[key]; exists {
					r.addSource(key, Source{
						Layer: LayerOSEnv,
						File:  "environment",
						Value: value,
					})
					// Update final value since OS env has highest precedence
					v.FinalValue = value
					v.FinalFrom = Source{Layer: LayerOSEnv, File: "environment", Value: value}
					v.Overridden = true
				}
			}
		}
	}

	// Filter to service if specified
	if opts.ServiceName != "" {
		r.filterToService(opts.ServiceName)
	}

	// Check for undefined vars if strict mode
	if opts.StrictMode {
		r.findUndefinedVars()
		if len(r.Undefined) > 0 {
			return r, fmt.Errorf("strict mode: %d undefined variable(s): %s", 
				len(r.Undefined), strings.Join(r.Undefined, ", "))
		}
	}

	return r, nil
}

// filterToService filters variables to only those used by a specific service
func (r *Resolution) filterToService(serviceName string) {
	var filtered []*Variable

	for _, v := range r.Variables {
		// Check if this variable is used by the specified service
		usedByService := false
		for _, src := range v.Chain {
			if src.Service == serviceName || src.Service == "" {
				usedByService = true
				break
			}
		}

		if usedByService {
			filtered = append(filtered, v)
		}
	}

	r.Variables = filtered
}

// findUndefinedVars looks for variables referenced but not defined
func (r *Resolution) findUndefinedVars() {
	// Check for ${VAR} references in compose that have empty final values
	for _, v := range r.Variables {
		if v.FinalValue == "" {
			// Check if this is only a reference (no definition found)
			hasDefinition := false
			for _, src := range v.Chain {
				if src.Value != "" {
					hasDefinition = true
					break
				}
			}
			if !hasDefinition {
				r.Undefined = append(r.Undefined, v.Name)
			}
		}
	}
}

func (r *Resolution) parseEnvFile(path string, layer Layer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle export prefix
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = unquote(value)

		if key == "" {
			continue
		}

		r.addSource(key, Source{
			Layer: layer,
			File:  path,
			Line:  lineNum,
			Value: value,
		})
	}

	return scanner.Err()
}

type composeFile struct {
	Services map[string]struct {
		Environment interface{} `yaml:"environment"`
		EnvFile     interface{} `yaml:"env_file"`
	} `yaml:"services"`
}

func (r *Resolution) parseComposeFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return err
	}

	for serviceName, svc := range compose.Services {
		// Parse env_file references
		if svc.EnvFile != nil {
			r.parseEnvFileRef(path, serviceName, svc.EnvFile)
		}

		// Parse inline environment
		if svc.Environment != nil {
			r.parseInlineEnv(path, serviceName, svc.Environment)
		}
	}

	return nil
}

func (r *Resolution) parseEnvFileRef(composePath, serviceName string, envFile interface{}) {
	baseDir := filepath.Dir(composePath)

	var files []string
	switch v := envFile.(type) {
	case string:
		files = []string{v}
	case []interface{}:
		for _, f := range v {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}
	}

	for _, f := range files {
		envPath := filepath.Join(baseDir, f)
		if _, err := os.Stat(envPath); err == nil {
			// Parse this env file as compose env_file layer
			file, err := os.Open(envPath)
			if err != nil {
				continue
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}

				key := strings.TrimSpace(parts[0])
				value := unquote(strings.TrimSpace(parts[1]))

				r.addSource(key, Source{
					Layer:   LayerComposeEnvFile,
					File:    envPath,
					Line:    lineNum,
					Service: serviceName,
					Value:   value,
				})
			}
		}
	}
}

func (r *Resolution) parseInlineEnv(composePath, serviceName string, env interface{}) {
	switch v := env.(type) {
	case map[string]interface{}:
		for key, val := range v {
			value := ""
			if val != nil {
				value = fmt.Sprintf("%v", val)
			}
			r.addSource(key, Source{
				Layer:    LayerComposeInline,
				File:     composePath,
				Service:  serviceName,
				Value:    value,
				IsInline: true,
			})
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				// Can be "KEY=VALUE" or just "KEY" (reference)
				parts := strings.SplitN(s, "=", 2)
				key := parts[0]
				value := ""
				if len(parts) == 2 {
					value = parts[1]
				}
				r.addSource(key, Source{
					Layer:    LayerComposeInline,
					File:     composePath,
					Service:  serviceName,
					Value:    value,
					IsInline: true,
				})
			}
		}
	}
}

func (r *Resolution) addSource(name string, src Source) {
	v, ok := r.ByName[name]
	if !ok {
		v = &Variable{Name: name}
		r.ByName[name] = v
	}
	v.Chain = append(v.Chain, src)
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// CompareResult holds the result of comparing two environment contexts
type CompareResult struct {
	OnlyInFirst  []string
	OnlyInSecond []string
	Different    []DiffVar
	Same         []string
}

// DiffVar represents a variable with different values
type DiffVar struct {
	Name       string
	FirstValue string
	SecondValue string
}

// Compare compares two resolutions and returns the differences
func Compare(first, second *Resolution) *CompareResult {
	result := &CompareResult{}

	firstVars := make(map[string]string)
	secondVars := make(map[string]string)

	for _, v := range first.Variables {
		firstVars[v.Name] = v.FinalValue
	}

	for _, v := range second.Variables {
		secondVars[v.Name] = v.FinalValue
	}

	// Find vars only in first
	for name := range firstVars {
		if _, exists := secondVars[name]; !exists {
			result.OnlyInFirst = append(result.OnlyInFirst, name)
		}
	}

	// Find vars only in second
	for name := range secondVars {
		if _, exists := firstVars[name]; !exists {
			result.OnlyInSecond = append(result.OnlyInSecond, name)
		}
	}

	// Find different and same
	for name, firstVal := range firstVars {
		if secondVal, exists := secondVars[name]; exists {
			if firstVal != secondVal {
				result.Different = append(result.Different, DiffVar{
					Name:        name,
					FirstValue:  firstVal,
					SecondValue: secondVal,
				})
			} else {
				result.Same = append(result.Same, name)
			}
		}
	}

	// Sort results
	sort.Strings(result.OnlyInFirst)
	sort.Strings(result.OnlyInSecond)
	sort.Strings(result.Same)
	sort.Slice(result.Different, func(i, j int) bool {
		return result.Different[i].Name < result.Different[j].Name
	})

	return result
}

// FormatCompare formats a compare result as human-readable text
func FormatCompare(first, second string, result *CompareResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Environment Comparison: %s vs %s\n\n", first, second))

	if len(result.OnlyInFirst) > 0 {
		sb.WriteString(fmt.Sprintf("## Only in %s (%d)\n", first, len(result.OnlyInFirst)))
		for _, name := range result.OnlyInFirst {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString("\n")
	}

	if len(result.OnlyInSecond) > 0 {
		sb.WriteString(fmt.Sprintf("## Only in %s (%d)\n", second, len(result.OnlyInSecond)))
		for _, name := range result.OnlyInSecond {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString("\n")
	}

	if len(result.Different) > 0 {
		sb.WriteString(fmt.Sprintf("## Different Values (%d)\n", len(result.Different)))
		for _, diff := range result.Different {
			sb.WriteString(fmt.Sprintf("  - %s:\n", diff.Name))
			sb.WriteString(fmt.Sprintf("      %s: %s\n", first, diff.FirstValue))
			sb.WriteString(fmt.Sprintf("      %s: %s\n", second, diff.SecondValue))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("## Summary\n"))
	sb.WriteString(fmt.Sprintf("  - %d variable(s) only in %s\n", len(result.OnlyInFirst), first))
	sb.WriteString(fmt.Sprintf("  - %d variable(s) only in %s\n", len(result.OnlyInSecond), second))
	sb.WriteString(fmt.Sprintf("  - %d variable(s) with different values\n", len(result.Different)))
	sb.WriteString(fmt.Sprintf("  - %d variable(s) with same values\n", len(result.Same)))

	return sb.String()
}
