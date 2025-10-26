# Building Workflows

This guide covers advanced workflow patterns including error handling, multi-branch routing, and complex state management.

## Workflow Architecture Patterns

### 1. Linear Pipeline

The simplest pattern: A → B → C

```go
// Define nodes
nodeA := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Phase: "A complete"},
        Route: graph.Goto("B"),
    }
})

nodeB := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Phase: "B complete"},
        Route: graph.Goto("C"),
    }
})

nodeC := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Phase: "C complete"},
        Route: graph.Stop(),
    }
})

// Wire up
engine.Add("A", nodeA)
engine.Add("B", nodeB)
engine.Add("C", nodeC)
engine.StartAt("A")
```

### 2. Conditional Branch (Decision Tree)

Route based on state values:

```go
// Confidence-based routing
checkNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    confidence := calculateConfidence(s)
    return graph.NodeResult[State]{
        Delta: State{Confidence: confidence},
        Route: graph.Next{}, // Let edges decide
    }
})

// High confidence path
highConfNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Quality: "high"},
        Route: graph.Stop(),
    }
})

// Low confidence path - retry
retryNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{RetryCount: s.RetryCount + 1},
        Route: graph.Goto("process"), // Loop back
    }
})

// Wire with predicates
engine.Add("check", checkNode)
engine.Add("high_conf", highConfNode)
engine.Add("retry", retryNode)

engine.Connect("check", "high_conf", func(s State) bool {
    return s.Confidence > 0.8
})
engine.Connect("check", "retry", func(s State) bool {
    return s.Confidence <= 0.8 && s.RetryCount < 3
})
```

### 3. Fan-Out/Fan-In (Parallel Processing)

Execute multiple tasks concurrently:

```go
// Fan-out node
fanOut := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"task1", "task2", "task3", "task4"},
        },
    }
})

// Parallel tasks (run concurrently)
task1 := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    result := performTask1(s)
    return graph.NodeResult[State]{
        Delta: State{Results: []string{result}},
        Route: graph.Stop(),
    }
})

// Reducer merges results deterministically
reducer := func(prev, delta State) State {
    prev.Results = append(prev.Results, delta.Results...)
    return prev
}
```

### 4. Loop with Exit Condition

Iterate until condition met:

```go
processNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    processed := process(s.Data)
    return graph.NodeResult[State]{
        Delta: State{
            Data:       processed,
            Iterations: s.Iterations + 1,
        },
        Route: graph.Next{}, // Use edges
    }
})

// Continue looping
engine.Connect("process", "process", func(s State) bool {
    return s.Iterations < 10 && !isConverged(s.Data)
})

// Exit loop
engine.Connect("process", "complete", func(s State) bool {
    return s.Iterations >= 10 || isConverged(s.Data)
})
```

## Error Handling

### Node-Level Errors

Nodes can return errors via `NodeResult.Err`:

```go
riskyNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    result, err := riskyOperation(s)
    if err != nil {
        return graph.NodeResult[State]{
            Err: fmt.Errorf("operation failed: %w", err),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next"),
    }
})
```

### Retry Logic

Automatic retries with `Options.Retries`:

```go
opts := graph.Options{
    MaxSteps: 100,
    Retries:  3, // Retry failed nodes up to 3 times
}

engine := graph.New(reducer, store, emitter, opts)
```

### Error Routing

Route to error handler node:

```go
processNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    if err := validate(s); err != nil {
        return graph.NodeResult[State]{
            Delta: State{LastError: err.Error()},
            Route: graph.Goto("error_handler"),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Status: "success"},
        Route: graph.Goto("complete"),
    }
})

errorHandler := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    log.Printf("Error occurred: %s", s.LastError)

    // Decide whether to retry or fail
    if s.RetryCount < 3 {
        return graph.NodeResult[State]{
            Delta: State{RetryCount: s.RetryCount + 1},
            Route: graph.Goto("process"),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Status: "failed"},
        Route: graph.Stop(),
    }
})
```

## Complex State Management

### Nested State

Use nested structs for organization:

```go
type WorkflowState struct {
    // User input
    Request UserRequest

    // Processing state
    Processing ProcessingState

    // Results
    Results Results

    // Metadata
    Metadata Metadata
}

type UserRequest struct {
    Query    string
    Context  map[string]string
    Priority int
}

type ProcessingState struct {
    Phase       string
    Confidence  float64
    Attempts    int
    LastError   string
}
```

### State Accumulation

Collect results from multiple steps:

```go
type CollectionState struct {
    Items      []Item
    TotalCount int
    Sources    []string
}

reducer := func(prev, delta CollectionState) CollectionState {
    // Append items
    prev.Items = append(prev.Items, delta.Items...)

    // Increment counter
    prev.TotalCount += delta.TotalCount

    // Track unique sources
    for _, src := range delta.Sources {
        if !contains(prev.Sources, src) {
            prev.Sources = append(prev.Sources, src)
        }
    }

    return prev
}
```

## Multi-Agent Workflows

LangGraph-Go excels at orchestrating multiple AI agents:

