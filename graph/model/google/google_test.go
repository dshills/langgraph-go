package google

import (
	"context"
	"errors"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// TestGoogleChatModel_Construction verifies model creation (T145).
func TestGoogleChatModel_Construction(t *testing.T) {
	t.Run("creates model with API key", func(t *testing.T) {
		m := NewChatModel("test-api-key", "gemini-pro")

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

// TestGoogleChatModel_Chat verifies basic chat functionality (T145).
func TestGoogleChatModel_Chat(t *testing.T) {
	t.Run("sends messages and returns response", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			response: "Hello! I'm Gemini, a helpful AI assistant.",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Hi there!"},
		}

		out, err := m.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Hello! I'm Gemini, a helpful AI assistant." {
			t.Errorf("expected specific text, got %q", out.Text)
		}

		if mockClient.callCount != 1 {
			t.Errorf("expected 1 API call, got %d", mockClient.callCount)
		}
	})

	t.Run("handles tool calls in response", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			toolCalls: []model.ToolCall{
				{Name: "search", Input: map[string]interface{}{"query": "test"}},
			},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
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
		mockClient := &mockGoogleClient{
			response: "Response",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
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

// TestGoogleChatModel_SafetyFilters verifies safety filter handling (T147).
func TestGoogleChatModel_SafetyFilters(t *testing.T) {
	t.Run("handles blocked content", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			err: &SafetyFilterError{
				reason:   "SAFETY",
				category: "HARM_CATEGORY_DANGEROUS_CONTENT",
			},
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Dangerous content"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected safety filter error, got nil")
		}

		var safetyErr *SafetyFilterError
		if !errors.As(err, &safetyErr) {
			t.Errorf("expected SafetyFilterError type, got %T", err)
		}

		if safetyErr.Category() != "HARM_CATEGORY_DANGEROUS_CONTENT" {
			t.Errorf("expected specific category, got %q", safetyErr.Category())
		}
	})

	t.Run("handles different safety categories", func(t *testing.T) {
		categories := []string{
			"HARM_CATEGORY_HATE_SPEECH",
			"HARM_CATEGORY_SEXUALLY_EXPLICIT",
			"HARM_CATEGORY_DANGEROUS_CONTENT",
			"HARM_CATEGORY_HARASSMENT",
		}

		for _, category := range categories {
			mockClient := &mockGoogleClient{
				err: &SafetyFilterError{
					reason:   "SAFETY",
					category: category,
				},
			}

			m := &ChatModel{
				client:    mockClient,
				modelName: "gemini-pro",
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			_, err := m.Chat(context.Background(), messages, nil)
			if err == nil {
				t.Errorf("expected error for category %s, got nil", category)
				continue
			}

			var safetyErr *SafetyFilterError
			if !errors.As(err, &safetyErr) {
				t.Errorf("expected SafetyFilterError for %s, got %T", category, err)
			}
		}
	})

	t.Run("passes through non-safety errors", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			err: errors.New("API error: quota exceeded"),
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var safetyErr *SafetyFilterError
		if errors.As(err, &safetyErr) {
			t.Error("expected non-safety error, got SafetyFilterError")
		}
	})
}

// TestGoogleChatModel_ErrorHandling verifies error scenarios (T147).
func TestGoogleChatModel_ErrorHandling(t *testing.T) {
	t.Run("handles API errors", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			err: errors.New("API error: invalid request"),
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("handles quota errors", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			err: errors.New("quota exceeded"),
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected quota error, got nil")
		}
	})

	t.Run("handles empty API key", func(t *testing.T) {
		m := NewChatModel("", "gemini-pro")

		messages := []model.Message{
			{Role: model.RoleUser, Content: "Test"},
		}

		_, err := m.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Error("expected error for empty API key")
		}
	})
}

// TestGoogleChatModel_SafetyFilterHandling verifies filter processing (T148).
func TestGoogleChatModel_SafetyFilterHandling(t *testing.T) {
	t.Run("wraps safety filter errors with context", func(t *testing.T) {
		err := &SafetyFilterError{
			reason:   "SAFETY",
			category: "HARM_CATEGORY_HATE_SPEECH",
		}

		wrapped := handleSafetyFilterError(err)

		var safetyErr *SafetyFilterError
		if !errors.As(wrapped, &safetyErr) {
			t.Fatalf("expected SafetyFilterError, got %T", wrapped)
		}

		if safetyErr.Category() != "HARM_CATEGORY_HATE_SPEECH" {
			t.Errorf("expected preserved category, got %q", safetyErr.Category())
		}
	})

	t.Run("provides user-friendly error messages", func(t *testing.T) {
		err := &SafetyFilterError{
			reason:   "SAFETY",
			category: "HARM_CATEGORY_DANGEROUS_CONTENT",
		}

		wrapped := handleSafetyFilterError(err)
		errMsg := wrapped.Error()

		if errMsg == "" {
			t.Error("expected non-empty error message")
		}

		// Should mention safety
		if len(errMsg) < 5 {
			t.Errorf("expected descriptive error message, got %q", errMsg)
		}
	})
}

// TestGoogleChatModel_MessageConversion verifies message format (T145).
func TestGoogleChatModel_MessageConversion(t *testing.T) {
	t.Run("converts messages to Google format", func(t *testing.T) {
		mockClient := &mockGoogleClient{
			response: "Converted successfully",
		}

		m := &ChatModel{
			client:    mockClient,
			modelName: "gemini-pro",
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
}

// Mock Google client for testing.
type mockGoogleClient struct {
	response     string
	toolCalls    []model.ToolCall
	err          error
	callCount    int
	lastMessages []model.Message
}

func (m *mockGoogleClient) generateContent(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	m.callCount++
	m.lastMessages = messages

	if m.err != nil {
		return model.ChatOut{}, m.err
	}

	return model.ChatOut{
		Text:      m.response,
		ToolCalls: m.toolCalls,
	}, nil
}

// safetyFilterError is imported from the google package
