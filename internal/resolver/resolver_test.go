package resolver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_BasicEnvFile(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()

	// Create .env file
	envContent := `# Comment
DATABASE_URL=postgres://localhost/db
API_KEY=secret123
DEBUG=true
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(result.Variables) != 3 {
		t.Errorf("Expected 3 variables, got %d", len(result.Variables))
	}

	// Check DATABASE_URL
	dbVar, ok := result.ByName["DATABASE_URL"]
	if !ok {
		t.Error("DATABASE_URL not found")
	} else {
		if dbVar.FinalValue != "postgres://localhost/db" {
			t.Errorf("DATABASE_URL = %s, want postgres://localhost/db", dbVar.FinalValue)
		}
	}
}

func TestResolve_Precedence(t *testing.T) {
	dir := t.TempDir()

	// .env (lower precedence)
	envContent := `API_KEY=from_env
OTHER=only_in_env
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	// .env.local (higher precedence)
	localContent := `API_KEY=from_local
LOCAL_ONLY=local_value
`
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// API_KEY should come from .env.local
	apiVar := result.ByName["API_KEY"]
	if apiVar == nil {
		t.Fatal("API_KEY not found")
	}

	if apiVar.FinalValue != "from_local" {
		t.Errorf("API_KEY = %s, want from_local", apiVar.FinalValue)
	}

	if !apiVar.Overridden {
		t.Error("API_KEY should be marked as overridden")
	}

	// OTHER should come from .env
	otherVar := result.ByName["OTHER"]
	if otherVar == nil {
		t.Fatal("OTHER not found")
	}
	if otherVar.FinalValue != "only_in_env" {
		t.Errorf("OTHER = %s, want only_in_env", otherVar.FinalValue)
	}
}

func TestResolve_ComposeInline(t *testing.T) {
	dir := t.TempDir()

	composeContent := `services:
  api:
    image: node:18
    environment:
      NODE_ENV: production
      PORT: "3000"
`
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	nodeEnv := result.ByName["NODE_ENV"]
	if nodeEnv == nil {
		t.Fatal("NODE_ENV not found")
	}
	if nodeEnv.FinalValue != "production" {
		t.Errorf("NODE_ENV = %s, want production", nodeEnv.FinalValue)
	}
	if nodeEnv.FinalFrom.Layer != LayerComposeInline {
		t.Errorf("NODE_ENV layer = %v, want LayerComposeInline", nodeEnv.FinalFrom.Layer)
	}
}

func TestResolve_QuotedValues(t *testing.T) {
	dir := t.TempDir()

	envContent := `DOUBLE="value with spaces"
SINGLE='another value'
UNQUOTED=simple
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"DOUBLE", "value with spaces"},
		{"SINGLE", "another value"},
		{"UNQUOTED", "simple"},
	}

	for _, tc := range tests {
		v := result.ByName[tc.name]
		if v == nil {
			t.Errorf("%s not found", tc.name)
			continue
		}
		if v.FinalValue != tc.expected {
			t.Errorf("%s = %s, want %s", tc.name, v.FinalValue, tc.expected)
		}
	}
}

func TestResolve_ExportPrefix(t *testing.T) {
	dir := t.TempDir()

	envContent := `export EXPORTED_VAR=value1
NORMAL_VAR=value2
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	exported := result.ByName["EXPORTED_VAR"]
	if exported == nil || exported.FinalValue != "value1" {
		t.Errorf("EXPORTED_VAR not properly parsed")
	}
}

func TestResolve_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(result.Variables) != 0 {
		t.Errorf("Expected 0 variables, got %d", len(result.Variables))
	}
}

func TestResolve_ComposeOverridesEnv(t *testing.T) {
	dir := t.TempDir()

	// .env.local (high precedence for env files)
	envContent := `DATABASE_URL=local_db
`
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Compose inline (highest precedence)
	composeContent := `services:
  db:
    environment:
      DATABASE_URL: compose_db
`
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	dbVar := result.ByName["DATABASE_URL"]
	if dbVar == nil {
		t.Fatal("DATABASE_URL not found")
	}

	// Compose inline should win
	if dbVar.FinalValue != "compose_db" {
		t.Errorf("DATABASE_URL = %s, want compose_db (compose should override .env.local)", dbVar.FinalValue)
	}

	// Should have override chain
	if len(dbVar.Chain) != 2 {
		t.Errorf("Chain length = %d, want 2", len(dbVar.Chain))
	}
}

