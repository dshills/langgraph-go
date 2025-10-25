package emit

import "time"

// Event represents an observability event emitted during workflow execution.
//
// Events provide detailed insight into workflow behavior:
//   - Node execution start/complete
//   - State changes and transitions
//   - Errors and warnings
//   - Performance metrics
//   - Checkpoint operations
//
// Events are emitted to an Emitter which can:
//   - Log to stdout/stderr
//   - Send to OpenTelemetry
//   - Store in time-series databases
//   - Trigger alerts
type Event struct {
	// RunID identifies the workflow execution that emitted this event.
	RunID string

	// Step is the sequential step number in the workflow (1-indexed).
	// Zero for workflow-level events (start, complete, error).
	Step int

	// NodeID identifies which node emitted this event.
	// Empty string for workflow-level events.
	NodeID string

	// Msg is a human-readable description of the event.
	Msg string

	// Meta contains additional structured data specific to this event (T168).
	//
	// Standard Metadata Conventions:
	//
	// Performance Metrics:
	//   - "duration_ms" (int64): Execution duration in milliseconds
	//   - "memory_bytes" (int64): Memory usage in bytes
	//   - "cpu_percent" (float64): CPU utilization percentage
	//
	// Error Context:
	//   - "error" (string): Error message from error.Error()
	//   - "error_type" (string): Error type or classification (e.g., "validation", "timeout")
	//   - "retryable" (bool): Whether the error can be retried
	//   - "retry_attempt" (int): Current retry attempt number (1-indexed)
	//   - "stack_trace" (string): Optional stack trace for debugging
	//
	// LLM-Specific:
	//   - "tokens" (int): Total token count (input + output)
	//   - "input_tokens" (int): Input token count
	//   - "output_tokens" (int): Output token count
	//   - "cost" (float64): Estimated cost in USD
	//   - "model" (string): Model identifier (e.g., "gpt-4", "claude-3-opus")
	//   - "temperature" (float64): Temperature parameter used
	//   - "max_tokens" (int): Max tokens parameter used
	//   - "finish_reason" (string): Completion reason (e.g., "stop", "length", "tool_calls")
	//
	// Node Classification:
	//   - "node_type" (string): Node category (e.g., "llm", "tool", "processor", "validator")
	//   - "node_version" (string): Node implementation version
	//
	// Checkpoint Context:
	//   - "checkpoint_id" (string): Checkpoint identifier
	//   - "checkpoint_label" (string): Human-readable checkpoint label
	//   - "state_size" (int): Serialized state size in bytes
	//
	// Tool Execution:
	//   - "tool_name" (string): Name of the tool being executed
	//   - "tool_input" (any): Tool input parameters (if serializable)
	//   - "tool_output" (any): Tool output result (if serializable)
	//
	// State Changes:
	//   - "delta" (any): State change applied by the node
	//   - "state_version" (int): State version number
	//
	// Routing Decisions:
	//   - "next_node" (string): Next node ID for routing decisions
	//   - "next_nodes" ([]string): Multiple next nodes for fan-out
	//   - "routing_reason" (string): Explanation of routing decision
	//   - "condition" (string): Condition that triggered the route
	//
	// Custom Application Data:
	// Applications can add domain-specific metadata using their own keys.
	// Use namespaced keys to avoid conflicts (e.g., "app_user_id", "app_request_id").
	//
	// Usage with helper methods:
	//
	//	event := Event{RunID: "run-001", NodeID: "llm-node", Msg: "node_end"}
	//	enriched := event.
	//		WithDuration(250 * time.Millisecond).
	//		WithNodeType("llm").
	//		WithMeta("tokens", 150).
	//		WithMeta("cost", 0.003).
	//		WithMeta("model", "gpt-4")
	Meta map[string]interface{}
}

// WithDuration returns a copy of the event with duration_ms metadata (T166).
//
// Sets the "duration_ms" field to the duration in milliseconds as an int64.
// Preserves all existing metadata fields.
//
// Example:
//
//	event := Event{RunID: "run-001", Msg: "node_end"}
//	enriched := event.WithDuration(250 * time.Millisecond)
//	// enriched.Meta["duration_ms"] == 250
func (e Event) WithDuration(d time.Duration) Event {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta["duration_ms"] = int64(d / time.Millisecond)
	return e
}

// WithError returns a copy of the event with error metadata (T166).
//
// Sets the "error" field to the error message string.
// Preserves all existing metadata fields.
//
// Example:
//
//	event := Event{RunID: "run-001", Msg: "error"}
//	enriched := event.WithError(errors.New("validation failed"))
//	// enriched.Meta["error"] == "validation failed"
func (e Event) WithError(err error) Event {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta["error"] = err.Error()
	return e
}

// WithNodeType returns a copy of the event with node_type metadata (T167).
//
// Sets the "node_type" field to the provided node type string.
// Preserves all existing metadata fields.
//
// Common node types:
//   - "llm": LLM call nodes
//   - "tool": Tool execution nodes
//   - "processor": Data processing nodes
//   - "validator": Validation nodes
//   - "aggregator": Result aggregation nodes
//
// Example:
//
//	event := Event{RunID: "run-001", NodeID: "llm-node"}
//	enriched := event.WithNodeType("llm")
//	// enriched.Meta["node_type"] == "llm"
func (e Event) WithNodeType(nodeType string) Event {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta["node_type"] = nodeType
	return e
}

// WithMeta returns a copy of the event with an additional metadata field (T167).
//
// Sets the specified key-value pair in the metadata map.
// Preserves all existing metadata fields.
// If the key already exists, it will be overwritten.
//
// This method supports chaining for fluent API usage:
//
//	event := Event{RunID: "run-001"}
//	enriched := event.
//		WithMeta("tokens", 150).
//		WithMeta("cost", 0.003).
//		WithMeta("model", "gpt-4")
//
// Example:
//
//	event := Event{RunID: "run-001", Msg: "llm_call"}
//	enriched := event.WithMeta("tokens", 150)
//	// enriched.Meta["tokens"] == 150
func (e Event) WithMeta(key string, value interface{}) Event {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta[key] = value
	return e
}
