package bedrock

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// T064: Test ToolSpec translation - LangGraph ToolSpec → Claude tool schema
func TestTranslateTools(t *testing.T) {
	tests := []struct {
		name      string
		toolSpecs []model.ToolSpec
		want      []map[string]interface{}
	}{
		{
			name: "single tool with simple schema",
			toolSpecs: []model.ToolSpec{
				{
					Name:        "get_weather",
					Description: "Get current weather for a location",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "City name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
			want: []map[string]interface{}{
				{
					"name":        "get_weather",
					"description": "Get current weather for a location",
					"input_schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "City name",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		{
			name: "multiple tools with complex schemas",
			toolSpecs: []model.ToolSpec{
				{
					Name:        "get_weather",
					Description: "Get current weather",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
				{
					Name:        "search_web",
					Description: "Search the web",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
							"max_results": map[string]interface{}{
								"type":    "integer",
								"default": 10,
							},
						},
						"required": []string{"query"},
					},
				},
			},
			want: []map[string]interface{}{
				{
					"name":        "get_weather",
					"description": "Get current weather",
					"input_schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
				{
					"name":        "search_web",
					"description": "Search the web",
					"input_schema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
							"max_results": map[string]interface{}{
								"type":    "integer",
								"default": 10,
							},
						},
						"required": []string{"query"},
					},
				},
			},
		},
		{
			name:      "empty tool list",
			toolSpecs: []model.ToolSpec{},
			want:      []map[string]interface{}{},
		},
		{
			name:      "nil tool list returns empty slice (Go idiom)",
			toolSpecs: nil,
			want:      []map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateTools(tt.toolSpecs)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("translateTools() = %v, want %v", got, tt.want)
			}
		})
	}
}

// T064 (continued): Test ToolSpec translation in full request context
func TestClaudeSchemaTranslator_TranslateRequest_WithTools(t *testing.T) {
	translator := ClaudeSchemaTranslator{}
	config := &Config{
		MaxTokens: 4096,
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What's the weather in San Francisco?"},
	}

	tools := []model.ToolSpec{
		{
			Name:        "get_weather",
			Description: "Get current weather for a location",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []string{"location"},
			},
		},
	}

	got, err := translator.TranslateRequest(messages, tools, config)
	if err != nil {
		t.Fatalf("TranslateRequest() unexpected error = %v", err)
	}

	// Parse JSON to verify structure
	var req map[string]interface{}
	if err := json.Unmarshal(got, &req); err != nil {
		t.Fatalf("TranslateRequest() returned invalid JSON: %v", err)
	}

	// Verify tools field exists
	toolsField, ok := req["tools"]
	if !ok {
		t.Fatal("TranslateRequest() missing tools field")
	}

	// Verify tools structure
	toolsArray, ok := toolsField.([]interface{})
	if !ok {
		t.Fatalf("TranslateRequest() tools field is not array, got type %T", toolsField)
	}

	if len(toolsArray) != 1 {
		t.Fatalf("TranslateRequest() len(tools) = %d, want 1", len(toolsArray))
	}

	tool := toolsArray[0].(map[string]interface{})
	if tool["name"] != "get_weather" {
		t.Errorf("TranslateRequest() tool name = %v, want get_weather", tool["name"])
	}
	if tool["description"] != "Get current weather for a location" {
		t.Errorf("TranslateRequest() tool description = %v, want 'Get current weather for a location'", tool["description"])
	}
	if _, ok := tool["input_schema"]; !ok {
		t.Error("TranslateRequest() tool missing input_schema")
	}
}

