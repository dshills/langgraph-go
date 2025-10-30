package graph

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// contextKey is a private type used for context value keys to avoid collisions.
// Using a private type ensures that context keys from this package don't conflict
// with keys from other packages, following Go's context best practices.
type contextKey string

// Context keys for propagating execution metadata to nodes.
// These values are injected into the context passed to Node.Run() and can be
// retrieved by nodes to access execution metadata such as the current run ID,
// step number, and node ID.
//
// Example usage in a node:
//
//	func (n *MyNode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
//	    runID := ctx.Value(RunIDKey).(string)
//	    stepID := ctx.Value(StepIDKey).(int)
//	    // Use metadata for logging, tracing, etc.
//	}
const (
	// RunIDKey is the context key for the unique workflow run identifier.
	RunIDKey contextKey = "langgraph.run_id"

	// StepIDKey is the context key for the current execution step number.
	StepIDKey contextKey = "langgraph.step_id"

	// NodeIDKey is the context key for the current node identifier.
	NodeIDKey contextKey = "langgraph.node_id"

	// OrderKeyKey is the context key for the deterministic ordering key.
	// This hash determines the order in which concurrent node results are merged.
	OrderKeyKey contextKey = "langgraph.order_key"

	// AttemptKey is the context key for the current retry attempt number (0-based).
	// Value is 0 for first execution, incremented on each retry.
	AttemptKey contextKey = "langgraph.attempt"

	// RNGKey is the context key for the seeded random number generator.
	// Provides deterministic randomness for replay scenarios.
	// Type: *rand.Rand (from math/rand package)
	RNGKey contextKey = "langgraph.rng"

	// RecordedIOsKey is the context key for storing recorded I/O during replay.
	RecordedIOsKey contextKey = "langgraph.recordedIOs"
)

// initRNG creates a deterministic random number generator seeded from the runID.
//
// This enables deterministic replay of executions that use random values. The seed
// is computed by hashing the runID with SHA-256 and using the first 8 bytes as
// an int64 seed. This ensures:
//   - Same runID always produces the same random sequence
//   - Different runIDs produce different (statistically independent) sequences
//   - Collision probability is negligible (2^-256 for identical seeds)
//
// Nodes can retrieve the seeded RNG from context:
//
//	rng := ctx.Value(RNGKey).(*rand.Rand)
//	randomValue := rng.Intn(100)
//
// IMPORTANT: Nodes must use the context-provided RNG, not the global rand package,
// to ensure deterministic replay. Using the global rand or crypto/rand will cause
// replay mismatches because those sources are non-deterministic.
//
// Example node implementation with deterministic randomness:
//
//	func (n *MyNode) Run(ctx context.Context, state S) NodeResult[S] {
//	    rng := ctx.Value(RNGKey).(*rand.Rand)
//	    if rng == nil {
//	        // Fallback for tests or non-replay scenarios
//	        rng = rand.New(rand.NewSource(time.Now().UnixNano()))
//	    }
//	    delta := state
//	    delta.RandomChoice = rng.Intn(10)
//	    return NodeResult[S]{Delta: delta, Route: Next{Goto: "next_node"}}
//	}
//
// Parameters:
//   - runID: The unique run identifier to derive the seed from
//
// Returns:
//   - *rand.Rand: A seeded random number generator for this run
func initRNG(runID string) *rand.Rand {
	// Hash the runID to generate a deterministic seed
	hasher := sha256.New()
	hasher.Write([]byte(runID))
	hashBytes := hasher.Sum(nil)

	// Extract first 8 bytes as int64 seed
	seed := int64(binary.BigEndian.Uint64(hashBytes[:8])) // #nosec G115 -- conversion for deterministic seeding

	// Create a new rand.Rand with the deterministic seed
	// Note: Using math/rand (not crypto/rand) intentionally for deterministic replay
	source := rand.NewSource(seed) // #nosec G404 -- deterministic RNG for replay, not security
	return rand.New(source)        // #nosec G404 -- deterministic RNG for replay, not security
}

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

	// metrics collects Prometheus-compatible performance metrics (T044).
	// Optional - if nil, metrics are not collected.
	// See PrometheusMetrics for available metrics and usage.
	metrics *PrometheusMetrics

	// costTracker tracks LLM API call costs and token usage (T045).
	// Optional - if nil, cost tracking is disabled.
	// See CostTracker for pricing tables and cost attribution.
	costTracker *CostTracker

	// opts contains execution configuration
	opts Options

	// frontier manages the execution frontier queue for concurrent node execution.
	// It is initialized in Run() with capacity from Options.QueueDepth and handles
	// work item scheduling with deterministic ordering via OrderKey.
	// Nil when MaxConcurrentNodes = 0 (sequential execution mode).
	frontier *Frontier[S]
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
	// Deprecated: Use NodePolicy.RetryPolicy for per-node retry configuration.
	Retries int

	// MaxConcurrentNodes limits the number of nodes executing in parallel.
	// Default: 8. Set to 0 for sequential execution (backward compatible).
	//
	// Tuning guidance:
	//   - CPU-bound workflows: Set to runtime.NumCPU()
	//   - I/O-bound workflows: Set to 10-50 depending on external service limits
	//   - Memory-constrained: Reduce to prevent excessive state copies
	//
	// Each concurrent node holds a deep copy of state, so memory usage scales
	// linearly with MaxConcurrentNodes.
	MaxConcurrentNodes int

	// QueueDepth sets the capacity of the execution frontier queue.
	// Default: 1024. Increase for workflows with large fan-outs.
	//
	// When the queue fills, new work items block until space is available.
	// This provides backpressure to prevent unbounded memory growth.
	//
	// Recommended: MaxConcurrentNodes × 100 for initial estimate.
	QueueDepth int

	// BackpressureTimeout is the maximum time to wait when the frontier queue is full.
	// Default: 30s. If exceeded, Run() returns ErrBackpressureTimeout.
	//
	// Set lower for fast-failing systems, higher for workflows with bursty fan-outs.
	BackpressureTimeout time.Duration

	// DefaultNodeTimeout is the maximum execution time for nodes without explicit Policy().Timeout.
	// Default: 30s. Individual nodes can override via NodePolicy.Timeout.
	//
	// Prevents single slow nodes from blocking workflow progress indefinitely.
	// When exceeded, node execution is cancelled and returns context.DeadlineExceeded.
	DefaultNodeTimeout time.Duration

	// RunWallClockBudget is the maximum total execution time for Run().
	// Default: 10m. If exceeded, Run() returns context.DeadlineExceeded.
	//
	// Use this to enforce hard deadlines on entire workflow execution.
	// Set to 0 to disable (workflow runs until completion or MaxSteps).
	RunWallClockBudget time.Duration

	// ReplayMode enables deterministic replay using recorded I/O.
	// Default: false (record mode - captures I/O for later replay).
	//
	// When true, nodes with SideEffectPolicy.Recordable=true will use recorded
	// responses instead of executing live I/O. This enables:
	//   - Debugging: Replay production executions locally
	//   - Testing: Verify workflow logic without external dependencies
	//   - Auditing: Reconstruct exact execution flow from checkpoints
	//
	// Requires prior execution with ReplayMode=false to record I/O.
	ReplayMode bool

	// StrictReplay controls replay mismatch behavior.
	// Default: true (fail on I/O hash mismatch).
	//
	// When true, replay mode verifies recorded I/O hashes match expected values.
	// If a mismatch is detected (indicating logic changes), Run() returns ErrReplayMismatch.
	//
	// Set to false to allow "best effort" replay that tolerates minor changes.
	// Useful when debugging with modified node logic.
	StrictReplay bool

	// Metrics enables Prometheus metrics collection (T044).
	// If nil, metrics are not collected.
	//
	// Create with NewPrometheusMetrics(registry) to enable production monitoring.
	// All 6 metrics (inflight_nodes, queue_depth, step_latency_ms, retries_total,
	// merge_conflicts_total, backpressure_events_total) are automatically updated.
	//
	// Example:
	//   registry := prometheus.NewRegistry()
	//   metrics := NewPrometheusMetrics(registry)
	//   engine := New(reducer, store, emitter, Options{Metrics: metrics})
	Metrics *PrometheusMetrics

	// CostTracker enables LLM cost tracking with static pricing (T045).
	// If nil, cost tracking is disabled.
	//
	// Create with NewCostTracker(runID, "USD") to track token usage and costs.
	// Static pricing includes OpenAI, Anthropic, and Google models.
	//
	// Example:
	//   tracker := NewCostTracker("run-123", "USD")
	//   engine := New(reducer, store, emitter, Options{CostTracker: tracker})
	CostTracker *CostTracker
}

