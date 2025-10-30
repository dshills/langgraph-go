package graph

import (
	"context"
	"fmt"
	"testing"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// T196: Benchmark for large workflow (100+ nodes)
//
// Tests performance characteristics of workflows with many nodes.
// Validates that the framework can handle complex workflows without
// significant performance degradation.
//
// Performance Goals (from plan.md):
// - Support 100+ node workflows without performance degradation
// - <100ms checkpoint save/restore overhead
// - Parallel branch execution with <10ms coordination overhead

type BenchState struct {
	Counter int
	Data    map[string]interface{}
}

func benchReducer(prev, delta BenchState) BenchState {
	if delta.Counter > 0 {
		prev.Counter = delta.Counter
	}
	if delta.Data != nil {
		if prev.Data == nil {
			prev.Data = make(map[string]interface{})
		}
		for k, v := range delta.Data {
			prev.Data[k] = v
		}
	}
	return prev
}

// BenchmarkLargeWorkflow tests performance with 100+ nodes
func BenchmarkLargeWorkflow(b *testing.B) {
	nodeCount := 100

	// Create workflow with 100 sequential nodes
	st := store.NewMemStore[BenchState]()
	emitter := emit.NewNullEmitter()
	opts := Options{MaxSteps: nodeCount + 10}
	engine := New(benchReducer, st, emitter, opts)

	// Add 100 nodes in sequence
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		nextNodeID := fmt.Sprintf("node%d", i+1)

		currentStep := i + 1
		if err := engine.Add(nodeID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
			delta := BenchState{
				Counter: currentStep,
				Data: map[string]interface{}{
					"step": currentStep,
				},
			}

			var route Next
			if currentStep < nodeCount {
				route = Goto(nextNodeID)
			} else {
				route = Stop()
			}

			return NodeResult[BenchState]{
				Delta: delta,
				Route: route,
			}
		})); err != nil {
			b.Fatalf("Failed to add node node: %v", err)
		}
	}

	if err := engine.StartAt("node0"); err != nil {
		b.Fatalf("Failed to set start node to node0: %v", err)
	}

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("bench-large-%d", i)
		initialState := BenchState{
			Counter: 0,
			Data:    make(map[string]interface{}),
		}

		_, err := engine.Run(context.Background(), runID, initialState)
		if err != nil {
			b.Fatalf("Workflow failed: %v", err)
		}
	}
	b.StopTimer()

	// Report performance metrics
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	msPerOp := b.Elapsed().Seconds() * 1000 / float64(b.N)
	b.ReportMetric(opsPerSec, "workflows/sec")
	b.ReportMetric(msPerOp, "ms/workflow")
	b.ReportMetric(float64(nodeCount), "nodes")
}

// T197: Benchmark for high-frequency small workflows
//
// Tests performance characteristics of frequent execution of small workflows.
// Validates that the framework can handle high-frequency workflow execution
// with minimal overhead.

// BenchmarkSmallWorkflowHighFrequency tests many small workflows
func BenchmarkSmallWorkflowHighFrequency(b *testing.B) {
	// Create simple 3-node workflow
	st := store.NewMemStore[BenchState]()
	emitter := emit.NewNullEmitter()
	opts := Options{MaxSteps: 10}
	engine := New(benchReducer, st, emitter, opts)

	if err := engine.Add("start", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 1, Data: map[string]interface{}{"step": "start"}},
			Route: Goto("process"),
		}
	})); err != nil {
		b.Fatalf("Failed to add start node: %v", err)
	}

	if err := engine.Add("process", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 2, Data: map[string]interface{}{"step": "process"}},
			Route: Goto("finish"),
		}
	})); err != nil {
		b.Fatalf("Failed to add process node: %v", err)
	}

	if err := engine.Add("finish", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 3, Data: map[string]interface{}{"step": "finish"}},
			Route: Stop(),
		}
	})); err != nil {
		b.Fatalf("Failed to add finish node: %v", err)
	}

	if err := engine.StartAt("start"); err != nil {
		b.Fatalf("Failed to set start node to start: %v", err)
	}

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("bench-small-%d", i)
		initialState := BenchState{
			Counter: 0,
			Data:    make(map[string]interface{}),
		}

		_, err := engine.Run(context.Background(), runID, initialState)
		if err != nil {
			b.Fatalf("Workflow failed: %v", err)
		}
	}
	b.StopTimer()

	// Report performance metrics
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
	b.ReportMetric(opsPerSec, "workflows/sec")
	b.ReportMetric(usPerOp, "μs/workflow")
	b.ReportMetric(3.0, "nodes")
}

