// Package ollama provides ChatModel adapter for Ollama API.
//
// Ollama enables running large language models locally on your machine,
// providing cost-free, offline LLM execution for development, testing,
// and privacy-sensitive workloads.
//
// # Basic Usage
//
// Create an adapter to connect to a local Ollama instance:
//
//	config := ollama.Config{
//	    Model: "gpt-oss", // or llama3.2, mistral, codellama
//	}
//	adapter, err := ollama.NewChatModel(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Send messages to the model:
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "What is the capital of France?"},
//	}
//	out, err := adapter.Chat(context.Background(), messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(out.Text) // "The capital of France is Paris."
//
// # Remote Instances
//
// Connect to a remote Ollama server:
//
//	config := ollama.Config{
//	    Endpoint: "http://ollama-server:11434",
//	    Model:    "mistral",
//	}
//	adapter, _ := ollama.NewChatModel(config)
//
// # Model Configuration
//
// Configure generation parameters:
//
//	seed := 42
//	temp := 0.7
//	topP := 0.9
//	config := ollama.Config{
//	    Model:       "gpt-oss",
//	    Temperature: &temp,
//	    TopP:        &topP,
//	    Seed:        &seed, // Deterministic generation
//	}
//	adapter, _ := ollama.NewChatModel(config)
//
// # Tool Calling
//
// Use tools for agentic workflows:
//
//	tools := []model.ToolSpec{
//	    {
//	        Name:        "get_weather",
//	        Description: "Get current weather",
//	        Schema: map[string]interface{}{
//	            "type": "object",
//	            "properties": map[string]interface{}{
//	                "location": map[string]interface{}{"type": "string"},
//	            },
//	        },
//	    },
//	}
//	out, _ := adapter.Chat(ctx, messages, tools)
//	for _, call := range out.ToolCalls {
//	    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
//	}
//
// # Error Handling
//
// Errors provide actionable guidance:
//
//	out, err := adapter.Chat(ctx, messages, nil)
//	if err != nil {
//	    var ollamaErr *ollama.OllamaError
//	    if errors.As(err, &ollamaErr) {
//	        switch ollamaErr.Code {
//	        case "connection":
//	            log.Println("Start Ollama with: ollama serve")
//	        case "model_not_found":
//	            log.Printf("Pull model with: ollama pull %s", config.Model)
//	        }
//	    }
//	}
//
// # Prerequisites
//
// Ollama must be installed and running. Install from https://ollama.com
//
// Pull a model before use:
//
//	ollama pull gpt-oss
//	ollama serve
//
// # Features
//
//   - Local and remote instance support
//   - Model selection and parameter configuration
//   - Tool/function calling for compatible models
//   - Context-aware (respects cancellation and timeouts)
//   - Rich error messages with actionable guidance
//   - Thread-safe for concurrent use
//
// # Performance
//
//   - Low adapter overhead (translation and HTTP setup)
//   - No state accumulation between calls
//   - Minimal memory footprint
//
// # Compatibility
//
//   - Implements model.ChatModel interface
//   - Compatible with LangGraph workflow engine
//   - Supports Ollama API v0.1.0+
//   - Requires Go 1.21 or later
package ollama