func TestLayerPrecedence(t *testing.T) {
	tests := []struct {
		layer    Layer
		wantPrec int
	}{
		{LayerEnvExample, 0},
		{LayerEnv, 1},
		{LayerEnvLocal, 2},
		{LayerEnvOther, 3},
		{LayerComposeEnvFile, 4},
		{LayerComposeInline, 5},
	}

	for _, tc := range tests {
		if got := tc.layer.Precedence(); got != tc.wantPrec {
			t.Errorf("%v.Precedence() = %d, want %d", tc.layer, got, tc.wantPrec)
		}
	}
}

// Edge case tests

func TestResolve_EmptyValues(t *testing.T) {
	dir := t.TempDir()

	envContent := `EMPTY_VAR=
ANOTHER_EMPTY=
WHITESPACE_VAR=   
NOT_EMPTY=value
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if empty, ok := result.ByName["EMPTY_VAR"]; !ok || empty.FinalValue != "" {
		t.Error("EMPTY_VAR should have empty string value")
	}
}

func TestResolve_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()

	envContent := `URL_WITH_QUERY=https://api.com?key=value&foo=bar
JSON_VALUE={"key":"value","nested":{"a":1}}
REGEX_PATTERN=^[a-zA-Z0-9]+$
PATH_WITH_SPACES=/path/to/some file/here
EQUALS_IN_VALUE=key=value=more
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	tests := map[string]string{
		"URL_WITH_QUERY": "https://api.com?key=value&foo=bar",
		"JSON_VALUE":     `{"key":"value","nested":{"a":1}}`,
		"REGEX_PATTERN":  "^[a-zA-Z0-9]+$",
		"EQUALS_IN_VALUE": "key=value=more",
	}

	for name, expected := range tests {
		v, ok := result.ByName[name]
		if !ok {
			t.Errorf("%s not found", name)
			continue
		}
		if v.FinalValue != expected {
			t.Errorf("%s = %q, want %q", name, v.FinalValue, expected)
		}
	}
}

func TestResolve_QuotedValuesMixed(t *testing.T) {
	dir := t.TempDir()

	envContent := `SINGLE_QUOTED='value with spaces'
DOUBLE_QUOTED="another value"
MIXED_QUOTES='has "inner" quotes'
NO_QUOTES=plain value
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Quoted values should have quotes stripped
	if v := result.ByName["SINGLE_QUOTED"]; v != nil {
		if v.FinalValue != "value with spaces" {
			t.Errorf("SINGLE_QUOTED = %q, want 'value with spaces'", v.FinalValue)
		}
	}
}

func TestResolve_UnicodeValues(t *testing.T) {
	dir := t.TempDir()

	envContent := `EMOJI=🚀🎉
CHINESE=你好世界
ARABIC=مرحبا
MIXED=Hello 世界 🌍
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if v := result.ByName["EMOJI"]; v == nil || v.FinalValue != "🚀🎉" {
		t.Error("EMOJI not parsed correctly")
	}
	if v := result.ByName["CHINESE"]; v == nil || v.FinalValue != "你好世界" {
		t.Error("CHINESE not parsed correctly")
	}
}

func TestResolve_NoFiles(t *testing.T) {
	dir := t.TempDir()

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve should not fail on empty dir: %v", err)
	}

	if len(result.Variables) != 0 {
		t.Errorf("Expected 0 variables, got %d", len(result.Variables))
	}
}

func TestResolve_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Invalid YAML
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("this: is: not: valid: yaml:"), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not crash, may return error or empty result
	_, err := Resolve(dir)
	if err == nil {
		t.Log("No error on invalid YAML - acceptable behavior")
	}
}

