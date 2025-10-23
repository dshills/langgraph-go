package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph/model"
)

// ChatModel implements model.ChatModel for OpenAI's API.
//
// Provides access to OpenAI models (GPT-4, GPT-3.5, etc.) with:
//   - Automatic retry logic for transient errors
//   - Rate limit handling
//   - Tool/function calling support
//   - Context cancellation
//
// Example usage:
//
//	apiKey := os.Getenv("OPENAI_API_KEY")
//	m := openai.NewChatModel(apiKey, "gpt-4")
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "What is the capital of France?"},
//	}
//
//	out, err := m.Chat(ctx, messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text)
type ChatModel struct {
	apiKey     string
	modelName  string
	client     openaiClient
	maxRetries int
	retryDelay time.Duration
}

// openaiClient defines the interface for OpenAI API operations.
// This allows for easy mocking in tests.
type openaiClient interface {
	createChatCompletion(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error)
}

// NewChatModel creates a new OpenAI ChatModel.
//
// Parameters:
//   - apiKey: OpenAI API key (get from https://platform.openai.com/api-keys)
//   - modelName: Model to use (e.g., "gpt-4", "gpt-3.5-turbo"). Empty string uses default.
//
// Returns a ChatModel configured with:
//   - 3 retry attempts for transient errors
//   - 1 second delay between retries
//   - Exponential backoff for rate limits
//
// Example:
//
//	model := openai.NewChatModel(apiKey, "gpt-4")
func NewChatModel(apiKey, modelName string) *ChatModel {
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
	}

	return &ChatModel{
		apiKey:     apiKey,
		modelName:  modelName,
		client:     &defaultClient{apiKey: apiKey, modelName: modelName},
		maxRetries: 3,
		retryDelay: time.Second,
	}
}

// Chat implements the model.ChatModel interface.
//
// Sends messages to OpenAI's API and returns the response.
// Automatically retries on transient errors (network issues, rate limits).
//
// Returns:
//   - ChatOut with Text and/or ToolCalls
//   - Error for authentication failures, invalid requests, or exceeded retries
func (m *ChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Check context cancellation
	if ctx.Err() != nil {
		return model.ChatOut{}, ctx.Err()
	}

	// Attempt with retries
	var lastErr error
	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		out, err := m.client.createChatCompletion(ctx, messages, tools)
		if err == nil {
			return out, nil
		}

		lastErr = err

		// Don't retry on non-transient errors
		if !isTransientError(err) {
			return model.ChatOut{}, err
		}

		// Don't retry if we've exhausted attempts
		if attempt >= m.maxRetries {
			break
		}

		// Wait before retry (with exponential backoff for rate limits)
		delay := m.retryDelay
		if isRateLimitError(err) {
			delay = m.retryDelay * time.Duration(attempt+1)
		}

		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return model.ChatOut{}, ctx.Err()
		}
	}

	return model.ChatOut{}, fmt.Errorf("OpenAI API failed after %d retries: %w", m.maxRetries, lastErr)
}

// isTransientError determines if an error should trigger a retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Rate limit errors are transient
	var rateLimitErr *rateLimitError
	if errors.As(err, &rateLimitErr) {
		return true
	}

	// Check for common transient error patterns
	msgLower := strings.ToLower(err.Error())
	transientPatterns := []string{
		"timeout",
		"network",
		"connection",
		"temporary",
		"503",
		"502",
		"500",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(msgLower, pattern) {
			return true
		}
	}

	return false
}

// isRateLimitError checks if error is a rate limit error.
func isRateLimitError(err error) bool {
	var rateLimitErr *rateLimitError
	return errors.As(err, &rateLimitErr)
}

// defaultClient is a placeholder for the actual OpenAI SDK client.
// In a real implementation, this would wrap the official openai-go SDK.
type defaultClient struct {
	apiKey    string
	modelName string
}

func (c *defaultClient) createChatCompletion(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate API key
	if c.apiKey == "" {
		return model.ChatOut{}, errors.New("OpenAI API key is required")
	}

	// This is a placeholder implementation.
	// In production, this would use the official openai-go SDK:
	//
	// client := openai.NewClient(c.apiKey)
	// req := openai.ChatCompletionRequest{
	//     Model:    c.modelName,
	//     Messages: convertMessages(messages),
	//     Tools:    convertTools(tools),
	// }
	// resp, err := client.CreateChatCompletion(ctx, req)
	// return convertResponse(resp), err

	return model.ChatOut{}, errors.New("OpenAI client not implemented - use mocked client for testing")
}