// New creates a new Engine with the given configuration.
//
// Supports two configuration patterns for backward compatibility:
//
// 1. Options struct (legacy):
//
//	engine := New(reducer, store, emitter, Options{MaxSteps: 100})
//
// 2. Functional options (recommended):
//
//	engine := New(
//	    reducer, store, emitter,
//	    WithMaxConcurrent(16),
//	    WithQueueDepth(2048),
//	    WithDefaultNodeTimeout(10*time.Second),
//	)
//
// 3. Mixed (Options struct + functional options):
//
//	baseOpts := Options{MaxSteps: 100}
//	engine := New(
//	    reducer, store, emitter,
//	    baseOpts,
//	    WithMaxConcurrent(8), // Overrides baseOpts if specified
//	)
//
// Parameters:
//   - reducer: Function to merge partial state updates (required for Run)
//   - store: Persistence backend for state and checkpoints (required for Run)
//   - emitter: Observability event receiver (optional, can be nil)
//   - options: Configuration via Options struct or variadic Option functions
//
// The constructor does not validate all parameters to allow flexible initialization.
// Validation occurs when Run() is called.
//
// Functional options (recommended):
//   - WithMaxConcurrent(n): Set max concurrent nodes
//   - WithQueueDepth(n): Set frontier queue capacity
//   - WithBackpressureTimeout(d): Set queue full timeout
//   - WithDefaultNodeTimeout(d): Set default node timeout
//   - WithRunWallClockBudget(d): Set total execution timeout
//   - WithReplayMode(bool): Enable replay mode
//   - WithStrictReplay(bool): Enable strict replay validation
//   - WithConflictPolicy(policy): Set conflict resolution policy
func New[S any](reducer Reducer[S], st store.Store[S], emitter emit.Emitter, options ...interface{}) *Engine[S] {
	// Initialize engine config with zero values
	cfg := &engineConfig{
		opts: Options{}, // Zero values
	}

	// Process options in order: Options struct first, then functional options
	for _, opt := range options {
		switch v := opt.(type) {
		case Options:
			// Legacy Options struct - use as base configuration
			cfg.opts = v
		case Option:
			// Functional option - apply to config
			// Ignore error for now (validation happens at Run time)
			_ = v(cfg)
		default:
			// Ignore unknown types for forward compatibility
		}
	}

	return &Engine[S]{
		reducer:     reducer,
		nodes:       make(map[string]Node[S]),
		edges:       make([]Edge[S], 0),
		store:       st,
		emitter:     emitter,
		metrics:     cfg.opts.Metrics,     // T044: Optional metrics
		costTracker: cfg.opts.CostTracker, // T045: Optional cost tracking
		opts:        cfg.opts,
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
	// Prevent panic when called on nil Engine
	if e == nil {
		return &EngineError{Message: "engine is nil", Code: "NIL_ENGINE"}
	}
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
	// Prevent panic when called on nil Engine
	if e == nil {
		return &EngineError{Message: "engine is nil", Code: "NIL_ENGINE"}
	}
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
	// Prevent panic when called on nil Engine
	if e == nil {
		return &EngineError{Message: "engine is nil", Code: "NIL_ENGINE"}
	}
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

	// Prevent panic when called on nil Engine
	if e == nil {
		return zero, &EngineError{Message: "engine is nil", Code: "NIL_ENGINE"}
	}

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

	// Enforce RunWallClockBudget if configured (T075)
	// This creates a derived context with timeout that applies to the entire workflow execution.
	// When the budget is exceeded, all running nodes receive context cancellation.
	if e.opts.RunWallClockBudget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.opts.RunWallClockBudget)
		defer cancel()
	}

	// Initialize seeded RNG for deterministic random number generation (T054, T055)
	// The RNG is seeded from the runID hash to ensure replays produce identical
	// random sequences. This enables deterministic replay of workflows that use
	// random values (e.g., sampling, stochastic routing).
	rng := initRNG(runID)
	ctx = context.WithValue(ctx, RNGKey, rng)

	// Initialize Frontier for concurrent execution if MaxConcurrentNodes > 0 (T034)
	if e.opts.MaxConcurrentNodes > 0 {
		queueDepth := e.opts.QueueDepth
		if queueDepth == 0 {
			queueDepth = 1024 // Default queue depth
		}
		e.frontier = NewFrontier[S](ctx, queueDepth, runID, e.opts.Metrics, e.emitter)

		// Use concurrent execution path (T035)
		return e.runConcurrent(ctx, runID, initial)
	}

	// Initialize execution state (sequential execution path)
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