// BenchmarkCheckpointOverhead tests checkpoint save/restore performance
func BenchmarkCheckpointOverhead(b *testing.B) {
	st := store.NewMemStore[BenchState]()
	ctx := context.Background()

	state := BenchState{
		Counter: 42,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
			"nested": map[string]interface{}{
				"a": 1,
				"b": 2,
			},
		},
	}

	b.Run("SaveCheckpoint", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cpID := fmt.Sprintf("checkpoint-%d", i)
			err := st.SaveCheckpoint(ctx, cpID, state, i)
			if err != nil {
				b.Fatalf("SaveCheckpoint failed: %v", err)
			}
		}
		b.StopTimer()

		// Report metrics
		usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
		b.ReportMetric(usPerOp, "μs/save")
	})

	// Save a checkpoint for load testing
	cpID := "load-test-checkpoint"
	_ = st.SaveCheckpoint(ctx, cpID, state, 1)

	b.Run("LoadCheckpoint", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := st.LoadCheckpoint(ctx, cpID)
			if err != nil {
				b.Fatalf("LoadCheckpoint failed: %v", err)
			}
		}
		b.StopTimer()

		// Report metrics
		usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
		b.ReportMetric(usPerOp, "μs/load")
	})
}

// BenchmarkParallelBranchCoordination tests parallel execution overhead
func BenchmarkParallelBranchCoordination(b *testing.B) {
	st := store.NewMemStore[BenchState]()
	emitter := emit.NewNullEmitter()
	opts := Options{MaxSteps: 20}
	engine := New(benchReducer, st, emitter, opts)

	// Fan-out node
	if err := engine.Add("start", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 1},
			Route: Next{Many: []string{"branch1", "branch2", "branch3", "branch4"}},
		}
	})); err != nil {
		b.Fatalf("Failed to add start node: %v", err)
	}

	// 4 parallel branches
	for i := 1; i <= 4; i++ {
		branchID := fmt.Sprintf("branch%d", i)
		if err := engine.Add(branchID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
			return NodeResult[BenchState]{
				Delta: BenchState{
					Data: map[string]interface{}{
						branchID: true,
					},
				},
				Route: Goto("join"),
			}
		})); err != nil {
			b.Fatalf("Failed to add node node: %v", err)
		}
	}

	// Join node
	if err := engine.Add("join", NodeFunc[BenchState](func(_ context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: state.Counter + 1},
			Route: Stop(),
		}
	})); err != nil {
		b.Fatalf("Failed to add join node: %v", err)
	}

	if err := engine.StartAt("start"); err != nil {
		b.Fatalf("Failed to set start node to start: %v", err)
	}

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("bench-parallel-%d", i)
		initialState := BenchState{
			Counter: 0,
			Data:    make(map[string]interface{}),
		}

		_, err := engine.Run(context.Background(), runID, initialState)
		if err != nil {
			b.Fatalf("Workflow failed: %v", err)
		}
	}
	b.StopTimer()

	// Report metrics
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
	b.ReportMetric(opsPerSec, "workflows/sec")
	b.ReportMetric(usPerOp, "μs/workflow")
	b.ReportMetric(4.0, "parallel_branches")
}

// T198: Profile memory usage with pprof
//
// Run with:
//   go test -bench=. -benchmem -memprofile=mem.prof -cpuprofile=cpu.prof ./graph
//   go tool pprof mem.prof
//   go tool pprof cpu.prof
//
// Memory profiling enabled automatically when -benchmem flag is used.
// The benchmarks above will report memory allocations:
//   - allocs/op: Number of allocations per operation
//   - B/op: Bytes allocated per operation
//
// Example analysis commands:
//   go tool pprof -http=:8080 mem.prof  # Open in browser
//   go tool pprof -top mem.prof         # Show top memory consumers
//   go tool pprof -list=NodeFunc mem.prof  # Show line-by-line allocations

// BenchmarkStateAllocation tests memory allocation patterns
func BenchmarkStateAllocation(b *testing.B) {
	st := store.NewMemStore[BenchState]()
	emitter := emit.NewNullEmitter()
	opts := Options{MaxSteps: 10}
	engine := New(benchReducer, st, emitter, opts)

	if err := engine.Add("process", NodeFunc[BenchState](func(_ context.Context, state BenchState) NodeResult[BenchState] {
		// Create new data map (allocation test)
		newData := make(map[string]interface{})
		newData["key"] = "value"
		newData["counter"] = state.Counter + 1

		return NodeResult[BenchState]{
			Delta: BenchState{
				Counter: state.Counter + 1,
				Data:    newData,
			},
			Route: Stop(),
		}
	})); err != nil {
		b.Fatalf("Failed to add process node: %v", err)
	}

	if err := engine.StartAt("process"); err != nil {
		b.Fatalf("Failed to set start node to process: %v", err)
	}

	// Run benchmark with memory reporting
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("bench-alloc-%d", i)
		initialState := BenchState{
			Counter: 0,
			Data:    make(map[string]interface{}),
		}

		_, err := engine.Run(context.Background(), runID, initialState)
		if err != nil {
			b.Fatalf("Workflow failed: %v", err)
		}
	}
	b.StopTimer()
}

