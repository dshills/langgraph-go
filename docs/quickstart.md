# Quickstart Guide

This guide will help you get started with LangGraph-Go in minutes.

## Installation

```bash
go get github.com/dshills/langgraph-go
```

## Basic Workflow

### 1. Define Your State

State is the data that flows through your workflow. Use any Go struct:

```go
type MyState struct {
    Query    string
    Result   string
    Counter  int
}
```

### 2. Create a Reducer

The reducer merges partial state updates deterministically:

```go
func reducer(prev, delta MyState) MyState {
    // Merge delta into prev
    if delta.Query != "" {
        prev.Query = delta.Query
    }
    if delta.Result != "" {
        prev.Result = delta.Result
    }
    prev.Counter += delta.Counter
    return prev
}
```

### 3. Define Nodes

Nodes are processing steps in your workflow:

```go
// Node as a function
processNode := graph.NodeFunc[MyState](func(ctx context.Context, s MyState) graph.NodeResult[MyState] {
    // Do work
    result := fmt.Sprintf("Processed: %s", s.Query)

    // Return delta state and routing decision
    return graph.NodeResult[MyState]{
        Delta: MyState{Result: result, Counter: 1},
        Route: graph.Stop(), // End workflow
    }
})
```

### 4. Create Engine with Functional Options (Recommended)

Use functional options for a clean, self-documenting API:

```go
import (
    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/store"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Create engine with functional options
engine := graph.New(
    reducer,
    store.NewMemStore[MyState](),
    emit.NewLogEmitter(os.Stdout, false),
    graph.WithMaxConcurrent(8),
    graph.WithQueueDepth(1024),
    graph.WithDefaultNodeTimeout(10*time.Second),
)
```

**Available Functional Options:**

- `WithMaxConcurrent(n int)` - Set max concurrent nodes (default: 8)
- `WithQueueDepth(n int)` - Set frontier queue capacity (default: 1024)
- `WithBackpressureTimeout(d time.Duration)` - Set queue full timeout (default: 30s)
- `WithDefaultNodeTimeout(d time.Duration)` - Set default node timeout (default: 30s)
- `WithRunWallClockBudget(d time.Duration)` - Set total execution timeout (default: 10m)
- `WithReplayMode(bool)` - Enable replay mode (default: false)
- `WithStrictReplay(bool)` - Enable strict replay validation (default: true)
- `WithConflictPolicy(policy ConflictPolicy)` - Set conflict resolution policy

### 5. Build Workflow Graph

Add nodes and define the execution flow:

```go
// Add nodes
engine.Add("process", processNode)
engine.Add("validate", validateNode)

// Define edges (routing)
engine.Connect("process", "validate", nil) // Unconditional edge
engine.Connect("validate", "process", func(s MyState) bool {
    return s.Counter < 3 // Loop condition
})

// Set entry point
engine.StartAt("process")
```

### 6. Run Workflow

```go
ctx := context.Background()
initial := MyState{Query: "hello world"}

final, err := engine.Run(ctx, "run-001", initial)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Final state: %+v\n", final)
```

## Complete Example

```go
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

type WorkflowState struct {
    Query   string
    Result  string
    Counter int
}

func main() {
    // Define reducer
    reducer := func(prev, delta WorkflowState) WorkflowState {
        if delta.Query != "" {
            prev.Query = delta.Query
        }
        if delta.Result != "" {
            prev.Result = delta.Result
        }
        prev.Counter += delta.Counter
        return prev
    }

    // Create engine with functional options
    engine := graph.New(
        reducer,
        store.NewMemStore[WorkflowState](),
        emit.NewLogEmitter(os.Stdout, false),
        graph.WithMaxConcurrent(8),
        graph.WithDefaultNodeTimeout(10*time.Second),
    )

    // Add nodes
    engine.Add("process", graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
        result := fmt.Sprintf("Processed: %s", s.Query)
        return graph.NodeResult[WorkflowState]{
            Delta: WorkflowState{Result: result, Counter: 1},
            Route: graph.Goto("validate"),
        }
    }))

    engine.Add("validate", graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
        if s.Counter >= 3 {
            return graph.NodeResult[WorkflowState]{
                Delta: WorkflowState{},
                Route: graph.Stop(),
            }
        }
        return graph.NodeResult[WorkflowState]{
            Delta: WorkflowState{Counter: 1},
            Route: graph.Goto("process"),
        }
    }))

    // Set start node
    if err := engine.StartAt("process"); err != nil {
        log.Fatal(err)
    }

    // Run workflow
    ctx := context.Background()
    initial := WorkflowState{Query: "hello world"}

    final, err := engine.Run(ctx, "run-001", initial)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Final state: %+v\n", final)
}
```

