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

// WorkflowState represents the state for our tracing example.
type WorkflowState struct {
	Input    string
	Results  []string
	Counter  int
	HasError bool
}

func main() {
	fmt.Println("=== LangGraph-Go Event Tracing Example ===")
	fmt.Println()

	// Create a buffered emitter to capture events for analysis
	bufferedEmitter := emit.NewBufferedEmitter()

	// Also log events to stdout for real-time monitoring
	logEmitter := emit.NewLogEmitter(os.Stdout, false)

	// Create a multi-emitter to send events to both destinations
	multiEmitter := &MultiEmitter{emitters: []emit.Emitter{bufferedEmitter, logEmitter}}

	// Define reducer
	reducer := func(prev, delta WorkflowState) WorkflowState {
		if len(delta.Results) > 0 {
			prev.Results = append(prev.Results, delta.Results...)
		}
		prev.Counter += delta.Counter
		if delta.HasError {
			prev.HasError = true
		}
		return prev
	}

	// Create engine
	st := store.NewMemStore[WorkflowState]()
	opts := graph.Options{MaxSteps: 100}
	engine := graph.New(reducer, st, multiEmitter, opts)

	// Define nodes
	startNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Println("\n[NODE: start] Processing input...")
		time.Sleep(50 * time.Millisecond) // Simulate work

		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{Counter: 1},
			Route: graph.Goto("process"),
		}
	})

	processNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Println("[NODE: process] Processing data...")
		time.Sleep(100 * time.Millisecond) // Simulate work

		result := fmt.Sprintf("processed_%s", s.Input)
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{Results: []string{result}, Counter: 1},
			Route: graph.Goto("validate"),
		}
	})

	validateNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Println("[NODE: validate] Validating results...")
		time.Sleep(75 * time.Millisecond) // Simulate work

		if len(s.Results) == 0 {
			return graph.NodeResult[WorkflowState]{
				Delta: WorkflowState{HasError: true},
				Route: graph.Goto("error_handler"),
				Err:   fmt.Errorf("validation failed: no results"),
			}
		}

		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{Counter: 1},
			Route: graph.Goto("finish"),
		}
	})

	errorHandlerNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Println("[NODE: error_handler] Handling error...")
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{Results: []string{"error_handled"}},
			Route: graph.Stop(),
		}
	})

	finishNode := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
		fmt.Println("[NODE: finish] Workflow complete!")
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{Counter: 1},
			Route: graph.Stop(),
		}
	})

	// Build the workflow graph
	if err := engine.Add("start", startNode); err != nil {
		log.Fatalf("Failed to add start node: %v", err)
	}
	if err := engine.Add("process", processNode); err != nil {
		log.Fatalf("Failed to add process node: %v", err)
	}
	if err := engine.Add("validate", validateNode); err != nil {
		log.Fatalf("Failed to add validate node: %v", err)
	}
	if err := engine.Add("error_handler", errorHandlerNode); err != nil {
		log.Fatalf("Failed to add error_handler node: %v", err)
	}
	if err := engine.Add("finish", finishNode); err != nil {
		log.Fatalf("Failed to add finish node: %v", err)
	}

	if err := engine.StartAt("start"); err != nil {
		log.Fatalf("Failed to set entry point: %v", err)
	}

	// Execute the workflow
	ctx := context.Background()
	initialState := WorkflowState{
		Input: "test_data",
	}

	fmt.Println("\n⏱️  Starting workflow execution...")
	startTime := time.Now()

	finalState, err := engine.Run(ctx, "tracing-demo-001", initialState)
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✅ Workflow completed in %v\n", elapsed)

	// Analyze execution history
	fmt.Println("\n=== Execution History Analysis ===")

	allEvents := bufferedEmitter.GetHistory("tracing-demo-001")
	fmt.Printf("\nTotal events captured: %d\n", len(allEvents))

	// Count events by type
	eventCounts := make(map[string]int)
	for _, event := range allEvents {
		eventCounts[event.Msg]++
	}

	fmt.Println("\nEvent breakdown:")
	for msgType, count := range eventCounts {
		fmt.Printf("  - %s: %d\n", msgType, count)
	}

	// Show node execution order
	fmt.Println("\nNode execution sequence:")
	nodeStarts := bufferedEmitter.GetHistoryWithFilter("tracing-demo-001", emit.HistoryFilter{Msg: "node_start"})
	for i, event := range nodeStarts {
		fmt.Printf("  %d. %s (step %d)\n", i+1, event.NodeID, event.Step)
	}

	// Show routing decisions
	fmt.Println("\nRouting decisions:")
	routingEvents := bufferedEmitter.GetHistoryWithFilter("tracing-demo-001", emit.HistoryFilter{Msg: "routing_decision"})
	for _, event := range routingEvents {
		if next, ok := event.Meta["next_node"].(string); ok {
			fmt.Printf("  Step %d: %s -> %s\n", event.Step, event.NodeID, next)
		} else if terminal, ok := event.Meta["terminal"].(bool); ok && terminal {
			fmt.Printf("  Step %d: %s -> STOP\n", event.Step, event.NodeID)
		}
	}

	// Check for errors
	errorEvents := bufferedEmitter.GetHistoryWithFilter("tracing-demo-001", emit.HistoryFilter{Msg: "error"})
	if len(errorEvents) > 0 {
		fmt.Println("\n⚠️  Errors detected:")
		for _, event := range errorEvents {
			fmt.Printf("  - Step %d, Node %s: %v\n", event.Step, event.NodeID, event.Meta["error"])
		}
	} else {
		fmt.Println("\n✓ No errors detected")
	}

	// Display final state
	fmt.Println("\n=== Final State ===")
	fmt.Printf("Input: %s\n", finalState.Input)
	fmt.Printf("Results: %v\n", finalState.Results)
	fmt.Printf("Counter: %d nodes executed\n", finalState.Counter)
	fmt.Printf("HasError: %v\n", finalState.HasError)

	fmt.Println("\n=== Demonstration Complete ===")
}

// MultiEmitter sends events to multiple emitters (fan-out pattern).
type MultiEmitter struct {
	emitters []emit.Emitter
}

func (m *MultiEmitter) Emit(event emit.Event) {
	for _, emitter := range m.emitters {
		emitter.Emit(event)
	}
}
