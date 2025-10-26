# Parallel Execution

This guide covers fan-out/fan-in patterns, concurrent node execution, state isolation, and best practices for building high-performance parallel workflows.

## Overview

LangGraph-Go supports parallel execution through **fan-out routing**. A single node can route to multiple nodes that execute concurrently, with results merged back deterministically using the reducer.

**Key Benefits**:
- **Performance**: Execute independent tasks simultaneously
- **State Isolation**: Each branch gets an isolated state copy
- **Deterministic Merging**: Reducer combines results predictably
- **Error Handling**: Individual branch failures don't crash the workflow

## Basic Fan-Out

Route to multiple nodes in parallel:

```go
type State struct {
    Input   string
    Results []string
}

// Fan-out node
func fanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"task1", "task2", "task3", "task4"},
        },
    }
}

// Task 1: Runs concurrently
func task1(ctx context.Context, s State) graph.NodeResult[State] {
    result := process1(s.Input)
    return graph.NodeResult[State]{
        Delta: State{Results: []string{result}},
        Route: graph.Stop(), // Terminal - no further routing
    }
}

// Task 2: Runs concurrently
func task2(ctx context.Context, s State) graph.NodeResult[State] {
    result := process2(s.Input)
    return graph.NodeResult[State]{
        Delta: State{Results: []string{result}},
        Route: graph.Stop(),
    }
}

// ... task3, task4 similar ...

// Reducer merges results from all branches
func reducer(prev, delta State) State {
    prev.Results = append(prev.Results, delta.Results...)
    return prev
}

// Wire up the workflow
engine.Add("fanout", graph.NodeFunc[State](fanOut))
engine.Add("task1", graph.NodeFunc[State](task1))
engine.Add("task2", graph.NodeFunc[State](task2))
engine.Add("task3", graph.NodeFunc[State](task3))
engine.Add("task4", graph.NodeFunc[State](task4))
engine.StartAt("fanout")

// Run the workflow
final, err := engine.Run(ctx, "parallel-001", initialState)
// final.Results contains outputs from all 4 tasks
```

## State Isolation

Each parallel branch receives an **isolated copy** of the state:

```go
type State struct {
    Counter  int
    SeenBy   []string
}

func fanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Counter: 10}, // Set counter to 10
        Route: graph.Next{Many: []string{"branchA", "branchB", "branchC"}},
    }
}

func branchA(ctx context.Context, s State) graph.NodeResult[State] {
    // Each branch sees Counter = 10 (from fanout)
    // Branches do NOT see each other's modifications
    fmt.Printf("Branch A sees Counter=%d\n", s.Counter) // 10

    return graph.NodeResult[State]{
        Delta: State{
            Counter: 5,          // Only affects A's delta
            SeenBy:  []string{"A"},
        },
        Route: graph.Stop(),
    }
}

func branchB(ctx context.Context, s State) graph.NodeResult[State] {
    // Branch B also sees Counter = 10, NOT the 5 from Branch A
    fmt.Printf("Branch B sees Counter=%d\n", s.Counter) // 10

    return graph.NodeResult[State]{
        Delta: State{
            Counter: 3,          // Only affects B's delta
            SeenBy:  []string{"B"},
        },
        Route: graph.Stop(),
    }
}

// Reducer merges all deltas
func reducer(prev, delta State) State {
    prev.Counter += delta.Counter // Accumulate counters
    prev.SeenBy = append(prev.SeenBy, delta.SeenBy...)
    return prev
}

// Final state after merge:
// Counter = 10 + 5 + 3 = 18
// SeenBy = ["A", "B", "C"]
```

**Key Insight**: Branches execute independently with isolated state. Only their deltas are merged, not their runtime modifications.

## Fan-Out Patterns

### Pattern 1: Data Parallel Processing

Process multiple items concurrently:

```go
type State struct {
    Items   []Item
    Results map[string]Result
}

func parallelProcess(ctx context.Context, s State) graph.NodeResult[State] {
    // Fan out to one node per item
    branches := make([]string, len(s.Items))
    for i := range s.Items {
        branches[i] = fmt.Sprintf("process-item-%d", i)
    }

    return graph.NodeResult[State]{
        Route: graph.Next{Many: branches},
    }
}

// Dynamically register nodes for each item
for i, item := range items {
    nodeID := fmt.Sprintf("process-item-%d", i)
    capturedItem := item // Capture for closure
    engine.Add(nodeID, graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
        result := processItem(capturedItem)
        return graph.NodeResult[State]{
            Delta: State{
                Results: map[string]Result{capturedItem.ID: result},
            },
            Route: graph.Stop(),
        }
    }))
}
```

### Pattern 2: Multi-Model Consensus

Query multiple LLMs in parallel:

```go
type State struct {
    Query     string
    Responses map[string]string
    Consensus string
}

func queryModels(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"gpt4", "claude", "gemini"},
        },
    }
}

func gpt4Node(ctx context.Context, s State) graph.NodeResult[State] {
    response := callGPT4(s.Query)
    return graph.NodeResult[State]{
        Delta: State{
            Responses: map[string]string{"gpt4": response},
        },
        Route: graph.Goto("consensus"), // Route to consensus node
    }
}

func claudeNode(ctx context.Context, s State) graph.NodeResult[State] {
    response := callClaude(s.Query)
    return graph.NodeResult[State]{
        Delta: State{
            Responses: map[string]string{"claude": response},
        },
        Route: graph.Goto("consensus"),
    }
}

func geminiNode(ctx context.Context, s State) graph.NodeResult[State] {
    response := callGemini(s.Query)
    return graph.NodeResult[State]{
        Delta: State{
            Responses: map[string]string{"gemini": response},
        },
        Route: graph.Goto("consensus"),
    }
}

// Consensus node (runs 3 times, once after each model)
func consensusNode(ctx context.Context, s State) graph.NodeResult[State] {
    if len(s.Responses) < 3 {
        // Not all responses received yet
        return graph.NodeResult[State]{
            Route: graph.Stop(), // Wait for more
        }
    }

    // All responses received - compute consensus
    consensus := computeConsensus(s.Responses)
    return graph.NodeResult[State]{
        Delta: State{Consensus: consensus},
        Route: graph.Stop(),
    }
}

// Reducer merges responses map
func reducer(prev, delta State) State {
    if prev.Responses == nil {
        prev.Responses = make(map[string]string)
    }
    for k, v := range delta.Responses {
        prev.Responses[k] = v
    }
    if delta.Consensus != "" {
        prev.Consensus = delta.Consensus
    }
    return prev
}
```

### Pattern 3: Pipeline Stages with Fan-Out

Each stage fans out to multiple workers:

```go
type State struct {
    Stage       string
    RawData     []byte
    Extracted   []string
    Transformed []string
    Validated   []string
}

// Stage 1: Extract (fan-out to 4 extractors)
func extract(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Stage: "extracting"},
        Route: graph.Next{
            Many: []string{"extract1", "extract2", "extract3", "extract4"},
        },
    }
}

// Stage 2: Transform (fan-out to 4 transformers)
func transform(ctx context.Context, s State) graph.NodeResult[State] {
    if s.Stage != "extracting" || len(s.Extracted) < 4 {
        return graph.NodeResult[State]{Route: graph.Stop()}
    }

    return graph.NodeResult[State]{
        Delta: State{Stage: "transforming"},
        Route: graph.Next{
            Many: []string{"transform1", "transform2", "transform3", "transform4"},
        },
    }
}

// Stage 3: Validate (fan-out to 4 validators)
func validate(ctx context.Context, s State) graph.NodeResult[State] {
    if s.Stage != "transforming" || len(s.Transformed) < 4 {
        return graph.NodeResult[State]{Route: graph.Stop()}
    }

    return graph.NodeResult[State]{
        Delta: State{Stage: "validating"},
        Route: graph.Next{
            Many: []string{"validate1", "validate2", "validate3", "validate4"},
        },
    }
}
```

### Pattern 4: Competitive Racing