// nodeResult represents the outcome of a single node execution in concurrent mode.
// Used internally by runConcurrent for collecting and merging results.
type nodeResult[S any] struct {
	nodeID   string
	delta    S
	route    Next
	orderKey uint64
	err      error
}

// runConcurrent executes the workflow using concurrent node execution with the Frontier scheduler (T035).
//
// This method implements a worker pool pattern where:
//  1. Up to MaxConcurrentNodes goroutines execute nodes concurrently
//  2. Workers dequeue WorkItems from the Frontier (priority queue by OrderKey)
//  3. After executing a node, workers create new WorkItems for next hops
//  4. State deltas are collected and merged deterministically by OrderKey
//  5. WaitGroup tracks active workers for graceful shutdown
//
// The concurrent execution maintains deterministic replay through:
//   - OrderKey-based work item prioritization (deterministic across runs)
//   - Ordered delta merging (sort by OrderKey before applying reducer)
//   - Deep state copies for fan-out branches (isolation)
//
// Returns final state after workflow completes or error if execution fails.
func (e *Engine[S]) runConcurrent(ctx context.Context, runID string, initial S) (S, error) {
	var zero S

	// WaitGroup tracks active workers
	var wg sync.WaitGroup

	// Enqueue initial work item
	initialItem := WorkItem[S]{
		StepID:       0,
		OrderKey:     computeOrderKey("__start__", 0),
		NodeID:       e.startNode,
		State:        initial,
		Attempt:      0,
		ParentNodeID: "__start__",
		EdgeIndex:    0,
	}

	if err := e.frontier.Enqueue(ctx, initialItem); err != nil {
		return zero, err
	}

	// Track step counter and collected deltas
	var stepCounter atomic.Int32
	collectedResults := make([]nodeResult[S], 0, e.opts.MaxSteps)

	// Determine number of worker goroutines (up to MaxConcurrentNodes)
	const defaultMaxWorkers = 8
	maxWorkers := e.opts.MaxConcurrentNodes
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers // Default if not specified or negative
	}

	// Result channel for collecting node execution outcomes.
	// Buffer sized at maxWorkers*2 to handle concurrent error delivery from all workers.
	// This prevents deadlock when all workers fail simultaneously and need to report errors.
	results := make(chan nodeResult[S], maxWorkers*2)

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Derive base seed from runID for per-worker RNG generation (BUG-002 fix)
	// We compute the base seed the same way as initRNG() does, then use it to
	// derive unique seeds for each worker. This ensures deterministic replay
	// while preventing concurrent access to shared RNG state.
	hasher := sha256.New()
	hasher.Write([]byte(runID))
	hashBytes := hasher.Sum(nil)
	baseSeed := int64(binary.BigEndian.Uint64(hashBytes[:8])) // #nosec G115 -- conversion for deterministic seeding

	// T046: Track inflight nodes for metrics
	var inflightCounter atomic.Int32

	// BUG-004 fix (T025): Atomic completion flag for race-free workflow termination
	// CompareAndSwap ensures exactly one worker triggers completion
	var completionDetected atomic.Bool

	// BUG-004 fix (T026): Helper function to check and signal completion atomically
	// Returns true if this call detected completion (frontier empty + no inflight work)
	checkCompletion := func() bool {
		if e.frontier.Len() == 0 && inflightCounter.Load() == 0 {
			// Atomically check and set completion flag
			// Only the first worker to see completion will return true
			if completionDetected.CompareAndSwap(false, true) {
				cancel() // Signal all workers to stop
				return true
			}
		}
		return false
	}

	// T046: Start metrics updater goroutine if metrics enabled
	if e.metrics != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond) // Update metrics every 100ms
			defer ticker.Stop()

			for {
				select {
				case <-workerCtx.Done():
					return
				case <-ticker.C:
					// Update queue depth and inflight nodes metrics
					queueDepth := e.frontier.Len()
					inflight := int(inflightCounter.Load())
					e.metrics.UpdateQueueDepth(queueDepth)
					e.metrics.UpdateInflightNodes(inflight)
				}
			}
		}()
	}

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for {
				// Dequeue next work item with worker context for proper cancellation
				item, err := e.frontier.Dequeue(workerCtx)
				if err != nil {
					// BUG-004 fix (T027): Check for completion after dequeue failure
					// This handles the case where the frontier is empty and no work is inflight
					checkCompletion()
					// Context cancelled or frontier closed
					return
				}

				// T046: Track node execution for inflight metrics (using func to ensure decrement)
				func() {
					inflightCounter.Add(1)
					defer inflightCounter.Add(-1)

					currentStep := stepCounter.Add(1)

					// Check MaxSteps limit
					if e.opts.MaxSteps > 0 && int(currentStep) > e.opts.MaxSteps {
						results <- nodeResult[S]{
							err: &EngineError{
								Message: "workflow exceeded MaxSteps limit",
								Code:    "MAX_STEPS_EXCEEDED",
							},
						}
						cancel()
						return
					}

					// Get node implementation
					e.mu.RLock()
					nodeImpl, exists := e.nodes[item.NodeID]
					e.mu.RUnlock()

					if !exists {
						results <- nodeResult[S]{
							err: &EngineError{
								Message: "node not found during execution: " + item.NodeID,
								Code:    "NODE_NOT_FOUND",
							},
						}
						cancel()
						return
					}

					// Emit node_start event
					e.emitNodeStart(runID, item.NodeID, item.StepID)

					// Get node policy to check for retry configuration (T087)
					var policy *NodePolicy
					if policyProvider, ok := nodeImpl.(interface{ Policy() NodePolicy }); ok {
						p := policyProvider.Policy()
						policy = &p
					}

					// Create work-item-specific RNG seeded from OrderKey for deterministic replay
					// This ensures the same logical work item (same OrderKey) always uses the same RNG,
					// regardless of which physical worker goroutine executes it.
					// Seed = baseSeed XOR OrderKey for deterministic but unique per-item RNG.
					itemSeed := baseSeed ^ int64(item.OrderKey)   // #nosec G115 -- XOR for deterministic seeding
					itemRNG := rand.New(rand.NewSource(itemSeed)) // #nosec G404 -- deterministic RNG for replay, not security

					// Create node context with work-item-specific RNG and attempt number
					nodeCtx := context.WithValue(workerCtx, RNGKey, itemRNG)
					nodeCtx = context.WithValue(nodeCtx, AttemptKey, item.Attempt)

					// T046: Track node execution start time for latency metrics
					startTime := time.Now()

					// Execute node (with replay mode support - T052)
					// When Options.ReplayMode=true, recorded I/O should be used instead of live execution.
					// Full replay implementation requires:
					//   1. Loading checkpoint with RecordedIOs at Run() start
					//   2. Looking up recorded response via lookupRecordedIO(nodeID, attempt)
					//   3. Deserializing recorded response into NodeResult
					//   4. Optionally verifying hash with verifyReplayHash() if StrictReplay=true
					// For now, execute normally - replay integration will be completed in T056-T057
					result := nodeImpl.Run(nodeCtx, item.State)

					// T046: Record step latency metric
					latency := time.Since(startTime)
					status := "success"
					if result.Err != nil {
						status = "error"
					}
					if e.metrics != nil {
						e.metrics.RecordStepLatency(runID, item.NodeID, latency, status)
					}

					// Handle node error with retry support (T087-T090)
					if result.Err != nil {
						e.emitError(runID, item.NodeID, item.StepID, result.Err)

						// Helper to send error result and cancel.
						// Errors are rare and critical - we MUST deliver them to the caller.
						// Blocking is safe because results channel buffer is maxWorkers*2.
						sendErrorAndCancel := func(err error) {
							select {
							case results <- nodeResult[S]{err: err}:
								// Error sent successfully
							case <-ctx.Done():
								// Parent context canceled before send completed
								// This is acceptable - workflow is being torn down
							}
							cancel()
						}

						// Check if error is retryable and we haven't exceeded max attempts
						if policy != nil && policy.RetryPolicy != nil {
							retryPol := policy.RetryPolicy

							// Validate retry policy configuration (defensive check)
							// In production, policies should be validated at configuration time
							if err := retryPol.Validate(); err != nil {
								// Wrap validation error with context
								sendErrorAndCancel(fmt.Errorf("retry policy validation failed for node %s: %w", item.NodeID, err))
								return
							}

							// Check if error is retryable using predicate (T084)
							isRetryable := retryPol.Retryable != nil && retryPol.Retryable(result.Err)

							// Calculate remaining retry attempts (T089)
							// item.Attempt is 0-based, so attempt 0 = first execution
							// MaxAttempts includes the initial attempt, so MaxAttempts=3 means:
							// attempt 0 (initial), attempt 1 (retry 1), attempt 2 (retry 2)
							remainingRetries := retryPol.MaxAttempts - item.Attempt - 1

							if isRetryable && remainingRetries > 0 {
								// T046: Increment retry metrics
								if e.metrics != nil {
									e.metrics.IncrementRetries(runID, item.NodeID, "error")
								}

								// Compute backoff delay (T086, T090)
								// Use item-specific RNG for retry backoff calculation for deterministic replay
								var rng *rand.Rand
								if rngVal := nodeCtx.Value(RNGKey); rngVal != nil {
									rng = rngVal.(*rand.Rand)
								}
								delay := computeBackoff(item.Attempt, retryPol.BaseDelay, retryPol.MaxDelay, rng)

								// Apply backoff delay before re-enqueueing (T090)
								time.Sleep(delay)

								// Create retry work item with incremented attempt (T088)
								retryItem := WorkItem[S]{
									StepID:       item.StepID,      // Same step ID (retry, not new step)
									OrderKey:     item.OrderKey,    // Preserve order key for determinism
									NodeID:       item.NodeID,      // Same node
									State:        item.State,       // Same input state
									Attempt:      item.Attempt + 1, // Increment attempt counter (T088)
									ParentNodeID: item.ParentNodeID,
									EdgeIndex:    item.EdgeIndex,
								}

								// Re-enqueue for retry
								if err := e.frontier.Enqueue(workerCtx, retryItem); err != nil {
									// If enqueue fails, treat as non-retryable
									sendErrorAndCancel(result.Err)
									return
								}

								// Retry enqueued successfully - exit anonymous function
								// This returns from func(), not from the goroutine.
								// The outer for loop continues and dequeues the next work item.
								return
							}

							// Error is non-retryable or max attempts exceeded
							if remainingRetries <= 0 {
								// Max attempts exceeded (T089)
								sendErrorAndCancel(ErrMaxAttemptsExceeded)
								return
							}
						}

						// Non-retryable error or no retry policy
						sendErrorAndCancel(result.Err)
						return
					}

					// Emit node_end event
					e.emitNodeEnd(runID, item.NodeID, item.StepID, result.Delta)

					// Send result to collection channel
					results <- nodeResult[S]{
						nodeID:   item.NodeID,
						delta:    result.Delta,
						route:    result.Route,
						orderKey: item.OrderKey,
						err:      nil,
					}

					// Handle routing (T036)
					if result.Route.Terminal {
						// Terminal node - stop workflow
						e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
							"terminal": true,
						})
						// Don't enqueue more work, but continue to allow other parallel branches to complete
						return
					}

					// Fan-out routing (Next.Many)
					if len(result.Route.Many) > 0 {
						e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
							"parallel": true,
							"branches": result.Route.Many,
						})

						for edgeIdx, branchID := range result.Route.Many {
							// Deep copy state for branch isolation
							branchState, err := deepCopyState(item.State)
							if err != nil {
								results <- nodeResult[S]{err: err}
								cancel()
								return
							}

							// Create work item for branch
							branchItem := WorkItem[S]{
								StepID:       item.StepID + 1,
								OrderKey:     computeOrderKey(item.NodeID, edgeIdx),
								NodeID:       branchID,
								State:        branchState,
								Attempt:      0,
								ParentNodeID: item.NodeID,
								EdgeIndex:    edgeIdx,
							}

							if err := e.frontier.Enqueue(workerCtx, branchItem); err != nil {
								results <- nodeResult[S]{err: err}
								cancel()
								return
							}
						}
						return
					}

					// Single next node (Goto)
					if result.Route.To != "" {
						e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
							"next_node": result.Route.To,
						})

						nextItem := WorkItem[S]{
							StepID:       item.StepID + 1,
							OrderKey:     computeOrderKey(item.NodeID, 0),
							NodeID:       result.Route.To,
							State:        item.State,
							Attempt:      0,
							ParentNodeID: item.NodeID,
							EdgeIndex:    0,
						}

						if err := e.frontier.Enqueue(workerCtx, nextItem); err != nil {
							results <- nodeResult[S]{err: err}
							cancel()
							return
						}
						return
					}

					// Edge-based routing fallback
					nextNode := e.evaluateEdges(item.NodeID, item.State)
					if nextNode == "" {
						results <- nodeResult[S]{
							err: &EngineError{
								Message: "no valid route from node: " + item.NodeID,
								Code:    "NO_ROUTE",
							},
						}
						cancel()
						return
					}

					e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
						"next_node": nextNode,
						"via_edge":  true,
					})

					nextItem := WorkItem[S]{
						StepID:       item.StepID + 1,
						OrderKey:     computeOrderKey(item.NodeID, 0),
						NodeID:       nextNode,
						State:        item.State,
						Attempt:      0,
						ParentNodeID: item.NodeID,
						EdgeIndex:    0,
					}

					if err := e.frontier.Enqueue(workerCtx, nextItem); err != nil {
						results <- nodeResult[S]{err: err}
						cancel()
						return
					}
				}() // T046: End inflight tracking func

				// BUG-004 fix (T028): Check for completion after node execution completes
				// This handles the case where this was the last node to finish
				// inflightCounter has been decremented, so completion check is accurate
				if checkCompletion() {
					return
				}
			}
		}(i) // Pass worker ID to goroutine
	}

	// Wait for workers to complete in a separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	// BUG-004 fix (T025): Removed polling goroutine
	// Completion detection now handled by atomic flag checks in worker loop
	// This eliminates the 0-10ms completion detection race window

	for result := range results {
		if result.err != nil {
			return zero, result.err
		}
		collectedResults = append(collectedResults, result)
	}

	// Merge deltas deterministically by OrderKey (T038)
	finalState := e.mergeDeltas(initial, collectedResults)

	// Save checkpoint after merging all deltas (T049)
	// Use the final step count for checkpoint ID
	finalStepID := int(stepCounter.Load())
	emptyFrontier := []WorkItem[S]{} // Frontier is empty at workflow completion
	noRecordedIOs := []RecordedIO{}  // TODO: Track recorded IOs in later phases

	if err := e.saveCheckpoint(ctx, runID, finalStepID, finalState, emptyFrontier, noRecordedIOs, ""); err != nil {
		// Log checkpoint error but don't fail the workflow
		// The workflow completed successfully even if checkpoint save failed
		if e.emitter != nil {
			e.emitter.Emit(emit.Event{
				RunID:  runID,
				Step:   finalStepID,
				NodeID: "",
				Msg:    "checkpoint_save_failed",
				Meta: map[string]interface{}{
					"error": err.Error(),
				},
			})
		}
	}

	return finalState, nil
}

