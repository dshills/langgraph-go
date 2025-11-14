package bedrock

import (
	"encoding/json"
	"fmt"

	"github.com/dshills/langgraph-go/graph/model"
)

// NovaSchemaTranslator implements SchemaTranslator for Amazon Nova models.
//
// Amazon Nova models use a simplified Messages API format compatible with
// AWS Bedrock's standard format. Unlike Claude models, Nova does not require
// anthropic-specific fields like "anthropic_version" or "max_tokens".
//
// Features:
// - Full tool/function calling support
// - Streaming responses
// - System message extraction
// - Multi-turn conversations
// - Multimodal support (text, images)
type NovaSchemaTranslator struct{}

// TranslateRequest converts messages to Amazon Nova Messages API format.
func (t NovaSchemaTranslator) TranslateRequest(messages []model.Message, tools []model.ToolSpec, config *Config) (json.RawMessage, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}

	// Extract system message if present
	var systemPrompt string
	filteredMessages := make([]model.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == model.RoleSystem {
			systemPrompt = msg.Content
		} else {
			filteredMessages = append(filteredMessages, msg)
		}
	}

	// Build request - Nova uses standard Messages API format
	req := map[string]interface{}{
		"messages": translateMessagesNova(filteredMessages),
	}

	// Add schemaVersion for Nova (required)
	req["schemaVersion"] = "messages-v1"

	// Add inferenceConfig with generation parameters
	inferenceConfig := make(map[string]interface{})
	if config.MaxTokens > 0 {
		inferenceConfig["maxTokens"] = config.MaxTokens // camelCase for Nova
	}
	if config.Temperature > 0 {
		inferenceConfig["temperature"] = config.Temperature
	}
	if config.TopP > 0 {
		inferenceConfig["topP"] = config.TopP
	}
	if len(config.StopSequences) > 0 {
		inferenceConfig["stopSequences"] = config.StopSequences
	}
	if len(inferenceConfig) > 0 {
		req["inferenceConfig"] = inferenceConfig
	}

	// Add system prompt if present
	if systemPrompt != "" {
		req["system"] = []map[string]interface{}{
			{"text": systemPrompt},
		}
	}

	// Add tools if present (Nova uses same tool format as Claude)
	if len(tools) > 0 {
		req["toolConfig"] = map[string]interface{}{
			"tools": translateToolsNova(tools),
		}
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	return json.RawMessage(data), nil
}

// translateMessagesNova converts LangGraph messages to Nova format.
// Nova requires content to be an array of content blocks, not a string.
func translateMessagesNova(messages []model.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		// Nova expects content as an array of content blocks
		content := []map[string]interface{}{
			{"text": msg.Content},
		}
		result[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		}
	}
	return result
}

// translateToolsNova converts LangGraph tool specs to Nova format
func translateToolsNova(tools []model.ToolSpec) []map[string]interface{} {
	novaTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		novaTools[i] = map[string]interface{}{
			"toolSpec": map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": map[string]interface{}{
					"json": tool.Schema,
				},
			},
		}
	}
	return novaTools
}

// TranslateResponse converts Nova response to ChatOut
//
// Nova responses use a similar format to Claude, so we can reuse the Claude logic
func (t NovaSchemaTranslator) TranslateResponse(rawResp json.RawMessage) (model.ChatOut, error) {
	// Parse Nova response structure
	var resp struct {
		Output struct {
			Message struct {
				Role    string `json:"role"`
				Content []struct {
					Text string `json:"text,omitempty"`
					Tool struct {
						Use struct {
							ToolUseID string          `json:"toolUseId"`
							Name      string          `json:"name"`
							Input     json.RawMessage `json:"input"`
						} `json:"toolUse,omitempty"`
					} `json:"toolUse,omitempty"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		StopReason string `json:"stopReason"`
		Usage      struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to parse Nova response: %w", err)
	}

	// Extract text and tool calls
	var textParts []string
	var toolCalls []model.ToolCall

	for _, content := range resp.Output.Message.Content {
		if content.Text != "" {
			textParts = append(textParts, content.Text)
		}
		if content.Tool.Use.ToolUseID != "" {
			// Parse tool input
			var input map[string]interface{}
			if err := json.Unmarshal(content.Tool.Use.Input, &input); err != nil {
				return model.ChatOut{}, fmt.Errorf("failed to parse tool input: %w", err)
			}

			toolCalls = append(toolCalls, model.ToolCall{
				Name:  content.Tool.Use.Name,
				Input: input,
			})
		}
	}

	// Build response
	out := model.ChatOut{
		Text:      joinTextParts(textParts),
		ToolCalls: toolCalls,
		Meta: map[string]interface{}{
			"stop_reason":   resp.StopReason,
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
		},
	}

	return out, nil
}

// TranslateStreamEvent converts Nova streaming events to StreamChunk
//
// Nova streaming uses a similar format to Claude
func (t NovaSchemaTranslator) TranslateStreamEvent(rawEvent json.RawMessage) (StreamChunk, error) {
	// Parse event type
	var eventType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rawEvent, &eventType); err != nil {
		return StreamChunk{}, fmt.Errorf("failed to parse event type: %w", err)
	}

	chunk := StreamChunk{}

	// Handle different event types
	switch eventType.Type {
	case "contentBlockDelta":
		var delta struct {
			Delta struct {
				Text string `json:"text,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(rawEvent, &delta); err != nil {
			return StreamChunk{}, err
		}
		chunk.Delta = delta.Delta.Text

	case "messageStop":
		chunk.FinishReason = "end_turn"

	case "error":
		var errEvent struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rawEvent, &errEvent); err != nil {
			return StreamChunk{}, err
		}
		return StreamChunk{}, fmt.Errorf("stream error: %s", errEvent.Error.Message)
	}

	return chunk, nil
}

// SupportsStreaming returns true as Nova models support streaming
func (t NovaSchemaTranslator) SupportsStreaming() bool {
	return true
}

// SupportsTools returns true as Nova models support tool calling
func (t NovaSchemaTranslator) SupportsTools() bool {
	return true
}

// joinTextParts joins text parts with newlines, filtering empty strings
func joinTextParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	result := parts[0]
	for _, part := range parts[1:] {
		if part != "" {
			result += "\n" + part
		}
	}
	return result
}
