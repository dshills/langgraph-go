package bedrock

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dshills/langgraph-go/graph/model"
)

// ModelFamily identifies the Bedrock model family for schema selection.
//
// Different model families use different request/response formats.
// The adapter selects the appropriate SchemaTranslator based on ModelFamily.
type ModelFamily int

const (
	// ModelFamilyUnknown indicates an unsupported or unrecognized model.
	ModelFamilyUnknown ModelFamily = iota

	// ModelFamilyClaude represents Anthropic Claude models.
	// Format: Anthropic Messages API
	// Features: Full tool support, streaming, system messages
	// Examples: anthropic.claude-3-5-sonnet-20241022-v2:0
	ModelFamilyClaude

	// ModelFamilyLlama represents Meta Llama models.
	// Format: Llama instruction template
	// Features: Text generation only, no tools
	// Examples: meta.llama3-2-90b-instruct-v1:0
	ModelFamilyLlama

	// ModelFamilyTitan represents Amazon Titan models.
	// Format: Titan text generation API
	// Features: Text generation only, no tools, no structured messages
	// Examples: amazon.titan-text-premier-v1:0
	ModelFamilyTitan

	// ModelFamilyMistral represents Mistral models.
	// Format: Mistral instruction template
	// Features: Text generation, limited tool support
	// Examples: mistral.mistral-large-2402-v1:0
	ModelFamilyMistral
)

// String returns the string representation of the ModelFamily.
func (mf ModelFamily) String() string {
	switch mf {
	case ModelFamilyClaude:
		return "Claude"
	case ModelFamilyLlama:
		return "Llama"
	case ModelFamilyTitan:
		return "Titan"
	case ModelFamilyMistral:
		return "Mistral"
	default:
		return "Unknown"
	}
}

// SchemaTranslator defines the interface for translating between LangGraph
// message format and Bedrock model-specific schemas.
//
// Each model family has its own implementation (Claude, Llama, Titan, Mistral).
// Translators are stateless and safe for concurrent use.
type SchemaTranslator interface {
	// TranslateRequest converts LangGraph messages to Bedrock request format.
	//
	// Parameters:
	// - messages: LangGraph conversation history
	// - tools: Optional tool specifications
	// - config: Bedrock configuration for generation parameters
	//
	// Returns:
	// - JSON-encoded request body for Bedrock InvokeModel API
	// - Error if translation fails (e.g., unsupported features)
	TranslateRequest(messages []model.Message, tools []model.ToolSpec, config *Config) (json.RawMessage, error)

	// TranslateResponse converts Bedrock response to LangGraph ChatOut format.
	//
	// Parameters:
	// - response: JSON response from Bedrock InvokeModel API
	//
	// Returns:
	// - ChatOut with extracted text and tool calls
	// - Error if parsing fails
	TranslateResponse(response json.RawMessage) (model.ChatOut, error)

	// TranslateStreamEvent converts a Bedrock streaming event to StreamChunk.
	//
	// Only applicable for models that support streaming (e.g., Claude).
	//
	// Parameters:
	// - event: JSON streaming event from Bedrock InvokeModelWithResponseStream
	//
	// Returns:
	// - StreamChunk with incremental content
	// - Error if parsing fails
	TranslateStreamEvent(event json.RawMessage) (StreamChunk, error)

	// SupportsStreaming returns true if this model family supports streaming responses.
	SupportsStreaming() bool

	// SupportsTools returns true if this model family supports tool/function calling.
	SupportsTools() bool
}

// StreamChunk represents an incremental piece of a streaming response.
//
// Used to assemble streaming responses token-by-token.
type StreamChunk struct {
	// Delta contains incremental text content.
	// Empty for non-text events (e.g., metadata-only events).
	Delta string

	// ToolCallDelta contains incremental tool call data (if applicable).
	// Only used for models that support tools and streaming tool calls.
	ToolCallDelta *ToolCallDelta

	// FinishReason indicates why generation stopped (if this is the final chunk).
	// Values: "end_turn", "max_tokens", "stop_sequence", "tool_use"
	// Empty for intermediate chunks.
	FinishReason string

	// Metadata contains model-specific metadata (token counts, request IDs, etc.).
	Metadata map[string]interface{}
}

// ToolCallDelta represents an incremental piece of a tool call.
//
// Tool calls may be streamed in parts, requiring accumulation before parsing.
type ToolCallDelta struct {
	// Index identifies which tool call this delta belongs to.
	// Multiple tool calls may be streamed in parallel.
	Index int

	// Name is the tool name (only present in first delta for this tool call).
	Name string

	// PartialJSON contains a partial JSON fragment for tool input.
	// Must be accumulated across deltas and parsed when complete.
	PartialJSON string
}