// BenchmarkConcurrentExecution (T027) benchmarks concurrent execution
// with varying numbers of parallel nodes to measure scheduler overhead.
//
// According to spec.md SC-001: Graphs with 5 independent nodes complete
// execution in 20% of sequential time (demonstrating parallelism).
//
// Requirements:
// - Benchmark with 1, 10, 50, 100 concurrent nodes
// - Measure scheduler overhead
// - Compare concurrent vs sequential execution time
//
// This benchmark should show performance improvements with concurrency.
func BenchmarkConcurrentExecution(b *testing.B) {
	b.Skip("Implementation pending - T035 will implement concurrent execution")

	concurrencyLevels := []int{1, 10, 50, 100}

	for _, nodeCount := range concurrencyLevels {
		b.Run(fmt.Sprintf("nodes=%d", nodeCount), func(b *testing.B) {
			st := store.NewMemStore[BenchState]()
			emitter := emit.NewNullEmitter()

			// Configure for concurrent execution
			opts := Options{
				MaxSteps:           nodeCount + 10,
				MaxConcurrentNodes: nodeCount, // Allow all to run concurrently
			}
			engine := New(benchReducer, st, emitter, opts)

			// Create N independent nodes
			for i := 0; i < nodeCount; i++ {
				nodeID := fmt.Sprintf("node%d", i)
				counter := i + 1

				if err := engine.Add(nodeID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
					// Minimal work - just return delta
					return NodeResult[BenchState]{
						Delta: BenchState{Counter: counter},
						Route: Stop(),
					}
				})); err != nil {
					b.Fatalf("Failed to add node node: %v", err)
				}
			}

			// Fan-out to all nodes
			nodeIDs := make([]string, nodeCount)
			for i := 0; i < nodeCount; i++ {
				nodeIDs[i] = fmt.Sprintf("node%d", i)
			}

			if err := engine.Add("start", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
				return NodeResult[BenchState]{
					Route: Next{Many: nodeIDs},
				}
			})); err != nil {
				b.Fatalf("Failed to add start node: %v", err)
			}
			if err := engine.StartAt("start"); err != nil {
				b.Fatalf("Failed to set start node to start: %v", err)
			}

			// Run benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				runID := fmt.Sprintf("bench-concurrent-%d-%d", nodeCount, i)
				initialState := BenchState{
					Counter: 0,
					Data:    make(map[string]interface{}),
				}

				_, err := engine.Run(context.Background(), runID, initialState)
				if err != nil {
					b.Fatalf("Run failed: %v", err)
				}
			}
			b.StopTimer()

			// Report metrics
			opsPerSec := float64(b.N) / b.Elapsed().Seconds()
			usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
			b.ReportMetric(opsPerSec, "runs/sec")
			b.ReportMetric(usPerOp, "μs/run")
			b.ReportMetric(float64(nodeCount), "nodes")

			// Calculate throughput (nodes processed per second)
			nodesPerSec := float64(b.N*nodeCount) / b.Elapsed().Seconds()
			b.ReportMetric(nodesPerSec, "nodes/sec")
		})
	}
}

