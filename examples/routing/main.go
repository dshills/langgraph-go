// Package main demonstrates conditional routing and decision-making in LangGraph-Go.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// AgentState represents the state of an LLM agent workflow with confidence-based routing.
type AgentState struct {
	Query      string  // User's input query
	Response   string  // Generated response
	Confidence float64 // Confidence score (0.0 - 1.0)
	Attempts   int     // Number of generation attempts
	Validated  bool    // Whether response passed validation
}

func main() {
	fmt.Println("LangGraph-Go Conditional Routing Example")
	fmt.Println("=========================================")
	fmt.Println()

	// Create reducer to merge state updates
	reducer := func(prev, delta AgentState) AgentState {
		if delta.Query != "" {
			prev.Query = delta.Query
		}
		if delta.Response != "" {
			prev.Response = delta.Response
		}
		if delta.Confidence > 0 {
			prev.Confidence = delta.Confidence
		}
		if delta.Attempts > 0 {
			prev.Attempts = delta.Attempts
		}
		if delta.Validated {
			prev.Validated = delta.Validated
		}
		return prev
	}

	// Create in-memory store and emitter
	st := store.NewMemStore[AgentState]()
	emitter := &simpleEmitter{}

	// Create engine with loop protection
	engine := graph.New(reducer, st, emitter, graph.WithMaxSteps(10))

	// Node 1: Analyze query and generate initial response
	analyzeNode := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
		fmt.Printf("Analyzing query: %q\n", s.Query)

		// Simulate analysis and response generation
		confidence := 0.65 // Simulated confidence score
		response := fmt.Sprintf("Initial response to: %s", s.Query)

		fmt.Printf("  Generated response with confidence: %.2f\n", confidence)

		return graph.NodeResult[AgentState]{
			Delta: AgentState{
				Response:   response,
				Confidence: confidence,
				Attempts:   s.Attempts + 1,
			},
			Route: graph.Next{}, // Use edge-based routing
		}
	})

	// Node 2: Refine response (for low confidence)
	refineNode := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
		fmt.Printf("Refining response (attempt %d)...\n", s.Attempts)

		// Simulate refinement with improved confidence
		confidence := s.Confidence + 0.20
		if confidence > 0.95 {
			confidence = 0.95
		}
		response := fmt.Sprintf("%s [refined]", s.Response)

		fmt.Printf("  Refined confidence: %.2f\n", confidence)

		return graph.NodeResult[AgentState]{
			Delta: AgentState{
				Response:   response,
				Confidence: confidence,
				Attempts:   s.Attempts + 1,
			},
			Route: graph.Next{}, // Check confidence again via edges
		}
	})

	// Node 3: Validate response (for high confidence)
	validateNode := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
		fmt.Printf("Validating response (confidence: %.2f)...\n", s.Confidence)

		// Simulate validation
		validated := s.Confidence >= 0.80

		if validated {
			fmt.Println("  ✓ Response validated!")
			return graph.NodeResult[AgentState]{
				Delta: AgentState{Validated: true},
				Route: graph.Stop(),
			}
		}

		fmt.Println("  ✗ Validation failed, refining...")
		return graph.NodeResult[AgentState]{
			Route: graph.Goto("refine"),
		}
	})

	// Add nodes to engine
	if err := engine.Add("analyze", analyzeNode); err != nil {
		log.Fatalf("Failed to add analyze node: %v", err)
	}
	if err := engine.Add("refine", refineNode); err != nil {
		log.Fatalf("Failed to add refine node: %v", err)
	}
	if err := engine.Add("validate", validateNode); err != nil {
		log.Fatalf("Failed to add validate node: %v", err)
	}

	// Set starting node
	if err := engine.StartAt("analyze"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Connect edges with confidence-based predicates
	// analyze → refine (if confidence < 0.80)
	lowConfidencePredicate := func(s AgentState) bool {
		return s.Confidence < 0.80
	}
	if err := engine.Connect("analyze", "refine", lowConfidencePredicate); err != nil {
		log.Fatalf("Failed to connect analyze→refine: %v", err)
	}

	// analyze → validate (if confidence >= 0.80)
	highConfidencePredicate := func(s AgentState) bool {
		return s.Confidence >= 0.80
	}
	if err := engine.Connect("analyze", "validate", highConfidencePredicate); err != nil {
		log.Fatalf("Failed to connect analyze→validate: %v", err)
	}

	// refine → refine (if still low confidence and attempts < 3)
	refineLoopPredicate := func(s AgentState) bool {
		return s.Confidence < 0.80 && s.Attempts < 3
	}
	if err := engine.Connect("refine", "refine", refineLoopPredicate); err != nil {
		log.Fatalf("Failed to connect refine→refine: %v", err)
	}

	// refine → validate (if confidence improved or max attempts reached)
	refineToValidatePredicate := func(s AgentState) bool {
		return s.Confidence >= 0.80 || s.Attempts >= 3
	}
	if err := engine.Connect("refine", "validate", refineToValidatePredicate); err != nil {
		log.Fatalf("Failed to connect refine→validate: %v", err)
	}

	// Example 1: Low confidence query (will loop through refine)
	fmt.Println("Example 1: Query requiring refinement")
	fmt.Println("--------------------------------------")

	ctx := context.Background()
	initialState := AgentState{
		Query: "What is the meaning of life?",
	}

	finalState, err := engine.Run(ctx, "routing-run-001", initialState)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("\nWorkflow completed!\n")
	fmt.Printf("Final state:\n")
	fmt.Printf("  Query: %s\n", finalState.Query)
	fmt.Printf("  Response: %s\n", finalState.Response)
	fmt.Printf("  Confidence: %.2f\n", finalState.Confidence)
	fmt.Printf("  Attempts: %d\n", finalState.Attempts)
	fmt.Printf("  Validated: %v\n", finalState.Validated)
	fmt.Println()

	fmt.Println("=========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println()
	fmt.Println("Key concepts demonstrated:")
	fmt.Println("  • Confidence-based conditional routing")
	fmt.Println("  • Edge predicates for routing decisions")
	fmt.Println("  • Loop with exit condition (max attempts)")
	fmt.Println("  • Multiple routing paths from a single node")
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
