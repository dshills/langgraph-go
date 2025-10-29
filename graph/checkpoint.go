// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// Checkpoint handles durable execution snapshots.

// ErrReplayMismatch is returned when recorded I/O hash does not match current execution during replay.
// This indicates non-deterministic behavior in a node (e.g., random values, system time, or external state).
// Replay mode expects nodes to produce identical outputs given identical inputs.
var ErrReplayMismatch = errors.New("replay mismatch: recorded I/O hash mismatch")

// ErrNoProgress is returned when the scheduler detects a deadlock condition.
// This occurs when the frontier queue is empty but no nodes are actively running,
// meaning the workflow cannot make forward progress. Common causes:
// - All nodes waiting on conditions that will never be satisfied.
// - Circular dependencies without conditional break.
// - Missing edges or routing logic.
var ErrNoProgress = errors.New("no progress: no runnable nodes in frontier")

// ErrBackpressureTimeout is returned when the frontier queue remains full beyond the configured timeout.
// This indicates nodes are being enqueued faster than they can be executed.
// The engine checkpoints state and pauses execution to prevent memory exhaustion.
// Resume execution after reducing load or increasing MaxConcurrentNodes.
var ErrBackpressureTimeout = errors.New("backpressure timeout: frontier queue full")

// ErrIdempotencyViolation is returned when attempting to commit a checkpoint with a duplicate idempotency key.
// This prevents duplicate execution of non-idempotent operations (e.g., charging payment twice).
// Idempotency keys are computed from runID, stepID, frontier state, and accumulated state.
// If this error occurs, the checkpoint was already successfully committed in a previous execution.
var ErrIdempotencyViolation = errors.New("idempotency violation: checkpoint already committed")

// ErrMaxAttemptsExceeded is returned when a node fails more times than allowed by its retry policy.
// The node's MaxAttempts limit has been reached, and the error is considered non-recoverable.
// Check the node's error logs to diagnose the root cause of repeated failures.
var ErrMaxAttemptsExceeded = errors.New("max retry attempts exceeded")

// Checkpoint represents a durable snapshot of execution state, enabling.
// resumption and deterministic replay of graph executions.
//
// Checkpoints are created atomically after each execution step and contain.
// all the information needed to resume from that point:
// - Current accumulated state.
// - Work items ready to execute (frontier).
// - Recorded I/O for replay.
// - RNG seed for deterministic random number generation.
// - Idempotency key for preventing duplicate commits.
//
// Checkpoints support both automatic resumption after failures and.
// user-initiated labeled snapshots for debugging or branching workflows.
type Checkpoint[S any] struct {
	// RunID uniquely identifies the execution this checkpoint belongs to.
	RunID string `json:"run_id"`

	// StepID is the execution step number at checkpoint time.
	// Monotonically increasing within a run.
	StepID int `json:"step_id"`

	// State is the current accumulated state after applying all deltas up to StepID.
	// Must be JSON-serializable for persistence.
	State S `json:"state"`

	// Frontier contains the work items ready to execute at this checkpoint.
	// These represent the nodes queued for execution when resuming from this checkpoint.
	Frontier []WorkItem[S] `json:"frontier"`

	// RNGSeed is the seed for deterministic random number generation.
	// Computed from RunID to ensure consistent random values across replays.
	RNGSeed int64 `json:"rng_seed"`

	// RecordedIOs contains all captured external interactions up to this checkpoint.
	// Indexed by (NodeID, Attempt) for lookup during replay.
	RecordedIOs []RecordedIO `json:"recorded_ios"`

	// IdempotencyKey is a hash of (RunID, StepID, State, Frontier) that prevents.
	// duplicate checkpoint commits. Format: "sha256:hex_encoded_hash".
	IdempotencyKey string `json:"idempotency_key"`

	// Timestamp records when this checkpoint was created.
	Timestamp time.Time `json:"timestamp"`

	// Label is an optional user-defined name for this checkpoint, useful for.
	// debugging or creating named save points (e.g., "before_summary", "after_validation").
	// Empty string for automatic checkpoints.
	Label string `json:"label,omitempty"`
}

// computeIdempotencyKey generates a deterministic hash for preventing duplicate checkpoint commits.
//
// The key is computed from:
// 1. Run ID - uniquely identifies the execution.
// 2. Step ID - identifies the scheduler tick.
// 3. Sorted work items - captures the frontier inputs (sorted by OrderKey for determinism).
// 4. State hash - captures the accumulated state after delta merging.
//
// This ensures that identical execution contexts produce identical idempotency keys,
// enabling exactly-once checkpoint commits even during retries or crash recovery.
//
// The hash uses SHA-256 for collision resistance and is returned as a hex-encoded string.
// with "sha256:" prefix for format versioning.
//
// Algorithm:
// 1. Create SHA-256 hasher.
// 2. Write run ID (string bytes).
// 3. Write step ID (8-byte big-endian int64).
//  4. For each work item (sorted by OrderKey):
//
// - Write node ID (string bytes).
// - Write order key (8-byte big-endian uint64).
// 5. Marshal state to JSON and write bytes.
// 6. Return "sha256:" + hex-encoded hash.
//
// Returns error if state JSON marshaling fails.
func computeIdempotencyKey[S any](runID string, stepID int, items []WorkItem[S], state S) (string, error) {
	// Create SHA-256 hasher.
	h := sha256.New()

	// Write run ID.
	h.Write([]byte(runID))

	// Write step ID as 8-byte big-endian int64.
	stepBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(stepBytes, uint64(stepID))
	h.Write(stepBytes)

	// Sort work items by OrderKey for deterministic ordering.
	sortedItems := make([]WorkItem[S], len(items))
	copy(sortedItems, items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].OrderKey < sortedItems[j].OrderKey
	})

	// Write each work item's identifying information.
	for _, item := range sortedItems {
		// Write node ID.
		h.Write([]byte(item.NodeID))

		// Write order key as 8-byte big-endian uint64.
		orderKeyBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(orderKeyBytes, item.OrderKey)
		h.Write(orderKeyBytes)
	}

	// Marshal state to JSON and write.
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	h.Write(stateJSON)

	// Return hex-encoded hash with format prefix.
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
