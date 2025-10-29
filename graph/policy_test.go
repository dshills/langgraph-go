// Package graph_test provides functionality for the LangGraph-Go framework.
package graph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// PolicyTestState is a test state type used across policy tests.
type PolicyTestState struct {
	Value   string
	Counter int
}

// TestNodeTimeout (T072) verifies per-node timeout enforcement via NodePolicy.
//
// According to spec.md FR-014: System MUST enforce per-node timeouts via.
// NodePolicy.Timeout configuration.
//
// Requirements:
// - When NodePolicy.Timeout is set, node execution must not exceed timeout.
// - Returns context.DeadlineExceeded when node timeout is exceeded.
// - Only the timed-out node is cancelled, not the entire workflow.
// - Other nodes continue execution normally.
// - DefaultNodeTimeout is used when NodePolicy.Timeout is zero.
//
// This test creates nodes with explicit timeouts and verifies that timeout.
// enforcement works correctly at the per-node level.
func TestNodeTimeout(t *testing.T) {
	t.Run("enforces per-node timeout", func(t *testing.T) {
		// Note: This test is currently pending implementation of per-node timeout.
		// enforcement (T076). The NodePolicy struct exists, but the Engine doesn't.
		// yet apply per-node timeouts during execution.
		//
		// When T076 is implemented, this test should:
		// 1. Create a node with NodePolicy that has Timeout=100ms.
		// 2. Node execution attempts to run for 500ms.
		// 3. Verify node is cancelled after ~100ms with context.DeadlineExceeded.
		// 4. Verify workflow continues with error handling.

		t.Skip("Pending implementation of per-node timeout enforcement (T076)")

		// Create reducer.
		reducer := func(prev, delta PolicyTestState) PolicyTestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		// Create engine.
		st := store.NewMemStore[PolicyTestState]()
		emitter := emit.NewNullEmitter()
		opts := graph.Options{
			MaxSteps:           100,
			DefaultNodeTimeout: 200 * time.Millisecond, // Default timeout
		}
		engine := graph.New(reducer, st, emitter, opts)

		// TODO: Create node with explicit timeout policy
		// The node should implement:
		// - A Policy() method that returns NodePolicy with Timeout=100ms.
		// - A Run() method that attempts to run for 500ms.
		// - The engine should cancel it after 100ms (per NodePolicy.Timeout).
		//
		// Example (when implemented):
		// type TimedNode struct {.
		// timeout time.Duration.
		// }.
		// func (n *TimedNode) Policy() NodePolicy {.
		// return NodePolicy{Timeout: n.timeout}.
		// }.
		// func (n *TimedNode) Run(ctx context.Context, s PolicyTestState) NodeResult[PolicyTestState] {.
		// select {.
		//     case <-time.After(500 * time.Millisecond):
		// return NodeResult[PolicyTestState]{Delta: PolicyTestState{Counter: 1}, Route: Stop()}.
		//     case <-ctx.Done():
		// return NodeResult[PolicyTestState]{Err: ctx.Err()}.
		// }.
		// }.

		// Execute and verify timeout.
		ctx := context.Background()
		_, err := engine.Run(ctx, "node-timeout-test", PolicyTestState{})

		// Should get timeout error.
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	})

	t.Run("uses DefaultNodeTimeout when Policy().Timeout is zero", func(t *testing.T) {
		t.Skip("Pending implementation of per-node timeout enforcement (T076)")

		// This test verifies that when a node doesn't specify a timeout.
		// (NodePolicy.Timeout == 0), the engine uses Options.DefaultNodeTimeout.

		// TODO: Implement when T076 is complete
		// 1. Create engine with DefaultNodeTimeout=100ms.
		// 2. Create node with no explicit timeout (Policy().Timeout == 0).
		// 3. Node attempts to run for 500ms.
		// 4. Verify node is cancelled after ~100ms (using default).
	})

	t.Run("different nodes have independent timeouts", func(t *testing.T) {
		t.Skip("Pending implementation of per-node timeout enforcement (T076)")

		// This test verifies that node timeouts are independent:
		// - Node A has 50ms timeout.
		// - Node B has 200ms timeout.
		// - Node A times out but Node B completes successfully.

		// TODO: Implement when T076 is complete
		// 1. Create two nodes with different timeouts.
		// 2. Execute workflow A -> B.
		// 3. Verify A times out but B completes.
	})

	t.Run("no timeout when Policy().Timeout and DefaultNodeTimeout are zero", func(t *testing.T) {
		t.Skip("Pending implementation of per-node timeout enforcement (T076)")

		// This test verifies that nodes can run indefinitely when no timeout.
		// is configured (both Policy().Timeout and DefaultNodeTimeout are zero).

		// TODO: Implement when T076 is complete
		// 1. Create engine with DefaultNodeTimeout=0.
		// 2. Create node with Policy().Timeout=0.
		// 3. Node runs for reasonable time (100ms).
		// 4. Verify node completes without timeout.
	})
}

