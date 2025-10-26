# State Management

This guide covers advanced state management patterns, reducer strategies, and best practices for designing robust workflow state.

## Understanding State in LangGraph-Go

State is the shared context that flows through your workflow. Every node reads the current state, performs computation, and returns a partial update (delta) that gets merged back into the accumulated state.

### State Requirements

1. **Must be a struct** - Go generics require concrete types
2. **Must be JSON-serializable** - For persistence and checkpointing
3. **Should support partial updates** - Nodes return deltas, not full state
4. **Should be immutable-friendly** - Reducers create new values, not mutate

## Reducer Patterns

The reducer function determines how state updates are merged. It's called after every node execution:

```go
func reducer(prev State, delta State) State
```

### Pattern 1: Last Write Wins (Replace)

Use for scalar values where the latest value is always correct:

```go
type State struct {
    Status      string  // "pending" → "processing" → "complete"
    CurrentStep string  // Latest step name
    Temperature float64 // Latest setting
}

func reducer(prev, delta State) State {
    // Replace if delta provides new value
    if delta.Status != "" {
        prev.Status = delta.Status
    }

    if delta.CurrentStep != "" {
        prev.CurrentStep = delta.CurrentStep
    }

    if delta.Temperature != 0.0 {
        prev.Temperature = delta.Temperature
    }

    return prev
}
```

**When to use**: Configuration values, status fields, latest measurements

### Pattern 2: Append (Accumulate)

Use for lists where all updates should be collected:

```go
type State struct {
    Messages    []string
    Errors      []error
    Checkpoints []string
    ProcessedBy []string
}

func reducer(prev, delta State) State {
    // Append all new items
    prev.Messages = append(prev.Messages, delta.Messages...)
    prev.Errors = append(prev.Errors, delta.Errors...)
    prev.Checkpoints = append(prev.Checkpoints, delta.Checkpoints...)
    prev.ProcessedBy = append(prev.ProcessedBy, delta.ProcessedBy...)

    return prev
}
```

**When to use**: Event logs, message history, error collection, agent chains

### Pattern 3: Increment (Counters)

Use for numeric values that should accumulate:

```go
type State struct {
    StepCount    int
    RetryCount   int
    TokensUsed   int
    ElapsedMs    int64
    BytesRead    int64
}

func reducer(prev, delta State) State {
    // Add deltas to previous values
    prev.StepCount += delta.StepCount
    prev.RetryCount += delta.RetryCount
    prev.TokensUsed += delta.TokensUsed
    prev.ElapsedMs += delta.ElapsedMs
    prev.BytesRead += delta.BytesRead

    return prev
}
```

**When to use**: Counters, metrics, resource tracking, duration accumulation

### Pattern 4: Merge Maps

Use for key-value stores where new keys are added and existing keys are updated:

```go
type State struct {
    Metadata     map[string]string
    Scores       map[string]float64
    AgentResults map[string]interface{}
}

func reducer(prev, delta State) State {
    // Initialize maps if nil
    if prev.Metadata == nil {
        prev.Metadata = make(map[string]string)
    }
    if prev.Scores == nil {
        prev.Scores = make(map[string]float64)
    }
    if prev.AgentResults == nil {
        prev.AgentResults = make(map[string]interface{})
    }

    // Merge delta maps into prev
    for k, v := range delta.Metadata {
        prev.Metadata[k] = v
    }
    for k, v := range delta.Scores {
        prev.Scores[k] = v
    }
    for k, v := range delta.AgentResults {
        prev.AgentResults[k] = v
    }

    return prev
}
```

**When to use**: Metadata, key-value stores, agent outputs, configuration overrides

### Pattern 5: Boolean Flags (OR/AND)

Use for flags that should be set once or require consensus:

```go
type State struct {
    // OR logic - once true, stays true
    Validated  bool
    HasError   bool
    IsComplete bool

    // AND logic - all must agree
    AllAgree   *bool // Pointer for three-state: nil, false, true
}

func reducer(prev, delta State) State {
    // OR: Set to true if delta is true
    prev.Validated = prev.Validated || delta.Validated
    prev.HasError = prev.HasError || delta.HasError
    prev.IsComplete = prev.IsComplete || delta.IsComplete

    // AND: Requires special handling
    if delta.AllAgree != nil {
        if prev.AllAgree == nil {
            prev.AllAgree = delta.AllAgree
        } else {
            agree := *prev.AllAgree && *delta.AllAgree
            prev.AllAgree = &agree
        }
    }

    return prev
}
```

**When to use**: Validation flags, error states, completion markers, consensus tracking

### Pattern 6: Conditional Merging

Use when merge logic depends on state values:

```go
type State struct {
    BestScore   float64
    BestAnswer  string
    WorstScore  float64
    WorstAnswer string
}

func reducer(prev, delta State) State {
    // Keep best score
    if delta.BestScore > prev.BestScore {
        prev.BestScore = delta.BestScore
        prev.BestAnswer = delta.BestAnswer
    }

    // Keep worst score (if initialized)
    if prev.WorstScore == 0 || delta.WorstScore < prev.WorstScore {
        prev.WorstScore = delta.WorstScore
        prev.WorstAnswer = delta.WorstAnswer
    }

    return prev
}
```

