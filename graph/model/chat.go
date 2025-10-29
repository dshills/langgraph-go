// Package model provides LLM integration adapters.
package model

import "context"

// ChatModel defines the interface for LLM chat providers.
//
// This interface abstracts the differences between various LLM providers.
// (OpenAI, Anthropic, Google, local models) providing a unified API for.
// chat-based interactions.
//
// Implementations should:
// - Handle provider-specific authentication.
// - Convert standard Message format to provider-specific format.
// - Parse provider responses back to standard ChatOut format.
// - Respect context cancellation and timeouts.
// - Handle retries and rate limiting appropriately.
//
// Example usage:
//
// model := openai.NewChatModel(apiKey).
// messages := []Message{.
//
//		    {Role: RoleUser, Content: "What is the capital of France?"},
//	}.
//
// out, err := model.Chat(ctx, messages, nil).
// if err != nil {.
// log.Fatal(err).
// }.
// fmt.Println(out.Text) // "The capital of France is Paris.".
//
// Example with tools:
//
// tools := []ToolSpec{.
// {.
//
//	Name:        "get_weather",
//	Description: "Get current weather for a location",
//
// Schema: map[string]interface{}{.
//
//	"type": "object",
//
// "properties": map[string]interface{}{.
// "location": map[string]interface{}{.
//
//		                    "type":        "string",
//		                    "description": "City name",
//		                },
//		            },
//		        },
//		    },
//	}.
//
// out, err := model.Chat(ctx, messages, tools).
// if err != nil {.
// log.Fatal(err).
// }.
// for _, call := range out.ToolCalls {.
// fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input).
// }.
type ChatModel interface {
	// Chat sends messages to the LLM and returns the response.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout control.
	// - messages: Conversation history (system, user, assistant messages).
	// - tools: Optional tool specifications the LLM can use (nil if no tools).
	//
	// Returns:
	// - ChatOut: LLM response containing text and/or tool calls.
	// - error: Provider errors, network errors, or context cancellation.
	//
	// The LLM may respond with:
	// - Text only: Direct answer to the user's question.
	// - Tool calls only: Request to invoke external tools.
	// - Both: Text explanation plus tool invocations.
	Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}

// Message represents a single message in an LLM conversation.
//
// Messages are the fundamental unit of communication with LLM providers.
// They follow the common chat format used by OpenAI, Anthropic, Google, and other providers.
//
// Typical conversation structure:
// - System message (optional): Sets context and behavior.
// - User messages: User input or questions.
// - Assistant messages: LLM responses.
//
// Example:
//
// conversation := []Message{.
//
//		    {Role: RoleSystem, Content: "You are a helpful assistant."},
//		    {Role: RoleUser, Content: "What is the capital of France?"},
//		    {Role: RoleAssistant, Content: "The capital of France is Paris."},
//	}.
type Message struct {
	// Role identifies the message sender.
	// Standard roles: "system", "user", "assistant".
	// Use the Role* constants for consistency.
	Role string

	// Content contains the message text.
	// May be empty for messages that only contain tool calls.
	Content string
}

// Standard role constants for LLM conversations.
// These align with the conventions used by major LLM providers.
const (
	// RoleSystem indicates a system message that sets context or instructions.
	// System messages typically appear first in a conversation.
	RoleSystem = "system"

	// RoleUser indicates a message from the human user.
	// User messages contain questions, requests, or input data.
	RoleUser = "user"

	// RoleAssistant indicates a response from the LLM.
	// Assistant messages contain generated text or tool calls.
	RoleAssistant = "assistant"
)

// ToolSpec describes a tool that an LLM can call.
//
// Tools enable LLMs to interact with external systems:
// - Web searches.
// - Database queries.
// - API calls.
// - Code execution.
//
// The Schema field follows JSON Schema format and describes the expected input parameters.
//
// Example:
//
// weatherTool := ToolSpec{.
//
//	Name:        "get_weather",
//	Description: "Get current weather for a location",
//
// Schema: map[string]interface{}{.
//
//	"type": "object",
//
// "properties": map[string]interface{}{.
// "location": map[string]interface{}{.
//
//		                "type":        "string",
//		                "description": "City name or coordinates",
//		            },
//		        },
//		        "required": []string{"location"},
//		    },
//	}.
type ToolSpec struct {
	// Name uniquely identifies the tool.
	// Must be a valid function name (alphanumeric + underscores).
	Name string

	// Description explains what the tool does.
	// The LLM uses this to decide when to call the tool.
	Description string

	// Schema defines the tool's input parameters using JSON Schema format.
	// Optional for tools with no parameters.
	Schema map[string]interface{}
}

