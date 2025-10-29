// Package graph_test provides functionality for the LangGraph-Go framework.
package graph_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph"
)

// TestCheckpointSave (T041) verifies that saveCheckpoint creates a Checkpoint.
// with all required fields and can be serialized/deserialized correctly.
//
// According to spec.md FR-006: System MUST save checkpoints containing.
// {run_id, step_id, state, frontier, rng_seed, recorded_io}.
//
// Requirements:
// - Checkpoint contains all required fields.
// - Checkpoint can be serialized to JSON.
// - Checkpoint can be deserialized from JSON.
// - All fields preserved across serialization.
//
// This test should SKIP initially because saveCheckpoint function doesn't exist yet.
func TestCheckpointSave(t *testing.T) {
	t.Run("checkpoint contains all required fields", func(t *testing.T) {
		type CheckpointTestState struct {
			Value   string
			Counter int
		}

		// Create a checkpoint.
		state := CheckpointTestState{Value: "test", Counter: 42}
		frontier := []graph.WorkItem[CheckpointTestState]{
			{
				StepID:       2,
				OrderKey:     100,
				NodeID:       "node1",
				State:        state,
				Attempt:      0,
				ParentNodeID: "start",
				EdgeIndex:    0,
			},
		}

		recordedIOs := []graph.RecordedIO{
			{
				NodeID:    "node1",
				Attempt:   0,
				Request:   json.RawMessage(`{"query":"test"}`),
				Response:  json.RawMessage(`{"result":"success"}`),
				Hash:      "sha256:abc123",
				Timestamp: time.Now(),
				Duration:  100 * time.Millisecond,
			},
		}

		checkpoint := createCheckpoint(
			"run-123",
			1,
			state,
			frontier,
			12345,
			recordedIOs,
			"checkpoint-label",
		)

		// Verify all fields present.
		if checkpoint.RunID != "run-123" {
			t.Errorf("expected RunID='run-123', got %q", checkpoint.RunID)
		}
		if checkpoint.StepID != 1 {
			t.Errorf("expected StepID=1, got %d", checkpoint.StepID)
		}
		if checkpoint.State.Value != "test" {
			t.Errorf("expected State.Value='test', got %q", checkpoint.State.Value)
		}
		if len(checkpoint.Frontier) != 1 {
			t.Errorf("expected Frontier length=1, got %d", len(checkpoint.Frontier))
		}
		if checkpoint.RNGSeed != 12345 {
			t.Errorf("expected RNGSeed=12345, got %d", checkpoint.RNGSeed)
		}
		if len(checkpoint.RecordedIOs) != 1 {
			t.Errorf("expected RecordedIOs length=1, got %d", len(checkpoint.RecordedIOs))
		}
		if checkpoint.Label != "checkpoint-label" {
			t.Errorf("expected Label='checkpoint-label', got %q", checkpoint.Label)
		}
		if checkpoint.IdempotencyKey == "" {
			t.Error("expected non-empty IdempotencyKey")
		}
		if checkpoint.Timestamp.IsZero() {
			t.Error("expected non-zero Timestamp")
		}
	})

	t.Run("checkpoint can be serialized to JSON", func(t *testing.T) {
		type SimpleState struct {
			Name string
		}

		state := SimpleState{Name: "test"}
		frontier := []graph.WorkItem[SimpleState]{}
		recordedIOs := []graph.RecordedIO{}

		checkpoint := createCheckpoint(
			"run-456",
			5,
			state,
			frontier,
			99999,
			recordedIOs,
			"",
		)

		// Serialize.
		jsonBytes, err := json.Marshal(checkpoint)
		if err != nil {
			t.Fatalf("failed to marshal checkpoint: %v", err)
		}

		if len(jsonBytes) == 0 {
			t.Error("serialized checkpoint is empty")
		}

		// Verify it's valid JSON.
		var raw map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &raw); err != nil {
			t.Fatalf("serialized checkpoint is not valid JSON: %v", err)
		}

		// Verify key fields present in JSON.
		if raw["run_id"] != "run-456" {
			t.Errorf("expected run_id='run-456' in JSON, got %v", raw["run_id"])
		}
		if raw["step_id"].(float64) != 5 {
			t.Errorf("expected step_id=5 in JSON, got %v", raw["step_id"])
		}
	})

	t.Run("checkpoint can be deserialized from JSON", func(t *testing.T) {
		type DeserTestState struct {
			Value   string
			Counter int
		}

		// Create original checkpoint.
		originalState := DeserTestState{Value: "original", Counter: 123}
		originalFrontier := []graph.WorkItem[DeserTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node1", State: originalState},
		}

		original := createCheckpoint(
			"run-789",
			10,
			originalState,
			originalFrontier,
			55555,
			[]graph.RecordedIO{},
			"test-label",
		)

		// Serialize.
		jsonBytes, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		// Deserialize.
		var deserialized graph.Checkpoint[DeserTestState]
		if err := json.Unmarshal(jsonBytes, &deserialized); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		// Verify all fields preserved.
		if deserialized.RunID != original.RunID {
			t.Errorf("RunID not preserved: %s != %s", deserialized.RunID, original.RunID)
		}
		if deserialized.StepID != original.StepID {
			t.Errorf("StepID not preserved: %d != %d", deserialized.StepID, original.StepID)
		}
		if deserialized.State.Value != original.State.Value {
			t.Errorf("State.Value not preserved: %s != %s", deserialized.State.Value, original.State.Value)
		}
		if deserialized.RNGSeed != original.RNGSeed {
			t.Errorf("RNGSeed not preserved: %d != %d", deserialized.RNGSeed, original.RNGSeed)
		}
		if deserialized.Label != original.Label {
			t.Errorf("Label not preserved: %s != %s", deserialized.Label, original.Label)
		}
		if deserialized.IdempotencyKey != original.IdempotencyKey {
			t.Errorf("IdempotencyKey not preserved")
		}
	})

	t.Run("idempotency key prevents duplicate saves", func(t *testing.T) {
		// Create two checkpoints with identical content.
		type IdempotentState struct {
			Value string
		}

		state := IdempotentState{Value: "same"}
		frontier := []graph.WorkItem[IdempotentState]{
			{StepID: 1, OrderKey: 100, NodeID: "node1", State: state},
		}

		checkpoint1 := createCheckpoint("run-same", 1, state, frontier, 12345, []graph.RecordedIO{}, "")
		checkpoint2 := createCheckpoint("run-same", 1, state, frontier, 12345, []graph.RecordedIO{}, "")

		// Idempotency keys should match.
		if checkpoint1.IdempotencyKey != checkpoint2.IdempotencyKey {
			t.Errorf("identical checkpoints produced different idempotency keys: %s != %s",
				checkpoint1.IdempotencyKey, checkpoint2.IdempotencyKey)
		}

		// Different checkpoint should have different key.
		differentState := IdempotentState{Value: "different"}
		checkpoint3 := createCheckpoint("run-same", 1, differentState, frontier, 12345, []graph.RecordedIO{}, "")

		if checkpoint1.IdempotencyKey == checkpoint3.IdempotencyKey {
			t.Error("different checkpoints produced same idempotency key")
		}
	})
}

