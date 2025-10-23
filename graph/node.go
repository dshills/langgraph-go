package graph

import "context"

// Node represents a processing unit in the workflow graph.
// It receives state of type S, performs computation, and returns a NodeResult.
//
// Nodes are the fundamental building blocks of LangGraph workflows.
// Each node can:
//   - Access the current state
//   - Perform computation (call LLMs, tools, or custom logic)
//   - Return state modifications via Delta
//   - Control routing via Route
//   - Emit observability events
//   - Handle errors
//
// Type parameter S is the state type shared across the workflow.
type Node[S any] interface {
	// Run executes the node's logic with the given context and state.
	// It returns a NodeResult containing state changes, routing decisions,
	// events, and any errors encountered.
	Run(ctx context.Context, state S) NodeResult[S]
}

// NodeResult represents the output of a node execution.
//
// It contains all information needed to continue workflow execution:
//   - Delta: Partial state update to be merged via reducer
//   - Route: Next hop(s) for execution flow
//   - Events: Observability events emitted during execution
//   - Err: Node-level error (if any)
type NodeResult[S any] struct {
	// Delta is the partial state update produced by this node.
	// It will be merged with the current state using the configured reducer.
	Delta S

	// Route specifies the next step(s) in workflow execution.
	// Use Stop() for terminal nodes, Goto(id) for explicit routing,
	// or set Many for fan-out to multiple nodes.
	Route Next

	// TODO: Add Events []Event field after T029-T030 (Event type definition)

	// Err contains any error that occurred during node execution.
	// Non-nil errors halt the workflow unless custom error handling is implemented.
	Err error
}

// Next specifies the next step(s) in workflow execution after a node completes.
//
// It supports three routing modes:
//   - Terminal: Stop execution (Route.Terminal = true)
//   - Single: Go to a specific node (Route.To = "nodeID")
//   - Fan-out: Go to multiple nodes in parallel (Route.Many = []string{"node1", "node2"})
type Next struct {
	// To specifies the next single node to execute.
	// Mutually exclusive with Many and Terminal.
	To string

	// Many specifies multiple nodes to execute in parallel (fan-out).
	// Mutually exclusive with To and Terminal.
	Many []string

	// Terminal indicates workflow execution should stop.
	// Mutually exclusive with To and Many.
	Terminal bool
}

// Stop returns a Next that terminates workflow execution.
func Stop() Next {
	return Next{Terminal: true}
}

// Goto returns a Next that routes to the specified node.
func Goto(nodeID string) Next {
	return Next{To: nodeID}
}

// NodeFunc is a function adapter that implements the Node interface.
// It allows using plain functions as nodes without creating custom types.
//
// Example:
//
//	processNode := NodeFunc[MyState](func(ctx context.Context, s MyState) NodeResult[MyState] {
//	    return NodeResult[MyState]{
//	        Delta: MyState{Result: "processed"},
//	        Route: Stop(),
//	    }
//	})
type NodeFunc[S any] func(ctx context.Context, state S) NodeResult[S]

// Run implements the Node interface for NodeFunc.
func (f NodeFunc[S]) Run(ctx context.Context, state S) NodeResult[S] {
	return f(ctx, state)
}

// NodeError represents an error that occurred during node execution.
// It provides structured error information for better observability and debugging.
type NodeError struct {
	// Message is the human-readable error description.
	Message string

	// Code is a machine-readable error code for programmatic handling.
	Code string

	// NodeID identifies which node produced this error.
	NodeID string

	// Cause is the underlying error that caused this NodeError.
	Cause error
}

// Error implements the error interface.
func (e *NodeError) Error() string {
	if e.NodeID != "" {
		return "node " + e.NodeID + ": " + e.Message
	}
	return e.Message
}

// Unwrap returns the underlying cause error for error wrapping support.
func (e *NodeError) Unwrap() error {
	return e.Cause
}
