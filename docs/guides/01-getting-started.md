# Getting Started with LangGraph-Go

This guide will help you build your first stateful workflow with LangGraph-Go in under 10 minutes.

## Installation

### Prerequisites

- Go 1.21 or later (requires generics support)
- Basic understanding of Go programming
- Familiarity with context-based programming

### Install the Framework

```bash
go get github.com/dshills/langgraph-go
```

### Verify Installation

Create a simple test to verify the installation:

```bash
mkdir langgraph-demo && cd langgraph-demo
go mod init example.com/demo
go get github.com/dshills/langgraph-go
```

## Your First Workflow

Let's build a simple 3-node workflow that processes a query through multiple steps.

### Step 1: Define Your State

State is the data that flows through your workflow. It must be a struct and JSON-serializable:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/emit"
    "github.com/dshills/langgraph-go/graph/store"
)

// QueryState represents the data flowing through our workflow
type QueryState struct {
    UserQuery    string
    Validated    bool
    ProcessedBy  []string
    FinalAnswer  string
    StepCount    int
}
```

### Step 2: Create a Reducer

The reducer determines how state updates are merged:

```go
// Reducer merges state updates from nodes
func reducer(prev, delta QueryState) QueryState {
    // Merge UserQuery (prefer new value if not empty)
    if delta.UserQuery != "" {
        prev.UserQuery = delta.UserQuery
    }

    // Merge boolean flags (OR logic)
    if delta.Validated {
        prev.Validated = true
    }

    // Append to lists
    prev.ProcessedBy = append(prev.ProcessedBy, delta.ProcessedBy...)

    // Update final answer if provided
    if delta.FinalAnswer != "" {
        prev.FinalAnswer = delta.FinalAnswer
    }

    // Increment counters
    prev.StepCount += delta.StepCount

    return prev
}
```

### Step 3: Define Your Nodes

Nodes are the processing units of your workflow. Each node receives state and returns modifications:

```go
// Node 1: Validate the query
func validateNode(ctx context.Context, s QueryState) graph.NodeResult[QueryState] {
    fmt.Println("🔍 Validating query...")

    // Validation logic
    isValid := len(s.UserQuery) > 0 && len(s.UserQuery) < 500

    return graph.NodeResult[QueryState]{
        Delta: QueryState{
            Validated:   isValid,
            ProcessedBy: []string{"validator"},
            StepCount:   1,
        },
        Route: graph.Goto("process"), // Route to next node
    }
}

// Node 2: Process the query
func processNode(ctx context.Context, s QueryState) graph.NodeResult[QueryState] {
    fmt.Println("⚙️  Processing query...")

    if !s.Validated {
        return graph.NodeResult[QueryState]{
            Err: fmt.Errorf("cannot process invalid query"),
        }
    }

    // Simulate processing
    processed := fmt.Sprintf("Processed: %s", s.UserQuery)

    return graph.NodeResult[QueryState]{
        Delta: QueryState{
            FinalAnswer: processed,
            ProcessedBy: []string{"processor"},
            StepCount:   1,
        },
        Route: graph.Goto("finalize"),
    }
}

