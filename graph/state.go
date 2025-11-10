// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"encoding/json"
	"fmt"
)

// StateCopier is an optional interface that state types can implement to provide
// custom deep copy logic. When a state type implements this interface, the engine
// will use the custom DeepCopy method instead of JSON-based serialization.
//
// This interface allows users to:
// - Avoid JSON serialization overhead in performance-critical paths
// - Properly handle unexported fields, channels, or other non-JSON-serializable types
// - Implement application-specific copying semantics
//
// The interface uses any return type to accommodate both value and pointer receiver
// implementations, avoiding generic interface instantiation issues where S = *T would
// require DeepCopy() (*T, error) instead of DeepCopy() (T, error).
//
// Example implementations:
//
// Value receiver (returns value):
//
//	type MyState struct {
//	    Counter int
//	    Data    []byte
//	}
//
//	func (s MyState) DeepCopy() (any, error) {
//	    copied := MyState{
//	        Counter: s.Counter,
//	        Data:    append([]byte(nil), s.Data...),
//	    }
//	    return copied, nil
//	}
//
// Pointer receiver (returns pointer):
//
//	type MyState struct {
//	    Counter int
//	    Data    []byte
//	}
//
//	func (s *MyState) DeepCopy() (any, error) {
//	    copied := &MyState{
//	        Counter: s.Counter,
//	        Data:    append([]byte(nil), s.Data...),
//	    }
//	    return copied, nil
//	}
//
// Thread-safety: DeepCopy implementations must be safe to call from multiple
// goroutines without external synchronization.
type StateCopier interface {
	DeepCopy() (any, error)
}

// deepCopy creates a deep copy of state S using JSON round-trip serialization (T102).
//
// This approach works for any Go type that can be JSON-marshaled, including:
// - Primitives (string, int, bool, float64).
// - Structs with exported fields.
// - Slices and arrays.
// - Maps.
// - Pointers (values are copied, not addresses).
//
// Limitations:
// - Unexported struct fields are not copied.
// - Channels, functions, and complex types that don't marshal to JSON will fail.
// - Circular references will cause infinite loops.
//
// Usage:
//
// original := MyState{Name: "test", Counter: 42}.
// copied, err := deepCopy(original).
// if err != nil {.
// return err.
// }.
// // Now `copied` is independent from `original`.
func deepCopy[S any](state S) (S, error) {
	var zero S

	// Serialize to JSON.
	data, err := json.Marshal(state)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal state: %w", err)
	}

	// Deserialize back to new instance.
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
// Copy Strategy:
// 1. If state implements StateCopier, uses the custom DeepCopy method (fastest, most flexible)
// 2. Otherwise, uses JSON serialization (slower, limited to JSON-serializable types)
//
// JSON serialization limitations:
// - Unexported struct fields are silently dropped
// - Channels, functions, and complex types will cause errors
// - Circular references will cause stack overflow panics
// - interface{} values lose type information
//
// For performance-critical fan-out operations, implement StateCopier on your
// state type to avoid JSON serialization overhead.
//
// Example custom copier:
//
//	type MyState struct {
//	    Counter int
//	    Data    []byte
//	}
//
//	func (s MyState) DeepCopy() (any, error) {
//	    return MyState{
//	        Counter: s.Counter,
//	        Data:    append([]byte(nil), s.Data...),
//	    }, nil
//	}
//
// Thread-safety: This function is safe to call from multiple goroutines as
// it does not modify the input state.
func deepCopyState[S any](state S) (S, error) {
	var zero S

	// Check if state implements StateCopier interface
	if copier, ok := any(state).(StateCopier); ok {
		v, err := copier.DeepCopy()
		if err != nil {
			return zero, fmt.Errorf("custom DeepCopy failed: %w", err)
		}

		// Type-assert the result back to S
		copied, ok := v.(S)
		if !ok {
			return zero, fmt.Errorf("DeepCopy returned wrong type: got %T, want %T", v, zero)
		}

		return copied, nil
	}

	// Fall back to JSON-based deep copy
	return deepCopy(state)
}
