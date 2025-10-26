# Event Tracing

This guide covers observability, event emission, history filtering, and monitoring patterns for building production-ready workflows.

## Overview

LangGraph-Go emits events for every significant workflow action:
- Node execution (start/end)
- Routing decisions
- Errors and warnings
- State changes
- Performance metrics

Events provide complete observability into workflow execution, enabling:
- **Debugging**: Understand exactly what happened and why
- **Monitoring**: Track performance and errors in production
- **Auditing**: Record all workflow actions for compliance
- **Optimization**: Identify bottlenecks and slow nodes

## Event Structure

Every event contains:

```go
type Event struct {
    RunID  string                 // Workflow execution ID
    Step   int                    // Sequential step number (1-indexed)
    NodeID string                 // Node that emitted the event
    Msg    string                 // Event type/description
    Meta   map[string]interface{} // Additional structured data
}
```

### Standard Event Types

**Node Execution**:
- `node_start`: Node begins execution
- `node_end`: Node completes, includes state delta
- `error`: Node encountered an error

**Routing**:
- `routing_decision`: Shows next node(s) to execute

**Workflow**:
- `workflow_start`: Workflow execution begins
- `workflow_complete`: Workflow finished successfully
- `workflow_error`: Workflow failed

## Emitter Types

### LogEmitter (Development)

Logs events to stdout/stderr:

```go
import (
    "os"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Text mode (human-readable)
emitter := emit.NewLogEmitter(os.Stdout, false)

// JSON mode (machine-readable)
emitter := emit.NewLogEmitter(os.Stdout, true)
```

**Example Output (Text Mode)**:
```
[node_start] runID=demo-001 step=0 nodeID=validate
[node_end] runID=demo-001 step=0 nodeID=validate meta={"delta":{...}}
[routing_decision] runID=demo-001 step=0 nodeID=validate meta={"next_node":"process"}
```

**Example Output (JSON Mode)**:
```json
{"msg":"node_start","runID":"demo-001","step":0,"nodeID":"validate"}
{"msg":"node_end","runID":"demo-001","step":0,"nodeID":"validate","meta":{"delta":{...}}}
{"msg":"routing_decision","runID":"demo-001","step":0,"nodeID":"validate","meta":{"next_node":"process"}}
```

### BufferedEmitter (Analysis)

Stores events in memory for querying:

```go
import "github.com/dshills/langgraph-go/graph/emit"

emitter := emit.NewBufferedEmitter()

// Execute workflow
engine := graph.New(reducer, store, emitter, opts)
finalState, err := engine.Run(ctx, "run-001", initialState)

// Query event history
allEvents := emitter.GetHistory("run-001")
fmt.Printf("Total events: %d\n", len(allEvents))

// Filter events
nodeStarts := emitter.GetHistoryWithFilter("run-001", emit.HistoryFilter{
    Msg: "node_start", // Only node_start events
})

// Filter by step range
stepsEvents := emitter.GetHistoryWithFilter("run-001", emit.HistoryFilter{
    MinStep: ptrInt(5),  // Steps 5-10
    MaxStep: ptrInt(10),
})

// Filter by node
nodeEvents := emitter.GetHistoryWithFilter("run-001", emit.HistoryFilter{
    NodeID: "process", // Only events from "process" node
})
```

### NullEmitter (Production High-Performance)

No-op emitter with zero overhead:

```go
emitter := emit.NewNullEmitter()
```

Use in production when observability overhead is unacceptable.

### Multi-Emitter Pattern

Send events to multiple destinations:

```go
type MultiEmitter struct {
    emitters []emit.Emitter
}

func (m *MultiEmitter) Emit(event emit.Event) {
    for _, e := range m.emitters {
        e.Emit(event)
    }
}

// Use multiple emitters
buffered := emit.NewBufferedEmitter()
logger := emit.NewLogEmitter(os.Stdout, false)

multi := &MultiEmitter{
    emitters: []emit.Emitter{buffered, logger},
}

engine := graph.New(reducer, store, multi, opts)
```

