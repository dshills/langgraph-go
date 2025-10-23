package model

import (
	"context"
	"errors"
	"testing"
)

// TestMockChatModel_SingleResponse verifies basic response behavior (T129).
func TestMockChatModel_SingleResponse(t *testing.T) {
	t.Run("returns configured response", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{Text: "Hello, world!"},
			},
		}

		messages := []Message{
			{Role: RoleUser, Content: "Hi"},
		}

		out, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Hello, world!" {
			t.Errorf("expected Text = 'Hello, world!', got %q", out.Text)
		}
	})

	t.Run("repeats last response when exhausted", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{Text: "Only response"},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		// First call
		out1, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}

		// Second call should repeat the response
		out2, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}

		if out1.Text != out2.Text {
			t.Errorf("expected same response, got %q and %q", out1.Text, out2.Text)
		}
	})

	t.Run("returns empty response when no responses configured", func(t *testing.T) {
		mock := &MockChatModel{}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		out, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "" {
			t.Errorf("expected empty Text, got %q", out.Text)
		}
		if len(out.ToolCalls) != 0 {
			t.Errorf("expected no tool calls, got %d", len(out.ToolCalls))
		}
	})
}

// TestMockChatModel_MultipleResponses verifies sequence behavior (T129).
func TestMockChatModel_MultipleResponses(t *testing.T) {
	t.Run("returns responses in sequence", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{Text: "First"},
				{Text: "Second"},
				{Text: "Third"},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		// Call 1
		out1, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("call 1 failed: %v", err)
		}
		if out1.Text != "First" {
			t.Errorf("call 1: expected 'First', got %q", out1.Text)
		}

		// Call 2
		out2, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("call 2 failed: %v", err)
		}
		if out2.Text != "Second" {
			t.Errorf("call 2: expected 'Second', got %q", out2.Text)
		}

		// Call 3
		out3, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("call 3 failed: %v", err)
		}
		if out3.Text != "Third" {
			t.Errorf("call 3: expected 'Third', got %q", out3.Text)
		}

		// Call 4 should repeat last response
		out4, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("call 4 failed: %v", err)
		}
		if out4.Text != "Third" {
			t.Errorf("call 4: expected 'Third' (repeat), got %q", out4.Text)
		}
	})
}

