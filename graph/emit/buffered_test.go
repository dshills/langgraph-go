// Package emit provides event emission and observability for graph execution.
package emit

import (
	"testing"
	"time"
)

// TestBufferedEmitter_StoresEvents verifies BufferedEmitter stores emitted events (T169).
func TestBufferedEmitter_StoresEvents(t *testing.T) {
	t.Run("stores single event", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "node1",
			Msg:    "node_start",
		}

		emitter.Emit(event)

		history := emitter.GetHistory("run-001")
		if len(history) != 1 {
			t.Fatalf("expected 1 event, got %d", len(history))
		}
		if history[0].NodeID != "node1" {
			t.Errorf("expected NodeID = 'node1', got %q", history[0].NodeID)
		}
	})

	t.Run("stores multiple events", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_start"},
			{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_end"},
			{RunID: "run-001", Step: 1, NodeID: "node2", Msg: "node_start"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		history := emitter.GetHistory("run-001")
		if len(history) != 3 {
			t.Fatalf("expected 3 events, got %d", len(history))
		}
	})

	t.Run("isolates events by runID", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		emitter.Emit(Event{RunID: "run-001", Msg: "event1"})
		emitter.Emit(Event{RunID: "run-002", Msg: "event2"})
		emitter.Emit(Event{RunID: "run-001", Msg: "event3"})

		history1 := emitter.GetHistory("run-001")
		history2 := emitter.GetHistory("run-002")

		if len(history1) != 2 {
			t.Errorf("expected 2 events for run-001, got %d", len(history1))
		}
		if len(history2) != 1 {
			t.Errorf("expected 1 event for run-002, got %d", len(history2))
		}
	})

	t.Run("returns empty slice for unknown runID", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		history := emitter.GetHistory("unknown-run")
		if history == nil {
			t.Error("expected empty slice, got nil")
		}
		if len(history) != 0 {
			t.Errorf("expected 0 events, got %d", len(history))
		}
	})
}

// TestBufferedEmitter_GetHistoryWithFilter verifies event filtering (T171, T172).
func TestBufferedEmitter_GetHistoryWithFilter(t *testing.T) {
	t.Run("filters by nodeID", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", NodeID: "node1", Msg: "event1"},
			{RunID: "run-001", NodeID: "node2", Msg: "event2"},
			{RunID: "run-001", NodeID: "node1", Msg: "event3"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		filter := HistoryFilter{NodeID: "node1"}
		history := emitter.GetHistoryWithFilter("run-001", filter)

		if len(history) != 2 {
			t.Fatalf("expected 2 events, got %d", len(history))
		}
		for _, event := range history {
			if event.NodeID != "node1" {
				t.Errorf("expected NodeID = 'node1', got %q", event.NodeID)
			}
		}
	})

	t.Run("filters by message", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", Msg: "node_start"},
			{RunID: "run-001", Msg: "node_end"},
			{RunID: "run-001", Msg: "node_start"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		filter := HistoryFilter{Msg: "node_start"}
		history := emitter.GetHistoryWithFilter("run-001", filter)

		if len(history) != 2 {
			t.Fatalf("expected 2 events, got %d", len(history))
		}
		for _, event := range history {
			if event.Msg != "node_start" {
				t.Errorf("expected Msg = 'node_start', got %q", event.Msg)
			}
		}
	})

	t.Run("filters by step range", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", Step: 0, Msg: "event0"},
			{RunID: "run-001", Step: 1, Msg: "event1"},
			{RunID: "run-001", Step: 2, Msg: "event2"},
			{RunID: "run-001", Step: 3, Msg: "event3"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		minStep := 1
		maxStep := 2
		filter := HistoryFilter{MinStep: &minStep, MaxStep: &maxStep}
		history := emitter.GetHistoryWithFilter("run-001", filter)

		if len(history) != 2 {
			t.Fatalf("expected 2 events, got %d", len(history))
		}
		if history[0].Step != 1 || history[1].Step != 2 {
			t.Error("expected steps 1 and 2")
		}
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", Step: 1, NodeID: "node1", Msg: "node_start"},
			{RunID: "run-001", Step: 1, NodeID: "node2", Msg: "node_start"},
			{RunID: "run-001", Step: 2, NodeID: "node1", Msg: "node_start"},
			{RunID: "run-001", Step: 1, NodeID: "node1", Msg: "node_end"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		step := 1
		filter := HistoryFilter{
			NodeID:  "node1",
			Msg:     "node_start",
			MinStep: &step,
			MaxStep: &step,
		}
		history := emitter.GetHistoryWithFilter("run-001", filter)

		if len(history) != 1 {
			t.Fatalf("expected 1 event, got %d", len(history))
		}
		if history[0].Step != 1 || history[0].NodeID != "node1" || history[0].Msg != "node_start" {
			t.Error("expected event with step=1, nodeID=node1, msg=node_start")
		}
	})

	t.Run("empty filter returns all events", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		events := []Event{
			{RunID: "run-001", Msg: "event1"},
			{RunID: "run-001", Msg: "event2"},
			{RunID: "run-001", Msg: "event3"},
		}

		for _, event := range events {
			emitter.Emit(event)
		}

		filter := HistoryFilter{}
		history := emitter.GetHistoryWithFilter("run-001", filter)

		if len(history) != 3 {
			t.Fatalf("expected 3 events, got %d", len(history))
		}
	})
}