**Benefits**:
- Real-time logging + post-execution analysis
- Local logs + remote telemetry
- Multiple monitoring systems

## Event Metadata

Events carry structured metadata in the `Meta` field:

### Performance Metrics

```go
// Emitted automatically by engine
event.Meta["duration_ms"]   = 1234  // Execution time
event.Meta["memory_bytes"]  = 50000 // Memory usage
event.Meta["cpu_percent"]   = 45.2  // CPU utilization
```

### Error Context

```go
// When node returns error
event.Meta["error"]         = "validation failed"
event.Meta["error_type"]    = "validation"
event.Meta["retryable"]     = true
event.Meta["retry_attempt"] = 2
```

### LLM Metrics

```go
// For LLM node execution
event.Meta["tokens"]         = 1500
event.Meta["input_tokens"]   = 1000
event.Meta["output_tokens"]  = 500
event.Meta["cost"]           = 0.015  // USD
event.Meta["model"]          = "gpt-4o"
event.Meta["finish_reason"]  = "stop"
```

### State Changes

```go
// node_end events include delta
event.Meta["delta"] = StateChange{
    Status: "processing",
    Count:  1,
}
```

### Routing Decisions

```go
// routing_decision events
event.Meta["next_node"]  = "process"        // Single routing
event.Meta["next_nodes"] = []string{"a","b"} // Fan-out
event.Meta["terminal"]   = true             // Stop
```

## Observability Patterns

### Pattern 1: Real-Time Monitoring

Monitor workflow execution in real-time:

```go
// Custom emitter that sends to monitoring system
type MonitoringEmitter struct {
    endpoint string
}

func (m *MonitoringEmitter) Emit(event emit.Event) {
    // Track errors
    if event.Msg == "error" {
        sendAlert(m.endpoint, "Error in node %s: %v", event.NodeID, event.Meta["error"])
    }

    // Track slow nodes
    if duration, ok := event.Meta["duration_ms"].(int64); ok {
        if duration > 5000 { // > 5 seconds
            sendAlert(m.endpoint, "Slow node %s: %dms", event.NodeID, duration)
        }
    }

    // Track LLM costs
    if cost, ok := event.Meta["cost"].(float64); ok {
        trackCost(m.endpoint, event.RunID, cost)
    }
}
```

### Pattern 2: Post-Execution Analysis

Analyze workflow after completion:

```go
func analyzeWorkflow(runID string, buffered *emit.BufferedEmitter) {
    events := buffered.GetHistory(runID)

    // Calculate total duration
    var totalDuration int64
    for _, event := range events {
        if event.Msg == "node_end" {
            if dur, ok := event.Meta["duration_ms"].(int64); ok {
                totalDuration += dur
            }
        }
    }

    // Find slowest node
    slowestNode := ""
    slowestDur := int64(0)
    for _, event := range events {
        if event.Msg == "node_end" {
            if dur, ok := event.Meta["duration_ms"].(int64); ok {
                if dur > slowestDur {
                    slowestDur = dur
                    slowestNode = event.NodeID
                }
            }
        }
    }

    fmt.Printf("Total duration: %dms\n", totalDuration)
    fmt.Printf("Slowest node: %s (%dms)\n", slowestNode, slowestDur)

    // Count errors
    errorEvents := buffered.GetHistoryWithFilter(runID, emit.HistoryFilter{
        Msg: "error",
    })
    fmt.Printf("Errors: %d\n", len(errorEvents))

    // Show execution path
    fmt.Println("\nExecution path:")
    nodeStarts := buffered.GetHistoryWithFilter(runID, emit.HistoryFilter{
        Msg: "node_start",
    })
    for i, event := range nodeStarts {
        fmt.Printf("  %d. %s\n", i+1, event.NodeID)
    }
}
```

### Pattern 3: Performance Profiling

Identify bottlenecks:

