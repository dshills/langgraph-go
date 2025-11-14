package ollama

import (
	"fmt"
	"strings"
)

// OllamaError wraps Ollama-specific errors with actionable user guidance.
//
// Provides error categorization and helpful messages to guide users toward
// resolution. Used throughout the adapter to translate low-level errors
// (network, HTTP, JSON parsing) into domain-specific errors with context.
//
// Example usage:
//
//	out, err := adapter.Chat(ctx, messages, nil)
//	if err != nil {
//	    var ollamaErr *OllamaError
//	    if errors.As(err, &ollamaErr) {
//	        switch ollamaErr.Code {
//	        case "connection":
//	            log.Println("Ensure Ollama is running: ollama serve")
//	        case "model_not_found":
//	            log.Printf("Pull model: ollama pull %s", modelName)
//	        case "timeout":
//	            log.Println("Request exceeded deadline, try increasing timeout")
//	        }
//	    }
//	}
type OllamaError struct {
	// Code categorizes the error type for programmatic handling.
	//
	// Standard codes:
	// - "connection": Failed to connect to Ollama endpoint
	// - "model_not_found": Requested model not available locally
	// - "invalid_request": Malformed request or unsupported operation
	// - "timeout": Request exceeded context deadline
	// - "unknown": Unexpected error without specific category
	Code string

	// Message provides a human-readable error description with actionable guidance.
	//
	// Messages include:
	// - What went wrong
	// - How to fix it (e.g., "ollama pull model-name")
	// - Context-specific details (model name, endpoint, etc.)
	Message string

	// Err is the wrapped original error for unwrapping with errors.Is/errors.As.
	//
	// Allows callers to inspect the underlying error:
	//   errors.Is(err, context.DeadlineExceeded)
	//   errors.As(err, &netErr)
	Err error
}

// Error implements the error interface.
//
// Returns a formatted error message combining the code and message:
//
//	"ollama error [connection]: Failed to connect to http://localhost:11434. Ensure Ollama is running with: ollama serve"
func (e *OllamaError) Error() string {
	return fmt.Sprintf("ollama error [%s]: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped original error for errors.Is/errors.As compatibility.
//
// Enables error chain inspection:
//
//	if errors.Is(err, context.DeadlineExceeded) {
//	    // Handle timeout specifically
//	}
//
//	var netErr *net.OpError
//	if errors.As(err, &netErr) {
//	    // Handle network error
//	}
func (e *OllamaError) Unwrap() error {
	return e.Err
}

// newConnectionError creates an error for Ollama connection failures.
//
// Used when:
// - Ollama service is not running
// - Network connection refused
// - Invalid endpoint URL
// - DNS resolution failures
//
// The error message guides users to start Ollama:
//
//	"Failed to connect to http://localhost:11434. Ensure Ollama is running with: ollama serve"
//
// Parameters:
//   - endpoint: The Ollama API endpoint URL that failed to connect
//   - err: The underlying connection error (wrapped for inspection)
//
// Example:
//
//	resp, err := http.Post(endpoint, "application/json", body)
//	if err != nil {
//	    return newConnectionError(endpoint, err)
//	}
func newConnectionError(endpoint string, err error) *OllamaError {
	return &OllamaError{
		Code:    "connection",
		Message: fmt.Sprintf("Failed to connect to %s. Ensure Ollama is running with: ollama serve", endpoint),
		Err:     err,
	}
}

// newModelNotFoundError creates an error for unavailable models.
//
// Used when:
// - Model not pulled locally (HTTP 404 response)
// - Typo in model name
// - Model deleted from local cache
//
// The error message guides users to pull the model:
//
//	"Model 'llama3.2' not found. Pull it with: ollama pull llama3.2"
//
// Parameters:
//   - model: The requested model name that was not found
//
// Example:
//
//	if resp.StatusCode == http.StatusNotFound {
//	    return newModelNotFoundError(config.Model)
//	}
func newModelNotFoundError(model string) *OllamaError {
	return &OllamaError{
		Code:    "model_not_found",
		Message: fmt.Sprintf("Model '%s' not found. Pull it with: ollama pull %s", model, model),
		Err:     nil, // No underlying error to wrap
	}
}

// newInvalidRequestError creates an error for malformed requests.
//
// Used when:
// - Invalid JSON in request body
// - Unsupported operation (e.g., streaming not supported by model)
// - Missing required fields
// - HTTP 400 Bad Request responses
//
// The error message includes specific validation failure details:
//
//	"Invalid request: temperature must be between 0 and 2"
//
// Parameters:
//   - details: Specific validation failure message from Ollama API
//   - err: The underlying error if available (e.g., JSON parsing error)
//
// Example:
//
//	if resp.StatusCode == http.StatusBadRequest {
//	    body, _ := io.ReadAll(resp.Body)
//	    return newInvalidRequestError(string(body), nil)
//	}
func newInvalidRequestError(details string, err error) *OllamaError {
	return &OllamaError{
		Code:    "invalid_request",
		Message: fmt.Sprintf("Invalid request: %s", details),
		Err:     err,
	}
}

// newTimeoutError creates an error for request timeouts.
//
// Used when:
// - Context deadline exceeded
// - HTTP client timeout
// - Model inference takes too long
//
// The error message guides users to adjust timeout configuration:
//
//	"Request timed out. Consider increasing the context timeout or using a faster model."
//
// Parameters:
//   - err: The underlying timeout error (context.DeadlineExceeded or similar)
//
// Example:
//
//	out, err := adapter.Chat(ctx, messages, nil)
//	if errors.Is(err, context.DeadlineExceeded) {
//	    return newTimeoutError(err)
//	}
func newTimeoutError(err error) *OllamaError {
	return &OllamaError{
		Code:    "timeout",
		Message: "Request timed out. Consider increasing the context timeout or using a faster model.",
		Err:     err,
	}
}

// translateError converts generic errors to OllamaError with appropriate categorization.
//
// Examines the error to determine the failure category:
//
// - context.DeadlineExceeded → timeout error
// - "connection refused" → connection error
// - "no such host" → connection error (DNS failure)
// - All other errors → unknown error
//
// Use this for error translation when the specific error category is not known upfront:
//
//	resp, err := http.Post(endpoint, "application/json", body)
//	if err != nil {
//	    return translateError(err, config.Endpoint)
//	}
//
// Parameters:
//   - err: The error to translate (may be nil, returns nil if so)
//   - endpoint: The Ollama endpoint URL for error messages
//
// Returns:
//   - *OllamaError with appropriate Code and Message
//   - nil if err is nil
//
// Example:
//
//	if err := json.Unmarshal(data, &response); err != nil {
//	    return newInvalidRequestError("failed to parse response", err)
//	}
//	// OR for generic translation:
//	if err != nil {
//	    return translateError(err, endpoint)
//	}
func translateError(err error, endpoint string) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	// Check for timeout errors
	if strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "timeout") {
		return newTimeoutError(err)
	}

	// Check for connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "connection reset") {
		return newConnectionError(endpoint, err)
	}

	// Unknown error category
	return &OllamaError{
		Code:    "unknown",
		Message: fmt.Sprintf("Unexpected error: %v", err),
		Err:     err,
	}
}