## Legacy Options Struct (Still Supported)

For backward compatibility, you can still use the Options struct:

```go
opts := graph.Options{
    MaxSteps:           100,
    MaxConcurrentNodes: 8,
    QueueDepth:         1024,
}

engine := graph.New(reducer, store, emitter, opts)
```

## Mixing Both Patterns

You can combine Options struct with functional options:

```go
baseOpts := graph.Options{MaxSteps: 100}

engine := graph.New(
    reducer, store, emitter,
    baseOpts,
    graph.WithMaxConcurrent(16), // Override baseOpts
)
```

## Store Selection

LangGraph-Go provides three store implementations with different trade-offs:

### Memory Store (Testing Only)

For quick testing and prototypes without persistence:

```go
store := store.NewMemStore[MyState]()
// ✅ Zero setup
// ✅ Fastest performance
// ❌ Data lost when process ends
```

### SQLite Store (Development & Small Production)

For local development and single-process production workloads:

```go
import "github.com/dshills/langgraph-go/graph/store"

sqliteStore, err := store.NewSQLiteStore[MyState]("./workflow.db")
if err != nil {
    log.Fatal(err)
}
defer sqliteStore.Close()
```

**SQLite Store Features:**
- ✅ **Zero Configuration**: No database server required
- ✅ **Automatic Setup**: Tables created on first use
- ✅ **Full Persistence**: Data survives restarts
- ✅ **ACID Guarantees**: Full transaction support
- ✅ **WAL Mode**: Concurrent reads enabled
- ✅ **Perfect for**: Development, edge computing, CI/CD, single-process apps
- ❌ **Limitation**: Single-writer (not for distributed systems)

**Quick Example:**
```bash
# Run the SQLite quickstart
cd examples/sqlite_quickstart
go run main.go
```

### MySQL/PostgreSQL Store (Distributed Production)

For high-concurrency distributed systems:

```go
import "github.com/dshills/langgraph-go/graph/store"

mysqlStore, err := store.NewMySQLStore[MyState]("user:pass@tcp(localhost:3306)/dbname")
if err != nil {
    log.Fatal(err)
}
defer mysqlStore.Close()
```

**MySQL Store Features:**
- ✅ **Multi-Writer**: Supports distributed execution
- ✅ **High Concurrency**: 100+ concurrent workers
- ✅ **Network Storage**: Shared database across machines
- ✅ **Connection Pooling**: Automatic connection management
- ✅ **Perfect for**: Production services, microservices, data pipelines
- ❌ **Requires**: Database server setup and management

**Migration Path:**

Start with SQLite for development, migrate to MySQL for production:

```go
// Development: SQLite (zero setup)
store, _ := store.NewSQLiteStore[MyState]("./dev.db")

// Production: MySQL (distributed)
store, _ := store.NewMySQLStore[MyState](os.Getenv("MYSQL_DSN"))
```

Both stores implement the same `Store[S]` interface - **no code changes needed!**

See [store-guarantees.md](store-guarantees.md) for detailed persistence semantics and exactly-once guarantees.

## Node Routing

Nodes control workflow execution via routing decisions:

### Stop Execution

```go
return graph.NodeResult[MyState]{
    Delta: delta,
    Route: graph.Stop(), // End workflow
}
```

### Go to Specific Node

```go
return graph.NodeResult[MyState]{
    Delta: delta,
    Route: graph.Goto("nextNode"), // Continue to specific node
}
```

### Parallel Fan-Out

```go
return graph.NodeResult[MyState]{
    Delta: delta,
    Route: graph.Next{
        Many: []string{"branchA", "branchB", "branchC"},
    },
}
```

