// Package mcp provides MCP (Model Context Protocol) server implementation for LangGraph workflows.
//
// This file implements prompt template management, including registration, listing, and rendering
// with parameter substitution.
package mcp

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// T078 [P] [US3] Define PromptTemplate struct in `graph/mcp/prompt_registry.go`
//
// PromptTemplate represents a reusable prompt template with parameter substitution.
// Templates use {{parameter}} placeholder syntax for variable substitution.
//
// Contract: contracts/prompt-registry.md
// Data Model: data-model.md#PromptTemplate
type PromptTemplate struct {
	// Name: Unique prompt identifier
	// MUST match pattern: ^[a-z][a-z0-9_]*$ (lowercase, underscores)
	// Examples: "start_workflow", "resume_checkpoint", "analyze_results"
	Name string

	// Description: Human-readable description of prompt purpose
	// MUST be non-empty
	Description string

	// Parameters: Template parameters (optional, can be empty array)
	// Defines which placeholders are available in the template
	Parameters []PromptParameter

	// Template: Template string with {{param}} placeholders
	// MUST be non-empty
	// Placeholders are substituted during rendering
	// Example: "Start workflow '{{workflow_id}}' with input: {{input_data}}"
	Template string
}

// T079 [P] [US3] Define PromptParameter struct in `graph/mcp/prompt_registry.go`
//
// PromptParameter defines metadata for a template parameter.
//
// Contract: contracts/prompt-registry.md#PromptParameter
// Data Model: data-model.md#PromptParameter
type PromptParameter struct {
	// Name: Parameter name (used in {{name}} placeholders)
	// SHOULD match pattern: ^[a-z][a-z0-9_]*$ (recommended)
	Name string

	// Description: Parameter description (optional)
	// Helps LLM understand parameter purpose
	Description string

	// Required: Whether parameter must be provided during rendering
	// If true and parameter not provided, Render() returns error
	Required bool

	// DefaultValue: Default value if parameter not provided (optional)
	// Only used when Required=false
	// If empty and parameter not provided, placeholder remains empty
	DefaultValue string
}

// T080 [P] [US3] Define PromptRegistry struct in `graph/mcp/prompt_registry.go`
//
// PromptRegistry manages prompt templates with thread-safe operations.
// Supports registration, retrieval, listing, and rendering of prompts.
//
// Thread Safety: Uses sync.RWMutex for concurrent access
// - Register: Write lock (infrequent, during initialization)
// - Get, List, Render: Read lock (frequent, during requests)
//
// Contract: contracts/prompt-registry.md#Prompt Registration
// Data Model: data-model.md#PromptRegistry
type PromptRegistry struct {
	// prompts: Map of prompt name → template
	// Protected by mu for concurrent access
	prompts map[string]*PromptTemplate

	// mu: Protects concurrent access to prompts map
	// Read-write mutex optimized for frequent reads, rare writes
	mu sync.RWMutex
}

// PromptInfo represents metadata for a registered prompt (returned by List()).
// Excludes the template string to reduce response size.
type PromptInfo struct {
	Name        string
	Description string
	Arguments   []PromptParameter
}

// RenderedPrompt represents a fully rendered prompt ready for LLM consumption.
// Returned by Render() after parameter substitution.
type RenderedPrompt struct {
	Description string
	Messages    []Message
}

// Message represents a single message in a rendered prompt.
// Follows MCP protocol message structure.
type Message struct {
	Role    string  // "user" or "assistant"
	Content Content // Message content
}

// Content represents message content with different types.
// Type determines which fields are populated.
type Content struct {
	Type     string // "text", "image", or "resource"
	Text     string // For type="text"
	Data     string // For type="image" (base64 encoded)
	MimeType string // For type="image"
	Resource string // For type="resource" (resource URI)
}

// promptNamePattern validates prompt names against MCP specification.
// Pattern: ^[a-z][a-z0-9_]*$ (lowercase, underscores)
var promptNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// placeholderPattern extracts placeholders from templates.
// Pattern: {{\s*([a-z][a-z0-9_]*)\s*}} (allows whitespace around parameter name)
var placeholderPattern = regexp.MustCompile(`{{\s*([a-z][a-z0-9_]*)\s*}}`)

// Errors returned by PromptRegistry operations
var (
	ErrPromptNotFound         = errors.New("prompt not found")
	ErrPromptAlreadyExists    = errors.New("prompt already registered")
	ErrInvalidPromptName      = errors.New("prompt name must match pattern ^[a-z][a-z0-9_]*$")
	ErrInvalidParameterName   = errors.New("parameter name must match pattern ^[a-z][a-z0-9_]*$")
	ErrDuplicateParameterName = errors.New("duplicate parameter name")
	ErrEmptyTemplate          = errors.New("prompt template cannot be empty")
	ErrMissingParameter       = errors.New("missing required parameter")
	ErrUndefinedPlaceholder   = errors.New("template contains placeholder with no matching parameter")
	// Note: ErrEmptyDescription is defined in tool_adapter.go and reused here
)

// NewPromptRegistry creates a new empty prompt registry.
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		prompts: make(map[string]*PromptTemplate),
	}
}

