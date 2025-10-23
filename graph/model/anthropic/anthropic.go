package anthropic

import (
	"context"
	"errors"

	"github.com/dshills/langgraph-go/graph/model"
)

// ChatModel implements model.ChatModel for Anthropic's Claude API.
//
// Provides access to Claude models (Claude 3 Opus, Sonnet, Haiku) with:
//   - Error translation to common format
//   - Tool/function calling support
//   - Context cancellation
//   - System prompt extraction (Anthropic uses separate system parameter)
//
// Example usage:
//
//	apiKey := os.Getenv("ANTHROPIC_API_KEY")
//	m := anthropic.NewChatModel(apiKey, "claude-3-opus-20240229")
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
	apiKey    string
	modelName string
	client    anthropicClient
}

// anthropicClient defines the interface for Anthropic API operations.
// This allows for easy mocking in tests.
type anthropicClient interface {
	createMessage(ctx context.Context, systemPrompt string, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error)
}

// NewChatModel creates a new Anthropic ChatModel.
//
// Parameters:
//   - apiKey: Anthropic API key (get from https://console.anthropic.com/)
//   - modelName: Model to use (e.g., "claude-3-opus-20240229"). Empty string uses default.
//
// Returns a ChatModel configured for Claude API.
//
// Example:
//
//	model := anthropic.NewChatModel(apiKey, "claude-3-opus-20240229")
func NewChatModel(apiKey, modelName string) *ChatModel {
	if modelName == "" {
		modelName = "claude-3-sonnet-20240229"
	}

	return &ChatModel{
		apiKey:    apiKey,
		modelName: modelName,
		client:    &defaultClient{apiKey: apiKey, modelName: modelName},
	}
}

// Chat implements the model.ChatModel interface.
//
// Sends messages to Anthropic's API and returns the response.
// Handles Anthropic-specific message format (system prompt extraction).
//
// Returns:
//   - ChatOut with Text and/or ToolCalls
//   - Error for authentication failures, invalid requests, or API errors
func (m *ChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Check context cancellation
	if ctx.Err() != nil {
		return model.ChatOut{}, ctx.Err()
	}

	// Extract system prompt (Anthropic uses separate system parameter)
	systemPrompt, conversationMessages := extractSystemPrompt(messages)

	// Call Anthropic API
	out, err := m.client.createMessage(ctx, systemPrompt, conversationMessages, tools)
	if err != nil {
		// Translate Anthropic errors to common format
		var anthropicErr *anthropicError
		if errors.As(err, &anthropicErr) {
			return model.ChatOut{}, translateAnthropicError(anthropicErr)
		}
		return model.ChatOut{}, err
	}

	return out, nil
}

// extractSystemPrompt separates the system message from conversation messages.
// Anthropic's API expects system prompts as a separate parameter, not in messages array.
func extractSystemPrompt(messages []model.Message) (string, []model.Message) {
	var systemPrompt string
	var conversationMessages []model.Message

	for _, msg := range messages {
		if msg.Role == model.RoleSystem {
			// Concatenate multiple system messages if present
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content
		} else {
			conversationMessages = append(conversationMessages, msg)
		}
	}

	return systemPrompt, conversationMessages
}

// translateAnthropicError converts Anthropic API errors to a common format.
//
// Anthropic error types:
//   - authentication_error: Invalid API key
//   - permission_error: Insufficient permissions
//   - not_found_error: Resource not found
//   - rate_limit_error: Rate limit exceeded
//   - overloaded_error: Service temporarily overloaded
//   - invalid_request_error: Invalid request parameters
//
// Returns the same error with preserved type information for client handling.
func translateAnthropicError(err *anthropicError) error {
	// For now, just pass through the error with type information
	// In production, you might map these to more specific error types
	return err
}

// defaultClient is a placeholder for the actual Anthropic SDK client.
// In a real implementation, this would wrap the official anthropic-sdk-go.
type defaultClient struct {
	apiKey    string
	modelName string
}

func (c *defaultClient) createMessage(ctx context.Context, systemPrompt string, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate API key
	if c.apiKey == "" {
		return model.ChatOut{}, errors.New("Anthropic API key is required")
	}

	// This is a placeholder implementation.
	// In production, this would use the official anthropic-sdk-go SDK:
	//
	// client := anthropic.NewClient(c.apiKey)
	// req := anthropic.MessageRequest{
	//     Model:    c.modelName,
	//     System:   systemPrompt,
	//     Messages: convertMessages(messages),
	//     Tools:    convertTools(tools),
	// }
	// resp, err := client.CreateMessage(ctx, req)
	// return convertResponse(resp), err

	return model.ChatOut{}, errors.New("Anthropic client not implemented - use mocked client for testing")
}

// anthropicError represents an Anthropic API error.
type anthropicError struct {
	Type    string
	Message string
}

func (e *anthropicError) Error() string {
	return e.Type + ": " + e.Message
}
