package ollama

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOllamaError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *OllamaError
		expected string
	}{
		{
			name: "connection error",
			err: &OllamaError{
				Code:    "connection",
				Message: "Failed to connect to http://localhost:11434",
				Err:     errors.New("connection refused"),
			},
			expected: "ollama error [connection]: Failed to connect to http://localhost:11434",
		},
		{
			name: "model not found error",
			err: &OllamaError{
				Code:    "model_not_found",
				Message: "Model 'llama3.2' not found",
				Err:     nil,
			},
			expected: "ollama error [model_not_found]: Model 'llama3.2' not found",
		},
		{
			name: "timeout error",
			err: &OllamaError{
				Code:    "timeout",
				Message: "Request timed out",
				Err:     context.DeadlineExceeded,
			},
			expected: "ollama error [timeout]: Request timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestOllamaError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	ollamaErr := &OllamaError{
		Code:    "connection",
		Message: "test error",
		Err:     originalErr,
	}

	unwrapped := ollamaErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}

	// Test errors.Is compatibility
	if !errors.Is(ollamaErr, originalErr) {
		t.Error("errors.Is(ollamaErr, originalErr) = false, want true")
	}
}

func TestNewConnectionError(t *testing.T) {
	endpoint := "http://localhost:11434"
	originalErr := errors.New("connection refused")

	err := newConnectionError(endpoint, originalErr)

	if err.Code != "connection" {
		t.Errorf("Code = %q, want %q", err.Code, "connection")
	}

	if !strings.Contains(err.Message, endpoint) {
		t.Errorf("Message should contain endpoint %q, got %q", endpoint, err.Message)
	}

	if !strings.Contains(err.Message, "ollama serve") {
		t.Errorf("Message should contain guidance 'ollama serve', got %q", err.Message)
	}

	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestNewModelNotFoundError(t *testing.T) {
	model := "llama3.2"

	err := newModelNotFoundError(model)

	if err.Code != "model_not_found" {
		t.Errorf("Code = %q, want %q", err.Code, "model_not_found")
	}

	if !strings.Contains(err.Message, model) {
		t.Errorf("Message should contain model %q, got %q", model, err.Message)
	}

	if !strings.Contains(err.Message, "ollama pull") {
		t.Errorf("Message should contain guidance 'ollama pull', got %q", err.Message)
	}

	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
}

func TestNewInvalidRequestError(t *testing.T) {
	details := "temperature must be between 0 and 2"
	originalErr := errors.New("validation failed")

	err := newInvalidRequestError(details, originalErr)

	if err.Code != "invalid_request" {
		t.Errorf("Code = %q, want %q", err.Code, "invalid_request")
	}

	if !strings.Contains(err.Message, details) {
		t.Errorf("Message should contain details %q, got %q", details, err.Message)
	}

	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}
}

func TestNewTimeoutError(t *testing.T) {
	originalErr := context.DeadlineExceeded

	err := newTimeoutError(originalErr)

	if err.Code != "timeout" {
		t.Errorf("Code = %q, want %q", err.Code, "timeout")
	}

	if !strings.Contains(err.Message, "timed out") {
		t.Errorf("Message should mention timeout, got %q", err.Message)
	}

	if err.Err != originalErr {
		t.Errorf("Err = %v, want %v", err.Err, originalErr)
	}

	// Test errors.Is compatibility with context.DeadlineExceeded
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("errors.Is(err, context.DeadlineExceeded) = false, want true")
	}
}

func TestTranslateError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode string
	}{
		{
			name:         "nil error",
			err:          nil,
			expectedCode: "",
		},
		{
			name:         "deadline exceeded",
			err:          context.DeadlineExceeded,
			expectedCode: "timeout",
		},
		{
			name:         "timeout error",
			err:          errors.New("request timeout"),
			expectedCode: "timeout",
		},
		{
			name:         "connection refused",
			err:          errors.New("connection refused"),
			expectedCode: "connection",
		},
		{
			name:         "no such host",
			err:          errors.New("dial tcp: lookup localhost: no such host"),
			expectedCode: "connection",
		},
		{
			name:         "network unreachable",
			err:          errors.New("network is unreachable"),
			expectedCode: "connection",
		},
		{
			name:         "connection reset",
			err:          errors.New("connection reset by peer"),
			expectedCode: "connection",
		},
		{
			name:         "unknown error",
			err:          errors.New("some unexpected error"),
			expectedCode: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateError(tt.err, "http://localhost:11434")

			if tt.err == nil {
				if result != nil {
					t.Errorf("translateError(nil) = %v, want nil", result)
				}
				return
			}

			ollamaErr, ok := result.(*OllamaError)
			if !ok {
				t.Fatalf("translateError() did not return *OllamaError, got %T", result)
			}

			if ollamaErr.Code != tt.expectedCode {
				t.Errorf("Code = %q, want %q", ollamaErr.Code, tt.expectedCode)
			}

			// Verify original error is wrapped
			if !errors.Is(ollamaErr, tt.err) {
				t.Error("errors.Is(ollamaErr, originalErr) = false, want true")
			}
		})
	}
}

func TestTranslateError_EndpointExtraction(t *testing.T) {
	// Test that connection errors with HTTP URLs extract the endpoint
	err := errors.New("Post http://localhost:11434/api/chat: connection refused")
	result := translateError(err, "http://localhost:11434")

	ollamaErr, ok := result.(*OllamaError)
	if !ok {
		t.Fatalf("translateError() did not return *OllamaError")
	}

	if ollamaErr.Code != "connection" {
		t.Errorf("Code = %q, want %q", ollamaErr.Code, "connection")
	}

	// Should contain the extracted endpoint or generic message
	if !strings.Contains(ollamaErr.Message, "http://") {
		t.Errorf("Message should contain endpoint URL, got %q", ollamaErr.Message)
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that error wrapping works correctly with errors.Is and errors.As
	originalErr := errors.New("network error")
	connectionErr := newConnectionError("http://localhost:11434", originalErr)

	// Test errors.Is
	if !errors.Is(connectionErr, originalErr) {
		t.Error("errors.Is(connectionErr, originalErr) = false, want true")
	}

	// Test errors.As
	var ollamaErr *OllamaError
	if !errors.As(connectionErr, &ollamaErr) {
		t.Error("errors.As(connectionErr, &ollamaErr) = false, want true")
	}

	if ollamaErr.Code != "connection" {
		t.Errorf("Code = %q, want %q", ollamaErr.Code, "connection")
	}
}
