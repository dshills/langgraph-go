package bedrock

import (
	"strings"
	"testing"
)

// T011: Test BedrockConfig.Validate() - region validation
func TestConfig_Validate_Region(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid region us-east-1",
			config: Config{
				Region:  "us-east-1",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: false,
		},
		{
			name: "valid region us-west-2",
			config: Config{
				Region:  "us-west-2",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: false,
		},
		{
			name: "valid region eu-west-1",
			config: Config{
				Region:  "eu-west-1",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: false,
		},
		{
			name: "empty region",
			config: Config{
				Region:  "",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: true,
			errMsg:  "region",
		},
		{
			name: "invalid region format",
			config: Config{
				Region:  "invalid-region-123",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: true,
			errMsg:  "region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// T012: Test BedrockConfig.Validate() - modelID format validation
func TestConfig_Validate_ModelID(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid Claude model ID",
			config: Config{
				Region:  "us-east-1",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: false,
		},
		{
			name: "valid Llama model ID",
			config: Config{
				Region:  "us-east-1",
				ModelID: "meta.llama3-2-90b-instruct-v1:0",
			},
			wantErr: false,
		},
		{
			name: "valid Titan model ID",
			config: Config{
				Region:  "us-east-1",
				ModelID: "amazon.titan-text-premier-v1:0",
			},
			wantErr: false,
		},
		{
			name: "empty model ID",
			config: Config{
				Region:  "us-east-1",
				ModelID: "",
			},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name: "invalid model ID format (no dot)",
			config: Config{
				Region:  "us-east-1",
				ModelID: "invalidmodel",
			},
			wantErr: true,
			errMsg:  "model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// T013: Test BedrockConfig.Validate() - temperature range validation
func TestConfig_Validate_Temperature(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid temperature 0.0",
			config: Config{
				Region:      "us-east-1",
				ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
				Temperature: 0.0,
			},
			wantErr: false,
		},
		{
			name: "valid temperature 0.7",
			config: Config{
				Region:      "us-east-1",
				ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
				Temperature: 0.7,
			},
			wantErr: false,
		},
		{
			name: "valid temperature 1.0",
			config: Config{
				Region:      "us-east-1",
				ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
				Temperature: 1.0,
			},
			wantErr: false,
		},
		{
			name: "invalid temperature below 0",
			config: Config{
				Region:      "us-east-1",
				ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
				Temperature: -0.1,
			},
			wantErr: true,
			errMsg:  "temperature",
		},
		{
			name: "invalid temperature above 1",
			config: Config{
				Region:      "us-east-1",
				ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
				Temperature: 1.5,
			},
			wantErr: true,
			errMsg:  "temperature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

// T014: Test BedrockConfig.Validate() - maxTokens validation
func TestConfig_Validate_MaxTokens(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid maxTokens 1",
			config: Config{
				Region:    "us-east-1",
				ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
				MaxTokens: 1,
			},
			wantErr: false,
		},
		{
			name: "valid maxTokens 4096",
			config: Config{
				Region:    "us-east-1",
				ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
				MaxTokens: 4096,
			},
			wantErr: false,
		},
		{
			name: "valid maxTokens 200000",
			config: Config{
				Region:    "us-east-1",
				ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
				MaxTokens: 200000,
			},
			wantErr: false,
		},
		{
			name: "zero maxTokens (should use default)",
			config: Config{
				Region:    "us-east-1",
				ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
				MaxTokens: 0,
			},
			wantErr: false, // 0 means use default
		},
		{
			name: "invalid negative maxTokens",
			config: Config{
				Region:    "us-east-1",
				ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
				MaxTokens: -1,
			},
			wantErr: true,
			errMsg:  "tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}
