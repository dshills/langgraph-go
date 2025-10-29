// Package emit provides event emission and observability for graph execution.
package emit

import (
	"testing"
)

// TestNullEmitter_NoOp verifies NullEmitter discards all events without errors (T164).
func TestNullEmitter_NoOp(t *testing.T) {
	t.Run("emits events without error", func(t *testing.T) {
		emitter := NewNullEmitter()

		// Emit several events - should not panic or error.
		events := []Event{
			{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_start"},
			{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_end"},
			{RunID: "run-001", Step: 1, NodeID: "node2", Msg: "error", Meta: map[string]interface{}{"error": "test"}},
		}

		for _, event := range events {
			// Should not panic.
			emitter.Emit(event)
		}

		t.Log("NullEmitter successfully discarded all events")
	})

	t.Run("can emit with nil meta", func(t *testing.T) {
		emitter := NewNullEmitter()

		event := Event{
			RunID:  "run-001",
			Step:   0,
			NodeID: "node1",
			Msg:    "test",
			Meta:   nil, // nil meta should be fine
		}

		// Should not panic.
		emitter.Emit(event)

		t.Log("NullEmitter handled nil meta without error")
	})
}

// TestNullEmitter_InterfaceContract verifies NullEmitter implements Emitter interface (T164).
func TestNullEmitter_InterfaceContract(t *testing.T) {
	var _ Emitter = NewNullEmitter()
}
