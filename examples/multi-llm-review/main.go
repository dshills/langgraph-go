package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/providers"
	"github.com/dshills/langgraph-go/examples/multi-llm-review/scanner"
	"github.com/dshills/langgraph-go/examples/multi-llm-review/workflow"
	yaml "go.yaml.in/yaml/v2"
)

// Args represents parsed command-line arguments.
type Args struct {
	// CodebasePath is the required path to the codebase to review
	CodebasePath string
	// ConfigFile is the path to the configuration YAML file (default: config.yaml)
	ConfigFile string
	// Format is the output format (markdown or json, default: markdown)
	Format string
	// Resume indicates whether to resume from a previous run
	Resume bool
	// Err is any error encountered during parsing
	Err error
}

// Config represents the structure of the configuration YAML file.
type Config struct {
	Providers []struct {
		Name    string `yaml:"name"`
		APIKey  string `yaml:"api_key"`
		Model   string `yaml:"model"`
		Enabled bool   `yaml:"enabled"`
	} `yaml:"providers"`
	Review struct {
		BatchSize       int      `yaml:"batch_size"`
		FocusAreas      []string `yaml:"focus_areas"`
		IncludePatterns []string `yaml:"include_patterns"`
		ExcludePatterns []string `yaml:"exclude_patterns"`
	} `yaml:"review"`
	Output struct {
		Directory string `yaml:"directory"`
		Format    string `yaml:"format"`
	} `yaml:"output"`
}

// parseArgs parses command-line arguments and returns an Args struct.
// Flags can appear before or after the positional codebase path.
// If parsing fails, the Err field will contain the error.
func parseArgs(osArgs []string) Args {
	// Separate positional arguments from flags
	// Find the first non-flag argument that isn't a flag value
	var codebasePath string
	var flagArgs []string

	for i := 0; i < len(osArgs); i++ {
		arg := osArgs[i]

		// Check if this looks like a flag
		if len(arg) > 0 && arg[0] == '-' && arg != "-" {
			flagArgs = append(flagArgs, arg)

			// If this flag expects a value, capture the next arg as its value
			flagName := arg
			if len(arg) > 2 && arg[:2] == "--" {
				flagName = arg[2:]
			} else if len(arg) > 1 {
				flagName = arg[1:]
			}

			// config and format expect values, resume doesn't
			if flagName != "resume" && i+1 < len(osArgs) {
				i++
				flagArgs = append(flagArgs, osArgs[i])
			}
		} else {
			// This is not a flag or flag value, so it's the positional codebase path
			if codebasePath == "" {
				codebasePath = arg
				// Collect all remaining args (could be more flags)
				flagArgs = append(flagArgs, osArgs[i+1:]...)
				break
			}
		}
	}

	// Now parse flags separately
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage message

	configFile := fs.String("config", "config.yaml", "path to config YAML file")
	format := fs.String("format", "markdown", "output format (markdown or json)")
	resume := fs.Bool("resume", false, "resume from previous run")

	if err := fs.Parse(flagArgs); err != nil {
		return Args{Err: fmt.Errorf("flag parsing error: %w", err)}
	}

	if codebasePath == "" {
		return Args{Err: fmt.Errorf("required argument missing: codebase path")}
	}

	return Args{
		CodebasePath: codebasePath,
		ConfigFile:   *configFile,
		Format:       *format,
		Resume:       *resume,
	}
}

// loadConfig loads and parses a YAML configuration file.
// Returns a Config struct or an error if the file cannot be read or parsed.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return &config, nil
}

