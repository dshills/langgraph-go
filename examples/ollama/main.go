package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dshills/langgraph-go/graph/model"
	"github.com/dshills/langgraph-go/graph/model/ollama"
)

func main() {
	fmt.Println("=== Ollama Model Provider Examples ===")

	// Example 1: Basic Local Chat
	fmt.Println("1. Basic Local Chat")
	basicLocalChat()
	fmt.Println()

	// Example 2: Remote Instance
	fmt.Println("2. Remote Instance Configuration")
	remoteInstance()
	fmt.Println()

	// Example 3: Deterministic Generation with Seed
	fmt.Println("3. Deterministic Generation")
	deterministicGeneration()
	fmt.Println()

	// Example 4: Tool Calling
	fmt.Println("4. Tool Calling (Function Calling)")
	toolCalling()
	fmt.Println()

	// Example 5: Custom Parameters
	fmt.Println("5. Custom Temperature and TopP")
	customParameters()
	fmt.Println()

	fmt.Println("=== All Examples Complete ===")
}

// basicLocalChat demonstrates basic local Ollama usage
func basicLocalChat() {
	// Create adapter for local Ollama (default: http://localhost:11434)
	config := ollama.Config{
		Model: "gpt-oss", // Use the model specified in user requirements
	}

	adapter, err := ollama.NewChatModel(config)
	if err != nil {
		log.Printf("Failed to create adapter: %v\n", err)
		log.Println("Ensure Ollama is running: ollama serve")
		log.Println("And model is pulled: ollama pull gpt-oss")
		return
	}

	// Send a simple message
	messages := []model.Message{
		{Role: model.RoleUser, Content: "What is the capital of France? Answer in one sentence."},
	}

	out, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", out.Text)
	fmt.Printf("Model: %s\n", out.Meta["model"])
}

// remoteInstance demonstrates connecting to a remote Ollama server
func remoteInstance() {
	// Connect to remote Ollama instance
	config := ollama.Config{
		Endpoint: "http://localhost:11434", // Change to your remote server
		Model:    "gpt-oss",
	}

	adapter, err := ollama.NewChatModel(config)
	if err != nil {
		log.Printf("Failed to create adapter: %v\n", err)
		return
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Say hello in French."},
	}

	out, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}

	fmt.Printf("Remote response: %s\n", out.Text)
}

// deterministicGeneration demonstrates using a seed for reproducible outputs
func deterministicGeneration() {
	seed := 42
	temp := 0.0 // Minimum randomness

	config := ollama.Config{
		Model:       "gpt-oss",
		Seed:        &seed,
		Temperature: &temp,
	}

	adapter, err := ollama.NewChatModel(config)
	if err != nil {
		log.Printf("Failed to create adapter: %v\n", err)
		return
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Count from 1 to 5."},
	}

	// Run twice to show deterministic output
	fmt.Println("First run:")
	out1, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}
	fmt.Printf("  %s\n", out1.Text)

	fmt.Println("Second run (same seed, same result):")
	out2, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}
	fmt.Printf("  %s\n", out2.Text)
}

// toolCalling demonstrates function calling for agentic workflows
func toolCalling() {
	config := ollama.Config{
		Model: "gpt-oss", // Ensure model supports tools
	}

	adapter, err := ollama.NewChatModel(config)
	if err != nil {
		log.Printf("Failed to create adapter: %v\n", err)
		return
	}

	// Define tools
	tools := []model.ToolSpec{
		{
			Name:        "get_weather",
			Description: "Get current weather for a location",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "City name (e.g., 'Paris', 'London')",
					},
					"unit": map[string]interface{}{
						"type":        "string",
						"description": "Temperature unit",
						"enum":        []string{"celsius", "fahrenheit"},
					},
				},
				"required": []string{"location"},
			},
		},
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What's the weather in Paris?"},
	}

	out, err := adapter.Chat(context.Background(), messages, tools)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}

	// Check for tool calls
	if len(out.ToolCalls) > 0 {
		fmt.Println("Tool calls requested:")
		for _, call := range out.ToolCalls {
			fmt.Printf("  - Tool: %s\n", call.Name)
			fmt.Printf("    Input: %v\n", call.Input)
		}
	} else {
		fmt.Printf("Text response: %s\n", out.Text)
		fmt.Println("Note: Tool calling may require a compatible model (e.g., llama3.1)")
	}
}

// customParameters demonstrates fine-tuning generation parameters
func customParameters() {
	temp := 0.7
	topP := 0.9
	numPredict := 50 // Limit to 50 tokens

	config := ollama.Config{
		Model:       "gpt-oss",
		Temperature: &temp,
		TopP:        &topP,
		NumPredict:  &numPredict,
	}

	adapter, err := ollama.NewChatModel(config)
	if err != nil {
		log.Printf("Failed to create adapter: %v\n", err)
		return
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Explain quantum computing."},
	}

	out, err := adapter.Chat(context.Background(), messages, nil)
	if err != nil {
		log.Printf("Chat failed: %v\n", err)
		return
	}

	fmt.Printf("Response (limited to ~50 tokens): %s\n", out.Text)
	if evalCount, ok := out.Meta["eval_count"].(int); ok {
		fmt.Printf("Actual tokens generated: %d\n", evalCount)
	}
}
