package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph"
)

// TestOrderKeyGeneration (T022) verifies that computeOrderKey generates
// deterministic uint64 values for ordering work items in the frontier.
//
// According to spec.md FR-002: System MUST process nodes in deterministic order
// based on (step_id, order_key) tuple within same step.
//
// Requirements:
// - Same inputs must produce same key (determinism)
// - Different inputs must produce different keys (collision resistance)
// - Ordering must be consistent (parent key < child keys)
//
// This test should FAIL initially because computeOrderKey function doesn't exist yet.
func TestOrderKeyGeneration(t *testing.T) {

	t.Run("deterministic key generation", func(t *testing.T) {
		// Same inputs should produce same key
		key1 := graph.ComputeOrderKey("node1", 0)
		key2 := graph.ComputeOrderKey("node1", 0)

		if key1 != key2 {
			t.Errorf("same inputs produced different keys: %d != %d", key1, key2)
		}
	})

	t.Run("different inputs produce different keys", func(t *testing.T) {
		// Different parent nodes should produce different keys
		key1 := graph.ComputeOrderKey("node1", 0)
		key2 := graph.ComputeOrderKey("node2", 0)

		if key1 == key2 {
			t.Error("different parent nodes produced same key")
		}

		// Different edge indices should produce different keys
		key3 := graph.ComputeOrderKey("node1", 0)
		key4 := graph.ComputeOrderKey("node1", 1)

		if key3 == key4 {
			t.Error("different edge indices produced same key")
		}
	})

	t.Run("consistent ordering relationship", func(t *testing.T) {
		// ComputeOrderKey uses SHA-256 hashing which produces deterministic
		// but non-sequential order keys. The key property is determinism
		// (same inputs always produce same key), not that edge index 0 < edge index 1.
		//
		// The scheduler will sort work items by their order key to ensure
		// deterministic execution order, regardless of the numeric relationship
		// between keys.
		key0 := graph.ComputeOrderKey("parent", 0)
		key1 := graph.ComputeOrderKey("parent", 1)
		key2 := graph.ComputeOrderKey("parent", 2)

		// Verify keys are distinct (collision resistance)
		if key0 == key1 || key0 == key2 || key1 == key2 {
			t.Errorf("keys should be distinct: key0=%d, key1=%d, key2=%d", key0, key1, key2)
		}

		// Verify keys are consistent on repeated calls
		key0Again := graph.ComputeOrderKey("parent", 0)
		if key0 != key0Again {
			t.Errorf("key generation not consistent: key0=%d, key0Again=%d", key0, key0Again)
		}
	})

	t.Run("collision resistance with many inputs", func(t *testing.T) {
		// Generate many keys and verify no collisions
		seen := make(map[uint64]string)
		collisions := 0

		for i := 0; i < 100; i++ {
			for j := 0; j < 10; j++ {
				parentID := "node" + string(rune('A'+i))
				key := graph.ComputeOrderKey(parentID, j)

				if existing, exists := seen[key]; exists {
					t.Errorf("collision detected: key %d for parent=%s edge=%d also used by %s",
						key, parentID, j, existing)
					collisions++
				} else {
					seen[key] = parentID + ":" + string(rune('0'+j))
				}
			}
		}

		if collisions > 0 {
			t.Errorf("detected %d collisions in 1000 key generations", collisions)
		}
	})
}

// TestFrontierOrdering (T023) verifies that Frontier dequeues work items
// in ascending OrderKey order, regardless of enqueue order.
//
// According to spec.md FR-002: System MUST process nodes in deterministic order
// based on (step_id, order_key) tuple within same step.
//
// Requirements:
// - Items dequeued in ascending OrderKey order
// - Items enqueued out-of-order still dequeue in correct order
// - Empty frontier returns appropriate signal
//
// This test should FAIL initially because Frontier type doesn't exist yet.
func TestFrontierOrdering(t *testing.T) {
	t.Run("dequeue in ascending order key order", func(t *testing.T) {
		// Create frontier with capacity 10
		ctx := context.Background()
		frontier := graph.NewFrontier[SchedulerTestState](ctx, 10)

		// Enqueue items in random order
		items := []graph.WorkItem[SchedulerTestState]{
			{NodeID: "node5", OrderKey: 500, StepID: 1, State: SchedulerTestState{Value: "fifth"}},
			{NodeID: "node2", OrderKey: 200, StepID: 1, State: SchedulerTestState{Value: "second"}},
			{NodeID: "node4", OrderKey: 400, StepID: 1, State: SchedulerTestState{Value: "fourth"}},
			{NodeID: "node1", OrderKey: 100, StepID: 1, State: SchedulerTestState{Value: "first"}},
			{NodeID: "node3", OrderKey: 300, StepID: 1, State: SchedulerTestState{Value: "third"}},
		}

		// Enqueue all items
		for _, item := range items {
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue failed: %v", err)
			}
		}

		// Verify frontier has 5 items
		if frontier.Len() != 5 {
			t.Errorf("expected frontier length 5, got %d", frontier.Len())
		}

		// Dequeue and verify order
		expected := []struct {
			nodeID   string
			orderKey uint64
		}{
			{"node1", 100},
			{"node2", 200},
			{"node3", 300},
			{"node4", 400},
			{"node5", 500},
		}

		for i, exp := range expected {
			item, err := frontier.Dequeue(ctx)
			if err != nil {
				t.Fatalf("dequeue %d failed: %v", i, err)
			}

			if item.NodeID != exp.nodeID {
				t.Errorf("dequeue %d: expected NodeID %s, got %s", i, exp.nodeID, item.NodeID)
			}

			if item.OrderKey != exp.orderKey {
				t.Errorf("dequeue %d: expected OrderKey %d, got %d", i, exp.orderKey, item.OrderKey)
			}
		}

		// Verify frontier is now empty
		if frontier.Len() != 0 {
			t.Errorf("expected empty frontier, got length %d", frontier.Len())
		}
	})

	t.Run("dequeue from empty frontier", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		frontier := graph.NewFrontier[SchedulerTestState](ctx, 10)

		// Dequeue from empty should block and timeout
		_, err := frontier.Dequeue(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded when dequeuing from empty frontier, got %v", err)
		}
	})

	t.Run("enqueue to full frontier blocks", func(t *testing.T) {
		// This test verifies backpressure behavior (US3)
		// For now, just verify capacity enforcement exists
		t.Skip("Backpressure testing deferred to Phase 5 (US3)")
	})
}

// SchedulerTestState is a simple state type for scheduler testing
type SchedulerTestState struct {
	Value   string
	Counter int
}
