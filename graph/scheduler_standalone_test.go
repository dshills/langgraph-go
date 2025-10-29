// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Standalone tests for scheduler implementation (T028-T032).
// These tests don't depend on engine.go to avoid compilation issues.

func TestComputeOrderKeyStandalone(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		key1 := ComputeOrderKey("node1", 0)
		key2 := ComputeOrderKey("node1", 0)
		if key1 != key2 {
			t.Errorf("Expected deterministic keys, got %d and %d", key1, key2)
		}
	})

	t.Run("different inputs produce different keys", func(t *testing.T) {
		key1 := ComputeOrderKey("node1", 0)
		key2 := ComputeOrderKey("node2", 0)
		key3 := ComputeOrderKey("node1", 1)

		if key1 == key2 {
			t.Error("Different parent nodes should produce different keys")
		}
		if key1 == key3 {
			t.Error("Different edge indices should produce different keys")
		}
	})

	t.Run("non-zero keys", func(t *testing.T) {
		key := ComputeOrderKey("test", 0)
		if key == 0 {
			t.Error("Order key should not be zero")
		}
	})
}

func TestFrontierStandalone(t *testing.T) {
	type testState struct {
		Value int
	}

	t.Run("enqueue and dequeue in order", func(t *testing.T) {
		ctx := context.Background()
		f := NewFrontier[testState](ctx, 10)

		// Enqueue items with different order keys.
		items := []WorkItem[testState]{
			{OrderKey: 300, NodeID: "node3", StepID: 1},
			{OrderKey: 100, NodeID: "node1", StepID: 1},
			{OrderKey: 200, NodeID: "node2", StepID: 1},
		}

		for _, item := range items {
			if err := f.Enqueue(ctx, item); err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}
		}

		// Verify length.
		if f.Len() != 3 {
			t.Errorf("Expected length 3, got %d", f.Len())
		}

		// Dequeue and verify order.
		for i, expectedKey := range []uint64{100, 200, 300} {
			item, err := f.Dequeue(ctx)
			if err != nil {
				t.Fatalf("Dequeue %d failed: %v", i, err)
			}
			if item.OrderKey != expectedKey {
				t.Errorf("Expected OrderKey %d, got %d", expectedKey, item.OrderKey)
			}
		}

		// Verify empty.
		if f.Len() != 0 {
			t.Errorf("Expected empty frontier, got length %d", f.Len())
		}
	})

	t.Run("backpressure on full queue", func(t *testing.T) {
		ctx := context.Background()
		f := NewFrontier[testState](ctx, 2)

		// Fill the queue.
		for i := 0; i < 2; i++ {
			item := WorkItem[testState]{OrderKey: uint64(i), NodeID: "node", StepID: i} // #nosec G115 -- test loop counter, bounded by loop limit

			if err := f.Enqueue(ctx, item); err != nil {
				t.Fatalf("Enqueue %d failed: %v", i, err)
			}
		}

		// Try to enqueue one more with timeout.
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		item := WorkItem[testState]{OrderKey: 999, NodeID: "blocking", StepID: 99}
		err := f.Enqueue(timeoutCtx, item)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected DeadlineExceeded, got %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		f := NewFrontier[testState](ctx, 10)

		cancel() // Cancel immediately

		// Enqueue should fail.
		item := WorkItem[testState]{OrderKey: 100, NodeID: "node", StepID: 1}
		err := f.Enqueue(ctx, item)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected Canceled, got %v", err)
		}

		// Dequeue should fail.
		_, err = f.Dequeue(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected Canceled, got %v", err)
		}
	})

	t.Run("concurrent operations", func(t *testing.T) {
		ctx := context.Background()
		f := NewFrontier[testState](ctx, 100)

		const numItems = 20
		done := make(chan bool)

		// Start dequeuing.
		go func() {
			for i := 0; i < numItems; i++ {
				_, err := f.Dequeue(ctx)
				if err != nil {
					t.Errorf("Dequeue failed: %v", err)
				}
			}
			done <- true
		}()

		// Enqueue items.
		for i := 0; i < numItems; i++ {
			item := WorkItem[testState]{
				OrderKey: uint64(i * 10), // #nosec G115 -- test loop counter, bounded by loop limit
				NodeID:   "node",
				StepID:   i,
			}
			if err := f.Enqueue(ctx, item); err != nil {
				t.Errorf("Enqueue failed: %v", err)
			}
		}

		// Wait for completion.
		select {
		case <-done:
			// Success.
		case <-time.After(2 * time.Second):
			t.Fatal("Test timed out")
		}

		// Verify empty.
		if f.Len() != 0 {
			t.Errorf("Expected empty frontier, got length %d", f.Len())
		}
	})
}
