# API Reference

Complete API reference for LangGraph-Go framework.

## Package Structure

```
github.com/dshills/langgraph-go/
├── graph/              # Core workflow engine
│   ├── model/         # LLM provider interfaces
│   │   ├── openai/    # OpenAI adapter
│   │   ├── anthropic/ # Anthropic adapter
│   │   └── google/    # Google Gemini adapter
│   ├── store/         # State persistence
│   ├── emit/          # Event emission
│   └── tool/          # Tool interface (future)
└── examples/          # Example workflows
```

## Quick Reference

### Core Types

- [`Node[S]`](#node) - Workflow processing unit interface
- [`NodeResult[S]`](#noderesult) - Node execution result
- [`Next`](#next) - Routing specification
- [`Engine[S]`](#engine) - Workflow orchestration engine
- [`Reducer[S]`](#reducer) - State merging function

### Interfaces

- [`Store[S]`](#store) - State persistence
- [`Emitter`](#emitter) - Event emission
- [`ChatModel`](#chatmodel) - LLM provider abstraction
- [`Tool`](#tool) - External tool invocation

---

## Core API

### Node

```go
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

**Description**: Represents a processing unit in the workflow graph. Each node receives the current state, performs computation, and returns modifications.

**Type Parameter**:
- `S` - The state type shared across the workflow

**Methods**:
- `Run(ctx, state)` - Execute node logic and return result

**Usage**:

```go
// Function-based node
nodeFunc := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Processed: true},
        Route: graph.Goto("next"),
    }
})

// Struct-based node
type MyNode struct {
    config Config
}

func (n *MyNode) Run(ctx context.Context, s State) graph.NodeResult[State] {
    // Use n.config...
    return graph.NodeResult[State]{...}
}
```

**See Also**:
- [NodeFunc](#nodefunc) - Function adapter for Node interface
- [Building Workflows](../guides/02-building-workflows.md)

---

### NodeResult

```go
type NodeResult[S any] struct {
    Delta  S       // State changes to apply
    Route  Next    // Routing decision
    Events []Event // Custom events (optional)
    Err    error   // Error if node failed
}
```

**Description**: Contains the output of a node execution.

**Fields**:
- `Delta` - Partial state update to be merged via reducer
- `Route` - Next hop(s) for execution flow
- `Events` - Custom observability events
- `Err` - Node-level error (halts workflow if non-nil)

**Usage**:

```go
// Success with routing
return graph.NodeResult[State]{
    Delta: State{Count: 1},
    Route: graph.Goto("next-node"),
}

// Error
return graph.NodeResult[State]{
    Err: fmt.Errorf("validation failed"),
}

// Fan-out
return graph.NodeResult[State]{
    Delta: State{Phase: "parallel"},
    Route: graph.Next{Many: []string{"task1", "task2", "task3"}},
}

// Custom events
return graph.NodeResult[State]{
    Delta: State{Result: "done"},
    Route: graph.Stop(),
    Events: []emit.Event{
        {Msg: "custom_metric", Meta: map[string]interface{}{"value": 42}},
    },
}
```

---

### Next

```go
type Next struct {
    To       string   // Single next node
    Many     []string // Multiple nodes (fan-out)
    Terminal bool     // Stop execution
}
```

**Description**: Specifies the next step(s) in workflow execution.

**Routing Modes**:
- **Single**: `To` field specifies one node
- **Fan-out**: `Many` field specifies multiple parallel nodes
- **Terminal**: `Terminal=true` stops execution

**Helper Functions**:

```go
// Route to specific node
graph.Goto("node-id")

// Stop execution
graph.Stop()

// Let edges determine routing
graph.Next{}
```

**Usage**:

```go
// Explicit routing
Route: graph.Goto("process")

// Parallel execution
Route: graph.Next{Many: []string{"a", "b", "c"}}

// Terminal node
Route: graph.Stop()

// Use edges (conditional routing)
Route: graph.Next{} // Empty Next uses edges
```

**See Also**:
- [Conditional Routing](../guides/05-routing.md)
- [Parallel Execution](../guides/06-parallel.md)

---

### Engine

```go
type Engine[S any] struct {
    // Fields are private
}

func New[S any](reducer Reducer[S], store Store[S], emitter Emitter, opts Options) *Engine[S]
```

**Description**: Orchestrates workflow execution, handles routing, persistence, and observability.

**Constructor**:
- `New(reducer, store, emitter, opts)` - Create new engine

**Methods**:

```go
// Add a node to the graph
Add(nodeID string, node Node[S]) error

// Set the entry point
StartAt(nodeID string) error

// Connect nodes with conditional edge
Connect(from, to string, predicate Predicate[S]) error

// Execute the workflow
Run(ctx context.Context, runID string, initialState S) (S, error)
```

**Options**:

```go
type Options struct {
    MaxSteps int // Maximum execution steps (prevents infinite loops)
    Retries  int // Retry attempts for failed nodes
}
```

**Usage**:

```go
// Create engine
reducer := func(prev, delta State) State { /*...*/ }
store := store.NewMemStore[State]()
emitter := emit.NewLogEmitter(os.Stdout, false)
opts := graph.Options{MaxSteps: 100, Retries: 3}

engine := graph.New(reducer, store, emitter, opts)

// Build graph
engine.Add("start", startNode)
engine.Add("process", processNode)
engine.StartAt("start")

// Add conditional edges
engine.Connect("process", "success", func(s State) bool {
    return s.IsValid
})

// Execute
final, err := engine.Run(ctx, "run-001", initialState)
```

**See Also**:
- [Getting Started](../guides/01-getting-started.md)

---

### Reducer

```go
type Reducer[S any] func(prev S, delta S) S
```

**Description**: Function that merges state updates from nodes into accumulated state.

**Parameters**:
- `prev` - Current accumulated state
- `delta` - Partial update from node

**Returns**: Updated state

**Usage**:

```go
func reducer(prev, delta State) State {
    // Replace (last write wins)
    if delta.Name != "" {
        prev.Name = delta.Name
    }

    // Append (accumulate)
    prev.Items = append(prev.Items, delta.Items...)

    // Increment (counters)
    prev.Count += delta.Count

    // OR (flags)
    prev.IsComplete = prev.IsComplete || delta.IsComplete

    return prev
}
```

**See Also**:
- [State Management](../guides/03-state-management.md)

---

## Store API

### Store Interface

```go
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, err error)
    SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error
    LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error)
}
```

**Description**: Provides persistence for workflow state and checkpoints.

**Methods**:

**`SaveStep`** - Persist state after node execution
- `runID` - Workflow execution ID
- `step` - Sequential step number
- `nodeID` - Node that produced this state
- `state` - Current workflow state

**`LoadLatest`** - Retrieve most recent state for resumption
- Returns: latest state, step number, error

**`SaveCheckpoint`** - Create named snapshot
- `cpID` - Checkpoint identifier
- `state` - State to snapshot
- `step` - Step number at checkpoint

**`LoadCheckpoint`** - Restore from checkpoint
- Returns: checkpointed state, step number, error

**Implementations**:

```go
// In-memory (testing/development)
store.NewMemStore[State]()

// MySQL (production)
mysql.NewMySQLStore[State](connectionString)
```

**See Also**:
- [Checkpoints & Resume](../guides/04-checkpoints.md)

---

## Emitter API

### Emitter Interface

```go
type Emitter interface {
    Emit(event Event)
}
```

**Description**: Receives workflow events for observability.

**Methods**:
- `Emit(event)` - Process a single event

**Event Structure**:

```go
type Event struct {
    RunID  string                 // Workflow execution ID
    Step   int                    // Sequential step number
    NodeID string                 // Node that emitted event
    Msg    string                 // Event type
    Meta   map[string]interface{} // Additional data
}
```

**Standard Event Types**:
- `node_start` - Node begins execution
- `node_end` - Node completes (includes delta in Meta)
- `routing_decision` - Shows next node(s)
- `error` - Node encountered error

**Implementations**:

```go
// Log to stdout/stderr
emit.NewLogEmitter(os.Stdout, jsonMode)

// Store in memory for analysis
emit.NewBufferedEmitter()

// No-op (zero overhead)
emit.NewNullEmitter()
```

**BufferedEmitter Methods**:

```go
// Get all events for a run
GetHistory(runID string) []Event

// Get filtered events
GetHistoryWithFilter(runID string, filter HistoryFilter) []Event

// Clear events for a run
Clear(runID string)
```

**See Also**:
- [Event Tracing](../guides/08-event-tracing.md)

---

## Model API

### ChatModel Interface

```go
type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}
```

**Description**: Unified interface for LLM providers.

**Methods**:

**`Chat`** - Send messages to LLM
- `ctx` - Context for cancellation/timeout
- `messages` - Conversation history
- `tools` - Optional tool specifications
- Returns: LLM response with text and/or tool calls

**Message Structure**:

```go
type Message struct {
    Role    string // "system", "user", or "assistant"
    Content string // Message text
}

const (
    RoleSystem    = "system"
    RoleUser      = "user"
    RoleAssistant = "assistant"
)
```

**Chat Output**:

```go
type ChatOut struct {
    Text      string     // LLM response text
    ToolCalls []ToolCall // Tools LLM wants to invoke
}

type ToolCall struct {
    Name  string                 // Tool name
    Input map[string]interface{} // Tool parameters
}
```

**Tool Specification**:

```go
type ToolSpec struct {
    Name        string                 // Tool identifier
    Description string                 // What the tool does
    Schema      map[string]interface{} // JSON Schema for parameters
}
```

**Provider Implementations**:

```go
// OpenAI
openai.NewChatModel(apiKey, "gpt-4o")

// Anthropic
anthropic.NewChatModel(apiKey, "claude-sonnet-4-5-20250929")

// Google
google.NewChatModel(apiKey, "gemini-2.5-flash")
```

**Usage**:

```go
model := openai.NewChatModel(apiKey, "gpt-4o")

messages := []model.Message{
    {Role: model.RoleSystem, Content: "You are a helpful assistant."},
    {Role: model.RoleUser, Content: "What is the capital of France?"},
}

out, err := model.Chat(ctx, messages, nil)
if err != nil {
    log.Fatal(err)
}

fmt.Println(out.Text) // "The capital of France is Paris."
```

**See Also**:
- [LLM Integration](../guides/07-llm-integration.md)

---

## Utility Functions

### NodeFunc

```go
func NodeFunc[S any](fn func(context.Context, S) NodeResult[S]) Node[S]
```

**Description**: Adapter that converts a function into a Node.

**Usage**:

```go
node := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{Processed: true},
        Route: graph.Goto("next"),
    }
})

