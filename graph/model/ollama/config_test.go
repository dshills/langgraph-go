package ollama

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantErr   bool
		errSubstr string // substring expected in error message
	}{
		{
			name: "valid config with all fields",
			config: Config{
				Model:       "gpt-oss",
				Endpoint:    "http://localhost:11434",
				Temperature: Float64Ptr(0.8),
				TopP:        Float64Ptr(0.9),
				NumPredict:  IntPtr(2048),
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			config: Config{
				Model: "llama3.2",
			},
			wantErr: false,
		},
		{
			name: "valid config with seed",
			config: Config{
				Model:       "mistral",
				Temperature: Float64Ptr(0.7),
				Seed:        IntPtr(42),
			},
			wantErr: false,
		},
		{
			name: "valid config with custom HTTP client",
			config: Config{
				Model: "codellama",
				HTTPClient: &http.Client{
					Timeout: 60 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "valid temperature at minimum",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(0.0),
			},
			wantErr: false,
		},
		{
			name: "valid temperature at maximum",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(2.0),
			},
			wantErr: false,
		},
		{
			name: "valid TopP at minimum",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(0.0),
			},
			wantErr: false,
		},
		{
			name: "valid TopP at maximum",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(1.0),
			},
			wantErr: false,
		},
		{
			name: "valid NumPredict unlimited",
			config: Config{
				Model:      "gpt-oss",
				NumPredict: IntPtr(-1),
			},
			wantErr: false,
		},
		{
			name: "valid NumPredict with limit",
			config: Config{
				Model:      "gpt-oss",
				NumPredict: IntPtr(100),
			},
			wantErr: false,
		},
		{
			name: "empty model (required field)",
			config: Config{
				Endpoint: "http://localhost:11434",
			},
			wantErr:   true,
			errSubstr: "model is required",
		},
		{
			name: "temperature below minimum",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(-0.1),
			},
			wantErr:   true,
			errSubstr: "temperature must be between 0.0 and 2.0",
		},
		{
			name: "temperature above maximum",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(2.1),
			},
			wantErr:   true,
			errSubstr: "temperature must be between 0.0 and 2.0",
		},
		{
			name: "TopP below minimum",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(-0.1),
			},
			wantErr:   true,
			errSubstr: "top_p must be between 0.0 and 1.0",
		},
		{
			name: "TopP above maximum",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(1.1),
			},
			wantErr:   true,
			errSubstr: "top_p must be between 0.0 and 1.0",
		},
		{
			name: "NumPredict below minimum",
			config: Config{
				Model:      "gpt-oss",
				NumPredict: IntPtr(-2),
			},
			wantErr:   true,
			errSubstr: "num_predict must be >= -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test table
			cfg := tt.config

			err := validateConfig(&cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConfig() expected error, got nil")
					return
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("validateConfig() error = %v, want error containing %q", err, tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("validateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateConfig_DefaultEndpoint(t *testing.T) {
	cfg := Config{
		Model: "gpt-oss",
		// Endpoint intentionally empty
	}

	err := validateConfig(&cfg)
	if err != nil {
		t.Fatalf("validateConfig() unexpected error = %v", err)
	}

	expectedEndpoint := "http://localhost:11434"
	if cfg.Endpoint != expectedEndpoint {
		t.Errorf("validateConfig() endpoint = %q, want %q", cfg.Endpoint, expectedEndpoint)
	}
}

func TestValidateConfig_PreservesEndpoint(t *testing.T) {
	customEndpoint := "http://ollama-server:11434"
	cfg := Config{
		Model:    "gpt-oss",
		Endpoint: customEndpoint,
	}

	err := validateConfig(&cfg)
	if err != nil {
		t.Fatalf("validateConfig() unexpected error = %v", err)
	}

	if cfg.Endpoint != customEndpoint {
		t.Errorf("validateConfig() endpoint = %q, want %q", cfg.Endpoint, customEndpoint)
	}
}

func TestValidateConfig_PreservesOtherFields(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}

	cfg := Config{
		Model:       "gpt-oss",
		Temperature: Float64Ptr(0.7),
		TopP:        Float64Ptr(0.9),
		Seed:        IntPtr(42),
		NumPredict:  IntPtr(2048),
		HTTPClient:  customClient,
	}

	err := validateConfig(&cfg)
	if err != nil {
		t.Fatalf("validateConfig() unexpected error = %v", err)
	}

	// Verify all fields preserved
	if cfg.Model != "gpt-oss" {
		t.Errorf("Model changed: got %q", cfg.Model)
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.7 {
		t.Errorf("Temperature changed: got %v", cfg.Temperature)
	}
	if cfg.TopP == nil || *cfg.TopP != 0.9 {
		t.Errorf("TopP changed: got %v", cfg.TopP)
	}
	if cfg.Seed == nil || *cfg.Seed != 42 {
		t.Errorf("Seed changed: got %v", cfg.Seed)
	}
	if cfg.NumPredict == nil || *cfg.NumPredict != 2048 {
		t.Errorf("NumPredict changed: got %v", cfg.NumPredict)
	}
	if cfg.HTTPClient != customClient {
		t.Errorf("HTTPClient changed")
	}
}
