package graph

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"
)

// Scheduler manages concurrent node execution with deterministic ordering

// WorkItem represents a schedulable unit of work in the execution frontier.
// Each WorkItem contains all the context needed to execute a node, including
// the node's input state, execution metadata, and provenance information for
// deterministic ordering.
//
// WorkItems are created when nodes produce routing decisions and are queued
// in the frontier for concurrent execution. The OrderKey ensures deterministic
// processing order even when nodes complete out of order.
type WorkItem[S any] struct {
	// StepID is the monotonically increasing step number in the run
	StepID int `json:"step_id"`

	// OrderKey is a deterministic sort key computed from hash(path_hash, edge_index).
	// This ensures consistent execution order across replays.
	OrderKey uint64 `json:"order_key"`

	// NodeID identifies the node to execute for this work item
	NodeID string `json:"node_id"`

	// State is the snapshot of state for this work item's execution
	State S `json:"state"`

	// Attempt is the retry counter (0 for first execution, 1+ for retries)
	Attempt int `json:"attempt"`

	// ParentNodeID is the node that spawned this work item, used for path hash computation
	ParentNodeID string `json:"parent_node_id"`

	// EdgeIndex is the index of the edge taken from parent, used for deterministic ordering
	EdgeIndex int `json:"edge_index"`
}

// ComputeOrderKey generates a deterministic sort key from the parent node ID and edge index.
// This key ensures consistent execution ordering across replays, regardless of runtime
// scheduling variations or goroutine completion order.
//
// The key is computed as follows:
//  1. Hash the concatenation of parentNodeID + edgeIndex (as 4-byte big-endian int)
//  2. Extract the first 8 bytes of the SHA-256 hash
//  3. Interpret as uint64 (big-endian)
//
// This approach guarantees:
//   - Determinism: Same inputs always produce the same order key
//   - Low collision probability: SHA-256 provides cryptographic collision resistance
//   - Total ordering: uint64 keys can be consistently sorted
//   - Path awareness: Keys capture the execution path context
//
// The order key enables the frontier queue to maintain deterministic work item
// priority even when nodes execute concurrently and complete out of order.
func ComputeOrderKey(parentNodeID string, edgeIndex int) uint64 {
	return computeOrderKey(parentNodeID, edgeIndex)
}

// computeOrderKey is the internal implementation (lowercase for package-internal use)
func computeOrderKey(parentNodeID string, edgeIndex int) uint64 {
	h := sha256.New()

	// Write parent node ID
	h.Write([]byte(parentNodeID))

	// Write edge index as 4-byte big-endian integer
	edgeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(edgeBytes, uint32(edgeIndex))
	h.Write(edgeBytes)

	// Get hash and extract first 8 bytes as uint64
	hashBytes := h.Sum(nil)
	orderKey := binary.BigEndian.Uint64(hashBytes[:8])

	return orderKey
}

// workHeap implements heap.Interface for priority queue ordering by OrderKey.
// This internal type is used by Frontier to maintain sorted order of work items.
type workHeap[S any] []WorkItem[S]

func (h workHeap[S]) Len() int { return len(h) }

func (h workHeap[S]) Less(i, j int) bool {
	// Min-heap: smaller OrderKey has higher priority
	return h[i].OrderKey < h[j].OrderKey
}

func (h workHeap[S]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *workHeap[S]) Push(x interface{}) {
	*h = append(*h, x.(WorkItem[S]))
}

func (h *workHeap[S]) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// Frontier manages the work queue for concurrent graph execution with bounded capacity
// and deterministic ordering. It combines a priority queue (heap) for ordering with a
// buffered channel for bounded queue depth and backpressure.
//
// The Frontier ensures that work items are dequeued in deterministic order (by OrderKey)
// even when they are enqueued concurrently from multiple goroutines. This is critical
// for deterministic replay of graph executions.
//
// The bounded channel provides backpressure: when the queue is full, Enqueue will block
// until capacity becomes available or the context is cancelled. This prevents unbounded
// memory growth when nodes produce work faster than it can be consumed.
//
// Thread-safety: All methods are safe for concurrent use by multiple goroutines.
type Frontier[S any] struct {
	heap     workHeap[S]      // Priority queue for deterministic ordering
	queue    chan WorkItem[S] // Buffered channel for bounded capacity
	capacity int              // Maximum queue depth
	ctx      context.Context  // Context for cancellation
	mu       sync.Mutex       // Protects heap and len operations
}

// NewFrontier creates a new Frontier with the specified capacity and context.
// The capacity determines the maximum number of work items that can be queued.
// The context is used for cancellation propagation.
func NewFrontier[S any](ctx context.Context, capacity int) *Frontier[S] {
	f := &Frontier[S]{
		heap:     make(workHeap[S], 0),
		queue:    make(chan WorkItem[S], capacity),
		capacity: capacity,
		ctx:      ctx,
	}
	heap.Init(&f.heap)
	return f
}

// Enqueue adds a work item to the frontier queue. The item is first added to the
// internal heap (sorted by OrderKey), then sent to the buffered channel.
//
// If the channel is full, this method blocks until:
//   - Space becomes available in the channel (backpressure), or
//   - The context is cancelled
//
// Returns an error if the context is cancelled before the item can be enqueued.
// This blocking behavior provides natural backpressure when nodes produce work
// faster than the system can process it.
func (f *Frontier[S]) Enqueue(ctx context.Context, item WorkItem[S]) error {
	// Check context first for fast failure
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Add to heap under lock
	f.mu.Lock()
	heap.Push(&f.heap, item)
	f.mu.Unlock()

	// Send to channel (may block if full)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case f.queue <- item:
		return nil
	}
}

// Dequeue retrieves the work item with the smallest OrderKey from the frontier.
// This method blocks until:
//   - A work item becomes available, or
//   - The context is cancelled
//
// Returns the work item with the minimum OrderKey and nil error on success.
// Returns a zero-value work item and context error if the context is cancelled.
//
// The deterministic ordering guarantee is maintained by popping from the heap,
// which keeps items sorted by OrderKey.
func (f *Frontier[S]) Dequeue(ctx context.Context) (WorkItem[S], error) {
	var zero WorkItem[S]

	// Check context first for fast failure
	if ctx.Err() != nil {
		return zero, ctx.Err()
	}

	// Receive from channel (may block if empty)
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case <-f.queue:
		// Pop min OrderKey item from heap under lock
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.heap.Len() == 0 {
			// Should not happen if queue and heap are synchronized
			return zero, context.Canceled
		}

		item := heap.Pop(&f.heap).(WorkItem[S])
		return item, nil
	}
}

// Len returns the current number of work items in the frontier queue.
// This method is thread-safe and can be called concurrently with Enqueue/Dequeue.
func (f *Frontier[S]) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heap.Len()
}
