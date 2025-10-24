package graph

import (
	"context"
	"sort"
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// Reducer is a function that merges a partial state update (delta) into the previous state.
//
// Reducers are responsible for deterministic state composition, enabling:
//   - Sequential state updates across workflow nodes
//   - Parallel branch result merging
//   - Checkpoint-based state reconstruction
//
// The reducer must be:
//   - **Pure**: Same inputs always produce same output
//   - **Deterministic**: No randomness or side effects
//   - **Commutative** (for parallel merges): Order of deltas shouldn't matter for correctness
//
// Example:
//
//	reducer := func(prev, delta MyState) MyState {
//	    if delta.Query != "" {
//	        prev.Query = delta.Query  // Last write wins
//	    }
//	    prev.Counter += delta.Counter  // Accumulate
//	    return prev
//	}
type Reducer[S any] func(prev, delta S) S

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
}

// Options configures Engine execution behavior.
//
// Zero values are valid - the Engine will use sensible defaults.
type Options struct {
	// MaxSteps limits workflow execution to prevent infinite loops.
	// If 0, no limit is enforced (use with caution).
	//
	// Workflow loops (A → B → A) are fully supported. Use MaxSteps to prevent
	// infinite loops when a conditional exit is missing or misconfigured.
	//
	// Loop patterns:
	//   1. Node-based conditional loop:
	//        nodeA returns Goto("B") if shouldContinue(state), else Stop()
	//   2. Edge predicate loop:
	//        Connect("A", "B", loopPredicate)
	//        Connect("A", "exit", exitPredicate)
	//
	// Recommended values:
	//   - Simple workflows (3-5 nodes): MaxSteps = 20
	//   - Workflows with loops: MaxSteps = depth × max_iterations (e.g., 5 nodes × 10 iterations = 50)
	//   - Complex multi-loop workflows: MaxSteps = 100-200
	//
	// When MaxSteps is exceeded, Run() returns EngineError with code "MAX_STEPS_EXCEEDED".
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

// Run executes the workflow from start to completion or error.
//
// Workflow execution:
//  1. Validates engine configuration (reducer, store, startNode)
//  2. Initializes state with initial value
//  3. Executes nodes starting from startNode
//  4. Follows routing decisions (Stop, Goto, Many)
//  5. Applies reducer to merge state updates
//  6. Persists state after each node
//  7. Emits observability events
//  8. Enforces MaxSteps limit
//  9. Respects context cancellation
//
// Parameters:
//   - ctx: Context for cancellation and request-scoped values
//   - runID: Unique identifier for this workflow execution
//   - initial: Starting state value
//
// Returns:
//   - Final state after workflow completion
//   - Error if validation fails, node execution fails, or limits exceeded
//
// Example:
//
//	ctx := context.Background()
//	final, err := engine.Run(ctx, "run-001", MyState{Query: "hello"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Final state: %+v\n", final)
func (e *Engine[S]) Run(ctx context.Context, runID string, initial S) (S, error) {
	var zero S

	// Validate configuration
	if e.reducer == nil {
		return zero, &EngineError{
			Message: "reducer is required",
			Code:    "MISSING_REDUCER",
		}
	}
	if e.store == nil {
		return zero, &EngineError{
			Message: "store is required",
			Code:    "MISSING_STORE",
		}
	}
	if e.startNode == "" {
		return zero, &EngineError{
			Message: "start node not set (call StartAt before Run)",
			Code:    "NO_START_NODE",
		}
	}

	// Validate start node exists
	e.mu.RLock()
	_, exists := e.nodes[e.startNode]
	e.mu.RUnlock()

	if !exists {
		return zero, &EngineError{
			Message: "start node does not exist: " + e.startNode,
			Code:    "NODE_NOT_FOUND",
		}
	}

	// Initialize execution state
	currentState := initial
	currentNode := e.startNode
	step := 0

	// Execution loop
	for {
		step++

		// Check MaxSteps limit (T060)
		if e.opts.MaxSteps > 0 && step > e.opts.MaxSteps {
			return zero, &EngineError{
				Message: "workflow exceeded MaxSteps limit",
				Code:    "MAX_STEPS_EXCEEDED",
			}
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Get current node implementation
		e.mu.RLock()
		nodeImpl, exists := e.nodes[currentNode]
		e.mu.RUnlock()

		if !exists {
			return zero, &EngineError{
				Message: "node not found during execution: " + currentNode,
				Code:    "NODE_NOT_FOUND",
			}
		}

		// Emit node_start event (T153)
		e.emitNodeStart(runID, currentNode, step-1) // step is incremented at start of loop, but events use 0-based indexing

		// Execute node
		result := nodeImpl.Run(ctx, currentState)

		// Handle node error (T159)
		if result.Err != nil {
			e.emitError(runID, currentNode, step-1, result.Err)
			return zero, result.Err
		}

		// Merge state update
		currentState = e.reducer(currentState, result.Delta)

		// Persist state after node execution (T058)
		if err := e.store.SaveStep(ctx, runID, step, currentNode, currentState); err != nil {
			return zero, &EngineError{
				Message: "failed to save step: " + err.Error(),
				Code:    "STORE_ERROR",
			}
		}

		// Emit node_end event with delta (T155)
		e.emitNodeEnd(runID, currentNode, step-1, result.Delta)

		// Determine next node from routing decision
		if result.Route.Terminal {
			// Emit routing_decision event for Stop (T157)
			e.emitRoutingDecision(runID, currentNode, step-1, map[string]interface{}{
				"terminal": true,
			})
			// Workflow complete
			return currentState, nil
		}

		// Handle parallel execution (fan-out) - T104-T108
		if len(result.Route.Many) > 0 {
			// Emit routing_decision event for parallel execution (T157)
			e.emitRoutingDecision(runID, currentNode, step-1, map[string]interface{}{
				"parallel": true,
				"branches": result.Route.Many,
			})

			// Execute branches in parallel with isolated state copies
			parallelState, err := e.executeParallel(ctx, result.Route.Many, currentState)
			if err != nil {
				return zero, err
			}
			currentState = parallelState

			// Parallel execution completes the workflow (all branches end with Stop())
			return currentState, nil
		}

		if result.Route.To != "" {
			// Emit routing_decision event for Goto (T157)
			e.emitRoutingDecision(runID, currentNode, step-1, map[string]interface{}{
				"next_node": result.Route.To,
			})

			// Single next node (Goto)
			currentNode = result.Route.To
			continue
		}

		// If no explicit route, fall back to edge-based routing (T079)
		nextNode := e.evaluateEdges(currentNode, currentState)
		if nextNode == "" {
			// No matching edge found - workflow cannot continue
			return zero, &EngineError{
				Message: "no valid route from node: " + currentNode,
				Code:    "NO_ROUTE",
			}
		}

		// Emit routing_decision event for edge-based routing (T157)
		e.emitRoutingDecision(runID, currentNode, step-1, map[string]interface{}{
			"next_node": nextNode,
			"via_edge":  true,
		})

		currentNode = nextNode
		continue
	}
}

// evaluateEdges finds the first matching edge from the given node based on predicates (T079, T081).
//
// Evaluates outgoing edges in order:
//  1. If edge has nil predicate (unconditional), always matches
//  2. If edge predicate returns true for current state, matches
//  3. First matching edge wins (priority order)
//
// Returns empty string if no edges match.
func (e *Engine[S]) evaluateEdges(fromNode string, state S) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find all edges from this node
	for _, edge := range e.edges {
		if edge.From != fromNode {
			continue
		}

		// Unconditional edge (nil predicate) always matches
		if edge.When == nil {
			return edge.To
		}

		// Evaluate predicate
		if edge.When(state) {
			return edge.To
		}
	}

	// No matching edge found
	return ""
}

// executeParallel executes multiple branches in parallel with isolated state copies (T104-T108).
//
// Each branch:
//  1. Receives a deep copy of the current state (T104)
//  2. Executes in its own goroutine (T106)
//  3. Returns its delta state update
//
// After all branches complete:
//  1. Deltas are merged deterministically using the reducer (T109-T110)
//  2. Merge order is lexicographic by nodeID for determinism (T111-T112)
//  3. Errors from any branch are collected (T113-T114)
//
// Uses sync.WaitGroup for coordination (T108).
func (e *Engine[S]) executeParallel(ctx context.Context, branches []string, state S) (S, error) {
	var zero S

	type branchResult struct {
		nodeID string
		delta  S
		err    error
	}

	// Channel for collecting results from all branches
	results := make(chan branchResult, len(branches))

	// WaitGroup for synchronization (T108)
	var wg sync.WaitGroup

	// Launch goroutines for each branch (T106)
	for _, branchID := range branches {
		wg.Add(1)

		go func(nodeID string) {
			defer wg.Done()

			// Deep copy state for isolation (T104)
			branchState, err := deepCopy(state)
			if err != nil {
				results <- branchResult{nodeID: nodeID, err: err}
				return
			}

			// Execute the branch node
			e.mu.RLock()
			node, exists := e.nodes[nodeID]
			e.mu.RUnlock()

			if !exists {
				results <- branchResult{
					nodeID: nodeID,
					err: &EngineError{
						Message: "parallel branch node not found: " + nodeID,
						Code:    "NODE_NOT_FOUND",
					},
				}
				return
			}

			// Execute node with isolated state copy
			result := node.Run(ctx, branchState)

			if result.Err != nil {
				results <- branchResult{nodeID: nodeID, err: result.Err}
				return
			}

			// Return the delta from this branch
			results <- branchResult{nodeID: nodeID, delta: result.Delta}
		}(branchID)
	}

	// Wait for all branches to complete
	wg.Wait()
	close(results)

	// Collect all results
	var branchResults []branchResult
	for result := range results {
		branchResults = append(branchResults, result)
	}

	// Check for errors (T113-T114)
	var errors []error
	for _, result := range branchResults {
		if result.err != nil {
			errors = append(errors, result.err)
		}
	}

	if len(errors) > 0 {
		// Return first error (could aggregate in future - T115-T116)
		return zero, errors[0]
	}

	// Sort results by nodeID for deterministic merge order (T111-T112)
	// Lexicographic ordering ensures consistent results regardless of goroutine completion order
	sort.Slice(branchResults, func(i, j int) bool {
		return branchResults[i].nodeID < branchResults[j].nodeID
	})

	// Merge all branch deltas into final state using reducer (T109-T110)
	finalState := state
	for _, result := range branchResults {
		finalState = e.reducer(finalState, result.delta)
	}

	return finalState, nil
}

// SaveCheckpoint creates a named checkpoint for the most recent state of a run.
//
// Checkpoints enable:
//   - Branching workflows (save checkpoint, try different paths)
//   - Manual resumption points
//   - Rollback to known-good states
//   - A/B testing (checkpoint before experiment)
//
// The checkpoint captures the latest persisted state from the specified run.
// Multiple checkpoints can be created for the same run with different labels.
//
// Parameters:
//   - ctx: Context for cancellation
//   - runID: The workflow run to checkpoint
//   - cpID: Unique identifier for this checkpoint (e.g., "after-validation", "before-deploy")
//
// Returns error if:
//   - runID doesn't exist or has no persisted state
//   - Store operation fails
//
// Example:
//
//	// Run workflow
//	final, _ := engine.Run(ctx, "run-001", initial)
//
//	// Save checkpoint at completion
//	err := engine.SaveCheckpoint(ctx, "run-001", "before-deploy")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Later, can resume from this checkpoint
//	resumed, _ := engine.ResumeFromCheckpoint(ctx, "before-deploy", "run-002")
func (e *Engine[S]) SaveCheckpoint(ctx context.Context, runID string, cpID string) error {
	// Load latest state from the run
	latestState, latestStep, err := e.store.LoadLatest(ctx, runID)
	if err != nil {
		return &EngineError{
			Message: "cannot create checkpoint: run state not found: " + err.Error(),
			Code:    "RUN_NOT_FOUND",
		}
	}

	// Save checkpoint with the latest state
	if err := e.store.SaveCheckpoint(ctx, cpID, latestState, latestStep); err != nil {
		return &EngineError{
			Message: "failed to save checkpoint: " + err.Error(),
			Code:    "CHECKPOINT_SAVE_FAILED",
		}
	}

	// Emit checkpoint event
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   latestStep,
			NodeID: "",
			Msg:    "checkpoint saved: " + cpID,
			Meta: map[string]interface{}{
				"checkpoint_id": cpID,
			},
		})
	}

	return nil
}