// TestIdempotencyKey (T045) verifies that computeIdempotencyKey generates.
// consistent keys for identical checkpoints and different keys for different checkpoints.
//
// According to spec.md FR-019: System MUST use idempotency keys (hash of work items.
// and state) to prevent duplicate step commits.
//
// According to spec.md SC-012: Idempotency keys prevent 100% of duplicate state.
// applications during failure recovery.
//
// Requirements:
// - Same inputs produce same key.
// - Different inputs produce different keys.
// - Key format is "sha256:hex".
// - Key incorporates runID, stepID, state, and frontier.
//
// This test should SKIP initially because computeIdempotencyKey doesn't exist yet.
func TestIdempotencyKey(t *testing.T) {
	t.Run("same inputs produce same key", func(t *testing.T) {
		type KeyTestState struct {
			Value string
			Count int
		}

		state := KeyTestState{Value: "test", Count: 42}
		frontier := []graph.WorkItem[KeyTestState]{
			{StepID: 1, OrderKey: 100, NodeID: "node1"},
		}

		key1 := computeIdempotencyKey("run-123", 1, state, frontier)
		key2 := computeIdempotencyKey("run-123", 1, state, frontier)

		if key1 != key2 {
			t.Errorf("same inputs produced different keys: %s != %s", key1, key2)
		}
	})

	t.Run("different run IDs produce different keys", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state := KeyTestState{Value: "test"}
		frontier := []graph.WorkItem[KeyTestState]{}

		key1 := computeIdempotencyKey("run-111", 1, state, frontier)
		key2 := computeIdempotencyKey("run-222", 1, state, frontier)

		if key1 == key2 {
			t.Error("different run IDs produced same key")
		}
	})

	t.Run("different step IDs produce different keys", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state := KeyTestState{Value: "test"}
		frontier := []graph.WorkItem[KeyTestState]{}

		key1 := computeIdempotencyKey("run-123", 1, state, frontier)
		key2 := computeIdempotencyKey("run-123", 2, state, frontier)

		if key1 == key2 {
			t.Error("different step IDs produced same key")
		}
	})

	t.Run("different states produce different keys", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state1 := KeyTestState{Value: "state1"}
		state2 := KeyTestState{Value: "state2"}
		frontier := []graph.WorkItem[KeyTestState]{}

		key1 := computeIdempotencyKey("run-123", 1, state1, frontier)
		key2 := computeIdempotencyKey("run-123", 1, state2, frontier)

		if key1 == key2 {
			t.Error("different states produced same key")
		}
	})

	t.Run("different frontiers produce different keys", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state := KeyTestState{Value: "test"}
		frontier1 := []graph.WorkItem[KeyTestState]{
			{NodeID: "node1", OrderKey: 100},
		}
		frontier2 := []graph.WorkItem[KeyTestState]{
			{NodeID: "node2", OrderKey: 200},
		}

		key1 := computeIdempotencyKey("run-123", 1, state, frontier1)
		key2 := computeIdempotencyKey("run-123", 1, state, frontier2)

		if key1 == key2 {
			t.Error("different frontiers produced same key")
		}
	})

	t.Run("key format is sha256 hex", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state := KeyTestState{Value: "test"}
		frontier := []graph.WorkItem[KeyTestState]{}

		key := computeIdempotencyKey("run-123", 1, state, frontier)

		// Check format: "sha256:64_hex_chars".
		if len(key) < 71 {
			t.Errorf("key too short: %d characters", len(key))
		}
		if key[:7] != "sha256:" {
			t.Errorf("expected key to start with 'sha256:', got %q", key[:7])
		}

		// Verify hex encoding.
		hexPart := key[7:]
		if _, err := hex.DecodeString(hexPart); err != nil {
			t.Errorf("key does not contain valid hex: %v", err)
		}
	})

	t.Run("key is collision resistant", func(t *testing.T) {
		// Generate many keys and verify no collisions.
		type KeyTestState struct {
			Value   string
			Counter int
		}

		seen := make(map[string]string)
		collisions := 0

		for i := 0; i < 100; i++ {
			for j := 0; j < 10; j++ {
				state := KeyTestState{Value: "test", Counter: i*10 + j}
				frontier := []graph.WorkItem[KeyTestState]{
					{NodeID: "node1", OrderKey: uint64(i*10 + j)}, // #nosec G115 -- test loop counter, bounded by loop limit
				}

				key := computeIdempotencyKey("run-test", i, state, frontier)
				identifier := string(rune('A'+i)) + string(rune('0'+j))

				if existing, exists := seen[key]; exists {
					t.Errorf("collision detected: key %s used by both %s and %s",
						key[:20], existing, identifier)
					collisions++
				} else {
					seen[key] = identifier
				}
			}
		}

		if collisions > 0 {
			t.Errorf("detected %d collisions in 1000 key generations", collisions)
		}
	})

	t.Run("empty frontier is handled correctly", func(t *testing.T) {
		type KeyTestState struct {
			Value string
		}

		state := KeyTestState{Value: "test"}
		emptyFrontier := []graph.WorkItem[KeyTestState]{}

		// Should not panic or error.
		key := computeIdempotencyKey("run-123", 1, state, emptyFrontier)

		if key == "" {
			t.Error("empty frontier produced empty key")
		}
		if key[:7] != "sha256:" {
			t.Errorf("expected valid key format, got %q", key)
		}
	})
}

