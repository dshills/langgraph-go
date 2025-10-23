package tool

import (
	"context"
	"sync"
)

// MockTool is a test implementation of Tool.
//
// Use MockTool in tests to verify workflow behavior without
// executing actual tool logic. It provides:
//   - Configurable tool name
//   - Configurable response sequences
//   - Call history tracking
//   - Error injection
//   - Thread-safe operation
//
// Example usage:
//
//	mock := &MockTool{
//	    ToolName: "search_web",
//	    Responses: []map[string]interface{}{
//	        {"results": []string{"result1", "result2"}},
//	    },
//	}
//	output, err := mock.Call(ctx, map[string]interface{}{"query": "test"})
//	// Returns {"results": ["result1", "result2"]}
//
// Example with error injection:
//
//	mock := &MockTool{
//	    ToolName: "api_call",
//	    Err:      errors.New("API timeout"),
//	}
//	_, err := mock.Call(ctx, input)
//	// Returns the configured error
type MockTool struct {
	// ToolName is the identifier returned by Name().
	// Must be set for the mock to function properly.
	ToolName string

	// Responses contains the sequence of outputs to return.
	// Each call to Call() returns the next response in order.
	// If all responses are consumed, the last response repeats.
	Responses []map[string]interface{}

	// Err, if set, will be returned by Call() instead of a response.
	Err error

	// Calls tracks the history of all Call() invocations.
	// Useful for verifying that tools were called with expected inputs.
	Calls []MockToolCall

	mu        sync.Mutex // Protects concurrent access to Calls and response index
	callIndex int        // Tracks which response to return next
}

// MockToolCall records a single invocation of Call().
type MockToolCall struct {
	Input map[string]interface{}
}

// Name implements the Tool interface.
func (m *MockTool) Name() string {
	return m.ToolName
}

// Call implements the Tool interface.
//
// Returns:
//   - The next response from Responses (or repeats the last response)
//   - Or Err if configured
//
// Always records the call in Calls history regardless of success/failure.
func (m *MockTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Check context cancellation first (before acquiring lock)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.Calls = append(m.Calls, MockToolCall{
		Input: input,
	})

	// Return error if configured
	if m.Err != nil {
		return nil, m.Err
	}

	// Return empty response if no responses configured
	if len(m.Responses) == 0 {
		return map[string]interface{}{}, nil
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
//	mock := &MockTool{ToolName: "test", Responses: []map[string]interface{}{{"ok": true}}}
//	// ... run test 1 ...
//	mock.Reset()
//	// ... run test 2 with clean state ...
func (m *MockTool) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = nil
	m.callIndex = 0
}

// CallCount returns the number of times Call() has been called.
//
// Thread-safe convenience method:
//
//	if mock.CallCount() != 2 {
//	    t.Errorf("expected 2 calls, got %d", mock.CallCount())
//	}
func (m *MockTool) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.Calls)
}