// main is the entry point for the multi-LLM code review application.
// It parses command-line arguments, loads configuration, and executes the review workflow.
func main() {
	args := parseArgs(os.Args[1:])
	if args.Err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", args.Err)
		os.Exit(1)
	}

	config, err := loadConfig(args.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Display startup information
	fmt.Printf("Multi-LLM Code Review\n")
	fmt.Printf("====================\n\n")
	fmt.Printf("Codebase: %s\n", args.CodebasePath)
	fmt.Printf("Config: %s\n", args.ConfigFile)
	fmt.Printf("Output format: %s\n", args.Format)
	fmt.Printf("Batch size: %d files\n\n", config.Review.BatchSize)

	// Create scanner from config patterns
	fileScanner := &scanner.ScannerAdapter{
		Scanner: &scanner.Scanner{
			IncludePatterns: config.Review.IncludePatterns,
			ExcludePatterns: config.Review.ExcludePatterns,
		},
	}

	// Create mock provider for now (replace with real providers based on config in future)
	// In production, this would iterate through config.Providers and create real providers
	mockProvider := &providers.MockProvider{
		Issues: []providers.ReviewIssue{
			{
				File:         "example.go",
				Line:         42,
				Severity:     "high",
				Category:     "security",
				Description:  "Example security issue",
				Remediation:  "Fix the security issue",
				ProviderName: "mock",
				Confidence:   0.95,
			},
		},
	}

	// Wrap provider with adapter to match workflow interface
	providerAdapter := &providers.ProviderAdapter{
		Provider: mockProvider,
	}

	// Create workflow engine
	engine, err := workflow.NewReviewWorkflow(providerAdapter, fileScanner, config.Review.BatchSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating workflow: %v\n", err)
		os.Exit(1)
	}

	// Set initial state
	initialState := workflow.ReviewState{
		CodebaseRoot: args.CodebasePath,
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]workflow.Review),
	}

	// Generate unique run ID
	runID := fmt.Sprintf("review-%d", time.Now().Unix())

	// Execute workflow
	ctx := context.Background()
	fmt.Printf("Starting review workflow (run ID: %s)...\n\n", runID)

	finalState, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running workflow: %v\n", err)
		os.Exit(1)
	}

	// Display results
	fmt.Printf("\n")
	fmt.Printf("Review Complete\n")
	fmt.Printf("===============\n\n")
	fmt.Printf("Files reviewed: %d\n", finalState.TotalFilesReviewed)
	fmt.Printf("Issues found: %d\n", len(finalState.ConsolidatedIssues))
	fmt.Printf("Report generated: %s\n", finalState.ReportPath)

	// Display any errors
	if finalState.LastError != "" {
		fmt.Printf("\nWarning: %s\n", finalState.LastError)
	}

	// Exit with appropriate code
	if finalState.LastError != "" {
		os.Exit(1)
	}
}

// runWorkflow is a helper function to execute the review workflow.
// It's extracted for testing purposes to allow tests to verify workflow execution.
// Returns the report path and any error encountered.
func runWorkflow(codebasePath string, batchSize int, outputDir string) (string, error) {
	// Create scanner with Go file patterns
	fileScanner := &scanner.ScannerAdapter{
		Scanner: &scanner.Scanner{
			IncludePatterns: []string{"*.go"},
			ExcludePatterns: []string{"*_test.go", "vendor/*", ".git/*"},
		},
	}

	// Create mock provider
	mockProvider := &providers.MockProvider{
		Issues: []providers.ReviewIssue{
			{
				File:         "test.go",
				Line:         1,
				Severity:     "info",
				Category:     "style",
				Description:  "Test issue",
				Remediation:  "Fix test issue",
				ProviderName: "mock",
				Confidence:   0.8,
			},
		},
	}

	// Wrap provider with adapter to match workflow interface
	providerAdapter := &providers.ProviderAdapter{
		Provider: mockProvider,
	}

	// Create workflow engine
	engine, err := workflow.NewReviewWorkflow(providerAdapter, fileScanner, batchSize)
	if err != nil {
		return "", fmt.Errorf("failed to create workflow: %w", err)
	}

	// Set initial state
	initialState := workflow.ReviewState{
		CodebaseRoot: codebasePath,
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]workflow.Review),
	}

	// Generate run ID
	runID := fmt.Sprintf("test-run-%d", time.Now().Unix())

	// Execute workflow
	ctx := context.Background()
	finalState, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		return "", fmt.Errorf("workflow execution failed: %w", err)
	}

	return finalState.ReportPath, nil
}
