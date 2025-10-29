// Package emit provides event emission and observability for graph execution.
package emit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestLogEmitter_StructuredOutput verifies LogEmitter outputs structured events to writer (T160).
func TestLogEmitter_StructuredOutput(t *testing.T) {
	t.Run("emits event with all fields", func(t *testing.T) {
		var buf bytes.Buffer
		emitter := NewLogEmitter(&buf, false)

		event := Event{
			RunID:  "test-run-001",
			Step:   1,
			NodeID: "testNode",
			Msg:    "node_start",
			Meta: map[string]interface{}{
				"key": "value",
			},
		}

		emitter.Emit(event)

		output := buf.String()
		if output == "" {
			t.Fatal("expected output, got empty string")
		}

		// Verify all fields are present in output.
		if !strings.Contains(output, "test-run-001") {
			t.Errorf("expected output to contain RunID 'test-run-001', got: %s", output)
		}
		if !strings.Contains(output, "testNode") {
			t.Errorf("expected output to contain NodeID 'testNode', got: %s", output)
		}
		if !strings.Contains(output, "node_start") {
			t.Errorf("expected output to contain Msg 'node_start', got: %s", output)
		}

		t.Logf("LogEmitter output: %s", output)
	})

	t.Run("emits multiple events", func(t *testing.T) {
		var buf bytes.Buffer
		emitter := NewLogEmitter(&buf, false)

		event1 := Event{
			RunID:  "run-001",
			Step:   0,
			NodeID: "node1",
			Msg:    "node_start",
		}
		event2 := Event{
			RunID:  "run-001",
			Step:   0,
			NodeID: "node1",
			Msg:    "node_end",
		}

		emitter.Emit(event1)
		emitter.Emit(event2)

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) < 2 {
			t.Errorf("expected at least 2 lines of output, got %d", len(lines))
		}

		t.Logf("LogEmitter multi-event output: %s", output)
	})
}

// TestLogEmitter_JSONFormatting verifies LogEmitter can output JSON format (T162).
func TestLogEmitter_JSONFormatting(t *testing.T) {
	t.Run("emits valid JSON when JSON mode enabled", func(t *testing.T) {
		var buf bytes.Buffer
		emitter := NewLogEmitter(&buf, true) // JSON mode

		event := Event{
			RunID:  "json-run-001",
			Step:   2,
			NodeID: "jsonNode",
			Msg:    "node_end",
			Meta: map[string]interface{}{
				"counter": 42,
				"status":  "success",
			},
		}

		emitter.Emit(event)

		output := buf.String()
		if output == "" {
			t.Fatal("expected JSON output, got empty string")
		}

		// Verify it's valid JSON by parsing.
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(output), &parsed); err != nil {
			t.Fatalf("expected valid JSON, got error: %v\nOutput: %s", err, output)
		}

		// Verify all fields are present.
		if parsed["runID"] != "json-run-001" {
			t.Errorf("expected runID 'json-run-001', got %v", parsed["runID"])
		}
		if parsed["step"] != float64(2) {
			t.Errorf("expected step 2, got %v", parsed["step"])
		}
		if parsed["nodeID"] != "jsonNode" {
			t.Errorf("expected nodeID 'jsonNode', got %v", parsed["nodeID"])
		}
		if parsed["msg"] != "node_end" {
			t.Errorf("expected msg 'node_end', got %v", parsed["msg"])
		}

		// Verify meta is present.
		meta, ok := parsed["meta"].(map[string]interface{})
		if !ok {
			t.Fatal("expected meta to be a map")
		}
		if meta["counter"] != float64(42) {
			t.Errorf("expected counter 42, got %v", meta["counter"])
		}

		t.Logf("LogEmitter JSON output: %s", output)
	})

	t.Run("emits multiple JSON events on separate lines", func(t *testing.T) {
		var buf bytes.Buffer
		emitter := NewLogEmitter(&buf, true)

		event1 := Event{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_start"}
		event2 := Event{RunID: "run-001", Step: 0, NodeID: "node1", Msg: "node_end"}

		emitter.Emit(event1)
		emitter.Emit(event2)

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 2 {
			t.Errorf("expected 2 lines of JSON, got %d", len(lines))
		}

		// Verify each line is valid JSON.
		for i, line := range lines {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(line), &parsed); err != nil {
				t.Errorf("line %d: expected valid JSON, got error: %v\nLine: %s", i, err, line)
			}
		}

		t.Logf("LogEmitter multi-event JSON output:\n%s", output)
	})
}

// TestLogEmitter_InterfaceContract verifies LogEmitter implements Emitter interface.
func TestLogEmitter_InterfaceContract(t *testing.T) {
	var buf bytes.Buffer
	var _ Emitter = NewLogEmitter(&buf, false)
}
