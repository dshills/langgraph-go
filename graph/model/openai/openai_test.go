package openai

import (
	"context"
	"errors"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// TestOpenAIChatModel_Construction verifies model creation (T135).
func TestOpenAIChatModel_Construction(t *testing.T) {
	t.Run("creates model with API key", func(t *testing.T) {
		m := NewChatModel("test-api-key", "gpt-4")

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

// TestOpenAIChatModel_Chat verifies basic chat functionality (T135).
func TestOpenAIChatModel_Chat(t *testing.T) {
	t.Run("sends messages and returns response", func(t *testing.T) {
		// Use mock client for testing
		mockClient := &mockOpenAIClient{
			response: "Hello! How can I help you?",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
		}

		messages := []model.Message{
			{Role: model.RoleSystem, Content: "You are helpful."},
			{Role: model.RoleUser, Content: "Hi there!"},
		}

		out, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Hello! How can I help you?" {
			t.Errorf("expected specific text, got %q", out.Text)
		}

		// Verify mock was called
		if mockClient.callCount != 1 {
			t.Errorf("expected 1 API call, got %d", mockClient.callCount)
		}
	})

	t.Run("handles tool calls in response", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			toolCalls: []model.ToolCall{
				{Name: "search", Input: map[string]interface{}{"query": "test"}},
			},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
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
		mockClient := &mockOpenAIClient{
			response: "Response",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
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

// TestOpenAIChatModel_ErrorHandling verifies error scenarios (T137).
func TestOpenAIChatModel_ErrorHandling(t *testing.T) {
	t.Run("handles API errors", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			err: errors.New("API error: invalid request"),
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("handles rate limit errors", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			err: &rateLimitError{message: "rate limit exceeded"},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected rate limit error, got nil")
		}

		var rateLimitErr *rateLimitError
		if !errors.As(err, &rateLimitErr) {
			t.Errorf("expected rateLimitError type, got %T", err)
		}
	})

	t.Run("handles empty API key", func(t *testing.T) {
		m := NewChatModel("", "gpt-4")

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Error("expected error for empty API key")
		}
	})
}

// TestOpenAIChatModel_RetryLogic verifies retry behavior (T137, T138).
func TestOpenAIChatModel_RetryLogic(t *testing.T) {
	t.Run("retries on transient errors", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			// Fail twice, then succeed
			errors: []error{
				errors.New("temporary network error"),
				errors.New("timeout"),
				nil,
			},
			response: "Success after retries",
		}

		m := &ChatModel{
			client:     mockClient,
			modelName:  "gpt-4",
			maxRetries: 3,
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		out, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected success after retries, got %v", err)
		}

		if out.Text != "Success after retries" {
			t.Errorf("expected success response, got %q", out.Text)
		}

		if mockClient.callCount != 3 {
			t.Errorf("expected 3 attempts (2 retries), got %d", mockClient.callCount)
		}
	})

	t.Run("does not retry on non-transient errors", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			err: errors.New("invalid API key"),
		}

		m := &ChatModel{
			client:     mockClient,
			modelName:  "gpt-4",
			maxRetries: 3,
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Should only try once for non-transient errors
		if mockClient.callCount != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", mockClient.callCount)
		}
	})

	t.Run("respects max retries limit", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			err: &rateLimitError{message: "rate limit"},
		}

		m := &ChatModel{
			client:     mockClient,
			modelName:  "gpt-4",
			maxRetries: 2,
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error after max retries, got nil")
		}

		// Initial attempt + 2 retries = 3 total
		if mockClient.callCount != 3 {
			t.Errorf("expected 3 attempts, got %d", mockClient.callCount)
		}
	})
}

// TestOpenAIChatModel_MessageConversion verifies message format conversion (T135).
func TestOpenAIChatModel_MessageConversion(t *testing.T) {
	t.Run("converts all message types", func(t *testing.T) {
		mockClient := &mockOpenAIClient{
			response: "Converted successfully",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gpt-4",
		}

		messages := []model.Message{
			{Role: model.RoleSystem, Content: "System prompt"},
			{Role: model.RoleUser, Content: "User message"},
			{Role: model.RoleAssistant, Content: "Assistant response"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify all messages were passed to client
		if len(mockClient.lastMessages) != 3 {
			t.Errorf("expected 3 messages sent, got %d", len(mockClient.lastMessages))
		}
	})
}

// Mock OpenAI client for testing.
type mockOpenAIClient struct {
	response     string
	toolCalls    []model.ToolCall
	err          error
	errors       []error // For testing retry logic
	callCount    int
	lastMessages []model.Message
}

func (m *mockOpenAIClient) createChatCompletion(_ context.Context, messages []model.Message, _ []model.ToolSpec) (model.ChatOut, error) {
	m.callCount++
	m.lastMessages = messages

	// Handle retry testing with multiple errors
	if len(m.errors) > 0 {
		if m.callCount <= len(m.errors) {
			err := m.errors[m.callCount-1]
			if err != nil {
				return model.ChatOut{}, err
			}
		}
	} else if m.err != nil {
		return model.ChatOut{}, m.err
	}

	return model.ChatOut{
		Text:      m.response,
		ToolCalls: m.toolCalls,
	}, nil
}

// rateLimitError is imported from the openai package