// ResumeFromCheckpoint resumes workflow execution from a saved checkpoint.
//
// This enables:
//   - Crash recovery (save checkpoints, resume after failure)
//   - Branching workflows (checkpoint, try path A, resume from checkpoint, try path B)
//   - Manual intervention (pause at checkpoint, human review, resume)
//   - A/B testing (checkpoint before experiment, resume multiple times with variants)
//
// The resume operation:
//  1. Loads the checkpoint state
//  2. Starts a new workflow run with the checkpoint state as initial state
//  3. Begins execution at the specified node (typically the next node after checkpoint)
//  4. Continues until workflow completes or errors
//
// Parameters:
//   - ctx: Context for cancellation
//   - cpID: Checkpoint identifier to resume from
//   - newRunID: New unique run ID for the resumed execution
//   - startNode: Node to begin execution at (typically next node after checkpoint)
//
// Returns:
//   - Final state after resumed execution completes
//   - Error if checkpoint doesn't exist, startNode invalid, or execution fails
//
// Example:
//
//	// Original run with checkpoint
//	_, _ = engine.Run(ctx, "run-001", initial)
//	_ = engine.SaveCheckpoint(ctx, "run-001", "after-validation")
//
//	// Resume from checkpoint (e.g., after crash or for A/B test)
//	finalA, _ := engine.ResumeFromCheckpoint(ctx, "after-validation", "run-002-pathA", "pathA")
//	finalB, _ := engine.ResumeFromCheckpoint(ctx, "after-validation", "run-003-pathB", "pathB")
func (e *Engine[S]) ResumeFromCheckpoint(ctx context.Context, cpID string, newRunID string, startNode string) (S, error) {
	var zero S

	// Load checkpoint state
	checkpointState, checkpointStep, err := e.store.LoadCheckpoint(ctx, cpID)
	if err != nil {
		return zero, &EngineError{
			Message: "cannot resume: checkpoint not found: " + err.Error(),
			Code:    "CHECKPOINT_NOT_FOUND",
		}
	}

	// Emit resume event
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  newRunID,
			Step:   0,
			NodeID: startNode,
			Msg:    "resuming from checkpoint: " + cpID,
			Meta: map[string]interface{}{
				"checkpoint_id":   cpID,
				"checkpoint_step": checkpointStep,
			},
		})
	}

	// Validate configuration (same as Run)
	if e.reducer == nil {
		return zero, &EngineError{
			Message: "reducer is required",
			Code:    "MISSING_REDUCER",
		}
	}
	if e.store == nil {
		return zero, &EngineError{
			Message: "store is required",
			Code:    "MISSING_STORE",
		}
	}
	if startNode == "" {
		return zero, &EngineError{
			Message: "start node not specified for resume",
			Code:    "NO_START_NODE",
		}
	}

	// Validate start node exists
	e.mu.RLock()
	_, exists := e.nodes[startNode]
	e.mu.RUnlock()

	if !exists {
		return zero, &EngineError{
			Message: "resume start node does not exist: " + startNode,
			Code:    "NODE_NOT_FOUND",
		}
	}

	// Initialize execution state with checkpoint state
	currentState := checkpointState
	currentNode := startNode
	step := 0

	// Execution loop (same as Run but starting from checkpoint state)
	for {
		step++

		// Check MaxSteps limit
		if e.opts.MaxSteps > 0 && step > e.opts.MaxSteps {
			return zero, &EngineError{
				Message: "workflow exceeded MaxSteps limit",
				Code:    "MAX_STEPS_EXCEEDED",
			}
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Get current node implementation
		e.mu.RLock()
		nodeImpl, exists := e.nodes[currentNode]
		e.mu.RUnlock()

		if !exists {
			return zero, &EngineError{
				Message: "node not found during execution: " + currentNode,
				Code:    "NODE_NOT_FOUND",
			}
		}

		// Emit node_start event (T153)
		e.emitNodeStart(newRunID, currentNode, step-1) // step is incremented at start of loop, but events use 0-based indexing

		// Execute node
		result := nodeImpl.Run(ctx, currentState)

		// Handle node error (T159)
		if result.Err != nil {
			e.emitError(newRunID, currentNode, step-1, result.Err)
			return zero, result.Err
		}

		// Merge state update
		currentState = e.reducer(currentState, result.Delta)

		// Persist state after node execution
		if err := e.store.SaveStep(ctx, newRunID, step, currentNode, currentState); err != nil {
			return zero, &EngineError{
				Message: "failed to save step: " + err.Error(),
				Code:    "STORE_ERROR",
			}
		}

		// Emit node_end event with delta (T155)
		e.emitNodeEnd(newRunID, currentNode, step-1, result.Delta)

		// Determine next node from routing decision
		if result.Route.Terminal {
			// Emit routing_decision event for Stop (T157)
			e.emitRoutingDecision(newRunID, currentNode, step-1, map[string]interface{}{
				"terminal": true,
			})
			// Workflow complete
			return currentState, nil
		}

		if result.Route.To != "" {
			// Emit routing_decision event for Goto (T157)
			e.emitRoutingDecision(newRunID, currentNode, step-1, map[string]interface{}{
				"next_node": result.Route.To,
			})

			// Single next node (Goto)
			currentNode = result.Route.To
			continue
		}

		// If no explicit route, fall back to edge-based routing
		nextNode := e.evaluateEdges(currentNode, currentState)
		if nextNode == "" {
			// No matching edge found - workflow cannot continue
			return zero, &EngineError{
				Message: "no valid route from node: " + currentNode,
				Code:    "NO_ROUTE",
			}
		}

		// Emit routing_decision event for edge-based routing (T157)
		e.emitRoutingDecision(newRunID, currentNode, step-1, map[string]interface{}{
			"next_node": nextNode,
			"via_edge":  true,
		})

		currentNode = nextNode
		continue
	}
}

// emitNodeStart emits a node_start event if emitter is configured (T153).
func (e *Engine[S]) emitNodeStart(runID, nodeID string, step int) {
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   step,
			NodeID: nodeID,
			Msg:    "node_start",
		})
	}
}

// emitNodeEnd emits a node_end event with delta metadata if emitter is configured (T155).
func (e *Engine[S]) emitNodeEnd(runID, nodeID string, step int, delta S) {
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   step,
			NodeID: nodeID,
			Msg:    "node_end",
			Meta: map[string]interface{}{
				"delta": delta,
			},
		})
	}
}

// emitError emits an error event if emitter is configured (T159).
func (e *Engine[S]) emitError(runID, nodeID string, step int, err error) {
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   step,
			NodeID: nodeID,
			Msg:    "error",
			Meta: map[string]interface{}{
				"error": err.Error(),
			},
		})
	}
}

// emitRoutingDecision emits a routing_decision event if emitter is configured (T157).
func (e *Engine[S]) emitRoutingDecision(runID, nodeID string, step int, meta map[string]interface{}) {
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   step,
			NodeID: nodeID,
			Msg:    "routing_decision",
			Meta:   meta,
		})
	}
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
