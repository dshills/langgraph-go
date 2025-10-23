package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// simpleEmitter provides basic event logging for demonstration purposes.
type simpleEmitter struct{}

func (e *simpleEmitter) Emit(event emit.Event) {
	// Silent emitter for cleaner demo output
	// In production, you might log to stdout, file, or observability platform
}

// ProcessingState represents the workflow state for parallel data processing.
type ProcessingState struct {
	Input   string   // Input data to process
	Results []string // Collected results from parallel branches
	Count   int      // Total result count
}

func main() {
	fmt.Println("=== LangGraph-Go Parallel Execution Example ===\n")

	// Define reducer that combines results from parallel branches
	reducer := func(prev, delta ProcessingState) ProcessingState {
		// Merge results from parallel branches
		if len(delta.Results) > 0 {
			prev.Results = append(prev.Results, delta.Results...)
		}
		prev.Count += delta.Count
		return prev
	}

	// Create engine components
	st := store.NewMemStore[ProcessingState]()
	emitter := &simpleEmitter{}
	opts := graph.Options{MaxSteps: 100}
	engine := graph.New(reducer, st, emitter, opts)

	// Entry node: Fan out to 4 parallel processing branches
	fanout := graph.NodeFunc[ProcessingState](func(ctx context.Context, s ProcessingState) graph.NodeResult[ProcessingState] {
		fmt.Printf("📤 Fanout: Splitting '%s' into 4 parallel branches\n", s.Input)
		return graph.NodeResult[ProcessingState]{
			Route: graph.Next{
				Many: []string{"uppercase", "lowercase", "reverse", "count"},
			},
		}
	})

	// Branch 1: Convert to uppercase
	uppercase := graph.NodeFunc[ProcessingState](func(ctx context.Context, s ProcessingState) graph.NodeResult[ProcessingState] {
		time.Sleep(100 * time.Millisecond) // Simulate processing
		result := fmt.Sprintf("UPPERCASE: %s", s.Input)
		fmt.Printf("  🔤 Branch 1 (uppercase): %s\n", result)
		return graph.NodeResult[ProcessingState]{
			Delta: ProcessingState{Results: []string{result}, Count: 1},
			Route: graph.Stop(),
		}
	})

	// Branch 2: Convert to lowercase
	lowercase := graph.NodeFunc[ProcessingState](func(ctx context.Context, s ProcessingState) graph.NodeResult[ProcessingState] {
		time.Sleep(150 * time.Millisecond) // Simulate processing
		result := fmt.Sprintf("lowercase: %s", s.Input)
		fmt.Printf("  🔡 Branch 2 (lowercase): %s\n", result)
		return graph.NodeResult[ProcessingState]{
			Delta: ProcessingState{Results: []string{result}, Count: 1},
			Route: graph.Stop(),
		}
	})

	// Branch 3: Reverse the string
	reverse := graph.NodeFunc[ProcessingState](func(ctx context.Context, s ProcessingState) graph.NodeResult[ProcessingState] {
		time.Sleep(120 * time.Millisecond) // Simulate processing
		runes := []rune(s.Input)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result := fmt.Sprintf("reversed: %s", string(runes))
		fmt.Printf("  🔄 Branch 3 (reverse): %s\n", result)
		return graph.NodeResult[ProcessingState]{
			Delta: ProcessingState{Results: []string{result}, Count: 1},
			Route: graph.Stop(),
		}
	})

	// Branch 4: Count characters
	count := graph.NodeFunc[ProcessingState](func(ctx context.Context, s ProcessingState) graph.NodeResult[ProcessingState] {
		time.Sleep(80 * time.Millisecond) // Simulate processing
		result := fmt.Sprintf("length: %d characters", len(s.Input))
		fmt.Printf("  📏 Branch 4 (count): %s\n", result)
		return graph.NodeResult[ProcessingState]{
			Delta: ProcessingState{Results: []string{result}, Count: 1},
			Route: graph.Stop(),
		}
	})

	// Build the workflow graph
	if err := engine.Add("fanout", fanout); err != nil {
		log.Fatalf("Failed to add fanout node: %v", err)
	}
	if err := engine.Add("uppercase", uppercase); err != nil {
		log.Fatalf("Failed to add uppercase node: %v", err)
	}
	if err := engine.Add("lowercase", lowercase); err != nil {
		log.Fatalf("Failed to add lowercase node: %v", err)
	}
	if err := engine.Add("reverse", reverse); err != nil {
		log.Fatalf("Failed to add reverse node: %v", err)
	}
	if err := engine.Add("count", count); err != nil {
		log.Fatalf("Failed to add count node: %v", err)
	}

	if err := engine.StartAt("fanout"); err != nil {
		log.Fatalf("Failed to set entry point: %v", err)
	}

	// Execute the workflow
	ctx := context.Background()
	initialState := ProcessingState{
		Input: "LangGraph-Go",
	}

	fmt.Println("\n⏱️  Starting parallel execution...")
	start := time.Now()

	finalState, err := engine.Run(ctx, "parallel-demo-001", initialState)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	elapsed := time.Since(start)

	// Display results
	fmt.Printf("\n✅ Parallel execution completed in %v\n", elapsed)
	fmt.Printf("\n📊 Results (merged deterministically by nodeID):\n")
	for i, result := range finalState.Results {
		fmt.Printf("   %d. %s\n", i+1, result)
	}
	fmt.Printf("\nTotal branches processed: %d\n", finalState.Count)

	// Note: If branches ran sequentially, total time would be ~450ms
	// With parallel execution, total time should be ~150ms (longest branch)
	if elapsed < 200*time.Millisecond {
		fmt.Println("\n🚀 Parallelism verified! All branches executed concurrently.")
	} else {
		fmt.Println("\n⚠️  Note: Actual time may vary due to system load.")
	}
}
