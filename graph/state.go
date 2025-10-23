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