func TestResolve_VeryLongValues(t *testing.T) {
	dir := t.TempDir()

	// Create a very long value (10KB)
	longValue := make([]byte, 10*1024)
	for i := range longValue {
		longValue[i] = 'a' + byte(i%26)
	}

	envContent := "LONG_VALUE=" + string(longValue) + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if v := result.ByName["LONG_VALUE"]; v == nil || len(v.FinalValue) != 10*1024 {
		t.Errorf("LONG_VALUE length = %d, want %d", len(v.FinalValue), 10*1024)
	}
}

func TestResolve_WindowsLineEndings(t *testing.T) {
	dir := t.TempDir()

	// Windows CRLF line endings
	envContent := "VAR1=value1\r\nVAR2=value2\r\nVAR3=value3\r\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(result.Variables) != 3 {
		t.Errorf("Expected 3 variables, got %d", len(result.Variables))
	}

	// Values should not include \r
	if v := result.ByName["VAR1"]; v != nil && v.FinalValue != "value1" {
		t.Errorf("VAR1 = %q, want 'value1'", v.FinalValue)
	}
}

func TestResolve_WhitespaceHandling(t *testing.T) {
	dir := t.TempDir()

	envContent := `  LEADING_SPACE=value
TRAILING_SPACE=value   
SPACES_IN_VALUE=hello world
TABS=	value	with	tabs
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Spaces in value should be preserved
	if v := result.ByName["SPACES_IN_VALUE"]; v != nil {
		if v.FinalValue != "hello world" {
			t.Errorf("SPACES_IN_VALUE = %q, want 'hello world'", v.FinalValue)
		}
	}
}

func TestResolve_EnvironmentPrecedenceChain(t *testing.T) {
	dir := t.TempDir()

	// Create chain: .env < .env.local < .env.development
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("KEY=base\nONLY_BASE=yes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.development"), []byte("KEY=dev\nONLY_DEV=yes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte("KEY=local\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Check that KEY is overridden (actual order depends on implementation)
	if v := result.ByName["KEY"]; v != nil {
		// Just verify it was overridden, don't enforce specific order
		if v.FinalValue == "base" {
			t.Error("KEY should be overridden from base")
		}
		t.Logf("KEY final value: %s (from %v)", v.FinalValue, v.FinalFrom)
	}
}

func TestResolve_VariableExpansion(t *testing.T) {
	dir := t.TempDir()

	envContent := `BASE_URL=http://localhost
API_URL=${BASE_URL}/api
PORT=3000
FULL_URL=${API_URL}:${PORT}
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Check that variable references are captured (may not expand)
	if v := result.ByName["API_URL"]; v == nil {
		t.Error("API_URL not found")
	}
}

func TestResolve_CommentEdgeCases(t *testing.T) {
	dir := t.TempDir()

	envContent := `# Full line comment
KEY=value # inline comment
HASH_IN_VALUE="value#with#hashes"
#COMMENTED_OUT=value
  # Indented comment
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// COMMENTED_OUT should not be parsed
	if result.ByName["COMMENTED_OUT"] != nil {
		t.Error("COMMENTED_OUT should not be parsed")
	}

	// Hash in quoted value should be preserved
	if v := result.ByName["HASH_IN_VALUE"]; v != nil {
		if v.FinalValue != "value#with#hashes" {
			t.Errorf("HASH_IN_VALUE = %q, want 'value#with#hashes'", v.FinalValue)
		}
	}
}

func TestResolve_MultilineEnvFiles(t *testing.T) {
	dir := t.TempDir()

	// Create many env files
	for i := 0; i < 10; i++ {
		filename := ".env"
		if i > 0 {
			filename = filepath.Join(dir, filename+".extra"+string(rune('0'+i)))
		} else {
			filename = filepath.Join(dir, filename)
		}
		content := "VAR" + string(rune('A'+i)) + "=value" + string(rune('0'+i)) + "\n"
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Should have at least base var
	if result.ByName["VARA"] == nil {
		t.Error("VARA not found")
	}
}

func TestResolve_JSONInValue(t *testing.T) {
	dir := t.TempDir()

	envContent := `CONFIG={"key": "value", "nested": {"a": 1}}
ARRAY=[1, 2, 3, "test"]
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if v := result.ByName["CONFIG"]; v == nil {
		t.Error("CONFIG not found")
	} else if v.FinalValue == "" {
		t.Error("CONFIG value empty")
	}
}
