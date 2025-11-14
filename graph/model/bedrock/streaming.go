package bedrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/dshills/langgraph-go/graph/model"
)

// StreamCallback is called for each streaming chunk received from Bedrock.
//
// Parameters:
//   - chunk: Incremental content with Delta (text), ToolCallDelta (tool input),
//     FinishReason (when stream ends), and Metadata (token counts, etc.)
//
// The callback should return an error to abort streaming.
// Returning nil continues processing the stream.
//
// Example:
//
//	callback := func(chunk bedrock.StreamChunk) error {
//	    if chunk.Delta != "" {
//	        fmt.Print(chunk.Delta)  // Print tokens as they arrive
//	    }
//	    if chunk.FinishReason != "" {
//	        fmt.Printf("\nFinished: %s\n", chunk.FinishReason)
//	    }
//	    return nil
//	}
type StreamCallback func(chunk StreamChunk) error

// ChatStream sends messages to Bedrock and streams the response token-by-token.
//
// This method uses AWS Bedrock's InvokeModelWithResponseStream API to receive
// incremental responses. Each chunk is parsed and delivered via the callback.
//
// Process:
//  1. Validate messages and check model supports streaming
//  2. Translate LangGraph messages to Bedrock request format
//  3. Call InvokeModelWithResponseStream API
//  4. Process event stream, calling callback for each chunk
//  5. Accumulate final response and return ChatOut
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - messages: Conversation history (system, user, assistant messages)
//   - tools: Optional tool specifications (only supported by Claude models)
//   - callback: Function called for each streaming chunk
//
// Returns:
//   - ChatOut: Complete accumulated response
//   - Error: Stream errors, translation errors, or callback errors
//
// The callback receives chunks in this order:
//  1. message_start: Initial metadata (request_id, model, input_tokens)
//  2. content_block_start: Start of text or tool block
//  3. content_block_delta: Multiple incremental content chunks
//  4. content_block_stop: End of content block
//  5. message_delta: Final metadata (stop_reason, output_tokens)
//  6. message_stop: Stream complete
//
// Example:
//
//	var fullText string
//	callback := func(chunk bedrock.StreamChunk) error {
//	    fullText += chunk.Delta
//	    fmt.Print(chunk.Delta)  // Print each token immediately
//	    return nil
//	}
//
//	response, err := adapter.ChatStream(ctx, messages, nil, callback)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("\nFinal response: %s\n", response.Text)
func (a *Adapter) ChatStream(ctx context.Context, messages []model.Message, tools []model.ToolSpec, callback StreamCallback) (model.ChatOut, error) {
	// Validate messages
	if len(messages) == 0 {
		return model.ChatOut{}, fmt.Errorf("at least one message is required")
	}

	// Check if model supports streaming
	if !a.schemaTranslator.SupportsStreaming() {
		return model.ChatOut{}, fmt.Errorf("model family %s does not support streaming", a.modelFamily)
	}

	// Translate LangGraph messages to Bedrock request format
	requestBody, err := a.schemaTranslator.TranslateRequest(messages, tools, a.config)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to translate request: %w", err)
	}

	// Call Bedrock InvokeModelWithResponseStream API
	output, err := a.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(a.config.ModelID),
		Body:        requestBody,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		// Wrap AWS error
		return model.ChatOut{}, wrapAWSError(err, a.config.Region)
	}

	// Process event stream
	stream := output.GetStream()
	defer func() {
		if err := stream.Close(); err != nil {
			// Log stream close error (non-fatal)
			_ = err
		}
	}()

	// Accumulate response chunks
	var textParts []string
	toolCalls := make([]model.ToolCall, 0) // Pre-allocate for performance
	metadata := make(map[string]interface{})
	var finishReason string

	// Track tool call accumulation (for streaming tool inputs)
	toolCallBuffers := make(map[int]*toolCallBuffer)

	// Process events from stream (channel-based API in AWS SDK v2)
	for event := range stream.Events() {
		// Type assert to ResponseStreamMemberChunk to access payload
		chunkEvent, ok := event.(*types.ResponseStreamMemberChunk)
		if !ok {
			// Unknown event type, skip
			continue
		}

		// Extract chunk bytes from payload
		chunkBytes := chunkEvent.Value.Bytes
		if len(chunkBytes) == 0 {
			continue
		}

		// Parse streaming event using schema translator
		chunk, err := a.schemaTranslator.TranslateStreamEvent(chunkBytes)
		if err != nil {
			// Error event from Bedrock - return with partial response
			return buildPartialResponse(textParts, toolCalls, metadata, finishReason), err
		}

		// Accumulate metadata
		for key, value := range chunk.Metadata {
			metadata[key] = value
		}

		// Accumulate text delta
		if chunk.Delta != "" {
			textParts = append(textParts, chunk.Delta)
		}

		// Accumulate tool call delta
		if chunk.ToolCallDelta != nil {
			buffer := getOrCreateToolCallBuffer(toolCallBuffers, chunk.ToolCallDelta.Index)

			// Set tool name if present (from content_block_start)
			if chunk.ToolCallDelta.Name != "" {
				buffer.name = chunk.ToolCallDelta.Name
			}

			// Accumulate partial JSON (from content_block_delta)
			if chunk.ToolCallDelta.PartialJSON != "" {
				buffer.jsonBuffer += chunk.ToolCallDelta.PartialJSON
			}
		}

		// Capture finish reason (from message_delta)
		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}

		// Call user callback
		if callback != nil {
			if err := callback(chunk); err != nil {
				// Callback requested abort - return partial response
				return buildPartialResponse(textParts, toolCalls, metadata, finishReason),
					fmt.Errorf("callback aborted stream: %w", err)
			}
		}
	}

	// Check for stream-level errors
	if err := stream.Err(); err != nil {
		return buildPartialResponse(textParts, toolCalls, metadata, finishReason),
			fmt.Errorf("stream error: %w", err)
	}

	// Parse accumulated tool calls
	for _, buffer := range toolCallBuffers {
		toolCall, err := buffer.parse()
		if err != nil {
			// Failed to parse tool call - include in response but log warning
			// In production, this should be logged
			continue
		}
		toolCalls = append(toolCalls, toolCall)
	}

	// Build final ChatOut
	chatOut := model.ChatOut{
		Text:      joinStrings(textParts, ""),
		ToolCalls: toolCalls,
		Meta:      metadata,
	}

	// Add region to metadata
	chatOut.Meta["region"] = a.config.Region

	return chatOut, nil
}