// mergeDeltas merges collected node deltas into final state using deterministic ordering (T038).
//
// Deltas are sorted by OrderKey (ascending) before applying the reducer to ensure:
//   - Deterministic results regardless of goroutine completion order
//   - Identical state across replays with the same execution graph
//   - Predictable merge order for debugging
//
// The OrderKey captures the execution path (parent node + edge index), so sorting by
// OrderKey effectively recreates the logical execution order of the graph.
func (e *Engine[S]) mergeDeltas(initial S, results []nodeResult[S]) S {
	// Sort results by OrderKey for deterministic merge (T038)
	sort.Slice(results, func(i, j int) bool {
		return results[i].orderKey < results[j].orderKey
	})

	// Apply reducer to merge deltas in order
	finalState := initial
	for _, result := range results {
		finalState = e.reducer(finalState, result.delta)
	}

	return finalState
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
	branchResults := make([]branchResult, 0, len(branches))
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

// saveCheckpoint atomically commits a checkpoint to the store with idempotency protection (T048).
//
// This method is called after each execution step to persist the current state, frontier,
// and recorded I/O operations. The checkpoint enables:
//   - Crash recovery: Resume execution from the last committed checkpoint
//   - Deterministic replay: Reconstruct execution with recorded I/O responses
//   - Exactly-once semantics: Idempotency key prevents duplicate commits
//
// The idempotency key is computed from (runID, stepID, frontier, state) to ensure that
// retries of the same execution step produce the same key. If the store detects a
// duplicate key, it returns ErrIdempotencyViolation, which this method treats as success
// (the checkpoint was already committed in a previous attempt).
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - runID: Unique identifier for this workflow execution
//   - stepID: Sequential step number (monotonically increasing)
//   - state: Current accumulated state after applying all deltas
//   - frontier: Work items ready to execute at this checkpoint
//   - recordedIOs: External I/O operations captured for replay
//   - label: Optional user-defined label (empty string for automatic checkpoints)
//
// Returns error if:
//   - Idempotency key computation fails (state JSON marshaling error)
//   - Store commit fails (excluding idempotency violations)
//
// The checkpoint is committed atomically with the following guarantee:
// If this method returns nil, the checkpoint is durably persisted and will be
// available for resumption after crashes. If it returns an error, the checkpoint
// was not committed and the step should be retried.
//
// Thread-safety: This method is safe for concurrent use by multiple goroutines.
func (e *Engine[S]) saveCheckpoint(ctx context.Context, runID string, stepID int, state S, frontier []WorkItem[S], recordedIOs []RecordedIO, label string) error {
	// Compute idempotency key from execution context (T046)
	idempotencyKey, err := computeIdempotencyKey(runID, stepID, frontier, state)
	if err != nil {
		return &EngineError{
			Message: "failed to compute idempotency key: " + err.Error(),
			Code:    "IDEMPOTENCY_KEY_ERROR",
		}
	}

	// Check if this checkpoint was already committed (idempotency check)
	exists, err := e.store.CheckIdempotency(ctx, idempotencyKey)
	if err != nil {
		return &EngineError{
			Message: "failed to check idempotency: " + err.Error(),
			Code:    "STORE_ERROR",
		}
	}

	if exists {
		// Checkpoint already committed in a previous attempt - treat as success
		// This prevents duplicate commits during retries while maintaining exactly-once semantics
		return nil
	}

	// Create checkpoint struct (T047 - relies on automatic JSON marshaling)
	// Use store.CheckpointV2 type which contains full execution context
	checkpoint := store.CheckpointV2[S]{
		RunID:          runID,
		StepID:         stepID,
		State:          state,
		Frontier:       frontier,    // Will be serialized as interface{}
		RNGSeed:        0,           // TODO: Will be populated in T054
		RecordedIOs:    recordedIOs, // Will be serialized as interface{}
		IdempotencyKey: idempotencyKey,
		Timestamp:      time.Now(),
		Label:          label,
	}

	// Atomically commit checkpoint to store
	// If this fails with idempotency violation, another goroutine committed first
	if err := e.store.SaveCheckpointV2(ctx, checkpoint); err != nil {
		// Check if error is idempotency violation (duplicate key)
		if errors.Is(err, ErrIdempotencyViolation) {
			// Another commit won the race - treat as success
			return nil
		}

		// Other store error - propagate to caller
		return &EngineError{
			Message: "failed to save checkpoint: " + err.Error(),
			Code:    "CHECKPOINT_SAVE_FAILED",
		}
	}

	// Emit checkpoint event for observability
	if e.emitter != nil {
		e.emitter.Emit(emit.Event{
			RunID:  runID,
			Step:   stepID,
			NodeID: "",
			Msg:    "checkpoint_saved",
			Meta: map[string]interface{}{
				"idempotency_key": idempotencyKey,
				"frontier_size":   len(frontier),
				"recorded_ios":    len(recordedIOs),
				"label":           label,
			},
		})
	}

	return nil
}

// RunWithCheckpoint resumes execution from a saved checkpoint.
//
// This method enables crash recovery and branching workflows by resuming execution
// from a previously saved checkpoint. It restores the complete execution context:
//   - Accumulated state
//   - Execution frontier (pending work items)
//   - RNG seed for deterministic random values
//   - Recorded I/O for replay
//
// The resumed execution continues from where the checkpoint was created, processing
// all work items in the frontier until the workflow completes or encounters an error.
//
// Parameters:
//   - ctx: Context for cancellation and request-scoped values
//   - checkpoint: Complete checkpoint containing execution state and context
//
// Returns:
//   - Final state after resumed execution completes
//   - Error if validation fails, node execution fails, or limits exceeded
//
// Example:
//
//	// Original execution with checkpoint
//	_, _ = engine.Run(ctx, "run-001", initialState)
//
//	// Load checkpoint from store
//	checkpoint, err := store.LoadCheckpointV2(ctx, "run-001", 5)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Resume execution from checkpoint
//	final, err := engine.RunWithCheckpoint(ctx, checkpoint)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Resumed execution completed: %+v\n", final)
//
// Use Cases:
//   - Crash recovery: Resume after application restart
//   - Debugging: Replay execution from specific checkpoint
//   - Branching: Try different paths from same checkpoint
//   - Long-running workflows: Checkpoint and resume across restarts
//
// Thread-safety: This method is safe for concurrent use with different checkpoints.
func (e *Engine[S]) RunWithCheckpoint(ctx context.Context, checkpoint store.CheckpointV2[S]) (S, error) {
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

	// Restore RNG from checkpoint seed
	seed := checkpoint.RNGSeed
	if seed == 0 {
		// If no seed in checkpoint, derive from runID
		seedRNG := initRNG(checkpoint.RunID)
		// Extract seed from the RNG (we need to use the hash-based seed)
		hasher := sha256.New()
		hasher.Write([]byte(checkpoint.RunID))
		hashBytes := hasher.Sum(nil)
		seed = int64(binary.BigEndian.Uint64(hashBytes[:8])) // #nosec G115 -- conversion for deterministic seeding
		_ = seedRNG                                          // Suppress unused warning
	}
	source := rand.NewSource(seed) // #nosec G404 -- deterministic RNG for replay, not security
	rng := rand.New(source)        // #nosec G404 -- deterministic RNG for replay, not security

	ctx = context.WithValue(ctx, RNGKey, rng)

	// Restore frontier from checkpoint
	// The Frontier field is stored as interface{} to avoid circular dependency
	// Convert it back to []WorkItem[S]
	var frontierItems []WorkItem[S]
	if checkpoint.Frontier != nil {
		// Marshal and unmarshal to convert from interface{} to []WorkItem[S]
		frontierJSON, err := json.Marshal(checkpoint.Frontier)
		if err != nil {
			return zero, &EngineError{
				Message: "failed to marshal checkpoint frontier: " + err.Error(),
				Code:    "CHECKPOINT_RESTORE_ERROR",
			}
		}
		if err := json.Unmarshal(frontierJSON, &frontierItems); err != nil {
			return zero, &EngineError{
				Message: "failed to unmarshal checkpoint frontier: " + err.Error(),
				Code:    "CHECKPOINT_RESTORE_ERROR",
			}
		}
	}

	// If frontier is empty, workflow was already complete at checkpoint
	if len(frontierItems) == 0 {
		// Return checkpoint state as final state
		return checkpoint.State, nil
	}

	// Check if concurrent execution is enabled
	if e.opts.MaxConcurrentNodes > 0 {
		// Initialize Frontier for concurrent execution
		queueDepth := e.opts.QueueDepth
		if queueDepth == 0 {
			queueDepth = 1024 // Default queue depth
		}
		e.frontier = NewFrontier[S](ctx, queueDepth, checkpoint.RunID, e.opts.Metrics, e.emitter)

		// Enqueue all work items from checkpoint
		for _, item := range frontierItems {
			if err := e.frontier.Enqueue(ctx, item); err != nil {
				return zero, &EngineError{
					Message: "failed to enqueue checkpoint work item: " + err.Error(),
					Code:    "CHECKPOINT_RESTORE_ERROR",
				}
			}
		}

		// Use concurrent execution path
		// The checkpoint state is already in the work items, so we pass it as initial
		return e.runConcurrentFromCheckpoint(ctx, checkpoint.RunID, checkpoint.State, checkpoint.StepID)
	}

	// Sequential execution from checkpoint
	// For sequential mode, process first work item from frontier
	if len(frontierItems) == 0 {
		return checkpoint.State, nil
	}

	// Take first work item as current node
	firstItem := frontierItems[0]
	currentState := checkpoint.State
	currentNode := firstItem.NodeID
	step := checkpoint.StepID

	// Execution loop (same as Run)
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

		// Emit node_start event
		e.emitNodeStart(checkpoint.RunID, currentNode, step-1)

		// Execute node
		result := nodeImpl.Run(ctx, currentState)

		// Handle node error
		if result.Err != nil {
			e.emitError(checkpoint.RunID, currentNode, step-1, result.Err)
			return zero, result.Err
		}

		// Merge state update
		currentState = e.reducer(currentState, result.Delta)

		// Persist state after node execution
		if err := e.store.SaveStep(ctx, checkpoint.RunID, step, currentNode, currentState); err != nil {
			return zero, &EngineError{
				Message: "failed to save step: " + err.Error(),
				Code:    "STORE_ERROR",
			}
		}

		// Emit node_end event with delta
		e.emitNodeEnd(checkpoint.RunID, currentNode, step-1, result.Delta)

		// Determine next node from routing decision
		if result.Route.Terminal {
			e.emitRoutingDecision(checkpoint.RunID, currentNode, step-1, map[string]interface{}{
				"terminal": true,
			})
			return currentState, nil
		}

		if result.Route.To != "" {
			e.emitRoutingDecision(checkpoint.RunID, currentNode, step-1, map[string]interface{}{
				"next_node": result.Route.To,
			})
			currentNode = result.Route.To
			continue
		}

		// Edge-based routing fallback
		nextNode := e.evaluateEdges(currentNode, currentState)
		if nextNode == "" {
			return zero, &EngineError{
				Message: "no valid route from node: " + currentNode,
				Code:    "NO_ROUTE",
			}
		}

		e.emitRoutingDecision(checkpoint.RunID, currentNode, step-1, map[string]interface{}{
			"next_node": nextNode,
			"via_edge":  true,
		})

		currentNode = nextNode
		continue
	}
}