// Node 3: Finalize the result
func finalizeNode(ctx context.Context, s QueryState) graph.NodeResult[QueryState] {
    fmt.Println("✅ Finalizing result...")

    finalAnswer := fmt.Sprintf("[FINAL] %s (processed by: %v)",
        s.FinalAnswer, s.ProcessedBy)

    return graph.NodeResult[QueryState]{
        Delta: QueryState{
            FinalAnswer: finalAnswer,
            ProcessedBy: []string{"finalizer"},
            StepCount:   1,
        },
        Route: graph.Stop(), // End the workflow
    }
}
```

### Step 4: Build the Workflow

Assemble your nodes into a workflow graph:

```go
func main() {
    // Create dependencies
    st := store.NewMemStore[QueryState]()
    emitter := emit.NewLogEmitter(os.Stdout, false) // Text output
    opts := graph.Options{
        MaxSteps: 100, // Prevent infinite loops
    }

    // Create the engine
    engine := graph.New(reducer, st, emitter, opts)

    // Register nodes
    engine.Add("validate", graph.NodeFunc[QueryState](validateNode))
    engine.Add("process", graph.NodeFunc[QueryState](processNode))
    engine.Add("finalize", graph.NodeFunc[QueryState](finalizeNode))

    // Set entry point
    engine.StartAt("validate")

    // Execute the workflow
    ctx := context.Background()
    initialState := QueryState{
        UserQuery: "What is LangGraph-Go?",
    }

    fmt.Println("🚀 Starting workflow...")
    finalState, err := engine.Run(ctx, "demo-run-001", initialState)
    if err != nil {
        panic(err)
    }

    // Display results
    fmt.Println("\n📊 Workflow Complete!")
    fmt.Printf("Steps executed: %d\n", finalState.StepCount)
    fmt.Printf("Final answer: %s\n", finalState.FinalAnswer)
}
```

### Step 5: Run Your Workflow

```bash
go run main.go
```

**Expected Output:**

```
🚀 Starting workflow...
[node_start] runID=demo-run-001 step=0 nodeID=validate
🔍 Validating query...
[node_end] runID=demo-run-001 step=0 nodeID=validate meta={"delta":{"UserQuery":"","Validated":true,"ProcessedBy":["validator"],"FinalAnswer":"","StepCount":1}}
[routing_decision] runID=demo-run-001 step=0 nodeID=validate meta={"next_node":"process"}
[node_start] runID=demo-run-001 step=1 nodeID=process
⚙️  Processing query...
[node_end] runID=demo-run-001 step=1 nodeID=process meta={"delta":{"UserQuery":"","Validated":false,"ProcessedBy":["processor"],"FinalAnswer":"Processed: What is LangGraph-Go?","StepCount":1}}
[routing_decision] runID=demo-run-001 step=1 nodeID=process meta={"next_node":"finalize"}
[node_start] runID=demo-run-001 step=2 nodeID=finalize
✅ Finalizing result...
[node_end] runID=demo-run-001 step=2 nodeID=finalize meta={"delta":{"UserQuery":"","Validated":false,"ProcessedBy":["finalizer"],"FinalAnswer":"[FINAL] Processed: What is LangGraph-Go? (processed by: [validator processor finalizer])","StepCount":1}}
[routing_decision] runID=demo-run-001 step=2 nodeID=finalize meta={"terminal":true}

📊 Workflow Complete!
Steps executed: 3
Final answer: [FINAL] Processed: What is LangGraph-Go? (processed by: [validator processor finalizer])
```

## Understanding the Output

Let's break down what happened:

1. **Event Tracing**: Every node execution emits events:
   - `node_start`: Node begins execution
   - `node_end`: Node completes, shows state delta
   - `routing_decision`: Shows which node executes next

2. **State Flow**:
   - Initial state contains only `UserQuery`
   - Each node adds data via `Delta`
   - Reducer merges deltas into accumulated state

3. **Routing**:
   - `validate` → `process`: Explicit routing via `graph.Goto()`
   - `process` → `finalize`: Explicit routing
   - `finalize` → END: Terminal via `graph.Stop()`

## Core Concepts Explained

### Nodes

Nodes are functions or structs that implement the `Node[S]` interface:

```go
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

**Function-based nodes** (easiest):
```go
graph.NodeFunc[MyState](func(ctx context.Context, s MyState) graph.NodeResult[MyState] {
    // Your logic here
    return graph.NodeResult[MyState]{
        Delta: MyState{/* changes */},
        Route: graph.Goto("next"),
    }
})
```

**Struct-based nodes** (for complex logic):
```go
type MyNode struct {
    config Config
}

func (n *MyNode) Run(ctx context.Context, s MyState) graph.NodeResult[MyState] {
    // Access n.config...
    return graph.NodeResult[MyState]{...}
}
```

### NodeResult

Every node returns a `NodeResult[S]` with:

```go
type NodeResult[S any] struct {
    Delta  S         // State changes to apply
    Route  Next      // Where to go next
    Events []Event   // Custom events (optional)
    Err    error     // Error if node failed
}
```