// ChatOut represents the output from an LLM chat completion.
//
// LLMs can respond with:
// - Text only: A direct answer.
// - Tool calls only: Request to invoke external tools.
// - Both: Text explanation plus tool invocations.
//
// Example text response:
//
// out := ChatOut{.
//
//		    Text: "The capital of France is Paris.",
//	}.
//
// Example tool call response:
//
// out := ChatOut{.
// ToolCalls: []ToolCall{.
// {.
//
//		            Name:  "search_web",
//		            Input: map[string]interface{}{"query": "Paris landmarks"},
//		        },
//		    },
//	}.
type ChatOut struct {
	// Text contains the LLM's generated response.
	// May be empty if the LLM only wants to call tools.
	Text string

	// ToolCalls contains tools the LLM wants to invoke.
	// Empty if the LLM provided a direct text response.
	ToolCalls []ToolCall
}

// ToolCall represents a request from the LLM to invoke a specific tool.
//
// After the LLM requests tool calls, the application should:
// 1. Execute each tool with the provided Input.
// 2. Collect the results.
// 3. Send results back to the LLM in a new message.
//
// Example:
//
// call := ToolCall{.
//
//		    Name:  "calculate",
//		    Input: map[string]interface{}{"expression": "2+2"},
//	}.
type ToolCall struct {
	// Name identifies which tool to call.
	// Must match a ToolSpec.Name from the available tools.
	Name string

	// Input contains the parameters for the tool call.
	// Structure matches the ToolSpec.Schema for this tool.
	// May be nil for tools that take no parameters.
	Input map[string]interface{}
}

