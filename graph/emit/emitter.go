// Package emit provides event emission and observability for graph execution.
package emit

import "context"

// Emitter receives and processes observability events from workflow execution.
//
// Emitters enable pluggable observability backends:
// - Logging: stdout, files, syslog.
// - Distributed tracing: OpenTelemetry, Jaeger, Zipkin.
// - Metrics: Prometheus, StatsD.
// - Analytics: DataDog, New Relic.
//
// Implementations should be:
// - Non-blocking: Avoid slowing down workflow execution.
// - Thread-safe: May be called concurrently from multiple nodes.
// - Resilient: Handle failures gracefully (don't crash workflow).
//
// Common patterns:
// - Buffering: Collect events and flush in batches.
// - Filtering: Only emit events matching criteria (e.g., errors only).
// - Multi-emit: Fan out to multiple backends.
// - Sampling: Emit only a percentage of events for high-volume workflows.
type Emitter interface {
	// Emit sends an observability event to the configured backend.
	//
	// Implementations should not block workflow execution.
	// If the backend is unavailable or slow, events should be:
	// - Buffered for later delivery.
	// - Dropped with error logging.
	// - Sent asynchronously.
	//
	// Emit should not panic. Errors should be logged internally.
	Emit(event Event)

	// EmitBatch sends multiple events in a single operation for improved performance.
	//
	// Batching reduces overhead when emitting high volumes of events by:
	// - Amortizing network round-trips across multiple events.
	// - Reducing serialization overhead.
	// - Enabling backend bulk insert optimizations.
	// - Improving throughput for high-concurrency workflows.
	//
	// Implementations should:
	// - Process events in order (maintain happened-before relationships).
	// - Not block workflow execution (buffer or process asynchronously).
	// - Handle partial failures gracefully (log and continue).
	// - Not panic on errors.
	//
	// Parameters:
	// - ctx: Context for cancellation and timeouts.
	// - events: Events to emit, ordered by creation time.
	//
	// Returns error only on catastrophic failures (e.g., configuration errors).
	// Individual event failures should be logged but not returned.
	//
	// Example usage:
	//
	// events := []Event{.
	//	    {RunID: "run-001", Msg: "step_start", ...},
	//	    {RunID: "run-001", Msg: "step_complete", ...},
	// }.
	// if err := emitter.EmitBatch(ctx, events); err != nil {.
	// log.Errorf("batch emit failed: %v", err).
	// }.
	EmitBatch(ctx context.Context, events []Event) error

	// Flush ensures all buffered events are sent to the backend.
	//
	// Call this method:
	// - Before application shutdown to prevent event loss.
	// - At workflow completion to ensure all events are delivered.
	// - After critical operations requiring immediate visibility.
	// - During testing to verify event emission.
	//
	// Implementations should:
	// - Block until all buffered events are sent or timeout occurs.
	// - Respect context cancellation and deadlines.
	// - Return error if events cannot be delivered.
	// - Be safe to call multiple times (idempotent).
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout.
	//
	// Returns error if flush fails or times out. Implementations should attempt.
	// best-effort delivery even on error (e.g., flush partial buffers).
	//
	// Example usage:
	//
	// defer func() {.
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second).
	// defer cancel().
	// if err := emitter.Flush(ctx); err != nil {.
	// log.Errorf("failed to flush events on shutdown: %v", err).
	// }.
	// }().
	Flush(ctx context.Context) error
}
