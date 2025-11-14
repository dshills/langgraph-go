package ollama

import (
	"fmt"
	"net/http"
	"time"
)

// Config holds configuration for the Ollama adapter.
//
// Ollama is a tool for running large language models locally on your machine.
// This config specifies which model to use, where the Ollama server is running,
// and generation parameters.
//
// Example:
//
//	temp := 0.7
//	config := ollama.Config{
//	    Model: "gpt-oss",
//	    Temperature: &temp,
//	}
//	adapter, err := ollama.NewChatModel(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
type Config struct {
	// Endpoint is the Ollama server URL.
	// Optional. Default: "http://localhost:11434"
	// Use this to connect to remote Ollama instances:
	//   - "http://localhost:11434" (local default)
	//   - "http://ollama-server:11434" (remote server)
	//   - "https://ollama.example.com" (HTTPS endpoint)
	Endpoint string

	// Model is the name of the Ollama model to use.
	// Required. Must be non-empty.
	// Examples:
	//   - "gpt-oss" (GPT-4 open source equivalent)
	//   - "llama3.2" (Meta's Llama 3.2)
	//   - "mistral" (Mistral AI model)
	//   - "codellama" (Code-specialized Llama)
	//
	// Model must be pulled before use:
	//   ollama pull gpt-oss
	//
	// List available models:
	//   ollama list
	Model string

	// Temperature controls randomness in model output.
	// Optional. Range: [0.0, 2.0]. Default: 0.8
	//   - 0.0: Deterministic, focused output
	//   - 0.8: Balanced creativity (default)
	//   - 2.0: Maximum randomness, diverse output
	//
	// Higher values increase diversity but may reduce coherence.
	// Use nil to accept the default. Use a pointer to explicitly set a value (including 0.0).
	Temperature *float64

	// TopP controls nucleus sampling diversity.
	// Optional. Range: [0.0, 1.0]. Default: 0.9
	//   - 0.1: Very focused on likely tokens
	//   - 0.9: Balanced (default)
	//   - 1.0: Consider all tokens
	//
	// Lower values make output more deterministic.
	// TopP and Temperature interact: use lower Temperature with higher TopP
	// for creative yet coherent output.
	// Use nil to accept the default. Use a pointer to explicitly set a value (including 0.0).
	TopP *float64

	// Seed provides deterministic generation when set.
	// Optional. Default: nil (non-deterministic)
	//
	// Use a seed to get reproducible outputs:
	//   seed := 42
	//   config := Config{Model: "gpt-oss", Seed: &seed}
	//
	// Same seed + same input + same model = same output
	// Useful for testing and debugging.
	Seed *int

	// NumPredict limits maximum tokens to generate.
	// Optional. Range: >= -1. Default: -1 (unlimited)
	//   - -1: No limit (generate until natural stop)
	//   - 100: Generate up to 100 tokens
	//   - 2048: Cap at 2048 tokens
	//
	// Use to control response length and API latency.
	// Model may stop earlier if it reaches a natural stopping point.
	// Use nil to accept the default (-1). Use a pointer to explicitly set a value.
	NumPredict *int

	// HTTPClient allows custom HTTP client configuration.
	// Optional. Default: http.Client with 60s timeout
	//
	// Use to configure:
	//   - Custom timeouts
	//   - TLS certificates
	//   - Proxy settings
	//   - Connection pooling
	//
	// Example:
	//   config := Config{
	//       Model: "gpt-oss",
	//       HTTPClient: &http.Client{
	//           Timeout: 120 * time.Second,
	//       },
	//   }
	HTTPClient *http.Client
}

// validateConfig checks if the configuration is valid and applies defaults.
//
// Validates:
//   - Model is non-empty (required field)
//   - Temperature is in range [0.0, 2.0] if set
//   - TopP is in range [0.0, 1.0] if set
//   - NumPredict is >= -1 if set
//
// Applies defaults:
//   - Endpoint: "http://localhost:11434" if empty
//   - Temperature: 0.8 if nil
//   - TopP: 0.9 if nil
//   - NumPredict: -1 if nil
//   - HTTPClient: http.DefaultClient with 60s timeout if nil
//
// Returns error describing first validation failure, or nil if valid.
// Mutates cfg to apply defaults.
//
// Example usage:
//
//	config := ollama.Config{Model: "gpt-oss"}
//	if err := validateConfig(&config); err != nil {
//	    log.Fatal(err)
//	}
func validateConfig(cfg *Config) error {
	// Validate Model (required)
	if cfg.Model == "" {
		return fmt.Errorf("model is required (e.g., \"gpt-oss\", \"llama3.2\")")
	}

	// Set default Endpoint if empty
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:11434"
	}

	// Validate and default Temperature
	if cfg.Temperature != nil {
		if *cfg.Temperature < 0.0 || *cfg.Temperature > 2.0 {
			return fmt.Errorf("temperature must be between 0.0 and 2.0, got: %f", *cfg.Temperature)
		}
	} else {
		defaultTemp := 0.8
		cfg.Temperature = &defaultTemp
	}

	// Validate and default TopP
	if cfg.TopP != nil {
		if *cfg.TopP < 0.0 || *cfg.TopP > 1.0 {
			return fmt.Errorf("top_p must be between 0.0 and 1.0, got: %f", *cfg.TopP)
		}
	} else {
		defaultTopP := 0.9
		cfg.TopP = &defaultTopP
	}

	// Validate and default NumPredict
	if cfg.NumPredict != nil {
		if *cfg.NumPredict < -1 {
			return fmt.Errorf("num_predict must be >= -1 (unlimited), got: %d", *cfg.NumPredict)
		}
	} else {
		defaultNumPredict := -1
		cfg.NumPredict = &defaultNumPredict
	}

	// Default HTTPClient
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 60 * time.Second,
		}
	}

	return nil
}

// Float64Ptr returns a pointer to the given float64 value.
// Helper for setting optional Config fields.
//
// Example:
//
//	config := Config{
//	    Model:       "gpt-oss",
//	    Temperature: Float64Ptr(0.0), // Explicitly set to 0.0
//	}
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr returns a pointer to the given int value.
// Helper for setting optional Config fields.
//
// Example:
//
//	config := Config{
//	    Model:      "gpt-oss",
//	    Seed:       IntPtr(42),       // Deterministic
//	    NumPredict: IntPtr(100),      // Limit to 100 tokens
//	}
func IntPtr(v int) *int {
	return &v
}