### Routing

Control flow with `Next`:

```go
// Go to specific node
graph.Goto("node-name")

// Stop execution (success)
graph.Stop()

// Fan out to multiple parallel nodes
graph.Next{Many: []string{"node1", "node2", "node3"}}

// Use edges (conditional routing)
graph.Next{} // Let edges determine next node
```

### State & Reducers

**State Requirements**:
- Must be a struct
- Must be JSON-serializable (for persistence)
- Should be designed for partial updates

**Reducer Pattern**:
```go
func reducer(prev, delta MyState) MyState {
    // Merge strategy depends on field type:

    // Replace (last write wins)
    if delta.Name != "" {
        prev.Name = delta.Name
    }

    // Append (accumulate)
    prev.Tags = append(prev.Tags, delta.Tags...)

    // Increment (counters)
    prev.Counter += delta.Counter

    // OR (flags)
    prev.IsComplete = prev.IsComplete || delta.IsComplete

    return prev
}
```

## Next Steps

Now that you have a working workflow, explore more advanced features:

1. **[Building Workflows](./02-building-workflows.md)** - Multi-branch workflows, error handling
2. **[State Management](./03-state-management.md)** - Advanced reducer patterns
3. **[Checkpoints & Resume](./04-checkpoints.md)** - Save and resume workflows
4. **[Conditional Routing](./05-routing.md)** - Dynamic control flow
5. **[Parallel Execution](./06-parallel.md)** - Concurrent node execution
6. **[LLM Integration](./07-llm-integration.md)** - Integrate OpenAI, Anthropic, Google
7. **[Event Tracing](./08-observability.md)** - Advanced observability patterns

## Common Patterns

### Pattern 1: Linear Pipeline

Sequential processing (A → B → C):

```go
engine.Add("A", nodeA)
engine.Add("B", nodeB)
engine.Add("C", nodeC)
engine.StartAt("A")

// In nodes: use graph.Goto("next-node")
```

### Pattern 2: Conditional Branch

Choose path based on state:

```go
engine.Add("check", checkNode)
engine.Add("path_a", pathANode)
engine.Add("path_b", pathBNode)

// Use conditional edges
engine.Connect("check", "path_a", func(s State) bool {
    return s.Score > 0.8
})
engine.Connect("check", "path_b", func(s State) bool {
    return s.Score <= 0.8
})
```

### Pattern 3: Retry Loop

Retry until success:

```go
engine.Add("process", processNode)
engine.Add("validate", validateNode)

// Loop back if validation fails
engine.Connect("validate", "process", func(s State) bool {
    return !s.IsValid && s.Attempts < 3
})
engine.Connect("validate", "complete", func(s State) bool {
    return s.IsValid
})
```

### Pattern 4: Fan-Out

Parallel processing:

```go
func fanOutNode(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{
            Many: []string{"task1", "task2", "task3"},
        },
    }
}
```

## Troubleshooting

### Issue: "MaxSteps exceeded"

**Problem**: Workflow hit the step limit.

**Solution**:
1. Check for infinite loops in your routing
2. Increase `Options.MaxSteps` if workflow is legitimately long
3. Add explicit `graph.Stop()` conditions

### Issue: "No valid route found"

**Problem**: Node returned `Next{}` but no edge matches.

**Solution**:
1. Add explicit routing: `graph.Goto("node-name")`
2. Ensure edge predicates can match
3. Add a default edge: `engine.Connect("node", "default", nil)`

### Issue: "State not merging correctly"

**Problem**: State changes not appearing in final state.

**Solution**:
1. Check your reducer logic
2. Ensure `Delta` fields are populated
3. Verify reducer actually merges fields

## Additional Resources

- **Examples**: Browse [`examples/`](../../examples/) for working code
- **API Reference**: See [docs/api/](../api/) for detailed API documentation
- **Constitution**: Read [constitution.md](../../.specify/memory/constitution.md) for design principles
- **Contributing**: See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development workflow

---

**Ready to build more complex workflows?** Continue to [Building Workflows](./02-building-workflows.md) →
