// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import "time"

// Option is a functional option for configuring an Engine.
//
// Functional options provide a clean, extensible API for engine configuration:
// - Chainable: engine := New(reducer, store, emitter, WithMaxConcurrent(8), WithQueueDepth(1024)).
// - Self-documenting: Option names clearly describe their purpose.
// - Optional: Only specify the configuration you need.
// - Backward compatible: Existing Options struct still works.
//
// Example:
//
// engine := graph.New(.
//
//	reducer,
//	store,
//	emitter,
//	graph.WithMaxConcurrent(16),
//	graph.WithQueueDepth(2048),
//	graph.WithDefaultNodeTimeout(10*time.Second),
//
// ).
//
// Options can be mixed with the Options struct:
//
// opts := graph.Options{MaxSteps: 100}.
// engine := graph.New(.
//
//	reducer,
//	store,
//	emitter,
//	opts,
//
// graph.WithMaxConcurrent(8), // Overrides opts if specified.
// ).
type Option func(*engineConfig) error

// engineConfig is an internal struct used to collect options before applying them to an Engine.
// This indirection allows validation and composition of options.
type engineConfig struct {
	opts Options
}

// WithMaxSteps limits workflow execution to prevent infinite loops.
//
// Default: 0 (no limit, use with caution).
//
// Workflow loops (A → B → A) are fully supported. Use MaxSteps to prevent
// infinite loops when a conditional exit is missing or misconfigured.
//
// Loop patterns:
//  1. Node-based conditional loop: nodeA returns Goto("B") if shouldContinue(state), else Stop()
//  2. Edge predicate loop: Connect("A", "B", loopPredicate) and Connect("A", "exit", exitPredicate)
//
// Recommended values:
//   - Simple workflows (3-5 nodes): MaxSteps = 20
//   - Workflows with loops: MaxSteps = depth × max_iterations (e.g., 5 nodes × 10 iterations = 50)
//   - Complex multi-loop workflows: MaxSteps = 100-200
//
// When MaxSteps is exceeded, Run() returns EngineError with code "MAX_STEPS_EXCEEDED".
//
// Example:
//
//	engine := graph.New(
//	    reducer, store, emitter,
//	    graph.WithMaxSteps(100), // Limit to 100 execution steps
//	)
func WithMaxSteps(n int) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.MaxSteps = n
		return nil
	}
}

// WithMaxConcurrent sets the maximum number of nodes executing concurrently.
//
// Default: 8 when concurrent mode is enabled (determined by presence of other concurrent options).
// Set to 0 for sequential execution (backward compatible default).
//
// Tuning guidance:
// - CPU-bound workflows: Set to runtime.NumCPU().
// - I/O-bound workflows: Set to 10-50 depending on external service limits.
// - Memory-constrained: Reduce to prevent excessive state copies.
//
// Each concurrent node holds a deep copy of state, so memory usage scales.
// linearly with MaxConcurrentNodes.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//
// graph.WithMaxConcurrent(16), // Up to 16 nodes execute in parallel.
// ).
func WithMaxConcurrent(n int) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.MaxConcurrentNodes = n
		return nil
	}
}

// WithQueueDepth sets the capacity of the execution frontier queue.
//
// Default: 1024. Increase for workflows with large fan-outs.
//
// When the queue fills, new work items block until space is available.
// This provides backpressure to prevent unbounded memory growth.
//
// Recommended: MaxConcurrentNodes × 100 for initial estimate.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//	graph.WithMaxConcurrent(8),
//
// graph.WithQueueDepth(2048), // Queue can hold 2048 work items.
// ).
func WithQueueDepth(n int) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.QueueDepth = n
		return nil
	}
}

// WithBackpressureTimeout sets the maximum time to wait when the frontier queue is full.
//
// Default: 30s. If exceeded, Run() returns ErrBackpressureTimeout.
//
// Set lower for fast-failing systems, higher for workflows with bursty fan-outs.
//
// When backpressure timeout is exceeded:
// 1. Execution pauses with checkpoint saved.
// 2. ErrBackpressureTimeout is returned.
// 3. Resume from checkpoint after reducing load or increasing MaxConcurrentNodes.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//	graph.WithMaxConcurrent(8),
//
// graph.WithBackpressureTimeout(60*time.Second), // Wait up to 60s for queue space.
// ).
func WithBackpressureTimeout(d time.Duration) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.BackpressureTimeout = d
		return nil
	}
}

// WithDefaultNodeTimeout sets the maximum execution time for nodes without explicit Policy().Timeout.
//
// Default: 30s. Individual nodes can override via NodePolicy.Timeout.
//
// Prevents single slow nodes from blocking workflow progress indefinitely.
// When exceeded, node execution is cancelled and returns context.DeadlineExceeded.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//
// graph.WithDefaultNodeTimeout(10*time.Second), // All nodes timeout after 10s by default.
// ).
func WithDefaultNodeTimeout(d time.Duration) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.DefaultNodeTimeout = d
		return nil
	}
}

// WithRunWallClockBudget sets the maximum total execution time for Run().
//
// Default: 10m. If exceeded, Run() returns context.DeadlineExceeded.
//
// Use this to enforce hard deadlines on entire workflow execution.
// Set to 0 to disable (workflow runs until completion or MaxSteps).
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//
// graph.WithRunWallClockBudget(5*time.Minute), // Entire workflow must complete in 5 minutes.
// ).
func WithRunWallClockBudget(d time.Duration) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.RunWallClockBudget = d
		return nil
	}
}

