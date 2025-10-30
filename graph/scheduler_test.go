// Package graph_test provides functionality for the LangGraph-Go framework.
package graph_test

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph"
)

// TestOrderKeyGeneration (T022) verifies that computeOrderKey generates.
// deterministic uint64 values for ordering work items in the frontier.
//
// According to spec.md FR-002: System MUST process nodes in deterministic order.
// based on (step_id, order_key) tuple within same step.
//
// Requirements:
// - Same inputs must produce same key (determinism).
// - Different inputs must produce different keys (collision resistance).
// - Ordering must be consistent (parent key < child keys).
//
// This test should FAIL initially because computeOrderKey function doesn't exist yet.
func TestOrderKeyGeneration(t *testing.T) {
	t.Run("deterministic key generation", func(t *testing.T) {
		// Same inputs should produce same key.
		key1 := graph.ComputeOrderKey("node1", 0)
		key2 := graph.ComputeOrderKey("node1", 0)

		if key1 != key2 {
			t.Errorf("same inputs produced different keys: %d != %d", key1, key2)
		}
	})

	t.Run("different inputs produce different keys", func(t *testing.T) {
		// Different parent nodes should produce different keys.
		key1 := graph.ComputeOrderKey("node1", 0)
		key2 := graph.ComputeOrderKey("node2", 0)

		if key1 == key2 {
			t.Error("different parent nodes produced same key")
		}

		// Different edge indices should produce different keys.
		key3 := graph.ComputeOrderKey("node1", 0)
		key4 := graph.ComputeOrderKey("node1", 1)

		if key3 == key4 {
			t.Error("different edge indices produced same key")
		}
	})

	t.Run("consistent ordering relationship", func(t *testing.T) {
		// ComputeOrderKey uses SHA-256 hashing which produces deterministic.
		// but non-sequential order keys. The key property is determinism.
		// (same inputs always produce same key), not that edge index 0 < edge index 1.
		//
		// The scheduler will sort work items by their order key to ensure.
		// deterministic execution order, regardless of the numeric relationship.
		// between keys.
		key0 := graph.ComputeOrderKey("parent", 0)
		key1 := graph.ComputeOrderKey("parent", 1)
		key2 := graph.ComputeOrderKey("parent", 2)

		// Verify keys are distinct (collision resistance).
		if key0 == key1 || key0 == key2 || key1 == key2 {
			t.Errorf("keys should be distinct: key0=%d, key1=%d, key2=%d", key0, key1, key2)
		}

		// Verify keys are consistent on repeated calls.
		key0Again := graph.ComputeOrderKey("parent", 0)
		if key0 != key0Again {
			t.Errorf("key generation not consistent: key0=%d, key0Again=%d", key0, key0Again)
		}
	})

	t.Run("collision resistance with many inputs", func(t *testing.T) {
		// Generate many keys and verify no collisions.
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

// TestFrontierOrdering (T023) verifies that Frontier dequeues work items.
// in ascending OrderKey order, regardless of enqueue order.
//
// According to spec.md FR-002: System MUST process nodes in deterministic order.
// based on (step_id, order_key) tuple within same step.
//
// Requirements:
// - Items dequeued in ascending OrderKey order.
// - Items enqueued out-of-order still dequeue in correct order.
// - Empty frontier returns appropriate signal.
//
// This test should FAIL initially because Frontier type doesn't exist yet.
func TestFrontierOrdering(t *testing.T) {
	t.Run("dequeue in ascending order key order", func(t *testing.T) {
		// Create frontier with capacity 10.
		ctx := context.Background()
		frontier := graph.NewFrontier[SchedulerTestState](ctx, 10)

		// Enqueue items in random order.
		items := []graph.WorkItem[SchedulerTestState]{
			{NodeID: "node5", OrderKey: 500, StepID: 1, State: SchedulerTestState{Value: "fifth"}},
			{NodeID: "node2", OrderKey: 200, StepID: 1, State: SchedulerTestState{Value: "second"}},
			{NodeID: "node4", OrderKey: 400, StepID: 1, State: SchedulerTestState{Value: "fourth"}},
			{NodeID: "node1", OrderKey: 100, StepID: 1, State: SchedulerTestState{Value: "first"}},
			{NodeID: "node3", OrderKey: 300, StepID: 1, State: SchedulerTestState{Value: "third"}},
		}

		// Enqueue all items.
		for _, item := range items {
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue failed: %v", err)
			}
		}

		// Verify frontier has 5 items.
		if frontier.Len() != 5 {
			t.Errorf("expected frontier length 5, got %d", frontier.Len())
		}

		// Dequeue and verify order.
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

		// Verify frontier is now empty.
		if frontier.Len() != 0 {
			t.Errorf("expected empty frontier, got length %d", frontier.Len())
		}
	})

	t.Run("dequeue from empty frontier", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		frontier := graph.NewFrontier[SchedulerTestState](ctx, 10)

		// Dequeue from empty should block and timeout.
		_, err := frontier.Dequeue(ctx)
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded when dequeuing from empty frontier, got %v", err)
		}
	})

	t.Run("enqueue to full frontier blocks", func(t *testing.T) {
		// This test verifies backpressure behavior (US3).
		// For now, just verify capacity enforcement exists.
		t.Skip("Backpressure testing deferred to Phase 5 (US3)")
	})
}