// TestBufferedEmitter_Clear verifies clearing stored events (T170).
func TestBufferedEmitter_Clear(t *testing.T) {
	t.Run("clears all events for runID", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		emitter.Emit(Event{RunID: "run-001", Msg: "event1"})
		emitter.Emit(Event{RunID: "run-002", Msg: "event2"})

		emitter.Clear("run-001")

		history1 := emitter.GetHistory("run-001")
		history2 := emitter.GetHistory("run-002")

		if len(history1) != 0 {
			t.Errorf("expected 0 events for run-001, got %d", len(history1))
		}
		if len(history2) != 1 {
			t.Errorf("expected 1 event for run-002, got %d", len(history2))
		}
	})

	t.Run("clears all events when runID is empty", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		emitter.Emit(Event{RunID: "run-001", Msg: "event1"})
		emitter.Emit(Event{RunID: "run-002", Msg: "event2"})

		emitter.Clear("")

		history1 := emitter.GetHistory("run-001")
		history2 := emitter.GetHistory("run-002")

		if len(history1) != 0 || len(history2) != 0 {
			t.Error("expected all events to be cleared")
		}
	})
}

// TestBufferedEmitter_ThreadSafety verifies concurrent access safety (T170).
func TestBufferedEmitter_ThreadSafety(t *testing.T) {
	t.Run("concurrent emit and read", func(t *testing.T) {
		emitter := NewBufferedEmitter()

		// Start 10 goroutines emitting events.
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(_ int) {
				for j := 0; j < 100; j++ {
					emitter.Emit(Event{
						RunID: "run-001",
						Step:  j,
						Msg:   "concurrent_event",
					})
				}
				done <- true
			}(i)
		}

		// Read history concurrently.
		readDone := make(chan bool)
		go func() {
			for i := 0; i < 100; i++ {
				emitter.GetHistory("run-001")
				time.Sleep(1 * time.Millisecond)
			}
			readDone <- true
		}()

		// Wait for all goroutines.
		for i := 0; i < 10; i++ {
			<-done
		}
		<-readDone

		history := emitter.GetHistory("run-001")
		if len(history) != 1000 {
			t.Errorf("expected 1000 events, got %d", len(history))
		}
	})
}

// TestBufferedEmitter_InterfaceContract verifies BufferedEmitter implements Emitter (T170).
func TestBufferedEmitter_InterfaceContract(_ *testing.T) {
	var _ Emitter = NewBufferedEmitter()
}
