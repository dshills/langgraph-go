package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
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
	mu             sync.RWMutex
	steps          map[string][]StepRecord[S] // runID -> list of steps
	checkpoints    map[string]Checkpoint[S]   // checkpointID -> checkpoint
	checkpointsV2  map[string]CheckpointV2[S] // "runID:stepID" -> checkpoint
	labelIndex     map[string]string          // label -> "runID:stepID"
	idempotencyMap map[string]bool            // idempotency key -> exists
	pendingEvents  []emit.Event               // pending events queue
	eventIDSet     map[string]int             // eventID -> index in pendingEvents
}

// NewMemStore creates a new in-memory store.
//
// Example:
//
//	store := NewMemStore[MyState]()
//	engine := graph.New(reducer, store, emitter, opts)
func NewMemStore[S any]() *MemStore[S] {
	return &MemStore[S]{
		steps:          make(map[string][]StepRecord[S]),
		checkpoints:    make(map[string]Checkpoint[S]),
		checkpointsV2:  make(map[string]CheckpointV2[S]),
		labelIndex:     make(map[string]string),
		idempotencyMap: make(map[string]bool),
		pendingEvents:  make([]emit.Event, 0),
		eventIDSet:     make(map[string]int),
	}
}

// SaveStep persists a workflow execution step (T036).
//
// Steps are appended to the run's history in the order they are saved.
// Thread-safe for concurrent writes.
func (m *MemStore[S]) SaveStep(_ context.Context, runID string, step int, nodeID string, state S) error {
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
func (m *MemStore[S]) LoadLatest(_ context.Context, runID string) (state S, step int, err error) {
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
func (m *MemStore[S]) SaveCheckpoint(_ context.Context, cpID string, state S, step int) error {
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
func (m *MemStore[S]) LoadCheckpoint(_ context.Context, cpID string) (state S, step int, err error) {
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
	Steps          map[string][]StepRecord[S] `json:"steps"`
	Checkpoints    map[string]Checkpoint[S]   `json:"checkpoints"`
	CheckpointsV2  map[string]CheckpointV2[S] `json:"checkpoints_v2"`
	LabelIndex     map[string]string          `json:"label_index"`
	IdempotencyMap map[string]bool            `json:"idempotency_map"`
	PendingEvents  []emit.Event               `json:"pending_events"`
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
		Steps:          m.steps,
		Checkpoints:    m.checkpoints,
		CheckpointsV2:  m.checkpointsV2,
		LabelIndex:     m.labelIndex,
		IdempotencyMap: m.idempotencyMap,
		PendingEvents:  m.pendingEvents,
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
	m.checkpointsV2 = s.CheckpointsV2
	m.labelIndex = s.LabelIndex
	m.idempotencyMap = s.IdempotencyMap
	m.pendingEvents = s.PendingEvents

	// Initialize empty maps if nil (for empty JSON objects)
	if m.steps == nil {
		m.steps = make(map[string][]StepRecord[S])
	}
	if m.checkpoints == nil {
		m.checkpoints = make(map[string]Checkpoint[S])
	}
	if m.checkpointsV2 == nil {
		m.checkpointsV2 = make(map[string]CheckpointV2[S])
	}
	if m.labelIndex == nil {
		m.labelIndex = make(map[string]string)
	}
	if m.idempotencyMap == nil {
		m.idempotencyMap = make(map[string]bool)
	}
	if m.pendingEvents == nil {
		m.pendingEvents = make([]emit.Event, 0)
	}

	// Rebuild eventIDSet from pendingEvents
	m.eventIDSet = make(map[string]int)
	for i, event := range m.pendingEvents {
		if event.Meta != nil {
			if id, ok := event.Meta["event_id"].(string); ok {
				m.eventIDSet[id] = i
			}
		}
	}

	return nil
}

// SaveCheckpointV2 persists an enhanced checkpoint with full execution context (T094).
//
// Stores checkpoint indexed by (runID, stepID) and optionally by label if provided.
// Returns error if the idempotency key already exists (duplicate commit prevention).
//
// Thread-safe for concurrent access.
func (m *MemStore[S]) SaveCheckpointV2(_ context.Context, checkpoint CheckpointV2[S]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check idempotency key to prevent duplicate commits
	if checkpoint.IdempotencyKey != "" {
		if m.idempotencyMap[checkpoint.IdempotencyKey] {
			return fmt.Errorf("duplicate checkpoint: idempotency key %q already exists", checkpoint.IdempotencyKey)
		}
		// Mark idempotency key as used
		m.idempotencyMap[checkpoint.IdempotencyKey] = true
	}

	// Create composite key for primary index
	key := fmt.Sprintf("%s:%d", checkpoint.RunID, checkpoint.StepID)
	m.checkpointsV2[key] = checkpoint

	// If labeled, also index by label for named checkpoint retrieval
	if checkpoint.Label != "" {
		m.labelIndex[checkpoint.Label] = key
	}

	return nil
}

// LoadCheckpointV2 retrieves an enhanced checkpoint by run ID and step ID (T095).
//
// Returns ErrNotFound if the checkpoint doesn't exist.
// Thread-safe for concurrent reads.
func (m *MemStore[S]) LoadCheckpointV2(_ context.Context, runID string, stepID int) (CheckpointV2[S], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%d", runID, stepID)
	checkpoint, exists := m.checkpointsV2[key]
	if !exists {
		var zero CheckpointV2[S]
		return zero, ErrNotFound
	}

	return checkpoint, nil
}

// CheckIdempotency verifies if an idempotency key has been used (T096).
//
// Returns (true, nil) if the key exists (has been used).
// Returns (false, nil) if the key doesn't exist (safe to use).
// Only returns error on store access failure (never for this in-memory implementation).
//
// Thread-safe for concurrent access.
func (m *MemStore[S]) CheckIdempotency(_ context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exists := m.idempotencyMap[key]
	return exists, nil
}

// PendingEvents retrieves events from the transactional outbox that haven't been emitted (T097).
//
// Returns up to 'limit' pending events ordered by insertion order.
// Empty list is not an error - it means no events are pending.
//
// Thread-safe for concurrent access.
func (m *MemStore[S]) PendingEvents(_ context.Context, limit int) ([]emit.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return up to 'limit' events
	count := len(m.pendingEvents)
	if limit > 0 && limit < count {
		count = limit
	}

	// Return a copy to prevent external modification
	result := make([]emit.Event, count)
	copy(result, m.pendingEvents[:count])

	return result, nil
}

// MarkEventsEmitted marks events as successfully emitted to prevent re-delivery (T098).
//
// Removes events from the pending queue by their IDs.
// Event IDs should be stored in the event's Meta map with key "event_id".
// If an event ID is not found, it is silently ignored (idempotent operation).
//
// Thread-safe for concurrent access.
func (m *MemStore[S]) MarkEventsEmitted(_ context.Context, eventIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(eventIDs) == 0 {
		return nil // No-op for empty list
	}

	// Build set of IDs to remove for O(1) lookup
	toRemove := make(map[string]bool, len(eventIDs))
	for _, id := range eventIDs {
		toRemove[id] = true
	}

	// Filter out events with matching IDs
	filtered := make([]emit.Event, 0, len(m.pendingEvents))
	newEventIDSet := make(map[string]int)

	for i, event := range m.pendingEvents {
		// Get event ID from Meta map
		eventID := ""
		if event.Meta != nil {
			if id, ok := event.Meta["event_id"].(string); ok {
				eventID = id
			}
		}

		// Keep event if not in removal set
		if !toRemove[eventID] {
			newEventIDSet[eventID] = len(filtered)
			filtered = append(filtered, event)
		} else {
			// Event is being removed, delete from original index
			delete(m.eventIDSet, eventID)
		}
		_ = i // Prevent unused variable warning
	}

	m.pendingEvents = filtered
	m.eventIDSet = newEventIDSet

	return nil
}