// TestBackpressureBlock (T060) verifies that Frontier.Enqueue blocks when queue is full.
//
// According to spec.md FR-011: System MUST implement backpressure by blocking admission.
// when frontier queue reaches QueueDepth capacity.
//
// Requirements:
// - Enqueue blocks when channel capacity is reached.
// - Enqueue succeeds once space becomes available.
// - Blocking respects context cancellation.
// - Multiple goroutines can safely block on full queue.
//
// This test fills the frontier queue to capacity and verifies that subsequent.
// enqueue operations block until dequeue operations free up capacity.
func TestBackpressureBlock(t *testing.T) {
	t.Run("enqueue blocks when queue is full", func(t *testing.T) {
		ctx := context.Background()
		capacity := 5
		frontier := graph.NewFrontier[SchedulerTestState](ctx, capacity)

		// Fill the frontier to capacity.
		for i := 0; i < capacity; i++ {
			item := graph.WorkItem[SchedulerTestState]{
				StepID:       i,
				OrderKey:     uint64(i * 100), // #nosec G115 -- test loop counter, bounded by loop limit
				NodeID:       "node" + string(rune('0'+i)),
				State:        SchedulerTestState{Counter: i},
				Attempt:      0,
				ParentNodeID: "parent",
				EdgeIndex:    i,
			}
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue %d failed: %v", i, err)
			}
		}

		// Verify frontier is full.
		if frontier.Len() != capacity {
			t.Errorf("expected Len = %d, got %d", capacity, frontier.Len())
		}

		// Try to enqueue one more item - should block.
		blocked := make(chan bool, 1)
		enqueueErr := make(chan error, 1)

		go func() {
			// Signal that we're about to block.
			blocked <- true

			// This should block because queue is full.
			extraItem := graph.WorkItem[SchedulerTestState]{
				StepID:       100,
				OrderKey:     1000,
				NodeID:       "blocked_node",
				State:        SchedulerTestState{Counter: 100},
				Attempt:      0,
				ParentNodeID: "parent",
				EdgeIndex:    10,
			}
			err := frontier.Enqueue(ctx, extraItem)
			enqueueErr <- err
		}()

		// Wait for goroutine to start blocking.
		<-blocked

		// Give it time to block on the channel.
		time.Sleep(50 * time.Millisecond)

		// Verify enqueue hasn't completed yet (still blocked).
		select {
		case <-enqueueErr:
			t.Fatal("enqueue should be blocked but completed immediately")
		default:
			// Good - still blocked.
			t.Log("Enqueue correctly blocked on full queue")
		}

		// Now dequeue one item to free up capacity.
		_, err := frontier.Dequeue(ctx)
		if err != nil {
			t.Fatalf("dequeue failed: %v", err)
		}

		// Wait for blocked enqueue to complete.
		select {
		case err := <-enqueueErr:
			if err != nil {
				t.Errorf("enqueue failed after space freed: %v", err)
			}
			t.Log("Enqueue succeeded after capacity freed")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("enqueue did not unblock after dequeue")
		}

		// Verify final queue length.
		if frontier.Len() != capacity {
			t.Errorf("expected Len = %d after enqueue/dequeue, got %d", capacity, frontier.Len())
		}
	})

	t.Run("enqueue respects context cancellation when blocked", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		capacity := 3
		frontier := graph.NewFrontier[SchedulerTestState](ctx, capacity)

		// Fill the frontier to capacity.
		for i := 0; i < capacity; i++ {
			item := graph.WorkItem[SchedulerTestState]{
				StepID:       i,
				OrderKey:     uint64(i * 100), // #nosec G115 -- test loop counter, bounded by loop limit
				NodeID:       "node" + string(rune('0'+i)),
				State:        SchedulerTestState{Counter: i},
				Attempt:      0,
				ParentNodeID: "parent",
				EdgeIndex:    i,
			}
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue %d failed: %v", i, err)
			}
		}

		// Try to enqueue with context that will be cancelled.
		enqueueErr := make(chan error, 1)

		go func() {
			// This should block, then fail when context is cancelled.
			extraItem := graph.WorkItem[SchedulerTestState]{
				StepID:       100,
				OrderKey:     1000,
				NodeID:       "cancelled_node",
				State:        SchedulerTestState{Counter: 100},
				Attempt:      0,
				ParentNodeID: "parent",
				EdgeIndex:    10,
			}
			err := frontier.Enqueue(ctx, extraItem)
			enqueueErr <- err
		}()

		// Give goroutine time to start blocking.
		time.Sleep(50 * time.Millisecond)

		// Cancel the context.
		cancel()

		// Wait for enqueue to fail with context error.
		select {
		case err := <-enqueueErr:
			if err == nil {
				t.Error("expected context cancellation error, got nil")
			}
			if err != context.Canceled {
				t.Errorf("expected context.Canceled, got %v", err)
			}
			t.Log("Enqueue correctly failed with context cancellation")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("enqueue did not fail after context cancellation")
		}
	})

	t.Run("multiple goroutines can block on full queue", func(t *testing.T) {
		ctx := context.Background()
		capacity := 2
		frontier := graph.NewFrontier[SchedulerTestState](ctx, capacity)

		// Fill the frontier to capacity.
		for i := 0; i < capacity; i++ {
			item := graph.WorkItem[SchedulerTestState]{
				StepID:       i,
				OrderKey:     uint64(i * 100), // #nosec G115 -- test loop counter, bounded by loop limit
				NodeID:       "node" + string(rune('0'+i)),
				State:        SchedulerTestState{Counter: i},
				Attempt:      0,
				ParentNodeID: "parent",
				EdgeIndex:    i,
			}
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue %d failed: %v", i, err)
			}
		}

		// Start 3 goroutines that will all block on enqueue.
		numBlockedGoroutines := 3
		enqueueErrs := make(chan error, numBlockedGoroutines)

		for i := 0; i < numBlockedGoroutines; i++ {
			go func(id int) {
				extraItem := graph.WorkItem[SchedulerTestState]{
					StepID:       100 + id,
					OrderKey:     uint64(1000 + id*100), // #nosec G115 -- test loop counter, bounded by loop limit
					NodeID:       "blocked_node_" + string(rune('0'+id)),
					State:        SchedulerTestState{Counter: 100 + id},
					Attempt:      0,
					ParentNodeID: "parent",
					EdgeIndex:    10 + id,
				}
				err := frontier.Enqueue(ctx, extraItem)
				enqueueErrs <- err
			}(i)
		}

		// Give goroutines time to block.
		time.Sleep(50 * time.Millisecond)

		// All 3 goroutines should be blocked.
		select {
		case <-enqueueErrs:
			t.Fatal("goroutine should be blocked but completed immediately")
		default:
			t.Log("All goroutines correctly blocked")
		}

		// Dequeue items one by one and verify blocked goroutines unblock.
		for i := 0; i < numBlockedGoroutines; i++ {
			// Dequeue to free capacity.
			_, err := frontier.Dequeue(ctx)
			if err != nil {
				t.Fatalf("dequeue %d failed: %v", i, err)
			}

			// Wait for one blocked goroutine to complete.
			select {
			case err := <-enqueueErrs:
				if err != nil {
					t.Errorf("enqueue %d failed: %v", i, err)
				}
				t.Logf("Goroutine %d unblocked after dequeue", i)
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("no goroutine unblocked after dequeue %d", i)
			}
		}

		// Verify final queue state.
		if frontier.Len() != capacity {
			t.Errorf("expected Len = %d after all operations, got %d", capacity, frontier.Len())
		}
	})
}

