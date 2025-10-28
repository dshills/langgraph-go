package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// State represents the workflow state with zero-configuration persistence.
type State struct {
	Message string
	Count   int
	Done    bool
}

func main() {
	fmt.Println("SQLite Quickstart: Zero-Setup Persistence")
	fmt.Println("==========================================")
	fmt.Println()

	// 1. Create SQLite store with a single file - no setup required!
	//    The database file and all tables are created automatically.
	dbPath := "./quickstart.db"
	sqliteStore, err := store.NewSQLiteStore[State](dbPath)
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer sqliteStore.Close()
	fmt.Printf("✓ Created SQLite database at: %s\n\n", dbPath)

	// 2. Define a simple reducer to merge state updates
	reducer := func(prev, delta State) State {
		if delta.Message != "" {
			prev.Message = delta.Message
		}
		if delta.Count > 0 {
			prev.Count = delta.Count
		}
		if delta.Done {
			prev.Done = delta.Done
		}
		return prev
	}

	// 3. Create a simple emitter for observability
	emitter := emit.NewLogEmitter(os.Stdout, false)

	// 4. Create engine with SQLite store
	engine := graph.New(reducer, sqliteStore, emitter, graph.Options{
		MaxSteps: 10,
	})

	// 5. Define workflow nodes
	startNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
		fmt.Println("→ Node 'start': Initializing workflow")
		return graph.NodeResult[State]{
			Delta: State{
				Message: "Workflow started",
				Count:   1,
			},
			Route: graph.Goto("process"),
		}
	})

	processNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
		fmt.Printf("→ Node 'process': Processing (count=%d, message=%q)\n", s.Count, s.Message)
		return graph.NodeResult[State]{
			Delta: State{
				Message: fmt.Sprintf("%s -> Processed", s.Message),
				Count:   s.Count + 1,
			},
			Route: graph.Goto("finish"),
		}
	})

	finishNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
		fmt.Printf("→ Node 'finish': Completing (count=%d, message=%q)\n", s.Count, s.Message)
		return graph.NodeResult[State]{
			Delta: State{
				Message: fmt.Sprintf("%s -> Complete!", s.Message),
				Count:   s.Count + 1,
				Done:    true,
			},
			Route: graph.Stop(),
		}
	})

	// 6. Add nodes to engine
	if err := engine.Add("start", startNode); err != nil {
		log.Fatalf("Failed to add start node: %v", err)
	}
	if err := engine.Add("process", processNode); err != nil {
		log.Fatalf("Failed to add process node: %v", err)
	}
	if err := engine.Add("finish", finishNode); err != nil {
		log.Fatalf("Failed to add finish node: %v", err)
	}

	// 7. Set entry point for workflow
	if err := engine.StartAt("start"); err != nil {
		log.Fatalf("Failed to set entry point: %v", err)
	}

	// Run the workflow
	runID := "quickstart-001"
	ctx := context.Background()

	fmt.Println("Starting workflow execution...")
	fmt.Println("─────────────────────────────")

	finalState, err := engine.Run(ctx, runID, State{})
	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	fmt.Println("─────────────────────────────")
	fmt.Printf("\n✓ Workflow completed!\n")
	fmt.Printf("  Final message: %q\n", finalState.Message)
	fmt.Printf("  Final count: %d\n", finalState.Count)
	fmt.Printf("  Done: %v\n\n", finalState.Done)

	// 8. Demonstrate persistence: Load state from database
	fmt.Println("Demonstrating persistence...")
	fmt.Println("─────────────────────────────")

	loadedState, step, err := sqliteStore.LoadLatest(ctx, runID)
	if err != nil {
		log.Fatalf("Failed to load state from database: %v", err)
	}

	fmt.Printf("✓ Loaded state from database:\n")
	fmt.Printf("  Step: %d\n", step)
	fmt.Printf("  Message: %q\n", loadedState.Message)
	fmt.Printf("  Count: %d\n", loadedState.Count)
	fmt.Printf("  Done: %v\n\n", loadedState.Done)

	// 9. Show database file info
	fileInfo, err := os.Stat(dbPath)
	if err == nil {
		fmt.Printf("Database file: %s (%d bytes)\n", dbPath, fileInfo.Size())
		fmt.Println("\nℹ️  The database persists across runs. Delete it to start fresh:")
		fmt.Printf("   rm %s\n", dbPath)
	}

	fmt.Println("\n✅ SQLite Quickstart Complete!")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("  • Zero-configuration setup (just provide a file path)")
	fmt.Println("  • Automatic schema creation")
	fmt.Println("  • State persistence across workflow steps")
	fmt.Println("  • WAL mode for concurrent reads")
	fmt.Println("  • Full ACID transaction guarantees")
	fmt.Println("\nNext Steps:")
	fmt.Println("  • Run this example multiple times to see persistence")
	fmt.Println("  • Check docs/store-guarantees.md for exactly-once semantics")
	fmt.Println("  • Try the checkpoint example for advanced features")
}