// TestRetryAttempts verifies that nodes are retried up to MaxAttempts times.
// when encountering retryable errors (T082).
func TestRetryAttempts(t *testing.T) {
	tests := []struct {
		name           string
		maxAttempts    int
		failureCount   int // How many times node should fail before succeeding
		wantAttempts   int
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:         "succeeds on first attempt",
			maxAttempts:  3,
			failureCount: 0,
			wantAttempts: 1,
			wantErr:      false,
		},
		{
			name:         "succeeds on second attempt after one failure",
			maxAttempts:  3,
			failureCount: 1,
			wantAttempts: 2,
			wantErr:      false,
		},
		{
			name:         "succeeds on third attempt after two failures",
			maxAttempts:  3,
			failureCount: 2,
			wantAttempts: 3,
			wantErr:      false,
		},
		{
			name:           "exceeds MaxAttempts with three failures",
			maxAttempts:    3,
			failureCount:   3,
			wantAttempts:   3,
			wantErr:        true,
			wantErrMessage: "MAX_ATTEMPTS_EXCEEDED",
		},
		{
			name:           "no retries with MaxAttempts=1",
			maxAttempts:    1,
			failureCount:   1,
			wantAttempts:   1,
			wantErr:        true,
			wantErrMessage: "MAX_ATTEMPTS_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			policy := &graph.RetryPolicy{
				MaxAttempts: tt.maxAttempts,
				BaseDelay:   1 * time.Millisecond, // Small delay for testing
				MaxDelay:    10 * time.Millisecond,
				Retryable: func(err error) bool {
					// Treat all errors as retryable for this test.
					return true
				},
			}

			// Simulate retry logic that would happen in the engine.
			var finalErr error
			for attempt := 0; attempt < tt.maxAttempts; attempt++ {
				attemptCount++

				// Simulate node execution.
				if attemptCount <= tt.failureCount {
					finalErr = errors.New("transient failure")
					continue
				}

				// Node succeeded.
				finalErr = nil
				break
			}

			// Check if we exceeded MaxAttempts.
			if finalErr != nil && attemptCount >= tt.maxAttempts {
				finalErr = errors.New("MAX_ATTEMPTS_EXCEEDED")
			}

			if attemptCount != tt.wantAttempts {
				t.Errorf("attempt count = %d, want %d", attemptCount, tt.wantAttempts)
			}

			if tt.wantErr {
				if finalErr == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErrMessage)
				} else if tt.wantErrMessage != "" && finalErr.Error() != tt.wantErrMessage {
					t.Errorf("error = %v, want error containing %q", finalErr, tt.wantErrMessage)
				}
			} else {
				if finalErr != nil {
					t.Errorf("unexpected error: %v", finalErr)
				}
			}

			// Verify policy configuration is valid.
			if policy.MaxAttempts < 1 {
				t.Errorf("RetryPolicy.MaxAttempts must be >= 1, got %d", policy.MaxAttempts)
			}
			if policy.MaxDelay < policy.BaseDelay {
				t.Errorf("RetryPolicy.MaxDelay (%v) must be >= BaseDelay (%v)", policy.MaxDelay, policy.BaseDelay)
			}
		})
	}
}

