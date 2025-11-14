package bedrock

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// T016: Test detectModelFamily() for Claude model IDs
func TestDetectModelFamily(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    ModelFamily
	}{
		{
			name:    "Claude 3.5 Sonnet",
			modelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			want:    ModelFamilyClaude,
		},
		{
			name:    "Claude 3 Sonnet",
			modelID: "anthropic.claude-3-sonnet-20240229-v1:0",
			want:    ModelFamilyClaude,
		},
		{
			name:    "Claude 3 Haiku",
			modelID: "anthropic.claude-3-haiku-20240307-v1:0",
			want:    ModelFamilyClaude,
		},
		{
			name:    "Llama 3.2 90B",
			modelID: "meta.llama3-2-90b-instruct-v1:0",
			want:    ModelFamilyLlama,
		},
		{
			name:    "Llama 3.1 70B",
			modelID: "meta.llama3-1-70b-instruct-v1:0",
			want:    ModelFamilyLlama,
		},
		{
			name:    "Titan Text Premier",
			modelID: "amazon.titan-text-premier-v1:0",
			want:    ModelFamilyTitan,
		},
		{
			name:    "Titan Text Express",
			modelID: "amazon.titan-text-express-v1",
			want:    ModelFamilyTitan,
		},
		{
			name:    "Mistral Large",
			modelID: "mistral.mistral-large-2402-v1:0",
			want:    ModelFamilyMistral,
		},
		{
			name:    "Mistral 7B",
			modelID: "mistral.mistral-7b-instruct-v0:2",
			want:    ModelFamilyMistral,
		},
		// Inference profile formats (cross-region)
		{
			name:    "Claude inference profile (US)",
			modelID: "us.anthropic.claude-3-5-sonnet-20241022-v2:0",
			want:    ModelFamilyClaude,
		},
		{
			name:    "Claude inference profile (EU)",
			modelID: "eu.anthropic.claude-3-sonnet-20240229-v1:0",
			want:    ModelFamilyClaude,
		},
		{
			name:    "Llama inference profile",
			modelID: "us.meta.llama3-1-70b-instruct-v1:0",
			want:    ModelFamilyLlama,
		},
		{
			name:    "Titan inference profile",
			modelID: "us.amazon.titan-text-premier-v1:0",
			want:    ModelFamilyTitan,
		},
		{
			name:    "Mistral inference profile",
			modelID: "eu.mistral.mistral-large-2402-v1:0",
			want:    ModelFamilyMistral,
		},
		{
			name:    "Unknown model",
			modelID: "unknown.model-v1:0",
			want:    ModelFamilyUnknown,
		},
		{
			name:    "Empty model ID",
			modelID: "",
			want:    ModelFamilyUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectModelFamily(tt.modelID)
			if got != tt.want {
				t.Errorf("detectModelFamily(%q) = %v, want %v", tt.modelID, got, tt.want)
			}
		})
	}
}

// T017: Test ClaudeSchemaTranslator.TranslateRequest() - basic message translation
func TestClaudeSchemaTranslator_TranslateRequest_Basic(t *testing.T) {
	translator := ClaudeSchemaTranslator{}
	config := &Config{
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	tests := []struct {
		name     string
		messages []model.Message
		tools    []model.ToolSpec
		wantErr  bool
	}{
		{
			name: "single user message",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "Hello, Claude!"},
			},
			tools:   nil,
			wantErr: false,
		},
		{
			name: "conversation with user and assistant",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "What is 2+2?"},
				{Role: model.RoleAssistant, Content: "The answer is 4."},
				{Role: model.RoleUser, Content: "Thanks!"},
			},
			tools:   nil,
			wantErr: false,
		},
		{
			name:     "empty messages",
			messages: []model.Message{},
			tools:    nil,
			wantErr:  true, // Should fail - at least one message required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.TranslateRequest(tt.messages, tt.tools, config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateRequest() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateRequest() unexpected error = %v", err)
				return
			}

			// Verify JSON is valid
			var req map[string]interface{}
			if err := json.Unmarshal(got, &req); err != nil {
				t.Errorf("TranslateRequest() returned invalid JSON: %v", err)
				return
			}

			// Verify required fields
			if _, ok := req["anthropic_version"]; !ok {
				t.Error("TranslateRequest() missing anthropic_version field")
			}
			if _, ok := req["max_tokens"]; !ok {
				t.Error("TranslateRequest() missing max_tokens field")
			}
			if _, ok := req["messages"]; !ok {
				t.Error("TranslateRequest() missing messages field")
			}
		})
	}
}