```go
func profileWorkflow(runID string, buffered *emit.BufferedEmitter) {
    events := buffered.GetHistory(runID)

    // Build performance profile by node
    profile := make(map[string]struct {
        count    int
        totalMs  int64
        avgMs    int64
    })

    for _, event := range events {
        if event.Msg == "node_end" {
            if dur, ok := event.Meta["duration_ms"].(int64); ok {
                p := profile[event.NodeID]
                p.count++
                p.totalMs += dur
                p.avgMs = p.totalMs / int64(p.count)
                profile[event.NodeID] = p
            }
        }
    }

    fmt.Println("Node Performance Profile:")
    for nodeID, p := range profile {
        fmt.Printf("  %s: %d calls, avg %dms, total %dms\n",
            nodeID, p.count, p.avgMs, p.totalMs)
    }
}
```

### Pattern 4: Audit Trail

Record all workflow actions for compliance:

```go
type AuditEmitter struct {
    db *sql.DB
}

func (a *AuditEmitter) Emit(event emit.Event) {
    // Store all events in audit database
    _, err := a.db.Exec(`
        INSERT INTO audit_log (run_id, step, node_id, msg, meta, timestamp)
        VALUES (?, ?, ?, ?, ?, ?)
    `, event.RunID, event.Step, event.NodeID, event.Msg,
        jsonMarshal(event.Meta), time.Now())

    if err != nil {
        log.Printf("Failed to write audit log: %v", err)
    }
}

// Query audit history
func queryAudit(db *sql.DB, runID string) ([]emit.Event, error) {
    rows, err := db.Query(`
        SELECT run_id, step, node_id, msg, meta
        FROM audit_log
        WHERE run_id = ?
        ORDER BY step
    `, runID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var events []emit.Event
    for rows.Next() {
        var event emit.Event
        var metaJSON string
        rows.Scan(&event.RunID, &event.Step, &event.NodeID, &event.Msg, &metaJSON)
        json.Unmarshal([]byte(metaJSON), &event.Meta)
        events = append(events, event)
    }

    return events, nil
}
```

### Pattern 5: Alerting

Trigger alerts on specific conditions:

```go
type AlertingEmitter struct {
    alerter Alerter
}

func (a *AlertingEmitter) Emit(event emit.Event) {
    // Alert on errors
    if event.Msg == "error" {
        a.alerter.Send(Alert{
            Severity: "error",
            Message:  fmt.Sprintf("Workflow %s failed at node %s", event.RunID, event.NodeID),
            Details:  event.Meta,
        })
    }

    // Alert on high LLM costs
    if cost, ok := event.Meta["cost"].(float64); ok {
        if cost > 1.0 { // > $1 per call
            a.alerter.Send(Alert{
                Severity: "warning",
                Message:  fmt.Sprintf("High LLM cost: $%.2f", cost),
                Details:  event.Meta,
            })
        }
    }

    // Alert on long execution
    if dur, ok := event.Meta["duration_ms"].(int64); ok {
        if dur > 30000 { // > 30 seconds
            a.alerter.Send(Alert{
                Severity: "warning",
                Message:  fmt.Sprintf("Slow node execution: %dms", dur),
                Details:  event.Meta,
            })
        }
    }
}
```

## Custom Events

Nodes can emit custom events:

```go
func customNode(ctx context.Context, s State) graph.NodeResult[State] {
    result := doWork(s)

    customEvents := []emit.Event{
        {
            RunID:  s.RunID,
            NodeID: "custom",
            Msg:    "custom_metric",
            Meta: map[string]interface{}{
                "metric_name":  "items_processed",
                "metric_value": len(result.Items),
            },
        },
    }

    return graph.NodeResult[State]{
        Delta:  State{Result: result},
        Route:  graph.Goto("next"),
        Events: customEvents, // Emitted after node execution
    }
}
```

## Event Metadata Helpers

Use helper methods to add metadata consistently:

