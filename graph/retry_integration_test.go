package graph_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	. "github.com/dshills/langgraph-go/graph" //nolint:revive // dot import improves test readability
	"github.com/dshills/langgraph-go/graph/store"
)

// Test helper node that wraps a function with a retry policy
type retryableNode[S any] struct {
	fn     func(context.Context, S) NodeResult[S]
	policy NodePolicy
}

func (n retryableNode[S]) Run(ctx context.Context, state S) NodeResult[S] {
	return n.fn(ctx, state)
}

func (n retryableNode[S]) Policy() NodePolicy {
	return n.policy
}

// TestRetryIntegration verifies end-to-end retry functionality (T093)
func TestRetryIntegration(t *testing.T) {
	type TestState struct {
		Value int
	}

	t.Run("node succeeds after two retries", func(t *testing.T) {
		attempts := 0
		var mu sync.Mutex

		node := retryableNode[TestState]{
			fn: func(_ context.Context, s TestState) NodeResult[TestState] {
				mu.Lock()
				attempts++
				currentAttempt := attempts
				mu.Unlock()

				if currentAttempt <= 2 {
					return NodeResult[TestState]{
						Err: errors.New("transient failure"),
					}
				}

				return NodeResult[TestState]{
					Delta: TestState{Value: s.Value + 1},
					Route: Stop(),
				}
			},
			policy: NodePolicy{
				RetryPolicy: &RetryPolicy{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Millisecond,
					MaxDelay:    10 * time.Millisecond,
					Retryable: func(_ error) bool {
						return true
					},
				},
			},
		}

		reducer := func(prev, delta TestState) TestState {
			return TestState{Value: prev.Value + delta.Value}
		}

		memStore := store.NewMemStore[TestState]()
		engine := New(reducer, memStore, nil, Options{
			MaxSteps:           10,
			MaxConcurrentNodes: 2,
		})

		_ = engine.Add("retry_node", node)
		if err := engine.StartAt("retry_node"); err != nil {
			t.Fatalf("Failed to set start node to retry_node: %v", err)
		}

		finalState, err := engine.Run(context.Background(), "test-retry", TestState{Value: 0})
		if err != nil {
			t.Fatalf("Run() failed: %v", err)
		}

		mu.Lock()
		finalAttempts := attempts
		mu.Unlock()

		if finalAttempts != 3 {
			t.Errorf("expected 3 attempts, got %d", finalAttempts)
		}

		if finalState.Value != 1 {
			t.Errorf("expected final value 1, got %d", finalState.Value)
		}
	})

	t.Run("node fails after MaxAttempts exceeded", func(t *testing.T) {
		attempts := 0
		var mu sync.Mutex

		node := retryableNode[struct{}]{
			fn: func(_ context.Context, _ struct{}) NodeResult[struct{}] {
				mu.Lock()
				attempts++
				mu.Unlock()

				return NodeResult[struct{}]{
					Err: errors.New("permanent failure"),
				}
			},
			policy: NodePolicy{
				RetryPolicy: &RetryPolicy{
					MaxAttempts: 2,
					BaseDelay:   1 * time.Millisecond,
					MaxDelay:    5 * time.Millisecond,
					Retryable: func(_ error) bool {
						return true
					},
				},
			},
		}

		reducer := func(_, _ struct{}) struct{} { return struct{}{} }
		memStore := store.NewMemStore[struct{}]()
		engine := New(reducer, memStore, nil, Options{
			MaxSteps:           10,
			MaxConcurrentNodes: 2,
		})

		_ = engine.Add("failing_node", node)
		if err := engine.StartAt("failing_node"); err != nil {
			t.Fatalf("Failed to set start node to failing_node: %v", err)
		}

		_, err := engine.Run(context.Background(), "test-fail", struct{}{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, ErrMaxAttemptsExceeded) {
			t.Errorf("expected ErrMaxAttemptsExceeded, got %v", err)
		}

		mu.Lock()
		finalAttempts := attempts
		mu.Unlock()

		if finalAttempts != 2 {
			t.Errorf("expected 2 attempts, got %d", finalAttempts)
		}
	})

	t.Run("non-retryable error fails immediately", func(t *testing.T) {
		attempts := 0
		var mu sync.Mutex
		nonRetryableErr := errors.New("validation error")

		node := retryableNode[struct{}]{
			fn: func(_ context.Context, _ struct{}) NodeResult[struct{}] {
				mu.Lock()
				attempts++
				mu.Unlock()

				return NodeResult[struct{}]{
					Err: nonRetryableErr,
				}
			},
			policy: NodePolicy{
				RetryPolicy: &RetryPolicy{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Millisecond,
					MaxDelay:    5 * time.Millisecond,
					Retryable: func(err error) bool {
						return !errors.Is(err, nonRetryableErr)
					},
				},
			},
		}

		reducer := func(_, _ struct{}) struct{} { return struct{}{} }
		memStore := store.NewMemStore[struct{}]()
		engine := New(reducer, memStore, nil, Options{
			MaxSteps:           10,
			MaxConcurrentNodes: 2,
		})

		_ = engine.Add("node", node)
		if err := engine.StartAt("node"); err != nil {
			t.Fatalf("Failed to set start node to node: %v", err)
		}

		_, err := engine.Run(context.Background(), "test-non-retryable", struct{}{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		mu.Lock()
		finalAttempts := attempts
		mu.Unlock()

		if finalAttempts != 1 {
			t.Errorf("expected 1 attempt, got %d", finalAttempts)
		}

		if !errors.Is(err, nonRetryableErr) {
			t.Errorf("expected original error, got %v", err)
		}
	})
}
