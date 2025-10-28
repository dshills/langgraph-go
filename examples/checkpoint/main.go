package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// WorkflowState represents the state passed between nodes in the workflow.
type WorkflowState struct {
	Query    string // The user's query or request
	Result   string // The accumulated result
	Step     int    // Track which step we're on
	Complete bool   // Whether processing is complete
}

func main() {
	fmt.Println("LangGraph-Go Checkpoint Example")
	fmt.Println("================================")
	fmt.Println()

	// Create reducer to merge state updates
	reducer := func(prev, delta WorkflowState) WorkflowState {
		// Merge non-zero fields from delta into prev
		if delta.Query != "" {
			prev.Query = delta.Query
		}
		if delta.Result != "" {
			prev.Result = delta.Result
		}
		if delta.Step > 0 {
			prev.Step = delta.Step
		}
		if delta.Complete {
			prev.Complete = delta.Complete
		}
		return prev
	}

	// Create in-memory store for state persistence
	st := store.NewMemStore[WorkflowState]()

	// Create simple emitter for observability
	emitter := &simpleEmitter{}

	// Create engine
	engine := graph.New(reducer, st, emitter, graph.Options{MaxSteps: 10})

	// Define the 3-node workflow
	// Node 1: Parse Query
	parseNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Printf("Node 1: Parsing query: %q\n", s.Query)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Result: fmt.Sprintf("Parsed: %s", s.Query),
				Step:   1,
			},
			Route: graph.Goto("process"),
		}
	})

	// Node 2: Process Request
	processNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Printf("Node 2: Processing request from step %d\n", s.Step)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Result: s.Result + " -> Processed",
				Step:   2,
			},
			Route: graph.Goto("finalize"),
		}
	})

	// Node 3: Finalize Result
	finalizeNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Printf("Node 3: Finalizing result from step %d\n", s.Step)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Result:   s.Result + " -> Done!",
				Step:     3,
				Complete: true,
			},
			Route: graph.Stop(),
		}
	})

	// Add nodes to engine
	if err := engine.Add("parse", parseNode); err != nil {
		log.Fatalf("Failed to add parse node: %v", err)
	}
	if err := engine.Add("process", processNode); err != nil {
		log.Fatalf("Failed to add process node: %v", err)
	}
	if err := engine.Add("finalize", finalizeNode); err != nil {
		log.Fatalf("Failed to add finalize node: %v", err)
	}

	// Set starting node
	if err := engine.StartAt("parse"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Example 1: Run complete workflow
	fmt.Println("Example 1: Running complete 3-node workflow")
	fmt.Println("-------------------------------------------")

	ctx := context.Background()
	initialState := WorkflowState{
		Query: "What is the weather?",
	}

	finalState, err := engine.Run(ctx, "run-001", initialState)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nWorkflow completed!\n")
	fmt.Printf("Final state: %+v\n", finalState)
	fmt.Println()

	// Example 2: Checkpoint and resume
	fmt.Println("Example 2: Checkpoint and Resume")
	fmt.Println("----------------------------------")

	// Run workflow and create checkpoint after node 2
	initialState2 := WorkflowState{
		Query: "How do I use checkpoints?",
	}

	_, err = engine.Run(ctx, "run-002", initialState2)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	// Save checkpoint
	fmt.Println("\nSaving checkpoint after workflow completes...")
	if err := engine.SaveCheckpoint(ctx, "run-002", "checkpoint-001"); err != nil {
		log.Fatalf("Failed to save checkpoint: %v", err)
	}
	fmt.Println("Checkpoint saved: checkpoint-001")

	// Resume from checkpoint with different query
	fmt.Println("\nResuming from checkpoint with new execution...")
	resumedState, err := engine.ResumeFromCheckpoint(ctx, "checkpoint-001", "run-003", "parse")
	if err != nil {
		log.Fatalf("Failed to resume from checkpoint: %v", err)
	}

	fmt.Printf("\nResumed workflow completed!\n")
	fmt.Printf("Final state: %+v\n", resumedState)
	fmt.Println()

	fmt.Println("================================")
	fmt.Println("Example completed successfully!")
}

// simpleEmitter implements the Emitter interface for observability.
type simpleEmitter struct{}

func (e *simpleEmitter) Emit(event emit.Event) {
	if event.Msg != "" {
		fmt.Printf("  [Event] Step %d, Node %q: %s\n", event.Step, event.NodeID, event.Msg)
	}
}

func (e *simpleEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		e.Emit(event)
	}
	return nil
}

func (e *simpleEmitter) Flush(ctx context.Context) error {
	return nil
}
