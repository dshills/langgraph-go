// Package store provides persistence implementations for graph state.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
)

// ErrNotFound is returned when a requested run ID or checkpoint ID does not exist.
var ErrNotFound = errors.New("not found")

// Store provides persistence for workflow state and checkpoints.
//
// It enables:
// - Step-by-step state persistence during execution.
// - Latest state retrieval for resumption.
// - Named checkpoint save/load for branching workflows.
//
// Implementations can use:
// - In-memory storage (for testing, see memory.go).
// - Relational databases (MySQL, PostgreSQL).
// - Key-value stores (Redis, DynamoDB).
// - Object storage (S3, GCS).
//
// Type parameter S is the state type to persist.
type Store[S any] interface {
	// SaveStep persists the state after a node execution step.
	// Each step is identified by runID + step number.
	//
	// Parameters:
	// - runID: Unique identifier for this workflow execution.
	// - step: Sequential step number (starts at 1).
	// - nodeID: ID of the node that produced this state.
	// - state: The current workflow state after merging delta.
	//
	// Returns error if persistence fails.
	SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error

	// LoadLatest retrieves the most recent state for a given run.
	// Used to resume execution from the last saved step.
	//
	// Parameters:
	// - runID: Unique identifier for the workflow execution.
	//
	// Returns:
	// - state: The most recent persisted state.
	// - step: The step number of the returned state.
	// - error: ErrNotFound if runID doesn't exist, or other persistence errors.
	LoadLatest(ctx context.Context, runID string) (state S, step int, err error)

	// SaveCheckpoint creates a named snapshot of workflow state.
	// Checkpoints enable branching workflows and manual resumption points.
	//
	// Parameters:
	// - cpID: Unique checkpoint identifier (user-defined).
	// - state: The workflow state to snapshot.
	// - step: The step number at which this checkpoint was created.
	//
	// Returns error if persistence fails.
	SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error

	// LoadCheckpoint retrieves a previously saved checkpoint.
	// Used to restore workflow state from a named checkpoint.
	//
	// Parameters:
	// - cpID: Unique checkpoint identifier.
	//
	// Returns:
	// - state: The checkpointed state.
	// - step: The step number when checkpoint was created.
	// - error: ErrNotFound if cpID doesn't exist, or other persistence errors.
	LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error)

	// SaveCheckpointV2 persists an enhanced checkpoint with full execution context.
	// This includes frontier state, recorded I/O, RNG seed, and idempotency key.
	//
	// The checkpoint contains all information needed for deterministic replay:
	// - Current state at the checkpoint.
	// - Pending work items in the execution frontier.
	// - Recorded I/O responses for replay.
	// - RNG seed for deterministic random values.
	// - Idempotency key to prevent duplicate commits.
	//
	// Parameters:
	// - checkpoint: Complete checkpoint with all execution context (CheckpointV2 type).
	//
	// Returns error if persistence fails or idempotency key already exists.
	//
	// This method extends SaveCheckpoint with support for concurrent execution.
	// and deterministic replay. Use this for v0.2.0+ features, or use the original.
	// SaveCheckpoint for simpler checkpointing needs.
	SaveCheckpointV2(ctx context.Context, checkpoint CheckpointV2[S]) error

	// LoadCheckpointV2 retrieves an enhanced checkpoint by run ID and step ID.
	//
	// Unlike LoadCheckpoint which uses a user-defined label, this method loads.
	// checkpoints by their system-generated identifiers. This enables:
	// - Resumption from any specific step in execution history.
	// - Replay of partial execution segments.
	// - Time-travel debugging through execution steps.
	//
	// Parameters:
	// - runID: Unique workflow run identifier.
	// - stepID: Step number to load checkpoint from.
	//
	// Returns:
	// - checkpoint: Complete checkpoint with execution context (CheckpointV2 type).
	// - error: ErrNotFound if checkpoint doesn't exist.
	LoadCheckpointV2(ctx context.Context, runID string, stepID int) (CheckpointV2[S], error)

	// CheckIdempotency verifies if an idempotency key has been used.
	//
	// Idempotency keys prevent duplicate step commits during retries or crash recovery.
	// The key is typically a hash of: runID + stepID + frontier state + node outputs.
	//
	// Parameters:
	// - key: Idempotency key to check (SHA-256 hash string).
	//
	// Returns:
	// - exists: true if key was previously used.
	// - error: Only on store access failure (not for "key not found").
	//
	// Implementation note: Store the key atomically when committing checkpoints.
	// Keys should be indexed for fast lookup. Consider TTL-based cleanup for old keys.
	CheckIdempotency(ctx context.Context, key string) (bool, error)

	// PendingEvents retrieves events from the transactional outbox that haven't been emitted.
	//
	// This implements the "transactional outbox pattern" for exactly-once event delivery:
	// 1. Events are persisted atomically with state changes.
	// 2. Separate process reads pending events and emits them.
	// 3. Successfully emitted events are marked via MarkEventsEmitted.
	// 4. Crashed emitters can resume from pending events.
	//
	// Parameters:
	// - limit: Maximum number of events to retrieve (for batching).
	//
	// Returns:
	// - events: Events pending emission, ordered by creation time.
	// - error: Only on store access failure (empty list is not an error).
	//
	// Use this with MarkEventsEmitted to implement reliable event delivery without.
	// message broker dependencies.
	PendingEvents(ctx context.Context, limit int) ([]emit.Event, error)

	// MarkEventsEmitted marks events as successfully emitted to prevent re-delivery.
	//
	// After successfully emitting events to external systems (logs, traces, metrics),
	// call this method to record their emission. This ensures:
	// - Events are emitted exactly once (not lost, not duplicated).
	// - Crash recovery doesn't re-emit already-delivered events.
	// - PendingEvents won't return these events again.
	//
	// Parameters:
	// - eventIDs: List of event IDs that were successfully emitted.
	//
	// Returns error if store update fails. On error, events may be re-emitted.
	// (at-least-once semantics).
	//
	// Implementation note: Mark as emitted in the same transaction/atomic operation.
	// as the external emit when possible, or use idempotency keys on the receiving end.
	MarkEventsEmitted(ctx context.Context, eventIDs []string) error
}

