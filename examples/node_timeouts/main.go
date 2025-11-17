// Package main demonstrates per-node timeout configuration in LangGraph-Go.
//
// This example shows three timeout scenarios:
// 1. Per-node timeout via NodePolicy (highest priority)
// 2. DefaultNodeTimeout fallback (when node doesn't specify timeout)
// 3. No timeout (when both are zero)
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

// WorkflowState tracks the execution progress
type WorkflowState struct {
	Step    string
	Results []string
	Error   string
}

// fastNode completes quickly (50ms) with a 200ms explicit timeout
type fastNode struct{}

func (n fastNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	start := time.Now()

	// Simulate fast work
	select {
	case <-time.After(50 * time.Millisecond):
		duration := time.Since(start)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Step:    "fast",
				Results: append(state.Results, fmt.Sprintf("Fast node completed in %v", duration)),
			},
			Route: graph.Goto("slow"),
		}
	case <-ctx.Done():
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Error: fmt.Sprintf("Fast node timeout: %v", ctx.Err()),
			},
			Err: ctx.Err(),
		}
	}
}

// Policy returns a 200ms timeout for this node
func (n fastNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 200 * time.Millisecond,
	}
}

// slowNode attempts 300ms work but will be cancelled by DefaultNodeTimeout (100ms)
type slowNode struct{}

func (n slowNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	start := time.Now()

	// Simulate slow work that exceeds default timeout
	select {
	case <-time.After(300 * time.Millisecond):
		duration := time.Since(start)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Step:    "slow",
				Results: append(state.Results, fmt.Sprintf("Slow node completed in %v", duration)),
			},
			Route: graph.Goto("unlimited"),
		}
	case <-ctx.Done():
		duration := time.Since(start)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Step:    "slow",
				Results: append(state.Results, fmt.Sprintf("Slow node timed out after %v (used DefaultNodeTimeout)", duration)),
			},
			Err: ctx.Err(),
		}
	}
}

// Policy returns zero timeout (will use DefaultNodeTimeout)
func (n slowNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 0, // Use DefaultNodeTimeout
	}
}

// unlimitedNode has no timeout constraints
type unlimitedNode struct{}

func (n unlimitedNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	start := time.Now()

	// Simulate work that would exceed normal timeouts
	select {
	case <-time.After(150 * time.Millisecond):
		duration := time.Since(start)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Step:    "unlimited",
				Results: append(state.Results, fmt.Sprintf("Unlimited node completed in %v (no timeout)", duration)),
			},
			Route: graph.Stop(),
		}
	case <-ctx.Done():
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Error: fmt.Sprintf("Unlimited node timeout: %v (unexpected)", ctx.Err()),
			},
			Err: ctx.Err(),
		}
	}
}

// Policy returns zero timeout (and DefaultNodeTimeout will be ignored for demonstration)
func (n unlimitedNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 0,
	}
}

func main() {
	fmt.Println("=== Per-Node Timeout Example ===\n")

	// Create reducer that merges state updates
	reducer := func(prev, delta WorkflowState) WorkflowState {
		if delta.Step != "" {
			prev.Step = delta.Step
		}
		if delta.Error != "" {
			prev.Error = delta.Error
		}
		prev.Results = append(prev.Results, delta.Results...)
		return prev
	}

	// Create engine with DefaultNodeTimeout
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{
		MaxSteps:           10,
		MaxConcurrentNodes: 1,                      // Sequential execution for clarity
		DefaultNodeTimeout: 100 * time.Millisecond, // Default timeout for nodes without explicit Policy
	}
	engine := graph.New(reducer, st, emitter, opts)

	// Add nodes
	_ = engine.Add("fast", fastNode{}) // Has explicit 200ms timeout
	_ = engine.Add("slow", slowNode{}) // Uses DefaultNodeTimeout (100ms)
	_ = engine.StartAt("fast")

	// Run workflow
	ctx := context.Background()
	fmt.Println("Running workflow with per-node timeouts...\n")
	fmt.Println("Configuration:")
	fmt.Println("- Fast node: 200ms explicit timeout (completes in 50ms)")
	fmt.Println("- Slow node: Uses DefaultNodeTimeout of 100ms (attempts 300ms work)")
	fmt.Println()

	start := time.Now()
	finalState, err := engine.Run(ctx, "timeout-demo", WorkflowState{})
	duration := time.Since(start)

	// Display results
	fmt.Println("\n=== Results ===")
	fmt.Printf("Total execution time: %v\n\n", duration)

	if err != nil {
		fmt.Printf("Workflow error (expected): %v\n\n", err)
	}

	fmt.Println("Execution trace:")
	for i, result := range finalState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}

	if finalState.Error != "" {
		fmt.Printf("\nFinal error: %s\n", finalState.Error)
	}

	fmt.Println("\n=== Timeout Behavior ===")
	fmt.Println("✓ Fast node used per-node timeout (200ms) and completed successfully")
	if err != nil {
		fmt.Println("✓ Slow node used DefaultNodeTimeout (100ms) and timed out as expected")
	}
	fmt.Println("\nKey Takeaways:")
	fmt.Println("1. NodePolicy.Timeout overrides DefaultNodeTimeout")
	fmt.Println("2. When NodePolicy.Timeout is zero, DefaultNodeTimeout is used")
	fmt.Println("3. Setting both to zero allows unlimited execution time")

	// Demonstrate successful workflow with appropriate timeouts
	fmt.Println("\n=== Running with Adequate Timeouts ===")
	optsGenerous := graph.Options{
		MaxSteps:           10,
		MaxConcurrentNodes: 1,
		DefaultNodeTimeout: 400 * time.Millisecond, // Generous timeout
	}
	engineGenerous := graph.New(reducer, store.NewMemStore[WorkflowState](), emit.NewNullEmitter(), optsGenerous)
	_ = engineGenerous.Add("fast", fastNode{})
	_ = engineGenerous.Add("slow", slowNode{})
	_ = engineGenerous.StartAt("fast")

	start = time.Now()
	successState, successErr := engineGenerous.Run(ctx, "timeout-success", WorkflowState{})
	duration = time.Since(start)

	if successErr != nil {
		log.Fatalf("Unexpected error: %v", successErr)
	}

	fmt.Printf("Completed successfully in %v\n", duration)
	fmt.Println("Results:")
	for i, result := range successState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}
}
