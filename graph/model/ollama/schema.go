// Package ollama internal schema translation documentation.
//
// This file documents the translation strategy between LangGraph's model types
// and Ollama SDK types. Since we use the official github.com/ollama/ollama/api
// SDK, the Ollama types are provided by the SDK and not defined here.
package ollama

// Translation Overview
//
// The ollama adapter translates between LangGraph's generic model types and
// Ollama's API-specific types. All translation happens in ollama.go using the
// helper functions documented here.
//
// # Message Translation
//
// LangGraph model.Message → Ollama api.Message:
//
//	model.Message{
//	    Role:    "user",           →  api.Message{
//	    Content: "Hello",              Role:    "user",
//	}                                  Content: "Hello",
//	                               }
//
// Ollama api.Message → LangGraph model.Message:
//
//	api.Message{
//	    Role:    "assistant",      →  model.Message{
//	    Content: "Hi there",           Role:    "assistant",
//	}                                  Content: "Hi there",
//	                               }
//
// # Role Mapping
//
// LangGraph and Ollama use the same role constants:
//   - "system"    → "system"     (system instructions)
//   - "user"      → "user"       (user messages)
//   - "assistant" → "assistant"  (model responses)
//
// Direct 1:1 mapping with no transformation required.
//
// # Tool Translation
//
// LangGraph model.ToolSpec → Ollama api.Tool:
//
//	model.ToolSpec{
//	    Name:        "get_weather",
//	    Description: "Get weather",
//	    Schema: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "location": map[string]interface{}{
//	                "type": "string",
//	                "description": "City",
//	            },
//	        },
//	        "required": []string{"location"},
//	    },
//	}
//
// Translates to:
//
//	api.Tool{
//	    Type: "function",
//	    Function: api.ToolFunction{
//	        Name:        "get_weather",
//	        Description: "Get weather",
//	        Parameters: api.ToolFunctionParameters{
//	            Type:       "object",
//	            Properties: map[string]api.ToolProperty{
//	                "location": {
//	                    Type:        "string",
//	                    Description: "City",
//	                },
//	            },
//	            Required: []string{"location"},
//	        },
//	    },
//	}
//
// # Tool Call Translation
//
// Ollama api.ToolCall → LangGraph model.ToolCall:
//
//	api.ToolCall{
//	    Function: api.ToolCallFunction{
//	        Name: "get_weather",
//	        Arguments: api.ToolCallFunctionArguments{
//	            "location": "Paris",
//	        },
//	    },
//	}
//
// Translates to:
//
//	model.ToolCall{
//	    Name: "get_weather",
//	    Input: map[string]interface{}{
//	        "location": "Paris",
//	    },
//	}
//
// # Response Translation
//
// Ollama api.ChatResponse → LangGraph model.ChatOut:
//
//	api.ChatResponse{
//	    Message: api.Message{
//	        Role:    "assistant",
//	        Content: "The weather in Paris is sunny.",
//	    },
//	    Model:     "gpt-oss",
//	    CreatedAt: time.Now(),
//	    Done:      true,
//	    Metrics: api.Metrics{
//	        TotalDuration:   123456789,
//	        PromptEvalCount: 10,
//	    },
//	}
//
// Translates to:
//
//	model.ChatOut{
//	    Text: "The weather in Paris is sunny.",
//	    Meta: map[string]interface{}{
//	        "model":        "gpt-oss",
//	        "created_at":   time.Now(),
//	        "done":         true,
//	        "total_duration": time.Duration(123456789),
//	        "prompt_eval_count": 10,
//	    },
//	}
//
// With tool calls:
//
//	api.ChatResponse{
//	    Message: api.Message{
//	        Role:    "assistant",
//	        Content: "Let me check the weather.",
//	        ToolCalls: []api.ToolCall{
//	            {
//	                Function: api.ToolCallFunction{
//	                    Name: "get_weather",
//	                    Arguments: api.ToolCallFunctionArguments{
//	                        "location": "Paris",
//	                    },
//	                },
//	            },
//	        },
//	    },
//	}
//
// Translates to:
//
//	model.ChatOut{
//	    Text: "Let me check the weather.",
//	    ToolCalls: []model.ToolCall{
//	        {
//	            Name: "get_weather",
//	            Input: map[string]interface{}{
//	                "location": "Paris",
//	            },
//	        },
//	    },
//	}
//
// # Translation Functions
//
// The following functions in ollama.go implement these translations:
//
//   - convertMessages([]model.Message) → []api.Message
//     Converts LangGraph messages to Ollama format for requests
//
//   - convertTools([]model.ToolSpec) → api.Tools
//     Converts LangGraph tool specs to Ollama format for requests
//
//   - convertResponse(*api.ChatResponse) → model.ChatOut
//     Converts Ollama response to LangGraph format
//
//   - convertToolCalls([]api.ToolCall) → []model.ToolCall
//     Converts Ollama tool calls to LangGraph format
//
//   - convertToolProperties(map[string]interface{}) → map[string]api.ToolProperty
//     Converts JSON schema properties to Ollama ToolProperty format
//
// # Schema Property Translation
//
// JSON Schema properties in ToolSpec.Schema["properties"] must be converted
// to Ollama's api.ToolProperty format.
//
// Simple property:
//
//	map[string]interface{}{
//	    "location": map[string]interface{}{
//	        "type":        "string",
//	        "description": "City name",
//	    },
//	}
//
// Converts to:
//
//	map[string]api.ToolProperty{
//	    "location": {
//	        Type:        "string",
//	        Description: "City name",
//	    },
//	}
//
// Complex property with enum:
//
//	map[string]interface{}{
//	    "units": map[string]interface{}{
//	        "type":        "string",
//	        "description": "Temperature units",
//	        "enum":        []interface{}{"celsius", "fahrenheit"},
//	    },
//	}
//
// Converts to:
//
//	map[string]api.ToolProperty{
//	    "units": {
//	        Type:        "string",
//	        Description: "Temperature units",
//	        Enum:        []interface{}{"celsius", "fahrenheit"},
//	    },
//	}
//
// Array property:
//
//	map[string]interface{}{
//	    "locations": map[string]interface{}{
//	        "type":  "array",
//	        "items": map[string]interface{}{"type": "string"},
//	    },
//	}
//
// Converts to:
//
//	map[string]api.ToolProperty{
//	    "locations": {
//	        Type:  "array",
//	        Items: map[string]interface{}{"type": "string"},
//	    },
//	}
//
// # Error Handling
//
// Translation errors are handled by:
//   - Validation: Check for required fields before translation
//   - Fallbacks: Use sensible defaults when optional fields are missing
//   - Type assertions: Safely extract nested values with ok-checks
//   - Error wrapping: Wrap translation errors with context
//
// Example validation in convertTools():
//
//	if tool.Name == "" {
//	    return nil, fmt.Errorf("tool name is required")
//	}
//
// Example type assertion in convertToolProperties():
//
//	propMap, ok := val.(map[string]interface{})
//	if !ok {
//	    // Skip invalid property, log warning
//	    continue
//	}
//
// # Metadata Preservation
//
// Ollama provides rich metadata in ChatResponse that we preserve in
// ChatOut.Meta for observability:
//
//   - model: Model name that generated the response
//   - remote_model: Upstream model name (for proxied requests)
//   - remote_host: Upstream Ollama host URL
//   - created_at: Response timestamp
//   - done: Whether response is complete (always true for non-streaming)
//   - done_reason: Why generation stopped (e.g., "stop", "length")
//   - total_duration: Total request time (nanoseconds → time.Duration)
//   - load_duration: Model load time (nanoseconds → time.Duration)
//   - prompt_eval_count: Number of prompt tokens evaluated
//   - prompt_eval_duration: Time to evaluate prompt
//   - eval_count: Number of generated tokens
//   - eval_duration: Time to generate tokens
//
// This metadata enables:
//   - Performance monitoring (latency, throughput)
//   - Cost tracking (token counts)
//   - Debugging (model routing, stop reasons)
//   - Audit logging (timestamps, model versions)
//
// # Special Cases
//
// ## Empty Content
// Ollama allows empty content when tool calls are present:
//
//	api.Message{
//	    Role:      "assistant",
//	    Content:   "",  // Empty content is valid
//	    ToolCalls: []api.ToolCall{...},
//	}
//
// We preserve this:
//
//	model.ChatOut{
//	    Text:      "",  // Empty text
//	    ToolCalls: []model.ToolCall{...},
//	}
//
// ## System Message Handling
// Unlike Anthropic (which uses separate system parameter), Ollama accepts
// system messages in the messages array directly:
//
//	[]api.Message{
//	    {Role: "system", Content: "You are helpful."},
//	    {Role: "user", Content: "Hello"},
//	}
//
// No special extraction or handling needed - pass through as-is.
//
// ## Streaming Responses
// For streaming (handled in ollama.go ChatStream method):
//   - Accumulate text from multiple partial responses
//   - Tool calls appear in final response only
//   - Done=true marks the final response
//   - Merge metadata from final response
//
// ## Image Support
// Ollama supports images in messages via Images field:
//
//	api.Message{
//	    Role:    "user",
//	    Content: "What's in this image?",
//	    Images:  []api.ImageData{...},
//	}
//
// Currently not supported in LangGraph model.Message.
// If needed, images could be added to model.Message or passed via Meta.
//
// # Performance Considerations
//
// Translation overhead:
//   - Message conversion: O(n) where n = number of messages
//   - Tool conversion: O(m*p) where m = tools, p = properties per tool
//   - Response conversion: O(1) for text, O(t) for tool calls
//   - Total overhead: < 1ms for typical requests (10 messages, 5 tools)
//
// Memory allocation:
//   - Pre-allocate slices when size is known (make with capacity)
//   - Reuse property maps where possible
//   - Avoid unnecessary string concatenation
//
// Example optimization in convertMessages():
//
//	result := make([]api.Message, len(messages))  // Pre-allocate
//	for i, msg := range messages {
//	    result[i] = api.Message{...}  // Direct assignment
//	}
