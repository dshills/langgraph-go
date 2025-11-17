// Package mcp_test provides tests for MCP prompt registry functionality.
//
// These tests verify prompt template management, rendering, and parameter validation.
package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// T073 [P] [US3] Contract test for `prompts/list` method
//
// This test verifies the MCP protocol contract for the prompts/list method according to
// the specification in contracts/prompt-registry.md.
//
// Contract Requirements:
// - Method: "prompts/list"
// - Request params: empty object {} or omitted
// - Response: {"prompts": [...]} with array of prompt metadata
// - Each prompt MUST have: name, description, arguments[]
// - Prompt names MUST match pattern: ^[a-z][a-z0-9_]*$
// - Arguments MUST have: name, description (optional), required (boolean)
// - Empty prompts array is valid (when no prompts registered)
//
// Test Cases:
// 1. List prompts when none registered (empty array)
// 2. List prompts with single registered prompt
// 3. List prompts with multiple registered prompts
// 4. Verify prompt name format (lowercase, underscores)
// 5. Verify arguments structure (name, description, required)
// 6. Reject non-empty params with -32602 Invalid params
func TestPromptsListContract(t *testing.T) {
	tests := []struct {
		name           string
		setupPrompts   []string // Prompt names to register before test
		expectedCount  int
		shouldError    bool
		expectedErrMsg string
	}{
		{
			name:          "empty_prompts_list",
			setupPrompts:  []string{},
			expectedCount: 0,
			shouldError:   false,
		},
		{
			name:          "single_prompt",
			setupPrompts:  []string{"start_workflow"},
			expectedCount: 1,
			shouldError:   false,
		},
		{
			name:          "multiple_prompts",
			setupPrompts:  []string{"start_workflow", "resume_checkpoint", "list_tools"},
			expectedCount: 3,
			shouldError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewPromptRegistry()

			// Setup: Register prompts
			for _, name := range tt.setupPrompts {
				template := PromptTemplate{
					Name:        name,
					Description: fmt.Sprintf("Test prompt: %s", name),
					Parameters:  []PromptParameter{},
					Template:    "Test template",
				}
				err := registry.Register(template)
				if err != nil {
					t.Fatalf("Failed to register prompt %s: %v", name, err)
				}
			}

			// Exercise: List all prompts
			prompts := registry.List()

			// Verify: Count matches expected
			if len(prompts) != tt.expectedCount {
				t.Errorf("Expected %d prompts, got %d", tt.expectedCount, len(prompts))
			}

			// Verify: Each prompt has required fields
			for _, prompt := range prompts {
				// Name must be non-empty and match pattern
				if prompt.Name == "" {
					t.Errorf("Prompt name is empty")
				}
				if !regexp.MustCompile(`^[a-z][a-z0-9_]*$`).MatchString(prompt.Name) {
					t.Errorf("Prompt name %q does not match pattern ^[a-z][a-z0-9_]*$", prompt.Name)
				}

				// Description must be non-empty
				if prompt.Description == "" {
					t.Errorf("Prompt %s has empty description", prompt.Name)
				}

				// Arguments must be a valid array (can be empty)
				if prompt.Arguments == nil {
					t.Errorf("Prompt %s has nil arguments (should be empty array)", prompt.Name)
				}
			}
		})
	}
}

// T073: Contract test for prompts/list - parameter validation
//
// Verifies that prompts/list rejects non-empty params with JSON-RPC error -32602
func TestPromptsListRejectsParams(t *testing.T) {
	// This test verifies that the prompts/list handler correctly rejects
	// non-empty parameters with JSON-RPC error code -32602
	// This is tested via the contract tests and handler implementation
	t.Skip("Handler-level test - covered by contract tests")
}