// WithReplayMode enables deterministic replay using recorded I/O.
//
// Default: false (record mode - captures I/O for later replay).
//
// When true, nodes with SideEffectPolicy.Recordable=true will use recorded.
// responses instead of executing live I/O. This enables:
// - Debugging: Replay production executions locally.
// - Testing: Verify workflow logic without external dependencies.
// - Auditing: Reconstruct exact execution flow from checkpoints.
//
// Requires prior execution with ReplayMode=false to record I/O.
//
// Example:
//
// // Original execution (record mode).
// recordEngine := graph.New(reducer, store, emitter, graph.WithReplayMode(false)).
// _, _ = recordEngine.Run(ctx, "run-001", initialState).
//
// // Later: replay execution.
// replayEngine := graph.New(reducer, store, emitter, graph.WithReplayMode(true)).
// replayedState, _ := replayEngine.ReplayRun(ctx, "run-001").
func WithReplayMode(enabled bool) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.ReplayMode = enabled
		return nil
	}
}

// WithStrictReplay controls replay mismatch behavior.
//
// Default: true (fail on I/O hash mismatch).
//
// When true, replay mode verifies recorded I/O hashes match expected values.
// If a mismatch is detected (indicating logic changes), Run() returns ErrReplayMismatch.
//
// Set to false to allow "best effort" replay that tolerates minor changes.
// Useful when debugging with modified node logic.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//	graph.WithReplayMode(true),
//
// graph.WithStrictReplay(false), // Allow replay even if logic changed.
// ).
func WithStrictReplay(enabled bool) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.StrictReplay = enabled
		return nil
	}
}

// ConflictPolicy defines how concurrent state updates are handled when multiple branches.
// modify the same state fields.
//
// This is a placeholder for future CRDT (Conflict-free Replicated Data Type) support.
// Currently only ConflictFail is supported, which returns an error on detected conflicts.
//
// Planned policies:
// - ConflictFail: Return error on conflict (default, implemented).
// - LastWriterWins: Last merge wins based on OrderKey.
// - CRDT: Use conflict-free replicated data types for automatic resolution.
type ConflictPolicy int

const (
	// ConflictFail returns an error when concurrent branches modify the same state field.
	// This is the safest default, requiring explicit conflict resolution.
	ConflictFail ConflictPolicy = iota

	// LastWriterWins uses the delta with the highest OrderKey when conflicts occur.
	// WARNING: Not yet implemented. Will return error if specified.
	LastWriterWins

	// ConflictCRDT uses CRDT semantics for automatic conflict resolution.
	// WARNING: Not yet implemented. Will return error if specified.
	ConflictCRDT
)

// WithConflictPolicy sets the policy for handling concurrent state update conflicts.
//
// Default: ConflictFail (returns error on conflict).
//
// Only ConflictFail is currently supported. Other policies are planned for future releases.
//
// Example:
//
// engine := graph.New(.
//
//	reducer, store, emitter,
//
// graph.WithConflictPolicy(graph.ConflictFail), // Explicit error on conflicts.
// ).
func WithConflictPolicy(policy ConflictPolicy) Option {
	return func(cfg *engineConfig) error {
		// Currently only ConflictFail is supported.
		// Future: Implement LastWriterWins and ConflictCRDT.
		if policy != ConflictFail {
			return &EngineError{
				Message: "only ConflictFail policy is currently supported",
				Code:    "UNSUPPORTED_CONFLICT_POLICY",
			}
		}
		// Note: Options struct doesn't have ConflictPolicy field yet.
		// This is reserved for future use when field is added.
		return nil
	}
}

// WithMetrics enables Prometheus metrics collection.
//
// Metrics enable production monitoring with 6 key metrics:
//   - inflight_nodes: Current concurrent node count
//   - queue_depth: Pending nodes in scheduler queue
//   - step_latency_ms: Node execution duration histogram
//   - retries_total: Cumulative retry attempts
//   - merge_conflicts_total: Concurrent state merge conflicts
//   - backpressure_events_total: Queue saturation events
//
// All metrics are automatically updated during workflow execution.
//
// Example:
//
//	registry := prometheus.NewRegistry()
//	metrics := graph.NewPrometheusMetrics(registry)
//	engine := graph.New(
//	    reducer, store, emitter,
//	    graph.WithMetrics(metrics),
//	)
//
//	// Expose metrics endpoint
//	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
func WithMetrics(metrics *PrometheusMetrics) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.Metrics = metrics
		return nil
	}
}

// WithCostTracker enables LLM cost tracking with static pricing.
//
// Cost tracker monitors:
//   - Per-model token usage (input/output)
//   - Cost calculation using static pricing tables
//   - Cumulative cost tracking across calls
//   - Per-model cost attribution
//
// Static pricing includes OpenAI, Anthropic, and Google models.
//
// Example:
//
//	tracker := graph.NewCostTracker("run-123", "USD")
//	engine := graph.New(
//	    reducer, store, emitter,
//	    graph.WithCostTracker(tracker),
//	)
//
//	// After execution, get cost summary
//	totalCost := tracker.GetTotalCost()
//	modelCosts := tracker.GetCostByModel()
func WithCostTracker(tracker *CostTracker) Option {
	return func(cfg *engineConfig) error {
		cfg.opts.CostTracker = tracker
		return nil
	}
}
