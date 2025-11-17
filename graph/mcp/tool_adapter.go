package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/tool"
)

// Tool name pattern as defined in MCP spec: ^[a-z][a-z0-9_]*$
var toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Common errors
var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrInvalidToolName   = errors.New("invalid tool name")
	ErrDuplicateToolName = errors.New("duplicate tool name")
	ErrEmptyDescription  = errors.New("description cannot be empty")
	ErrNilInputSchema    = errors.New("inputSchema cannot be nil")
	ErrToolNameMismatch  = errors.New("tool name does not match metadata name")
)

// ToolAdapterMetadata represents metadata for tool registration in the adapter.
// This is separate from the server's ToolMetadata to follow MCP spec exactly.
type ToolAdapterMetadata struct {
	Name        string                 // Tool name (validated against pattern ^[a-z][a-z0-9_]*$)
	Description string                 // Human-readable description
	InputSchema map[string]interface{} // JSON Schema for input validation
}

// RegisteredTool wraps a tool with its metadata.
type RegisteredTool struct {
	Tool     tool.Tool
	Metadata ToolAdapterMetadata
}

// ToolResult represents the result of a tool invocation.
type ToolResult struct {
	Content []ContentBlock
	IsError bool
}

// ContentBlock represents a content item in tool results.
type ContentBlock struct {
	Type     string // "text", "image", "resource"
	Text     string
	Data     string // Base64 for images
	MimeType string
	Resource string // Resource URI
}

// ToolRegistry manages tool registration and invocation for MCP.
type ToolRegistry interface {
	// Register a LangGraph tool with MCP metadata
	Register(tool tool.Tool, metadata ToolAdapterMetadata) error

	// Get registered tool by name
	Get(name string) (*RegisteredTool, error)

	// List all tool metadata
	List() []ToolAdapterMetadata

	// Invoke tool with validated input
	Invoke(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
}

// toolRegistry is the concrete implementation of ToolRegistry.
type toolRegistry struct {
	tools   map[string]*RegisteredTool
	mu      sync.RWMutex
	emitter emit.Emitter // Optional emitter for observability
}

// NewToolRegistry creates a new tool registry instance.
func NewToolRegistry(opts ...RegistryOption) ToolRegistry {
	r := &toolRegistry{
		tools: make(map[string]*RegisteredTool),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// RegistryOption configures the tool registry.
type RegistryOption func(*toolRegistry)

// WithEmitter sets an emitter for observability events.
func WithEmitter(emitter emit.Emitter) RegistryOption {
	return func(r *toolRegistry) {
		r.emitter = emitter
	}
}

// Register implements ToolRegistry.Register (T025).
//
// Validates:
// - Tool name matches pattern ^[a-z][a-z0-9_]*$
// - Tool name matches metadata name
// - Description is non-empty
// - InputSchema is not nil
// - Tool name is unique (not already registered)
func (r *toolRegistry) Register(t tool.Tool, metadata ToolAdapterMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate tool name pattern
	if !toolNamePattern.MatchString(metadata.Name) {
		return fmt.Errorf("%w: name '%s' must match pattern ^[a-z][a-z0-9_]*$", ErrInvalidToolName, metadata.Name)
	}

	// Validate tool name matches metadata name
	if t.Name() != metadata.Name {
		return fmt.Errorf("name mismatch: tool.Name()='%s' does not match metadata.Name='%s'", t.Name(), metadata.Name)
	}

	// Validate description is non-empty
	if metadata.Description == "" {
		return fmt.Errorf("%w for tool '%s'", ErrEmptyDescription, metadata.Name)
	}

	// Validate input schema is not nil
	if metadata.InputSchema == nil {
		return fmt.Errorf("schema cannot be nil for tool '%s'", metadata.Name)
	}

	// Validate tool name is unique
	if _, exists := r.tools[metadata.Name]; exists {
		return fmt.Errorf("%w: '%s' is already registered", ErrDuplicateToolName, metadata.Name)
	}

	// Register the tool
	r.tools[metadata.Name] = &RegisteredTool{
		Tool:     t,
		Metadata: metadata,
	}

	return nil
}

// Get implements ToolRegistry.Get (T026).
//
// Retrieves a registered tool by name. Returns ErrToolNotFound if not found.
func (r *toolRegistry) Get(name string) (*RegisteredTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("%w: '%s'", ErrToolNotFound, name)
	}

	return t, nil
}

// List implements ToolRegistry.List (T027).
//
// Returns metadata for all registered tools.
func (r *toolRegistry) List() []ToolAdapterMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolAdapterMetadata, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t.Metadata)
	}

	return result
}

