package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/dshills/langgraph-go/graph/model"
	"github.com/dshills/langgraph-go/graph/model/anthropic"
	"github.com/dshills/langgraph-go/graph/model/google"
	"github.com/dshills/langgraph-go/graph/model/openai"
)

// This example demonstrates using multiple LLM providers in a workflow.
// It shows provider switching, error handling, and fallback patterns.
func main() {
	ctx := context.Background()

	// Example 1: Basic usage of each provider
	fmt.Println("=== Example 1: Basic Provider Usage ===")
	if err := basicProviderUsage(ctx); err != nil {
		log.Printf("Example 1 failed: %v", err)
	}

	// Example 2: Provider switching based on requirements
	fmt.Println("\n=== Example 2: Provider Switching ===")
	if err := providerSwitching(ctx); err != nil {
		log.Printf("Example 2 failed: %v", err)
	}

	// Example 3: Error handling and fallbacks
	fmt.Println("\n=== Example 3: Error Handling & Fallbacks ===")
	if err := errorHandlingExample(ctx); err != nil {
		log.Printf("Example 3 failed: %v", err)
	}

	// Example 4: Tool usage with LLMs
	fmt.Println("\n=== Example 4: Tool Usage ===")
	if err := toolUsageExample(ctx); err != nil {
		log.Printf("Example 4 failed: %v", err)
	}
}

// basicProviderUsage demonstrates simple usage of each provider.
func basicProviderUsage(ctx context.Context) error {
	messages := []model.Message{
		{Role: model.RoleUser, Content: "What is the capital of France?"},
	}

	// OpenAI
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		fmt.Println("\n--- OpenAI (GPT-4o) ---")
		m := openai.NewChatModel(openaiKey, "gpt-4o")
		out, err := m.Chat(ctx, messages, nil)
		if err != nil {
			return fmt.Errorf("OpenAI error: %w", err)
		}
		fmt.Printf("Response: %s\n", out.Text)
	} else {
		fmt.Println("Skipping OpenAI (no API key)")
	}

	// Anthropic
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey != "" {
		fmt.Println("\n--- Anthropic (Claude Sonnet 4.5) ---")
		m := anthropic.NewChatModel(anthropicKey, "claude-sonnet-4-5-20250929")
		out, err := m.Chat(ctx, messages, nil)
		if err != nil {
			return fmt.Errorf("anthropic error: %w", err)
		}
		fmt.Printf("Response: %s\n", out.Text)
	} else {
		fmt.Println("Skipping Anthropic (no API key)")
	}

	// Google
	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey != "" {
		fmt.Println("\n--- Google (Gemini 2.5 Flash) ---")
		m := google.NewChatModel(googleKey, "gemini-2.5-flash")
		out, err := m.Chat(ctx, messages, nil)
		if err != nil {
			// Handle Google-specific safety filter errors
			var safetyErr *google.SafetyFilterError
			if errors.As(err, &safetyErr) {
				fmt.Printf("Content blocked: %s (category: %s)\n",
					safetyErr.Reason(), safetyErr.Category())
				return nil
			}
			return fmt.Errorf("google error: %w", err)
		}
		fmt.Printf("Response: %s\n", out.Text)
	} else {
		fmt.Println("Skipping Google (no API key)")
	}

	return nil
}