Execute multiple strategies, take the first to finish:

```go
type State struct {
    Query     string
    BestScore float64
    BestAnswer string
    Attempts   int
}

func race(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"strategy1", "strategy2", "strategy3"},
        },
    }
}

func strategy1(ctx context.Context, s State) graph.NodeResult[State] {
    answer, score := tryStrategy1(s.Query)
    return graph.NodeResult[State]{
        Delta: State{
            BestScore:  score,
            BestAnswer: answer,
            Attempts:   1,
        },
        Route: graph.Stop(),
    }
}

// Reducer keeps the best score
func reducer(prev, delta State) State {
    if delta.BestScore > prev.BestScore {
        prev.BestScore = delta.BestScore
        prev.BestAnswer = delta.BestAnswer
    }
    prev.Attempts += delta.Attempts
    return prev
}
```

## Error Handling in Parallel Execution

### Individual Branch Errors

Branches can fail independently without crashing the workflow:

```go
type State struct {
    Results []string
    Errors  []error
}

func task1(ctx context.Context, s State) graph.NodeResult[State] {
    result, err := riskyOperation()
    if err != nil {
        return graph.NodeResult[State]{
            Delta: State{Errors: []error{err}},
            Route: graph.Stop(),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Results: []string{result}},
        Route: graph.Stop(),
    }
}

// Reducer collects both successes and errors
func reducer(prev, delta State) State {
    prev.Results = append(prev.Results, delta.Results...)
    prev.Errors = append(prev.Errors, delta.Errors...)
    return prev
}

// After execution, check results
final, err := engine.Run(ctx, "run-001", initialState)
if err != nil {
    // Workflow-level error
    log.Fatal(err)
}

// Check individual branch errors
if len(final.Errors) > 0 {
    fmt.Printf("%d branches failed:\n", len(final.Errors))
    for _, e := range final.Errors {
        fmt.Printf("  - %v\n", e)
    }
}

fmt.Printf("%d branches succeeded\n", len(final.Results))
```

### Fail-Fast Mode

Stop workflow if any branch fails:

```go
type State struct {
    Results   []string
    HasError  bool
    LastError string
}

func failingBranch(ctx context.Context, s State) graph.NodeResult[State] {
    err := riskyOperation()
    if err != nil {
        return graph.NodeResult[State]{
            Err: err, // NodeResult.Err halts the workflow
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Results: []string{"success"}},
        Route: graph.Stop(),
    }
}

// Workflow stops on first error
final, err := engine.Run(ctx, "run-001", initialState)
if err != nil {
    fmt.Printf("Workflow failed: %v\n", err)
    // Some branches may not have executed
}
```

## Performance Considerations

### Optimal Parallelism

Don't over-parallelize:

```go
// ❌ BAD: 1000 concurrent goroutines
Many: generateNodeIDs(1000) // Too much overhead

// ✅ GOOD: Chunk work appropriately
const maxParallel = 10
chunks := chunkItems(items, len(items)/maxParallel)
Many: generateChunkNodeIDs(chunks) // 10 concurrent nodes
```

### Goroutine Overhead

Each parallel branch spawns a goroutine:

```go
// Each branch = 1 goroutine
Route: Next{Many: []string{"a", "b", "c", "d"}} // 4 goroutines

// For large fan-outs, consider worker pools or chunking
```

### State Copy Cost

Large state objects are copied for each branch:

```go
// ❌ BAD: Large state copied 100 times
type State struct {
    LargeData [1000000]byte // 1MB copied 100x = 100MB
}
Route: Next{Many: generate100Nodes()}

// ✅ GOOD: Use references
type State struct {
    DataRef *SharedData // Pointer shared across branches
}
```

## Testing Parallel Workflows

### Test Deterministic Merging

