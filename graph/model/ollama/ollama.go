package ollama

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dshills/langgraph-go/graph/model"
	"github.com/ollama/ollama/api"
)

// ChatModel implements model.ChatModel for Ollama's API.
//
// Provides access to locally-hosted Ollama models with:
//   - Direct integration with Ollama HTTP API
//   - Temperature, TopP, and seed configuration
//   - Non-streaming request/response
//   - Context cancellation support
//   - Comprehensive error translation
//
// Example usage:
//
//	config := ollama.Config{
//	    Model: "llama3.2",
//	    Temperature: 0.7,
//	}
//	adapter, err := ollama.NewChatModel(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "What is Go?"},
//	}
//
//	out, err := adapter.Chat(ctx, messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text)
type ChatModel struct {
	client *api.Client
	config Config
}

// NewChatModel creates a new Ollama ChatModel adapter.
//
// Validates configuration, applies defaults, and initializes the Ollama API client.
// Configuration validation and defaulting is performed by validateConfig.
//
// Configuration defaults (applied by validateConfig):
//   - Endpoint: "http://localhost:11434" if empty
//   - Temperature: 0.8 if nil
//   - TopP: 0.9 if nil
//   - NumPredict: -1 (unlimited) if nil
//   - HTTPClient: http.DefaultClient with 60s timeout if nil
//
// Parameters:
//   - config: Ollama configuration (Model is required)
//
// Returns:
//   - ChatModel configured for Ollama API
//   - Error if configuration is invalid
//
// Example:
//
//	config := Config{Model: "gpt-oss"}
//	adapter, err := NewChatModel(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewChatModel(config Config) (*ChatModel, error) {
	// Validate config and apply defaults
	// This mutates config to set defaults for nil pointer fields
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	// Parse endpoint URL
	baseURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL %q: %w", config.Endpoint, err)
	}

	// Create Ollama API client
	client := api.NewClient(baseURL, config.HTTPClient)

	return &ChatModel{
		client: client,
		config: config,
	}, nil
}

// Chat implements the model.ChatModel interface.
//
// Sends messages to the Ollama API and returns the response.
// Uses non-streaming mode for simplicity and consistency.
//
// Process:
// 1. Check context cancellation
// 2. Translate messages to Ollama format
// 3. Build request with model configuration and tools (if provided)
// 4. Call Ollama Chat API (non-streaming)
// 5. Parse response to ChatOut format with tool calls
// 6. Translate errors to OllamaError
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - messages: Conversation history (system, user, assistant messages)
//   - tools: Tool specifications for function calling (optional)
//
// Returns:
//   - ChatOut with response text and/or tool calls
//   - Error for connection failures, model not found, or API errors
//
// Example:
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "Hello"},
//	}
//	out, err := adapter.Chat(ctx, messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text)
func (m *ChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Check context cancellation
	if ctx.Err() != nil {
		return model.ChatOut{}, ctx.Err()
	}

	// Translate messages to Ollama format
	ollamaMessages := toOllamaMessages(messages)

	// Build options map (dereference pointer values)
	// Note: These should never be nil if constructed via NewChatModel (which calls validateConfig)
	options := map[string]interface{}{}
	if m.config.Temperature != nil {
		options["temperature"] = *m.config.Temperature
	}
	if m.config.TopP != nil {
		options["top_p"] = *m.config.TopP
	}
	if m.config.NumPredict != nil {
		options["num_predict"] = *m.config.NumPredict
	}

	// Add seed if provided
	if m.config.Seed != nil {
		options["seed"] = *m.config.Seed
	}

	// Build request
	req := &api.ChatRequest{
		Model:    m.config.Model,
		Messages: ollamaMessages,
		Stream:   boolPtr(false), // Non-streaming mode
		Options:  options,
	}

	// Add tools if provided
	if len(tools) > 0 {
		req.Tools = toOllamaTools(tools)
	}

	// Call Ollama Chat API with non-streaming callback
	var finalResp api.ChatResponse
	err := m.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		finalResp = resp
		return nil
	})

	if err != nil {
		// Translate error to OllamaError with endpoint context
		return model.ChatOut{}, translateError(err, m.config.Endpoint)
	}

	// Parse response to ChatOut format
	out, err := toLangGraphOutput(finalResp)
	if err != nil {
		return model.ChatOut{}, err
	}

	return out, nil
}

// toOllamaMessages translates LangGraph messages to Ollama message format.
//
// Direct field mapping:
//   - Role: system, user, assistant (unchanged)
//   - Content: message text
//
// Parameters:
//   - messages: LangGraph message list
//
// Returns:
//   - Ollama API message list
func toOllamaMessages(messages []model.Message) []api.Message {
	result := make([]api.Message, len(messages))

	for i, msg := range messages {
		result[i] = api.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return result
}

// toOllamaTools translates LangGraph ToolSpec to Ollama Tool format.
//
// Converts tool specifications for function calling:
//   - Name: Tool function name
//   - Description: Tool purpose and usage
//   - Schema: JSON schema for parameters (converted to ToolFunctionParameters)
//
// Parameters:
//   - tools: LangGraph tool specifications
//
// Returns:
//   - Ollama API tool list
func toOllamaTools(tools []model.ToolSpec) []api.Tool {
	result := make([]api.Tool, len(tools))

	for i, tool := range tools {
		// Convert schema map to ToolFunctionParameters
		params := schemaToParams(tool.Schema)

		result[i] = api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		}
	}

	return result
}

