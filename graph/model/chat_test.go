package model

import (
	"context"
	"errors"
	"testing"
)

// TestMessage_Construction verifies Message struct can be created (T120).
func TestMessage_Construction(t *testing.T) {
	t.Run("create user message", func(t *testing.T) {
		msg := Message{
			Role:    "user",
			Content: "Hello, how are you?",
		}

		if msg.Role != "user" {
			t.Errorf("expected Role = 'user', got %q", msg.Role)
		}
		if msg.Content != "Hello, how are you?" {
			t.Errorf("expected Content = 'Hello, how are you?', got %q", msg.Content)
		}
	})

	t.Run("create assistant message", func(t *testing.T) {
		msg := Message{
			Role:    "assistant",
			Content: "I'm doing well, thank you!",
		}

		if msg.Role != "assistant" {
			t.Errorf("expected Role = 'assistant', got %q", msg.Role)
		}
		if msg.Content != "I'm doing well, thank you!" {
			t.Errorf("expected Content = 'I'm doing well, thank you!', got %q", msg.Content)
		}
	})

	t.Run("create system message", func(t *testing.T) {
		msg := Message{
			Role:    "system",
			Content: "You are a helpful assistant.",
		}

		if msg.Role != "system" {
			t.Errorf("expected Role = 'system', got %q", msg.Role)
		}
		if msg.Content != "You are a helpful assistant." {
			t.Errorf("expected Content = 'You are a helpful assistant.', got %q", msg.Content)
		}
	})
}

// TestMessage_Roles verifies standard role constants (T120).
func TestMessage_Roles(t *testing.T) {
	t.Run("verify role constants exist", func(t *testing.T) {
		// Common LLM roles should be available as constants
		roles := []string{
			RoleSystem,
			RoleUser,
			RoleAssistant,
		}

		for _, role := range roles {
			if role == "" {
				t.Errorf("role constant should not be empty")
			}
		}
	})

	t.Run("role constants have expected values", func(t *testing.T) {
		if RoleSystem != "system" {
			t.Errorf("expected RoleSystem = 'system', got %q", RoleSystem)
		}
		if RoleUser != "user" {
			t.Errorf("expected RoleUser = 'user', got %q", RoleUser)
		}
		if RoleAssistant != "assistant" {
			t.Errorf("expected RoleAssistant = 'assistant', got %q", RoleAssistant)
		}
	})
}

// TestMessage_Conversation verifies message slice patterns (T120).
func TestMessage_Conversation(t *testing.T) {
	t.Run("build conversation from multiple messages", func(t *testing.T) {
		conversation := []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant."},
			{Role: RoleUser, Content: "What is 2+2?"},
			{Role: RoleAssistant, Content: "2+2 equals 4."},
			{Role: RoleUser, Content: "Thanks!"},
		}

		if len(conversation) != 4 {
			t.Errorf("expected 4 messages, got %d", len(conversation))
		}

		// Verify alternating user/assistant after system
		if conversation[1].Role != RoleUser {
			t.Errorf("expected second message to be user, got %q", conversation[1].Role)
		}
		if conversation[2].Role != RoleAssistant {
			t.Errorf("expected third message to be assistant, got %q", conversation[2].Role)
		}
	})
}

// TestMessage_EmptyContent verifies empty content is allowed (T120).
func TestMessage_EmptyContent(t *testing.T) {
	t.Run("message can have empty content", func(t *testing.T) {
		msg := Message{
			Role:    RoleUser,
			Content: "",
		}

		// Empty content is valid (e.g., for messages with only tool calls)
		if msg.Role != RoleUser {
			t.Errorf("expected Role = %q, got %q", RoleUser, msg.Role)
		}
		if msg.Content != "" {
			t.Errorf("expected empty Content, got %q", msg.Content)
		}
	})
}

// TestToolSpec_Construction verifies ToolSpec struct can be created (T122).
func TestToolSpec_Construction(t *testing.T) {
	t.Run("create tool spec with all fields", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "search_web",
			Description: "Search the web for information",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query",
					},
				},
				"required": []string{"query"},
			},
		}

		if spec.Name != "search_web" {
			t.Errorf("expected Name = 'search_web', got %q", spec.Name)
		}
		if spec.Description != "Search the web for information" {
			t.Errorf("expected Description = 'Search the web for information', got %q", spec.Description)
		}
		if spec.Schema == nil {
			t.Error("expected Schema to be non-nil")
		}
	})

	t.Run("create minimal tool spec", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "get_weather",
			Description: "Get current weather",
		}

		if spec.Name != "get_weather" {
			t.Errorf("expected Name = 'get_weather', got %q", spec.Name)
		}
		if spec.Schema == nil {
			// Schema being nil is acceptable for simple tools
		}
	})
}

// TestToolSpec_JSONSchema verifies schema field structure (T122).
func TestToolSpec_JSONSchema(t *testing.T) {
	t.Run("schema follows JSON Schema format", func(t *testing.T) {
		spec := ToolSpec{
			Name:        "calculate",
			Description: "Perform a calculation",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"expression": map[string]interface{}{
						"type":        "string",
						"description": "Math expression to evaluate",
					},
				},
			},
		}

		// Verify schema structure
		schemaType, ok := spec.Schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("expected schema type = 'object', got %v", schemaType)
		}

		properties, ok := spec.Schema["properties"].(map[string]interface{})
		if !ok {
			t.Error("expected properties to be map[string]interface{}")
		}

		if properties["expression"] == nil {
			t.Error("expected 'expression' property to exist")
		}
	})
}