**When to use**: Best/worst tracking, ranking, optimization, competitive values

### Pattern 7: Deep Merge (Nested Structs)

Use for complex hierarchical state:

```go
type UserRequest struct {
    Query    string
    Filters  map[string]string
    Priority int
}

type ProcessingState struct {
    Phase      string
    Confidence float64
    Attempts   int
}

type Results struct {
    Items      []Item
    TotalCount int
    Summary    string
}

type WorkflowState struct {
    Request    UserRequest
    Processing ProcessingState
    Results    Results
}

func reducer(prev, delta WorkflowState) WorkflowState {
    // Merge Request (replace non-empty values)
    if delta.Request.Query != "" {
        prev.Request.Query = delta.Request.Query
    }
    if delta.Request.Priority != 0 {
        prev.Request.Priority = delta.Request.Priority
    }
    // Merge Request.Filters map
    if prev.Request.Filters == nil {
        prev.Request.Filters = make(map[string]string)
    }
    for k, v := range delta.Request.Filters {
        prev.Request.Filters[k] = v
    }

    // Merge Processing (replace + increment)
    if delta.Processing.Phase != "" {
        prev.Processing.Phase = delta.Processing.Phase
    }
    if delta.Processing.Confidence != 0 {
        prev.Processing.Confidence = delta.Processing.Confidence
    }
    prev.Processing.Attempts += delta.Processing.Attempts

    // Merge Results (append + increment + replace)
    prev.Results.Items = append(prev.Results.Items, delta.Results.Items...)
    prev.Results.TotalCount += delta.Results.TotalCount
    if delta.Results.Summary != "" {
        prev.Results.Summary = delta.Results.Summary
    }

    return prev
}
```

**When to use**: Complex workflows, multi-phase processing, hierarchical data

## State Design Best Practices

### 1. Design for Partial Updates

Nodes should only set fields they modify:

```go
// ❌ BAD: Node returns full state
func badNode(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{
            Query:      s.Query,      // Unchanged - don't copy
            Status:     "processing", // Changed
            RetryCount: s.RetryCount, // Unchanged - don't copy
        },
        Route: graph.Goto("next"),
    }
}

// ✅ GOOD: Node returns only changes
func goodNode(ctx context.Context, s State) graph.NodeResult[State] {
    return graph.NodeResult[State]{
        Delta: State{
            Status: "processing", // Only changed field
        },
        Route: graph.Goto("next"),
    }
}
```

### 2. Use Zero Values Carefully

Zero values (0, "", false, nil) are often skipped in reducers. Use pointers for tri-state:

```go
type State struct {
    // Can't distinguish "not set" from "set to zero"
    Temperature float64 // 0.0 means "not set" or "set to zero"?

    // Can distinguish three states: nil, false, true
    Enabled *bool
}

func reducer(prev, delta State) State {
    // Skip zero temperature
    if delta.Temperature != 0.0 {
        prev.Temperature = delta.Temperature
    }

    // Explicit nil check for pointer
    if delta.Enabled != nil {
        prev.Enabled = delta.Enabled
    }

    return prev
}
```

### 3. Initialize Maps and Slices

Always initialize nil maps before merging:

```go
func reducer(prev, delta State) State {
    // ❌ BAD: Panic if prev.Metadata is nil
    for k, v := range delta.Metadata {
        prev.Metadata[k] = v
    }

    // ✅ GOOD: Initialize first
    if prev.Metadata == nil {
        prev.Metadata = make(map[string]string)
    }
    for k, v := range delta.Metadata {
        prev.Metadata[k] = v
    }

    return prev
}
```

### 4. Keep State Minimal

Only include data needed for workflow execution:

```go
// ❌ BAD: Large intermediate data
type BadState struct {
    RawHTML     string        // Megabytes of HTML
    AllDocs     []Document    // Thousands of documents
    FullHistory []StateChange // Every state change
}

// ✅ GOOD: References and summaries
type GoodState struct {
    DocumentIDs []string      // Just IDs, fetch from DB
    DocCount    int           // Summary statistics
    LastChange  StateChange   // Only latest change
}
```

### 5. Use Semantic Field Names

Field names should indicate merge behavior:

```go
type State struct {
    // Clear intent: accumulate
    TotalTokens    int
    AllMessages    []string
    ErrorHistory   []error

    // Clear intent: replace
    CurrentPhase   string
    LatestScore    float64
    FinalResult    string

    // Clear intent: flags
    IsValidated    bool
    HasCompleted   bool
}
```

## Advanced Patterns

### Pattern: Versioned State

Track state version for debugging:

```go
type State struct {
    Version    int    // Increments on every merge
    Data       string
    ModifiedBy string
}

func reducer(prev, delta State) State {
    prev.Version++ // Always increment

    if delta.Data != "" {
        prev.Data = delta.Data
    }
    if delta.ModifiedBy != "" {
        prev.ModifiedBy = delta.ModifiedBy
    }

    return prev
}
```

