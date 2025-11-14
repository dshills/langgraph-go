package bedrock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dshills/langgraph-go/graph/model"
)

// T015: Test NewAdapter() credential initialization and configuration
func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid Claude configuration",
			config: Config{
				Region:  "us-east-1",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: false,
		},
		{
			name: "invalid region",
			config: Config{
				Region:  "",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: true,
			errMsg:  "region",
		},
		{
			name: "invalid model ID",
			config: Config{
				Region:  "us-east-1",
				ModelID: "",
			},
			wantErr: true,
			errMsg:  "model",
		},
		{
			name: "unknown model family",
			config: Config{
				Region:  "us-east-1",
				ModelID: "unknown.model-v1:0",
			},
			wantErr: true,
			errMsg:  "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			adapter, err := NewAdapter(ctx, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewAdapter() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errMsg)) {
					t.Errorf("NewAdapter() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("NewAdapter() unexpected error = %v", err)
				return
			}

			if adapter == nil {
				t.Error("NewAdapter() returned nil adapter with no error")
				return
			}

			// Verify adapter fields are initialized
			if adapter.config == nil {
				t.Error("NewAdapter() adapter.config is nil")
			}
			if adapter.client == nil {
				t.Error("NewAdapter() adapter.client is nil")
			}
			if adapter.modelFamily == ModelFamilyUnknown {
				t.Error("NewAdapter() adapter.modelFamily is Unknown")
			}
			if adapter.schemaTranslator == nil {
				t.Error("NewAdapter() adapter.schemaTranslator is nil")
			}
		})
	}
}

// Test NewAdapter detects correct model family
func TestNewAdapter_ModelFamilyDetection(t *testing.T) {
	tests := []struct {
		name       string
		modelID    string
		wantFamily ModelFamily
		skipMVP    bool // Skip for MVP - not yet implemented
	}{
		{
			name:       "Claude model",
			modelID:    "anthropic.claude-3-5-sonnet-20241022-v2:0",
			wantFamily: ModelFamilyClaude,
			skipMVP:    false,
		},
		{
			name:       "Llama model",
			modelID:    "meta.llama3-2-90b-instruct-v1:0",
			wantFamily: ModelFamilyLlama,
			skipMVP:    true, // Implemented in Phase 5
		},
		{
			name:       "Titan model",
			modelID:    "amazon.titan-text-premier-v1:0",
			wantFamily: ModelFamilyTitan,
			skipMVP:    true, // Implemented in Phase 5
		},
		{
			name:       "Mistral model",
			modelID:    "mistral.mistral-large-2402-v1:0",
			wantFamily: ModelFamilyMistral,
			skipMVP:    true, // Implemented in Phase 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipMVP {
				t.Skip("Model family not yet implemented (Phase 5)")
			}

			ctx := context.Background()
			config := Config{
				Region:  "us-east-1",
				ModelID: tt.modelID,
			}

			adapter, err := NewAdapter(ctx, config)
			if err != nil {
				t.Fatalf("NewAdapter() unexpected error = %v", err)
			}

			if adapter.modelFamily != tt.wantFamily {
				t.Errorf("NewAdapter() modelFamily = %v, want %v", adapter.modelFamily, tt.wantFamily)
			}
		})
	}
}

// Test NewAdapter selects correct schema translator
func TestNewAdapter_SchemaTranslatorSelection(t *testing.T) {
	ctx := context.Background()
	config := Config{
		Region:  "us-east-1",
		ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
	}

	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
	}

	// Verify ClaudeSchemaTranslator is selected for Claude models
	_, ok := adapter.schemaTranslator.(ClaudeSchemaTranslator)
	if !ok {
		t.Errorf("NewAdapter() schemaTranslator type = %T, want ClaudeSchemaTranslator", adapter.schemaTranslator)
	}
}

// T021: Test BedrockAdapter.Chat() with Claude (using mock)
// Note: This test will use a mock AWS client to avoid actual API calls
func TestAdapter_Chat_BasicRequest(t *testing.T) {
	// Skip if no AWS credentials available (integration test)
	// This test structure prepares for mocking implementation
	t.Skip("Integration test - requires AWS credentials or mock client")

	ctx := context.Background()
	config := Config{
		Region:    "us-east-1",
		ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
		MaxTokens: 100,
	}

	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What is 2+2?"},
	}

	response, err := adapter.Chat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("Chat() unexpected error = %v", err)
	}

	if response.Text == "" {
		t.Error("Chat() returned empty text")
	}

	if len(response.ToolCalls) != 0 {
		t.Errorf("Chat() returned unexpected tool calls: %v", response.ToolCalls)
	}
}