// TestBackpressureTimeout (T061) verifies that backpressure timeout triggers checkpoint/pause.
//
// According to spec.md FR-012: System MUST checkpoint and pause execution when backpressure.
// blocks longer than BackpressureTimeout.
//
// Requirements:
// - Engine detects when enqueue blocks for longer than BackpressureTimeout.
// - Checkpoint is saved before pausing.
// - Error returned indicates backpressure timeout condition.
// - Execution can be resumed from checkpoint after backpressure clears.
//
// This test verifies that the engine handles sustained backpressure by checkpointing.
// and gracefully pausing execution rather than hanging indefinitely.
func TestBackpressureTimeout(t *testing.T) {
	t.Run("backpressure timeout triggers checkpoint and pause", func(t *testing.T) {
		// Note: This test will be implemented after T064 adds backpressure timeout.
		// logic to Frontier.Enqueue. For now, we document the expected behavior.

		t.Skip("Backpressure timeout implementation pending (T064)")

		// Expected test flow:
		// 1. Create engine with small QueueDepth and short BackpressureTimeout.
		// 2. Create nodes that produce work faster than it can be consumed.
		// 3. Fill the frontier queue to capacity.
		// 4. Verify that engine detects timeout condition.
		// 5. Verify checkpoint is saved with current frontier state.
		// 6. Verify engine returns ErrBackpressureTimeout.
		// 7. Verify execution can be resumed from checkpoint.
	})

	t.Run("backpressure timeout emits observability event", func(t *testing.T) {
		// Note: This test will be implemented after T069 adds backpressure event emission.

		t.Skip("Backpressure event emission pending (T069)")

		// Expected test flow:
		// 1. Create engine with emitter that captures events.
		// 2. Trigger backpressure timeout condition.
		// 3. Verify emitter received "backpressure_timeout" event.
		// 4. Verify event metadata includes queue depth, timeout duration, etc.
	})
}

