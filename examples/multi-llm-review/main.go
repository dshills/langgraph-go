package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/internal"
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

	// Get default config path in user's home directory
	defaultConfigPath := getDefaultConfigPath()

	// Now parse flags separately
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.Usage = func() {} // Suppress default usage message

	configFile := fs.String("config", defaultConfigPath, "path to config YAML file")
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

// getDefaultConfigPath returns the default config file path using OS-appropriate config directory.
// Returns a platform-specific config path (e.g., ~/.config/multi-llm-review/config.yaml on Linux).
func getDefaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// If UserConfigDir fails, try home directory as fallback
		homeDir, homeErr := os.UserHomeDir()
		if homeErr != nil {
			// Last resort: current directory (not recommended for production)
			return "config.yaml"
		}
		return filepath.Join(homeDir, ".multi-llm-review", "config.yaml")
	}
	return filepath.Join(configDir, "multi-llm-review", "config.yaml")
}

// Default configuration template
const defaultConfigTemplate = `# Multi-LLM Code Review Configuration

# AI Provider Configuration
providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}  # Set via environment variable
    model: gpt-4
    enabled: true

  - name: anthropic
    api_key: ${ANTHROPIC_API_KEY}  # Set via environment variable
    model: claude-3-5-sonnet-20241022
    enabled: true

  - name: google
    api_key: ${GOOGLE_API_KEY}  # Set via environment variable
    model: gemini-1.5-flash
    enabled: false  # Set to true and provide API key to enable

# Review Configuration
review:
  # Number of files per batch (adjust based on file size)
  batch_size: 20

  # Focus areas for code review
  focus_areas:
    - security
    - performance
    - best-practices

  # File patterns to include (glob syntax)
  include_patterns:
    - "*.go"
    - "*.py"
    - "*.js"
    - "*.ts"

  # File patterns to exclude
  exclude_patterns:
    - "*_test.go"
    - "vendor/**"
    - "node_modules/**"
    - "*.pb.go"

# Output Configuration
output:
  directory: "./review-results"
  format: markdown  # markdown or json
`

// createDefaultConfig creates a default config file at the specified path with secure permissions.
// Returns error if the file already exists (will not overwrite).
func createDefaultConfig(cfgPath string) error {
	// Create parent directory with secure permissions (0700 - owner only)
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create config file with O_EXCL to prevent overwriting existing file
	// Use 0600 permissions (owner read/write only) for security
	file, err := os.OpenFile(cfgPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Config already exists - this is not an error
			return nil
		}
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Write default config
	if _, err := file.WriteString(defaultConfigTemplate); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
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

	// Expand environment variables in API keys
	for i := range config.Providers {
		config.Providers[i].APIKey = expandEnvVars(config.Providers[i].APIKey)
	}

	return &config, nil
}

// expandEnvVars expands environment variable references like ${VAR_NAME} in the input string.
// If the environment variable is not set, it returns an empty string for that variable.
func expandEnvVars(s string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name (remove ${ and })
		varName := match[2 : len(match)-1]
		return os.Getenv(varName)
	})
}

// createProviders creates real provider instances based on the configuration.
// Returns a slice of enabled and properly configured providers and their names.
func createProviders(config *Config) ([]workflow.CodeReviewer, []string, error) {
	var providersList []workflow.CodeReviewer
	var providerNames []string

	for _, providerConfig := range config.Providers {
		if !providerConfig.Enabled {
			continue
		}

		if providerConfig.APIKey == "" {
			fmt.Fprintf(os.Stderr, "Warning: %s provider enabled but API key is empty, skipping\n", providerConfig.Name)
			continue
		}

		var provider providers.CodeReviewer
		var err error

		switch providerConfig.Name {
		case "openai":
			provider, err = providers.NewOpenAIProvider(providerConfig.APIKey, providerConfig.Model)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create OpenAI provider: %w", err)
			}
			providerNames = append(providerNames, "OpenAI")

		case "anthropic":
			provider = providers.NewAnthropicProvider(providerConfig.APIKey, providerConfig.Model)
			providerNames = append(providerNames, "Anthropic")

		case "google":
			provider, err = providers.NewGoogleProvider(providerConfig.APIKey, providerConfig.Model)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create Google provider: %w", err)
			}
			providerNames = append(providerNames, "Google")

		default:
			fmt.Fprintf(os.Stderr, "Warning: Unknown provider '%s', skipping\n", providerConfig.Name)
			continue
		}

		// Wrap provider with adapter to match workflow interface
		providerAdapter := &providers.ProviderAdapter{
			Provider: provider,
		}
		providersList = append(providersList, providerAdapter)
	}

	if len(providersList) == 0 {
		return nil, nil, fmt.Errorf("no enabled providers found in configuration")
	}

	return providersList, providerNames, nil
}

// main is the entry point for the multi-LLM code review application.
// It parses command-line arguments, loads configuration, and executes the review workflow.
func main() {
	args := parseArgs(os.Args[1:])
	if args.Err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", args.Err)
		os.Exit(1)
	}

	// Ensure config directory exists if using default path
	if args.ConfigFile == getDefaultConfigPath() {
		configDir := filepath.Dir(args.ConfigFile)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
			os.Exit(1)
		}

		// If config doesn't exist, create it from the embedded default
		if _, err := os.Stat(args.ConfigFile); os.IsNotExist(err) {
			if err := createDefaultConfig(args.ConfigFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating default config: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Created default config at %s\n", args.ConfigFile)
			fmt.Fprintf(os.Stderr, "Please set your API keys in the config file or via environment variables\n\n")
		}
	}

	config, err := loadConfig(args.ConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create real providers from config
	providersList, providerNames, err := createProviders(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Display startup information (minimal)
	fmt.Printf("Reviewing %s with %s...\n\n", args.CodebasePath, internal.FormatProviderList(providerNames))

	// Create scanner from config patterns
	fileScanner := &scanner.ScannerAdapter{
		Scanner: &scanner.Scanner{
			IncludePatterns: config.Review.IncludePatterns,
			ExcludePatterns: config.Review.ExcludePatterns,
		},
	}

	// Create progress emitter for clean output
	progressEmitter := internal.NewProgressEmitter(os.Stdout, providerNames)

	// Create workflow engine with multiple providers and progress emitter
	engine, err := workflow.NewReviewWorkflowWithProvidersAndEmitter(providersList, fileScanner, config.Review.BatchSize, progressEmitter)
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

	finalState, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Workflow error: %v\n", err)
		os.Exit(1)
	}

	// Display results (minimal)
	fmt.Printf("\n✓ Review complete\n")
	fmt.Printf("  Files reviewed: %d\n", finalState.TotalFilesReviewed)
	fmt.Printf("  Issues found: %d\n", len(finalState.ConsolidatedIssues))
	fmt.Printf("  Report: %s\n", finalState.ReportPath)

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
// This function uses mock providers for testing. For real usage, use the main function
// with a proper configuration file.
// Returns the report path and any error encountered.
func runWorkflow(codebasePath string, batchSize int, outputDir string) (string, error) {
	// Create scanner with Go file patterns
	fileScanner := &scanner.ScannerAdapter{
		Scanner: &scanner.Scanner{
			IncludePatterns: []string{"*.go"},
			ExcludePatterns: []string{"*_test.go", "vendor/*", ".git/*"},
		},
	}

	// For testing, use mock provider
	// In production, use createProviders() with a real config file
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

	// Create workflow engine with mock provider for testing
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
