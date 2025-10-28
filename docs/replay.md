# Deterministic Replay Guide

LangGraph-Go v0.2.0 provides deterministic replay capabilities that enable exact reproduction of workflow executions for debugging, auditing, and testing purposes. This guide explains how to record, replay, and debug executions.

## Table of Contents

- [Overview](#overview)
- [Why Deterministic Replay?](#why-deterministic-replay)
- [Recording Executions](#recording-executions)
  - [Automatic Checkpointing](#automatic-checkpointing)
  - [I/O Recording](#io-recording)
  - [Random Number Generation](#random-number-generation)
- [Replaying Executions](#replaying-executions)
  - [Replay Modes](#replay-modes)
  - [Strict vs Lenient Replay](#strict-vs-lenient-replay)
- [Debugging with Replay](#debugging-with-replay)
  - [Reproducing Failures](#reproducing-failures)
  - [Time-Travel Debugging](#time-travel-debugging)
  - [Comparing Executions](#comparing-executions)
- [Writing Replayable Nodes](#writing-replayable-nodes)
  - [Using Seeded RNG](#using-seeded-rng)
  - [Recording External I/O](#recording-external-io)
  - [Avoiding Non-Determinism](#avoiding-non-determinism)
- [Troubleshooting](#troubleshooting)
- [Advanced Topics](#advanced-topics)

## Overview

Deterministic replay enables you to:

- **Debug Production Issues**: Reproduce failures exactly as they occurred
- **Audit Compliance**: Verify workflow behavior for regulatory requirements
- **Test Workflows**: Replay executions without external dependencies
- **Performance Analysis**: Profile identical executions repeatedly

**Key Guarantees:**

✅ Same state transitions across replays
✅ Same routing decisions
✅ Same random values (via seeded RNG)
✅ No external API calls during replay
✅ Identical final state

## Why Deterministic Replay?

### Problem: Non-Deterministic Failures

```go
// Production execution fails at step 147
Error: API rate limit exceeded at node "fetch-data"
State: {...complex state...}

// Traditional debugging challenges:
// - Can't reproduce in test environment (different rate limits)
// - Can't see intermediate state at failure point
// - External APIs have changed since original execution
```

### Solution: Replay from Checkpoint

```go
// 1. Load checkpoint from production failure
checkpoint, err := store.LoadCheckpointV2(ctx, "prod-run-147", "")

// 2. Replay execution using recorded I/O
opts := graph.Options{
    ReplayMode:   true,    // Use recorded I/O
    StrictReplay: true,    // Enforce exact matching
}

// 3. Reproduce exact failure with full context
final, err := engine.RunWithCheckpoint(ctx, "debug-replay", checkpoint, opts)
// Error: API rate limit exceeded (reproduced exactly)
// Full state available for inspection
```

## Recording Executions

### Automatic Checkpointing

LangGraph-Go automatically saves checkpoints after each step:

```go
// Enable automatic checkpointing via Store
store := store.NewMySQLStore[MyState](db)

engine := graph.New(reducer, store, emitter, opts)

// Every step is automatically checkpointed with:
// - Current state
// - Frontier (queued work items)
// - RNG seed
// - Recorded I/O (if enabled)
// - Idempotency key
final, err := engine.Run(ctx, "auto-checkpoint-run", initialState)
```

**Checkpoint Structure:**

```go
type Checkpoint[S any] struct {
    RunID         string          // Unique run identifier
    StepID        int             // Step number (monotonic)
    State         S               // Accumulated state
    Frontier      []WorkItem[S]   // Queued work items
    RNGSeed       int64           // Deterministic RNG seed
    RecordedIOs   []RecordedIO    // Captured I/O interactions
    IdempotencyKey string         // Deduplication key
    Timestamp     time.Time       // Checkpoint creation time
    Label         string          // Optional human-readable label
}
```

### I/O Recording

Record external interactions for replay:

```go
type RecordedIO struct {
    NodeID    string        // Node that performed I/O
    Attempt   int           // Retry attempt number
    Request   interface{}   // Serialized request
    Response  interface{}   // Serialized response
    Hash      string        // SHA-256 of request+response
    Timestamp time.Time     // I/O timestamp
    Duration  time.Duration // I/O duration
}
```

**Recording Example:**

```go
// Node with I/O recording
func (n *APINode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    // Check if replaying
    if replay, ok := ctx.Value("replay").(bool); ok && replay {
        // Use recorded response
        recorded := lookupRecordedIO(ctx, n.NodeID)
        return NodeResult[MyState]{
            Delta: MyState{APIData: recorded.Response},
            Route: Goto("process"),
        }
    }

    // Normal execution: perform I/O and record
    request := buildAPIRequest(state)
    response, err := n.client.Call(request)

    // Record I/O for replay
    recordIO(ctx, RecordedIO{
        NodeID:   n.NodeID,
        Request:  request,
        Response: response,
        Hash:     computeHash(request, response),
    })

    delta := MyState{APIData: response}
    return NodeResult[MyState]{Delta: delta, Route: Goto("process")}
}
```

### Random Number Generation

Use seeded RNG for deterministic randomness:

```go
// ✅ Good: Use context-provided RNG
func (n *RandomNode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    // Retrieve seeded RNG from context
    rng := ctx.Value(RNGKey).(*rand.Rand)
    if rng == nil {
        // Fallback for tests
        rng = rand.New(rand.NewSource(time.Now().UnixNano()))
    }

    // Generate deterministic random value
    randomChoice := rng.Intn(10)

    delta := MyState{Choice: randomChoice}
    return NodeResult[MyState]{Delta: delta, Route: Goto("next")}
}

// ❌ Bad: Use global rand (non-deterministic)
func (n *RandomNode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    randomChoice := rand.Intn(10) // Different on each execution!

    delta := MyState{Choice: randomChoice}
    return NodeResult[MyState]{Delta: delta, Route: Goto("next")}
}
```

**RNG Seeding:**

```go
// Engine automatically seeds RNG based on runID
rngSeed := hash(runID) // Deterministic seed from run ID

// Same runID always produces same random sequence
rng := rand.New(rand.NewSource(rngSeed))

// Inject into context for nodes
ctx = context.WithValue(ctx, RNGKey, rng)
```

## Replaying Executions

### Replay Modes

LangGraph-Go supports two replay modes:

**1. Full Replay** (from beginning):

```go
// Load checkpoint from any step
checkpoint, err := store.LoadCheckpointV2(ctx, "original-run", "step-42")

// Replay from beginning using recorded I/O
opts := graph.Options{
    ReplayMode:   true,
    StrictReplay: true,
}

finalState, err := engine.ReplayRun(ctx, "replay-run", checkpoint)
// Executes steps 0-42 using recorded I/O, produces identical state
```

**2. Checkpoint Resume** (continue from checkpoint):

```go
// Resume execution from checkpoint
checkpoint, err := store.LoadCheckpointV2(ctx, "original-run", "step-42")

// Continue execution with fresh I/O
opts := graph.Options{
    ReplayMode: false, // Live execution from this point
}

finalState, err := engine.RunWithCheckpoint(ctx, "resume-run", checkpoint, opts)
// Continues from step 42 with new I/O
```

### Strict vs Lenient Replay

**Strict Replay** (default): Enforces exact I/O matching

```go
opts := graph.Options{
    ReplayMode:   true,
    StrictReplay: true, // Raises error on I/O mismatch
}

// If node output differs from recorded:
// Error: ErrReplayMismatch - Hash mismatch at node "api-call"
//   Expected: 0x1234abcd
//   Got:      0x5678efgh
```

**Lenient Replay**: Allows I/O deviations

```go
opts := graph.Options{
    ReplayMode:   true,
    StrictReplay: false, // Warns on mismatch, continues
}

// If node output differs from recorded:
// Warning: Replay mismatch at node "api-call", continuing anyway
// Useful for debugging schema changes or non-critical differences
```

## Debugging with Replay

### Reproducing Failures

Step-by-step guide to debugging production failures:

```go
// 1. Capture failure context from production
// (Automatic via checkpoint system)

// 2. Load checkpoint in development
func debugProductionFailure() {
    ctx := context.Background()

    // Load checkpoint from production database
    prodStore := store.NewMySQLStore[MyState](prodDB)
    checkpoint, err := prodStore.LoadCheckpointV2(ctx, "prod-run-failed-123", "")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Loaded checkpoint from step %d", checkpoint.StepID)
    log.Printf("State at failure: %+v", checkpoint.State)

    // 3. Replay in debug environment
    opts := graph.Options{
        ReplayMode:   true,
        StrictReplay: true,
    }

    devStore := store.NewMemStore[MyState]() // Local replay
    emitter := emit.NewLogEmitter(os.Stdout, true) // Verbose logging

    engine := graph.New(reducer, devStore, emitter, opts)
    // ... configure graph topology

    // 4. Reproduce exact failure
    final, err := engine.ReplayRun(ctx, "debug-replay", checkpoint)

    // 5. Inspect state at failure point
    if err != nil {
        log.Printf("Reproduced error: %v", err)
        log.Printf("State at failure: %+v", final)

        // Analyze state to identify root cause
        if final.APIRetries > 5 {
            log.Println("Root cause: Excessive API retries")
        }
    }
}
```

### Time-Travel Debugging

Replay to specific steps to inspect intermediate state:

```go
// Replay to step 42 to inspect state before failure
checkpoint, err := store.LoadCheckpointV2(ctx, "original-run", "step-42")

log.Printf("State at step 42: %+v", checkpoint.State)
log.Printf("Frontier at step 42: %+v", checkpoint.Frontier)

// Replay to step 43 to see what changed
checkpoint43, err := store.LoadCheckpointV2(ctx, "original-run", "step-43")

log.Printf("State at step 43: %+v", checkpoint43.State)

// Compare states
diff := compareStates(checkpoint.State, checkpoint43.State)
log.Printf("Changes in step 43: %+v", diff)
```

### Comparing Executions

Compare original and replayed executions to verify correctness:

```go
func verifyReplayCorrectness() {
    // 1. Run original execution
    originalFinal, err := engine.Run(ctx, "original", initialState)
    originalCheckpoint, _ := store.LoadCheckpointV2(ctx, "original", "")

    // 2. Replay execution
    opts := graph.Options{ReplayMode: true}
    replayFinal, err := engine.ReplayRun(ctx, "replay", originalCheckpoint)

    // 3. Compare final states
    if !reflect.DeepEqual(originalFinal, replayFinal) {
        log.Fatal("Replay produced different final state!")
    }

    // 4. Compare step-by-step checkpoints
    for step := 0; step <= originalCheckpoint.StepID; step++ {
        origStep, _ := store.LoadCheckpointV2(ctx, "original", fmt.Sprintf("step-%d", step))
        replayStep, _ := store.LoadCheckpointV2(ctx, "replay", fmt.Sprintf("step-%d", step))

        if !reflect.DeepEqual(origStep.State, replayStep.State) {
            log.Printf("Divergence at step %d:", step)
            log.Printf("  Original: %+v", origStep.State)
            log.Printf("  Replay:   %+v", replayStep.State)
            break
        }
    }

    log.Println("✅ Replay verified: identical execution")
}
```

## Writing Replayable Nodes

### Using Seeded RNG

Always use the context-provided RNG for randomness:

```go
type DecisionNode struct{}

func (n *DecisionNode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    // ✅ Get seeded RNG from context
    rng, ok := ctx.Value(RNGKey).(*rand.Rand)
    if !ok || rng == nil {
        // Fallback for tests or non-replay scenarios
        rng = rand.New(rand.NewSource(42)) // Fixed seed for tests
    }

    // Generate deterministic random decision
    threshold := 0.5
    randomValue := rng.Float64()

    var route Next
    if randomValue < threshold {
        route = Goto("option-a")
    } else {
        route = Goto("option-b")
    }

    delta := MyState{
        Decision:    route.Goto,
        RandomValue: randomValue, // Store for debugging
    }

    return NodeResult[MyState]{Delta: delta, Route: route}
}
```

### Recording External I/O

Wrap external calls with recording logic:

```go
type APINode struct {
    client *http.Client
}

func (n *APINode) Run(ctx context.Context, state MyState) NodeResult[MyState] {
    // Check replay mode
    if isReplay(ctx) {
        // Use recorded response
        recorded := getRecordedIO(ctx)
        return NodeResult[MyState]{
            Delta: MyState{APIResponse: recorded.Response.(string)},
            Route: Goto("process"),
        }
    }

    // Normal execution with recording
    req := buildRequest(state.Query)

    // Perform I/O
    startTime := time.Now()
    resp, err := n.client.Do(req)
    duration := time.Since(startTime)

    if err != nil {
        return NodeResult[MyState]{Err: err}
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    // Record I/O for future replay
    recordIO(ctx, RecordedIO{
        NodeID:   "api-node",
        Attempt:  getAttempt(ctx),
        Request:  req,
        Response: string(body),
        Hash:     computeHash(req, body),
        Duration: duration,
    })

    delta := MyState{APIResponse: string(body)}
    return NodeResult[MyState]{Delta: delta, Route: Goto("process")}
}

// Helper functions
func isReplay(ctx context.Context) bool {
    replay, ok := ctx.Value("replay_mode").(bool)
    return ok && replay
}

func getRecordedIO(ctx context.Context) RecordedIO {
    recorded := ctx.Value("recorded_io").(RecordedIO)
    return recorded
}

func recordIO(ctx context.Context, io RecordedIO) {
    // Store I/O in context for checkpoint system to capture
    // (Implementation provided by engine)
}

func computeHash(req *http.Request, resp []byte) string {
    h := sha256.New()
    h.Write([]byte(req.URL.String()))
    h.Write([]byte(req.Method))
    h.Write(resp)
    return fmt.Sprintf("%x", h.Sum(nil))
}
```

### Avoiding Non-Determinism

Common sources of non-determinism and how to fix them:

**1. Time-based values:**

```go
// ❌ Bad: Current time (different on replay)
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    delta := S{Timestamp: time.Now()}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// ✅ Good: Use execution start time from state
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    // State includes ExecutionStartTime from initial state
    delta := S{Timestamp: state.ExecutionStartTime}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}
```

**2. Global random state:**

```go
// ❌ Bad: Global rand package
import "math/rand"

func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    value := rand.Intn(100) // Non-deterministic!
    delta := S{RandomValue: value}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// ✅ Good: Context-provided seeded RNG
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    rng := ctx.Value(RNGKey).(*rand.Rand)
    value := rng.Intn(100) // Deterministic!
    delta := S{RandomValue: value}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}
```

**3. Map iteration order:**

```go
// ❌ Bad: Iterate map directly (non-deterministic order in Go)
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    var results []string
    for key, value := range state.DataMap {
        results = append(results, fmt.Sprintf("%s=%s", key, value))
    }
    // results order is non-deterministic!

    delta := S{Summary: strings.Join(results, ",")}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// ✅ Good: Sort keys before iteration
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    // Extract and sort keys
    keys := make([]string, 0, len(state.DataMap))
    for key := range state.DataMap {
        keys = append(keys, key)
    }
    sort.Strings(keys)

    // Iterate in sorted order
    var results []string
    for _, key := range keys {
        value := state.DataMap[key]
        results = append(results, fmt.Sprintf("%s=%s", key, value))
    }

    delta := S{Summary: strings.Join(results, ",")}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}
```

**4. Concurrency without ordering:**

```go
// ❌ Bad: Goroutines with channels (non-deterministic order)
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    results := make(chan string, 3)

    go func() { results <- fetchA() }()
    go func() { results <- fetchB() }()
    go func() { results <- fetchC() }()

    var data []string
    for i := 0; i < 3; i++ {
        data = append(data, <-results) // Order non-deterministic!
    }

    delta := S{Results: data}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// ✅ Good: Use graph fan-out with order keys (deterministic merge)
// Create separate nodes for A, B, C
// Let engine handle concurrent execution and merging
// Reducer receives results in deterministic order
```

## Troubleshooting

### Replay Mismatch Errors

**Error**: `ErrReplayMismatch: Hash mismatch at node "api-call"`

**Causes**:
1. API response changed between original run and replay
2. Node logic was modified after checkpoint
3. External state changed (database, cache)
4. Non-deterministic code in node

**Solutions**:

```go
// 1. Use lenient replay to see differences
opts := graph.Options{
    ReplayMode:   true,
    StrictReplay: false, // Don't fail on mismatch
}

// 2. Compare hashes to identify what changed
origIO := checkpoint.RecordedIOs[0]
newResp := fetchFreshData()
newHash := computeHash(origIO.Request, newResp)

log.Printf("Original hash: %s", origIO.Hash)
log.Printf("New hash:      %s", newHash)
log.Printf("Original resp: %v", origIO.Response)
log.Printf("New resp:      %v", newResp)

// 3. If node logic changed, regenerate checkpoint
// Run original code version to create valid checkpoint

// 4. If external state changed, restore original state
// Use test fixtures or mocks to match original conditions
```

### Missing Recorded I/O

**Error**: `No recorded I/O found for node "api-call" attempt 0`

**Causes**:
1. Checkpoint saved before I/O recording was enabled
2. I/O was not properly recorded during original execution
3. Checkpoint loaded from wrong run

**Solutions**:

```go
// 1. Verify checkpoint contains recorded I/O
checkpoint, _ := store.LoadCheckpointV2(ctx, runID, label)
log.Printf("Recorded I/Os: %d", len(checkpoint.RecordedIOs))
for _, io := range checkpoint.RecordedIOs {
    log.Printf("  Node: %s, Attempt: %d", io.NodeID, io.Attempt)
}

// 2. Re-run original execution with I/O recording enabled
// Ensure nodes call recordIO() function

// 3. Use correct checkpoint run ID
correctCheckpoint, _ := store.LoadCheckpointV2(ctx, "correct-run-id", "")
```

### Non-Deterministic Random Values

**Problem**: Random values differ across replays

**Solution**:

```go
// ❌ Current code (wrong)
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    value := rand.Intn(100) // Global rand - non-deterministic
    delta := S{Value: value}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// ✅ Fixed code
func (n *Node) Run(ctx context.Context, state S) NodeResult[S] {
    // Get seeded RNG from context
    rng, ok := ctx.Value(RNGKey).(*rand.Rand)
    if !ok {
        log.Fatal("RNG not found in context")
    }

    value := rng.Intn(100) // Seeded RNG - deterministic
    delta := S{Value: value}
    return NodeResult[S]{Delta: delta, Route: Stop()}
}

// Verify determinism
func TestDeterministicRandom(t *testing.T) {
    rng1 := rand.New(rand.NewSource(42))
    rng2 := rand.New(rand.NewSource(42))

    for i := 0; i < 100; i++ {
        v1 := rng1.Intn(1000)
        v2 := rng2.Intn(1000)
        assert.Equal(t, v1, v2, "Same seed must produce same sequence")
    }
}
```

## Advanced Topics

### Schema Evolution

Handle state structure changes between versions:

```go
// Version 1 state
type StateV1 struct {
    Count int
}

// Version 2 state (added field)
type StateV2 struct {
    Count   int
    NewField string
}

// Migration function
func migrateCheckpoint(oldCheckpoint CheckpointV1) CheckpointV2 {
    return CheckpointV2{
        RunID:  oldCheckpoint.RunID,
        StepID: oldCheckpoint.StepID,
        State: StateV2{
            Count:    oldCheckpoint.State.Count,
            NewField: "default-value", // Populate new field
        },
        // ... copy other fields
    }
}

// Load and migrate old checkpoint
oldCP, _ := store.LoadCheckpointV2(ctx, runID, label)
newCP := migrateCheckpoint(oldCP)

// Replay with migrated state
final, err := engine.ReplayRun(ctx, "replay-migrated", newCP)
```

### Selective Replay

Replay specific nodes while skipping others:

```go
// Replay with node filtering
type ReplayOptions struct {
    SkipNodes  []string // Skip these nodes during replay
    LiveNodes  []string // Execute these nodes live (no recording)
}

opts := ReplayOptions{
    SkipNodes: []string{"cache-update", "metrics-emit"},
    LiveNodes: []string{"critical-api-call"}, // Re-execute with fresh I/O
}

// Custom replay logic
// (Implementation would require engine extensions)
```

### Performance Profiling with Replay

Profile identical executions repeatedly:

```go
// Replay for profiling
for i := 0; i < 10; i++ {
    // Load same checkpoint
    checkpoint, _ := store.LoadCheckpointV2(ctx, "profile-run", "")

    // Replay with profiling
    if i == 0 {
        defer profile.Start().Stop()
    }

    _, err := engine.ReplayRun(ctx, fmt.Sprintf("profile-%d", i), checkpoint)
    // Each replay is identical - isolates performance characteristics
}

// Analyze profile for bottlenecks
// go tool pprof cpu.prof
```

## Related Documentation

- [Concurrency Model](./concurrency.md) - Parallel execution details
- [Checkpoints & Resume](./guides/04-checkpoints.md) - Checkpoint management
- [State Management](./guides/03-state-management.md) - Reducer design patterns
- [Event Tracing](./guides/08-event-tracing.md) - Observability

## Summary

Deterministic replay provides:

✅ **Exact Reproduction**: Replay executions with identical results
✅ **Debug Production**: Reproduce failures in development
✅ **Audit Compliance**: Verify workflow behavior
✅ **Performance Testing**: Profile identical executions
✅ **Integration Testing**: Test without external dependencies

**Best Practices:**

1. Use context-provided seeded RNG for randomness
2. Record all external I/O (APIs, databases, files)
3. Avoid time.Now() - use state-provided timestamps
4. Sort map iterations for deterministic order
5. Test replay in CI to catch non-determinism early

Start recording executions in production, and use replay to debug issues without impacting external systems or reproducing complex failure scenarios.
