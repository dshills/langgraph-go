# Observability and Monitoring

LangGraph-Go provides comprehensive observability features for production monitoring and debugging of stateful LLM workflows. This guide covers metrics collection, cost tracking, distributed tracing, and monitoring best practices.

## Table of Contents

- [Overview](#overview)
- [Prometheus Metrics](#prometheus-metrics)
- [Cost Tracking](#cost-tracking)
- [OpenTelemetry Tracing](#opentelemetry-tracing)
- [Event Logging](#event-logging)
- [Integration Examples](#integration-examples)
- [Best Practices](#best-practices)

## Overview

LangGraph-Go supports three complementary observability approaches:

1. **Prometheus Metrics**: Real-time performance metrics for monitoring and alerting
2. **Cost Tracking**: LLM token usage and cost attribution with static pricing
3. **OpenTelemetry Tracing**: Distributed tracing with span attributes for debugging
4. **Event Logging**: Structured logging for workflow lifecycle events

Each approach serves different needs and can be used independently or combined.

## Prometheus Metrics

### Available Metrics

LangGraph-Go exposes 6 Prometheus metrics for production monitoring:

#### 1. `langgraph_inflight_nodes` (Gauge)

Current number of nodes executing concurrently.

**Use cases:**
- Monitor concurrency levels vs MaxConcurrentNodes limit
- Detect bottlenecks where nodes block waiting for resources
- Capacity planning for workflow scaling

**Labels:** None (global metric)

**Example queries:**
```promql
# Current concurrency
langgraph_inflight_nodes

# Average concurrency over 5 minutes
avg_over_time(langgraph_inflight_nodes[5m])

# Alert: High concurrency saturation
langgraph_inflight_nodes / MaxConcurrentNodes > 0.9
```

#### 2. `langgraph_queue_depth` (Gauge)

Number of pending nodes waiting for execution in the scheduler queue.

**Use cases:**
- Track backpressure and queue saturation
- Monitor workflow responsiveness
- Detect when QueueDepth limits are reached

**Labels:** None (global metric)

**Example queries:**
```promql
# Current queue depth
langgraph_queue_depth

# Alert: Queue saturation
langgraph_queue_depth / QueueDepth > 0.8

# Rate of queue growth
rate(langgraph_queue_depth[1m])
```

#### 3. `langgraph_step_latency_ms` (Histogram)

Node execution duration in milliseconds.

**Use cases:**
- P50/P95/P99 latency analysis per node
- Identify slow nodes impacting workflow performance
- SLA monitoring and alerting

**Labels:**
- `run_id`: Workflow execution ID
- `node_id`: Node that executed
- `status`: Execution outcome (`success`, `error`, `timeout`)

**Buckets:** [1, 5, 10, 50, 100, 500, 1000, 5000, 10000] ms

**Example queries:**
```promql
# P95 latency for all nodes
histogram_quantile(0.95, rate(langgraph_step_latency_ms_bucket[5m]))

# P99 latency by node
histogram_quantile(0.99,
  sum by (node_id, le) (rate(langgraph_step_latency_ms_bucket[5m]))
)

# Alert: Slow node P95 > 5s
histogram_quantile(0.95,
  sum by (node_id, le) (rate(langgraph_step_latency_ms_bucket{node_id="research_node"}[5m]))
) > 5000
```

#### 4. `langgraph_retries_total` (Counter)

Cumulative retry attempts across all nodes.

**Use cases:**
- Identify flaky nodes requiring investigation
- Monitor error patterns and transient failures
- Track retry rate as proxy for system stability

**Labels:**
- `run_id`: Workflow execution ID
- `node_id`: Node being retried
- `reason`: Retry cause (`error`, `timeout`, `transient`)

**Example queries:**
```promql
# Retry rate per second
rate(langgraph_retries_total[5m])

# Retries by node (top 5)
topk(5, sum by (node_id) (rate(langgraph_retries_total[5m])))

# Alert: High retry rate for specific node
rate(langgraph_retries_total{node_id="api_call"}[5m]) > 0.1
```

#### 5. `langgraph_merge_conflicts_total` (Counter)

Concurrent state merge conflicts detected during parallel execution.

**Use cases:**
- Monitor determinism violations in concurrent workflows
- Detect reducer errors in state composition
- Validate conflict resolution policies

**Labels:**
- `run_id`: Workflow execution ID
- `conflict_type`: Type of conflict (`reducer_error`, `state_divergence`)

**Example queries:**
```promql
# Merge conflict rate
rate(langgraph_merge_conflicts_total[5m])

# Conflicts by type
sum by (conflict_type) (rate(langgraph_merge_conflicts_total[5m]))

# Alert: Any merge conflicts (indicates non-deterministic behavior)
rate(langgraph_merge_conflicts_total[5m]) > 0
```

#### 6. `langgraph_backpressure_events_total` (Counter)

Queue saturation events where execution was throttled due to resource limits.

**Use cases:**
- Track when execution is throttled due to resource limits
- Monitor system capacity vs load
- Identify patterns of queue saturation

**Labels:**
- `run_id`: Workflow execution ID
- `reason`: Backpressure cause (`queue_full`, `max_concurrent`, `timeout`)

**Example queries:**
```promql
# Backpressure event rate
rate(langgraph_backpressure_events_total[5m])

# Backpressure by reason
sum by (reason) (rate(langgraph_backpressure_events_total[5m]))

# Alert: Frequent backpressure indicates capacity issues
rate(langgraph_backpressure_events_total[5m]) > 1
```

### Setup and Configuration

#### 1. Create Metrics Instance

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/dshills/langgraph-go/graph"
)

// Create custom registry (recommended for isolation)
registry := prometheus.NewRegistry()
metrics := graph.NewPrometheusMetrics(registry)

// Or use default registry (shares with application metrics)
metrics := graph.NewPrometheusMetrics(prometheus.DefaultRegisterer)
```

#### 2. Configure Engine with Metrics

```go
engine := graph.New(
    reducer,
    store,
    emitter,
    graph.Options{
        Metrics: metrics,  // Enable metrics collection
        MaxConcurrentNodes: 16,
        QueueDepth: 1024,
    },
)
```

#### 3. Expose Metrics HTTP Endpoint

```go
// Expose /metrics endpoint for Prometheus scraping
http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
go http.ListenAndServe(":9090", nil)

// Prometheus will scrape this endpoint based on scrape_interval
```

#### 4. Configure Prometheus Scraper

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'langgraph'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:9090']
```

### Metrics Update Behavior

Metrics are automatically updated during workflow execution:

- **Queue Depth & Inflight Nodes**: Updated every 100ms by background goroutine
- **Step Latency**: Recorded immediately after node execution completes
- **Retries**: Incremented when retry logic triggers
- **Merge Conflicts**: Incremented when reducer returns error
- **Backpressure**: Incremented when queue enqueue blocks

All metrics methods are thread-safe and use atomic operations or mutexes.

## Cost Tracking

### Overview

Cost tracking provides accurate LLM token usage and cost attribution with static pricing for major providers:

- **OpenAI**: GPT-4o, GPT-4o-mini, GPT-4-turbo, GPT-3.5-turbo
- **Anthropic**: Claude 3.5 Sonnet, Claude 3 Opus/Sonnet/Haiku
- **Google**: Gemini 1.5 Pro/Flash, Gemini 1.0 Pro

Pricing is based on static tables as of January 2025 (per 1M tokens):

| Model | Input ($/1M) | Output ($/1M) |
|-------|-------------|---------------|
| GPT-4o | $2.50 | $10.00 |
| GPT-4o-mini | $0.15 | $0.60 |
| Claude 3.5 Sonnet | $3.00 | $15.00 |
| Claude 3 Opus | $15.00 | $75.00 |
| Claude 3 Haiku | $0.25 | $1.25 |
| Gemini 1.5 Pro | $1.25 | $5.00 |
| Gemini 1.5 Flash | $0.075 | $0.30 |

### Setup and Usage

#### 1. Create Cost Tracker

```go
import "github.com/dshills/langgraph-go/graph"

// Create tracker for workflow run
tracker := graph.NewCostTracker("run-123", "USD")

// Optional: Override pricing for enterprise rates
tracker.SetCustomPricing("gpt-4o", 2.00, 8.00)  // Custom pricing
```

#### 2. Configure Engine with Cost Tracker

```go
engine := graph.New(
    reducer,
    store,
    emitter,
    graph.Options{
        CostTracker: tracker,  // Enable cost tracking
    },
)
```

#### 3. Record LLM Calls

Cost tracking is automatic when integrated with the engine. For manual recording:

```go
// Record LLM API call with token usage
err := tracker.RecordLLMCall(
    "gpt-4o",      // Model name
    1000,          // Input tokens
    500,           // Output tokens
    "nodeA",       // Node ID (optional)
)

// Calculate cost: (1000 / 1M * $2.50) + (500 / 1M * $10.00) = $0.0075
```

#### 4. Query Cost Data

```go
// Get total cost
total := tracker.GetTotalCost()
fmt.Printf("Total cost: $%.4f\n", total)

// Get per-model breakdown
costs := tracker.GetCostByModel()
for model, cost := range costs {
    fmt.Printf("%s: $%.4f\n", model, cost)
}

// Get token usage
inputTokens, outputTokens := tracker.GetTokenUsage()
fmt.Printf("Tokens: %d in, %d out\n", inputTokens, outputTokens)

// Get full call history
calls := tracker.GetCallHistory()
for _, call := range calls {
    fmt.Printf("%s at %s: %d in, %d out = $%.4f\n",
        call.Model, call.Timestamp, call.InputTokens,
        call.OutputTokens, call.CostUSD)
}
```

### Cost Tracking Accuracy

Cost calculations are accurate to within $0.01 for 100+ LLM calls, verified by tests. Accuracy depends on:

1. **Correct token counts**: LLM API must return accurate usage data
2. **Up-to-date pricing**: Update `defaultModelPricing` map as providers change rates
3. **Model name matching**: Use exact model names from provider (e.g., "gpt-4o", not "gpt4o")

For enterprise or custom deployments, use `SetCustomPricing()` to override static pricing.

## OpenTelemetry Tracing

### Overview

OpenTelemetry integration provides distributed tracing with rich span attributes for debugging and performance analysis.

### Span Attributes

All spans include standard attributes:

- `langgraph.run_id`: Unique workflow execution ID
- `langgraph.step`: Step number in workflow execution
- `langgraph.node_id`: Node that executed

#### Concurrency Attributes

For concurrent workflows, additional attributes enable trace analysis:

- `langgraph.step_id`: Unique identifier for execution step
- `langgraph.order_key`: Deterministic ordering hash for replay
- `langgraph.attempt`: Retry attempt number (0-based)

#### Cost Tracking Attributes

When cost tracking is enabled, spans include:

- `langgraph.llm.model`: LLM model name (e.g., "gpt-4o")
- `langgraph.llm.tokens_in`: Input tokens consumed
- `langgraph.llm.tokens_out`: Output tokens generated
- `langgraph.llm.cost_usd`: Calculated cost in USD

#### Performance Attributes

- `langgraph.node.latency_ms`: Node execution duration in milliseconds

### Setup and Configuration

#### 1. Configure OpenTelemetry Provider

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Create Jaeger exporter
exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
    jaeger.WithEndpoint("http://localhost:14268/api/traces"),
))
if err != nil {
    log.Fatal(err)
}

// Create trace provider
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter),
    sdktrace.WithResource(resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName("langgraph-workflow"),
    )),
)
otel.SetTracerProvider(tp)

// Flush spans on shutdown
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    tp.ForceFlush(ctx)
}()
```

#### 2. Create OTelEmitter

```go
import (
    "go.opentelemetry.io/otel"
    "github.com/dshills/langgraph-go/graph/emit"
)

