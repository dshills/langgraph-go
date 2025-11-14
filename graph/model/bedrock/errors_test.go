package bedrock

import (
	"errors"
	"strings"
	"testing"
)

// T022: Test error wrapping (BedrockError)
func TestWrapAWSError(t *testing.T) {
	tests := []struct {
		name          string
		awsErr        error
		region        string
		wantCode      string
		wantRetryable bool
		wantMsg       string
	}{
		{
			name:          "ThrottlingException (retryable)",
			awsErr:        errors.New("ThrottlingException: Rate exceeded"),
			region:        "us-east-1",
			wantCode:      "ThrottlingException",
			wantRetryable: true,
			wantMsg:       "rate exceeded",
		},
		{
			name:          "ServiceUnavailableException (retryable)",
			awsErr:        errors.New("ServiceUnavailableException: Service unavailable"),
			region:        "us-west-2",
			wantCode:      "ServiceUnavailableException",
			wantRetryable: true,
			wantMsg:       "service unavailable",
		},
		{
			name:          "ModelTimeoutException (retryable)",
			awsErr:        errors.New("ModelTimeoutException: Model timeout"),
			region:        "eu-west-1",
			wantCode:      "ModelTimeoutException",
			wantRetryable: true,
			wantMsg:       "model timeout",
		},
		{
			name:          "ValidationException (non-retryable)",
			awsErr:        errors.New("ValidationException: Invalid input"),
			region:        "us-east-1",
			wantCode:      "ValidationException",
			wantRetryable: false,
			wantMsg:       "invalid input",
		},
		{
			name:          "AccessDeniedException (non-retryable)",
			awsErr:        errors.New("AccessDeniedException: Access denied"),
			region:        "us-east-1",
			wantCode:      "AccessDeniedException",
			wantRetryable: false,
			wantMsg:       "access denied",
		},
		{
			name:          "ResourceNotFoundException (non-retryable)",
			awsErr:        errors.New("ResourceNotFoundException: Model not found"),
			region:        "us-east-1",
			wantCode:      "ResourceNotFoundException",
			wantRetryable: false,
			wantMsg:       "model not found",
		},
		{
			name:          "ModelNotReadyException (retryable)",
			awsErr:        errors.New("ModelNotReadyException: Model loading"),
			region:        "us-east-1",
			wantCode:      "ModelNotReadyException",
			wantRetryable: true,
			wantMsg:       "model loading",
		},
		{
			name:          "InternalServerException (retryable)",
			awsErr:        errors.New("InternalServerException: Internal error"),
			region:        "us-east-1",
			wantCode:      "InternalServerException",
			wantRetryable: true,
			wantMsg:       "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bedrockErr := wrapAWSError(tt.awsErr, tt.region)

			if bedrockErr == nil {
				t.Fatal("wrapAWSError() returned nil")
			}

			if bedrockErr.Code != tt.wantCode {
				t.Errorf("wrapAWSError() Code = %q, want %q", bedrockErr.Code, tt.wantCode)
			}

			if bedrockErr.Region != tt.region {
				t.Errorf("wrapAWSError() Region = %q, want %q", bedrockErr.Region, tt.region)
			}

			if bedrockErr.Retryable != tt.wantRetryable {
				t.Errorf("wrapAWSError() Retryable = %v, want %v", bedrockErr.Retryable, tt.wantRetryable)
			}

			if !strings.Contains(strings.ToLower(bedrockErr.Message), strings.ToLower(tt.wantMsg)) {
				t.Errorf("wrapAWSError() Message = %q, want to contain %q", bedrockErr.Message, tt.wantMsg)
			}

			if bedrockErr.OriginalError != tt.awsErr {
				t.Errorf("wrapAWSError() OriginalError = %v, want %v", bedrockErr.OriginalError, tt.awsErr)
			}
		})
	}
}

// Test BedrockError.Error() formatting
func TestBedrockError_Error(t *testing.T) {
	tests := []struct {
		name       string
		bedrockErr *BedrockError
		wantMsg    string
		wantRegion string
	}{
		{
			name: "error with request ID",
			bedrockErr: &BedrockError{
				Code:      "ThrottlingException",
				Message:   "Rate exceeded",
				RequestID: "req-123456",
				Region:    "us-east-1",
				Retryable: true,
			},
			wantMsg:    "rate exceeded",
			wantRegion: "us-east-1",
		},
		{
			name: "error without request ID",
			bedrockErr: &BedrockError{
				Code:      "ValidationException",
				Message:   "Invalid input",
				RequestID: "",
				Region:    "eu-west-1",
				Retryable: false,
			},
			wantMsg:    "invalid input",
			wantRegion: "eu-west-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.bedrockErr.Error()

			if !strings.Contains(strings.ToLower(errStr), strings.ToLower(tt.wantMsg)) {
				t.Errorf("BedrockError.Error() = %q, want to contain %q", errStr, tt.wantMsg)
			}

			if !strings.Contains(errStr, tt.wantRegion) {
				t.Errorf("BedrockError.Error() = %q, want to contain region %q", errStr, tt.wantRegion)
			}

			if !strings.Contains(errStr, tt.bedrockErr.Code) {
				t.Errorf("BedrockError.Error() = %q, want to contain code %q", errStr, tt.bedrockErr.Code)
			}

			if tt.bedrockErr.RequestID != "" && !strings.Contains(errStr, tt.bedrockErr.RequestID) {
				t.Errorf("BedrockError.Error() = %q, want to contain request ID %q", errStr, tt.bedrockErr.RequestID)
			}
		})
	}
}

// Test isRetryableError classification
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		errorCode string
		want      bool
	}{
		// Retryable errors
		{"ThrottlingException", "ThrottlingException", true},
		{"ServiceUnavailableException", "ServiceUnavailableException", true},
		{"ModelTimeoutException", "ModelTimeoutException", true},
		{"ModelNotReadyException", "ModelNotReadyException", true},
		{"InternalServerException", "InternalServerException", true},

		// Non-retryable errors
		{"ValidationException", "ValidationException", false},
		{"AccessDeniedException", "AccessDeniedException", false},
		{"ResourceNotFoundException", "ResourceNotFoundException", false},
		{"ModelErrorException", "ModelErrorException", false},
		{"UnknownException", "UnknownException", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.errorCode)
			if got != tt.want {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.errorCode, got, tt.want)
			}
		})
	}
}
