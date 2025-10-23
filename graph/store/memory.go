package store

import (
	"context"
	"encoding/json"
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

// serializableMemStore is the JSON-serializable representation of MemStore.
//
// Used for persisting MemStore contents to disk or transmitting over network.
// The generic type S must be JSON-serializable (implement json.Marshaler or have exported fields).
type serializableMemStore[S any] struct {
	Steps       map[string][]StepRecord[S] `json:"steps"`
	Checkpoints map[string]Checkpoint[S]   `json:"checkpoints"`
}

// MarshalJSON serializes the MemStore to JSON (T072).
//
// The resulting JSON can be saved to disk, transmitted over network, or used for debugging.
// All state values must be JSON-serializable.
//
// Thread-safe: acquires read lock during serialization.
//
// Example:
//
//	store := NewMemStore[MyState]()
//	// ... add steps and checkpoints ...
//	data, err := store.MarshalJSON()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	os.WriteFile("store.json", data, 0644)
func (m *MemStore[S]) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create serializable representation
	s := serializableMemStore[S]{
		Steps:       m.steps,
		Checkpoints: m.checkpoints,
	}

	return json.Marshal(s)
}

// UnmarshalJSON deserializes JSON data into the MemStore (T074).
//
// Replaces the current contents of the MemStore with the deserialized data.
// All existing steps and checkpoints are discarded.
//
// Thread-safe: acquires write lock during deserialization.
//
// Example:
//
//	data, _ := os.ReadFile("store.json")
//	store := NewMemStore[MyState]()
//	if err := store.UnmarshalJSON(data); err != nil {
//	    log.Fatal(err)
//	}
//	// Store now contains data from JSON file
func (m *MemStore[S]) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unmarshal into temporary struct
	var s serializableMemStore[S]
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Replace store contents
	m.steps = s.Steps
	m.checkpoints = s.Checkpoints

	// Initialize empty maps if nil (for empty JSON objects)
	if m.steps == nil {
		m.steps = make(map[string][]StepRecord[S])
	}
	if m.checkpoints == nil {
		m.checkpoints = make(map[string]Checkpoint[S])
	}

	return nil
}