tracer := otel.Tracer("langgraph-go")
emitter := emit.NewOTelEmitter(tracer)
```

#### 3. Configure Engine

```go
engine := graph.New(
    reducer,
    store,
    emitter,  // Use OTelEmitter for tracing
    graph.Options{
        CostTracker: tracker,  // Cost attributes in spans
        Metrics: metrics,       // Prometheus metrics
    },
)
```

### Viewing Traces

Traces can be viewed in:

- **Jaeger UI**: http://localhost:16686
- **Zipkin UI**: http://localhost:9411
- **Grafana Tempo**: Integrated with Grafana dashboards
- **Cloud providers**: AWS X-Ray, Google Cloud Trace, Datadog APM

## Event Logging

### Overview

Event logging provides structured logging for workflow lifecycle events.

### LogEmitter

```go
import (
    "os"
    "github.com/dshills/langgraph-go/graph/emit"
)

// Create log emitter (outputs to stdout)
logEmitter := emit.NewLogEmitter(os.Stdout, true)  // verbose=true

engine := graph.New(
    reducer,
    store,
    logEmitter,  // Use LogEmitter for structured logging
)
```

### Event Types

LogEmitter outputs structured events:

- `node_start`: Node begins execution
- `node_end`: Node completes successfully
- `node_error`: Node encounters error
- `routing_decision`: Workflow routing choice made
- `state_update`: State merged after node execution
- `retry_attempt`: Node being retried
- `workflow_complete`: Workflow reaches terminal state

Each event includes:
- `run_id`: Workflow execution ID
- `step`: Step number
- `node_id`: Node identifier
- `timestamp`: Event time
- `metadata`: Event-specific fields

## Integration Examples

### Complete Production Setup

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/emit"
    "github.com/dshills/langgraph-go/graph/store"
)

func main() {
    // 1. Setup Prometheus metrics
    registry := prometheus.NewRegistry()
    metrics := graph.NewPrometheusMetrics(registry)

    // Expose /metrics endpoint
    http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
    go http.ListenAndServe(":9090", nil)

    // 2. Setup OpenTelemetry tracing
    exporter, _ := jaeger.New(jaeger.WithCollectorEndpoint())
    tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
    otel.SetTracerProvider(tp)
    defer tp.ForceFlush(context.Background())

    tracer := otel.Tracer("langgraph-workflow")
    emitter := emit.NewOTelEmitter(tracer)

    // 3. Setup cost tracking
    tracker := graph.NewCostTracker("run-001", "USD")

    // 4. Create engine with full observability
    engine := graph.New(
        myReducer,
        store.NewMemStore[MyState](),
        emitter,
        graph.Options{
            Metrics:            metrics,
            CostTracker:        tracker,
            MaxConcurrentNodes: 16,
            QueueDepth:         1024,
        },
    )

    // Add nodes and execute
    engine.Add("nodeA", nodeA)
    engine.StartAt("nodeA")

    result, err := engine.Run(context.Background(), "run-001", MyState{})
    if err != nil {
        log.Fatal(err)
    }

    // Print cost summary
    log.Printf("Workflow complete. Total cost: $%.4f\n", tracker.GetTotalCost())
    for model, cost := range tracker.GetCostByModel() {
        log.Printf("  %s: $%.4f\n", model, cost)
    }
}
```

