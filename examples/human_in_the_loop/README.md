# Human-in-the-Loop Example

This example demonstrates human-in-the-loop workflows where execution pauses for external approval before continuing. It shows approval gates, pause/resume patterns, rejection handling, and automatic workflow resumption.

## Overview

Human-in-the-loop (HITL) workflows combine automation with human judgment, enabling:

- **Approval gates**: Review AI-generated outputs before taking action
- **Quality control**: Human verification of automated results
- **Compliance**: Required sign-offs for critical operations
- **Iterative refinement**: Reject and regenerate until satisfactory

## What This Example Demonstrates

### 1. Interactive Approval Workflow

The workflow generates output that requires human approval:

```
Generate Output → Approval Gate (PAUSE) → Finalize
                       ↓
                  If rejected & attempts < 3:
                  Return to Generate
```

**Key Features:**

- ✅ Workflow pauses at approval gate
- ✅ Human reviewer makes approve/reject decision
- ✅ Comments can be attached to decisions
- ✅ Rejection triggers regeneration with feedback
- ✅ Max attempts prevent infinite loops

### 2. Checkpoint-Based Resume

Demonstrates resuming paused workflows:

```
1. Start workflow → Pause at approval
2. Save checkpoint (state + position)
3. Later: Load checkpoint → Apply approval → Resume
```

**Use Cases:**

- Long-running workflows that span hours/days
- Approval requires external stakeholder availability
- Multi-level approval chains
- Integration with external approval systems (Slack, email, web UI)

### 3. Rejection and Retry

Shows handling rejections with automatic regeneration:

```
Generate → Pause → Reject with feedback
    ↑                     ↓
    └──── Regenerate ─────┘
         (attempt 2)
```

**Benefits:**

- Feedback loop for iterative improvement
- Automatic retry with context
- Track attempt count and feedback history
- Graceful failure after max attempts

## Usage

### Build and Run

```bash
# From project root
make examples
./build/human_in_the_loop

# Or directly
cd examples/human_in_the_loop
go run main.go
```

### Interactive Demo

The example runs three demonstrations:

1. **Interactive Approval**: You'll be prompted to approve/reject output
2. **Automatic Resume**: Simulates external approval system
3. **Rejection with Retry**: Shows regeneration loop

### Example Output

```
======================================================================
Human-in-the-Loop: Approval Workflow Demo
======================================================================

🚀 Starting workflow...

🤖 Generating output for request: Create a marketing email for new product launch

⏸️  Workflow paused - awaiting human approval
Generated Output:
Generated response for 'Create a marketing email for new product launch': This is an automated output that needs review.

👤 Approve this output? (y/n): y
Add comment (optional, press Enter to skip): Looks good!

▶️  Resuming workflow...

✅ Approved by human reviewer
Comment: Looks good!

✨ Finalizing approved output...
Output: Generated response for 'Create a marketing email for new product launch': This is an automated output that needs review.
Status: Approved and published

======================================================================
✅ Workflow completed successfully - Output approved and finalized
======================================================================
```

## Code Structure

### State Definition

```go
type ApprovalState struct {
    Request         string   // Input request
    GeneratedOutput string   // Generated output awaiting approval
    RequiresApproval bool    // Flag for approval requirement
    Approved        *bool    // nil = pending, true/false = decision
    ApprovalComment string   // Reviewer feedback
    Timestamp       time.Time // Generation time
    Attempts        int      // Regeneration attempts
}
```

**Key Design Choice: `Approved *bool`**

Using a pointer allows distinguishing:
- `nil` = No decision yet (paused)
- `true` = Approved
- `false` = Rejected

### Node Implementations

**Generate Node**: Creates output requiring approval

```go
func GenerateOutputNode(ctx context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
    output := generateOutput(s.Request)

    return graph.NodeResult[ApprovalState]{
        Delta: ApprovalState{
            GeneratedOutput:  output,
            RequiresApproval: true,
            Attempts:         s.Attempts + 1,
        },
        Route: graph.Goto("approval-gate"),
    }
}
```

**Approval Gate Node**: Pauses for human decision

```go
func ApprovalGateNode(ctx context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
    // No decision yet - pause execution
    if s.Approved == nil {
        return graph.NodeResult[ApprovalState]{
            Route: graph.Stop(), // Workflow pauses here
        }
    }

    // Approved - continue to finalize
    if *s.Approved {
        return graph.NodeResult[ApprovalState]{
            Route: graph.Goto("finalize"),
        }
    }

    // Rejected - regenerate or stop
    if s.Attempts < 3 {
        return graph.NodeResult[ApprovalState]{
            Delta: ApprovalState{Approved: nil}, // Reset approval
            Route: graph.Goto("generate"),
        }
    } else {
        return graph.NodeResult[ApprovalState]{
            Route: graph.Stop(), // Max attempts reached
        }
    }
}
```

### Pause and Resume Pattern