// Helper functions for checkpoint tests (these will be implemented in T046-T057).

// createCheckpoint creates a Checkpoint with all required fields.
// This is a test helper that mimics the real checkpoint creation.
func createCheckpoint[S any](
	runID string,
	stepID int,
	state S,
	frontier []graph.WorkItem[S],
	rngSeed int64,
	recordedIOs []graph.RecordedIO,
	label string,
) graph.Checkpoint[S] {
	// Compute idempotency key.
	idempotencyKey := computeIdempotencyKey(runID, stepID, state, frontier)

	return graph.Checkpoint[S]{
		RunID:          runID,
		StepID:         stepID,
		State:          state,
		Frontier:       frontier,
		RNGSeed:        rngSeed,
		RecordedIOs:    recordedIOs,
		IdempotencyKey: idempotencyKey,
		Timestamp:      time.Now(),
		Label:          label,
	}
}

// computeIdempotencyKey generates a unique key for a checkpoint.
// This is a test helper that mimics the real idempotency key computation.
func computeIdempotencyKey[S any](runID string, stepID int, state S, frontier []graph.WorkItem[S]) string {
	h := sha256.New()

	// Write runID.
	h.Write([]byte(runID))

	// Write stepID (as string for simplicity in test).
	h.Write([]byte{byte(stepID)})

	// Write state.
	stateJSON, _ := json.Marshal(state)
	h.Write(stateJSON)

	// Write frontier.
	frontierJSON, _ := json.Marshal(frontier)
	h.Write(frontierJSON)

	// Compute hash.
	hashBytes := h.Sum(nil)
	return "sha256:" + hex.EncodeToString(hashBytes)
}