```go
func TestParallelMerge(t *testing.T) {
    store := store.NewMemStore[State]()
    emitter := emit.NewNullEmitter()
    engine := graph.New(reducer, store, emitter, graph.Options{})

    // Build parallel workflow
    engine.Add("fanout", fanOutNode)
    engine.Add("branch1", branch1Node)
    engine.Add("branch2", branch2Node)
    engine.Add("branch3", branch3Node)
    engine.StartAt("fanout")

    // Run multiple times
    for i := 0; i < 10; i++ {
        final, err := engine.Run(context.Background(), fmt.Sprintf("run-%d", i), initialState)
        if err != nil {
            t.Fatal(err)
        }

        // Results should be deterministic
        if len(final.Results) != 3 {
            t.Errorf("expected 3 results, got %d", len(final.Results))
        }
    }
}
```

### Test Error Isolation

```go
func TestParallelErrorIsolation(t *testing.T) {
    store := store.NewMemStore[State]()
    emitter := emit.NewNullEmitter()
    engine := graph.New(reducer, store, emitter, graph.Options{})

    // Mix failing and succeeding branches
    engine.Add("fanout", fanOutNode)
    engine.Add("success1", successNode)
    engine.Add("failing", failingNode) // Returns Err
    engine.Add("success2", successNode)
    engine.StartAt("fanout")

    final, err := engine.Run(context.Background(), "test", initialState)

    // Workflow should complete with partial results
    if err != nil {
        t.Fatal("workflow should not fail completely")
    }

    // Check that successful branches completed
    if len(final.Results) != 2 {
        t.Errorf("expected 2 successful results, got %d", len(final.Results))
    }

    // Check that error was recorded
    if len(final.Errors) != 1 {
        t.Errorf("expected 1 error, got %d", len(final.Errors))
    }
}
```

## Advanced Patterns

### Dynamic Fan-Out

Determine parallelism at runtime:

```go
func dynamicFanOut(ctx context.Context, s State) graph.NodeResult[State] {
    // Determine branches based on state
    var branches []string

    if s.Priority == "high" {
        branches = []string{"fast1", "fast2", "fast3"}
    } else {
        branches = []string{"slow1", "slow2"}
    }

    if s.RequiresValidation {
        branches = append(branches, "validator")
    }

    return graph.NodeResult[State]{
        Route: graph.Next{Many: branches},
    }
}
```

### Nested Parallelism

Fan-out from parallel branches:

```go
// Level 1: Fan out to 3 regions
func regionFanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{Many: []string{"us", "eu", "asia"}},
    }
}

// Level 2: Each region fans out to zones
func usFanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{Many: []string{"us-east", "us-west", "us-central"}},
    }
}

func euFanOut(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{Many: []string{"eu-west", "eu-central"}},
    }
}

// Total: 3 regions × ~2-3 zones each = 8 parallel leaf nodes
```

### Barrier Synchronization

Wait for all branches before continuing:

```go
type State struct {
    BranchesComplete int
    TotalBranches    int
    CanProceed       bool
}

func barrier(ctx context.Context, s State) graph.NodeResult[State] {
    // Check if all branches completed
    if s.BranchesComplete >= s.TotalBranches {
        return graph.NodeResult[State]{
            Delta: State{CanProceed: true},
            Route: graph.Goto("next-stage"),
        }
    }

    // Not ready yet
    return graph.NodeResult[State]{
        Route: graph.Stop(),
    }
}

// Each branch increments the counter
func branch(ctx context.Context, s State) graph.NodeResult[State] {
    result := doWork()
    return graph.NodeResult[State]{
        Delta: State{
            BranchesComplete: 1, // Reducer increments
        },
        Route: graph.Goto("barrier"),
    }
}
```

## Best Practices

1. **Keep Branches Independent**: Avoid dependencies between parallel branches
2. **Design for Isolation**: Don't rely on shared mutable state
3. **Handle Partial Failures**: Collect errors, don't fail the entire workflow
4. **Test Determinism**: Ensure results are consistent across runs
5. **Monitor Goroutines**: Be aware of concurrency limits
6. **Optimize State Size**: Minimize copying overhead for large fan-outs
7. **Use Timeouts**: Set context deadlines to prevent hanging branches

---

**Next:** Learn how to integrate LLMs with [LLM Integration](./07-llm-integration.md) →