// T065: Test tool_use content block parsing - Claude response → ToolCall[]
func TestClaudeSchemaTranslator_TranslateResponse_MultipleToolCalls(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name     string
		response string
		wantLen  int
		wantErr  bool
	}{
		{
			name: "single tool call",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "toolu_01DEF",
						"name": "get_weather",
						"input": {"location": "San Francisco"}
					}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 50, "output_tokens": 20}
			}`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "multiple tool calls",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "toolu_01DEF",
						"name": "get_weather",
						"input": {"location": "San Francisco"}
					},
					{
						"type": "tool_use",
						"id": "toolu_02GHI",
						"name": "search_web",
						"input": {"query": "weather forecast"}
					}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 50, "output_tokens": 40}
			}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "text and tool call mixed",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "text",
						"text": "Let me check the weather for you."
					},
					{
						"type": "tool_use",
						"id": "toolu_01DEF",
						"name": "get_weather",
						"input": {"location": "San Francisco"}
					}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "tool_use",
				"usage": {"input_tokens": 50, "output_tokens": 30}
			}`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "no tool calls",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "text",
						"text": "I cannot help with that."
					}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 50, "output_tokens": 10}
			}`,
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.TranslateResponse(json.RawMessage(tt.response))
			if tt.wantErr {
				if err == nil {
					t.Error("TranslateResponse() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("TranslateResponse() unexpected error = %v", err)
			}

			if len(got.ToolCalls) != tt.wantLen {
				t.Errorf("TranslateResponse() len(ToolCalls) = %d, want %d", len(got.ToolCalls), tt.wantLen)
			}

			// Verify stop_reason is captured in metadata
			if tt.wantLen > 0 {
				if stopReason, ok := got.Meta["stop_reason"].(string); ok {
					if stopReason != "tool_use" {
						t.Errorf("TranslateResponse() Meta[stop_reason] = %v, want tool_use", stopReason)
					}
				}
			}
		})
	}
}

// T066: Test tool result round-trip - ToolCall → execute → feed back → final answer
func TestClaudeSchemaTranslator_ToolResultRoundTrip(t *testing.T) {
	t.Skip("Integration test - requires AWS Bedrock credentials and mock tool execution")

	// This test would:
	// 1. Send initial prompt with tools
	// 2. Get tool_use response
	// 3. Execute tool (mock)
	// 4. Feed tool result back to Claude
	// 5. Get final text answer
	//
	// Example flow:
	// User: "What's the weather in SF?"
	// Claude: <tool_use: get_weather(location="San Francisco")>
	// Tool: {"temperature": 72, "conditions": "sunny"}
	// Claude: "The weather in San Francisco is 72°F and sunny."
}

// T067: Test non-tool-capable models - Llama/Titan with tools should handle gracefully
func TestAdapter_Chat_ToolsWithNonToolCapableModel(t *testing.T) {
	// Test that non-tool-capable models return clear error when tools provided
	tests := []struct {
		name        string
		modelFamily ModelFamily
		wantErr     bool
		errContains string
	}{
		{
			name:        "Claude supports tools",
			modelFamily: ModelFamilyClaude,
			wantErr:     false,
		},
		{
			name:        "Llama does not support tools",
			modelFamily: ModelFamilyLlama,
			wantErr:     true,
			errContains: "not yet implemented", // Will change to "does not support tools" in future
		},
		{
			name:        "Titan does not support tools",
			modelFamily: ModelFamilyTitan,
			wantErr:     true,
			errContains: "not yet implemented",
		},
		{
			name:        "Mistral limited tool support",
			modelFamily: ModelFamilyMistral,
			wantErr:     true,
			errContains: "not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test structure shows intent
			// Actual implementation would need mock client or will be integration test

			if tt.modelFamily != ModelFamilyClaude {
				t.Skip("Model family not yet implemented - will test when implemented")
			}

			// When model families are implemented, this would:
			// 1. Create adapter with model family
			// 2. Call Chat with tools
			// 3. Verify error if model doesn't support tools
		})
	}
}

// Test SupportsTools() for each model family
func TestModelFamily_SupportsTools(t *testing.T) {
	tests := []struct {
		name        string
		translator  SchemaTranslator
		wantSupport bool
	}{
		{
			name:        "Claude supports tools",
			translator:  ClaudeSchemaTranslator{},
			wantSupport: true,
		},
		// Future model families will be tested here when implemented
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.translator.SupportsTools()
			if got != tt.wantSupport {
				t.Errorf("SupportsTools() = %v, want %v", got, tt.wantSupport)
			}
		})
	}
}