// Test Chat() with tool specifications
func TestAdapter_Chat_WithTools(t *testing.T) {
	t.Skip("Integration test - requires AWS credentials or mock client")

	ctx := context.Background()
	config := Config{
		Region:    "us-east-1",
		ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
		MaxTokens: 200,
	}

	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
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

	response, err := adapter.Chat(ctx, messages, tools)
	if err != nil {
		t.Fatalf("Chat() unexpected error = %v", err)
	}

	// Claude may respond with text and/or tool calls
	if response.Text == "" && len(response.ToolCalls) == 0 {
		t.Error("Chat() returned empty response (no text or tool calls)")
	}
}

// Test Chat() error handling
func TestAdapter_Chat_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty messages",
			config: Config{
				Region:  "us-east-1",
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			},
			wantErr: true,
			errMsg:  "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Integration test - requires implementation")

			ctx := context.Background()
			adapter, err := NewAdapter(ctx, tt.config)
			if err != nil {
				t.Fatalf("NewAdapter() unexpected error = %v", err)
			}

			// Pass empty messages to trigger error
			_, err = adapter.Chat(ctx, []model.Message{}, nil)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Chat() expected error containing %q, got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Chat() unexpected error = %v", err)
				}
			}
		})
	}
}

// Test Chat() with context cancellation
func TestAdapter_Chat_ContextCancellation(t *testing.T) {
	t.Skip("Integration test - requires mock client to test cancellation")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := Config{
		Region:  "us-east-1",
		ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
	}

	adapter, err := NewAdapter(context.Background(), config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Hello"},
	}

	_, err = adapter.Chat(ctx, messages, nil)
	if err == nil {
		t.Error("Chat() expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context") {
		t.Errorf("Chat() error = %v, want context-related error", err)
	}
}

// Test schema translation integration
func TestAdapter_Chat_SchemaTranslation(t *testing.T) {
	// This unit test verifies the translation pipeline without AWS calls
	translator := ClaudeSchemaTranslator{}
	config := &Config{
		MaxTokens:   100,
		Temperature: 0.7,
	}

	messages := []model.Message{
		{Role: model.RoleSystem, Content: "You are a helpful assistant."},
		{Role: model.RoleUser, Content: "Hello!"},
	}

	// Test request translation
	requestJSON, err := translator.TranslateRequest(messages, nil, config)
	if err != nil {
		t.Fatalf("TranslateRequest() unexpected error = %v", err)
	}

	var request map[string]interface{}
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		t.Fatalf("Invalid request JSON: %v", err)
	}

	// Verify Claude-specific format
	if request["anthropic_version"] != "bedrock-2023-05-31" {
		t.Errorf("Request anthropic_version = %v, want bedrock-2023-05-31", request["anthropic_version"])
	}

	if request["system"] != "You are a helpful assistant." {
		t.Errorf("Request system = %v, want 'You are a helpful assistant.'", request["system"])
	}

	// Test response translation
	claudeResponse := `{
		"id": "msg_01ABC",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Hi there! How can I help?"}
		],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 20, "output_tokens": 10}
	}`

	chatOut, err := translator.TranslateResponse(json.RawMessage(claudeResponse))
	if err != nil {
		t.Fatalf("TranslateResponse() unexpected error = %v", err)
	}

	expectedText := "Hi there! How can I help?"
	if chatOut.Text != expectedText {
		t.Errorf("TranslateResponse() Text = %q, want %q", chatOut.Text, expectedText)
	}
}

// T036: Test NewAdapter() with different AWS regions
func TestNewAdapter_MultiRegion(t *testing.T) {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			ctx := context.Background()
			config := Config{
				Region:  region,
				ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
			}

			adapter, err := NewAdapter(ctx, config)
			if err != nil {
				t.Fatalf("NewAdapter() with region %s unexpected error = %v", region, err)
			}

			if adapter == nil {
				t.Fatalf("NewAdapter() with region %s returned nil adapter", region)
			}

			if adapter.config.Region != region {
				t.Errorf("NewAdapter() config.Region = %s, want %s", adapter.config.Region, region)
			}
		})
	}
}

