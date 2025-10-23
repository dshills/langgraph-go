package graph

// Reducer is a function that merges partial state updates (delta) into accumulated state (prev).
//
// Reducers are the core of LangGraph's deterministic state management.
// They define how state evolves as nodes produce Delta values in NodeResult.
//
// Key properties of reducers:
//   - Deterministic: Same (prev, delta) always produces same result
//   - Associative: Applying deltas in sequence produces consistent state
//   - Idempotent-friendly: Replaying same deltas should be safe
//
// Common patterns:
//   - Replace: If delta field is non-zero, use it; otherwise keep prev
//   - Accumulate: Add delta values to prev values (counters, lists)
//   - Merge: Deep merge for nested structs/maps
//
// Example:
//
//	func reduce(prev, delta MyState) MyState {
//	    if delta.Query != "" {
//	        prev.Query = delta.Query
//	    }
//	    prev.Steps += delta.Steps // accumulate
//	    return prev
//	}
//
// Type parameter S is the state type shared across the workflow.
type Reducer[S any] func(prev S, delta S) S