// schemaToParams converts a JSON schema map to Ollama ToolFunctionParameters.
//
// Extracts fields from the schema:
//   - type: Parameter type (usually "object")
//   - properties: Field definitions (converted to ToolProperty)
//   - required: Required field names
//   - $defs, items: Pass through for complex schemas
//
// Parameters:
//   - schema: JSON schema as map[string]interface{}
//
// Returns:
//   - Ollama ToolFunctionParameters structure
func schemaToParams(schema map[string]interface{}) api.ToolFunctionParameters {
	params := api.ToolFunctionParameters{
		Properties: make(map[string]api.ToolProperty),
	}

	// Extract type (default to "object" if not specified)
	if t, ok := schema["type"].(string); ok {
		params.Type = t
	} else {
		params.Type = "object"
	}

	// Extract properties and convert to ToolProperty
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, propDef := range props {
			if propMap, ok := propDef.(map[string]interface{}); ok {
				property := api.ToolProperty{}

				// Extract type field (can be string or array of strings)
				if t, ok := propMap["type"].(string); ok {
					property.Type = api.PropertyType{t}
				} else if tArr, ok := propMap["type"].([]string); ok {
					property.Type = api.PropertyType(tArr)
				}

				// Extract description
				if desc, ok := propMap["description"].(string); ok {
					property.Description = desc
				}

				// Extract enum values
				if enum, ok := propMap["enum"].([]interface{}); ok {
					property.Enum = enum
				}

				// Extract items for array types
				if items, ok := propMap["items"]; ok {
					property.Items = items
				}

				params.Properties[name] = property
			}
		}
	}

	// Extract required fields
	if req, ok := schema["required"].([]interface{}); ok {
		required := make([]string, len(req))
		for i, r := range req {
			if s, ok := r.(string); ok {
				required[i] = s
			}
		}
		params.Required = required
	} else if req, ok := schema["required"].([]string); ok {
		params.Required = req
	}

	// Pass through $defs and items for complex schemas
	if defs, ok := schema["$defs"]; ok {
		params.Defs = defs
	}
	if items, ok := schema["items"]; ok {
		params.Items = items
	}

	return params
}

// toLangGraphOutput translates Ollama ChatResponse to LangGraph ChatOut.
//
// Extracts:
//   - Text from Message.Content
//   - Tool calls from Message.ToolCalls (if present)
//   - Metadata: model, created_at, done, total_duration, token counts
//
// Parameters:
//   - resp: Ollama API ChatResponse
//
// Returns:
//   - ChatOut with text, tool calls, and metadata
//   - Error if parsing fails
func toLangGraphOutput(resp api.ChatResponse) (model.ChatOut, error) {
	out := model.ChatOut{
		Text: resp.Message.Content,
		Meta: make(map[string]interface{}),
	}

	// Extract tool calls if present
	if len(resp.Message.ToolCalls) > 0 {
		out.ToolCalls = make([]model.ToolCall, len(resp.Message.ToolCalls))
		for i, tc := range resp.Message.ToolCalls {
			// Parse tool arguments
			args := make(map[string]interface{})
			for k, v := range tc.Function.Arguments {
				args[k] = v
			}

			out.ToolCalls[i] = model.ToolCall{
				Name:  tc.Function.Name,
				Input: args,
			}
		}
	}

	// Extract metadata
	out.Meta["model"] = resp.Model
	out.Meta["done"] = resp.Done

	// Add created_at if present
	if !resp.CreatedAt.IsZero() {
		out.Meta["created_at"] = resp.CreatedAt
	}

	// Add timing metadata if present
	if resp.Metrics.TotalDuration > 0 {
		out.Meta["total_duration"] = resp.Metrics.TotalDuration
	}
	if resp.Metrics.LoadDuration > 0 {
		out.Meta["load_duration"] = resp.Metrics.LoadDuration
	}
	if resp.Metrics.PromptEvalDuration > 0 {
		out.Meta["prompt_eval_duration"] = resp.Metrics.PromptEvalDuration
	}
	if resp.Metrics.EvalDuration > 0 {
		out.Meta["eval_duration"] = resp.Metrics.EvalDuration
	}

	// Add token count metadata if present
	if resp.Metrics.PromptEvalCount > 0 {
		out.Meta["prompt_eval_count"] = resp.Metrics.PromptEvalCount
	}
	if resp.Metrics.EvalCount > 0 {
		out.Meta["eval_count"] = resp.Metrics.EvalCount
	}

	// Add done_reason if present
	if resp.DoneReason != "" {
		out.Meta["done_reason"] = resp.DoneReason
	}

	return out, nil
}

// boolPtr returns a pointer to the given bool value.
// Helper for setting optional fields in Ollama API requests.
func boolPtr(b bool) *bool {
	return &b
}
