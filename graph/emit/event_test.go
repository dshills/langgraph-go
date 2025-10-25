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

// TestEvent_WithDuration verifies WithDuration metadata helper (T166).
func TestEvent_WithDuration(t *testing.T) {
	t.Run("sets duration_ms in metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "node1",
			Msg:    "test",
		}

		duration := 125 * time.Millisecond
		result := event.WithDuration(duration)

		if result.Meta == nil {
			t.Fatal("expected Meta to be initialized")
		}
		if result.Meta["duration_ms"] != int64(125) {
			t.Errorf("expected duration_ms = 125, got %v", result.Meta["duration_ms"])
		}
	})

	t.Run("preserves existing metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "node1",
			Msg:    "test",
			Meta: map[string]interface{}{
				"existing_key": "existing_value",
			},
		}

		duration := 250 * time.Millisecond
		result := event.WithDuration(duration)

		if result.Meta["existing_key"] != "existing_value" {
			t.Errorf("expected existing_key to be preserved, got %v", result.Meta["existing_key"])
		}
		if result.Meta["duration_ms"] != int64(250) {
			t.Errorf("expected duration_ms = 250, got %v", result.Meta["duration_ms"])
		}
	})

	t.Run("handles zero duration", func(t *testing.T) {
		event := Event{RunID: "run-001", Msg: "test"}
		result := event.WithDuration(0)

		if result.Meta["duration_ms"] != int64(0) {
			t.Errorf("expected duration_ms = 0, got %v", result.Meta["duration_ms"])
		}
	})
}

// TestEvent_WithError verifies WithError metadata helper (T166).
func TestEvent_WithError(t *testing.T) {
	t.Run("sets error in metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "node1",
			Msg:    "error",
		}

		err := &time.ParseError{Value: "invalid"}
		result := event.WithError(err)

		if result.Meta == nil {
			t.Fatal("expected Meta to be initialized")
		}
		errStr, ok := result.Meta["error"].(string)
		if !ok {
			t.Fatal("expected error to be string")
		}
		if errStr == "" {
			t.Error("expected error message to be non-empty")
		}
	})

	t.Run("preserves existing metadata", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Msg:   "error",
			Meta: map[string]interface{}{
				"request_id": "req-123",
			},
		}

		err := &time.ParseError{Value: "test"}
		result := event.WithError(err)

		if result.Meta["request_id"] != "req-123" {
			t.Error("expected request_id to be preserved")
		}
		if result.Meta["error"] == nil {
			t.Error("expected error to be set")
		}
	})
}

// TestEvent_WithNodeType verifies WithNodeType metadata helper (T167).
func TestEvent_WithNodeType(t *testing.T) {
	t.Run("sets node_type in metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "llm-node",
			Msg:    "node_start",
		}

		result := event.WithNodeType("llm")

		if result.Meta == nil {
			t.Fatal("expected Meta to be initialized")
		}
		if result.Meta["node_type"] != "llm" {
			t.Errorf("expected node_type = 'llm', got %v", result.Meta["node_type"])
		}
	})

	t.Run("preserves existing metadata", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Msg:   "test",
			Meta: map[string]interface{}{
				"version": "1.0",
			},
		}

		result := event.WithNodeType("tool")

		if result.Meta["version"] != "1.0" {
			t.Error("expected version to be preserved")
		}
		if result.Meta["node_type"] != "tool" {
			t.Errorf("expected node_type = 'tool', got %v", result.Meta["node_type"])
		}
	})

	t.Run("handles empty node type", func(t *testing.T) {
		event := Event{RunID: "run-001", Msg: "test"}
		result := event.WithNodeType("")

		if result.Meta["node_type"] != "" {
			t.Errorf("expected empty node_type, got %v", result.Meta["node_type"])
		}
	})
}

// TestEvent_WithMeta verifies WithMeta chaining helper (T167).
func TestEvent_WithMeta(t *testing.T) {
	t.Run("sets single metadata field", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Msg:   "test",
		}

		result := event.WithMeta("tokens", 150)

		if result.Meta == nil {
			t.Fatal("expected Meta to be initialized")
		}
		if result.Meta["tokens"] != 150 {
			t.Errorf("expected tokens = 150, got %v", result.Meta["tokens"])
		}
	})

	t.Run("preserves existing metadata", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Msg:   "test",
			Meta: map[string]interface{}{
				"key1": "value1",
			},
		}

		result := event.WithMeta("key2", "value2")

		if result.Meta["key1"] != "value1" {
			t.Error("expected key1 to be preserved")
		}
		if result.Meta["key2"] != "value2" {
			t.Errorf("expected key2 = 'value2', got %v", result.Meta["key2"])
		}
	})

	t.Run("allows chaining", func(t *testing.T) {
		event := Event{RunID: "run-001", Msg: "test"}

		result := event.
			WithMeta("key1", "value1").
			WithMeta("key2", 42).
			WithMeta("key3", true)

		if result.Meta["key1"] != "value1" {
			t.Error("expected key1 to be set")
		}
		if result.Meta["key2"] != 42 {
			t.Error("expected key2 to be set")
		}
		if result.Meta["key3"] != true {
			t.Error("expected key3 to be set")
		}
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		event := Event{
			RunID: "run-001",
			Msg:   "test",
			Meta: map[string]interface{}{
				"status": "pending",
			},
		}

		result := event.WithMeta("status", "complete")

		if result.Meta["status"] != "complete" {
			t.Errorf("expected status to be overwritten to 'complete', got %v", result.Meta["status"])
		}
	})
}

// TestEvent_MetaHelpers_Integration verifies chaining multiple helpers (T167).
func TestEvent_MetaHelpers_Integration(t *testing.T) {
	t.Run("chain duration and node type", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   1,
			NodeID: "llm-node",
			Msg:    "node_end",
		}

		result := event.
			WithDuration(150 * time.Millisecond).
			WithNodeType("llm")

		if result.Meta["duration_ms"] != int64(150) {
			t.Error("expected duration_ms to be set")
		}
		if result.Meta["node_type"] != "llm" {
			t.Error("expected node_type to be set")
		}
	})

	t.Run("chain error with custom metadata", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   2,
			NodeID: "validator",
			Msg:    "error",
		}

		err := &time.ParseError{Value: "bad input"}
		result := event.
			WithError(err).
			WithMeta("retryable", true).
			WithMeta("attempt", 2)

		if result.Meta["error"] == nil {
			t.Error("expected error to be set")
		}
		if result.Meta["retryable"] != true {
			t.Error("expected retryable to be set")
		}
		if result.Meta["attempt"] != 2 {
			t.Error("expected attempt to be set")
		}
	})

	t.Run("complex chain with all helpers", func(t *testing.T) {
		event := Event{
			RunID:  "run-001",
			Step:   5,
			NodeID: "processor",
			Msg:    "node_end",
		}

		result := event.
			WithDuration(500*time.Millisecond).
			WithNodeType("processor").
			WithMeta("records_processed", 1000).
			WithMeta("cache_hits", 850).
			WithMeta("cost", 0.05)

		// Verify all metadata is present
		if result.Meta["duration_ms"] != int64(500) {
			t.Error("expected duration_ms")
		}
		if result.Meta["node_type"] != "processor" {
			t.Error("expected node_type")
		}
		if result.Meta["records_processed"] != 1000 {
			t.Error("expected records_processed")
		}
		if result.Meta["cache_hits"] != 850 {
			t.Error("expected cache_hits")
		}
		if result.Meta["cost"] != 0.05 {
			t.Error("expected cost")
		}
	})
}
