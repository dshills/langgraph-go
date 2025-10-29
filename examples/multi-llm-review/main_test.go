package main

import (
	"os"
	"testing"
)

// TestParseArgs_ValidCodebase tests parsing with a valid codebase path.
func TestParseArgs_ValidCodebase(t *testing.T) {
	args := []string{"./testdata/fixtures"}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "./testdata/fixtures" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "./testdata/fixtures")
	}
	if parsed.ConfigFile != "config.yaml" {
		t.Errorf("ConfigFile = %q, want %q", parsed.ConfigFile, "config.yaml")
	}
	if parsed.Format != "markdown" {
		t.Errorf("Format = %q, want %q", parsed.Format, "markdown")
	}
	if parsed.Resume != false {
		t.Errorf("Resume = %v, want false", parsed.Resume)
	}
}

// TestParseArgs_CustomConfig tests parsing with --config flag.
func TestParseArgs_CustomConfig(t *testing.T) {
	args := []string{"./testdata", "--config", "custom.yaml"}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "./testdata" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "./testdata")
	}
	if parsed.ConfigFile != "custom.yaml" {
		t.Errorf("ConfigFile = %q, want %q", parsed.ConfigFile, "custom.yaml")
	}
}

// TestParseArgs_CustomFormat tests parsing with --format flag.
func TestParseArgs_CustomFormat(t *testing.T) {
	args := []string{"/tmp/codebase", "--format", "json"}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "/tmp/codebase" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "/tmp/codebase")
	}
	if parsed.Format != "json" {
		t.Errorf("Format = %q, want %q", parsed.Format, "json")
	}
}

// TestParseArgs_Resume tests parsing with --resume flag.
func TestParseArgs_Resume(t *testing.T) {
	args := []string{"/tmp/codebase", "--resume"}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "/tmp/codebase" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "/tmp/codebase")
	}
	if parsed.Resume != true {
		t.Errorf("Resume = %v, want true", parsed.Resume)
	}
}

// TestParseArgs_AllFlags tests parsing with all flags set.
func TestParseArgs_AllFlags(t *testing.T) {
	args := []string{
		"/src/myproject",
		"--config", "review.yaml",
		"--format", "json",
		"--resume",
	}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "/src/myproject" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "/src/myproject")
	}
	if parsed.ConfigFile != "review.yaml" {
		t.Errorf("ConfigFile = %q, want %q", parsed.ConfigFile, "review.yaml")
	}
	if parsed.Format != "json" {
		t.Errorf("Format = %q, want %q", parsed.Format, "json")
	}
	if parsed.Resume != true {
		t.Errorf("Resume = %v, want true", parsed.Resume)
	}
}

// TestParseArgs_MissingCodebase tests that missing codebase path returns an error.
func TestParseArgs_MissingCodebase(t *testing.T) {
	args := []string{}

	parsed := parseArgs(args)

	if parsed.Err == nil {
		t.Error("expected error for missing codebase path, got nil")
	}
}

// TestParseArgs_FlagBeforePositional tests flags before the positional argument.
func TestParseArgs_FlagBeforePositional(t *testing.T) {
	args := []string{
		"--config", "settings.yaml",
		"./code",
		"--format", "markdown",
	}

	parsed := parseArgs(args)

	if parsed.CodebasePath != "./code" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "./code")
	}
	if parsed.ConfigFile != "settings.yaml" {
		t.Errorf("ConfigFile = %q, want %q", parsed.ConfigFile, "settings.yaml")
	}
	if parsed.Format != "markdown" {
		t.Errorf("Format = %q, want %q", parsed.Format, "markdown")
	}
}

// TestLoadConfig tests loading a YAML configuration file.
func TestLoadConfig(t *testing.T) {
	// Use the example config file that exists in the project
	configPath := "config.example.yaml"

	config, err := loadConfig(configPath)

	if err != nil {
		t.Fatalf("loadConfig(%q) failed: %v", configPath, err)
	}
	if config == nil {
		t.Error("loadConfig returned nil config")
	}

	// Verify structure (config has Providers field)
	if len(config.Providers) == 0 {
		t.Error("expected at least one provider in config")
	}

	// Verify first provider
	if config.Providers[0].Name != "openai" {
		t.Errorf("first provider name = %q, want %q", config.Providers[0].Name, "openai")
	}
}