// BenchmarkSequentialVsConcurrent compares sequential vs concurrent execution
// to measure the parallel speedup factor.
//
// This benchmark demonstrates the performance benefit of concurrent execution.
func BenchmarkSequentialVsConcurrent(b *testing.B) {
	b.Skip("Implementation pending - T035 will implement concurrent execution")

	nodeCount := 10

	b.Run("sequential", func(b *testing.B) {
		st := store.NewMemStore[BenchState]()
		emitter := emit.NewNullEmitter()

		opts := Options{
			MaxSteps:           nodeCount + 10,
			MaxConcurrentNodes: 1, // Force sequential execution
		}
		engine := New(benchReducer, st, emitter, opts)

		// Create chain of sequential nodes
		for i := 0; i < nodeCount; i++ {
			nodeID := fmt.Sprintf("node%d", i)
			nextNodeID := fmt.Sprintf("node%d", i+1)
			counter := i + 1

			isLast := (i == nodeCount-1)

			if err := engine.Add(nodeID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
				var route Next
				if isLast {
					route = Stop()
				} else {
					route = Goto(nextNodeID)
				}

				return NodeResult[BenchState]{
					Delta: BenchState{Counter: counter},
					Route: route,
				}
			})); err != nil {
				b.Fatalf("Failed to add node node: %v", err)
			}
		}

		if err := engine.StartAt("node0"); err != nil {
			b.Fatalf("Failed to set start node to node0: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runID := fmt.Sprintf("bench-seq-%d", i)
			_, err := engine.Run(context.Background(), runID, BenchState{})
			if err != nil {
				b.Fatalf("Run failed: %v", err)
			}
		}
		b.StopTimer()

		usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
		b.ReportMetric(usPerOp, "μs/run")
	})

	b.Run("concurrent", func(b *testing.B) {
		st := store.NewMemStore[BenchState]()
		emitter := emit.NewNullEmitter()

		opts := Options{
			MaxSteps:           nodeCount + 10,
			MaxConcurrentNodes: nodeCount, // Allow full concurrency
		}
		engine := New(benchReducer, st, emitter, opts)

		// Create N independent nodes
		nodeIDs := make([]string, nodeCount)
		for i := 0; i < nodeCount; i++ {
			nodeID := fmt.Sprintf("node%d", i)
			nodeIDs[i] = nodeID
			counter := i + 1

			if err := engine.Add(nodeID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
				return NodeResult[BenchState]{
					Delta: BenchState{Counter: counter},
					Route: Stop(),
				}
			})); err != nil {
				b.Fatalf("Failed to add node node: %v", err)
			}
		}

		// Fan-out to all nodes
		if err := engine.Add("start", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
			return NodeResult[BenchState]{
				Route: Next{Many: nodeIDs},
			}
		})); err != nil {
			b.Fatalf("Failed to add start node: %v", err)
		}
		if err := engine.StartAt("start"); err != nil {
			b.Fatalf("Failed to set start node to start: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			runID := fmt.Sprintf("bench-conc-%d", i)
			_, err := engine.Run(context.Background(), runID, BenchState{})
			if err != nil {
				b.Fatalf("Run failed: %v", err)
			}
		}
		b.StopTimer()

		usPerOp := b.Elapsed().Seconds() * 1000000 / float64(b.N)
		b.ReportMetric(usPerOp, "μs/run")
	})
}

// BenchmarkSchedulerOverhead measures the overhead of the scheduler
// by comparing workflow execution time with minimal node work.
func BenchmarkSchedulerOverhead(b *testing.B) {
	b.Skip("Implementation pending - T035 will implement scheduler")

	b.Run("1_node", func(b *testing.B) {
		benchSchedulerOverheadWithNodes(b, 1)
	})

	b.Run("10_nodes", func(b *testing.B) {
		benchSchedulerOverheadWithNodes(b, 10)
	})

	b.Run("100_nodes", func(b *testing.B) {
		benchSchedulerOverheadWithNodes(b, 100)
	})
}

func benchSchedulerOverheadWithNodes(b *testing.B, nodeCount int) {
	st := store.NewMemStore[BenchState]()
	emitter := emit.NewNullEmitter()

	opts := Options{
		MaxSteps:           nodeCount + 10,
		MaxConcurrentNodes: nodeCount,
	}
	engine := New(benchReducer, st, emitter, opts)

	// Create nodes with minimal work (just counter increment)
	nodeIDs := make([]string, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		nodeIDs[i] = nodeID

		if err := engine.Add(nodeID, NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
			return NodeResult[BenchState]{
				Delta: BenchState{Counter: 1},
				Route: Stop(),
			}
		})); err != nil {
			b.Fatalf("Failed to add node node: %v", err)
		}
	}

	if err := engine.Add("start", NodeFunc[BenchState](func(_ context.Context, _ BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Route: Next{Many: nodeIDs},
		}
	})); err != nil {
		b.Fatalf("Failed to add start node: %v", err)
	}
	if err := engine.StartAt("start"); err != nil {
		b.Fatalf("Failed to set start node to start: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("bench-overhead-%d", i)
		_, err := engine.Run(context.Background(), runID, BenchState{})
		if err != nil {
			b.Fatalf("Run failed: %v", err)
		}
	}
	b.StopTimer()

	// Report overhead per node
	nsPerNode := b.Elapsed().Nanoseconds() / int64(b.N*nodeCount)
	b.ReportMetric(float64(nsPerNode), "ns/node")
}
