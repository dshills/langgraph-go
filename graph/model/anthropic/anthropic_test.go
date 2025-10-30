package anthropic

import (
	"context"
	"errors"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// TestAnthropicChatModel_Construction verifies model creation (T140).
func TestAnthropicChatModel_Construction(t *testing.T) {
	t.Run("creates model with API key", func(t *testing.T) {
		m := NewChatModel("test-api-key", "claude-3-opus-20240229")

		if m == nil {
			t.Fatal("expected non-nil model")
		}
	})

	t.Run("creates model with default model name", func(t *testing.T) {
		m := NewChatModel("test-api-key", "")

		if m == nil {
			t.Fatal("expected non-nil model")
		}
	})
}

// TestAnthropicChatModel_Chat verifies basic chat functionality (T140).
func TestAnthropicChatModel_Chat(t *testing.T) {
	t.Run("sends messages and returns response", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			response: "Hello! I'm Claude, an AI assistant.",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Hi there!"},
		}

		out, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Hello! I'm Claude, an AI assistant." {
			t.Errorf("expected specific text, got %q", out.Text)
		}

		if mockClient.callCount != 1 {
			t.Errorf("expected 1 API call, got %d", mockClient.callCount)
		}
	})

	t.Run("handles tool calls in response", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			toolCalls: []model.ToolCall{
				{Name: "search", Input: map[string]interface{}{"query": "test"}},
			},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Search for test"},
		}
		tools := []model.ToolSpec{
			{Name: "search", Description: "Search the web"},
		}

		out, err := m.Chat(context.Background(), messages, tools)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(out.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(out.ToolCalls))
		}

		if out.ToolCalls[0].Name != "search" {
			t.Errorf("expected tool name 'search', got %q", out.ToolCalls[0].Name)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			response: "Response",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(ctx, messages, nil)
		if err == nil {
			t.Fatal("expected context.Canceled error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}

// TestAnthropicChatModel_ErrorHandling verifies error scenarios (T142).
func TestAnthropicChatModel_ErrorHandling(t *testing.T) {
	t.Run("handles API errors", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			err: errors.New("API error: invalid request"),
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("translates Anthropic errors to common format", func(t *testing.T) {
		anthropicErr := &anthropicError{
			Type:    "overloaded_error",
			Message: "Service temporarily overloaded",
		}

		mockClient := &mockAnthropicClient{
			err: anthropicErr,
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Verify error was translated
		var translatedErr *anthropicError
		if !errors.As(err, &translatedErr) {
			t.Errorf("expected anthropicError type, got %T", err)
		}

		if translatedErr.Type != "overloaded_error" {
			t.Errorf("expected type 'overloaded_error', got %q", translatedErr.Type)
		}
	})

	t.Run("handles rate limit errors", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			err: &anthropicError{
				Type:    "rate_limit_error",
				Message: "Rate limit exceeded",
			},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var anthropicErr *anthropicError
		if !errors.As(err, &anthropicErr) {
			t.Errorf("expected anthropicError type, got %T", err)
		}

		if anthropicErr.Type != "rate_limit_error" {
			t.Errorf("expected type 'rate_limit_error', got %q", anthropicErr.Type)
		}
	})

	t.Run("handles empty API key", func(t *testing.T) {
		m := NewChatModel("", "claude-3-opus-20240229")

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Error("expected error for empty API key")
		}
	})
}

// TestAnthropicChatModel_ErrorTranslation verifies error mapping (T143).
func TestAnthropicChatModel_ErrorTranslation(t *testing.T) {
	t.Run("translates overloaded_error", func(t *testing.T) {
		err := &anthropicError{
			Type:    "overloaded_error",
			Message: "Service overloaded",
		}

		translated := translateAnthropicError(err)

		var translatedErr *anthropicError
		if !errors.As(translated, &translatedErr) {
			t.Fatalf("expected anthropicError type, got %T", translated)
		}

		if translatedErr.Type != "overloaded_error" {
			t.Errorf("expected preserved type, got %q", translatedErr.Type)
		}
	})

	t.Run("translates authentication_error", func(t *testing.T) {
		err := &anthropicError{
			Type:    "authentication_error",
			Message: "Invalid API key",
		}

		translated := translateAnthropicError(err)

		var translatedErr *anthropicError
		if !errors.As(translated, &translatedErr) {
			t.Fatalf("expected anthropicError type, got %T", translated)
		}

		if translatedErr.Type != "authentication_error" {
			t.Errorf("expected preserved type, got %q", translatedErr.Type)
		}
	})

	t.Run("preserves unknown error types", func(t *testing.T) {
		err := &anthropicError{
			Type:    "unknown_error",
			Message: "Something went wrong",
		}

		translated := translateAnthropicError(err)

		var translatedErr *anthropicError
		if !errors.As(translated, &translatedErr) {
			t.Fatalf("expected anthropicError type, got %T", translated)
		}

		if translatedErr.Type != "unknown_error" {
			t.Errorf("expected preserved type, got %q", translatedErr.Type)
		}
	})
}

// TestAnthropicChatModel_MessageConversion verifies message format (T140).
func TestAnthropicChatModel_MessageConversion(t *testing.T) {
	t.Run("converts messages to Anthropic format", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			response: "Converted successfully",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "User message"},
			{Role: model.RoleAssistant, Content: "Assistant response"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(mockClient.lastMessages) != 2 {
			t.Errorf("expected 2 messages sent, got %d", len(mockClient.lastMessages))
		}
	})

	t.Run("extracts system message separately", func(t *testing.T) {
		mockClient := &mockAnthropicClient{
			response: "System extracted",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "claude-3-opus-20240229",
		}

		messages := []model.Message{
			{Role: model.RoleSystem, Content: "You are helpful"},
			{Role: model.RoleUser, Content: "User message"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// System message should be extracted
		if mockClient.systemPrompt != "You are helpful" {
			t.Errorf("expected system prompt extracted, got %q", mockClient.systemPrompt)
		}

		// Only user message should remain
		if len(mockClient.lastMessages) != 1 {
			t.Errorf("expected 1 message (user), got %d", len(mockClient.lastMessages))
		}
	})
}

// Mock Anthropic client for testing.
type mockAnthropicClient struct {
	response     string
	toolCalls    []model.ToolCall
	err          error
	callCount    int
	lastMessages []model.Message
	systemPrompt string
}

func (m *mockAnthropicClient) createMessage(_ context.Context, systemPrompt string, messages []model.Message, _ []model.ToolSpec) (model.ChatOut, error) {
	m.callCount++
	m.lastMessages = messages
	m.systemPrompt = systemPrompt

	if m.err != nil {
		return model.ChatOut{}, m.err
	}

	return model.ChatOut{
		Text:      m.response,
		ToolCalls: m.toolCalls,
	}, nil
}

// anthropicError is imported from the anthropic package
