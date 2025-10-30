// Package anthropic provides ChatModel adapter for Anthropic Claude API.
package anthropic

import (
	"context"
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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
		modelName = "claude-sonnet-4-5-20250929" // Claude Sonnet 4.5 (latest as of Sept 2025)
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

// defaultClient wraps the official Anthropic SDK client.
type defaultClient struct {
	apiKey    string
	modelName string
}

func (c *defaultClient) createMessage(ctx context.Context, systemPrompt string, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate API key
	if c.apiKey == "" {
		return model.ChatOut{}, errors.New("anthropic API key is required")
	}

	// Create Anthropic client
	client := anthropicsdk.NewClient(option.WithAPIKey(c.apiKey))

	// Convert messages to Anthropic format
	anthropicMessages := convertMessages(messages)

	// Build request parameters
	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(c.modelName),
		Messages:  anthropicMessages,
		MaxTokens: 4096, // Default max tokens
	}

	// Add system prompt if provided
	if systemPrompt != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	// Add tools if provided
	if len(tools) > 0 {
		params.Tools = convertTools(tools)
	}

	// Call Anthropic API
	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("anthropic API error: %w", err)
	}

	// Convert response to our format (resp is already a pointer)
	return convertResponse(resp), nil
}

// convertMessages converts our Message format to Anthropic's format.
func convertMessages(messages []model.Message) []anthropicsdk.MessageParam {
	result := make([]anthropicsdk.MessageParam, len(messages))

	for i, msg := range messages {
		switch msg.Role {
		case model.RoleUser:
			result[i] = anthropicsdk.NewUserMessage(anthropicsdk.NewTextBlock(msg.Content))
		case model.RoleAssistant:
			result[i] = anthropicsdk.NewAssistantMessage(anthropicsdk.NewTextBlock(msg.Content))
		default:
			// Fallback to user message for unknown roles (system is handled separately)
			result[i] = anthropicsdk.NewUserMessage(anthropicsdk.NewTextBlock(msg.Content))
		}
	}

	return result
}

// convertTools converts our ToolSpec format to Anthropic's format.
func convertTools(tools []model.ToolSpec) []anthropicsdk.ToolUnionParam {
	result := make([]anthropicsdk.ToolUnionParam, len(tools))

	for i, tool := range tools {
		// Extract properties and required from schema
		var properties any
		var required []string
		if tool.Schema != nil {
			if props, ok := tool.Schema["properties"]; ok {
				properties = props
			}
			if req, ok := tool.Schema["required"].([]string); ok {
				required = req
			} else if req, ok := tool.Schema["required"].([]interface{}); ok {
				// Convert []interface{} to []string
				required = make([]string, len(req))
				for j, v := range req {
					if s, ok := v.(string); ok {
						required[j] = s
					}
				}
			}
		}

		inputSchema := anthropicsdk.ToolInputSchemaParam{
			Properties: properties,
			Required:   required,
		}

		result[i] = anthropicsdk.ToolUnionParam{
			OfTool: &anthropicsdk.ToolParam{
				Name:        tool.Name,
				Description: anthropicsdk.String(tool.Description),
				InputSchema: inputSchema,
			},
		}
	}

	return result
}

// convertResponse converts Anthropic's response to our ChatOut format.
func convertResponse(resp *anthropicsdk.Message) model.ChatOut {
	out := model.ChatOut{}

	// Extract content from response
	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropicsdk.TextBlock:
			// Append text content
			if out.Text != "" {
				out.Text += "\n"
			}
			out.Text += b.Text

		case anthropicsdk.ToolUseBlock:
			// Extract tool calls
			out.ToolCalls = append(out.ToolCalls, model.ToolCall{
				Name:  b.Name,
				Input: convertToolInput(b.Input),
			})
		}
	}

	return out
}

// convertToolInput converts Anthropic's tool input to our format.
func convertToolInput(input interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}

	// If it's already a map, return it directly
	if m, ok := input.(map[string]interface{}); ok {
		return m
	}

	// Otherwise wrap it
	return map[string]interface{}{
		"_raw": input,
	}
}

// anthropicError represents an Anthropic API error.
type anthropicError struct {
	Type    string
	Message string
}

func (e *anthropicError) Error() string {
	return e.Type + ": " + e.Message
}