// TestMockChatModel_ErrorInjection verifies error behavior (T129).
func TestMockChatModel_ErrorInjection(t *testing.T) {
	t.Run("returns configured error", func(t *testing.T) {
		expectedErr := errors.New("simulated API error")
		mock := &MockChatModel{
			Err: expectedErr,
			Responses: []ChatOut{
				{Text: "Should not be returned"},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		_, err := mock.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("error takes precedence over responses", func(t *testing.T) {
		mock := &MockChatModel{
			Err: errors.New("error"),
			Responses: []ChatOut{
				{Text: "Response"},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		_, err := mock.Chat(context.Background(), messages, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// TestMockChatModel_CallHistory verifies tracking behavior (T129).
func TestMockChatModel_CallHistory(t *testing.T) {
	t.Run("records all calls", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{{Text: "OK"}},
		}

		// Make multiple calls with different inputs
		messages1 := []Message{{Role: RoleUser, Content: "First"}}
		messages2 := []Message{{Role: RoleUser, Content: "Second"}}
		tools := []ToolSpec{{Name: "search", Description: "Search"}}

		_, _ = mock.Chat(context.Background(), messages1, nil)
		_, _ = mock.Chat(context.Background(), messages2, tools)

		if len(mock.Calls) != 2 {
			t.Fatalf("expected 2 calls recorded, got %d", len(mock.Calls))
		}

		// Verify first call
		if len(mock.Calls[0].Messages) != 1 {
			t.Errorf("call 0: expected 1 message, got %d", len(mock.Calls[0].Messages))
		}
		if mock.Calls[0].Messages[0].Content != "First" {
			t.Errorf("call 0: expected content 'First', got %q", mock.Calls[0].Messages[0].Content)
		}
		if mock.Calls[0].Tools != nil {
			t.Errorf("call 0: expected nil tools, got %v", mock.Calls[0].Tools)
		}

		// Verify second call
		if len(mock.Calls[1].Messages) != 1 {
			t.Errorf("call 1: expected 1 message, got %d", len(mock.Calls[1].Messages))
		}
		if mock.Calls[1].Messages[0].Content != "Second" {
			t.Errorf("call 1: expected content 'Second', got %q", mock.Calls[1].Messages[0].Content)
		}
		if len(mock.Calls[1].Tools) != 1 {
			t.Errorf("call 1: expected 1 tool, got %d", len(mock.Calls[1].Tools))
		}
	})

	t.Run("records calls even when error configured", func(t *testing.T) {
		mock := &MockChatModel{
			Err: errors.New("error"),
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		_, _ = mock.Chat(context.Background(), messages, nil)

		if len(mock.Calls) != 1 {
			t.Errorf("expected 1 call recorded, got %d", len(mock.Calls))
		}
	})
}

// TestMockChatModel_Reset verifies reset behavior (T129).
func TestMockChatModel_Reset(t *testing.T) {
	t.Run("clears call history", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{{Text: "OK"}},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		// Make some calls
		_, _ = mock.Chat(context.Background(), messages, nil)
		_, _ = mock.Chat(context.Background(), messages, nil)

		if len(mock.Calls) != 2 {
			t.Fatalf("expected 2 calls before reset, got %d", len(mock.Calls))
		}

		// Reset
		mock.Reset()

		if len(mock.Calls) != 0 {
			t.Errorf("expected 0 calls after reset, got %d", len(mock.Calls))
		}
	})

	t.Run("resets response index", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{Text: "First"},
				{Text: "Second"},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		// Exhaust first response
		out1, _ := mock.Chat(context.Background(), messages, nil)
		if out1.Text != "First" {
			t.Fatalf("expected 'First', got %q", out1.Text)
		}

		// Reset and verify we get first response again
		mock.Reset()

		out2, _ := mock.Chat(context.Background(), messages, nil)
		if out2.Text != "First" {
			t.Errorf("expected 'First' after reset, got %q", out2.Text)
		}
	})
}

// TestMockChatModel_CallCount verifies count behavior (T129).
func TestMockChatModel_CallCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{{Text: "OK"}},
		}

		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls initially, got %d", mock.CallCount())
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		_, _ = mock.Chat(context.Background(), messages, nil)
		if mock.CallCount() != 1 {
			t.Errorf("expected 1 call, got %d", mock.CallCount())
		}

		_, _ = mock.Chat(context.Background(), messages, nil)
		if mock.CallCount() != 2 {
			t.Errorf("expected 2 calls, got %d", mock.CallCount())
		}
	})

	t.Run("resets with Reset()", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{{Text: "OK"}},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		_, _ = mock.Chat(context.Background(), messages, nil)
		_, _ = mock.Chat(context.Background(), messages, nil)

		if mock.CallCount() != 2 {
			t.Fatalf("expected 2 calls before reset, got %d", mock.CallCount())
		}

		mock.Reset()

		if mock.CallCount() != 0 {
			t.Errorf("expected 0 calls after reset, got %d", mock.CallCount())
		}
	})
}

// TestMockChatModel_ToolCalls verifies tool call responses (T129).
func TestMockChatModel_ToolCalls(t *testing.T) {
	t.Run("returns tool calls", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{
					ToolCalls: []ToolCall{
						{Name: "search", Input: map[string]interface{}{"query": "Go"}},
					},
				},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Search for Go"}}
		tools := []ToolSpec{{Name: "search", Description: "Search"}}

		out, err := mock.Chat(context.Background(), messages, tools)
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

	t.Run("returns both text and tool calls", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{
				{
					Text: "Let me search for that.",
					ToolCalls: []ToolCall{
						{Name: "search", Input: map[string]interface{}{"query": "test"}},
					},
				},
			},
		}

		messages := []Message{{Role: RoleUser, Content: "Find test"}}

		out, err := mock.Chat(context.Background(), messages, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if out.Text != "Let me search for that." {
			t.Errorf("expected Text = 'Let me search for that.', got %q", out.Text)
		}
		if len(out.ToolCalls) != 1 {
			t.Errorf("expected 1 tool call, got %d", len(out.ToolCalls))
		}
	})
}

// TestMockChatModel_Concurrency verifies thread-safety (T129).
func TestMockChatModel_Concurrency(t *testing.T) {
	t.Run("handles concurrent calls safely", func(t *testing.T) {
		mock := &MockChatModel{
			Responses: []ChatOut{{Text: "OK"}},
		}

		messages := []Message{{Role: RoleUser, Content: "Test"}}

		// Launch multiple concurrent calls
		const goroutines = 10
		done := make(chan bool, goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				_, _ = mock.Chat(context.Background(), messages, nil)
				done <- true
			}()
		}

		// Wait for all to complete
		for i := 0; i < goroutines; i++ {
			<-done
		}

		// Verify all calls were recorded
		if mock.CallCount() != goroutines {
			t.Errorf("expected %d calls, got %d", goroutines, mock.CallCount())
		}
	})
}
