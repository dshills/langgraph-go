// Package main demonstrates AWS Bedrock Nova model integration with LangGraph-Go.
//
// This example shows how to use Amazon Nova models (Micro, Lite, Pro) through
// AWS Bedrock for text generation and tool calling.
//
// Setup:
//   - AWS credentials configured via environment variables, ~/.aws/credentials, or IAM role
//   - Model access enabled in AWS Bedrock console for your region
//   - Region: us-east-1 (or adjust in config)
//
// Run:
//
//	go run main.go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dshills/langgraph-go/graph/model"
	"github.com/dshills/langgraph-go/graph/model/bedrock"
)

func main() {
	ctx := context.Background()

	// Example 1: Basic chat with Nova Lite
	fmt.Println("=== Example 1: Basic Chat ===")
	if err := basicChatExample(ctx); err != nil {
		log.Printf("Example 1 failed: %v\n", err)
	}

	// Example 2: Tool calling with Nova
	fmt.Println("\n=== Example 2: Tool Calling ===")
	if err := toolCallingExample(ctx); err != nil {
		log.Printf("Example 2 failed: %v\n", err)
	}

	// Example 3: Multi-turn conversation
	fmt.Println("\n=== Example 3: Multi-Turn Conversation ===")
	if err := multiTurnExample(ctx); err != nil {
		log.Printf("Example 3 failed: %v\n", err)
	}

	// Example 4: Using different Nova variants
	fmt.Println("\n=== Example 4: Nova Model Variants ===")
	if err := modelVariantsExample(ctx); err != nil {
		log.Printf("Example 4 failed: %v\n", err)
	}
}

// basicChatExample demonstrates a simple question-answer interaction with Nova Lite.
func basicChatExample(ctx context.Context) error {
	// Configure Nova Lite adapter
	config := bedrock.Config{
		Region:    "us-east-1",
		ModelID:   "us.amazon.nova-lite-v1:0", // Use inference profile for cross-region routing
		MaxTokens: 512,
	}

	// Create adapter
	adapter, err := bedrock.NewAdapter(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// Ask a simple question
	messages := []model.Message{
		{Role: model.RoleUser, Content: "What is the capital of France? Answer in one sentence."},
	}

	response, err := adapter.Chat(ctx, messages, nil)
	if err != nil {
		return fmt.Errorf("chat failed: %w", err)
	}

	fmt.Printf("Question: What is the capital of France?\n")
	fmt.Printf("Nova Lite: %s\n", response.Text)
	fmt.Printf("Tokens: %d input, %d output\n",
		response.Meta["input_tokens"], response.Meta["output_tokens"])

	return nil
}

// toolCallingExample demonstrates Nova's tool calling capabilities.
func toolCallingExample(ctx context.Context) error {
	config := bedrock.Config{
		Region:    "us-east-1",
		ModelID:   "amazon.nova-lite-v1:0", // Direct model ID
		MaxTokens: 512,
	}

	adapter, err := bedrock.NewAdapter(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// Define a weather tool
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
		{Role: model.RoleUser, Content: "What's the weather like in Tokyo?"},
	}

	response, err := adapter.Chat(ctx, messages, tools)
	if err != nil {
		return fmt.Errorf("chat failed: %w", err)
	}

	fmt.Printf("Question: What's the weather like in Tokyo?\n")

	if len(response.ToolCalls) > 0 {
		fmt.Printf("Nova wants to call tool: %s\n", response.ToolCalls[0].Name)
		fmt.Printf("With arguments: %v\n", response.ToolCalls[0].Input)
	}

	if response.Text != "" {
		fmt.Printf("Nova's thinking: %s\n", response.Text)
	}

	return nil
}

// multiTurnExample demonstrates multi-turn conversations with context memory.
func multiTurnExample(ctx context.Context) error {
	config := bedrock.Config{
		Region:      "us-east-1",
		ModelID:     "us.amazon.nova-lite-v1:0",
		MaxTokens:   512,
		Temperature: 0.7, // Add some creativity
	}

	adapter, err := bedrock.NewAdapter(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}

	// Build a conversation
	messages := []model.Message{
		{Role: model.RoleUser, Content: "My favorite color is blue."},
	}

	// First turn
	response1, err := adapter.Chat(ctx, messages, nil)
	if err != nil {
		return fmt.Errorf("turn 1 failed: %w", err)
	}

	fmt.Printf("User: My favorite color is blue.\n")
	fmt.Printf("Nova: %s\n\n", response1.Text)

	// Add assistant response to history
	messages = append(messages, model.Message{
		Role:    model.RoleAssistant,
		Content: response1.Text,
	})

	// Second turn - test memory
	messages = append(messages, model.Message{
		Role:    model.RoleUser,
		Content: "What color did I just mention?",
	})

	response2, err := adapter.Chat(ctx, messages, nil)
	if err != nil {
		return fmt.Errorf("turn 2 failed: %w", err)
	}

	fmt.Printf("User: What color did I just mention?\n")
	fmt.Printf("Nova: %s\n", response2.Text)

	return nil
}

// modelVariantsExample demonstrates using different Nova model variants.
func modelVariantsExample(ctx context.Context) error {
	variants := []struct {
		name    string
		modelID string
		desc    string
	}{
		{
			name:    "Nova Micro",
			modelID: "amazon.nova-micro-v1:0",
			desc:    "Fastest, lowest cost",
		},
		{
			name:    "Nova Lite",
			modelID: "amazon.nova-lite-v1:0",
			desc:    "Balanced speed and capability",
		},
		{
			name:    "Nova Pro",
			modelID: "amazon.nova-pro-v1:0",
			desc:    "Most capable, multimodal",
		},
	}

	question := "What is 2+2?"

	for _, variant := range variants {
		fmt.Printf("\n--- %s (%s) ---\n", variant.name, variant.desc)

		config := bedrock.Config{
			Region:    "us-east-1",
			ModelID:   variant.modelID,
			MaxTokens: 100,
		}

		adapter, err := bedrock.NewAdapter(ctx, config)
		if err != nil {
			fmt.Printf("Failed to create adapter: %v\n", err)
			continue
		}

		messages := []model.Message{
			{Role: model.RoleUser, Content: question},
		}

		response, err := adapter.Chat(ctx, messages, nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Response: %s\n", response.Text)
		fmt.Printf("Tokens: %d in, %d out\n",
			response.Meta["input_tokens"], response.Meta["output_tokens"])
	}

	return nil
}
