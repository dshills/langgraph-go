// Package main demonstrates usage of the LangGraph-Go framework.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// State represents workflow state for benchmarking.
type State struct {
	WorkflowType string
	Steps        int
	StartTime    time.Time
	Data         map[string]interface{}
}

// Reducer merges state updates.
func reducer(prev, delta State) State {
	if delta.WorkflowType != "" {
		prev.WorkflowType = delta.WorkflowType
	}
	if delta.Steps > 0 {
		prev.Steps = delta.Steps
	}
	if !delta.StartTime.IsZero() {
		prev.StartTime = delta.StartTime
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

func main() {
	fmt.Println("=== LangGraph-Go Performance Comparison ===")
	fmt.Println()

	// Scenario 1: Small High-Frequency Workflows.
	fmt.Println("📊 Scenario 1: High-Frequency Small Workflows")
	fmt.Println("Testing 1,000 executions of a 3-node workflow...")

	start := time.Now()
	smallWorkflowCount := 1000

	st := store.NewMemStore[State]()
	emitter := emit.NewNullEmitter() // Zero overhead emitter

	engine := graph.New(reducer, st, emitter, graph.WithMaxSteps(10))

	// 3-node workflow: start → process → finish.
	if err := engine.Add("start", graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		return graph.NodeResult[State]{
			Delta: State{Steps: 1, Data: map[string]interface{}{"step": "start"}},
			Route: graph.Goto("process"),
		}
	})); err != nil {
		log.Fatalf("Failed to add start node: %v", err)
	}

	if err := engine.Add("process", graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		return graph.NodeResult[State]{
			Delta: State{Steps: 2, Data: map[string]interface{}{"step": "process"}},
			Route: graph.Goto("finish"),
		}
	})); err != nil {
		log.Fatalf("Failed to add process node: %v", err)
	}

	if err := engine.Add("finish", graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		return graph.NodeResult[State]{
			Delta: State{Steps: 3, Data: map[string]interface{}{"step": "finish"}},
			Route: graph.Stop(),
		}
	})); err != nil {
		log.Fatalf("Failed to add finish node: %v", err)
	}

	if err := engine.StartAt("start"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Execute 1,000 workflows.
	for i := 0; i < smallWorkflowCount; i++ {
		runID := fmt.Sprintf("small-%d", i)
		initialState := State{
			WorkflowType: "small",
			StartTime:    time.Now(),
			Data:         make(map[string]interface{}),
		}

		_, err := engine.Run(context.Background(), runID, initialState)
		if err != nil {
			fmt.Printf("❌ Workflow %d failed: %v\n", i, err)
			os.Exit(1)
		}
	}

	elapsed := time.Since(start)
	throughput := float64(smallWorkflowCount) / elapsed.Seconds()
	avgLatency := elapsed / time.Duration(smallWorkflowCount)

	fmt.Printf("✅ Completed %d workflows\n", smallWorkflowCount)
	fmt.Printf("⏱️  Total time: %v\n", elapsed)
	fmt.Printf("🚀 Throughput: %.0f workflows/sec\n", throughput)
	fmt.Printf("📈 Average latency: %v\n", avgLatency)
	fmt.Println()

	// Scenario 2: Large Complex Workflow.
	fmt.Println("📊 Scenario 2: Large Complex Workflow")
	fmt.Println("Testing execution of a 50-node sequential workflow...")

	nodeCount := 50
	st2 := store.NewMemStore[State]()
	emitter2 := emit.NewNullEmitter()

	engine2 := graph.New(reducer, st2, emitter2, graph.WithMaxSteps(nodeCount+10))

	// Build 50-node sequential workflow.
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		nextNodeID := fmt.Sprintf("node%d", i+1)
		stepNum := i + 1

		if err := engine2.Add(nodeID, graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
			delta := State{
				Steps: stepNum,
				Data: map[string]interface{}{
					"node": nodeID,
					"step": stepNum,
				},
			}

			var route graph.Next
			if stepNum < nodeCount {
				route = graph.Goto(nextNodeID)
			} else {
				route = graph.Stop()
			}

			return graph.NodeResult[State]{
				Delta: delta,
				Route: route,
			}
		})); err != nil {
			log.Fatalf("Failed to add node %s: %v", nodeID, err)
		}
	}

	if err := engine2.StartAt("node0"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Execute large workflow.
	start = time.Now()
	runID := "large-workflow-001"
	initialState := State{
		WorkflowType: "large",
		StartTime:    time.Now(),
		Data:         make(map[string]interface{}),
	}

	result, err := engine2.Run(context.Background(), runID, initialState)
	if err != nil {
		fmt.Printf("❌ Large workflow failed: %v\n", err)
		os.Exit(1)
	}

	elapsed = time.Since(start)
	avgStepTime := elapsed / time.Duration(nodeCount)

	fmt.Printf("✅ Completed %d-node workflow\n", nodeCount)
	fmt.Printf("⏱️  Total time: %v\n", elapsed)
	fmt.Printf("📊 Steps executed: %d\n", result.Steps)
	fmt.Printf("🕐 Average step time: %v\n", avgStepTime)
	fmt.Println()

	// Scenario 3: Parallel Execution Performance.
	fmt.Println("📊 Scenario 3: Parallel Branch Execution")
	fmt.Println("Testing 4 parallel branches with fan-out/fan-in...")

	st3 := store.NewMemStore[State]()
	emitter3 := emit.NewNullEmitter()

	engine3 := graph.New(reducer, st3, emitter3, graph.WithMaxSteps(20))

	// Fan-out node.
	if err := engine3.Add("start", graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		return graph.NodeResult[State]{
			Delta: State{Steps: 1, Data: map[string]interface{}{"fanout": true}},
			Route: graph.Next{Many: []string{"branch1", "branch2", "branch3", "branch4"}},
		}
	})); err != nil {
		log.Fatalf("Failed to add start node: %v", err)
	}

	// 4 parallel branches.
	for i := 1; i <= 4; i++ {
		branchID := fmt.Sprintf("branch%d", i)
		if err := engine3.Add(branchID, graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
			// Simulate work.
			time.Sleep(50 * time.Millisecond)

			return graph.NodeResult[State]{
				Delta: State{
					Data: map[string]interface{}{
						branchID: fmt.Sprintf("completed-%d", i),
					},
				},
				Route: graph.Goto("join"),
			}
		})); err != nil {
			log.Fatalf("Failed to add branch node %s: %v", branchID, err)
		}
	}

	// Join node.
	if err := engine3.Add("join", graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		return graph.NodeResult[State]{
			Delta: State{Steps: state.Steps + 1, Data: map[string]interface{}{"joined": true}},
			Route: graph.Stop(),
		}
	})); err != nil {
		log.Fatalf("Failed to add join node: %v", err)
	}

	if err := engine3.StartAt("start"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Execute parallel workflow.
	start = time.Now()
	runID = "parallel-workflow-001"
	initialState = State{
		WorkflowType: "parallel",
		StartTime:    time.Now(),
		Data:         make(map[string]interface{}),
	}

	result, err = engine3.Run(context.Background(), runID, initialState)
	if err != nil {
		fmt.Printf("❌ Parallel workflow failed: %v\n", err)
		os.Exit(1)
	}

	elapsed = time.Since(start)
	sequentialTime := 4 * 50 * time.Millisecond
	parallelSpeedup := float64(sequentialTime) / float64(elapsed)

	fmt.Printf("✅ Completed parallel execution\n")
	fmt.Printf("⏱️  Total time: %v\n", elapsed)
	fmt.Printf("🚀 Parallel speedup: %.1fx (vs sequential)\n", parallelSpeedup)
	fmt.Printf("📊 Branches completed: 4\n")
	fmt.Println()

	// Scenario 4: Checkpoint Performance.
	fmt.Println("📊 Scenario 4: Checkpoint Save/Restore Performance")
	fmt.Println("Testing 100 checkpoint save and 100 load operations...")

	st4 := store.NewMemStore[State]()
	ctx := context.Background()

	checkpointCount := 100
	testState := State{
		WorkflowType: "checkpoint-test",
		Steps:        42,
		StartTime:    time.Now(),
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"nested": map[string]interface{}{
				"a": 1,
				"b": 2,
			},
		},
	}

	// Benchmark saves.
	start = time.Now()
	for i := 0; i < checkpointCount; i++ {
		cpID := fmt.Sprintf("checkpoint-%d", i)
		err := st4.SaveCheckpoint(ctx, cpID, testState, i)
		if err != nil {
			fmt.Printf("❌ SaveCheckpoint failed: %v\n", err)
			os.Exit(1)
		}
	}
	saveElapsed := time.Since(start)
	avgSaveTime := saveElapsed / time.Duration(checkpointCount)

	// Benchmark loads.
	start = time.Now()
	for i := 0; i < checkpointCount; i++ {
		cpID := fmt.Sprintf("checkpoint-%d", i)
		_, _, err := st4.LoadCheckpoint(ctx, cpID)
		if err != nil {
			fmt.Printf("❌ LoadCheckpoint failed: %v\n", err)
			os.Exit(1)
		}
	}
	loadElapsed := time.Since(start)
	avgLoadTime := loadElapsed / time.Duration(checkpointCount)

	fmt.Printf("✅ Save operations: %d\n", checkpointCount)
	fmt.Printf("⏱️  Total save time: %v\n", saveElapsed)
	fmt.Printf("📊 Average save time: %v\n", avgSaveTime)
	fmt.Println()
	fmt.Printf("✅ Load operations: %d\n", checkpointCount)
	fmt.Printf("⏱️  Total load time: %v\n", loadElapsed)
	fmt.Printf("📊 Average load time: %v\n", avgLoadTime)
	fmt.Println()

	// Summary.
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("📋 Performance Summary")
	fmt.Println()
	fmt.Printf("Small workflows:    %.0f workflows/sec (avg %v)\n", throughput, avgLatency)
	fmt.Printf("Large workflow:     %v total (%v/step)\n", elapsed, avgStepTime)
	fmt.Printf("Parallel execution: %.1fx speedup\n", parallelSpeedup)
	fmt.Printf("Checkpoint save:    %v average\n", avgSaveTime)
	fmt.Printf("Checkpoint load:    %v average\n", avgLoadTime)
	fmt.Println("\n✅ All performance tests completed successfully!")
}
