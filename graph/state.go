package graph

import (
	"encoding/json"
	"fmt"
)

// deepCopy creates a deep copy of state S using JSON round-trip serialization (T102).
//
// This approach works for any Go type that can be JSON-marshaled, including:
//   - Primitives (string, int, bool, float64)
//   - Structs with exported fields
//   - Slices and arrays
//   - Maps
//   - Pointers (values are copied, not addresses)
//
// Limitations:
//   - Unexported struct fields are not copied
//   - Channels, functions, and complex types that don't marshal to JSON will fail
//   - Circular references will cause infinite loops
//
// Usage:
//
//	original := MyState{Name: "test", Counter: 42}
//	copied, err := deepCopy(original)
//	if err != nil {
//	    return err
//	}
//	// Now `copied` is independent from `original`
func deepCopy[S any](state S) (S, error) {
	var zero S

	// Serialize to JSON
	data, err := json.Marshal(state)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal state: %w", err)
	}

	// Deserialize back to new instance
	var copied S
	if err := json.Unmarshal(data, &copied); err != nil {
		return zero, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return copied, nil
}

// deepCopyState creates a deep copy of state S for fan-out execution (T037).
//
// This function implements copy-on-write semantics for parallel node execution.
// When a node returns Next.Many (fan-out routing), each branch receives an
// independent copy of the state to ensure isolation between concurrent executions.
//
// The implementation uses JSON serialization for deep copying, which works for
// any JSON-marshalable type but has the following limitations:
//   - Unexported struct fields are not copied
//   - Channels, functions, and complex types that don't marshal to JSON will fail
//   - Circular references will cause infinite loops
//
// For custom state types that require specialized copying logic, consider
// implementing a custom copy method on your state type and calling it from
// a custom reducer.
//
// Example usage in concurrent execution:
//
//	// When node returns Next{Many: []string{"branchA", "branchB"}}
//	for _, branchID := range result.Route.Many {
//	    branchState, err := deepCopyState(currentState)
//	    if err != nil {
//	        return err
//	    }
//	    // Execute branchID with isolated branchState copy
//	}
//
// Thread-safety: This function is safe to call from multiple goroutines as
// it does not modify the input state.
func deepCopyState[S any](state S) (S, error) {
	return deepCopy(state)
}
