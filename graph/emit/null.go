package emit

import "context"

// NullEmitter implements Emitter by discarding all events (T165).
//
// This is a no-op emitter for production environments where event
// logging is not desired. It implements the Emitter interface but
// does nothing with emitted events.
//
// Use cases:
//   - Production deployments where observability overhead is unwanted
//   - Testing scenarios where event capture is not needed
//   - Disabling event emission without changing code
//
// Example usage:
//
//	// Disable all event logging
//	emitter := emit.NewNullEmitter()
//	engine := graph.New(reducer, store, emitter, opts)
type NullEmitter struct{}

// NewNullEmitter creates a new NullEmitter (T165).
//
// Returns a NullEmitter that discards all events without any processing.
// This is safe for concurrent use and has zero overhead.
func NewNullEmitter() *NullEmitter {
	return &NullEmitter{}
}

// Emit discards the event without any processing (T165).
//
// This method is a no-op that immediately returns. It never errors
// and performs no I/O or processing.
func (n *NullEmitter) Emit(event Event) {
	// No-op: discard the event
}

// TODO: Implement in Phase 8
// EmitBatch discards multiple events in a single operation.
func (n *NullEmitter) EmitBatch(ctx context.Context, events []Event) error {
	// No-op: discard all events
	return nil
}

// TODO: Implement in Phase 8
// Flush is a no-op for null emitter.
func (n *NullEmitter) Flush(ctx context.Context) error {
	// No-op: nothing to flush
	return nil
}
