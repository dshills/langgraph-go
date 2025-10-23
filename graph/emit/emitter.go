package emit

// Emitter receives and processes observability events from workflow execution.
//
// Emitters enable pluggable observability backends:
//   - Logging: stdout, files, syslog
//   - Distributed tracing: OpenTelemetry, Jaeger, Zipkin
//   - Metrics: Prometheus, StatsD
//   - Analytics: DataDog, New Relic
//
// Implementations should be:
//   - Non-blocking: Avoid slowing down workflow execution
//   - Thread-safe: May be called concurrently from multiple nodes
//   - Resilient: Handle failures gracefully (don't crash workflow)
//
// Common patterns:
//   - Buffering: Collect events and flush in batches
//   - Filtering: Only emit events matching criteria (e.g., errors only)
//   - Multi-emit: Fan out to multiple backends
//   - Sampling: Emit only a percentage of events for high-volume workflows
type Emitter interface {
	// Emit sends an observability event to the configured backend.
	//
	// Implementations should not block workflow execution.
	// If the backend is unavailable or slow, events should be:
	//   - Buffered for later delivery
	//   - Dropped with error logging
	//   - Sent asynchronously
	//
	// Emit should not panic. Errors should be logged internally.
	Emit(event Event)
}