### Grafana Dashboard Example

See [examples/prometheus_monitoring](../examples/prometheus_monitoring) for complete Grafana dashboard JSON with:

- Workflow execution rate panel
- Node latency heatmap (P50/P95/P99)
- Retry rate by node
- Queue depth and concurrency gauges
- LLM cost breakdown by model
- Alert rules for performance degradation

## Best Practices

### Metrics

1. **Use custom registry**: Isolate workflow metrics from application metrics
2. **Set appropriate scrape interval**: 15s for most workflows, 5s for high-throughput
3. **Create alerts proactively**: Don't wait for production issues
4. **Monitor all 6 metrics**: Each provides unique insights
5. **Label cardinality**: Be cautious with high-cardinality labels (e.g., unique run_ids)

### Cost Tracking

1. **Update pricing regularly**: Check provider pricing pages quarterly
2. **Use custom pricing for enterprise**: Override defaults with actual rates
3. **Track by run and model**: Enable cost attribution to specific workflows
4. **Set budget alerts**: Monitor total cost and cost per model
5. **Validate token counts**: Ensure LLM APIs return accurate usage data

### Tracing

1. **Use sampling**: For high-volume workflows, sample traces (e.g., 1%)
2. **Batch export**: Use batch span processor for efficiency
3. **Set retention**: Configure trace backend retention policy (7-30 days typical)
4. **Correlate with logs**: Use run_id to link traces with log events
5. **Flush on shutdown**: Always flush spans before application exit

### General

1. **Start simple**: Begin with LogEmitter, add metrics and tracing as needed
2. **Test locally**: Verify observability setup works before production
3. **Document dashboards**: Share dashboard JSON with team
4. **Automate alerts**: Use Prometheus Alertmanager or cloud provider alerts
5. **Review regularly**: Check metrics weekly to identify trends and issues

## See Also

- [Prometheus Documentation](https://prometheus.io/docs/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [Grafana Dashboards](https://grafana.com/docs/grafana/latest/dashboards/)
- [Example: Prometheus Monitoring](../examples/prometheus_monitoring/)
- [Example: Tracing](../examples/tracing/)