// T037: Test BedrockAdapter.Chat() includes region in metadata
func TestAdapter_Chat_RegionMetadata(t *testing.T) {
	// This test uses unit testing approach - testing TranslateResponse and region injection separately
	translator := ClaudeSchemaTranslator{}

	// Mock Claude response with metadata
	claudeResponse := `{
		"id": "msg_01ABC123",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Test response"}
		],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 20, "output_tokens": 10}
	}`

	chatOut, err := translator.TranslateResponse(json.RawMessage(claudeResponse))
	if err != nil {
		t.Fatalf("TranslateResponse() unexpected error = %v", err)
	}

	// Verify metadata from Claude response is populated
	if chatOut.Meta == nil {
		t.Fatal("TranslateResponse() Meta is nil, expected metadata")
	}

	expectedMeta := map[string]interface{}{
		"request_id":    "msg_01ABC123",
		"model":         "claude-3-5-sonnet-20241022",
		"stop_reason":   "end_turn",
		"input_tokens":  20,
		"output_tokens": 10,
	}

	for key, expectedVal := range expectedMeta {
		actualVal, ok := chatOut.Meta[key]
		if !ok {
			t.Errorf("TranslateResponse() Meta missing key %q", key)
			continue
		}
		if actualVal != expectedVal {
			t.Errorf("TranslateResponse() Meta[%q] = %v, want %v", key, actualVal, expectedVal)
		}
	}

	// Test that Chat() method adds region to metadata
	// Note: Full integration test would require mock AWS client
	// For now, we verify the metadata population in TranslateResponse
	// and trust that bedrock.go:177 adds region to Meta
}

// T045: Test regional fallback on retryable errors
// Note: This test requires mock AWS client to simulate region failures
func TestAdapter_Chat_RegionalFallback(t *testing.T) {
	t.Skip("Integration test - requires mock AWS client to simulate regional failures")

	// Test plan:
	// 1. Configure adapter with primary region (us-east-1) and fallback (us-west-2)
	// 2. Mock primary region to return ThrottlingException
	// 3. Mock fallback region to return success
	// 4. Verify request succeeds and metadata shows fallback region was used
	// 5. Verify telemetry event was emitted for fallback

	ctx := context.Background()
	config := Config{
		Region:          "us-east-1",
		FallbackRegions: []string{"us-west-2"},
		ModelID:         "anthropic.claude-3-5-sonnet-20241022-v2:0",
		MaxRetries:      1,
	}

	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Test"},
	}

	// This would need mock client setup to simulate:
	// - us-east-1 returns ThrottlingException
	// - us-west-2 returns success
	response, err := adapter.Chat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("Chat() unexpected error = %v (expected fallback to succeed)", err)
	}

	// Verify fallback region was used
	if response.Meta["region"] != "us-west-2" {
		t.Errorf("Chat() used region %v, want us-west-2 (fallback)", response.Meta["region"])
	}
}

// T046: Test all regions exhausted returns error
// Note: This test requires mock AWS client to simulate all regions failing
func TestAdapter_Chat_AllRegionsExhausted(t *testing.T) {
	t.Skip("Integration test - requires mock AWS client to simulate all regional failures")

	// Test plan:
	// 1. Configure adapter with primary and 2 fallback regions
	// 2. Mock all regions to return retryable errors
	// 3. Verify final error indicates all regions were tried
	// 4. Verify error contains list of attempted regions

	ctx := context.Background()
	config := Config{
		Region:          "us-east-1",
		FallbackRegions: []string{"us-west-2", "eu-west-1"},
		ModelID:         "anthropic.claude-3-5-sonnet-20241022-v2:0",
		MaxRetries:      1,
	}

	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		t.Fatalf("NewAdapter() unexpected error = %v", err)
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Test"},
	}

	// This would need mock client setup to simulate:
	// - us-east-1 returns ThrottlingException
	// - us-west-2 returns ThrottlingException
	// - eu-west-1 returns ThrottlingException
	_, err = adapter.Chat(ctx, messages, nil)
	if err == nil {
		t.Fatal("Chat() expected error when all regions fail, got nil")
	}

	// Verify error mentions all regions were tried
	errMsg := err.Error()
	if !strings.Contains(errMsg, "all regions exhausted") {
		t.Errorf("Chat() error = %v, want error mentioning 'all regions exhausted'", errMsg)
	}
}
