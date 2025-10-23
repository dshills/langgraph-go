package model

import "context"

// ChatModel defines the interface for LLM chat providers.
//
// This interface abstracts the differences between various LLM providers
// (OpenAI, Anthropic, Google, local models) providing a unified API for
// chat-based interactions.
//
// Implementations should:
//   - Handle provider-specific authentication
//   - Convert standard Message format to provider-specific format
//   - Parse provider responses back to standard ChatOut format
//   - Respect context cancellation and timeouts
//   - Handle retries and rate limiting appropriately
//
// Example usage:
//
//	model := openai.NewChatModel(apiKey)
//	messages := []Message{
//	    {Role: RoleUser, Content: "What is the capital of France?"},
//	}
//	out, err := model.Chat(ctx, messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text) // "The capital of France is Paris."
//
// Example with tools:
//
//	tools := []ToolSpec{
//	    {
//	        Name:        "get_weather",
//	        Description: "Get current weather for a location",
//	        Schema: map[string]interface{}{
//	            "type": "object",
//	            "properties": map[string]interface{}{
//	                "location": map[string]interface{}{
//	                    "type":        "string",
//	                    "description": "City name",
//	                },
//	            },
//	        },
//	    },
//	}
//	out, err := model.Chat(ctx, messages, tools)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, call := range out.ToolCalls {
//	    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
//	}
type ChatModel interface {
	// Chat sends messages to the LLM and returns the response.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - messages: Conversation history (system, user, assistant messages)
	//   - tools: Optional tool specifications the LLM can use (nil if no tools)
	//
	// Returns:
	//   - ChatOut: LLM response containing text and/or tool calls
	//   - error: Provider errors, network errors, or context cancellation
	//
	// The LLM may respond with:
	//   - Text only: Direct answer to the user's question
	//   - Tool calls only: Request to invoke external tools
	//   - Both: Text explanation plus tool invocations
	Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}

// Message represents a single message in an LLM conversation.
//
// Messages are the fundamental unit of communication with LLM providers.
// They follow the common chat format used by OpenAI, Anthropic, Google, and other providers.
//
// Typical conversation structure:
//   - System message (optional): Sets context and behavior
//   - User messages: User input or questions
//   - Assistant messages: LLM responses
//
// Example:
//
//	conversation := []Message{
//	    {Role: RoleSystem, Content: "You are a helpful assistant."},
//	    {Role: RoleUser, Content: "What is the capital of France?"},
//	    {Role: RoleAssistant, Content: "The capital of France is Paris."},
//	}
type Message struct {
	// Role identifies the message sender.
	// Standard roles: "system", "user", "assistant"
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
//   - Web searches
//   - Database queries
//   - API calls
//   - Code execution
//
// The Schema field follows JSON Schema format and describes the expected input parameters.
//
// Example:
//
//	weatherTool := ToolSpec{
//	    Name:        "get_weather",
//	    Description: "Get current weather for a location",
//	    Schema: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "location": map[string]interface{}{
//	                "type":        "string",
//	                "description": "City name or coordinates",
//	            },
//	        },
//	        "required": []string{"location"},
//	    },
//	}
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
//   - Text only: A direct answer
//   - Tool calls only: Request to invoke external tools
//   - Both: Text explanation plus tool invocations
//
// Example text response:
//
//	out := ChatOut{
//	    Text: "The capital of France is Paris.",
//	}
//
// Example tool call response:
//
//	out := ChatOut{
//	    ToolCalls: []ToolCall{
//	        {
//	            Name:  "search_web",
//	            Input: map[string]interface{}{"query": "Paris landmarks"},
//	        },
//	    },
//	}
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
//  1. Execute each tool with the provided Input
//  2. Collect the results
//  3. Send results back to the LLM in a new message
//
// Example:
//
//	call := ToolCall{
//	    Name:  "calculate",
//	    Input: map[string]interface{}{"expression": "2+2"},
//	}
type ToolCall struct {
	// Name identifies which tool to call.
	// Must match a ToolSpec.Name from the available tools.
	Name string

	// Input contains the parameters for the tool call.
	// Structure matches the ToolSpec.Schema for this tool.
	// May be nil for tools that take no parameters.
	Input map[string]interface{}
}