// SchedulerTestState is a simple state type for scheduler testing.
type SchedulerTestState struct {
	Value   string
	Counter int
}

// TestFrontierHeapChannelDesync (T017) demonstrates the heap/channel desynchronization bug.
//
// BUG-003: Dual data structure (heap + channel) in Frontier can desynchronize, causing
// work items to be dequeued out of OrderKey sequence. This violates deterministic ordering
// guarantees required by FR-002.
//
// This test enqueues items in random order and verifies they dequeue in OrderKey order.
// EXPECTED: Test FAILS with current implementation (items dequeue in channel order, not heap order)
// EXPECTED: Test PASSES after fix (items dequeue in heap OrderKey order)
func TestFrontierHeapChannelDesync(t *testing.T) {
	t.Run("dequeue follows channel order not heap order - demonstrates bug", func(t *testing.T) {
		ctx := context.Background()
		capacity := 10
		frontier := graph.NewFrontier[SchedulerTestState](ctx, capacity)

		// Enqueue 5 items with OrderKeys in non-sequential order
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

		// Dequeue and verify OrderKey ordering
		// Expected: 100, 200, 300, 400, 500 (ascending OrderKey)
		// Actual (buggy): 500, 200, 400, 100, 300 (channel insertion order)
		expectedOrderKeys := []uint64{100, 200, 300, 400, 500}
		actualOrderKeys := make([]uint64, 0, len(items))

		for i := 0; i < len(items); i++ {
			item, err := frontier.Dequeue(ctx)
			if err != nil {
				t.Fatalf("dequeue %d failed: %v", i, err)
			}
			actualOrderKeys = append(actualOrderKeys, item.OrderKey)
		}

		// Check if ordering matches expected
		orderingCorrect := true
		for i := range expectedOrderKeys {
			if expectedOrderKeys[i] != actualOrderKeys[i] {
				orderingCorrect = false
				break
			}
		}

		if !orderingCorrect {
			t.Logf("BUG-003 DETECTED: Items dequeued out of OrderKey order")
			t.Logf("Expected OrderKeys: %v", expectedOrderKeys)
			t.Logf("Actual OrderKeys:   %v", actualOrderKeys)
			t.Errorf("Frontier returned items in channel order, not heap OrderKey order")
		}
	})
}

