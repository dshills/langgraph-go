# Human-in-the-Loop Patterns

LangGraph-Go enables human-in-the-loop workflows where execution pauses for external input, approval, or decision-making before continuing. This guide demonstrates pause/resume patterns, approval workflows, and integration strategies.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Pause/Resume Pattern](#pauseresume-pattern)
- [Approval Workflows](#approval-workflows)
- [External Input Integration](#external-input-integration)
- [Best Practices](#best-practices)
- [Common Patterns](#common-patterns)
- [Production Considerations](#production-considerations)

## Overview

**Human-in-the-loop (HITL)** workflows combine automated processing with human judgment. Common use cases include:

- **Approval gates**: Review LLM outputs before taking action
- **Data validation**: Human verification of extracted information
- **Decision points**: Route workflow based on user choice
- **Error correction**: Fix issues before continuing
- **Progressive disclosure**: Show intermediate results and gather feedback

**Key Benefits:**

- ✅ Combine automation speed with human judgment
- ✅ Build trust in AI systems through transparency
- ✅ Handle edge cases requiring human expertise
- ✅ Enable iterative refinement workflows
- ✅ Support compliance and audit requirements

## Core Concepts

### State-Based Pausing

LangGraph-Go implements HITL through **checkpoint-based pausing**:

1. Workflow executes until reaching a pause point
2. Engine saves checkpoint with current state
3. External system collects human input
4. Workflow resumes from checkpoint with input integrated

### No Special "Pause" API

Instead of a dedicated pause mechanism, use:
- **Checkpoints**: Save state at pause points
- **State fields**: Store flags indicating pause status
- **Conditional routing**: Route to pause node or continue
- **External queues**: Manage pending approval requests

This approach is:
- ✅ Simple and composable
- ✅ Naturally resumable across restarts
- ✅ Fully auditable via checkpoints
- ✅ Works with existing state management

## Pause/Resume Pattern

### Basic Pattern

```go
type WorkflowState struct {
    Query         string
    DraftResponse string
    HumanApproval *bool    // nil = pending, true/false = decision
    FinalResponse string
}

// Reducer
func reduce(prev, delta WorkflowState) WorkflowState {
    if delta.Query != "" {
        prev.Query = delta.Query
    }
    if delta.DraftResponse != "" {
        prev.DraftResponse = delta.DraftResponse
    }
    if delta.HumanApproval != nil {
        prev.HumanApproval = delta.HumanApproval
    }
    if delta.FinalResponse != "" {
        prev.FinalResponse = delta.FinalResponse
    }
    return prev
}

// Node: Generate draft response
generateDraft := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
    draft := callLLM(s.Query)
    return graph.NodeResult[WorkflowState]{
        Delta: WorkflowState{DraftResponse: draft},
        Route: graph.Goto("await-approval"),
    }
})

// Node: Wait for approval (pause point)
awaitApproval := graph.NodeFunc[WorkflowState](func(ctx context.Context, s WorkflowState) graph.NodeResult[WorkflowState] {
    // Check if approval received
    if s.HumanApproval == nil {
        // Not yet approved - route to pause
        return graph.NodeResult[WorkflowState]{
            Route: graph.Stop(), // Execution pauses here
        }
    }

    // Approval received - route based on decision
    if *s.HumanApproval {
        return graph.NodeResult[WorkflowState]{
            Route: graph.Goto("finalize"),
        }
    } else {
        return graph.NodeResult[WorkflowState]{
            Route: graph.Goto("regenerate"),
        }
    }
})

// Build graph
engine.Add("generate-draft", generateDraft)
engine.Add("await-approval", awaitApproval)
engine.Add("finalize", finalizeNode)
engine.Add("regenerate", regenerateNode)
engine.StartAt("generate-draft")
```

### Pausing Execution

```go
// Start workflow
ctx := context.Background()
runID := "order-123"

state := WorkflowState{
    Query: "Summarize this document...",
}

// Execute until pause
final, err := engine.Run(ctx, runID, state)
// Execution stops at await-approval node
// final.HumanApproval == nil (pending)

// Save checkpoint automatically created by engine
checkpoint, err := store.LoadCheckpointV2(ctx, runID, "")
```

### Resuming with Input

```go
// External system collects approval
approval := true

// Resume workflow with input
resumeState := checkpoint.State
resumeState.HumanApproval = &approval

// Continue execution
final, err := engine.RunWithCheckpoint(ctx, runID, checkpoint, graph.Options{})
// Workflow continues from await-approval
// Routes to "finalize" based on approval
```

## Approval Workflows

### Simple Approval Gate

```go
type ApprovalState struct {
    Content       string
    RequiresApproval bool
    Approved      *bool
    Reason        string
}

approvalGate := graph.NodeFunc[ApprovalState](func(ctx context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
    if !s.RequiresApproval {
        // Skip approval
        return graph.NodeResult[ApprovalState]{
            Route: graph.Goto("execute"),
        }
    }

    if s.Approved == nil {
        // Wait for approval
        return graph.NodeResult[ApprovalState]{
            Route: graph.Stop(),
        }
    }

    if *s.Approved {
        return graph.NodeResult[ApprovalState]{
            Route: graph.Goto("execute"),
        }
    } else {
        return graph.NodeResult[ApprovalState]{
            Delta: ApprovalState{Reason: "Rejected by approver"},
            Route: graph.Stop(),
        }
    }
})
```

### Multi-Level Approval

```go
type MultiApprovalState struct {
    Amount           float64
    ManagerApproved  *bool
    DirectorApproved *bool
    RejectionReason  string
}

func multiLevelApproval(ctx context.Context, s MultiApprovalState) graph.NodeResult[MultiApprovalState] {
    // Manager approval required for amounts > $1000
    if s.Amount > 1000 && s.ManagerApproved == nil {
        return graph.NodeResult[MultiApprovalState]{
            Route: graph.Stop(), // Pause for manager
        }
    }

    // Director approval required for amounts > $10000
    if s.Amount > 10000 && s.DirectorApproved == nil {
        return graph.NodeResult[MultiApprovalState]{
            Route: graph.Stop(), // Pause for director
        }
    }

    // Check rejections
    if s.ManagerApproved != nil && !*s.ManagerApproved {
        return graph.NodeResult[MultiApprovalState]{
            Delta: MultiApprovalState{RejectionReason: "Manager rejected"},
            Route: graph.Stop(),
        }
    }

    if s.DirectorApproved != nil && !*s.DirectorApproved {
        return graph.NodeResult[MultiApprovalState]{
            Delta: MultiApprovalState{RejectionReason: "Director rejected"},
            Route: graph.Stop(),
        }
    }

    // All approvals received
    return graph.NodeResult[MultiApprovalState]{
        Route: graph.Goto("process-payment"),
    }
}
```

### Approval with Timeout

```go
type TimeoutApprovalState struct {
    RequestTime time.Time
    Approved    *bool
    TimedOut    bool
}

func approvalWithTimeout(ctx context.Context, s TimeoutApprovalState) graph.NodeResult[TimeoutApprovalState] {
    timeout := 24 * time.Hour

    // Check timeout
    if s.Approved == nil && time.Since(s.RequestTime) > timeout {
        return graph.NodeResult[TimeoutApprovalState]{
            Delta: TimeoutApprovalState{TimedOut: true},
            Route: graph.Goto("handle-timeout"),
        }
    }

    // Still waiting
    if s.Approved == nil {
        return graph.NodeResult[TimeoutApprovalState]{
            Route: graph.Stop(),
        }
    }

    // Approved
    if *s.Approved {
        return graph.NodeResult[TimeoutApprovalState]{
            Route: graph.Goto("proceed"),
        }
    } else {
        return graph.NodeResult[TimeoutApprovalState]{
            Route: graph.Goto("cancel"),
        }
    }
}
```

## External Input Integration

### REST API Integration

```go
// API endpoint to get pending approvals
func GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
    // Query checkpoints with HumanApproval == nil
    pending, err := findPendingApprovals(ctx, store)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(pending)
}

// API endpoint to submit approval
func SubmitApproval(w http.ResponseWriter, r *http.Request) {
    var req struct {
        RunID    string `json:"run_id"`
        Approved bool   `json:"approved"`
        Comment  string `json:"comment"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Load checkpoint
    checkpoint, err := store.LoadCheckpointV2(ctx, req.RunID, "")
    if err != nil {
        http.Error(w, "Workflow not found", http.StatusNotFound)
        return
    }

    // Update state with approval
    checkpoint.State.HumanApproval = &req.Approved
    checkpoint.State.ApprovalComment = req.Comment

    // Resume workflow
    go func() {
        engine.RunWithCheckpoint(context.Background(), req.RunID, checkpoint, graph.Options{})
    }()

    w.WriteHeader(http.StatusAccepted)
}
```

### Queue-Based Integration

```go
// Worker to resume workflows when input arrives
func approvalWorker(ctx context.Context, approvalQueue <-chan ApprovalInput) {
    for approval := range approvalQueue {
        // Load checkpoint
        checkpoint, err := store.LoadCheckpointV2(ctx, approval.RunID, "")
        if err != nil {
            log.Printf("Failed to load checkpoint: %v", err)
            continue
        }

        // Inject approval
        checkpoint.State.Approved = &approval.Decision
        checkpoint.State.ApproverID = approval.UserID
        checkpoint.State.ApprovedAt = time.Now()

        // Resume workflow
        _, err = engine.RunWithCheckpoint(ctx, approval.RunID, checkpoint, graph.Options{})
        if err != nil {
            log.Printf("Failed to resume workflow: %v", err)
        }
    }
}

type ApprovalInput struct {
    RunID    string
    Decision bool
    UserID   string
}
```

### WebSocket Real-Time Updates

```go
// Notify frontend when workflow pauses
func notifyPauseEvent(runID string, state WorkflowState) {
    event := PauseEvent{
        RunID:         runID,
        DraftResponse: state.DraftResponse,
        Timestamp:     time.Now(),
    }

    // Send to WebSocket clients
    broadcastToClients(event)
}

// Handle WebSocket approval submission
func handleWebSocketApproval(conn *websocket.Conn, msg ApprovalMessage) {
    // Load checkpoint
    checkpoint, err := store.LoadCheckpointV2(context.Background(), msg.RunID, "")
    if err != nil {
        sendError(conn, "Workflow not found")
        return
    }

    // Update state
    checkpoint.State.HumanApproval = &msg.Approved

    // Resume workflow
    go func() {
        final, err := engine.RunWithCheckpoint(context.Background(), msg.RunID, checkpoint, graph.Options{})
        if err != nil {
            log.Printf("Resume error: %v", err)
            return
        }

        // Send result back to client
        sendResult(conn, final)
    }()

    sendAck(conn, "Approval received")
}
```

## Best Practices

### 1. Store Complete Context in State

Include all information needed for human decision:

```go
type ReviewState struct {
    // Input context
    OriginalQuery string
    UserID        string

    // Generated output
    Response      string
    Confidence    float64
    Sources       []string

    // Human decision
    Approved      *bool
    Feedback      string
    ReviewedBy    string
    ReviewedAt    time.Time
}
```

### 2. Use Nullable Pointers for Pending State

```go
// ✅ Good: Can distinguish "not yet decided" from "decided false"
type State struct {
    Approved *bool
}

// ❌ Bad: Can't tell if false means "rejected" or "not yet decided"
type State struct {
    Approved bool
}
```

### 3. Include Audit Trail

```go
type AuditableState struct {
    // Decision
    Approved *bool

    // Audit fields
    ApprovedBy    string
    ApprovedAt    time.Time
    ReviewDuration time.Duration
    Comment       string
    IPAddress     string
}
```

### 4. Handle Edge Cases

```go
func robustApprovalNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Case 1: No approval needed
    if !s.RequiresApproval {
        return graph.NodeResult[State]{
            Route: graph.Goto("continue"),
        }
    }

    // Case 2: Approval pending
    if s.Approved == nil {
        // Check for timeout
        if time.Since(s.PauseStartTime) > 24*time.Hour {
            return graph.NodeResult[State]{
                Delta: State{TimedOut: true},
                Route: graph.Goto("timeout-handler"),
            }
        }

        return graph.NodeResult[State]{
            Route: graph.Stop(),
        }
    }

    // Case 3: Approved
    if *s.Approved {
        return graph.NodeResult[State]{
            Route: graph.Goto("execute"),
        }
    }

    // Case 4: Rejected
    return graph.NodeResult[State]{
        Route: graph.Goto("rejection-handler"),
    }
}
```

### 5. Prevent Race Conditions

```go
// Use idempotency keys for resume operations
func resumeWorkflow(ctx context.Context, runID string, approval ApprovalInput) error {
    // Check idempotency
    idempotencyKey := fmt.Sprintf("resume-%s-%s", runID, approval.SubmissionID)
    exists, err := store.CheckIdempotency(ctx, runID, idempotencyKey)
    if err != nil {
        return err
    }
    if exists {
        return nil // Already processed
    }

    // Load checkpoint
    checkpoint, err := store.LoadCheckpointV2(ctx, runID, "")
    if err != nil {
        return err
    }

    // Apply approval
    checkpoint.State.Approved = &approval.Decision

    // Resume with idempotency key
    opts := graph.Options{
        IdempotencyKey: idempotencyKey,
    }
    _, err = engine.RunWithCheckpoint(ctx, runID, checkpoint, opts)
    return err
}
```

## Common Patterns

### Pattern: Review-Edit-Approve Loop

```go
type EditableState struct {
    Draft      string
    EditCount  int
    MaxEdits   int
    Approved   *bool
    Edits      []Edit
}

func reviewEditLoop(ctx context.Context, s EditableState) graph.NodeResult[EditableState] {
    // Check for approval
    if s.Approved != nil && *s.Approved {
        return graph.NodeResult[EditableState]{
            Route: graph.Goto("finalize"),
        }
    }

    // Check edit limit
    if s.EditCount >= s.MaxEdits {
        return graph.NodeResult[EditableState]{
            Route: graph.Goto("escalate"),
        }
    }

    // Wait for human input (edit or approve)
    return graph.NodeResult[EditableState]{
        Route: graph.Stop(),
    }
}

type Edit struct {
    Timestamp time.Time
    UserID    string
    Changes   string
}
```

### Pattern: Progressive Approval

```go
// Automatically approve simple cases, require human review for complex
func smartApproval(ctx context.Context, s State) graph.NodeResult[State] {
    // Auto-approve if confidence high and low risk
    if s.Confidence > 0.95 && s.RiskScore < 0.1 {
        approved := true
        return graph.NodeResult[State]{
            Delta: State{
                Approved:   &approved,
                AutoApproved: true,
            },
            Route: graph.Goto("execute"),
        }
    }

    // Require human review
    return graph.NodeResult[State]{
        Route: graph.Stop(),
    }
}
```

### Pattern: Collaborative Review

```go
type CollaborativeState struct {
    Reviews      map[string]Review // UserID -> Review
    RequiredVotes int
    ApprovedCount int
    RejectedCount int
}

type Review struct {
    Approved bool
    Comment  string
    Timestamp time.Time
}

func collaborativeReview(ctx context.Context, s CollaborativeState) graph.NodeResult[CollaborativeState] {
    // Count votes
    approved := 0
    rejected := 0
    for _, review := range s.Reviews {
        if review.Approved {
            approved++
        } else {
            rejected++
        }
    }

    // Check if consensus reached
    if approved >= s.RequiredVotes {
        return graph.NodeResult[CollaborativeState]{
            Delta: CollaborativeState{ApprovedCount: approved},
            Route: graph.Goto("approved"),
        }
    }

    if rejected >= s.RequiredVotes {
        return graph.NodeResult[CollaborativeState]{
            Delta: CollaborativeState{RejectedCount: rejected},
            Route: graph.Goto("rejected"),
        }
    }

    // Still waiting for votes
    return graph.NodeResult[CollaborativeState]{
        Route: graph.Stop(),
    }
}
```

## Production Considerations

### Monitoring Paused Workflows

```go
// Query for workflows paused longer than threshold
func findStalePausedWorkflows(ctx context.Context, store Store, threshold time.Duration) ([]string, error) {
    // Implementation depends on store
    // Query checkpoints where:
    // - State indicates pause (Approved == nil)
    // - Timestamp older than threshold
    return store.QueryCheckpoints(ctx, PauseQuery{
        MaxAge: threshold,
        Status: "paused",
    })
}

// Alert on stale workflows
func monitorPausedWorkflows(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        stale, err := findStalePausedWorkflows(ctx, store, 48*time.Hour)
        if err != nil {
            log.Printf("Error checking stale workflows: %v", err)
            continue
        }

        if len(stale) > 0 {
            alertOps("Found %d stale paused workflows", len(stale))
        }
    }
}
```

### Testing Human-in-the-Loop

```go
func TestHumanApprovalWorkflow(t *testing.T) {
    // Setup
    engine := setupEngine()
    ctx := context.Background()

    // Start workflow
    runID := "test-approval-001"
    initial := WorkflowState{Query: "test query"}

    final, err := engine.Run(ctx, runID, initial)
    require.NoError(t, err)

    // Verify paused at approval node
    assert.Nil(t, final.HumanApproval)

    // Simulate approval
    checkpoint, err := store.LoadCheckpointV2(ctx, runID, "")
    require.NoError(t, err)

    approved := true
    checkpoint.State.HumanApproval = &approved

    // Resume workflow
    final, err = engine.RunWithCheckpoint(ctx, runID, checkpoint, graph.Options{})
    require.NoError(t, err)

    // Verify completion
    assert.NotEmpty(t, final.FinalResponse)
}
```

### Handling Workflow Restarts

```go
// Resume all paused workflows on startup
func resumePausedWorkflows(ctx context.Context, engine *Engine, store Store) {
    pending, err := store.QueryCheckpoints(ctx, CheckpointQuery{
        Status: "paused",
    })
    if err != nil {
        log.Printf("Failed to load pending workflows: %v", err)
        return
    }

    log.Printf("Found %d paused workflows to resume", len(pending))

    for _, checkpoint := range pending {
        // Only resume if approval already provided
        if checkpoint.State.HumanApproval != nil {
            go func(cp Checkpoint) {
                _, err := engine.RunWithCheckpoint(ctx, cp.RunID, cp, graph.Options{})
                if err != nil {
                    log.Printf("Failed to resume %s: %v", cp.RunID, err)
                }
            }(checkpoint)
        }
    }
}
```

## Related Documentation

- [Checkpoints & Resume](./guides/04-checkpoints.md) - Checkpoint management
- [State Management](./guides/03-state-management.md) - State design patterns
- [Building Workflows](./guides/02-building-workflows.md) - Graph construction
- [Event Tracing](./guides/08-event-tracing.md) - Monitoring paused workflows

## Summary

**Human-in-the-loop workflows in LangGraph-Go:**

✅ **Checkpoint-based**: Pause/resume via state and checkpoints
✅ **Natural**: No special pause API - use conditional routing
✅ **Flexible**: Support approval gates, reviews, edits, timeouts
✅ **Auditable**: Full history via checkpoints
✅ **Production-ready**: Handle edge cases, race conditions, timeouts

**Implementation Checklist:**

1. ✅ Use nullable pointers for pending decisions
2. ✅ Store complete context in state
3. ✅ Include audit trail fields
4. ✅ Handle timeouts and edge cases
5. ✅ Use idempotency for resume operations
6. ✅ Monitor stale paused workflows
7. ✅ Test pause/resume cycles thoroughly

See the [Human-in-the-Loop Example](../examples/human_in_the_loop/) for a complete working implementation.
