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
		engine.Add(nodeID, NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
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
		}))
	}

	engine.StartAt("node0")

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

	engine.Add("start", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 1, Data: map[string]interface{}{"step": "start"}},
			Route: Goto("process"),
		}
	}))

	engine.Add("process", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 2, Data: map[string]interface{}{"step": "process"}},
			Route: Goto("finish"),
		}
	}))

	engine.Add("finish", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 3, Data: map[string]interface{}{"step": "finish"}},
			Route: Stop(),
		}
	}))

	engine.StartAt("start")

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
	engine.Add("start", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: 1},
			Route: Next{Many: []string{"branch1", "branch2", "branch3", "branch4"}},
		}
	}))

	// 4 parallel branches
	for i := 1; i <= 4; i++ {
		branchID := fmt.Sprintf("branch%d", i)
		engine.Add(branchID, NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
			return NodeResult[BenchState]{
				Delta: BenchState{
					Data: map[string]interface{}{
						branchID: true,
					},
				},
				Route: Goto("join"),
			}
		}))
	}

	// Join node
	engine.Add("join", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
		return NodeResult[BenchState]{
			Delta: BenchState{Counter: state.Counter + 1},
			Route: Stop(),
		}
	}))

	engine.StartAt("start")

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

	engine.Add("process", NodeFunc[BenchState](func(ctx context.Context, state BenchState) NodeResult[BenchState] {
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
	}))

	engine.StartAt("process")

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
