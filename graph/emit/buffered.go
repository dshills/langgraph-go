package emit

import "sync"

// BufferedEmitter implements Emitter by storing events in memory (T169-T172).
//
// This emitter captures all events and provides query capabilities for
// execution history analysis. Events are organized by runID for efficient
// retrieval and filtering.
//
// Features:
//   - Thread-safe concurrent access
//   - Query by runID with optional filtering
//   - Filter by nodeID, message, step range
//   - Clear events by runID or all events
//
// Use cases:
//   - Development and debugging
//   - Testing and validation
//   - Real-time monitoring dashboards
//   - Post-execution analysis
//
// Warning: This emitter stores all events in memory. For production
// deployments with long-running workflows or high event volume, consider
// using a persistent storage backend or implement event rotation/cleanup.
//
// Example usage:
//
//	// Create buffered emitter for testing
//	emitter := emit.NewBufferedEmitter()
//	engine := graph.New(reducer, store, emitter, opts)
//
//	// Run workflow
//	engine.Run(ctx, "run-001", initialState)
//
//	// Query execution history
//	allEvents := emitter.GetHistory("run-001")
//	errorEvents := emitter.GetHistoryWithFilter("run-001", emit.HistoryFilter{Msg: "error"})
//
//	// Clean up old runs
//	emitter.Clear("run-001")
type BufferedEmitter struct {
	mu     sync.RWMutex
	events map[string][]Event // runID -> events
}

// HistoryFilter specifies criteria for filtering execution history (T171, T172).
//
// All filter fields are optional. When multiple fields are set, they are
// combined with AND logic (all conditions must match).
//
// Fields:
//   - NodeID: Filter by specific node
//   - Msg: Filter by message type (e.g., "node_start", "error")
//   - MinStep: Filter events with step >= MinStep (nil = no lower bound)
//   - MaxStep: Filter events with step <= MaxStep (nil = no upper bound)
//
// Example usage:
//
//	// Get all errors from a specific node
//	filter := emit.HistoryFilter{
//		NodeID: "validator",
//		Msg:    "error",
//	}
//	errors := emitter.GetHistoryWithFilter("run-001", filter)
//
//	// Get events from steps 5-10
//	minStep, maxStep := 5, 10
//	filter := emit.HistoryFilter{
//		MinStep: &minStep,
//		MaxStep: &maxStep,
//	}
//	stepEvents := emitter.GetHistoryWithFilter("run-001", filter)
type HistoryFilter struct {
	NodeID  string // Filter by node ID (empty = no filter)
	Msg     string // Filter by message (empty = no filter)
	MinStep *int   // Minimum step number (nil = no filter)
	MaxStep *int   // Maximum step number (nil = no filter)
}

// NewBufferedEmitter creates a new BufferedEmitter (T169).
//
// Returns a BufferedEmitter that stores all events in memory and provides
// query capabilities. Safe for concurrent use.
func NewBufferedEmitter() *BufferedEmitter {
	return &BufferedEmitter{
		events: make(map[string][]Event),
	}
}

// Emit stores an event in the buffer (T169).
//
// Events are organized by runID for efficient retrieval. This method is
// thread-safe and can be called concurrently from multiple goroutines.
func (b *BufferedEmitter) Emit(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events[event.RunID] = append(b.events[event.RunID], event)
}

// GetHistory retrieves all events for a specific runID (T170).
//
// Returns events in the order they were emitted. Returns an empty slice
// if no events exist for the given runID.
//
// This method is thread-safe and returns a copy of the events to prevent
// concurrent modification issues.
//
// Example:
//
//	events := emitter.GetHistory("run-001")
//	for _, event := range events {
//		fmt.Printf("[%s] %s: %s\n", event.RunID, event.NodeID, event.Msg)
//	}
func (b *BufferedEmitter) GetHistory(runID string) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	events := b.events[runID]
	if events == nil {
		return []Event{} // Return empty slice instead of nil
	}

	// Return a copy to prevent external modification
	result := make([]Event, len(events))
	copy(result, events)
	return result
}

// GetHistoryWithFilter retrieves filtered events for a specific runID (T171, T172).
//
// Applies the provided filter criteria to select matching events. All filter
// conditions must match for an event to be included (AND logic).
//
// Returns events in the order they were emitted. Returns an empty slice if
// no events match the filter.
//
// This method is thread-safe and returns a copy of the events.
//
// Example:
//
//	// Get error events from "validator" node
//	filter := emit.HistoryFilter{
//		NodeID: "validator",
//		Msg:    "error",
//	}
//	errors := emitter.GetHistoryWithFilter("run-001", filter)
//
//	// Get events from steps 10-20
//	minStep, maxStep := 10, 20
//	filter := emit.HistoryFilter{
//		MinStep: &minStep,
//		MaxStep: &maxStep,
//	}
//	stepEvents := emitter.GetHistoryWithFilter("run-001", filter)
func (b *BufferedEmitter) GetHistoryWithFilter(runID string, filter HistoryFilter) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	events := b.events[runID]
	if events == nil {
		return []Event{}
	}

	// If filter is empty, return all events
	if filter.NodeID == "" && filter.Msg == "" && filter.MinStep == nil && filter.MaxStep == nil {
		result := make([]Event, len(events))
		copy(result, events)
		return result
	}

	// Apply filters
	var result []Event
	for _, event := range events {
		if !b.matchesFilter(event, filter) {
			continue
		}
		result = append(result, event)
	}

	if result == nil {
		return []Event{} // Return empty slice instead of nil
	}
	return result
}

// matchesFilter checks if an event matches the filter criteria.
func (b *BufferedEmitter) matchesFilter(event Event, filter HistoryFilter) bool {
	// Filter by NodeID
	if filter.NodeID != "" && event.NodeID != filter.NodeID {
		return false
	}

	// Filter by Msg
	if filter.Msg != "" && event.Msg != filter.Msg {
		return false
	}

	// Filter by MinStep
	if filter.MinStep != nil && event.Step < *filter.MinStep {
		return false
	}

	// Filter by MaxStep
	if filter.MaxStep != nil && event.Step > *filter.MaxStep {
		return false
	}

	return true
}

// Clear removes stored events (T170).
//
// If runID is non-empty, clears only events for that specific run.
// If runID is empty, clears all stored events across all runs.
//
// This method is thread-safe and can be called concurrently.
//
// Example:
//
//	// Clear specific run
//	emitter.Clear("run-001")
//
//	// Clear all runs
//	emitter.Clear("")
func (b *BufferedEmitter) Clear(runID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if runID == "" {
		// Clear all events
		b.events = make(map[string][]Event)
	} else {
		// Clear specific runID
		delete(b.events, runID)
	}
}