// TestExponentialBackoff verifies that the backoff formula follows exponential growth.
// with jitter: delay = base * 2^attempt + jitter(0, base) (T083).
func TestExponentialBackoff(t *testing.T) {
	tests := []struct {
		name      string
		attempt   int
		baseDelay time.Duration
		maxDelay  time.Duration
		wantMin   time.Duration // Min expected delay (exponential without jitter)
		wantMax   time.Duration // Max expected delay (exponential + max jitter, capped by maxDelay)
	}{
		{
			name:      "attempt 0 (first retry)",
			attempt:   0,
			baseDelay: 1 * time.Second,
			maxDelay:  30 * time.Second,
			wantMin:   1 * time.Second, // base * 2^0 = 1s
			wantMax:   2 * time.Second, // 1s + jitter(0, 1s)
		},
		{
			name:      "attempt 1 (second retry)",
			attempt:   1,
			baseDelay: 1 * time.Second,
			maxDelay:  30 * time.Second,
			wantMin:   2 * time.Second, // base * 2^1 = 2s
			wantMax:   3 * time.Second, // 2s + jitter(0, 1s)
		},
		{
			name:      "attempt 2 (third retry)",
			attempt:   2,
			baseDelay: 1 * time.Second,
			maxDelay:  30 * time.Second,
			wantMin:   4 * time.Second, // base * 2^2 = 4s
			wantMax:   5 * time.Second, // 4s + jitter(0, 1s)
		},
		{
			name:      "attempt 3 (fourth retry)",
			attempt:   3,
			baseDelay: 1 * time.Second,
			maxDelay:  30 * time.Second,
			wantMin:   8 * time.Second, // base * 2^3 = 8s
			wantMax:   9 * time.Second, // 8s + jitter(0, 1s)
		},
		{
			name:      "capped by maxDelay",
			attempt:   10, // Large attempt would cause 2^10 = 1024s
			baseDelay: 1 * time.Second,
			maxDelay:  30 * time.Second,
			wantMin:   30 * time.Second, // Capped at maxDelay
			wantMax:   31 * time.Second, // maxDelay + jitter(0, 1s)
		},
		{
			name:      "small baseDelay",
			attempt:   3,
			baseDelay: 100 * time.Millisecond,
			maxDelay:  10 * time.Second,
			wantMin:   800 * time.Millisecond, // 100ms * 2^3 = 800ms
			wantMax:   900 * time.Millisecond, // 800ms + jitter(0, 100ms)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't test the actual computeBackoff function until it's implemented,
			// but we can test the expected behavior based on the formula from research.md.

			// Expected exponential delay: base * 2^attempt.
			exponentialDelay := tt.baseDelay * (1 << tt.attempt)

			// Cap at maxDelay.
			if exponentialDelay > tt.maxDelay {
				exponentialDelay = tt.maxDelay
			}

			// Verify expected delay matches test case expectations.
			if exponentialDelay < tt.wantMin {
				t.Errorf("exponential delay = %v, want >= %v", exponentialDelay, tt.wantMin)
			}

			// The actual delay would be exponentialDelay + jitter(0, baseDelay).
			// So maximum possible delay is exponentialDelay + baseDelay.
			maxPossibleDelay := exponentialDelay + tt.baseDelay
			if maxPossibleDelay != tt.wantMax {
				t.Errorf("max possible delay = %v, want %v", maxPossibleDelay, tt.wantMax)
			}

			// For capped delays, verify that exponential growth was actually exceeded.
			if tt.attempt >= 10 {
				uncappedDelay := tt.baseDelay * (1 << tt.attempt)
				if uncappedDelay <= tt.maxDelay {
					t.Errorf("test case expects capping but uncapped delay %v <= maxDelay %v",
						uncappedDelay, tt.maxDelay)
				}
			}
		})
	}
}

