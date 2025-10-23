package emit

import (
	"testing"
	"time"
)

// TestEvent_Struct verifies Event struct fields (T029).
func TestEvent_Struct(t *testing.T) {
	t.Run("complete event with all fields", func(t *testing.T) {
		meta := map[string]interface{}{
			"duration_ms": 125,
			"retry":       false,
		}

		event := Event{
			RunID:  "run-001",
			Step:   3,
			NodeID: "process-node",
			Msg:    "Processing completed successfully",
			Meta:   meta,
		}

		if event.RunID != "run-001" {
			t.Errorf("expected RunID = 'run-001', got %q", event.RunID)
		}
		if event.Step != 3 {
			t.Errorf("expected Step = 3, got %d", event.Step)
		}
		if event.NodeID != "process-node" {
			t.Errorf("expected NodeID = 'process-node', got %q", event.NodeID)
		}
		if event.Msg != "Processing completed successfully" {
			t.Errorf("expected Msg = 'Processing completed successfully', got %q", event.Msg)
		}
		if event.Meta["duration_ms"] != 125 {
			t.Errorf("expected Meta['duration_ms'] = 125, got %v", event.Meta["duration_ms"])
		}
	})

	t.Run("minimal event", func(t *testing.T) {
		event := Event{
			RunID: "run-002",
			Msg:   "Started",
		}

		if event.Step != 0 {
			t.Errorf("expected Step = 0 (zero value), got %d", event.Step)
		}
		if event.NodeID != "" {
			t.Errorf("expected NodeID = \"\" (zero value), got %q", event.NodeID)
		}
		if event.Meta != nil {
			t.Error("expected Meta = nil (zero value)")
		}
	})

	t.Run("event with metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-003",
			Step:   1,
			NodeID: "start",
			Msg:    "Execution started",
			Meta: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"user_id":   "user-123",
				"tags":      []string{"production", "high-priority"},
			},
		}

		if event.Meta["user_id"] != "user-123" {
			t.Errorf("expected user_id = 'user-123', got %v", event.Meta["user_id"])
		}

		tags, ok := event.Meta["tags"].([]string)
		if !ok {
			t.Fatal("expected tags to be []string")
		}
		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(tags))
		}
	})

	t.Run("zero value event", func(t *testing.T) {
		var event Event

		if event.RunID != "" {
			t.Errorf("expected zero value RunID, got %q", event.RunID)
		}
		if event.Step != 0 {
			t.Errorf("expected zero value Step, got %d", event.Step)
		}
		if event.NodeID != "" {
			t.Errorf("expected zero value NodeID, got %q", event.NodeID)
		}
		if event.Msg != "" {
			t.Errorf("expected zero value Msg, got %q", event.Msg)
		}
		if event.Meta != nil {
			t.Error("expected zero value Meta to be nil")
		}
	})
}

// TestEvent_UseCases verifies common event patterns.
func TestEvent_UseCases(t *testing.T) {
	t.Run("node start event", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "llm-call",
			Msg:    "Starting LLM call",
		}

		if event.NodeID != "llm-call" {
			t.Errorf("expected NodeID = 'llm-call', got %q", event.NodeID)
		}
	})

	t.Run("node complete event", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "llm-call",
			Msg:    "LLM call completed",
			Meta: map[string]interface{}{
				"tokens": 150,
				"cost":   0.003,
			},
		}

		if event.Meta["tokens"] != 150 {
			t.Errorf("expected tokens = 150, got %v", event.Meta["tokens"])
		}
	})

	t.Run("error event", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   2,
			NodeID: "validator",
			Msg:    "Validation failed: invalid input",
			Meta: map[string]interface{}{
				"error_code": "INVALID_INPUT",
				"retryable":  true,
			},
		}

		if event.Meta["retryable"] != true {
			t.Error("expected retryable = true")
		}
	})

	t.Run("checkpoint event", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Step:  5,
			Msg:   "Checkpoint saved",
			Meta: map[string]interface{}{
				"checkpoint_id": "cp-after-validation",
				"state_size":    1024,
			},
		}

		cpID, ok := event.Meta["checkpoint_id"].(string)
		if !ok || cpID != "cp-after-validation" {
			t.Errorf("expected checkpoint_id = 'cp-after-validation', got %v", cpID)
		}
	})
}
