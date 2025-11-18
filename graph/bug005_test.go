package graph

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// TestBUG005_HeapChannelDesync tests for the heap/channel desynchronization
// bug when context is cancelled during Enqueue after heap push but before
// channel send.
//
// Bug: BUG-005 - Context cancellation between heap.Push() and channel send
// causes orphaned items in heap with no corresponding notification.
//
// Symptom: frontier.Len() > 0 but no notifications available, causing deadlock
func TestBUG005_HeapChannelDesync(t *testing.T) {
	const (
		numWorkers    = 10
		numIterations = 1000
		queueCapacity = 200 // Large capacity to avoid backpressure
	)

	type state struct {
		counter int
	}

	// Track orphaned items (in heap but no notification sent)
	var orphanedCount atomic.Int32

	for iteration := 0; iteration < numIterations; iteration++ {
		// Frontier context - long-lived, represents frontier lifetime
		frontierCtx, frontierCancel := context.WithCancel(context.Background())
		defer frontierCancel()
		frontier := NewFrontier[state](frontierCtx, queueCapacity, "test-run", nil, emit.NewBufferedEmitter())

		// Worker context - can be cancelled independently (simulates worker pool cancellation)
		workerCtx, workerCancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup

		// Enqueuers: rapidly enqueue items
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					item := WorkItem[state]{
						StepID:       i,
						OrderKey:     uint64(workerID*100 + i),
						NodeID:       "test-node",
						State:        state{counter: i},
						Attempt:      0,
						ParentNodeID: "start",
						EdgeIndex:    0,
					}

					// Simulate worker context cancellation during enqueue
					// This simulates what happens when an error or completion is detected
					if i == 5 && workerID == 0 {
						// Cancel worker context while other workers are still enqueueing
						time.Sleep(1 * time.Millisecond)
						workerCancel()
					}

					err := frontier.Enqueue(workerCtx, item)
					if err != nil {
						// Context cancelled - this is expected
						// But the bug is: was item already added to heap?
						continue
					}
				}
			}(w)
		}

		// Dequeuers: consume items
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					_, err := frontier.Dequeue(workerCtx)
					if err != nil {
						// Worker context cancelled
						return
					}
				}
			}()
		}

		wg.Wait()

		// Check for orphaned items
		// If frontier.Len() > 0, items are stuck in heap with no notifications
		heapLen := frontier.Len()
		if heapLen > 0 {
			orphanedCount.Add(1)
			t.Logf("Iteration %d: Found %d orphaned items in heap (no notifications sent)", iteration, heapLen)
		}
	}

	orphaned := orphanedCount.Load()
	if orphaned > 0 {
		t.Errorf("BUG-005 detected: Found orphaned items in %d/%d iterations", orphaned, numIterations)
		t.Errorf("This indicates heap/channel desynchronization when context cancelled during Enqueue")
	}
}

// TestBUG005_WorkflowDeadlock tests for workflow deadlock caused by orphaned
// items in the frontier queue.
//
// This test simulates a real workflow scenario where context timeout during
// node execution causes items to be orphaned in the heap, leading to permanent
// workflow hang.
func TestBUG005_WorkflowDeadlock(t *testing.T) {
	type testState struct {
		step int
	}

	// Create a simple graph that should complete quickly
	reducer := func(prev, delta testState) testState {
		return delta
	}

	engine := New(reducer, store.NewMemStore[testState](), emit.NewBufferedEmitter(),
		WithMaxConcurrent(4),
		WithMaxSteps(100),
		WithQueueDepth(10),
	)

	// Node that completes quickly
	node1 := NodeFunc[testState](func(ctx context.Context, state testState) NodeResult[testState] {
		time.Sleep(10 * time.Millisecond)
		return NodeResult[testState]{
			Delta: testState{step: state.step + 1},
			Route: Next{To: "node2"},
		}
	})

	// Node that fans out to trigger concurrent enqueues
	node2 := NodeFunc[testState](func(ctx context.Context, state testState) NodeResult[testState] {
		time.Sleep(10 * time.Millisecond)
		return NodeResult[testState]{
			Delta: testState{step: state.step + 1},
			Route: Next{Many: []string{"node3", "node4", "node5"}},
		}
	})

	// Terminal nodes
	node3 := NodeFunc[testState](func(ctx context.Context, state testState) NodeResult[testState] {
		time.Sleep(5 * time.Millisecond)
		return NodeResult[testState]{
			Delta: testState{step: state.step + 1},
			Route: Next{Terminal: true},
		}
	})

	engine.Add("node1", node1)
	engine.Add("node2", node2)
	engine.Add("node3", node3)
	engine.Add("node4", node3)
	engine.Add("node5", node3)
	engine.StartAt("node1")

	// Run with aggressive timeout to trigger cancellation during enqueue
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should either complete successfully or return context.DeadlineExceeded
	// It should NOT hang forever
	done := make(chan struct{})
	go func() {
		_, err := engine.Run(ctx, "node1", testState{step: 0})
		if err != nil {
			t.Logf("Workflow error (expected due to timeout): %v", err)
		}
		close(done)
	}()

	// Wait with a safety timeout
	select {
	case <-done:
		// Success - workflow completed or failed gracefully
		t.Log("Workflow completed (either success or expected error)")
	case <-time.After(5 * time.Second):
		// This indicates BUG-005: workflow hung due to orphaned items
		t.Fatal("BUG-005 detected: Workflow hung for 5 seconds, likely due to orphaned items in frontier")
	}
}