// detectModelFamily determines the model family from a Bedrock model ID.
//
// Supports both direct model IDs and inference profile formats:
// - "anthropic.claude-*" or "us.anthropic.claude-*" → Claude
// - "meta.llama*" or "us.meta.llama*" → Llama
// - "amazon.titan-*" or "us.amazon.titan-*" → Titan
// - "mistral.*" or "us.mistral.*" → Mistral
//
// Inference profiles (e.g., "us.anthropic.claude-*") enable cross-region
// routing and are required in some AWS accounts for on-demand throughput.
//
// Returns ModelFamilyUnknown for unsupported or malformed model IDs.
func detectModelFamily(modelID string) ModelFamily {
	if len(modelID) == 0 {
		return ModelFamilyUnknown
	}

	// Check prefixes for direct model IDs
	if hasPrefix(modelID, "anthropic.claude") {
		return ModelFamilyClaude
	}
	if hasPrefix(modelID, "meta.llama") {
		return ModelFamilyLlama
	}
	if hasPrefix(modelID, "amazon.titan") {
		return ModelFamilyTitan
	}
	if hasPrefix(modelID, "mistral.") {
		return ModelFamilyMistral
	}

	// Check for inference profile format: us.anthropic.claude-*, eu.anthropic.claude-*, etc.
	if strings.Contains(modelID, "anthropic.claude") {
		return ModelFamilyClaude
	}
	if strings.Contains(modelID, "meta.llama") {
		return ModelFamilyLlama
	}
	if strings.Contains(modelID, "amazon.titan") {
		return ModelFamilyTitan
	}
	if strings.Contains(modelID, "mistral.") {
		return ModelFamilyMistral
	}

	return ModelFamilyUnknown
}

// hasPrefix is a helper to check string prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// ClaudeSchemaTranslator implements SchemaTranslator for Anthropic Claude models.
//
// Uses the Anthropic Messages API format as documented in:
// specs/008-bedrock-llm-support/contracts/bedrock-claude-request.json
//
// Features:
// - Full tool/function calling support
// - Streaming responses
// - System message extraction
// - Multi-turn conversations
type ClaudeSchemaTranslator struct{}

// TranslateRequest converts messages to Claude Messages API format.
func (t ClaudeSchemaTranslator) TranslateRequest(messages []model.Message, tools []model.ToolSpec, config *Config) (json.RawMessage, error) {
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

	// Build request
	req := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        config.MaxTokens,
		"messages":          translateMessages(filteredMessages),
	}

	// Add system prompt if present
	if systemPrompt != "" {
		req["system"] = systemPrompt
	}

	// Add temperature if set
	if config.Temperature > 0 {
		req["temperature"] = config.Temperature
	}

	// Add top_p if set
	if config.TopP > 0 {
		req["top_p"] = config.TopP
	}

	// Add stop sequences if present
	if len(config.StopSequences) > 0 {
		req["stop_sequences"] = config.StopSequences
	}

	// Add tools if present
	if len(tools) > 0 {
		req["tools"] = translateTools(tools)
	}

	// Set default max_tokens if not configured
	if config.MaxTokens == 0 {
		req["max_tokens"] = 4096
	}

	return json.Marshal(req)
}

// translateMessages converts LangGraph messages to Claude message format.
func translateMessages(messages []model.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}
	return result
}

// translateTools converts LangGraph tool specs to Claude tool format.
func translateTools(tools []model.ToolSpec) []map[string]interface{} {
	result := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		result[i] = map[string]interface{}{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.Schema,
		}
	}
	return result
}

