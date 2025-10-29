// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"math/rand"
	"time"
)

// Policy defines node execution policies and retry strategies.

// NodePolicy configures the execution behavior for a specific node, including.
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
// When a node execution fails, the retry policy determines whether the failure.
// is retryable and how long to wait before the next attempt. Exponential backoff.
// with jitter is used to avoid thundering herd problems.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of execution attempts (including initial attempt).
	// Must be >= 1. A value of 1 means no retries.
	MaxAttempts int

	// BaseDelay is the base delay for exponential backoff between retries.
	// The actual delay is computed as: min(BaseDelay * 2^attempt + jitter, MaxDelay).
	BaseDelay time.Duration

	// MaxDelay is the maximum delay cap for exponential backoff.
	// Must be >= BaseDelay.
	MaxDelay time.Duration

	// Retryable is a predicate function that determines if an error is retryable.
	// If nil, all errors are considered non-retryable.
	// Common patterns:
	// - Network errors: temporary, connection refused, timeout.
	// - HTTP 429, 503, 504.
	// - Database deadlocks.
	Retryable func(error) bool
}

// SideEffectPolicy declares the external I/O characteristics of a node,
// informing the replay engine whether the node's interactions should be.
// recorded and replayed.
//
// This policy affects deterministic replay behavior:
// - Recordable=true: I/O is captured and can be replayed without re-execution.
// - RequiresIdempotency=true: Node needs idempotency key to ensure exactly-once semantics.
type SideEffectPolicy struct {
	// Recordable indicates whether this node's I/O can be captured for replay.
	// Examples:
	// - LLM API calls: true (responses are cacheable).
	// - Pure functions: false (no external I/O).
	// - Database queries: false (may be non-deterministic).
	Recordable bool

	// RequiresIdempotency indicates whether this node requires an idempotency key.
	// to prevent duplicate execution. This is important for side-effecting operations.
	// like database writes, payments, or notifications.
	//
	// If true, the node must provide an IdempotencyKeyFunc in its NodePolicy.
	RequiresIdempotency bool
}

// computeBackoff calculates the delay before retrying a failed node execution.
// using exponential backoff with jitter (T086).
//
// The backoff formula follows research.md section 7:
//
// delay = min(base * 2^attempt, maxDelay) + jitter(0, base).
//
// Where:
// - attempt: Retry attempt number (0 for first retry, 1 for second, etc.).
// - base: Base delay from RetryPolicy.BaseDelay.
// - maxDelay: Maximum cap from RetryPolicy.MaxDelay.
// - jitter: Random value between 0 and base to prevent thundering herd.
//
// The exponential component (2^attempt) doubles the delay with each retry,
// reducing load on failing services. Jitter randomizes retry timing across.
// concurrent nodes to avoid synchronized retry storms.
//
// Parameters:
// - attempt: Zero-based retry attempt number (0 = first retry).
// - base: Base delay for exponential calculation.
// - maxDelay: Maximum allowed delay (caps exponential growth).
// - rng: Random number generator for jitter (use context RNG for determinism).
//
// Returns:
// - Computed delay duration including exponential backoff and jitter.
//
// Example delays with base=1s, maxDelay=30s:
// - attempt 0: 1s + jitter(0, 1s) = 1-2s.
// - attempt 1: 2s + jitter(0, 1s) = 2-3s.
// - attempt 2: 4s + jitter(0, 1s) = 4-5s.
// - attempt 3: 8s + jitter(0, 1s) = 8-9s.
// - attempt 10: 30s + jitter(0, 1s) = 30-31s (capped).
func computeBackoff(attempt int, base, maxDelay time.Duration, rng *rand.Rand) time.Duration {
	// Compute exponential delay: base * 2^attempt.
	// Use bit shift for efficient 2^attempt calculation.
	exponentialDelay := base * (1 << attempt)

	// Cap at maxDelay to prevent unbounded growth.
	if exponentialDelay > maxDelay {
		exponentialDelay = maxDelay
	}

	// Add jitter: random value between 0 and base.
	// This prevents synchronized retries (thundering herd).
	var jitter time.Duration
	if rng != nil {
		jitter = time.Duration(rng.Int63n(int64(base)))
	} else {
		// Fallback to time-based random if no RNG provided.
		// Not deterministic, but safe for non-replay scenarios.
		// Note: Using math/rand for jitter timing, not security-sensitive
		jitter = time.Duration(rand.Int63n(int64(base))) // #nosec G404 -- jitter for retry timing, not security
	}

	return exponentialDelay + jitter
}

// Validate checks if the RetryPolicy configuration is valid.
// Returns an error if any constraints are violated:
//   - MaxAttempts must be >= 1 (1 means no retries, just initial attempt)
//   - If both MaxDelay and BaseDelay are > 0, then MaxDelay must be >= BaseDelay
//     (MaxDelay == 0 is treated as "no maximum delay cap")
func (rp *RetryPolicy) Validate() error {
	if rp.MaxAttempts < 1 {
		return ErrInvalidRetryPolicy
	}
	if rp.MaxDelay > 0 && rp.BaseDelay > 0 && rp.MaxDelay < rp.BaseDelay {
		return ErrInvalidRetryPolicy
	}
	return nil
}