engine.Add("my-node", node)
```

### Routing Helpers

```go
// Route to specific node
func Goto(nodeID string) Next

// Stop execution
func Stop() Next
```

**Usage**:

```go
Route: graph.Goto("next-node")
Route: graph.Stop()
```

---

## Type Constraints

All generic type parameters (`S` for state) must be:
- **Struct types** - Cannot be primitives or interfaces
- **JSON-serializable** - For persistence and checkpointing
- **Copyable** - For parallel execution isolation

**Example Valid State Types**:

```go
✅ type State struct { Count int; Items []string }
✅ type ComplexState struct { Nested NestedStruct; Map map[string]int }

❌ type State int                    // Not a struct
❌ type State interface{}            // Not concrete type
❌ type State struct { Ch chan int } // Not JSON-serializable
```

---

## Error Handling

### Node Errors

```go
// Return error from node
return graph.NodeResult[State]{
    Err: fmt.Errorf("validation failed"),
}
```

**Behavior**: Workflow halts unless retry configured

### Workflow Errors

```go
final, err := engine.Run(ctx, runID, initialState)
if err != nil {
    // Workflow failed
    // - MaxSteps exceeded
    // - Node returned error (after retries)
    // - No valid route found
}
```

### Context Cancellation

All operations respect `context.Context`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

_, err := engine.Run(ctx, runID, initialState)
if errors.Is(err, context.DeadlineExceeded) {
    // Workflow timed out
}
```

---

## Performance Considerations

### State Size

Minimize state size for better performance:
- ✅ Store IDs/references, fetch data as needed
- ❌ Embed large data structures in state

### Parallel Execution

Each parallel branch creates a goroutine:
- Limit fan-out to reasonable numbers (< 100)
- Consider chunking for large-scale parallelism

### Event Emission

Choose emitter based on requirements:
- Development: `LogEmitter`
- Production: `NullEmitter` (zero overhead)
- Analysis: `BufferedEmitter` (memory overhead)

---

## GoDoc

For detailed API documentation with inline examples, see:

```bash
godoc -http=:6060
# Visit http://localhost:6060/pkg/github.com/dshills/langgraph-go/
```

Or online at:
https://pkg.go.dev/github.com/dshills/langgraph-go

---

## See Also

- [Getting Started Guide](../guides/01-getting-started.md)
- [Building Workflows](../guides/02-building-workflows.md)
- [State Management](../guides/03-state-management.md)
- [Examples Directory](../../examples/)