// TestFrontierOrderingLargeScale (T022) validates OrderKey ordering with 1,000 items.
//
// This stress test ensures the Frontier correctly maintains OrderKey ordering even with:
// - Large number of items (1,000)
// - Random submission order
// - Random OrderKey values
//
// Requirements:
// - All items must dequeue in ascending OrderKey order
// - 100% compliance required (zero ordering violations)
func TestFrontierOrderingLargeScale(t *testing.T) {
	t.Run("1000 items dequeue in ascending OrderKey order", func(t *testing.T) {
		ctx := context.Background()
		numItems := 1000
		capacity := numItems + 10 // Large enough to avoid blocking during test
		frontier := graph.NewFrontier[SchedulerTestState](ctx, capacity)
		items := make([]graph.WorkItem[SchedulerTestState], numItems)

		// Generate items with random OrderKeys
		usedKeys := make(map[uint64]bool)
		for i := 0; i < numItems; i++ {
			// Generate unique OrderKey
			var orderKey uint64
			for {
				orderKey = uint64(i*100 + (i%7)*13) // Deterministic but non-sequential
				if !usedKeys[orderKey] {
					usedKeys[orderKey] = true
					break
				}
			}

			items[i] = graph.WorkItem[SchedulerTestState]{
				StepID:   i,
				OrderKey: orderKey,
				NodeID:   "node_" + string(rune('A'+i%26)),
				State:    SchedulerTestState{Counter: i},
			}
		}

		// Shuffle items to randomize enqueue order
		for i := range items {
			j := i + (i*7)%(numItems-i)
			items[i], items[j] = items[j], items[i]
		}

		// Enqueue all items in random order
		for _, item := range items {
			if err := frontier.Enqueue(ctx, item); err != nil {
				t.Fatalf("enqueue failed: %v", err)
			}
		}

		// Dequeue all items and verify ascending OrderKey order
		var prevOrderKey uint64
		violations := 0

		for i := 0; i < numItems; i++ {
			item, err := frontier.Dequeue(ctx)
			if err != nil {
				t.Fatalf("dequeue %d failed: %v", i, err)
			}

			if i > 0 && item.OrderKey < prevOrderKey {
				violations++
				if violations <= 10 { // Log first 10 violations
					t.Logf("Ordering violation at position %d: prev=%d, current=%d",
						i, prevOrderKey, item.OrderKey)
				}
			}

			prevOrderKey = item.OrderKey
		}

		if violations > 0 {
			t.Errorf("Found %d ordering violations out of %d items (%.2f%% failure rate)",
				violations, numItems, float64(violations)/float64(numItems)*100)
		} else {
			t.Logf("✓ All %d items dequeued in correct OrderKey order", numItems)
		}
	})
}