// StepRecord represents a single execution step in the workflow history.
// Used internally by Store implementations to track step-by-step progression.
type StepRecord[S any] struct {
	// Step is the sequential step number (1-indexed).
	Step int

	// NodeID identifies which node produced this state.
	NodeID string

	// State is the workflow state after this step completed.
	State S
}

// Checkpoint represents a named snapshot of workflow state.
// Used by Store implementations to persist and restore checkpoints.
//
// Deprecated: Use CheckpointV2 for enhanced checkpointing features.
// This type is kept for backward compatibility with the original SaveCheckpoint/LoadCheckpoint methods.
type Checkpoint[S any] struct {
	// ID is the unique checkpoint identifier.
	ID string

	// State is the snapshotted workflow state.
	State S

	// Step is the step number when this checkpoint was created.
	Step int
}

// CheckpointV2 represents an enhanced checkpoint with full execution context for deterministic replay.
//
// This type contains all information needed to resume execution from a specific point:
// - Current accumulated state.
// - Work items ready to execute (frontier).
// - Recorded I/O for replay.
// - RNG seed for deterministic random number generation.
// - Idempotency key for preventing duplicate commits.
//
// CheckpointV2 supports both automatic resumption after failures and.
// user-initiated labeled snapshots for debugging or branching workflows.
//
// This type is generic over the state type S, which must be JSON-serializable.
type CheckpointV2[S any] struct {
	// RunID uniquely identifies the execution this checkpoint belongs to.
	RunID string `json:"run_id"`

	// StepID is the execution step number at checkpoint time.
	// Monotonically increasing within a run.
	StepID int `json:"step_id"`

	// State is the current accumulated state after applying all deltas up to StepID.
	// Must be JSON-serializable for persistence.
	State S `json:"state"`

	// Frontier contains the work items ready to execute at this checkpoint.
	// Must be JSON-serializable. Type is interface{} to avoid circular dependency.
	// Expected to be []WorkItem[S] from graph package.
	Frontier interface{} `json:"frontier"`

	// RNGSeed is the seed for deterministic random number generation.
	// Computed from RunID to ensure consistent random values across replays.
	RNGSeed int64 `json:"rng_seed"`

	// RecordedIOs contains all captured external interactions up to this checkpoint.
	// Must be JSON-serializable. Type is interface{} to avoid circular dependency.
	// Expected to be []RecordedIO from graph package.
	RecordedIOs interface{} `json:"recorded_ios"`

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