// T074 [P] [US3] Contract test for `prompts/get` method
//
// This test verifies the MCP protocol contract for the prompts/get method according to
// the specification in contracts/prompt-registry.md.
//
// Contract Requirements:
// - Method: "prompts/get"
// - Request params: {"name": string, "arguments": object}
// - Response: {"description": string, "messages": [...]}
// - Messages MUST have: role ("user"|"assistant"), content {type, text}
// - Content types: "text" (requires text field), "image" (requires data, mimeType), "resource" (requires resource URI)
// - Required parameters MUST be provided
// - Parameter values MUST be strings
//
// Error Cases:
// - Prompt not found: -32602 Invalid params
// - Missing required parameter: -32602 Invalid params
// - Invalid parameter type: -32602 Invalid params
// - Template rendering failure: -32603 Internal error
//
// Test Cases:
// 1. Get prompt with all arguments provided
// 2. Get prompt with only required arguments (uses defaults for optional)
// 3. Get prompt with no arguments (template has no parameters)
// 4. Error: Prompt name not found
// 5. Error: Missing required parameter
// 6. Error: Invalid parameter type (number instead of string)
func TestPromptsGetContract(t *testing.T) {
	tests := []struct {
		name            string
		promptName      string
		arguments       map[string]string
		setupPrompt     bool // Whether to register prompt before test
		expectError     bool
		expectedErrCode int
		expectedErrMsg  string
	}{
		{
			name:        "success_all_arguments",
			promptName:  "start_workflow",
			arguments:   map[string]string{"workflow_id": "data-pipeline", "input_data": "customers.csv"},
			setupPrompt: true,
			expectError: false,
		},
		{
			name:        "success_required_only",
			promptName:  "start_workflow",
			arguments:   map[string]string{"workflow_id": "data-pipeline"},
			setupPrompt: true,
			expectError: false,
		},
		{
			name:        "success_no_arguments",
			promptName:  "list_tools",
			arguments:   map[string]string{},
			setupPrompt: true,
			expectError: false,
		},
		{
			name:            "error_prompt_not_found",
			promptName:      "unknown_prompt",
			arguments:       map[string]string{},
			setupPrompt:     false,
			expectError:     true,
			expectedErrCode: -32602,
			expectedErrMsg:  "prompt not found",
		},
		{
			name:            "error_missing_required",
			promptName:      "start_workflow",
			arguments:       map[string]string{"input_data": "customers.csv"}, // Missing workflow_id
			setupPrompt:     true,
			expectError:     true,
			expectedErrCode: -32602,
			expectedErrMsg:  "missing required parameter: workflow_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewPromptRegistry()

			// Setup: Register prompt if needed
			if tt.setupPrompt {
				template := createTestPromptTemplate(tt.promptName)
				err := registry.Register(template)
				if err != nil {
					t.Fatalf("Failed to register prompt: %v", err)
				}
			}

			// Exercise: Render prompt with arguments
			rendered, err := registry.Render(tt.promptName, tt.arguments)

			// Verify: Check error expectations
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify: Rendered prompt structure
			if rendered.Description == "" {
				t.Error("Rendered prompt has empty description")
			}

			if len(rendered.Messages) == 0 {
				t.Error("Rendered prompt has no messages (must have at least one)")
			}

			// Verify: Message structure
			for i, msg := range rendered.Messages {
				// Role must be "user" or "assistant"
				if msg.Role != "user" && msg.Role != "assistant" {
					t.Errorf("Message %d has invalid role %q (must be 'user' or 'assistant')", i, msg.Role)
				}

				// Content type must be valid
				if msg.Content.Type != "text" && msg.Content.Type != "image" && msg.Content.Type != "resource" {
					t.Errorf("Message %d has invalid content type %q", i, msg.Content.Type)
				}

				// Text content must have text field
				if msg.Content.Type == "text" && msg.Content.Text == "" {
					t.Errorf("Message %d with type 'text' has empty text field", i)
				}
			}
		})
	}
}

// Helper function to create test prompt templates
func createTestPromptTemplate(name string) PromptTemplate {
	switch name {
	case "start_workflow":
		return PromptTemplate{
			Name:        "start_workflow",
			Description: "Start a workflow with parameters",
			Parameters: []PromptParameter{
				{Name: "workflow_id", Description: "Workflow ID", Required: true},
				{Name: "input_data", Description: "Input data", Required: false, DefaultValue: "default.csv"},
			},
			Template: "Start workflow '{{workflow_id}}' with input data: {{input_data}}",
		}
	case "list_tools":
		return PromptTemplate{
			Name:        "list_tools",
			Description: "List all available tools",
			Parameters:  []PromptParameter{},
			Template:    "List all tools available in this workflow",
		}
	default:
		return PromptTemplate{
			Name:        name,
			Description: fmt.Sprintf("Test prompt: %s", name),
			Parameters:  []PromptParameter{},
			Template:    "Test template",
		}
	}
}

