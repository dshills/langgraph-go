# Conditional Routing

This guide covers dynamic control flow, edge predicates, routing strategies, and patterns for building adaptive workflows.

## Routing Fundamentals

LangGraph-Go supports three routing mechanisms:

1. **Explicit Routing**: Nodes directly specify next node via `Goto(nodeID)`
2. **Edge-Based Routing**: Predicates evaluate state to determine next node
3. **Fan-Out**: Nodes route to multiple nodes in parallel via `Many`

## Explicit Routing

Nodes control routing directly in `NodeResult`:

```go
func simpleNode(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Phase: "processing"},
        Route: graph.Goto("next-node"), // Explicit routing
    }
}
```

**When to use**:
- Linear pipelines (A → B → C)
- Fixed workflows with no branching
- Routing logic is simple and node-specific

## Edge-Based Routing

Use predicates to conditionally select next node:

### Basic Conditional Edge

```go
// Node returns Next{} to delegate routing to edges
func checkNode(ctx context.Context, s State) graph.NodeResult[State] {
    score := calculateScore(s)

    return graph.NodeResult[State]{
        Delta: State{Score: score},
        Route: graph.Next{}, // Let edges decide
    }
}

// Define conditional edges
engine.Connect("check", "high-path", func(s State) bool {
    return s.Score > 0.8 // High confidence path
})

engine.Connect("check", "low-path", func(s State) bool {
    return s.Score <= 0.8 // Low confidence path
})
```

**When to use**:
- State-based branching
- Multiple possible next nodes
- Routing logic shared across nodes
- Complex decision trees

### Multiple Conditional Edges

Edges are evaluated in the order they were added. First matching edge wins:

```go
// Define edges in priority order
engine.Connect("validator", "success", func(s State) bool {
    return s.IsValid && s.Confidence > 0.9
})

engine.Connect("validator", "retry", func(s State) bool {
    return !s.IsValid && s.RetryCount < 3
})

engine.Connect("validator", "manual-review", func(s State) bool {
    return s.IsValid && s.Confidence <= 0.9
})

engine.Connect("validator", "fail", nil) // Default: no predicate = always match
```

**Evaluation Order**:
1. Check "success" predicate
2. If false, check "retry" predicate
3. If false, check "manual-review" predicate
4. If false, take "fail" edge (no predicate = always true)

### Default Edges

Provide a fallback when no predicates match:

```go
// Specific conditions first
engine.Connect("process", "path-a", func(s State) bool {
    return s.Type == "typeA"
})

engine.Connect("process", "path-b", func(s State) bool {
    return s.Type == "typeB"
})

// Default edge (no predicate)
engine.Connect("process", "path-default", nil)
```

## Common Routing Patterns

### Pattern 1: Confidence-Based Routing

Route based on confidence scores:

```go
type State struct {
    Query      string
    Answer     string
    Confidence float64
    RetryCount int
}

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    answer, confidence := callLLM(s.Query)

    return graph.NodeResult[State]{
        Delta: State{
            Answer:     answer,
            Confidence: confidence,
        },
        Route: graph.Next{}, // Use edges
    }
}

// High confidence: accept answer
engine.Connect("llm", "finalize", func(s State) bool {
    return s.Confidence > 0.85
})

// Medium confidence: verify with another model
engine.Connect("llm", "verify", func(s State) bool {
    return s.Confidence >= 0.6 && s.Confidence <= 0.85
})

// Low confidence: retry or fail
engine.Connect("llm", "retry", func(s State) bool {
    return s.Confidence < 0.6 && s.RetryCount < 3
})

engine.Connect("llm", "fail", func(s State) bool {
    return s.Confidence < 0.6 && s.RetryCount >= 3
})
```

### Pattern 2: Loop with Exit Condition

Iterate until condition is met:

```go
type State struct {
    Data       string
    Iterations int
    Converged  bool
}

func iterateNode(ctx context.Context, s State) graph.NodeResult[State] {
    newData := refine(s.Data)
    converged := isConverged(newData, s.Data)

    return graph.NodeResult[State]{
        Delta: State{
            Data:       newData,
            Iterations: 1, // Reducer increments
            Converged:  converged,
        },
        Route: graph.Next{}, // Use edges
    }
}

// Continue looping
engine.Connect("iterate", "iterate", func(s State) bool {
    return !s.Converged && s.Iterations < 10
})

// Exit loop
engine.Connect("iterate", "complete", func(s State) bool {
    return s.Converged || s.Iterations >= 10
})
```

### Pattern 3: Multi-Factor Decision

Route based on multiple state fields:

```go
type State struct {
    Priority   int
    Urgency    string
    AssignedTo string
}

func routeTicket(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Route: graph.Next{}, // Use edges
    }
}

// Critical tickets: immediate escalation
engine.Connect("route-ticket", "escalate", func(s State) bool {
    return s.Priority >= 9 && s.Urgency == "critical"
})

// High priority assigned tickets: notify assignee
engine.Connect("route-ticket", "notify", func(s State) bool {
    return s.Priority >= 7 && s.AssignedTo != ""
})

// Medium priority: queue for processing
engine.Connect("route-ticket", "queue", func(s State) bool {
    return s.Priority >= 4 && s.Priority < 7
})

// Default: archive
engine.Connect("route-ticket", "archive", nil)
```

