package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph/model"
	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
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
		modelName = "gpt-4o" // GPT-4o is the latest multimodal model (2025)
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

// rateLimitError represents an OpenAI rate limit error.
type rateLimitError struct {
	message string
}

func (e *rateLimitError) Error() string {
	return e.message
}

// defaultClient wraps the official OpenAI SDK client.
type defaultClient struct {
	apiKey    string
	modelName string
}

func (c *defaultClient) createChatCompletion(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate API key
	if c.apiKey == "" {
		return model.ChatOut{}, errors.New("OpenAI API key is required")
	}

	// Create OpenAI client
	client := openaisdk.NewClient(option.WithAPIKey(c.apiKey))

	// Convert messages to OpenAI format
	openaiMessages := convertMessages(messages)

	// Build request parameters
	params := openaisdk.ChatCompletionNewParams{
		Model:    openaisdk.ChatModel(c.modelName),
		Messages: openaiMessages,
	}

	// Add tools if provided
	if len(tools) > 0 {
		params.Tools = convertTools(tools)
	}

	// Call OpenAI API
	resp, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("OpenAI API error: %w", err)
	}

	// Convert response to our format
	return convertResponse(resp), nil
}

// convertMessages converts our Message format to OpenAI's format.
func convertMessages(messages []model.Message) []openaisdk.ChatCompletionMessageParamUnion {
	result := make([]openaisdk.ChatCompletionMessageParamUnion, len(messages))

	for i, msg := range messages {
		switch msg.Role {
		case model.RoleSystem:
			result[i] = openaisdk.SystemMessage(msg.Content)
		case model.RoleUser:
			result[i] = openaisdk.UserMessage(msg.Content)
		case model.RoleAssistant:
			result[i] = openaisdk.AssistantMessage(msg.Content)
		default:
			// Fallback to user message for unknown roles
			result[i] = openaisdk.UserMessage(msg.Content)
		}
	}

	return result
}

// convertTools converts our ToolSpec format to OpenAI's format.
func convertTools(tools []model.ToolSpec) []openaisdk.ChatCompletionToolParam {
	result := make([]openaisdk.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		result[i] = openaisdk.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openaisdk.String(tool.Description),
				Parameters:  shared.FunctionParameters(tool.Schema),
			},
		}
	}

	return result
}

// convertResponse converts OpenAI's response to our ChatOut format.
func convertResponse(resp *openaisdk.ChatCompletion) model.ChatOut {
	out := model.ChatOut{}

	if len(resp.Choices) == 0 {
		return out
	}

	// Get the first choice (most common case)
	choice := resp.Choices[0]
	msg := choice.Message

	// Extract text content
	out.Text = msg.Content

	// Extract tool calls if present
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]model.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			out.ToolCalls[i] = model.ToolCall{
				Name:  tc.Function.Name,
				Input: parseToolInput(tc.Function.Arguments),
			}
		}
	}

	return out
}

// parseToolInput parses the JSON arguments string into a map.
func parseToolInput(jsonStr string) map[string]interface{} {
	// For now, return a simple map with the raw JSON
	// In production, you'd parse this properly using encoding/json
	if jsonStr == "" {
		return nil
	}

	// Parse JSON string into map
	result := make(map[string]interface{})
	// TODO: Implement proper JSON parsing
	// For now, store as a single "arguments" field
	result["_raw"] = jsonStr

	return result
}