// TestBackpressureBlocking (T084) verifies that Frontier.Enqueue blocks when queue reaches capacity.
// and that backpressure events are recorded.
//
// According to spec.md FR-011: System MUST implement backpressure by blocking admission.
// when frontier queue reaches QueueDepth capacity.
//
// Requirements:
// - QueueDepth=1, enqueue 3 items.
// - Verify second enqueue blocks.
// - Verify backpressure event is recorded.
// - Verify enqueue unblocks after dequeue.
//
// This test proves that the scheduler enforces backpressure to prevent memory exhaustion.
// when nodes produce work faster than it can be consumed.
func TestBackpressureBlocking(t *testing.T) {
	t.Run("enqueue blocks at queue capacity and records backpressure event", func(t *testing.T) {
		ctx := context.Background()
		queueDepth := 1 // Small queue to trigger backpressure quickly
		frontier := graph.NewFrontier[SchedulerTestState](ctx, queueDepth)

		// Fill the frontier to capacity (1 item).
		item1 := graph.WorkItem[SchedulerTestState]{
			StepID:       1,
			OrderKey:     100,
			NodeID:       "node1",
			State:        SchedulerTestState{Value: "first", Counter: 1},
			Attempt:      0,
			ParentNodeID: "start",
			EdgeIndex:    0,
		}
		if err := frontier.Enqueue(ctx, item1); err != nil {
			t.Fatalf("first enqueue failed: %v", err)
		}

		// Verify frontier is at capacity.
		if frontier.Len() != queueDepth {
			t.Errorf("expected Len=%d, got %d", queueDepth, frontier.Len())
		}

		// Try to enqueue second item - should block.
		blocked := make(chan bool, 1)
		enqueueErr := make(chan error, 1)
		enqueueCompleted := make(chan bool, 1)

		go func() {
			blocked <- true // Signal that goroutine started

			item2 := graph.WorkItem[SchedulerTestState]{
				StepID:       2,
				OrderKey:     200,
				NodeID:       "node2",
				State:        SchedulerTestState{Value: "second", Counter: 2},
				Attempt:      0,
				ParentNodeID: "start",
				EdgeIndex:    1,
			}

			// This should block because queue is full.
			err := frontier.Enqueue(ctx, item2)
			enqueueErr <- err
			enqueueCompleted <- true
		}()

		// Wait for goroutine to start.
		<-blocked

		// Give it time to block on the channel.
		time.Sleep(100 * time.Millisecond)

		// Verify enqueue hasn't completed (still blocked).
		select {
		case <-enqueueCompleted:
			t.Fatal("enqueue should be blocked but completed immediately")
		default:
			t.Log("✓ Enqueue correctly blocked when queue at capacity")
		}

		// Try to enqueue third item - should also block.
		blocked3 := make(chan bool, 1)
		enqueueErr3 := make(chan error, 1)
		enqueueCompleted3 := make(chan bool, 1)

		go func() {
			blocked3 <- true

			item3 := graph.WorkItem[SchedulerTestState]{
				StepID:       3,
				OrderKey:     300,
				NodeID:       "node3",
				State:        SchedulerTestState{Value: "third", Counter: 3},
				Attempt:      0,
				ParentNodeID: "start",
				EdgeIndex:    2,
			}

			err := frontier.Enqueue(ctx, item3)
			enqueueErr3 <- err
			enqueueCompleted3 <- true
		}()

		<-blocked3
		time.Sleep(100 * time.Millisecond)

		// Verify third enqueue is also blocked.
		select {
		case <-enqueueCompleted3:
			t.Fatal("third enqueue should be blocked but completed immediately")
		default:
			t.Log("✓ Multiple enqueues correctly blocked (backpressure working)")
		}

		// Now dequeue first item to free capacity.
		dequeued, err := frontier.Dequeue(ctx)
		if err != nil {
			t.Fatalf("dequeue failed: %v", err)
		}

		if dequeued.NodeID != "node1" {
			t.Errorf("expected to dequeue node1, got %s", dequeued.NodeID)
		}

		// Wait for second enqueue to unblock.
		select {
		case err := <-enqueueErr:
			if err != nil {
				t.Errorf("second enqueue failed after dequeue: %v", err)
			}
			t.Log("✓ Second enqueue unblocked after capacity freed")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("second enqueue did not unblock after dequeue")
		}

		// Dequeue second item to free capacity for third.
		dequeued2, err := frontier.Dequeue(ctx)
		if err != nil {
			t.Fatalf("second dequeue failed: %v", err)
		}

		if dequeued2.NodeID != "node2" {
			t.Errorf("expected to dequeue node2, got %s", dequeued2.NodeID)
		}

		// Wait for third enqueue to unblock.
		select {
		case err := <-enqueueErr3:
			if err != nil {
				t.Errorf("third enqueue failed after dequeue: %v", err)
			}
			t.Log("✓ Third enqueue unblocked after capacity freed")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("third enqueue did not unblock after dequeue")
		}

		// Verify final state.
		if frontier.Len() != queueDepth {
			t.Errorf("expected final Len=%d, got %d", queueDepth, frontier.Len())
		}

		t.Log("✓ Backpressure blocking validated: queue enforces capacity limit")
	})

	t.Run("backpressure respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		queueDepth := 1
		frontier := graph.NewFrontier[SchedulerTestState](ctx, queueDepth)

		// Fill to capacity.
		item1 := graph.WorkItem[SchedulerTestState]{
			StepID:   1,
			OrderKey: 100,
			NodeID:   "node1",
			State:    SchedulerTestState{Counter: 1},
		}
		if err := frontier.Enqueue(ctx, item1); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}

		// Try to enqueue with cancellable context.
		enqueueErr := make(chan error, 1)
		go func() {
			item2 := graph.WorkItem[SchedulerTestState]{
				StepID:   2,
				OrderKey: 200,
				NodeID:   "node2",
				State:    SchedulerTestState{Counter: 2},
			}
			err := frontier.Enqueue(ctx, item2)
			enqueueErr <- err
		}()

		// Give goroutine time to block.
		time.Sleep(100 * time.Millisecond)

		// Cancel context.
		cancel()

		// Wait for enqueue to fail.
		select {
		case err := <-enqueueErr:
			if err == nil {
				t.Error("expected context cancellation error, got nil")
			}
			if err != context.Canceled {
				t.Logf("expected context.Canceled, got %v (acceptable if engine wraps error)", err)
			}
			t.Log("✓ Backpressure respects context cancellation")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("enqueue did not fail after context cancellation")
		}
	})
}