### Pattern 4: Error Routing

Route to error handlers based on error type:

```go
type State struct {
    LastError     string
    ErrorType     string
    CanRetry      bool
    RetryCount    int
}

func riskyOp(ctx context.Context, s State) graph.NodeResult[State] {
    err := performOperation(s)
    if err != nil {
        return graph.NodeResult[State]{
            Delta: State{
                LastError: err.Error(),
                ErrorType: classifyError(err),
            },
            Route: graph.Next{}, // Let edges route to error handler
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Status: "success"},
        Route: graph.Goto("complete"),
    }
}

// Transient errors: retry
engine.Connect("risky-op", "retry", func(s State) bool {
    return s.ErrorType == "transient" && s.RetryCount < 3
})

// Rate limit errors: backoff
engine.Connect("risky-op", "backoff", func(s State) bool {
    return s.ErrorType == "rate_limit"
})

// Permission errors: escalate
engine.Connect("risky-op", "escalate", func(s State) bool {
    return s.ErrorType == "permission"
})

// Fatal errors: fail
engine.Connect("risky-op", "fail", nil)
```

### Pattern 5: Dynamic Routing Table

Route based on lookup tables:

```go
type State struct {
    EventType  string
    EventData  map[string]interface{}
}

var routingTable = map[string]string{
    "user.created":   "send-welcome",
    "user.deleted":   "cleanup",
    "order.placed":   "process-payment",
    "order.shipped":  "send-tracking",
}

func eventRouter(ctx context.Context, s State) graph.NodeResult[State] {
    nextNode, exists := routingTable[s.EventType]
    if !exists {
        return graph.NodeResult[State]{
            Route: graph.Goto("unknown-event"),
        }
    }

    return graph.NodeResult[State]{
        Route: graph.Goto(nextNode),
    }
}
```

### Pattern 6: State Machine Routing

Implement finite state machines:

```go
type State struct {
    Phase      string
    Data       string
    Validated  bool
    Processed  bool
}

// State machine transitions
var transitions = map[string]map[string]string{
    "init": {
        "start": "validating",
    },
    "validating": {
        "success": "processing",
        "failure": "error",
    },
    "processing": {
        "success": "finalizing",
        "failure": "retry",
    },
    "retry": {
        "retry": "processing",
        "giveup": "error",
    },
    "finalizing": {
        "done": "complete",
    },
}

func stateNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Determine transition
    var transition string
    if s.Validated {
        transition = "success"
    } else {
        transition = "failure"
    }

    // Look up next phase
    nextPhases := transitions[s.Phase]
    nextPhase := nextPhases[transition]

    return graph.NodeResult[State]{
        Delta: State{Phase: nextPhase},
        Route: graph.Goto(nextPhase + "-node"),
    }
}
```

## Advanced Routing

### Combining Explicit and Edge-Based Routing

Nodes can use explicit routing for happy paths and edge-based for errors:

```go
func smartNode(ctx context.Context, s State) graph.NodeResult[State] {
    result, err := process(s)

    if err != nil {
        // Error case: use edges for error routing
        return graph.NodeResult[State]{
            Delta: State{
                LastError: err.Error(),
                ErrorCode: getErrorCode(err),
            },
            Route: graph.Next{}, // Let edges handle errors
        }
    }

    // Success case: explicit routing
    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Goto("next-node"),
    }
}

// Only define error edges
engine.Connect("smart-node", "retry-handler", func(s State) bool {
    return s.ErrorCode == "RETRY_ABLE"
})

engine.Connect("smart-node", "fail-handler", nil) // Default error handler
```

### Predicate Composition

Build complex predicates from simple ones:

```go
// Simple predicates
func isHighPriority(s State) bool {
    return s.Priority > 7
}

func isAssigned(s State) bool {
    return s.AssignedTo != ""
}

func isUrgent(s State) bool {
    return s.Urgency == "critical"
}

// Composed predicates
func and(predicates ...func(State) bool) func(State) bool {
    return func(s State) bool {
        for _, p := range predicates {
            if !p(s) {
                return false
            }
        }
        return true
    }
}

func or(predicates ...func(State) bool) func(State) bool {
    return func(s State) bool {
        for _, p := range predicates {
            if p(s) {
                return true
            }
        }
        return false
    }
}

func not(predicate func(State) bool) func(State) bool {
    return func(s State) bool {
        return !predicate(s)
    }
}

// Use composed predicates
engine.Connect("route", "escalate", and(isHighPriority, isUrgent))
engine.Connect("route", "assign", and(isHighPriority, not(isAssigned)))
engine.Connect("route", "notify", or(isUrgent, isHighPriority))
```

### Stateful Routing

Track routing history in state:

