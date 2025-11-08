# Event Emission and Observability

The `emit` package provides event emission and observability for LangGraph-Go workflow execution. It enables pluggable observability backends through a clean interface, supporting logging, distributed tracing, metrics, and analytics.

## Table of Contents

- [Overview](#overview)
- [Available Emitters](#available-emitters)
- [OpenTelemetry Integration](#opentelemetry-integration)
- [Multi-Emitter Pattern](#multi-emitter-pattern)
- [Event Schema](#event-schema)
- [Performance Considerations](#performance-considerations)

## Overview

The `Emitter` interface provides a standard way to capture and process workflow execution events:

```go
type Emitter interface {
    Emit(event Event)
    EmitBatch(ctx context.Context, events []Event) error
    Flush(ctx context.Context) error
}
```

Events are emitted at key points during workflow execution:
- Node execution start/completion
- State transitions
- Errors and warnings
- Checkpoint operations
- Routing decisions

## Available Emitters

### LogEmitter

Writes structured logs to any `io.Writer` in text or JSON format.

```go
import (
    "log"
    "os"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Human-readable text output to stdout
textEmitter := emit.NewLogEmitter(os.Stdout, false)

// Machine-readable JSON output to file
f, err := os.Create("events.jsonl")
if err != nil {
    log.Fatal(err)
}
defer f.Close()
jsonEmitter := emit.NewLogEmitter(f, true)
```

**Use cases:**
- Development and debugging
- Simple production deployments
- File-based audit trails

### BufferedEmitter

Stores events in memory with query capabilities for execution history analysis.

```go
import (
    "context"
    "log"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Create buffered emitter
emitter := emit.NewBufferedEmitter()

// Run workflow
_, err := engine.Run(ctx, "run-001", initialState)
if err != nil {
    log.Printf("Workflow failed: %v", err)
}

// Query execution history
allEvents := emitter.GetHistory("run-001")
errorEvents := emitter.GetHistoryWithFilter("run-001", emit.HistoryFilter{
    Msg: "error",
})

// Clean up after analysis to prevent memory leaks
emitter.Clear("run-001")
```

**Use cases:**
- Testing and validation
- Real-time monitoring dashboards
- Post-execution analysis

**⚠️ Memory Warning:**
- Stores **ALL events in memory** with no automatic eviction
- Memory usage grows unbounded for long-running workflows
- **Best practices:**
  - Call `Clear(runID)` after analyzing each workflow
  - Monitor memory usage: `len(emitter.GetHistory(runID))` events stored
  - For production, use LogEmitter or OTelEmitter with external storage
  - Limit to short-lived workflows or implement periodic cleanup

### NullEmitter

Discards all events for maximum performance.

```go
emitter := emit.NewNullEmitter()
```

**Use cases:**
- Performance-critical production deployments where observability is disabled
- Benchmarking workflow execution overhead

### OTelEmitter

Creates OpenTelemetry spans for distributed tracing. See [OpenTelemetry Integration](#opentelemetry-integration) for details.

## OpenTelemetry Integration

The `OTelEmitter` integrates LangGraph-Go workflows with OpenTelemetry distributed tracing, enabling visualization of workflow execution in tools like Jaeger, Zipkin, and cloud APM platforms.

### Quick Start

```go
import (
    "context"
    "log"
    "time"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/emit"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Setup OpenTelemetry with Jaeger exporter
func setupTracing() (*sdktrace.TracerProvider, error) {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("langgraph-workflow"),
        )),
    )
    otel.SetTracerProvider(tp)
    return tp, nil
}

// Create OTelEmitter and use with engine
func main() {
    tp, err := setupTracing()
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := tp.Shutdown(ctx); err != nil {
            log.Printf("Error shutting down tracer provider: %v", err)
        }
    }()

    tracer := otel.Tracer("langgraph-go")
    emitter := emit.NewOTelEmitter(tracer)

    engine := graph.New[MyState](
        graph.WithEmitter(emitter),
    )

    // Run workflow - spans will be created automatically
    ctx := context.Background()
    _, err = engine.Run(ctx, "run-001", initialState)
    if err != nil {
        log.Printf("Workflow failed: %v", err)
    }
}
```

### Exporters

OpenTelemetry supports multiple backends:

#### Jaeger (Self-Hosted)

```go
import "go.opentelemetry.io/otel/exporters/jaeger"

exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
    jaeger.WithEndpoint("http://localhost:14268/api/traces"),
))
```

#### OTLP (OpenTelemetry Protocol)

```go
import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

exporter, err := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)
```

#### Zipkin

```go
import "go.opentelemetry.io/otel/exporters/zipkin"

exporter, err := zipkin.New("http://localhost:9411/api/v2/spans")
```

#### Console (Development)

```go
import "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

exporter, err := stdouttrace.New(
    stdouttrace.WithPrettyPrint(),
)
```

### Span Schema

Each event creates a span with the following structure:

#### Span Name
The span name is set to `event.Msg` (e.g., `node_start`, `node_end`, `error`).

#### Standard Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `langgraph.run_id` | string | Unique workflow execution identifier |
| `langgraph.step` | int | Sequential step number (1-indexed) |
| `langgraph.node_id` | string | Node identifier that emitted the event |

#### Metadata Attributes

Event metadata (`event.Meta`) is automatically mapped to span attributes:

**Type Conversions:**
- `string` → `attribute.String`
- `int`, `int64` → `attribute.Int64`
- `float64` → `attribute.Float64`
- `bool` → `attribute.Bool`
- `time.Duration` → `attribute.Int64` (milliseconds)
- Other types → `attribute.String` (formatted)

**Special Mappings:**

| Event Meta Key | Span Attribute | Description |
|----------------|----------------|-------------|
| `input_tokens` | `langgraph.llm.input_tokens` | LLM input token count |
| `output_tokens` | `langgraph.llm.output_tokens` | LLM output token count |
| `cost` | `langgraph.llm.cost` | LLM cost in USD |
| `duration_ms` | `langgraph.node.duration_ms` | Node execution duration |
| `model` | `langgraph.llm.model` | LLM model identifier |

**Note**: Use these canonical metadata keys consistently. The OTel emitter automatically maps these to span attributes.

#### Concurrency Attributes

For concurrent execution tracking:

| Attribute | Type | Description |
|-----------|------|-------------|
| `langgraph.step_id` | string | Unique step execution identifier |
| `langgraph.order_key` | string | Deterministic ordering key for replay |
| `langgraph.attempt` | int | Retry attempt number (0-indexed) |

#### Error Status

When `event.Meta["error"]` exists:
- Span status is set to `codes.Error`
- Error message is recorded as span status description
- Error event is added to the span

### Flushing Spans

Always flush spans before application shutdown to ensure delivery:

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := emitter.Flush(ctx); err != nil {
        log.Printf("Failed to flush spans: %v", err)
    }
}()
```

## Multi-Emitter Pattern

Combine multiple emitters to send events to different backends simultaneously:

```go
import (
    "context"
    "errors"
    "os"

    "github.com/dshills/langgraph-go/graph/emit"
    "go.opentelemetry.io/otel"
)

// MultiEmitter wraps multiple emitters
type MultiEmitter struct {
    emitters []emit.Emitter
}

func NewMultiEmitter(emitters ...emit.Emitter) *MultiEmitter {
    return &MultiEmitter{emitters: emitters}
}

func (m *MultiEmitter) Emit(event emit.Event) {
    for _, e := range m.emitters {
        e.Emit(event)
    }
}

func (m *MultiEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
    var errs []error
    for _, e := range m.emitters {
        if err := e.EmitBatch(ctx, events); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...) // Preserves all errors with proper unwrapping
    }
    return nil
}

func (m *MultiEmitter) Flush(ctx context.Context) error {
    var errs []error
    for _, e := range m.emitters {
        if err := e.Flush(ctx); err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return errors.Join(errs...) // Preserves all errors with proper unwrapping
    }
    return nil
}

// Note: errors.Join requires Go 1.20+. For earlier versions, use a multierror package
// or concatenate error messages manually.

// Usage: logs + distributed tracing
func main() {
    logEmitter := emit.NewLogEmitter(os.Stdout, true)
    otelEmitter := emit.NewOTelEmitter(otel.Tracer("langgraph-go"))

    multi := NewMultiEmitter(logEmitter, otelEmitter)

    engine := graph.New[MyState](
        graph.WithEmitter(multi),
    )
}
```

## Event Schema

Events follow a standard structure defined in `event.go`:

```go
type Event struct {
    RunID  string                 // Workflow execution ID
    Step   int                    // Sequential step number
    NodeID string                 // Node that emitted the event
    Msg    string                 // Event type/message
    Meta   map[string]interface{} // Additional metadata
}
```

### ⚠️ Security: Sensitive Data Handling

**Critical**: Event metadata can contain sensitive information. Always sanitize before emission:

**Sensitive Fields to Redact:**
- User prompts and LLM inputs (may contain PII)
- API keys and authentication tokens
- Personal identifiable information (names, emails, addresses)
- System credentials and connection strings
- Proprietary business logic or trade secrets

**Best Practices:**
- Implement metadata redaction filters before emission
- Use hash/truncate for sensitive values (e.g., hash user IDs)
- Configure allowlist/blocklist for metadata keys
- Encrypt event storage at rest and in transit
- Set retention policies and compliance controls
- Review OpenTelemetry collector configuration for sensitive attribute filtering

**Example Redaction:**
```go
func sanitizeMetadata(meta map[string]interface{}) map[string]interface{} {
    sanitized := make(map[string]interface{})
    redactKeys := map[string]bool{"prompt": true, "api_key": true, "user_email": true}

    for k, v := range meta {
        if redactKeys[k] {
            sanitized[k] = "[REDACTED]"
        } else if str, ok := v.(string); ok && len(str) > 1000 {
            sanitized[k] = str[:1000] + "... [TRUNCATED]"
        } else {
            sanitized[k] = v
        }
    }
    return sanitized
}
```

### Standard Metadata Conventions

#### Performance Metrics
- `duration_ms` (int64): Execution duration in milliseconds
- `memory_bytes` (int64): Memory usage in bytes
- `cpu_percent` (float64): CPU utilization percentage

#### Error Context
- `error` (string): Error message
- `error_type` (string): Error classification
- `retryable` (bool): Whether error can be retried
- `retry_attempt` (int): Current retry attempt (1-indexed)
- `stack_trace` (string): Stack trace for debugging

#### LLM-Specific
- `tokens` (int): Total token count
- `input_tokens` (int): Input token count
- `output_tokens` (int): Output token count
- `cost` (float64): Estimated cost in USD
- `model` (string): Model identifier
- `temperature` (float64): Temperature parameter
- `max_tokens` (int): Max tokens parameter
- `finish_reason` (string): Completion reason

#### Node Classification
- `node_type` (string): Node category (e.g., "llm", "tool", "processor")
- `node_version` (string): Node implementation version

#### Routing Decisions
- `next_node` (string): Next node ID for single routing
- `next_nodes` ([]string): Multiple next nodes for fan-out
- `routing_reason` (string): Explanation of routing decision
- `condition` (string): Condition that triggered the route

### Event Helper Methods

```go
// Add duration metadata
event := Event{RunID: "run-001", Msg: "node_end"}.
    WithDuration(250 * time.Millisecond)

// Add error metadata
event := Event{RunID: "run-001", Msg: "error"}.
    WithError(errors.New("validation failed"))

// Add node type metadata
event := Event{RunID: "run-001", NodeID: "llm-node"}.
    WithNodeType("llm")

// Chain multiple metadata fields (using canonical keys)
event := Event{RunID: "run-001", NodeID: "llm-node", Msg: "node_end"}.
    WithDuration(250 * time.Millisecond).
    WithNodeType("llm").
    WithMeta("input_tokens", 100).
    WithMeta("output_tokens", 50).
    WithMeta("cost", 0.003).
    WithMeta("model", "gpt-4")
```

## Performance Considerations

### Batching

Use `EmitBatch` for high-volume workflows:

```go
// Collect events during execution
events := []emit.Event{
    {RunID: "run-001", Step: 1, NodeID: "nodeA", Msg: "node_start"},
    {RunID: "run-001", Step: 1, NodeID: "nodeA", Msg: "node_end"},
    {RunID: "run-001", Step: 2, NodeID: "nodeB", Msg: "node_start"},
}

// Emit in batch for better performance
if err := emitter.EmitBatch(ctx, events); err != nil {
    log.Printf("Batch emit failed: %v", err)
}
```

**Benefits:**
- Reduces network round-trips
- Amortizes serialization overhead
- Enables backend bulk insert optimizations
- Improves throughput for concurrent workflows

### Async Emission

Emitters should not block workflow execution. For high-throughput scenarios, buffer events asynchronously:

```go
import (
    "context"
    "log"
    "sync"

    "github.com/dshills/langgraph-go/graph/emit"
)

type AsyncEmitter struct {
    delegate emit.Emitter
    buffer   chan emit.Event
    wg       sync.WaitGroup
    closed   bool
    mu       sync.Mutex
}

func NewAsyncEmitter(delegate emit.Emitter, bufferSize int) *AsyncEmitter {
    a := &AsyncEmitter{
        delegate: delegate,
        buffer:   make(chan emit.Event, bufferSize),
    }
    a.wg.Add(1)
    go a.worker()
    return a
}

func (a *AsyncEmitter) worker() {
    defer a.wg.Done()
    for event := range a.buffer {
        a.delegate.Emit(event)
    }
}

func (a *AsyncEmitter) Emit(event emit.Event) {
    a.mu.Lock()
    defer a.mu.Unlock()

    if a.closed {
        return
    }

    // Hold mutex during send to prevent race with Flush closing the channel
    select {
    case a.buffer <- event:
        // Buffered successfully
    default:
        // Buffer full - log and drop (or block)
        log.Printf("Event buffer full, dropping event: %v", event.Msg)
    }
}

func (a *AsyncEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
    for _, event := range events {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            a.Emit(event)
        }
    }
    return nil
}

func (a *AsyncEmitter) Flush(ctx context.Context) error {
    a.mu.Lock()
    if a.closed {
        a.mu.Unlock()
        return nil
    }
    a.closed = true
    close(a.buffer)
    a.mu.Unlock()

    // Wait for worker to drain with timeout
    done := make(chan struct{})
    go func() {
        a.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return a.delegate.Flush(ctx)
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Sampling

For extremely high-volume workflows, emit only a percentage of events:

```go
import (
    "context"
    "math/rand"

    "github.com/dshills/langgraph-go/graph/emit"
)

type SamplingEmitter struct {
    delegate   emit.Emitter
    sampleRate float64 // 0.0 to 1.0
}

func NewSamplingEmitter(delegate emit.Emitter, sampleRate float64) *SamplingEmitter {
    return &SamplingEmitter{
        delegate:   delegate,
        sampleRate: sampleRate,
    }
}

func (s *SamplingEmitter) Emit(event emit.Event) {
    if rand.Float64() < s.sampleRate {
        s.delegate.Emit(event)
    }
}

func (s *SamplingEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
    sampled := make([]emit.Event, 0, len(events))
    for _, event := range events {
        if rand.Float64() < s.sampleRate {
            sampled = append(sampled, event)
        }
    }
    if len(sampled) > 0 {
        return s.delegate.EmitBatch(ctx, sampled)
    }
    return nil
}

func (s *SamplingEmitter) Flush(ctx context.Context) error {
    return s.delegate.Flush(ctx)
}
```

### OpenTelemetry Performance

OpenTelemetry spans are lightweight but not free. Performance tips:

1. **Use Batch Span Processor** (default in SDK):
   ```go
   tp := sdktrace.NewTracerProvider(
       sdktrace.WithBatcher(exporter), // Batches spans automatically
   )
   ```

2. **Limit Attribute Size**: Keep metadata small (< 1KB per span)

3. **Sample Traces**: Use sampling for high-volume production:
   ```go
   tp := sdktrace.NewTracerProvider(
       sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 10% sampling
   )
   ```

4. **Set Batch Timeouts**: Control export frequency:
   ```go
   tp := sdktrace.NewTracerProvider(
       sdktrace.WithBatcher(exporter,
           sdktrace.WithBatchTimeout(5*time.Second),
           sdktrace.WithMaxExportBatchSize(512),
       ),
   )
   ```

### Memory Usage

**LogEmitter**: Minimal overhead (writes directly to io.Writer)

**BufferedEmitter**: Stores all events in memory. Monitor with:
```go
events := emitter.GetHistory("run-001")
fmt.Printf("Stored %d events for run-001\n", len(events))
```

**OTelEmitter**: Spans are batched in memory before export. Monitor OpenTelemetry metrics for buffer usage.

### Benchmarking

Measure emitter overhead in your workflow:

```go
func BenchmarkEmitter(b *testing.B) {
    emitter := emit.NewOTelEmitter(otel.Tracer("test"))
    event := emit.Event{
        RunID: "run-001",
        Step: 1,
        NodeID: "nodeA",
        Msg: "node_start",
        Meta: map[string]interface{}{
            "node_type": "llm",
        },
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        emitter.Emit(event)
    }
}
```

## Examples

See the examples directory for complete working examples:
- `examples/otel-jaeger/` - Jaeger integration
- `examples/otel-console/` - Console output for development
- `examples/multi-emitter/` - Combining multiple backends

## References

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/instrumentation/go/)
- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Jaeger Getting Started](https://www.jaegertracing.io/docs/latest/getting-started/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
