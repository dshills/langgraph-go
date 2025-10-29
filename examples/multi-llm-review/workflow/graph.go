package workflow

import (
	"os"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// NewReviewWorkflow creates a new review workflow engine with the given provider and scanner.
//
// The workflow consists of four main nodes:
//  1. discover: Discovers all code files and creates batches
//  2. review-batch: Reviews a single batch with the AI provider (loops until all batches done)
//  3. consolidate: Merges duplicate issues from multiple batches
//  4. report: Generates the final markdown report
//
// Parameters:
//   - provider: The AI code reviewer to use for analysis
//   - scanner: The file scanner to discover code files
//   - batchSize: Number of files to process per batch
//
// Returns:
//   - *graph.Engine[ReviewState]: The configured workflow engine
//   - error: Configuration error if any
//
// Example:
//
//	provider := providers.NewAnthropicProvider(apiKey)
//	scanner := scanner.NewGoScanner()
//	engine, err := NewReviewWorkflow(provider, scanner, 10)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	initialState := ReviewState{
//	    CodebaseRoot: "/path/to/code",
//	    StartTime:    time.Now().Format(time.RFC3339),
//	    Reviews:      make(map[string][]Review),
//	}
//
//	finalState, err := engine.Run(ctx, "run-001", initialState)
func NewReviewWorkflow(provider CodeReviewer, scanner FileScanner, batchSize int) (*graph.Engine[ReviewState], error) {
	return NewReviewWorkflowWithProviders([]CodeReviewer{provider}, scanner, batchSize)
}

// NewReviewWorkflowWithProviders creates a new review workflow engine with multiple providers.
// All providers will be called concurrently for each batch.
//
// Parameters:
//   - providers: The AI code reviewers to use for analysis (called concurrently)
//   - scanner: The file scanner to discover code files
//   - batchSize: Number of files to process per batch
//
// Returns:
//   - *graph.Engine[ReviewState]: The configured workflow engine
//   - error: Configuration error if any
func NewReviewWorkflowWithProviders(providers []CodeReviewer, scanner FileScanner, batchSize int) (*graph.Engine[ReviewState], error) {
	// Create in-memory store for checkpoints (T040)
	// In production, this would be replaced with a persistent store (MySQL, PostgreSQL, etc.)
	st := store.NewMemStore[ReviewState]()

	// Create stdout log emitter for observability (T041)
	// Emits node_start, node_end, routing_decision, and error events
	emitter := emit.NewLogEmitter(os.Stdout, false) // false = text mode (not JSON)

	// Create the engine with reducer, store, emitter, and options
	// MaxSteps: 100 - Prevent infinite loops (reasonable for batched processing)
	engine := graph.New(ReduceReviewState, st, emitter, graph.WithMaxSteps(100))

	// Create and add nodes to the graph

	// 1. DiscoverFilesNode: Entry point - discovers files and creates batches
	discoverNode := &DiscoverFilesNode{
		Scanner:   scanner,
		BatchSize: batchSize,
	}
	if err := engine.Add("discover", discoverNode); err != nil {
		return nil, err
	}

	// 2. ReviewBatchNode: Reviews current batch with all AI providers concurrently
	reviewNode := NewReviewBatchNode(providers)
	if err := engine.Add("review-batch", reviewNode); err != nil {
		return nil, err
	}

	// 3. ConsolidateNode: Merges duplicate issues after all batches reviewed
	consolidateNode := &ConsolidateNode{}
	if err := engine.Add("consolidate", consolidateNode); err != nil {
		return nil, err
	}

	// 4. ReportNode: Generates final markdown report
	reportNode := &ReportNode{}
	if err := engine.Add("report", reportNode); err != nil {
		return nil, err
	}

	// Set the entry point (workflow starts at discover)
	if err := engine.StartAt("discover"); err != nil {
		return nil, err
	}

	// Note: Node routing is handled via NodeResult.Route in each node's Run method.
	// The flow is:
	//   discover → review-batch → (loops back to review-batch) → consolidate → report → Stop
	//
	// No explicit edges are needed because each node returns explicit routing decisions:
	//   - DiscoverFilesNode: returns Goto("review-batch")
	//   - ReviewBatchNode: returns Goto("review-batch") if more batches, else Goto("consolidate")
	//   - ConsolidateNode: returns Goto("report")
	//   - ReportNode: returns Stop()

	return engine, nil
}
