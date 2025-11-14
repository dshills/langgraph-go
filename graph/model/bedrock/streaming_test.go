package bedrock

import (
	"encoding/json"
	"testing"
)

// T052: Test TranslateStreamEvent() - message_start event
func TestClaudeSchemaTranslator_TranslateStreamEvent_MessageStart(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	// Mock message_start event from Bedrock streaming API
	eventJSON := `{
		"type": "message_start",
		"message": {
			"id": "msg_01ABC123",
			"type": "message",
			"role": "assistant",
			"content": [],
			"model": "claude-3-5-sonnet-20241022",
			"usage": {
				"input_tokens": 25,
				"output_tokens": 0
			}
		}
	}`

	chunk, err := translator.TranslateStreamEvent(json.RawMessage(eventJSON))
	if err != nil {
		t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
	}

	// message_start should populate metadata, no delta
	if chunk.Delta != "" {
		t.Errorf("TranslateStreamEvent() message_start should have empty Delta, got %q", chunk.Delta)
	}

	if chunk.FinishReason != "" {
		t.Errorf("TranslateStreamEvent() message_start should have empty FinishReason, got %q", chunk.FinishReason)
	}

	// Verify metadata extraction
	if chunk.Metadata == nil {
		t.Fatal("TranslateStreamEvent() Metadata is nil, expected metadata")
	}

	if requestID, ok := chunk.Metadata["request_id"].(string); !ok || requestID != "msg_01ABC123" {
		t.Errorf("TranslateStreamEvent() Metadata[request_id] = %v, want %q", chunk.Metadata["request_id"], "msg_01ABC123")
	}

	if model, ok := chunk.Metadata["model"].(string); !ok || model != "claude-3-5-sonnet-20241022" {
		t.Errorf("TranslateStreamEvent() Metadata[model] = %v, want %q", chunk.Metadata["model"], "claude-3-5-sonnet-20241022")
	}

	if inputTokens, ok := chunk.Metadata["input_tokens"].(int); !ok || inputTokens != 25 {
		t.Errorf("TranslateStreamEvent() Metadata[input_tokens] = %v, want 25", chunk.Metadata["input_tokens"])
	}
}

// T053: Test TranslateStreamEvent() - content_block_delta event (text)
func TestClaudeSchemaTranslator_TranslateStreamEvent_ContentBlockDelta(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name      string
		eventJSON string
		wantDelta string
		wantIndex int
	}{
		{
			name: "text delta",
			eventJSON: `{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": "Hello"
				}
			}`,
			wantDelta: "Hello",
			wantIndex: 0,
		},
		{
			name: "text delta with whitespace",
			eventJSON: `{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": " world!"
				}
			}`,
			wantDelta: " world!",
			wantIndex: 0,
		},
		{
			name: "empty text delta",
			eventJSON: `{
				"type": "content_block_delta",
				"index": 0,
				"delta": {
					"type": "text_delta",
					"text": ""
				}
			}`,
			wantDelta: "",
			wantIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := translator.TranslateStreamEvent(json.RawMessage(tt.eventJSON))
			if err != nil {
				t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
			}

			if chunk.Delta != tt.wantDelta {
				t.Errorf("TranslateStreamEvent() Delta = %q, want %q", chunk.Delta, tt.wantDelta)
			}

			if chunk.FinishReason != "" {
				t.Errorf("TranslateStreamEvent() FinishReason should be empty, got %q", chunk.FinishReason)
			}
		})
	}
}