```go
type AgentState struct {
    UserQuery     string
    ResearchData  []Document
    Analysis      string
    Critique      string
    FinalReport   string
    AgentChain    []string // Track which agents ran
}

// Agent 1: Researcher
researcher := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
    docs := searchDocuments(ctx, s.UserQuery)

    return graph.NodeResult[AgentState]{
        Delta: AgentState{
            ResearchData: docs,
            AgentChain:   []string{"researcher"},
        },
        Route: graph.Goto("analyst"),
    }
})

// Agent 2: Analyst
analyst := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
    analysis := analyzeDocuments(ctx, s.ResearchData)

    return graph.NodeResult[AgentState]{
        Delta: AgentState{
            Analysis:   analysis,
            AgentChain: []string{"analyst"},
        },
        Route: graph.Goto("critic"),
    }
})

// Agent 3: Critic
critic := graph.NodeFunc[AgentState](func(ctx context.Context, s AgentState) graph.NodeResult[AgentState] {
    critique := reviewAnalysis(ctx, s.Analysis)
    needsRevision := critique.Score < 0.7

    return graph.NodeResult[AgentState]{
        Delta: AgentState{
            Critique:   critique.Text,
            AgentChain: []string{"critic"},
        },
        Route: graph.Next{}, // Use edges for routing
    }
})

// Route back to analyst if critique score low
engine.Connect("critic", "analyst", func(s AgentState) bool {
    return s.CritiqueScore < 0.7 && len(s.AgentChain) < 10
})

engine.Connect("critic", "reporter", func(s AgentState) bool {
    return s.CritiqueScore >= 0.7
})
```

## Testing Workflows

### Unit Testing Nodes

Test nodes in isolation:

```go
func TestProcessNode(t *testing.T) {
    ctx := context.Background()
    input := State{Data: "test input"}

    result := processNode.Run(ctx, input)

    if result.Err != nil {
        t.Fatalf("unexpected error: %v", result.Err)
    }

    if result.Delta.Processed != "expected output" {
        t.Errorf("wrong output: %s", result.Delta.Processed)
    }
}
```

### Integration Testing Workflows

Test complete workflows:

```go
func TestCompleteWorkflow(t *testing.T) {
    // Setup
    reducer := func(prev, delta State) State { /*...*/ }
    store := store.NewMemStore[State]()
    emitter := emit.NewNullEmitter() // Silent for tests
    opts := graph.Options{MaxSteps: 100}

    engine := graph.New(reducer, store, emitter, opts)

    // Build workflow
    engine.Add("A", nodeA)
    engine.Add("B", nodeB)
    engine.Add("C", nodeC)
    engine.StartAt("A")

    // Execute
    ctx := context.Background()
    final, err := engine.Run(ctx, "test-001", initialState)

    // Verify
    if err != nil {
        t.Fatalf("workflow failed: %v", err)
    }

    if final.Status != "complete" {
        t.Errorf("expected complete status, got: %s", final.Status)
    }
}
```

## Best Practices

### 1. Keep Nodes Focused

Each node should have a single responsibility:

```go
// ❌ BAD: Node does too much
func godNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Validates, processes, stores, and sends notification
    // Hard to test, hard to reuse
}

// ✅ GOOD: Separate concerns
func validateNode(ctx context.Context, s State) graph.NodeResult[State] { /*...*/ }
func processNode(ctx context.Context, s State) graph.NodeResult[State] { /*...*/ }
func storeNode(ctx context.Context, s State) graph.NodeResult[State] { /*...*/ }
func notifyNode(ctx context.Context, s State) graph.NodeResult[State] { /*...*/ }
```

### 2. Use Context for Cancellation

Respect context deadlines and cancellation:

```go
func longRunningNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return graph.NodeResult[State]{
            Err: ctx.Err(),
        }
    default:
    }

    // Do work...
    result := expensiveOperation(ctx, s)

    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next"),
    }
}
```

### 3. Design for Determinism

Ensure workflows are replayable:

```go
// ❌ BAD: Non-deterministic
func badNode(ctx context.Context, s State) graph.NodeResult[State] {
    timestamp := time.Now() // Different on replay!
    random := rand.Float64() // Different on replay!

    return graph.NodeResult[State]{
        Delta: State{Timestamp: timestamp, Random: random},
        Route: graph.Goto("next"),
    }
}

// ✅ GOOD: Deterministic
func goodNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Timestamp comes from input state
    // Random seed comes from input state

    return graph.NodeResult[State]{
        Delta: State{
            ProcessedAt: s.StartedAt,
            Seed:        s.RandomSeed,
        },
        Route: graph.Goto("next"),
    }
}
```

### 4. Handle Partial Failures

Design for resilience:

```go
func resilientNode(ctx context.Context, s State) graph.NodeResult[State] {
    results := make([]Result, 0)
    errors := make([]error, 0)

    // Try all operations, collect successes and failures
    for _, item := range s.Items {
        result, err := process(item)
        if err != nil {
            errors = append(errors, err)
        } else {
            results = append(results, result)
        }
    }

    // Return partial success
    return graph.NodeResult[State]{
        Delta: State{
            Results:      results,
            FailedCount:  len(errors),
            LastError:    errorsToString(errors),
        },
        Route: graph.Goto("next"),
    }
}
```

## Performance Optimization

### 1. Parallel Execution

Use fan-out for independent tasks:

```go
// Process 4 tasks in parallel (completes in ~1s, not ~4s)
fanOut := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"task1", "task2", "task3", "task4"},
        },
    }
})
```

### 2. Minimize State Size

Only include necessary data in state:

```go
// ❌ BAD: Large intermediate data in state
type BadState struct {
    RawHTMLPages []string // Megabytes of HTML!
    AllDocuments []Document // Thousands of documents!
}

// ✅ GOOD: References or summaries
type GoodState struct {
    DocumentIDs  []string // Just IDs, fetch from DB as needed
    SummaryStats Stats // Aggregate data only
}
```

### 3. Use Buffering for Events

Choose appropriate emitter:

```go
import (
    "os"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Development: Full logging
emitter := emit.NewLogEmitter(os.Stdout, true) // JSON mode

// Production: Buffered or external system
emitter := emit.NewBufferedEmitter() // Query history later

// High-performance: No-op emitter
emitter := emit.NewNullEmitter() // Zero overhead
```

---

**Next:** Learn advanced [State Management](./03-state-management.md) patterns →