// T081 [US3] Implement PromptRegistry.Register method
//
// Register adds a prompt template to the registry with validation.
//
// Validation:
// - Prompt name MUST match pattern ^[a-z][a-z0-9_]*$
// - Prompt name MUST be unique (no duplicates)
// - Description MUST be non-empty
// - Template MUST be non-empty
// - Parameter names MUST match pattern ^[a-z][a-z0-9_]*$
// - Parameter names MUST be unique (no duplicates)
// - All placeholders in template MUST have corresponding parameters
//
// Returns error if validation fails.
// Thread-safe: Uses write lock.
func (r *PromptRegistry) Register(template PromptTemplate) error {
	// Validate prompt name pattern
	if !promptNamePattern.MatchString(template.Name) {
		return ErrInvalidPromptName
	}

	// Validate description
	if template.Description == "" {
		return ErrEmptyDescription
	}

	// Validate template
	if template.Template == "" {
		return ErrEmptyTemplate
	}

	// Validate parameters: check names and detect duplicates
	paramNames := make(map[string]bool)
	for _, param := range template.Parameters {
		// Validate parameter name pattern
		if param.Name != "" && !promptNamePattern.MatchString(param.Name) {
			return fmt.Errorf("%w: %s", ErrInvalidParameterName, param.Name)
		}

		// Check for duplicate parameter names
		if paramNames[param.Name] {
			return fmt.Errorf("%w: %s", ErrDuplicateParameterName, param.Name)
		}
		paramNames[param.Name] = true
	}

	// Extract all placeholders from template and verify they have matching parameters
	matches := placeholderPattern.FindAllStringSubmatch(template.Template, -1)
	for _, match := range matches {
		if len(match) > 1 {
			placeholderName := match[1] // First capture group contains parameter name
			if !paramNames[placeholderName] {
				return fmt.Errorf("%w: {{%s}}", ErrUndefinedPlaceholder, placeholderName)
			}
		}
	}

	// Acquire write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate name
	if _, exists := r.prompts[template.Name]; exists {
		return ErrPromptAlreadyExists
	}

	// Store template (make a deep copy to avoid external mutations)
	templateCopy := template
	// Deep copy Parameters slice
	paramsCopy := make([]PromptParameter, len(template.Parameters))
	copy(paramsCopy, template.Parameters)
	templateCopy.Parameters = paramsCopy
	r.prompts[template.Name] = &templateCopy

	return nil
}

// T082 [US3] Implement PromptRegistry.Get method
//
// Get retrieves a prompt template by name.
//
// Returns:
// - *PromptTemplate: The template (read-only copy)
// - error: ErrPromptNotFound if name doesn't exist
//
// Thread-safe: Uses read lock.
func (r *PromptRegistry) Get(name string) (*PromptTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, exists := r.prompts[name]
	if !exists {
		return nil, ErrPromptNotFound
	}

	// Return deep copy to prevent external mutations
	templateCopy := *template
	// Deep copy Parameters slice
	paramsCopy := make([]PromptParameter, len(template.Parameters))
	copy(paramsCopy, template.Parameters)
	templateCopy.Parameters = paramsCopy
	return &templateCopy, nil
}

// T083 [US3] Implement PromptRegistry.List method
//
// List returns metadata for all registered prompts.
// Excludes template string to reduce response size.
// Results are sorted by name for consistency.
//
// Returns:
// - []PromptInfo: Array of prompt metadata (may be empty)
//
// Thread-safe: Uses read lock.
func (r *PromptRegistry) List() []PromptInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]PromptInfo, 0, len(r.prompts))
	for _, template := range r.prompts {
		// Deep copy Parameters slice to prevent external mutations
		paramsCopy := make([]PromptParameter, len(template.Parameters))
		copy(paramsCopy, template.Parameters)

		infos = append(infos, PromptInfo{
			Name:        template.Name,
			Description: template.Description,
			Arguments:   paramsCopy,
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})

	return infos
}

// T084 [US3] Implement PromptRegistry.Render method (substitute {{param}} placeholders)
//
// Render generates a fully rendered prompt by substituting template placeholders.
//
// Placeholder syntax: {{parameter_name}} or {{ parameter_name }} (whitespace allowed)
//
// Parameter substitution rules:
// 1. Required parameters MUST be present in arguments
// 2. Optional parameters use DefaultValue if not provided
// 3. Extra arguments (not in parameters) are ignored
// 4. All parameter values MUST be strings
//
// Placeholders are matched using regex to support whitespace variants.
// Substitution is performed in a single pass using strings.Replacer for efficiency.
//
// Returns:
// - *RenderedPrompt: Rendered prompt with messages
// - error: ErrPromptNotFound, ErrMissingParameter, etc.
//
// Thread-safe: Uses read lock.
func (r *PromptRegistry) Render(name string, arguments map[string]string) (*RenderedPrompt, error) {
	// Get template (with read lock)
	template, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	// Validate required parameters and build argument map
	argMap := make(map[string]string)
	for _, param := range template.Parameters {
		value, provided := arguments[param.Name]
		if !provided {
			// Parameter not provided
			if param.Required {
				return nil, fmt.Errorf("%w: %s", ErrMissingParameter, param.Name)
			}
			// Use default value
			value = param.DefaultValue
		}
		argMap[param.Name] = value
	}

	// Substitute placeholders using regex replacement function
	// This supports whitespace variants like {{param}}, {{ param }}, etc.
	rendered := placeholderPattern.ReplaceAllStringFunc(template.Template, func(match string) string {
		// Extract parameter name from match (e.g., "{{ param }}" -> "param")
		submatches := placeholderPattern.FindStringSubmatch(match)
		if len(submatches) > 1 {
			paramName := submatches[1]
			if value, ok := argMap[paramName]; ok {
				return value
			}
		}
		// If parameter not found in argMap, return original match
		// (This shouldn't happen due to validation in Register(), but be defensive)
		return match
	})

	// Create rendered prompt with single user message (text content)
	return &RenderedPrompt{
		Description: template.Description,
		Messages: []Message{
			{
				Role: "user",
				Content: Content{
					Type: "text",
					Text: rendered,
				},
			},
		},
	}, nil
}
