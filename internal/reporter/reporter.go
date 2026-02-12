// Package reporter provides output formatting for resolution results
package reporter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/stackgen-cli/envmerge/internal/resolver"
)

// FormatText generates colored text output
func FormatText(r *resolver.Resolution) (string, error) {
	var sb strings.Builder

	sb.WriteString(color.CyanString("Environment Resolution Report\n"))
	sb.WriteString(color.CyanString("=============================\n\n"))

	// Summary
	sb.WriteString(fmt.Sprintf("Scanned path: %s\n", r.Path))
	sb.WriteString(fmt.Sprintf("Env files: %d\n", len(r.EnvFiles)))
	sb.WriteString(fmt.Sprintf("Compose files: %d\n", len(r.ComposeFiles)))
	sb.WriteString(fmt.Sprintf("Variables resolved: %d\n\n", len(r.Variables)))

	// Warnings
	if len(r.Warnings) > 0 {
		sb.WriteString(color.YellowString("⚠️  Warnings\n"))
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  • %s\n", w))
		}
		sb.WriteString("\n")
	}

	// Variables with overrides first
	overridden := []*resolver.Variable{}
	clean := []*resolver.Variable{}
	for _, v := range r.Variables {
		if v.Overridden {
			overridden = append(overridden, v)
		} else {
			clean = append(clean, v)
		}
	}

	if len(overridden) > 0 {
		sb.WriteString(color.YellowString("Variables with Overrides\n"))
		sb.WriteString("------------------------\n")
		for _, v := range overridden {
			formatVariable(&sb, v, true)
		}
		sb.WriteString("\n")
	}

	if len(clean) > 0 {
		sb.WriteString(color.GreenString("Cleanly Resolved Variables\n"))
		sb.WriteString("--------------------------\n")
		for _, v := range clean {
			formatVariable(&sb, v, false)
		}
	}

	return sb.String(), nil
}

func formatVariable(sb *strings.Builder, v *resolver.Variable, showChain bool) {
	// Variable name
	sb.WriteString(color.WhiteString(v.Name))
	sb.WriteString("\n")

	// Final value
	finalVal := v.FinalValue
	if finalVal == "" {
		finalVal = color.HiBlackString("(empty)")
	}
	sb.WriteString(fmt.Sprintf("  final: %s\n", finalVal))

	// Source
	src := v.FinalFrom
	loc := src.File
	if src.Line > 0 {
		loc = fmt.Sprintf("%s:%d", src.File, src.Line)
	}
	if src.Service != "" {
		sb.WriteString(fmt.Sprintf("  from: %s (service: %s)\n", src.Layer, src.Service))
	} else {
		sb.WriteString(fmt.Sprintf("  from: %s\n", loc))
	}

	// Show override chain for overridden vars
	if showChain && len(v.Chain) > 1 {
		sb.WriteString(color.HiBlackString("  chain:\n"))
		for i := len(v.Chain) - 1; i >= 0; i-- {
			s := v.Chain[i]
			marker := "  "
			if i == len(v.Chain)-1 {
				marker = "→ "
			}

			loc := s.Layer.String()
			if s.File != "" {
				loc = s.File
				if s.Line > 0 {
					loc = fmt.Sprintf("%s:%d", s.File, s.Line)
				}
			}

			val := s.Value
			if val == "" {
				val = "(empty)"
			}
			sb.WriteString(fmt.Sprintf("    %s%s = %s\n", marker, loc, val))
		}
	}

	sb.WriteString("\n")
}

// FormatJSON generates JSON output
func FormatJSON(r *resolver.Resolution) (string, error) {
	type jsonSource struct {
		Layer   string `json:"layer"`
		File    string `json:"file,omitempty"`
		Line    int    `json:"line,omitempty"`
		Service string `json:"service,omitempty"`
		Value   string `json:"value"`
	}

	type jsonVariable struct {
		Name       string       `json:"name"`
		FinalValue string       `json:"final_value"`
		FinalFrom  jsonSource   `json:"final_from"`
		Overridden bool         `json:"overridden"`
		Chain      []jsonSource `json:"chain,omitempty"`
	}

	type jsonOutput struct {
		Path         string         `json:"path"`
		EnvFiles     []string       `json:"env_files"`
		ComposeFiles []string       `json:"compose_files"`
		Variables    []jsonVariable `json:"variables"`
		Warnings     []string       `json:"warnings,omitempty"`
	}

	out := jsonOutput{
		Path:         r.Path,
		EnvFiles:     r.EnvFiles,
		ComposeFiles: r.ComposeFiles,
		Warnings:     r.Warnings,
	}

	for _, v := range r.Variables {
		jv := jsonVariable{
			Name:       v.Name,
			FinalValue: v.FinalValue,
			FinalFrom: jsonSource{
				Layer:   v.FinalFrom.Layer.String(),
				File:    v.FinalFrom.File,
				Line:    v.FinalFrom.Line,
				Service: v.FinalFrom.Service,
				Value:   v.FinalFrom.Value,
			},
			Overridden: v.Overridden,
		}

		if v.Overridden {
			for _, s := range v.Chain {
				jv.Chain = append(jv.Chain, jsonSource{
					Layer:   s.Layer.String(),
					File:    s.File,
					Line:    s.Line,
					Service: s.Service,
					Value:   s.Value,
				})
			}
		}

		out.Variables = append(out.Variables, jv)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatMarkdown generates markdown output
func FormatMarkdown(r *resolver.Resolution) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Environment Resolution Report\n\n")
	sb.WriteString(fmt.Sprintf("**Path:** `%s`\n\n", r.Path))

	// Summary table
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Env files scanned | %d |\n", len(r.EnvFiles)))
	sb.WriteString(fmt.Sprintf("| Compose files scanned | %d |\n", len(r.ComposeFiles)))
	sb.WriteString(fmt.Sprintf("| Variables resolved | %d |\n", len(r.Variables)))

	// Count overrides
	overrides := 0
	for _, v := range r.Variables {
		if v.Overridden {
			overrides++
		}
	}
	sb.WriteString(fmt.Sprintf("| Variables with overrides | %d |\n", overrides))
	sb.WriteString("\n")

	// Warnings
	if len(r.Warnings) > 0 {
		sb.WriteString("## ⚠️ Warnings\n\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", w))
		}
		sb.WriteString("\n")
	}

	// Variables table
	sb.WriteString("## Resolved Variables\n\n")
	sb.WriteString("| Variable | Final Value | Source | Overridden |\n")
	sb.WriteString("|----------|-------------|--------|------------|\n")

	// Sort: overridden first
	vars := make([]*resolver.Variable, len(r.Variables))
	copy(vars, r.Variables)
	sort.Slice(vars, func(i, j int) bool {
		if vars[i].Overridden != vars[j].Overridden {
			return vars[i].Overridden
		}
		return vars[i].Name < vars[j].Name
	})

	for _, v := range vars {
		val := v.FinalValue
		if len(val) > 30 {
			val = val[:27] + "..."
		}
		val = "`" + val + "`"

		src := v.FinalFrom.Layer.String()
		if v.FinalFrom.Service != "" {
			src = fmt.Sprintf("%s (%s)", src, v.FinalFrom.Service)
		}

		override := ""
		if v.Overridden {
			override = "⚠️ Yes"
		}

		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", v.Name, val, src, override))
	}

	return sb.String(), nil
}