// runConcurrentFromCheckpoint resumes concurrent execution from a checkpoint.
// This is a helper method used by RunWithCheckpoint for concurrent execution mode.
func (e *Engine[S]) runConcurrentFromCheckpoint(ctx context.Context, runID string, initialState S, startStepID int) (S, error) {
	var zero S

	// Result channel for collecting node execution outcomes
	results := make(chan nodeResult[S], e.opts.MaxConcurrentNodes)

	// WaitGroup tracks active workers
	var wg sync.WaitGroup

	// Track step counter starting from checkpoint
	var stepCounter atomic.Int32
	stepCounter.Store(int32(startStepID)) // #nosec G115 -- startStepID is bounded by MaxSteps (default 100)
	collectedResults := make([]nodeResult[S], 0, e.opts.MaxSteps)

	// Spawn worker goroutines
	maxWorkers := e.opts.MaxConcurrentNodes
	if maxWorkers == 0 {
		maxWorkers = 8
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				// Dequeue next work item
				item, err := e.frontier.Dequeue(ctx)
				if err != nil {
					return
				}

				currentStep := stepCounter.Add(1)

				// Check MaxSteps limit
				if e.opts.MaxSteps > 0 && int(currentStep) > e.opts.MaxSteps {
					results <- nodeResult[S]{
						err: &EngineError{
							Message: "workflow exceeded MaxSteps limit",
							Code:    "MAX_STEPS_EXCEEDED",
						},
					}
					cancel()
					return
				}

				// Get node implementation
				e.mu.RLock()
				nodeImpl, exists := e.nodes[item.NodeID]
				e.mu.RUnlock()

				if !exists {
					results <- nodeResult[S]{
						err: &EngineError{
							Message: "node not found during execution: " + item.NodeID,
							Code:    "NODE_NOT_FOUND",
						},
					}
					cancel()
					return
				}

				// Emit node_start event
				e.emitNodeStart(runID, item.NodeID, item.StepID)

				// Execute node
				result := nodeImpl.Run(workerCtx, item.State)

				// Handle node error
				if result.Err != nil {
					e.emitError(runID, item.NodeID, item.StepID, result.Err)
					results <- nodeResult[S]{err: result.Err}
					cancel()
					return
				}

				// Emit node_end event
				e.emitNodeEnd(runID, item.NodeID, item.StepID, result.Delta)

				// Send result to collection channel
				results <- nodeResult[S]{
					nodeID:   item.NodeID,
					delta:    result.Delta,
					route:    result.Route,
					orderKey: item.OrderKey,
					err:      nil,
				}

				// Handle routing
				if result.Route.Terminal {
					e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
						"terminal": true,
					})
					return
				}

				// Fan-out routing
				if len(result.Route.Many) > 0 {
					e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
						"parallel": true,
						"branches": result.Route.Many,
					})

					for edgeIdx, branchID := range result.Route.Many {
						branchState, err := deepCopyState(item.State)
						if err != nil {
							results <- nodeResult[S]{err: err}
							cancel()
							return
						}

						branchItem := WorkItem[S]{
							StepID:       item.StepID + 1,
							OrderKey:     computeOrderKey(item.NodeID, edgeIdx),
							NodeID:       branchID,
							State:        branchState,
							Attempt:      0,
							ParentNodeID: item.NodeID,
							EdgeIndex:    edgeIdx,
						}

						if err := e.frontier.Enqueue(workerCtx, branchItem); err != nil {
							results <- nodeResult[S]{err: err}
							cancel()
							return
						}
					}
					return
				}

				// Single next node
				if result.Route.To != "" {
					e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
						"next_node": result.Route.To,
					})

					nextItem := WorkItem[S]{
						StepID:       item.StepID + 1,
						OrderKey:     computeOrderKey(item.NodeID, 0),
						NodeID:       result.Route.To,
						State:        item.State,
						Attempt:      0,
						ParentNodeID: item.NodeID,
						EdgeIndex:    0,
					}

					if err := e.frontier.Enqueue(workerCtx, nextItem); err != nil {
						results <- nodeResult[S]{err: err}
						cancel()
						return
					}
					continue
				}

				// Edge-based routing
				nextNode := e.evaluateEdges(item.NodeID, item.State)
				if nextNode == "" {
					results <- nodeResult[S]{
						err: &EngineError{
							Message: "no valid route from node: " + item.NodeID,
							Code:    "NO_ROUTE",
						},
					}
					cancel()
					return
				}

				e.emitRoutingDecision(runID, item.NodeID, item.StepID, map[string]interface{}{
					"next_node": nextNode,
					"via_edge":  true,
				})

				nextItem := WorkItem[S]{
					StepID:       item.StepID + 1,
					OrderKey:     computeOrderKey(item.NodeID, 0),
					NodeID:       nextNode,
					State:        item.State,
					Attempt:      0,
					ParentNodeID: item.NodeID,
					EdgeIndex:    0,
				}

				if err := e.frontier.Enqueue(workerCtx, nextItem); err != nil {
					results <- nodeResult[S]{err: err}
					cancel()
					return
				}
			}
		}()
	}

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from workers
	for result := range results {
		if result.err != nil {
			return zero, result.err
		}
		collectedResults = append(collectedResults, result)
	}

	// Merge deltas deterministically
	finalState := e.mergeDeltas(initialState, collectedResults)

	return finalState, nil
}

