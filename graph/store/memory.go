package store

import (
	"context"
	"sync"
)

// MemStore is an in-memory implementation of Store[S].
//
// It stores workflow state and checkpoints in memory using maps.
// Designed for:
//   - Testing and development
//   - Single-process workflows
//   - Short-lived workflows where persistence isn't required
//
// MemStore is thread-safe and supports concurrent access.
//
// Limitations:
//   - Data is lost when process terminates
//   - Not suitable for distributed systems
//   - Memory usage grows with workflow history
//
// For production use with persistence, use database-backed stores (MySQL, PostgreSQL, Redis).
//
// Type parameter S is the state type to persist.
type MemStore[S any] struct {
	mu          sync.RWMutex
	steps       map[string][]StepRecord[S] // runID -> list of steps
	checkpoints map[string]Checkpoint[S]   // checkpointID -> checkpoint
}

// NewMemStore creates a new in-memory store.
//
// Example:
//
//	store := NewMemStore[MyState]()
//	engine := graph.New(reducer, store, emitter, opts)
func NewMemStore[S any]() *MemStore[S] {
	return &MemStore[S]{
		steps:       make(map[string][]StepRecord[S]),
		checkpoints: make(map[string]Checkpoint[S]),
	}
}

// SaveStep persists a workflow execution step (T036).
//
// Steps are appended to the run's history in the order they are saved.
// Thread-safe for concurrent writes.
func (m *MemStore[S]) SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record := StepRecord[S]{
		Step:   step,
		NodeID: nodeID,
		State:  state,
	}

	m.steps[runID] = append(m.steps[runID], record)
	return nil
}

// LoadLatest retrieves the most recent step for a run (T038).
//
// Returns the step with the highest step number.
// This handles out-of-order step saves correctly.
func (m *MemStore[S]) LoadLatest(ctx context.Context, runID string) (state S, step int, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records, exists := m.steps[runID]
	if !exists || len(records) == 0 {
		var zero S
		return zero, 0, ErrNotFound
	}

	// Find the record with the highest step number
	latest := records[0]
	for _, record := range records[1:] {
		if record.Step > latest.Step {
			latest = record
		}
	}

	return latest.State, latest.Step, nil
}

// SaveCheckpoint creates a named checkpoint (T040).
//
// Checkpoints can be used to:
//   - Create branching workflows (save checkpoint, try different paths)
//   - Mark significant milestones (after-validation, before-deploy)
//   - Provide manual resumption points
//
// If a checkpoint with the same ID exists, it is overwritten.
func (m *MemStore[S]) SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoints[cpID] = Checkpoint[S]{
		ID:    cpID,
		State: state,
		Step:  step,
	}

	return nil
}

// LoadCheckpoint retrieves a named checkpoint (T042).
//
// Returns ErrNotFound if the checkpoint ID doesn't exist.
func (m *MemStore[S]) LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[cpID]
	if !exists {
		var zero S
		return zero, 0, ErrNotFound
	}

	return cp.State, cp.Step, nil
}