### Pattern: State Validation

Validate state invariants in reducer:

```go
func reducer(prev, delta State) State {
    // Merge normally
    prev.Count += delta.Count
    prev.Items = append(prev.Items, delta.Items...)

    // Enforce invariants
    if prev.Count < 0 {
        prev.Count = 0
    }
    if len(prev.Items) > 1000 {
        prev.Items = prev.Items[:1000] // Cap at 1000
    }

    return prev
}
```

### Pattern: Conflict Resolution

Handle conflicting updates:

```go
type State struct {
    Value     string
    Timestamp time.Time
}

func reducer(prev, delta State) State {
    // Keep newer value based on timestamp
    if delta.Timestamp.After(prev.Timestamp) {
        prev.Value = delta.Value
        prev.Timestamp = delta.Timestamp
    }

    return prev
}
```

### Pattern: Multi-Value Aggregation

Collect and aggregate values from parallel nodes:

```go
type State struct {
    Scores     []float64
    AvgScore   float64
    MinScore   float64
    MaxScore   float64
}

func reducer(prev, delta State) State {
    // Collect all scores
    prev.Scores = append(prev.Scores, delta.Scores...)

    // Recalculate aggregates
    if len(prev.Scores) > 0 {
        sum := 0.0
        min := prev.Scores[0]
        max := prev.Scores[0]

        for _, score := range prev.Scores {
            sum += score
            if score < min {
                min = score
            }
            if score > max {
                max = score
            }
        }

        prev.AvgScore = sum / float64(len(prev.Scores))
        prev.MinScore = min
        prev.MaxScore = max
    }

    return prev
}
```

## State Migration

### Handling Schema Changes

When your state schema evolves, use versioning:

```go
type StateV1 struct {
    Version int
    Name    string
}

type StateV2 struct {
    Version   int
    FirstName string
    LastName  string
}

func migrateState(v1 StateV1) StateV2 {
    parts := strings.Split(v1.Name, " ")
    return StateV2{
        Version:   2,
        FirstName: parts[0],
        LastName:  strings.Join(parts[1:], " "),
    }
}
```

### Backward Compatibility

Design reducers to handle both old and new formats:

```go
type State struct {
    // New field
    Items []Item

    // Deprecated field (keep for backward compat)
    LegacyData string `json:"legacy_data,omitempty"`
}

func reducer(prev, delta State) State {
    // Merge new format
    prev.Items = append(prev.Items, delta.Items...)

    // Support legacy format if present
    if delta.LegacyData != "" {
        item := parseLegacyData(delta.LegacyData)
        prev.Items = append(prev.Items, item)
    }

    return prev
}
```

## Common Pitfalls

### Pitfall 1: Mutating Previous State

```go
// ❌ BAD: Modifies prev.Items slice
func badReducer(prev, delta State) State {
    prev.Items[0] = delta.Items[0] // Mutation!
    return prev
}

// ✅ GOOD: Create new slice
func goodReducer(prev, delta State) State {
    items := make([]Item, len(prev.Items))
    copy(items, prev.Items)
    items[0] = delta.Items[0]
    prev.Items = items
    return prev
}
```

### Pitfall 2: Forgetting to Return

```go
// ❌ BAD: Doesn't return updated state
func badReducer(prev, delta State) State {
    prev.Count += delta.Count
    // Missing return!
}

// ✅ GOOD: Always return
func goodReducer(prev, delta State) State {
    prev.Count += delta.Count
    return prev
}
```

### Pitfall 3: Over-Complicated Logic

```go
// ❌ BAD: Complex branching
func badReducer(prev, delta State) State {
    if delta.Type == "A" {
        if prev.Count > 10 {
            // Complex logic...
        } else {
            // More complex logic...
        }
    } else if delta.Type == "B" {
        // Even more logic...
    }
    return prev
}

// ✅ GOOD: Simple, predictable merging
func goodReducer(prev, delta State) State {
    // Merge each field independently
    if delta.Count != 0 {
        prev.Count = delta.Count
    }
    if delta.Type != "" {
        prev.Type = delta.Type
    }
    return prev
}
```

## Testing Reducers

Always test reducer logic independently:

```go
func TestReducer(t *testing.T) {
    tests := []struct {
        name  string
        prev  State
        delta State
        want  State
    }{
        {
            name:  "append messages",
            prev:  State{Messages: []string{"a"}},
            delta: State{Messages: []string{"b", "c"}},
            want:  State{Messages: []string{"a", "b", "c"}},
        },
        {
            name:  "increment counter",
            prev:  State{Count: 5},
            delta: State{Count: 3},
            want:  State{Count: 8},
        },
        {
            name:  "replace status",
            prev:  State{Status: "pending"},
            delta: State{Status: "complete"},
            want:  State{Status: "complete"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := reducer(tt.prev, tt.delta)
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("reducer() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

**Next:** Learn how to save and resume workflows with [Checkpoints & Resume](./04-checkpoints.md) →