// TranslateResponse parses Claude response to ChatOut.
func (t ClaudeSchemaTranslator) TranslateResponse(response json.RawMessage) (model.ChatOut, error) {
	var resp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type  string                 `json:"type"`
			Text  string                 `json:"text,omitempty"`
			ID    string                 `json:"id,omitempty"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(response, &resp); err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to parse Claude response: %w", err)
	}

	out := model.ChatOut{
		Meta: make(map[string]interface{}),
	}

	// Extract text content
	var textParts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}
	if len(textParts) > 0 {
		out.Text = joinStrings(textParts, "")
	}

	// Extract tool calls
	for _, block := range resp.Content {
		if block.Type == "tool_use" {
			out.ToolCalls = append(out.ToolCalls, model.ToolCall{
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	// Populate metadata from Claude response
	if resp.ID != "" {
		out.Meta["request_id"] = resp.ID
	}
	if resp.Model != "" {
		out.Meta["model"] = resp.Model
	}
	if resp.StopReason != "" {
		out.Meta["stop_reason"] = resp.StopReason
	}
	if resp.Usage.InputTokens > 0 {
		out.Meta["input_tokens"] = resp.Usage.InputTokens
	}
	if resp.Usage.OutputTokens > 0 {
		out.Meta["output_tokens"] = resp.Usage.OutputTokens
	}

	return out, nil
}

// joinStrings concatenates strings with a separator.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// TranslateStreamEvent parses Claude streaming events.
//
// Handles 7 Claude streaming event types:
//   - message_start: Initial metadata (request_id, model, input_tokens)
//   - content_block_start: Start of text or tool_use block
//   - content_block_delta: Incremental content (text_delta or input_json_delta)
//   - content_block_stop: End of content block (no-op)
//   - message_delta: Final metadata with stop_reason and output_tokens
//   - message_stop: Stream complete (no-op)
//   - error: Error event (returns error)
//
// Returns StreamChunk with populated fields based on event type.
func (t ClaudeSchemaTranslator) TranslateStreamEvent(event json.RawMessage) (StreamChunk, error) {
	// Parse event to determine type
	var baseEvent struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(event, &baseEvent); err != nil {
		return StreamChunk{}, fmt.Errorf("failed to parse streaming event: %w", err)
	}

	chunk := StreamChunk{
		Metadata: make(map[string]interface{}),
	}

	switch baseEvent.Type {
	case "message_start":
		// Extract initial metadata
		var msgStart struct {
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(event, &msgStart); err != nil {
			return StreamChunk{}, fmt.Errorf("failed to parse message_start event: %w", err)
		}

		chunk.Metadata["request_id"] = msgStart.Message.ID
		chunk.Metadata["model"] = msgStart.Message.Model
		chunk.Metadata["input_tokens"] = msgStart.Message.Usage.InputTokens

	case "content_block_start":
		// Extract tool name if this is a tool_use block
		var blockStart struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				Name string `json:"name,omitempty"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal(event, &blockStart); err != nil {
			return StreamChunk{}, fmt.Errorf("failed to parse content_block_start event: %w", err)
		}

		if blockStart.ContentBlock.Type == "tool_use" {
			chunk.ToolCallDelta = &ToolCallDelta{
				Index: blockStart.Index,
				Name:  blockStart.ContentBlock.Name,
			}
		}

	case "content_block_delta":
		// Extract text delta or tool input delta
		var blockDelta struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(event, &blockDelta); err != nil {
			return StreamChunk{}, fmt.Errorf("failed to parse content_block_delta event: %w", err)
		}

		if blockDelta.Delta.Type == "text_delta" {
			chunk.Delta = blockDelta.Delta.Text
		} else if blockDelta.Delta.Type == "input_json_delta" {
			chunk.ToolCallDelta = &ToolCallDelta{
				Index:       blockDelta.Index,
				PartialJSON: blockDelta.Delta.PartialJSON,
			}
		}

	case "content_block_stop":
		// No-op: content block finished, no additional data needed
		return chunk, nil

	case "message_delta":
		// Extract stop reason and output token count
		var msgDelta struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(event, &msgDelta); err != nil {
			return StreamChunk{}, fmt.Errorf("failed to parse message_delta event: %w", err)
		}

		chunk.FinishReason = msgDelta.Delta.StopReason
		chunk.Metadata["output_tokens"] = msgDelta.Usage.OutputTokens

	case "message_stop":
		// No-op: stream complete, no additional data needed
		return chunk, nil

	case "error":
		// Extract error and return as error
		var errEvent struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(event, &errEvent); err != nil {
			return StreamChunk{}, fmt.Errorf("failed to parse error event: %w", err)
		}

		return StreamChunk{}, fmt.Errorf("streaming error [%s]: %s",
			errEvent.Error.Type, errEvent.Error.Message)

	default:
		// Unknown event type - return empty chunk (graceful degradation)
		return chunk, nil
	}

	return chunk, nil
}

// SupportsStreaming returns true (Claude supports streaming).
func (t ClaudeSchemaTranslator) SupportsStreaming() bool {
	return true
}

// SupportsTools returns true (Claude supports tool calling).
func (t ClaudeSchemaTranslator) SupportsTools() bool {
	return true
}
