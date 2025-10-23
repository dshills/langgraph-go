package emit

import (
	"testing"
)

// TestEmitter_InterfaceContract verifies Emitter interface can be implemented (T031).
func TestEmitter_InterfaceContract(t *testing.T) {
	// Verify interface can be declared
	var _ Emitter = (*mockEmitter)(nil)
}

// mockEmitter is a minimal Emitter implementation for testing the interface contract.
type mockEmitter struct {
	events []Event
}

func (m *mockEmitter) Emit(event Event) {
	if m.events == nil {
		m.events = make([]Event, 0)
	}
	m.events = append(m.events, event)
}

// TestEmitter_Emit verifies Emit method behavior.
func TestEmitter_Emit(t *testing.T) {
	t.Run("emit single event", func(t *testing.T) {
		emitter := &mockEmitter{}

		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "node1",
			Msg:    "Test event",
		}

		emitter.Emit(event)

		if len(emitter.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(emitter.events))
		}
		if emitter.events[0].Msg != "Test event" {
			t.Errorf("expected Msg = 'Test event', got %q", emitter.events[0].Msg)
		}
	})

	t.Run("emit multiple events", func(t *testing.T) {
		emitter := &mockEmitter{}

		events := []Event{
			{RunID: "run-001", Step: 1, Msg: "Event 1"},
			{RunID: "run-001", Step: 2, Msg: "Event 2"},
			{RunID: "run-001", Step: 3, Msg: "Event 3"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		if len(emitter.events) != 3 {
			t.Fatalf("expected 3 events, got %d", len(emitter.events))
		}

		for i, event := range emitter.events {
			expectedStep := i + 1
			if event.Step != expectedStep {
				t.Errorf("event %d: expected Step = %d, got %d", i, expectedStep, event.Step)
			}
		}
	})

	t.Run("emit with metadata", func(t *testing.T) {
		emitter := &mockEmitter{}

		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "llm",
			Msg:    "LLM call completed",
			Meta: map[string]interface{}{
				"tokens":      150,
				"duration_ms": 250,
			},
		}

		emitter.Emit(event)

		if len(emitter.events) != 1 {
			t.Fatal("expected 1 event")
		}

		meta := emitter.events[0].Meta
		if meta["tokens"] != 150 {
			t.Errorf("expected tokens = 150, got %v", meta["tokens"])
		}
		if meta["duration_ms"] != 250 {
			t.Errorf("expected duration_ms = 250, got %v", meta["duration_ms"])
		}
	})

	t.Run("emit zero value event", func(t *testing.T) {
		emitter := &mockEmitter{}

		// Zero value event should be accepted (no panic)
		emitter.Emit(Event{})

		if len(emitter.events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(emitter.events))
		}
	})
}

// TestEmitter_Patterns verifies common emitter patterns.
func TestEmitter_Patterns(t *testing.T) {
	t.Run("buffering emitter", func(t *testing.T) {
		// Emitters can buffer events before flushing
		emitter := &mockEmitter{
			events: make([]Event, 0, 10), // pre-allocated buffer
		}

		for i := 1; i <= 5; i++ {
			emitter.Emit(Event{
				RunID: "run-001",
				Step:  i,
				Msg:   "Event",
			})
		}

		if len(emitter.events) != 5 {
			t.Errorf("expected 5 buffered events, got %d", len(emitter.events))
		}
	})

	t.Run("filtering emitter", func(t *testing.T) {
		// Emitters can filter events based on criteria
		type filteringEmitter struct {
			events      []Event
			minLogLevel string
		}

		emitter := &filteringEmitter{
			events:      make([]Event, 0),
			minLogLevel: "ERROR",
		}

		// Only emit ERROR level events
		emit := func(event Event) {
			level, ok := event.Meta["level"].(string)
			if ok && level == "ERROR" {
				emitter.events = append(emitter.events, event)
			}
		}

		emit(Event{
			Msg:  "Debug message",
			Meta: map[string]interface{}{"level": "DEBUG"},
		})
		emit(Event{
			Msg:  "Error message",
			Meta: map[string]interface{}{"level": "ERROR"},
		})

		if len(emitter.events) != 1 {
			t.Errorf("expected 1 ERROR event, got %d", len(emitter.events))
		}
		if emitter.events[0].Msg != "Error message" {
			t.Errorf("expected 'Error message', got %q", emitter.events[0].Msg)
		}
	})
}