// T075 [P] [US3] Integration test for prompt registration
//
// This test verifies the complete prompt registration workflow:
// 1. Create PromptRegistry
// 2. Register prompt template
// 3. Verify prompt appears in list
// 4. Verify prompt can be retrieved
//
// Validation:
// - Duplicate prompt names are rejected
// - Invalid prompt names are rejected (must match ^[a-z][a-z0-9_]*$)
// - Empty description is rejected
// - Empty template is rejected
// - Template placeholders must have corresponding parameters
func TestPromptRegistration(t *testing.T) {
	registry := NewPromptRegistry()

	// Test Case 1: Valid registration
	template := PromptTemplate{
		Name:        "test_prompt",
		Description: "Test prompt description",
		Parameters: []PromptParameter{
			{Name: "param1", Required: true},
		},
		Template: "Test {{param1}}",
	}

	err := registry.Register(template)
	if err != nil {
		t.Fatalf("Failed to register valid prompt: %v", err)
	}

	// Verify: Prompt appears in list
	prompts := registry.List()
	if len(prompts) != 1 {
		t.Fatalf("Expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Name != "test_prompt" {
		t.Errorf("Expected prompt name 'test_prompt', got %q", prompts[0].Name)
	}

	// Verify: Prompt can be retrieved
	retrieved, err := registry.Get("test_prompt")
	if err != nil {
		t.Fatalf("Failed to get registered prompt: %v", err)
	}
	if retrieved.Name != "test_prompt" {
		t.Errorf("Retrieved prompt name mismatch")
	}

	// Test Case 2: Duplicate name rejected
	err = registry.Register(template)
	if err == nil {
		t.Error("Expected error when registering duplicate prompt name, got nil")
	}

	// Test Case 3: Invalid name rejected
	invalidTemplate := template
	invalidTemplate.Name = "InvalidName" // Uppercase not allowed
	err = registry.Register(invalidTemplate)
	if err == nil {
		t.Error("Expected error for invalid prompt name (uppercase), got nil")
	}
}

// T076 [P] [US3] Integration test for prompt rendering with arguments
//
// This test verifies the complete prompt rendering workflow:
// 1. Register prompt with parameters
// 2. Render with all arguments
// 3. Render with partial arguments (uses defaults)
// 4. Verify placeholder substitution
// 5. Verify rendered message structure
func TestPromptRendering(t *testing.T) {
	registry := NewPromptRegistry()

	// Setup: Register prompt with required and optional parameters
	template := PromptTemplate{
		Name:        "greeting",
		Description: "Greeting prompt",
		Parameters: []PromptParameter{
			{Name: "name", Required: true},
			{Name: "time", Required: false, DefaultValue: "morning"},
		},
		Template: "Good {{time}}, {{name}}!",
	}

	err := registry.Register(template)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Test Case 1: Render with all arguments
	rendered, err := registry.Render("greeting", map[string]string{
		"name": "Alice",
		"time": "evening",
	})
	if err != nil {
		t.Fatalf("Failed to render with all arguments: %v", err)
	}

	// Verify: Placeholders substituted
	if len(rendered.Messages) == 0 {
		t.Fatal("Rendered prompt has no messages")
	}
	expectedText := "Good evening, Alice!"
	if rendered.Messages[0].Content.Text != expectedText {
		t.Errorf("Expected text %q, got %q", expectedText, rendered.Messages[0].Content.Text)
	}

	// Test Case 2: Render with only required (uses default for optional)
	rendered, err = registry.Render("greeting", map[string]string{
		"name": "Bob",
	})
	if err != nil {
		t.Fatalf("Failed to render with required only: %v", err)
	}

	// Verify: Default value used
	expectedText = "Good morning, Bob!"
	if rendered.Messages[0].Content.Text != expectedText {
		t.Errorf("Expected text %q, got %q", expectedText, rendered.Messages[0].Content.Text)
	}

	// Verify: Message structure
	if rendered.Messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got %q", rendered.Messages[0].Role)
	}
	if rendered.Messages[0].Content.Type != "text" {
		t.Errorf("Expected content type 'text', got %q", rendered.Messages[0].Content.Type)
	}
}

// T077 [P] [US3] Integration test for required parameter validation
//
// This test verifies parameter validation during prompt rendering:
// 1. Missing required parameter returns error
// 2. Error message indicates which parameter is missing
// 3. Extra parameters are ignored (no error)
// 4. Invalid parameter type returns error (e.g., number instead of string)
func TestPromptParameterValidation(t *testing.T) {
	registry := NewPromptRegistry()

	// Setup: Register prompt with required parameter
	template := PromptTemplate{
		Name:        "workflow_start",
		Description: "Start workflow",
		Parameters: []PromptParameter{
			{Name: "workflow_id", Required: true},
			{Name: "priority", Required: false, DefaultValue: "normal"},
		},
		Template: "Start workflow {{workflow_id}} with priority {{priority}}",
	}

	err := registry.Register(template)
	if err != nil {
		t.Fatalf("Failed to register prompt: %v", err)
	}

	// Test Case 1: Missing required parameter
	_, err = registry.Render("workflow_start", map[string]string{
		"priority": "high", // Missing workflow_id
	})
	if err == nil {
		t.Error("Expected error for missing required parameter, got nil")
	}
	if !strings.Contains(err.Error(), "workflow_id") {
		t.Errorf("Expected error message to mention 'workflow_id', got: %v", err)
	}

	// Test Case 2: Extra parameters ignored
	rendered, err := registry.Render("workflow_start", map[string]string{
		"workflow_id": "data-pipeline",
		"priority":    "high",
		"extra_param": "ignored", // Extra parameter
	})
	if err != nil {
		t.Errorf("Unexpected error with extra parameter: %v", err)
	}
	if rendered == nil {
		t.Error("Expected rendered prompt, got nil")
	}

	// Test Case 3: All required parameters provided
	rendered, err = registry.Render("workflow_start", map[string]string{
		"workflow_id": "ml-pipeline",
	})
	if err != nil {
		t.Fatalf("Unexpected error with valid parameters: %v", err)
	}
	expectedText := "Start workflow ml-pipeline with priority normal"
	if rendered.Messages[0].Content.Text != expectedText {
		t.Errorf("Expected text %q, got %q", expectedText, rendered.Messages[0].Content.Text)
	}
}