// providerSwitching demonstrates choosing providers based on task requirements.
func providerSwitching(ctx context.Context) error {
	// Use Claude for long-form reasoning tasks
	fmt.Println("\n--- Long-form reasoning (Claude Sonnet 4.5) ---")
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey != "" {
		claude := anthropic.NewChatModel(anthropicKey, "claude-sonnet-4-5-20250929")
		messages := []model.Message{
			{Role: model.RoleSystem, Content: "You are a thoughtful philosopher."},
			{Role: model.RoleUser, Content: "Explain the trolley problem in 2 sentences."},
		}
		out, err := claude.Chat(ctx, messages, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Claude: %s\n", out.Text)
	}

	// Use GPT-4o for tasks requiring recent knowledge
	fmt.Println("\n--- Recent knowledge (GPT-4o) ---")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		gpt4 := openai.NewChatModel(openaiKey, "gpt-4o")
		messages := []model.Message{
			{Role: model.RoleUser, Content: "What are the latest developments in AI?"},
		}
		out, err := gpt4.Chat(ctx, messages, nil)
		if err != nil {
			return err
		}
		fmt.Printf("GPT-4: %s\n", out.Text)
	}

	// Use Gemini for multimodal tasks (placeholder - vision not in this example)
	fmt.Println("\n--- Fast responses (Gemini 2.5 Flash) ---")
	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey != "" {
		gemini := google.NewChatModel(googleKey, "gemini-2.5-flash")
		messages := []model.Message{
			{Role: model.RoleUser, Content: "What is 2+2?"},
		}
		out, err := gemini.Chat(ctx, messages, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Gemini: %s\n", out.Text)
	}

	return nil
}

// errorHandlingExample demonstrates handling provider-specific errors with fallbacks.
func errorHandlingExample(ctx context.Context) error {
	messages := []model.Message{
		{Role: model.RoleUser, Content: "Hello!"},
	}

	// Try providers in order, with fallbacks
	providers := []struct {
		name  string
		model model.ChatModel
	}{
		{"OpenAI", createOpenAIModel()},
		{"Anthropic", createAnthropicModel()},
		{"Google", createGoogleModel()},
	}

	for _, p := range providers {
		if p.model == nil {
			fmt.Printf("Skipping %s (not configured)\n", p.name)
			continue
		}

		fmt.Printf("\nTrying %s...\n", p.name)
		out, err := p.model.Chat(ctx, messages, nil)
		if err != nil {
			fmt.Printf("  %s failed: %v\n", p.name, err)
			continue
		}

		fmt.Printf("  Success! Response: %s\n", out.Text)
		return nil
	}

	return fmt.Errorf("all providers failed")
}

// toolUsageExample demonstrates using tools with different providers.
func toolUsageExample(ctx context.Context) error {
	// Define a simple calculator tool
	tools := []model.ToolSpec{
		{
			Name:        "calculator",
			Description: "Performs basic arithmetic operations",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"add", "subtract", "multiply", "divide"},
						"description": "The operation to perform",
					},
					"a": map[string]interface{}{
						"type":        "number",
						"description": "First number",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "Second number",
					},
				},
				"required": []string{"operation", "a", "b"},
			},
		},
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What is 15 multiplied by 7?"},
	}

	// Try with OpenAI (best tool calling support)
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		fmt.Println("\n--- Tool calling with OpenAI ---")
		m := openai.NewChatModel(openaiKey, "gpt-4o")
		out, err := m.Chat(ctx, messages, tools)
		if err != nil {
			return err
		}

		if len(out.ToolCalls) > 0 {
			fmt.Printf("Tool called: %s\n", out.ToolCalls[0].Name)
			fmt.Printf("Arguments: %v\n", out.ToolCalls[0].Input)

			// In a real workflow, you'd execute the tool and send results back
			// For now, just demonstrate the structure
		} else {
			fmt.Printf("Direct answer: %s\n", out.Text)
		}
	}

	return nil
}

// Helper functions to create models (with nil fallback if no API key)

func createOpenAIModel() model.ChatModel {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil
	}
	return openai.NewChatModel(key, "gpt-4o")
}

func createAnthropicModel() model.ChatModel {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil
	}
	return anthropic.NewChatModel(key, "claude-sonnet-4-5-20250929")
}

func createGoogleModel() model.ChatModel {
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		return nil
	}
	// Use gemini-2.5-flash (latest stable Flash model as of 2025)
	return google.NewChatModel(key, "gemini-2.5-flash")
}