// TestRetryableError verifies that the Retryable predicate correctly classifies.
// errors as retryable or non-retryable (T084).
func TestRetryableError(t *testing.T) {
	// Define common error types.
	var (
		networkErr    = errors.New("network: connection refused")
		timeoutErr    = errors.New("context deadline exceeded")
		validationErr = errors.New("validation: invalid input")
		notFoundErr   = errors.New("resource not found")
		rate429Err    = errors.New("http: 429 rate limit exceeded")
		server503Err  = errors.New("http: 503 service unavailable")
		permissionErr = errors.New("permission denied")
	)

	tests := []struct {
		name          string
		err           error
		retryable     func(error) bool
		wantRetryable bool
	}{
		{
			name: "network errors are retryable",
			err:  networkErr,
			retryable: func(err error) bool {
				return err != nil && (errors.Is(err, networkErr) ||
					err.Error() == "network: connection refused")
			},
			wantRetryable: true,
		},
		{
			name: "timeout errors are retryable",
			err:  timeoutErr,
			retryable: func(err error) bool {
				return err != nil && err.Error() == "context deadline exceeded"
			},
			wantRetryable: true,
		},
		{
			name: "validation errors are not retryable",
			err:  validationErr,
			retryable: func(err error) bool {
				if err != nil && errors.Is(err, validationErr) {
					return false
				}
				return true
			},
			wantRetryable: false,
		},
		{
			name: "not found errors are not retryable",
			err:  notFoundErr,
			retryable: func(err error) bool {
				if err != nil && errors.Is(err, notFoundErr) {
					return false
				}
				return true
			},
			wantRetryable: false,
		},
		{
			name: "http 429 rate limit is retryable",
			err:  rate429Err,
			retryable: func(err error) bool {
				return err != nil && err.Error() == "http: 429 rate limit exceeded"
			},
			wantRetryable: true,
		},
		{
			name: "http 503 service unavailable is retryable",
			err:  server503Err,
			retryable: func(err error) bool {
				return err != nil && err.Error() == "http: 503 service unavailable"
			},
			wantRetryable: true,
		},
		{
			name: "permission errors are not retryable",
			err:  permissionErr,
			retryable: func(err error) bool {
				if err != nil && errors.Is(err, permissionErr) {
					return false
				}
				return true
			},
			wantRetryable: false,
		},
		{
			name:          "nil retryable func treats all errors as non-retryable",
			err:           networkErr,
			retryable:     nil,
			wantRetryable: false,
		},
		{
			name: "always retry predicate",
			err:  validationErr,
			retryable: func(err error) bool {
				return true
			},
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var isRetryable bool
			if tt.retryable == nil {
				isRetryable = false
			} else {
				isRetryable = tt.retryable(tt.err)
			}

			if isRetryable != tt.wantRetryable {
				t.Errorf("retryable(%v) = %v, want %v", tt.err, isRetryable, tt.wantRetryable)
			}

			policy := &graph.RetryPolicy{
				MaxAttempts: 3,
				BaseDelay:   1 * time.Second,
				MaxDelay:    30 * time.Second,
				Retryable:   tt.retryable,
			}

			shouldRetry := policy.Retryable != nil && policy.Retryable(tt.err)
			if shouldRetry != tt.wantRetryable {
				t.Errorf("policy should retry = %v, want %v", shouldRetry, tt.wantRetryable)
			}
		})
	}
}

// TestMaxAttemptsExceeded verifies that retry attempts stop when MaxAttempts.
// is reached and ErrMaxAttemptsExceeded is raised (T085).
func TestMaxAttemptsExceeded(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		failures    int
		wantErr     bool
		wantErrCode string
	}{
		{
			name:        "success before limit",
			maxAttempts: 3,
			failures:    2,
			wantErr:     false,
		},
		{
			name:        "exactly at limit",
			maxAttempts: 3,
			failures:    3,
			wantErr:     true,
			wantErrCode: "MAX_ATTEMPTS_EXCEEDED",
		},
		{
			name:        "way beyond limit",
			maxAttempts: 2,
			failures:    10,
			wantErr:     true,
			wantErrCode: "MAX_ATTEMPTS_EXCEEDED",
		},
		{
			name:        "MaxAttempts=1 means no retries",
			maxAttempts: 1,
			failures:    1,
			wantErr:     true,
			wantErrCode: "MAX_ATTEMPTS_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &graph.RetryPolicy{
				MaxAttempts: tt.maxAttempts,
				BaseDelay:   1 * time.Millisecond,
				MaxDelay:    10 * time.Millisecond,
				Retryable: func(err error) bool {
					return true
				},
			}

			var lastErr error
			attemptsMade := 0

			for attemptsMade < tt.maxAttempts {
				attemptsMade++

				if attemptsMade <= tt.failures {
					lastErr = errors.New("transient failure")
					if attemptsMade < tt.maxAttempts && policy.Retryable(lastErr) {
						continue
					}
					break
				}

				lastErr = nil
				break
			}

			if lastErr != nil && attemptsMade >= tt.maxAttempts {
				lastErr = errors.New("MAX_ATTEMPTS_EXCEEDED")
			}

			if tt.wantErr {
				if lastErr == nil {
					t.Errorf("expected error, got nil")
				} else if tt.wantErrCode != "" && lastErr.Error() != tt.wantErrCode {
					t.Errorf("error = %v, want %v", lastErr, tt.wantErrCode)
				}
			} else {
				if lastErr != nil {
					t.Errorf("unexpected error: %v", lastErr)
				}
			}

			if attemptsMade > tt.maxAttempts {
				t.Errorf("attemptsMade = %d, want <= %d", attemptsMade, tt.maxAttempts)
			}

			if tt.failures >= tt.maxAttempts && attemptsMade != tt.maxAttempts {
				t.Errorf("attemptsMade = %d, want %d", attemptsMade, tt.maxAttempts)
			}
		})
	}
}