// Provider Selection Patterns.
//
// # Choosing the Right LLM Provider.
//
// Different LLM providers have different strengths. Choose based on:
// - Task requirements (reasoning, speed, cost).
// - Feature needs (tools, vision, context length).
// - Provider capabilities and limitations.
//
// Available providers:
// - OpenAI (GPT-4, GPT-3.5): General-purpose, strong tool calling.
// - Anthropic (Claude 3): Long context, detailed reasoning.
// - Google (Gemini): Fast responses, multimodal capabilities.
//
// # Provider Selection Strategies.
//
// ## 1. Task-Based Selection.
//
// Select provider based on task type:
//
// func selectModel(taskType string) ChatModel {.
// switch taskType {.
//	    case "reasoning":
// return anthropic.NewChatModel(key, "claude-3-opus-20240229").
//	    case "quick":
// return google.NewChatModel(key, "gemini-pro").
//	    case "tools":
// return openai.NewChatModel(key, "gpt-4").
//	    default:
// return openai.NewChatModel(key, "gpt-3.5-turbo").
// }.
// }.
//
// ## 2. Fallback Pattern.
//
// Handle provider errors with fallback to secondary provider:
//
// providers := []ChatModel{primaryModel, secondaryModel, tertiaryModel}.
// var lastErr error.
//
// for _, model := range providers {.
// out, err := model.Chat(ctx, messages, tools).
// if err == nil {.
// return out, nil.
// }.
// lastErr = err.
// log.Printf("Provider failed: %v, trying next...", err).
// }.
// return ChatOut{}, fmt.Errorf("all providers failed: %w", lastErr).
//
// ## 3. Cost Optimization.
//
// Use cheaper models for simple tasks, expensive for complex:
//
// func selectByComplexity(complexity int) ChatModel {.
// if complexity > 8 {.
// return openai.NewChatModel(key, "gpt-4")           // High quality.
// } else if complexity > 5 {.
// return anthropic.NewChatModel(key, "claude-3-sonnet") // Balanced.
// } else {.
// return openai.NewChatModel(key, "gpt-3.5-turbo")  // Fast & cheap.
// }.
// }.
//
// ## 4. Feature-Based Selection.
//
// Choose provider based on required features:
//
// func selectByFeatures(needsTools, needsVision bool) ChatModel {.
// if needsVision {.
// return google.NewChatModel(key, "gemini-pro-vision").
// }.
// if needsTools {.
// return openai.NewChatModel(key, "gpt-4").
// }.
// return anthropic.NewChatModel(key, "claude-3-sonnet").
// }.
//
// # Error Handling Patterns.
//
// ## Provider-Specific Errors.
//
// Handle provider-specific error types:
//
// out, err := googleModel.Chat(ctx, messages, nil).
// if err != nil {.
// var safetyErr *google.SafetyFilterError.
// if errors.As(err, &safetyErr) {.
// log.Printf("Content blocked: %s", safetyErr.Category()).
// // Handle safety filter - maybe retry with different provider.
// return fallbackModel.Chat(ctx, messages, nil).
// }.
// return ChatOut{}, err.
// }.
//
// ## Retry Logic.
//
// Some providers (like OpenAI) implement automatic retries:
//
// // OpenAI automatically retries transient errors.
// model := openai.NewChatModel(key, "gpt-4").
// out, err := model.Chat(ctx, messages, nil).
// // Transient errors are retried up to 3 times with exponential backoff.
//
// For other providers, implement your own retry logic:
//
// var out ChatOut.
// var err error.
// for attempt := 0; attempt < 3; attempt++ {.
// out, err = model.Chat(ctx, messages, nil).
// if err == nil {.
// break.
// }.
// if !isTransient(err) {.
// break // Don't retry non-transient errors.
// }.
// time.Sleep(time.Second * time.Duration(attempt+1)).
// }.
//
// ## Context Cancellation.
//
// All providers respect context cancellation:
//
// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second).
// defer cancel().
//
// out, err := model.Chat(ctx, messages, nil).
// if errors.Is(err, context.DeadlineExceeded) {.
// log.Println("Request timed out").
// }.
//
// # Multi-Provider Workflows.
//
// ## Sequential Processing.
//
// Use different providers for different stages:
//
// // Stage 1: Fast initial processing with Gemini.
// draft, _ := gemini.Chat(ctx, messages, nil).
//
// // Stage 2: Detailed refinement with Claude.
// refined := append(messages, Message{.
//	    Role:    RoleAssistant,
//	    Content: draft.Text,
// }).
// refined = append(refined, Message{.
//	    Role:    RoleUser,
//	    Content: "Please refine and expand this response.",
// }).
// final, _ := claude.Chat(ctx, refined, nil).
//
// ## Provider Comparison.
//
// Get responses from multiple providers and choose best:
//
// responses := make([]ChatOut, 0, 3).
// for _, model := range []ChatModel{gpt4, claude, gemini} {.
// out, err := model.Chat(ctx, messages, nil).
// if err == nil {.
// responses = append(responses, out).
// }.
// }.
// best := selectBestResponse(responses) // Custom selection logic.
//
// # Best Practices.
//
// 1. **Start Simple**: Begin with a single provider, add fallbacks when needed.
//
// 2. **Monitor Costs**: Track API usage and costs per provider.
//
// 3. **Handle Errors Gracefully**: Always have a fallback strategy.
//
// 4. **Respect Rate Limits**: Implement backoff for rate limit errors.
//
// 5. **Cache When Possible**: Cache responses for identical inputs.
//
// 6. **Use Timeouts**: Set reasonable context deadlines.
//
// 7. **Log Failures**: Track which providers fail and why.
//
// 8. **Test Fallbacks**: Verify fallback logic works before production.
//
// # Provider Capabilities Matrix.
//
// Feature comparison:
//
// | Feature           | OpenAI  | Anthropic | Google  |.
// |-------------------|---------|-----------|---------|.
// | Tool Calling      | ✓✓      | ✓         | ✓       |.
// | Vision            | ✓       | ✓         | ✓✓      |.
// | Long Context      | ✓       | ✓✓        | ✓       |.
// | Speed             | ✓       | ✓         | ✓✓      |.
// | System Prompts    | ✓       | ✓✓        | ✓       |.
// | JSON Mode         | ✓       | ✗         | ✓       |.
// | Automatic Retry   | ✓       | ✗         | ✗       |.
//
// ✓✓ = Excellent, ✓ = Good, ✗ = Not Available.
//
// # Example: Production-Ready Multi-Provider Setup.
//
// type ModelSelector struct {.
// primary    ChatModel.
// fallback   ChatModel.
// costSaver  ChatModel.
// }.
//
// func NewModelSelector(openaiKey, anthropicKey, googleKey string) *ModelSelector {.
// return &ModelSelector{.
//	        primary:   openai.NewChatModel(openaiKey, "gpt-4"),
//	        fallback:  anthropic.NewChatModel(anthropicKey, "claude-3-sonnet"),
//	        costSaver: google.NewChatModel(googleKey, "gemini-pro"),
// }.
// }.
//
// func (s *ModelSelector) Chat(ctx context.Context, messages []Message, complexity int) (ChatOut, error) {.
// // Choose model based on complexity.
// var model ChatModel.
// if complexity > 7 {.
// model = s.primary.
// } else {.
// model = s.costSaver.
// }.
//
// // Try primary choice.
// out, err := model.Chat(ctx, messages, nil).
// if err == nil {.
// return out, nil.
// }.
//
// // Fallback on error.
// log.Printf("Primary model failed: %v, using fallback", err).
// return s.fallback.Chat(ctx, messages, nil).
// }.
