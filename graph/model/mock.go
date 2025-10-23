package model

import (
	"context"
	"sync"
)

// MockChatModel is a test implementation of ChatModel.
//
// Use MockChatModel in tests to verify workflow behavior without
// making actual LLM API calls. It provides:
//   - Configurable responses
//   - Call history tracking
//   - Error injection
//   - Thread-safe operation
//
// Example usage:
//
//	mock := &MockChatModel{
//	    Responses: []ChatOut{
//	        {Text: "First response"},
//	        {Text: "Second response"},
//	    },
//	}
//	out, err := mock.Chat(ctx, messages, nil)
//	// Returns "First response", then "Second response" on subsequent calls
//
// Example with error injection:
//
//	mock := &MockChatModel{
//	    Err: errors.New("API error"),
//	}
//	_, err := mock.Chat(ctx, messages, nil)
//	// Returns the configured error
type MockChatModel struct {
	// Responses contains the sequence of responses to return.
	// Each call to Chat() returns the next response in order.
	// If all responses are consumed, the last response repeats.
	Responses []ChatOut

	// Err, if set, will be returned by Chat() instead of a response.
	Err error

	// Calls tracks the history of all Chat() invocations.
	// Useful for verifying that nodes called the model with expected inputs.
	Calls []MockChatCall

	mu        sync.Mutex // Protects concurrent access to Calls and response index
	callIndex int        // Tracks which response to return next
}

// MockChatCall records a single invocation of Chat().
type MockChatCall struct {
	Messages []Message
	Tools    []ToolSpec
}

// Chat implements the ChatModel interface.
//
// Returns:
//   - The next response from Responses (or repeats the last response)
//   - Or Err if configured
//
// Always records the call in Calls history regardless of success/failure.
func (m *MockChatModel) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error) {
	// Check context cancellation first (before acquiring lock)
	if ctx.Err() != nil {
		return ChatOut{}, ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.Calls = append(m.Calls, MockChatCall{
		Messages: messages,
		Tools:    tools,
	})

	// Return error if configured
	if m.Err != nil {
		return ChatOut{}, m.Err
	}

	// Return empty response if no responses configured
	if len(m.Responses) == 0 {
		return ChatOut{}, nil
	}

	// Get the current response
	idx := m.callIndex
	if idx >= len(m.Responses) {
		idx = len(m.Responses) - 1 // Repeat last response
	} else {
		m.callIndex++ // Advance to next response
	}

	return m.Responses[idx], nil
}

// Reset clears the call history and resets the response index.
//
// Useful when reusing the same mock across multiple test cases:
//
//	mock := &MockChatModel{Responses: []ChatOut{{Text: "OK"}}}
//	// ... run test 1 ...
//	mock.Reset()
//	// ... run test 2 with clean state ...
func (m *MockChatModel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = nil
	m.callIndex = 0
}

// CallCount returns the number of times Chat() has been called.
//
// Thread-safe convenience method:
//
//	if mock.CallCount() != 3 {
//	    t.Errorf("expected 3 calls, got %d", mock.CallCount())
//	}
func (m *MockChatModel) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.Calls)
}