// T053 (continued): Test TranslateStreamEvent() - content_block_delta event (tool input JSON)
func TestClaudeSchemaTranslator_TranslateStreamEvent_ToolCallDelta(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	eventJSON := `{
		"type": "content_block_delta",
		"index": 1,
		"delta": {
			"type": "input_json_delta",
			"partial_json": "{\"location\": \"San Francisco\""
		}
	}`

	chunk, err := translator.TranslateStreamEvent(json.RawMessage(eventJSON))
	if err != nil {
		t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
	}

	// Tool input delta should populate ToolCallDelta, not Delta
	if chunk.Delta != "" {
		t.Errorf("TranslateStreamEvent() Delta should be empty for tool input, got %q", chunk.Delta)
	}

	if chunk.ToolCallDelta == nil {
		t.Fatal("TranslateStreamEvent() ToolCallDelta is nil, expected tool call delta")
	}

	if chunk.ToolCallDelta.Index != 1 {
		t.Errorf("TranslateStreamEvent() ToolCallDelta.Index = %d, want 1", chunk.ToolCallDelta.Index)
	}

	wantJSON := "{\"location\": \"San Francisco\""
	if chunk.ToolCallDelta.PartialJSON != wantJSON {
		t.Errorf("TranslateStreamEvent() ToolCallDelta.PartialJSON = %q, want %q",
			chunk.ToolCallDelta.PartialJSON, wantJSON)
	}
}

// T054: Test TranslateStreamEvent() - message_delta event with stop_reason
func TestClaudeSchemaTranslator_TranslateStreamEvent_MessageDelta(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name           string
		eventJSON      string
		wantFinish     string
		wantOutputToks int
	}{
		{
			name: "end_turn stop reason",
			eventJSON: `{
				"type": "message_delta",
				"delta": {
					"stop_reason": "end_turn"
				},
				"usage": {
					"output_tokens": 42
				}
			}`,
			wantFinish:     "end_turn",
			wantOutputToks: 42,
		},
		{
			name: "max_tokens stop reason",
			eventJSON: `{
				"type": "message_delta",
				"delta": {
					"stop_reason": "max_tokens"
				},
				"usage": {
					"output_tokens": 4096
				}
			}`,
			wantFinish:     "max_tokens",
			wantOutputToks: 4096,
		},
		{
			name: "tool_use stop reason",
			eventJSON: `{
				"type": "message_delta",
				"delta": {
					"stop_reason": "tool_use"
				},
				"usage": {
					"output_tokens": 128
				}
			}`,
			wantFinish:     "tool_use",
			wantOutputToks: 128,
		},
		{
			name: "stop_sequence stop reason",
			eventJSON: `{
				"type": "message_delta",
				"delta": {
					"stop_reason": "stop_sequence",
					"stop_sequence": "</answer>"
				},
				"usage": {
					"output_tokens": 200
				}
			}`,
			wantFinish:     "stop_sequence",
			wantOutputToks: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := translator.TranslateStreamEvent(json.RawMessage(tt.eventJSON))
			if err != nil {
				t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
			}

			if chunk.FinishReason != tt.wantFinish {
				t.Errorf("TranslateStreamEvent() FinishReason = %q, want %q", chunk.FinishReason, tt.wantFinish)
			}

			if chunk.Delta != "" {
				t.Errorf("TranslateStreamEvent() Delta should be empty, got %q", chunk.Delta)
			}

			// Verify output token count in metadata
			if chunk.Metadata == nil {
				t.Fatal("TranslateStreamEvent() Metadata is nil, expected metadata")
			}

			if outputTokens, ok := chunk.Metadata["output_tokens"].(int); !ok || outputTokens != tt.wantOutputToks {
				t.Errorf("TranslateStreamEvent() Metadata[output_tokens] = %v, want %d",
					chunk.Metadata["output_tokens"], tt.wantOutputToks)
			}
		})
	}
}