```go
type State struct {
    RouteHistory []string
    CurrentNode  string
    MaxLoops     int
}

func trackingNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Check for infinite loops
    loopCount := countOccurrences(s.RouteHistory, "tracking-node")
    if loopCount >= s.MaxLoops {
        return graph.NodeResult[State]{
            Route: graph.Goto("loop-breaker"),
        }
    }

    return graph.NodeResult[State]{
        Delta: State{
            RouteHistory: []string{"tracking-node"},
        },
        Route: graph.Next{}, // Continue normal routing
    }
}

func countOccurrences(slice []string, value string) int {
    count := 0
    for _, v := range slice {
        if v == value {
            count++
        }
    }
    return count
}
```

## Testing Routing Logic

### Test Predicates Independently

```go
func TestRoutingPredicates(t *testing.T) {
    tests := []struct {
        name      string
        state     State
        predicate func(State) bool
        want      bool
    }{
        {
            name:      "high confidence",
            state:     State{Confidence: 0.9},
            predicate: func(s State) bool { return s.Confidence > 0.8 },
            want:      true,
        },
        {
            name:      "low confidence",
            state:     State{Confidence: 0.5},
            predicate: func(s State) bool { return s.Confidence > 0.8 },
            want:      false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.predicate(tt.state)
            if got != tt.want {
                t.Errorf("predicate() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Routing Paths

```go
func TestWorkflowRouting(t *testing.T) {
    buffered := emit.NewBufferedEmitter()
    store := store.NewMemStore[State]()
    engine := graph.New(reducer, store, buffered, graph.Options{})

    // Build workflow with routing
    engine.Add("start", startNode)
    engine.Add("high", highNode)
    engine.Add("low", lowNode)

    engine.Connect("start", "high", func(s State) bool {
        return s.Score > 0.8
    })
    engine.Connect("start", "low", func(s State) bool {
        return s.Score <= 0.8
    })

    engine.StartAt("start")

    // Test high score path
    highState := State{Score: 0.9}
    final, err := engine.Run(context.Background(), "test-high", highState)
    if err != nil {
        t.Fatal(err)
    }

    // Verify high path was taken
    events := buffered.GetHistory("test-high")
    routedToHigh := false
    for _, evt := range events {
        if evt.Msg == "routing_decision" {
            if nextNode, ok := evt.Meta["next_node"].(string); ok && nextNode == "high" {
                routedToHigh = true
            }
        }
    }
    if !routedToHigh {
        t.Error("expected high confidence path")
    }

    // Test low score path
    lowState := State{Score: 0.5}
    engine.Run(context.Background(), "test-low", lowState)

    // Verify low path was taken
    events = buffered.GetHistory("test-low")
    routedToLow := false
    for _, evt := range events {
        if evt.Msg == "routing_decision" {
            if nextNode, ok := evt.Meta["next_node"].(string); ok && nextNode == "low" {
                routedToLow = true
            }
        }
    }
    if !routedToLow {
        t.Error("expected low confidence path")
    }
}
```

## Best Practices

### 1. Keep Predicates Pure

Predicates should only read state, not modify it or perform side effects:

```go
// ❌ BAD: Modifies state
func badPredicate(s State) bool {
    s.Counter++ // Don't mutate!
    return s.Counter > 10
}

// ❌ BAD: Side effects
func badPredicate2(s State) bool {
    log.Println("Evaluating predicate") // Don't log!
    return s.Score > 0.8
}

// ✅ GOOD: Pure function
func goodPredicate(s State) bool {
    return s.Score > 0.8
}
```

### 2. Make Predicates Mutually Exclusive

Avoid overlapping conditions:

```go
// ❌ BAD: Overlapping predicates
engine.Connect("node", "path-a", func(s State) bool {
    return s.Score > 0.7 // Could match 0.85
})
engine.Connect("node", "path-b", func(s State) bool {
    return s.Score > 0.8 // Could also match 0.85
})

// ✅ GOOD: Mutually exclusive
engine.Connect("node", "path-a", func(s State) bool {
    return s.Score > 0.8
})
engine.Connect("node", "path-b", func(s State) bool {
    return s.Score > 0.7 && s.Score <= 0.8
})
engine.Connect("node", "path-c", func(s State) bool {
    return s.Score <= 0.7
})
```

### 3. Always Provide a Default Edge

Prevent routing failures with a fallback:

```go
// Specific conditions
engine.Connect("router", "path-a", specificCondition)
engine.Connect("router", "path-b", anotherCondition)

// Always add a default
engine.Connect("router", "default-path", nil)
```

### 4. Document Routing Logic

Make routing decisions clear:

```go
// Document complex predicates
engine.Connect("process", "high-value-path", func(s State) bool {
    // Route to high-value path if:
    // - Customer is premium tier
    // - Order value exceeds $1000
    // - Not a refund/return
    return s.CustomerTier == "premium" &&
           s.OrderValue > 1000 &&
           s.OrderType != "refund"
})
```

---

**Next:** Learn about parallel execution with [Parallel Execution](./06-parallel.md) →