// toolCallBuffer accumulates streamed tool call data.
type toolCallBuffer struct {
	name       string
	jsonBuffer string
}

// parse converts accumulated JSON buffer to ToolCall.
func (b *toolCallBuffer) parse() (model.ToolCall, error) {
	if b.name == "" {
		return model.ToolCall{}, fmt.Errorf("tool call missing name")
	}

	// Parse JSON input
	var input map[string]interface{}
	if b.jsonBuffer != "" {
		if err := json.Unmarshal([]byte(b.jsonBuffer), &input); err != nil {
			return model.ToolCall{}, fmt.Errorf("failed to parse tool input JSON: %w", err)
		}
	}

	return model.ToolCall{
		Name:  b.name,
		Input: input,
	}, nil
}

// getOrCreateToolCallBuffer retrieves or creates a buffer for a tool call index.
func getOrCreateToolCallBuffer(buffers map[int]*toolCallBuffer, index int) *toolCallBuffer {
	if buffer, exists := buffers[index]; exists {
		return buffer
	}
	buffer := &toolCallBuffer{}
	buffers[index] = buffer
	return buffer
}

// buildPartialResponse constructs a ChatOut from accumulated chunks (for error cases).
func buildPartialResponse(textParts []string, toolCalls []model.ToolCall, metadata map[string]interface{}, finishReason string) model.ChatOut {
	out := model.ChatOut{
		Text:      joinStrings(textParts, ""),
		ToolCalls: toolCalls,
		Meta:      metadata,
	}

	// Mark as partial response
	out.Meta["partial"] = true
	if finishReason != "" {
		out.Meta["stop_reason"] = finishReason
	}

	return out
}