```go
import "github.com/dshills/langgraph-go/graph/emit"

event := emit.Event{
    RunID:  "run-001",
    NodeID: "process",
    Msg:    "node_end",
    Meta:   make(map[string]interface{}),
}

// Add duration
event = event.WithDuration(123 * time.Millisecond)
// Meta["duration_ms"] = 123

// Add error
event = event.WithError(errors.New("failed"))
// Meta["error"] = "failed"

// Add node type
event = event.WithNodeType("llm")
// Meta["node_type"] = "llm"

// Add custom metadata
event = event.WithMeta("custom_key", "custom_value")
// Meta["custom_key"] = "custom_value"
```

## Best Practices

### 1. Choose the Right Emitter

```go
// Development: LogEmitter for debugging
emitter := emit.NewLogEmitter(os.Stdout, false)

// Testing: BufferedEmitter for assertions
emitter := emit.NewBufferedEmitter()

// Production: Multi-emitter for logging + monitoring
emitter := &MultiEmitter{
    emitters: []emit.Emitter{
        emit.NewLogEmitter(logFile, true),
        productionMonitoringEmitter,
    },
}

// High-performance: NullEmitter when observability not needed
emitter := emit.NewNullEmitter()
```

### 2. Use Structured Metadata

```go
// ❌ BAD: Unstructured error message
event.Meta["info"] = "Node failed with error: connection timeout after 30s"

// ✅ GOOD: Structured metadata
event.Meta["error"] = "connection timeout"
event.Meta["error_type"] = "timeout"
event.Meta["timeout_seconds"] = 30
event.Meta["retryable"] = true
```

### 3. Filter Events Efficiently

```go
// Query specific events instead of filtering in memory
nodeEvents := buffered.GetHistoryWithFilter(runID, emit.HistoryFilter{
    NodeID: "llm-node",
    Msg:    "node_end",
})

// More efficient than:
allEvents := buffered.GetHistory(runID)
filtered := filterInMemory(allEvents) // Slower
```

### 4. Clean Up Old Events

```go
// BufferedEmitter stores events in memory
// Clear periodically to prevent memory growth
buffered.Clear(oldRunID)
```

### 5. Don't Leak Sensitive Data

```go
// ❌ BAD: Exposes API keys in events
event.Meta["api_key"] = apiKey

// ✅ GOOD: Redact sensitive data
event.Meta["api_key"] = "[REDACTED]"
event.Meta["has_api_key"] = true // Presence indicator only
```

## Integration with OpenTelemetry

For production observability, integrate with OpenTelemetry:

```go
// (Requires graph/emit/otel package - T192-T195)
import "github.com/dshills/langgraph-go/graph/emit/otel"

otelEmitter := otel.NewOtelEmitter(tracerProvider)

engine := graph.New(reducer, store, otelEmitter, opts)

// Events automatically create OpenTelemetry spans
// - Each node execution = span
// - Metadata becomes span attributes
// - Errors create error events
// - Routing creates span links
```

## Testing with Events

Verify workflow behavior using events:

```go
func TestWorkflowExecution(t *testing.T) {
    buffered := emit.NewBufferedEmitter()
    store := store.NewMemStore[State]()
    engine := graph.New(reducer, store, buffered, graph.Options{})

    // Build and run workflow
    engine.Add("A", nodeA)
    engine.Add("B", nodeB)
    engine.StartAt("A")

    _, err := engine.Run(context.Background(), "test-001", initialState)
    if err != nil {
        t.Fatal(err)
    }

    // Verify execution path
    nodeStarts := buffered.GetHistoryWithFilter("test-001", emit.HistoryFilter{
        Msg: "node_start",
    })

    expectedPath := []string{"A", "B"}
    for i, event := range nodeStarts {
        if event.NodeID != expectedPath[i] {
            t.Errorf("Step %d: expected %s, got %s", i, expectedPath[i], event.NodeID)
        }
    }

    // Verify no errors
    errors := buffered.GetHistoryWithFilter("test-001", emit.HistoryFilter{
        Msg: "error",
    })
    if len(errors) > 0 {
        t.Errorf("Expected no errors, got %d", len(errors))
    }
}
```

---

**Next Steps:**
- Explore [API Reference](../api/) for detailed API documentation
- Read [FAQ](../FAQ.md) for common questions
- Check [examples/](../../examples/) for more patterns