// Invoke implements ToolRegistry.Invoke (T028, T032, T033, T034).
//
// Validates input against JSON Schema (T032), invokes the tool,
// maps errors to JSON-RPC codes (T033), and emits observability events (T034).
func (r *toolRegistry) Invoke(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error) {
	// Get the registered tool
	registered, err := r.Get(name)
	if err != nil {
		// Tool not found maps to -32602 Invalid params (T033)
		return nil, &Error{
			Code:    ErrCodeInvalidParams,
			Message: fmt.Sprintf("Invalid params: tool '%s' not found", name),
			Data: map[string]interface{}{
				"toolName": name,
			},
		}
	}

	// Validate input against schema (T032)
	if err := validateInputSchema(arguments, registered.Metadata.InputSchema); err != nil {
		// Validation errors map to -32602 Invalid params (T033)
		return nil, &Error{
			Code:    ErrCodeInvalidParams,
			Message: fmt.Sprintf("Invalid params: %s", err.Error()),
			Data: map[string]interface{}{
				"toolName": name,
				"error":    err.Error(),
			},
		}
	}

	// Emit tool_call_start event (T034)
	if r.emitter != nil {
		r.emitter.Emit(emit.Event{
			RunID:  getRunIDFromContext(ctx),
			NodeID: name,
			Msg:    "tool_call_start",
			Meta: map[string]interface{}{
				"tool_name": name,
				"arguments": arguments,
			},
		})
	}

	// Invoke the tool
	output, err := registered.Tool.Call(ctx, arguments)

	// Emit tool_call_end event (T034)
	if r.emitter != nil {
		event := emit.Event{
			RunID:  getRunIDFromContext(ctx),
			NodeID: name,
			Msg:    "tool_call_end",
			Meta: map[string]interface{}{
				"tool_name": name,
			},
		}

		if err != nil {
			event = event.WithError(err)
		} else {
			event.Meta["output"] = output
		}

		r.emitter.Emit(event)
	}

	// Handle tool execution errors (T033)
	if err != nil {
		// Check for context cancellation - return the context error directly
		// so errors.Is() can detect it properly
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		// Check if context was cancelled during execution
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Tool execution errors map to -32603 Internal error (T033)
		return nil, &Error{
			Code:    ErrCodeInternalError,
			Message: "Internal error: tool execution failed",
			Data: map[string]interface{}{
				"toolName": name,
				"error":    err.Error(),
			},
		}
	}

	// Convert tool output to ToolResult
	result := formatToolOutput(output)
	return result, nil
}

// validateInputSchema validates input arguments against a JSON Schema (T032).
//
// Performs basic JSON Schema validation:
// - Required fields are present
// - Types match schema types
// - Values conform to constraints (enum, min, max, etc.)
func validateInputSchema(arguments map[string]interface{}, schema map[string]interface{}) error {
	// If arguments is nil, treat as empty object
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	// Extract schema properties
	properties, _ := schema["properties"].(map[string]interface{})
	required, _ := schema["required"].([]interface{})

	// Validate required fields
	for _, req := range required {
		fieldName, ok := req.(string)
		if !ok {
			continue
		}

		if _, exists := arguments[fieldName]; !exists {
			return fmt.Errorf("missing required parameter '%s'", fieldName)
		}
	}

	// Validate types and constraints for provided arguments
	for key, value := range arguments {
		propSchema, exists := properties[key]
		if !exists {
			// Skip validation for properties not in schema
			// (additionalProperties handling could be added here)
			continue
		}

		propMap, ok := propSchema.(map[string]interface{})
		if !ok {
			continue
		}

		// Validate type
		if err := validateType(value, propMap); err != nil {
			return fmt.Errorf("parameter '%s' %s", key, err.Error())
		}

		// Validate enum constraint
		if err := validateEnum(value, propMap); err != nil {
			return fmt.Errorf("parameter '%s' %s", key, err.Error())
		}

		// Validate numeric constraints
		if err := validateNumericConstraints(value, propMap); err != nil {
			return fmt.Errorf("parameter '%s' %s", key, err.Error())
		}
	}

	return nil
}

// validateType validates that a value matches the expected JSON Schema type.
func validateType(value interface{}, schema map[string]interface{}) error {
	expectedType, ok := schema["type"].(string)
	if !ok {
		return nil // No type constraint
	}

	actualType := getJSONType(value)
	if actualType != expectedType {
		return fmt.Errorf("must be %s", expectedType)
	}

	return nil
}

// validateEnum validates that a value is in the allowed enum values.
func validateEnum(value interface{}, schema map[string]interface{}) error {
	enumValues, ok := schema["enum"].([]interface{})
	if !ok {
		return nil // No enum constraint
	}

	for _, allowed := range enumValues {
		if value == allowed {
			return nil
		}
	}

	return fmt.Errorf("must be one of %v", enumValues)
}

// validateNumericConstraints validates numeric min/max constraints.
func validateNumericConstraints(value interface{}, schema map[string]interface{}) error {
	// Convert value to float64 for numeric comparison
	var num float64
	switch v := value.(type) {
	case float64:
		num = v
	case float32:
		num = float64(v)
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	case int32:
		num = float64(v)
	default:
		return nil // Not a numeric type
	}

	// Check minimum
	if min, ok := schema["minimum"].(float64); ok {
		if num < min {
			return fmt.Errorf("must be >= %v", min)
		}
	}

	// Check maximum
	if max, ok := schema["maximum"].(float64); ok {
		if num > max {
			return fmt.Errorf("must be <= %v", max)
		}
	}

	return nil
}

// getJSONType returns the JSON Schema type name for a Go value.
func getJSONType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// formatToolOutput converts tool output to MCP ToolResult format.
//
// Serializes the output as JSON and wraps it in a text ContentBlock.
func formatToolOutput(output map[string]interface{}) *ToolResult {
	// Serialize output to JSON
	data, err := json.Marshal(output)
	if err != nil {
		// If marshaling fails, return error as text
		return &ToolResult{
			Content: []ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf("Error serializing tool output: %v", err),
				},
			},
			IsError: true,
		}
	}

	return &ToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: string(data),
			},
		},
		IsError: false,
	}
}

// getRunIDFromContext extracts the run ID from context for observability.
// Returns empty string if not found.
func getRunIDFromContext(ctx context.Context) string {
	runID, ok := ctx.Value("runID").(string)
	if !ok {
		return ""
	}
	return runID
}