```go
// 1. Start workflow - pauses at approval gate
final, err := engine.Run(ctx, runID, initialState)

// 2. Workflow is paused (final.Approved == nil)
if final.Approved == nil {
    // 3. Load checkpoint
    checkpoint, _ := store.LoadCheckpointV2(ctx, runID, "")

    // 4. Apply approval decision
    approved := getUserApproval()
    checkpoint.State.Approved = &approved
    checkpoint.State.ApprovalComment = "Reviewer comment"

    // 5. Resume from checkpoint
    final, err = engine.RunWithCheckpoint(ctx, runID, checkpoint, graph.Options{})
}
```

## Use Cases

### 1. Content Moderation

```go
// AI generates content → Human reviews for appropriateness → Publish if approved
GenerateContent → ModerationGate → Publish
```

### 2. Financial Transactions

```go
// System recommends transaction → Approver reviews → Execute if approved
RecommendTrade → ApprovalGate → ExecuteTrade
```

### 3. Customer Communications

```go
// Draft customer response → Manager reviews → Send if approved
DraftResponse → ReviewGate → SendEmail
```

### 4. Multi-Level Approval

```go
// Workflow with escalating approval requirements
Request → ManagerApproval → DirectorApproval → VPApproval → Execute
```

## Production Integration

### REST API Integration

```go
// Endpoint to get pending approvals
GET /api/approvals/pending
→ Returns list of workflows awaiting approval

// Endpoint to submit approval
POST /api/approvals/{runID}
{
  "approved": true,
  "comment": "Looks good to me"
}
→ Resumes workflow
```

### WebSocket Real-Time Updates

```go
// Notify frontend when workflow pauses
websocket.send({
  type: "approval_required",
  runID: "run-123",
  output: "Generated content...",
  timestamp: "2024-01-01T12:00:00Z"
})

// Receive approval from frontend
websocket.receive({
  type: "approval_decision",
  runID: "run-123",
  approved: true,
  comment: "Approved"
})
```

### Queue-Based Processing

```go
// Worker pool resuming workflows when approvals arrive
for approval := range approvalQueue {
    checkpoint, _ := store.LoadCheckpointV2(ctx, approval.RunID, "")
    checkpoint.State.Approved = &approval.Approved

    go engine.RunWithCheckpoint(ctx, approval.RunID, checkpoint, graph.Options{})
}
```

## Best Practices

### 1. Use Nullable Pointers for Pending State

```go
// ✅ Good: Can distinguish pending from decided
Approved *bool

// ❌ Bad: Can't tell if false means "rejected" or "not decided"
Approved bool
```

### 2. Include Audit Trail

```go
type ApprovalState struct {
    Approved     *bool
    ApprovedBy   string    // User ID
    ApprovedAt   time.Time // Decision timestamp
    Comment      string    // Approval/rejection reason
    IPAddress    string    // Source IP for audit
}
```

### 3. Handle Timeouts

```go
if s.Approved == nil {
    // Check if approval request has timed out
    if time.Since(s.RequestTime) > 24*time.Hour {
        return NodeResult[State]{
            Route: Goto("timeout-handler"),
        }
    }

    return NodeResult[State]{
        Route: Stop(), // Still waiting
    }
}
```

### 4. Prevent Race Conditions

```go
// Use idempotency keys when resuming
idempotencyKey := fmt.Sprintf("resume-%s-%s", runID, approvalID)
opts := graph.Options{
    IdempotencyKey: idempotencyKey,
}
engine.RunWithCheckpoint(ctx, runID, checkpoint, opts)
```

### 5. Monitor Stale Workflows

```go
// Alert on workflows paused longer than threshold
func alertStalePausedWorkflows(ctx context.Context) {
    stale := findPausedWorkflows(ctx, olderThan: 48*time.Hour)
    if len(stale) > 0 {
        alertOps(fmt.Sprintf("%d workflows need attention", len(stale)))
    }
}
```

## Key Concepts

- **Pause Point**: Node returns `Stop()` when waiting for input
- **Resume**: Load checkpoint, update state, call `RunWithCheckpoint`
- **Nullable Pointers**: Distinguish "pending" from boolean decisions
- **Checkpoint**: Workflow position saved automatically at pause
- **Idempotency**: Prevents duplicate execution on resume

## Related Documentation

- [Human-in-the-Loop Guide](../../docs/human-in-the-loop.md) - Comprehensive patterns and integration
- [Checkpoints Guide](../../docs/guides/04-checkpoints.md) - Checkpoint management
- [State Management](../../docs/guides/03-state-management.md) - State design patterns
- [Building Workflows](../../docs/guides/02-building-workflows.md) - Graph construction

## Extending This Example

Try modifying the example to:

1. **Add Multi-Level Approval**: Require manager + director approval
2. **Implement Timeout**: Auto-reject after 24 hours
3. **Add Approval Delegation**: Allow approvers to delegate decisions
4. **Track Approval History**: Store all approval decisions in state
5. **Conditional Approval**: Only require approval if amount > $10,000

## Next Steps

- Integrate with external approval systems (Slack, email, web UI)
- Add persistence with MySQL/SQLite store
- Implement approval delegation and escalation
- Add event tracing for approval audit logs
- Build approval dashboard UI

See [docs/human-in-the-loop.md](../../docs/human-in-the-loop.md) for production patterns and best practices.
