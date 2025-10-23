package emit

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

	// Meta contains additional structured data specific to this event.
	// Common keys:
	//   - "duration_ms": Execution duration in milliseconds
	//   - "error": Error details
	//   - "tokens": Token count for LLM calls
	//   - "checkpoint_id": Checkpoint identifier
	//   - "retryable": Whether an error can be retried
	Meta map[string]interface{}
}