// TestLoadConfig_MissingFile tests that missing config file returns an error.
func TestLoadConfig_MissingFile(t *testing.T) {
	configPath := "/nonexistent/path/to/config.yaml"

	_, err := loadConfig(configPath)

	if err == nil {
		t.Errorf("loadConfig(%q) expected error for missing file, got nil", configPath)
	}
}

// TestLoadConfig_InvalidYAML tests that invalid YAML returns an error.
func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	tmpFile, err := os.CreateTemp("", "invalid-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("invalid: yaml: content: ["); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	_, err = loadConfig(tmpFile.Name())

	if err == nil {
		t.Error("loadConfig expected error for invalid YAML, got nil")
	}
}

// TestArgsParsing_PositionalOnly tests parsing with only positional argument.
func TestArgsParsing_PositionalOnly(t *testing.T) {
	args := []string{"/home/user/project"}

	parsed := parseArgs(args)

	if parsed.Err != nil {
		t.Fatalf("parseArgs() unexpected error: %v", parsed.Err)
	}
	if parsed.CodebasePath != "/home/user/project" {
		t.Errorf("CodebasePath = %q, want %q", parsed.CodebasePath, "/home/user/project")
	}
	if parsed.ConfigFile != "config.yaml" {
		t.Errorf("ConfigFile = %q, want default", parsed.ConfigFile)
	}
	if parsed.Format != "markdown" {
		t.Errorf("Format = %q, want default", parsed.Format)
	}
	if parsed.Resume {
		t.Errorf("Resume = %v, want false by default", parsed.Resume)
	}
}

// TestParseArgs_ResumeWithoutValue tests that --resume flag works as a boolean.
func TestParseArgs_ResumeWithoutValue(t *testing.T) {
	args := []string{"/tmp/code", "--resume", "--config", "other.yaml"}

	parsed := parseArgs(args)

	if !parsed.Resume {
		t.Errorf("Resume = %v, want true", parsed.Resume)
	}
	if parsed.ConfigFile != "other.yaml" {
		t.Errorf("ConfigFile = %q, want %q", parsed.ConfigFile, "other.yaml")
	}
}

// BenchmarkParseArgs benchmarks argument parsing performance.
func BenchmarkParseArgs(b *testing.B) {
	args := []string{
		"/tmp/codebase",
		"--config", "config.yaml",
		"--format", "markdown",
		"--resume",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseArgs(args)
	}
}

// TestWorkflowExecution_SuccessfulRun tests successful workflow execution with mock provider.
func TestWorkflowExecution_SuccessfulRun(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test Go file
	testFile := tmpDir + "/test.go"
	testContent := `package main

func main() {
	println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create output directory for test run
	outputDir := tmpDir + "/review-results"

	// Run workflow execution using runWorkflow (the extracted helper)
	reportPath, err := runWorkflow(tmpDir, 5, outputDir)

	// Verify workflow completed successfully
	if err != nil {
		t.Fatalf("runWorkflow() failed: %v", err)
	}

	// Verify report was created
	if reportPath == "" {
		t.Error("runWorkflow() returned empty report path")
	}

	// Verify report file exists
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Errorf("report file does not exist: %s", reportPath)
	}
}

// TestWorkflowExecution_InvalidCodebasePath tests workflow with non-existent codebase path.
func TestWorkflowExecution_InvalidCodebasePath(t *testing.T) {
	// Use a path that doesn't exist
	invalidPath := "/nonexistent/path/to/codebase"

	// Create output directory for test run
	outputDir := t.TempDir() + "/review-results"

	// Run workflow execution - should fail during discovery
	_, err := runWorkflow(invalidPath, 5, outputDir)

	// Verify error occurred
	if err == nil {
		t.Error("runWorkflow() expected error for invalid path, got nil")
	}
}

// TestWorkflowExecution_EmptyCodebase tests workflow with empty directory.
func TestWorkflowExecution_EmptyCodebase(t *testing.T) {
	// Create an empty temporary directory
	tmpDir := t.TempDir()

	// Create output directory for test run
	outputDir := tmpDir + "/review-results"

	// Run workflow execution - should fail gracefully with no files
	_, err := runWorkflow(tmpDir, 5, outputDir)

	// Workflow should fail when no files are found (no batches to process)
	if err == nil {
		t.Error("runWorkflow() expected error for empty directory, got nil")
	}
}