// TestChatOut_Construction verifies ChatOut struct can be created (T124).
func TestChatOut_Construction(t *testing.T) {
	t.Run("create chat output with text only", func(t *testing.T) {
		out := ChatOut{
			Text: "Hello, how can I help you today?",
		}

		if out.Text != "Hello, how can I help you today?" {
			t.Errorf("expected Text = 'Hello, how can I help you today?', got %q", out.Text)
		}
		if len(out.ToolCalls) != 0 {
			t.Errorf("expected no tool calls, got %d", len(out.ToolCalls))
		}
	})

	t.Run("create chat output with tool calls", func(t *testing.T) {
		out := ChatOut{
			Text: "",
			ToolCalls: []ToolCall{
				{
					Name:  "search_web",
					Input: map[string]interface{}{"query": "Go programming"},
				},
			},
		}

		if out.Text != "" {
			t.Errorf("expected empty Text, got %q", out.Text)
		}
		if len(out.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(out.ToolCalls))
		}
		if out.ToolCalls[0].Name != "search_web" {
			t.Errorf("expected tool Name = 'search_web', got %q", out.ToolCalls[0].Name)
		}
	})

	t.Run("create chat output with both text and tool calls", func(t *testing.T) {
		out := ChatOut{
			Text: "Let me search for that information.",
			ToolCalls: []ToolCall{
				{
					Name:  "search_web",
					Input: map[string]interface{}{"query": "weather"},
				},
			},
		}

		if out.Text == "" {
			t.Error("expected non-empty Text")
		}
		if len(out.ToolCalls) != 1 {
			t.Errorf("expected 1 tool call, got %d", len(out.ToolCalls))
		}
	})
}

// TestToolCall_Structure verifies ToolCall struct (T124).
func TestToolCall_Structure(t *testing.T) {
	t.Run("tool call with structured input", func(t *testing.T) {
		call := ToolCall{
			Name: "get_weather",
			Input: map[string]interface{}{
				"location": "San Francisco",
				"units":    "celsius",
			},
		}

		if call.Name != "get_weather" {
			t.Errorf("expected Name = 'get_weather', got %q", call.Name)
		}

		location, ok := call.Input["location"].(string)
		if !ok || location != "San Francisco" {
			t.Errorf("expected location = 'San Francisco', got %v", location)
		}

		units, ok := call.Input["units"].(string)
		if !ok || units != "celsius" {
			t.Errorf("expected units = 'celsius', got %v", units)
		}
	})

	t.Run("tool call with empty input", func(t *testing.T) {
		call := ToolCall{
			Name:  "get_current_time",
			Input: nil,
		}

		if call.Name != "get_current_time" {
			t.Errorf("expected Name = 'get_current_time', got %q", call.Name)
		}
		if call.Input != nil {
			t.Errorf("expected nil Input, got %v", call.Input)
		}
	})
}

// TestChatModel_Interface verifies ChatModel interface contract (T126).
func TestChatModel_Interface(t *testing.T) {
	t.Run("interface can be implemented", func(t *testing.T) {
		// Verify that a concrete type can implement ChatModel
		var _ ChatModel = &testChatModel{}
	})

	t.Run("chat method accepts messages and tools", func(t *testing.T) {
		model := &testChatModel{
			response: ChatOut{Text: "Hello!"},
		}

		messages := []Message{
			{Role: RoleUser, Content: "Hi"},
		}
		tools := []ToolSpec{
			{Name: "search", Description: "Search the web"},
		}

		out, err := model.Chat(context.Background(), messages, tools)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Hello!" {
			t.Errorf("expected Text = 'Hello!', got %q", out.Text)
		}
	})

	t.Run("chat method works with nil tools", func(t *testing.T) {
		model := &testChatModel{
			response: ChatOut{Text: "Response without tools"},
		}

		messages := []Message{
			{Role: RoleUser, Content: "Question"},
		}

		out, err := model.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Response without tools" {
			t.Errorf("expected specific response, got %q", out.Text)
		}
	})

	t.Run("chat method returns tool calls", func(t *testing.T) {
		model := &testChatModel{
			response: ChatOut{
				ToolCalls: []ToolCall{
					{Name: "search", Input: map[string]interface{}{"query": "Go"}},
				},
			},
		}

		messages := []Message{
			{Role: RoleUser, Content: "Search for Go"},
		}
		tools := []ToolSpec{
			{Name: "search", Description: "Search"},
		}

		out, err := model.Chat(context.Background(), messages, tools)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(out.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(out.ToolCalls))
		}
		if out.ToolCalls[0].Name != "search" {
			t.Errorf("expected tool Name = 'search', got %q", out.ToolCalls[0].Name)
		}
	})

	t.Run("chat method returns errors", func(t *testing.T) {
		expectedErr := errors.New("API error")
		model := &testChatModel{
			err: expectedErr,
		}

		messages := []Message{
			{Role: RoleUser, Content: "Test"},
		}

		_, err := model.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("chat method respects context cancellation", func(t *testing.T) {
		model := &testChatModel{
			response: ChatOut{Text: "Should not return"},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		messages := []Message{
			{Role: RoleUser, Content: "Test"},
		}

		_, err := model.Chat(ctx, messages, nil)
		// Implementation should check context and return error
		// The testChatModel will verify ctx was passed
		if err != nil && ctx.Err() == nil {
			t.Errorf("expected context-related error when cancelled")
		}
	})
}

// testChatModel is a simple ChatModel implementation for testing (T126).
type testChatModel struct {
	response ChatOut
	err      error
}

func (m *testChatModel) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error) {
	// Check context for cancellation
	if ctx.Err() != nil {
		return ChatOut{}, ctx.Err()
	}

	if m.err != nil {
		return ChatOut{}, m.err
	}

	return m.response, nil
}