// Test TranslateStreamEvent() - content_block_start event
func TestClaudeSchemaTranslator_TranslateStreamEvent_ContentBlockStart(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name      string
		eventJSON string
		wantType  string
		wantName  string // For tool_use blocks
	}{
		{
			name: "text block start",
			eventJSON: `{
				"type": "content_block_start",
				"index": 0,
				"content_block": {
					"type": "text",
					"text": ""
				}
			}`,
			wantType: "text",
		},
		{
			name: "tool_use block start",
			eventJSON: `{
				"type": "content_block_start",
				"index": 1,
				"content_block": {
					"type": "tool_use",
					"id": "toolu_01ABC",
					"name": "get_weather",
					"input": {}
				}
			}`,
			wantType: "tool_use",
			wantName: "get_weather",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := translator.TranslateStreamEvent(json.RawMessage(tt.eventJSON))
			if err != nil {
				t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
			}

			// content_block_start for tool_use should populate ToolCallDelta.Name
			if tt.wantType == "tool_use" {
				if chunk.ToolCallDelta == nil {
					t.Fatal("TranslateStreamEvent() ToolCallDelta is nil for tool_use block start")
				}
				if chunk.ToolCallDelta.Name != tt.wantName {
					t.Errorf("TranslateStreamEvent() ToolCallDelta.Name = %q, want %q",
						chunk.ToolCallDelta.Name, tt.wantName)
				}
			}
		})
	}
}

// Test TranslateStreamEvent() - content_block_stop and message_stop events
func TestClaudeSchemaTranslator_TranslateStreamEvent_StopEvents(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name      string
		eventJSON string
	}{
		{
			name: "content_block_stop",
			eventJSON: `{
				"type": "content_block_stop",
				"index": 0
			}`,
		},
		{
			name: "message_stop",
			eventJSON: `{
				"type": "message_stop"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := translator.TranslateStreamEvent(json.RawMessage(tt.eventJSON))
			if err != nil {
				t.Fatalf("TranslateStreamEvent() unexpected error = %v", err)
			}

			// Stop events should return empty chunks
			if chunk.Delta != "" {
				t.Errorf("TranslateStreamEvent() Delta should be empty, got %q", chunk.Delta)
			}
			if chunk.FinishReason != "" {
				t.Errorf("TranslateStreamEvent() FinishReason should be empty, got %q", chunk.FinishReason)
			}
		})
	}
}

// Test TranslateStreamEvent() - error event
func TestClaudeSchemaTranslator_TranslateStreamEvent_Error(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	eventJSON := `{
		"type": "error",
		"error": {
			"type": "overloaded_error",
			"message": "Overloaded - please retry"
		}
	}`

	_, err := translator.TranslateStreamEvent(json.RawMessage(eventJSON))
	if err == nil {
		t.Fatal("TranslateStreamEvent() expected error for error event, got nil")
	}

	// Verify error message contains type and message
	errStr := err.Error()
	if !contains(errStr, "overloaded_error") {
		t.Errorf("TranslateStreamEvent() error should contain error type, got %q", errStr)
	}
	if !contains(errStr, "Overloaded") {
		t.Errorf("TranslateStreamEvent() error should contain error message, got %q", errStr)
	}
}

// Test TranslateStreamEvent() - invalid JSON
func TestClaudeSchemaTranslator_TranslateStreamEvent_InvalidJSON(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	invalidJSON := `{invalid json`

	_, err := translator.TranslateStreamEvent(json.RawMessage(invalidJSON))
	if err == nil {
		t.Fatal("TranslateStreamEvent() expected error for invalid JSON, got nil")
	}
}

// Test TranslateStreamEvent() - unknown event type
func TestClaudeSchemaTranslator_TranslateStreamEvent_UnknownType(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	unknownEventJSON := `{
		"type": "unknown_event_type",
		"data": {}
	}`

	chunk, err := translator.TranslateStreamEvent(json.RawMessage(unknownEventJSON))
	if err != nil {
		t.Fatalf("TranslateStreamEvent() should handle unknown event gracefully, got error: %v", err)
	}

	// Unknown events should return empty chunk (no-op)
	if chunk.Delta != "" || chunk.FinishReason != "" {
		t.Errorf("TranslateStreamEvent() unknown event should return empty chunk")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