// T018: Test ClaudeSchemaTranslator.TranslateRequest() - system message extraction
func TestClaudeSchemaTranslator_TranslateRequest_SystemMessage(t *testing.T) {
	translator := ClaudeSchemaTranslator{}
	config := &Config{
		MaxTokens: 4096,
	}

	tests := []struct {
		name        string
		messages    []model.Message
		wantSystem  bool
		systemValue string
	}{
		{
			name: "system message at start",
			messages: []model.Message{
				{Role: model.RoleSystem, Content: "You are a helpful assistant."},
				{Role: model.RoleUser, Content: "Hello!"},
			},
			wantSystem:  true,
			systemValue: "You are a helpful assistant.",
		},
		{
			name: "no system message",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "Hello!"},
			},
			wantSystem: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.TranslateRequest(tt.messages, nil, config)
			if err != nil {
				t.Errorf("TranslateRequest() unexpected error = %v", err)
				return
			}

			var req map[string]interface{}
			if err := json.Unmarshal(got, &req); err != nil {
				t.Errorf("TranslateRequest() returned invalid JSON: %v", err)
				return
			}

			system, hasSystem := req["system"]
			if tt.wantSystem {
				if !hasSystem {
					t.Error("TranslateRequest() expected system field, got none")
					return
				}
				if system != tt.systemValue {
					t.Errorf("TranslateRequest() system = %q, want %q", system, tt.systemValue)
				}
			} else {
				if hasSystem {
					t.Errorf("TranslateRequest() unexpected system field: %v", system)
				}
			}

			// Verify system message not in messages array
			messages, ok := req["messages"].([]interface{})
			if !ok {
				t.Error("TranslateRequest() messages field is not an array")
				return
			}
			for _, msg := range messages {
				msgMap, ok := msg.(map[string]interface{})
				if !ok {
					continue
				}
				role, ok := msgMap["role"].(string)
				if ok && role == "system" {
					t.Error("TranslateRequest() system message found in messages array (should be extracted to system field)")
				}
			}
		})
	}
}

// T019: Test ClaudeSchemaTranslator.TranslateResponse() - text content parsing
func TestClaudeSchemaTranslator_TranslateResponse_Text(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	tests := []struct {
		name     string
		response string
		wantText string
		wantErr  bool
	}{
		{
			name: "simple text response",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Hello! How can I help?"}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 10, "output_tokens": 15}
			}`,
			wantText: "Hello! How can I help?",
			wantErr:  false,
		},
		{
			name: "multiple text blocks",
			response: `{
				"id": "msg_01ABC",
				"type": "message",
				"role": "assistant",
				"content": [
					{"type": "text", "text": "First paragraph."},
					{"type": "text", "text": " Second paragraph."}
				],
				"model": "claude-3-5-sonnet-20241022",
				"stop_reason": "end_turn",
				"usage": {"input_tokens": 10, "output_tokens": 20}
			}`,
			wantText: "First paragraph. Second paragraph.",
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			response: `{invalid json}`,
			wantText: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.TranslateResponse(json.RawMessage(tt.response))
			if tt.wantErr {
				if err == nil {
					t.Errorf("TranslateResponse() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("TranslateResponse() unexpected error = %v", err)
				return
			}

			if got.Text != tt.wantText {
				t.Errorf("TranslateResponse() Text = %q, want %q", got.Text, tt.wantText)
			}
		})
	}
}

// T020: Test ClaudeSchemaTranslator.TranslateResponse() - metadata extraction
func TestClaudeSchemaTranslator_TranslateResponse_Metadata(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	response := `{
		"id": "msg_01XYZ123",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Response text"}
		],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "max_tokens",
		"usage": {"input_tokens": 100, "output_tokens": 50}
	}`

	got, err := translator.TranslateResponse(json.RawMessage(response))
	if err != nil {
		t.Fatalf("TranslateResponse() unexpected error = %v", err)
	}

	// Note: ChatOut doesn't have a Meta field in the current interface,
	// but data-model.md specifies it should. For now, we'll test that
	// the response is parsed correctly. In implementation, we may need
	// to extend ChatOut or handle metadata differently.

	if got.Text != "Response text" {
		t.Errorf("TranslateResponse() Text = %q, want %q", got.Text, "Response text")
	}

	// Test tool calls are empty for text-only response
	if len(got.ToolCalls) != 0 {
		t.Errorf("TranslateResponse() ToolCalls = %v, want empty", got.ToolCalls)
	}
}

// TestClaudeSchemaTranslator_SupportsFeatures tests feature support flags
func TestClaudeSchemaTranslator_SupportsFeatures(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	if !translator.SupportsStreaming() {
		t.Error("ClaudeSchemaTranslator.SupportsStreaming() = false, want true")
	}

	if !translator.SupportsTools() {
		t.Error("ClaudeSchemaTranslator.SupportsTools() = false, want true")
	}
}

// TestModelFamily_String tests string representation
func TestModelFamily_String(t *testing.T) {
	tests := []struct {
		family ModelFamily
		want   string
	}{
		{ModelFamilyClaude, "Claude"},
		{ModelFamilyLlama, "Llama"},
		{ModelFamilyTitan, "Titan"},
		{ModelFamilyMistral, "Mistral"},
		{ModelFamilyUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.family.String()
			if got != tt.want {
				t.Errorf("ModelFamily.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// T021: Test tool call translation (for Phase 7 User Story 5, but defining structure now)
func TestClaudeSchemaTranslator_TranslateResponse_ToolCalls(t *testing.T) {
	translator := ClaudeSchemaTranslator{}

	response := `{
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
	}`

	got, err := translator.TranslateResponse(json.RawMessage(response))
	if err != nil {
		t.Fatalf("TranslateResponse() unexpected error = %v", err)
	}

	if len(got.ToolCalls) != 1 {
		t.Fatalf("TranslateResponse() len(ToolCalls) = %d, want 1", len(got.ToolCalls))
	}

	toolCall := got.ToolCalls[0]
	if toolCall.Name != "get_weather" {
		t.Errorf("ToolCall.Name = %q, want %q", toolCall.Name, "get_weather")
	}

	expectedInput := map[string]interface{}{"location": "San Francisco"}
	if !reflect.DeepEqual(toolCall.Input, expectedInput) {
		t.Errorf("ToolCall.Input = %v, want %v", toolCall.Input, expectedInput)
	}
}
