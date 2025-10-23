package graph

import (
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// Engine orchestrates stateful workflow execution with checkpointing support.
//
// The Engine is the core runtime that:
//   - Manages workflow graph topology (nodes and edges)
//   - Executes nodes in sequence or parallel
//   - Merges state updates via the reducer
//   - Persists state at each step via the store
//   - Emits observability events via the emitter
//   - Enforces execution limits (MaxSteps, Retries)
//   - Supports checkpoint save/resume
//
// Type parameter S is the state type shared across the workflow.
//
// Example:
//
//	reducer := func(prev, delta MyState) MyState {
//	    if delta.Query != "" {
//	        prev.Query = delta.Query
//	    }
//	    prev.Steps++
//	    return prev
//	}
//
//	store := store.NewMemStore[MyState]()
//	emitter := emit.NewLogEmitter()
//	opts := Options{MaxSteps: 100}
//
//	engine := New(reducer, store, emitter, opts)
//	engine.Add("process", processNode)
//	engine.StartAt("process")
//
//	final, err := engine.Run(ctx, "run-001", MyState{Query: "hello"})
type Engine[S any] struct {
	mu sync.RWMutex

	// reducer merges partial state updates deterministically
	reducer Reducer[S]

	// nodes maps node IDs to Node implementations
	nodes map[string]Node[S]

	// edges defines conditional transitions between nodes
	edges []Edge[S]

	// startNode is the entry point for workflow execution
	startNode string

	// store persists workflow state and checkpoints
	store store.Store[S]

	// emitter receives observability events
	emitter emit.Emitter

	// opts contains execution configuration
	opts Options

	// currentState tracks the most recent state during execution
	currentState S

	// currentStep tracks the current step number during execution
	currentStep int
}

// Options configures Engine execution behavior.
//
// Zero values are valid - the Engine will use sensible defaults.
type Options struct {
	// MaxSteps limits workflow execution to prevent infinite loops.
	// If 0, no limit is enforced (use with caution).
	// Recommended: Set based on expected workflow depth (e.g., 100 for most workflows).
	MaxSteps int

	// Retries specifies how many times to retry a node on transient errors.
	// If 0, nodes are not retried.
	// Transient errors are identified by checking if error implements Retryable interface.
	Retries int
}

// New creates a new Engine with the given configuration.
//
// Parameters:
//   - reducer: Function to merge partial state updates (required for Run)
//   - store: Persistence backend for state and checkpoints (required for Run)
//   - emitter: Observability event receiver (optional, can be nil)
//   - opts: Execution configuration (MaxSteps, Retries)
//
// The constructor does not validate all parameters to allow flexible initialization.
// Validation occurs when Run() is called.
//
// Example:
//
//	engine := New(
//	    myReducer,
//	    store.NewMemStore[MyState](),
//	    emit.NewLogEmitter(),
//	    Options{MaxSteps: 100, Retries: 3},
//	)
func New[S any](reducer Reducer[S], st store.Store[S], emitter emit.Emitter, opts Options) *Engine[S] {
	return &Engine[S]{
		reducer: reducer,
		nodes:   make(map[string]Node[S]),
		edges:   make([]Edge[S], 0),
		store:   st,
		emitter: emitter,
		opts:    opts,
	}
}

// Add registers a node in the workflow graph.
//
// Nodes must be added before calling StartAt or Run.
// Node IDs must be unique within the workflow.
//
// Parameters:
//   - nodeID: Unique identifier for this node (cannot be empty)
//   - node: Node implementation (cannot be nil)
//
// Returns error if:
//   - nodeID is empty
//   - node is nil
//   - a node with this ID already exists
//
// Example:
//
//	processNode := NodeFunc[MyState](func(ctx context.Context, s MyState) NodeResult[MyState] {
//	    return NodeResult[MyState]{
//	        Delta: MyState{Result: "processed"},
//	        Route: Stop(),
//	    }
//	})
//
//	err := engine.Add("process", processNode)
func (e *Engine[S]) Add(nodeID string, node Node[S]) error {
	if nodeID == "" {
		return &EngineError{Message: "node ID cannot be empty"}
	}
	if node == nil {
		return &EngineError{Message: "node cannot be nil"}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[nodeID]; exists {
		return &EngineError{
			Message: "duplicate node ID: " + nodeID,
			Code:    "DUPLICATE_NODE",
		}
	}

	e.nodes[nodeID] = node
	return nil
}

// StartAt sets the entry point for workflow execution.
//
// The start node is executed first when Run() is called.
// The node must have been registered via Add() before calling StartAt.
//
// Parameters:
//   - nodeID: ID of the node to start execution at
//
// Returns error if:
//   - nodeID is empty
//   - node with this ID doesn't exist
//
// Example:
//
//	engine.Add("start", startNode)
//	engine.StartAt("start")
func (e *Engine[S]) StartAt(nodeID string) error {
	if nodeID == "" {
		return &EngineError{Message: "start node ID cannot be empty"}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.nodes[nodeID]; !exists {
		return &EngineError{
			Message: "start node does not exist: " + nodeID,
			Code:    "NODE_NOT_FOUND",
		}
	}

	e.startNode = nodeID
	return nil
}

// Connect creates an edge between two nodes.
//
// Edges define possible transitions in the workflow graph.
// They can be:
//   - Unconditional: Always traverse (predicate = nil)
//   - Conditional: Only traverse if predicate returns true
//
// Node explicit routing via NodeResult.Route takes precedence over edges.
//
// Parameters:
//   - from: Source node ID (cannot be empty)
//   - to: Destination node ID (cannot be empty)
//   - predicate: Optional condition for traversal (nil = unconditional)
//
// Returns error if:
//   - from or to is empty
//
// Note: Node existence is not validated (lazy validation) to allow
// flexible graph construction order.
//
// Example:
//
//	// Unconditional edge
//	engine.Connect("nodeA", "nodeB", nil)
//
//	// Conditional edge
//	engine.Connect("router", "pathA", func(s MyState) bool {
//	    return s.Score > 0.8
//	})
func (e *Engine[S]) Connect(from, to string, predicate Predicate[S]) error {
	if from == "" {
		return &EngineError{Message: "from node ID cannot be empty"}
	}
	if to == "" {
		return &EngineError{Message: "to node ID cannot be empty"}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	edge := Edge[S]{
		From: from,
		To:   to,
		When: predicate,
	}

	e.edges = append(e.edges, edge)
	return nil
}

// EngineError represents an error from Engine operations.
type EngineError struct {
	Message string
	Code    string
}

func (e *EngineError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}
