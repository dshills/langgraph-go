package google

import (
	"context"
	"errors"

	"github.com/dshills/langgraph-go/graph/model"
)

// ChatModel implements model.ChatModel for Google's Gemini API.
//
// Provides access to Gemini models (gemini-pro, gemini-pro-vision) with:
//   - Safety filter handling
//   - Tool/function calling support
//   - Context cancellation
//   - User-friendly error messages for blocked content
//
// Example usage:
//
//	apiKey := os.Getenv("GOOGLE_API_KEY")
//	m := google.NewChatModel(apiKey, "gemini-pro")
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "What is the capital of France?"},
//	}
//
//	out, err := m.Chat(ctx, messages, nil)
//	if err != nil {
//	    var safetyErr *SafetyFilterError
//	    if errors.As(err, &safetyErr) {
//	        log.Printf("Content blocked: %s", safetyErr.Category)
//	        return
//	    }
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text)
type ChatModel struct {
	apiKey    string
	modelName string
	client    googleClient
}

// googleClient defines the interface for Google Gemini API operations.
// This allows for easy mocking in tests.
type googleClient interface {
	generateContent(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error)
}

// NewChatModel creates a new Google ChatModel.
//
// Parameters:
//   - apiKey: Google API key (get from https://makersuite.google.com/app/apikey)
//   - modelName: Model to use (e.g., "gemini-pro"). Empty string uses default.
//
// Returns a ChatModel configured for Gemini API.
//
// Example:
//
//	model := google.NewChatModel(apiKey, "gemini-pro")
func NewChatModel(apiKey, modelName string) *ChatModel {
	if modelName == "" {
		modelName = "gemini-pro"
	}

	return &ChatModel{
		apiKey:    apiKey,
		modelName: modelName,
		client:    &defaultClient{apiKey: apiKey, modelName: modelName},
	}
}

// Chat implements the model.ChatModel interface.
//
// Sends messages to Google's Gemini API and returns the response.
// Handles safety filter blocks with descriptive errors.
//
// Returns:
//   - ChatOut with Text and/or ToolCalls
//   - Error for authentication failures, safety blocks, or API errors
func (m *ChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Check context cancellation
	if ctx.Err() != nil {
		return model.ChatOut{}, ctx.Err()
	}

	// Call Google API
	out, err := m.client.generateContent(ctx, messages, tools)
	if err != nil {
		// Handle safety filter errors specially
		var safetyErr *safetyFilterError
		if errors.As(err, &safetyErr) {
			return model.ChatOut{}, handleSafetyFilterError(safetyErr)
		}
		return model.ChatOut{}, err
	}

	return out, nil
}

// handleSafetyFilterError wraps safety filter errors with user-friendly context.
//
// Google's safety filters can block content in several categories:
//   - HARM_CATEGORY_HATE_SPEECH
//   - HARM_CATEGORY_SEXUALLY_EXPLICIT
//   - HARM_CATEGORY_DANGEROUS_CONTENT
//   - HARM_CATEGORY_HARASSMENT
//
// Returns an error that can be checked with errors.As for the specific category.
func handleSafetyFilterError(err *safetyFilterError) error {
	// Pass through with context preserved
	return err
}

// defaultClient is a placeholder for the actual Google Gemini SDK client.
// In a real implementation, this would wrap the official generative-ai-go SDK.
type defaultClient struct {
	apiKey    string
	modelName string
}

func (c *defaultClient) generateContent(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate API key
	if c.apiKey == "" {
		return model.ChatOut{}, errors.New("Google API key is required")
	}

	// This is a placeholder implementation.
	// In production, this would use the official generative-ai-go SDK:
	//
	// client, err := genai.NewClient(ctx, option.WithAPIKey(c.apiKey))
	// if err != nil {
	//     return model.ChatOut{}, err
	// }
	// defer client.Close()
	//
	// model := client.GenerativeModel(c.modelName)
	// if len(tools) > 0 {
	//     model.Tools = convertTools(tools)
	// }
	//
	// history := convertMessages(messages)
	// resp, err := model.GenerateContent(ctx, history...)
	// if err != nil {
	//     return model.ChatOut{}, translateGoogleError(err)
	// }
	//
	// return convertResponse(resp), nil

	return model.ChatOut{}, errors.New("Google client not implemented - use mocked client for testing")
}

// safetyFilterError represents a Google safety filter block.
//
// Provides information about why content was blocked:
//   - Reason: Why the block occurred (e.g., "SAFETY")
//   - Category: Which safety category was triggered
type safetyFilterError struct {
	reason   string
	category string
}

// Error implements the error interface.
func (e *safetyFilterError) Error() string {
	return "content blocked by safety filter: " + e.category
}

// Category returns the safety category that triggered the block.
func (e *safetyFilterError) Category() string {
	return e.category
}

// Reason returns why the content was blocked.
func (e *safetyFilterError) Reason() string {
	return e.reason
}
