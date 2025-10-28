package graph

import "time"

// Policy defines node execution policies and retry strategies

// NodePolicy configures the execution behavior for a specific node, including
// timeouts, retry logic, and idempotency key generation.
//
// Policies are attached to nodes and enforced by the scheduler. If not specified,
// default values from Options are used.
type NodePolicy struct {
	// Timeout is the maximum execution time allowed for this node.
	// If zero, Options.DefaultNodeTimeout is used.
	Timeout time.Duration

	// RetryPolicy specifies automatic retry behavior for transient failures.
	// If nil, no retries are attempted.
	RetryPolicy *RetryPolicy

	// IdempotencyKeyFunc generates a custom idempotency key from the state.
	// If nil, a default key based on node ID and step ID is used.
	// This is useful for side-effecting nodes that need exactly-once semantics.
	IdempotencyKeyFunc func(state any) string
}

// RetryPolicy defines automatic retry configuration for transient node failures.
//
// When a node execution fails, the retry policy determines whether the failure
// is retryable and how long to wait before the next attempt. Exponential backoff
// with jitter is used to avoid thundering herd problems.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of execution attempts (including initial attempt).
	// Must be >= 1. A value of 1 means no retries.
	MaxAttempts int

	// BaseDelay is the base delay for exponential backoff between retries.
	// The actual delay is computed as: min(BaseDelay * 2^attempt + jitter, MaxDelay)
	BaseDelay time.Duration

	// MaxDelay is the maximum delay cap for exponential backoff.
	// Must be >= BaseDelay.
	MaxDelay time.Duration

	// Retryable is a predicate function that determines if an error is retryable.
	// If nil, all errors are considered non-retryable.
	// Common patterns:
	//   - Network errors: temporary, connection refused, timeout
	//   - HTTP 429, 503, 504
	//   - Database deadlocks
	Retryable func(error) bool
}

// SideEffectPolicy declares the external I/O characteristics of a node,
// informing the replay engine whether the node's interactions should be
// recorded and replayed.
//
// This policy affects deterministic replay behavior:
//   - Recordable=true: I/O is captured and can be replayed without re-execution
//   - RequiresIdempotency=true: Node needs idempotency key to ensure exactly-once semantics
type SideEffectPolicy struct {
	// Recordable indicates whether this node's I/O can be captured for replay.
	// Examples:
	//   - LLM API calls: true (responses are cacheable)
	//   - Pure functions: false (no external I/O)
	//   - Database queries: false (may be non-deterministic)
	Recordable bool

	// RequiresIdempotency indicates whether this node requires an idempotency key
	// to prevent duplicate execution. This is important for side-effecting operations
	// like database writes, payments, or notifications.
	//
	// If true, the node must provide an IdempotencyKeyFunc in its NodePolicy.
	RequiresIdempotency bool
}
