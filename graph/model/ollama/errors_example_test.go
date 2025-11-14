package ollama_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/dshills/langgraph-go/graph/model/ollama"
)

// Example demonstrating how to handle OllamaError types programmatically.
func ExampleOllamaError_handling() {
	// Simulate an error from the adapter
	err := simulateOllamaRequest()

	// Check if it's an OllamaError and handle accordingly
	var ollamaErr *ollama.OllamaError
	if errors.As(err, &ollamaErr) {
		switch ollamaErr.Code {
		case "connection":
			fmt.Println("Connection failed. Run: ollama serve")
		case "model_not_found":
			fmt.Println("Model not found. Pull it first.")
		case "timeout":
			fmt.Println("Request timed out. Increase timeout.")
		case "invalid_request":
			fmt.Println("Invalid request. Check parameters.")
		default:
			fmt.Println("Unknown error occurred.")
		}
	}

	// Output:
	// Request timed out. Increase timeout.
}

// Example showing error unwrapping with errors.Is
func ExampleOllamaError_unwrap() {
	// Create a timeout error
	err := createTimeoutError()

	// Check if the underlying error is context.DeadlineExceeded
	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("Context deadline exceeded")
	}

	// Output:
	// Context deadline exceeded
}

// Example demonstrating actionable error messages
func ExampleOllamaError_messages() {
	// Simulate different error scenarios
	errors := []error{
		simulateConnectionError(),
		simulateModelNotFoundError(),
		simulateInvalidRequestError(),
	}

	for _, err := range errors {
		fmt.Println(err.Error())
	}

	// Output:
	// ollama error [connection]: Failed to connect to http://localhost:11434. Ensure Ollama is running with: ollama serve
	// ollama error [model_not_found]: Model 'llama3.2' not found. Pull it with: ollama pull llama3.2
	// ollama error [invalid_request]: Invalid request: temperature must be between 0 and 2
}

// simulateOllamaRequest simulates an Ollama request that times out
func simulateOllamaRequest() error {
	// In real code, this would be an actual API call
	return &ollama.OllamaError{
		Code:    "timeout",
		Message: "Request timed out. Consider increasing the context timeout or using a faster model.",
		Err:     context.DeadlineExceeded,
	}
}

// createTimeoutError creates a timeout error for unwrapping example
func createTimeoutError() error {
	return &ollama.OllamaError{
		Code:    "timeout",
		Message: "Request timed out",
		Err:     context.DeadlineExceeded,
	}
}

// simulateConnectionError creates a connection error
func simulateConnectionError() error {
	return &ollama.OllamaError{
		Code:    "connection",
		Message: "Failed to connect to http://localhost:11434. Ensure Ollama is running with: ollama serve",
		Err:     errors.New("connection refused"),
	}
}

// simulateModelNotFoundError creates a model not found error
func simulateModelNotFoundError() error {
	return &ollama.OllamaError{
		Code:    "model_not_found",
		Message: "Model 'llama3.2' not found. Pull it with: ollama pull llama3.2",
		Err:     nil,
	}
}

// simulateInvalidRequestError creates an invalid request error
func simulateInvalidRequestError() error {
	return &ollama.OllamaError{
		Code:    "invalid_request",
		Message: "Invalid request: temperature must be between 0 and 2",
		Err:     errors.New("validation failed"),
	}
}

// Example showing how to use OllamaError in production code
func ExampleOllamaError_production() {
	// In a real application, you might retry connection errors
	err := attemptOllamaCall()

	var ollamaErr *ollama.OllamaError
	if errors.As(err, &ollamaErr) {
		if ollamaErr.Code == "connection" {
			log.Println("Retrying after connection failure...")
			// Implement retry logic here
		} else {
			log.Printf("Non-retryable error: %s", ollamaErr.Code)
		}
	}
}

func attemptOllamaCall() error {
	// Simulate a connection failure
	return &ollama.OllamaError{
		Code:    "connection",
		Message: "Failed to connect to http://localhost:11434. Ensure Ollama is running with: ollama serve",
		Err:     errors.New("connection refused"),
	}
}
