// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

// Edge represents a connection between two nodes in the workflow graph.
//
// Edges define the control flow between nodes. They can be:
// - Unconditional: Always traverse (When = nil).
// - Conditional: Only traverse if predicate returns true (When != nil).
//
// Edges are used during graph construction to define possible transitions.
// At runtime, the Engine evaluates predicates to determine which edge to follow.
//
// For explicit routing, nodes can return Next in NodeResult which overrides.
// edge-based routing.
//
// Type parameter S is the state type used for predicate evaluation.
type Edge[S any] struct {
	// From is the source node ID.
	From string

	// To is the destination node ID.
	To string

	// When is an optional predicate that determines if this edge should be traversed.
	// If nil, the edge is unconditional (always traverse).
	// If non-nil, the edge is only traversed when When(state) returns true.
	When Predicate[S]
}

// Predicate is a function that evaluates state to determine if an edge should be traversed.
//
// Predicates enable conditional routing based on workflow state.
// They should be pure functions (deterministic, no side effects).
//
// Common patterns:
// - Threshold: state.Score > 0.8.
// - Presence: state.Result != "".
// - Boolean flag: state.IsReady.
// - Complex logic: state.Retries < 3 && state.Error == nil.
//
// Type parameter S is the state type to evaluate.
type Predicate[S any] func(state S) bool
