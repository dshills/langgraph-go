package store

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a requested run ID or checkpoint ID does not exist.
var ErrNotFound = errors.New("not found")

// Store provides persistence for workflow state and checkpoints.
//
// It enables:
//   - Step-by-step state persistence during execution
//   - Latest state retrieval for resumption
//   - Named checkpoint save/load for branching workflows
//
// Implementations can use:
//   - In-memory storage (for testing, see memory.go)
//   - Relational databases (MySQL, PostgreSQL)
//   - Key-value stores (Redis, DynamoDB)
//   - Object storage (S3, GCS)
//
// Type parameter S is the state type to persist.
type Store[S any] interface {
	// SaveStep persists the state after a node execution step.
	// Each step is identified by runID + step number.
	//
	// Parameters:
	//   - runID: Unique identifier for this workflow execution
	//   - step: Sequential step number (starts at 1)
	//   - nodeID: ID of the node that produced this state
	//   - state: The current workflow state after merging delta
	//
	// Returns error if persistence fails.
	SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error

	// LoadLatest retrieves the most recent state for a given run.
	// Used to resume execution from the last saved step.
	//
	// Parameters:
	//   - runID: Unique identifier for the workflow execution
	//
	// Returns:
	//   - state: The most recent persisted state
	//   - step: The step number of the returned state
	//   - error: ErrNotFound if runID doesn't exist, or other persistence errors
	LoadLatest(ctx context.Context, runID string) (state S, step int, err error)

	// SaveCheckpoint creates a named snapshot of workflow state.
	// Checkpoints enable branching workflows and manual resumption points.
	//
	// Parameters:
	//   - cpID: Unique checkpoint identifier (user-defined)
	//   - state: The workflow state to snapshot
	//   - step: The step number at which this checkpoint was created
	//
	// Returns error if persistence fails.
	SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error

	// LoadCheckpoint retrieves a previously saved checkpoint.
	// Used to restore workflow state from a named checkpoint.
	//
	// Parameters:
	//   - cpID: Unique checkpoint identifier
	//
	// Returns:
	//   - state: The checkpointed state
	//   - step: The step number when checkpoint was created
	//   - error: ErrNotFound if cpID doesn't exist, or other persistence errors
	LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error)
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
type Checkpoint[S any] struct {
	// ID is the unique checkpoint identifier.
	ID string

	// State is the snapshotted workflow state.
	State S

	// Step is the step number when this checkpoint was created.
	Step int
}