### Conditional Routing via Edges

```go
// Node returns empty route - engine uses edges
return graph.NodeResult[MyState]{
    Delta: delta,
    Route: graph.Next{}, // Use edge predicates
}

// Define conditional edges
engine.Connect("router", "pathA", func(s MyState) bool {
    return s.Counter < 10
})
engine.Connect("router", "pathB", func(s MyState) bool {
    return s.Counter >= 10
})
```

## Error Handling

LangGraph-Go provides typed errors for different failure modes:

```go
import "errors"

final, err := engine.Run(ctx, runID, initial)
if err != nil {
    if errors.Is(err, graph.ErrMaxStepsExceeded) {
        // Workflow exceeded MaxSteps limit
    } else if errors.Is(err, graph.ErrBackpressureTimeout) {
        // Frontier queue full for too long
    } else if errors.Is(err, graph.ErrReplayMismatch) {
        // Replay detected logic change
    }
    // Handle error...
}
```

See [error-handling.md](error-handling.md) for complete error reference.

## Observability

### Logging

Use the log emitter for simple text output:

```go
emitter := emit.NewLogEmitter(os.Stdout, false) // false = text, true = JSON
```

### OpenTelemetry Tracing

For production observability with distributed tracing:

```go
import "github.com/dshills/langgraph-go/graph/emit"

emitter := emit.NewOTelEmitter("my-service")
engine := graph.New(reducer, store, emitter, ...)
```

See examples/tracing for complete setup.

## Next Steps

- Read [concurrency.md](concurrency.md) for concurrent execution patterns
- Read [determinism-guarantees.md](determinism-guarantees.md) for replay guarantees
- Read [store-guarantees.md](store-guarantees.md) for persistence semantics
- Explore [examples/](../examples/) for real-world use cases

## Common Patterns

### Loop Pattern

```go
engine.Connect("process", "validate", nil)
engine.Connect("validate", "process", func(s MyState) bool {
    return s.Counter < maxIterations
})
engine.Connect("validate", "complete", func(s MyState) bool {
    return s.Counter >= maxIterations
})
```

### Conditional Branch

```go
engine.Connect("router", "pathA", func(s MyState) bool {
    return s.Score > threshold
})
engine.Connect("router", "pathB", func(s MyState) bool {
    return s.Score <= threshold
})
```

### Human-in-the-Loop

```go
// Save checkpoint before human intervention
engine.SaveCheckpoint(ctx, runID, "before-approval")

// Later: Resume after approval
engine.ResumeFromCheckpoint(ctx, "before-approval", newRunID, "nextNode")
```

See [human-in-the-loop.md](human-in-the-loop.md) for detailed patterns.

## Troubleshooting

### Infinite Loop

**Problem**: Workflow exceeds MaxSteps

**Solution**: Add MaxSteps limit and ensure loop exit conditions:

```go
engine := graph.New(
    reducer, store, emitter,
    graph.Options{MaxSteps: 100},
)
```

### Memory Growth

**Problem**: High memory usage with many concurrent nodes

**Solution**: Reduce MaxConcurrentNodes or increase selective processing:

```go
engine := graph.New(
    reducer, store, emitter,
    graph.WithMaxConcurrent(4), // Reduce from default 8
)
```

### Slow Nodes Blocking

**Problem**: One slow node blocks entire workflow

**Solution**: Set node-specific timeouts:

```go
engine := graph.New(
    reducer, store, emitter,
    graph.WithDefaultNodeTimeout(5*time.Second),
)
```

## Best Practices

1. **Use Functional Options**: Cleaner, more self-documenting API
2. **Set MaxSteps**: Always limit loop iterations to prevent infinite loops
3. **Use Memory Store for Dev**: Fast, zero-config development experience
4. **Use MySQL for Production**: Durable, supports crash recovery
5. **Enable Tracing**: Use OpenTelemetry for production observability
6. **Test Reducers**: Ensure deterministic, commutative merge logic
7. **Keep State Small**: Large state impacts memory and serialization cost
8. **Use Checkpoints**: Save progress for long-running workflows