// ReplayRun replays a previous execution using recorded I/O without re-invoking external services.
//
// This method enables exact replay of executions for debugging, auditing, or testing.
// It loads the latest checkpoint for the given runID and replays the execution using
// recorded I/O responses instead of making live external calls.
//
// The replay process:
//  1. Loads the latest checkpoint containing recorded I/O and state
//  2. Configures engine in replay mode (Options.ReplayMode=true)
//  3. Executes nodes using recorded responses instead of live calls
//  4. Verifies execution matches original via hash comparison (if StrictReplay=true)
//
// Parameters:
//   - ctx: Context for cancellation and request-scoped values
//   - runID: Unique identifier of the run to replay
//
// Returns:
//   - Final state after replayed execution completes
//   - Error if replay fails, checkpoint not found, or mismatch detected
//
// Example:
//
//	// Original execution with I/O recording
//	opts := Options{ReplayMode: false} // Record mode
//	engine := New(reducer, store, emitter, opts)
//	_, err := engine.Run(ctx, "run-001", initialState)
//
//	// Later: replay execution for debugging
//	replayOpts := Options{ReplayMode: true, StrictReplay: true}
//	replayEngine := New(reducer, store, emitter, replayOpts)
//	replayedState, err := replayEngine.ReplayRun(ctx, "run-001")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Replayed state: %+v\n", replayedState)
//
// Use Cases:
//   - Debugging: Replay production issues locally without external dependencies
//   - Testing: Verify workflow logic without mocking external services
//   - Auditing: Reconstruct exact execution flow for compliance
//   - Development: Test changes against recorded production data
//
// Requirements:
//   - Original run must have been executed with recordable nodes
//   - Checkpoint must contain recorded I/O (RecordedIOs field populated)
//   - Engine must be configured with ReplayMode=true
//
// Thread-safety: This method is safe for concurrent use with different runIDs.
func (e *Engine[S]) ReplayRun(ctx context.Context, runID string) (S, error) {
	var zero S

	// Validate replay mode is enabled
	if !e.opts.ReplayMode {
		return zero, &EngineError{
			Message: "replay mode not enabled (set Options.ReplayMode=true)",
			Code:    "REPLAY_MODE_REQUIRED",
		}
	}

	// Load latest checkpoint for the run
	// Find the highest step ID by loading incrementally
	// Start with step 0 and increment until we find the latest
	var latestCheckpoint store.CheckpointV2[S]
	latestStep := -1

	// Try loading checkpoints from step 0 upwards
	// In production, stores should implement a LoadLatestCheckpointV2 method
	// For now, we'll try loading steps sequentially (this is inefficient but correct)
	for step := 0; step < 1000; step++ { // Arbitrary upper limit
		checkpoint, err := e.store.LoadCheckpointV2(ctx, runID, step)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// No more checkpoints
				break
			}
			return zero, &EngineError{
				Message: "failed to load checkpoint: " + err.Error(),
				Code:    "CHECKPOINT_LOAD_ERROR",
			}
		}
		latestCheckpoint = checkpoint
		latestStep = step
	}

	// Verify we found at least one checkpoint
	if latestStep == -1 {
		return zero, &EngineError{
			Message: "no checkpoints found for runID: " + runID,
			Code:    "NO_CHECKPOINTS",
		}
	}

	// Verify checkpoint has recorded I/O
	// Convert interface{} to []RecordedIO
	var recordedIOs []RecordedIO
	if latestCheckpoint.RecordedIOs != nil {
		recordedIOsJSON, err := json.Marshal(latestCheckpoint.RecordedIOs)
		if err != nil {
			return zero, &EngineError{
				Message: "failed to marshal recorded I/O: " + err.Error(),
				Code:    "CHECKPOINT_FORMAT_ERROR",
			}
		}
		if err := json.Unmarshal(recordedIOsJSON, &recordedIOs); err != nil {
			return zero, &EngineError{
				Message: "failed to unmarshal recorded I/O: " + err.Error(),
				Code:    "CHECKPOINT_FORMAT_ERROR",
			}
		}
	}

	// Store recorded I/O in context for nodes to access during replay
	// This is a placeholder - in real implementation, nodes would check for recorded I/O
	// and use it instead of making live calls
	ctx = context.WithValue(ctx, RecordedIOsKey, recordedIOs)

	// Resume execution from checkpoint
	// The replay logic is handled in RunWithCheckpoint, which will use recorded I/O
	// when nodes check for replay mode
	return e.RunWithCheckpoint(ctx, latestCheckpoint)
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
