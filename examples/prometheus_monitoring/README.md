# Prometheus Monitoring Example

This example demonstrates comprehensive Prometheus metrics collection for LangGraph-Go workflows, including:

- Real-time performance metrics (latency, concurrency, queue depth)
- Retry and error tracking
- Parallel execution monitoring
- HTTP endpoint for Prometheus scraping

## Quick Start

```bash
# Run the example
cd examples/prometheus_monitoring
go run main.go

# Metrics will be exposed at http://localhost:9090/metrics
# The workflow will execute continuously every 2 seconds
```

## Workflow Structure

The example workflow demonstrates various execution patterns:

```
fast (1-10ms)
  → medium (50-100ms)
    → slow (500-1000ms)
      → parallel (fan-out)
        → branchA (100-500ms) ⎤
        → branchB (100-500ms) ⎥→ terminal
        → branchC (100-500ms) ⎦
```

**Node characteristics:**
- **FastNode**: Quick execution (1-10ms) - demonstrates low latency
- **MediumNode**: Medium latency (50-100ms) - typical API call
- **SlowNode**: Slow execution (500-1000ms) - simulates expensive operations
- **ParallelNode**: Fan-out to 3 parallel branches - demonstrates concurrency
- **BranchNodes**: Parallel execution with variable latency
- **FlakyNode**: Fails 30% of the time - demonstrates retry metrics

## Metrics Exposed

### 1. `langgraph_inflight_nodes`

Current number of nodes executing concurrently.

**Query examples:**
```promql
# Current concurrency
langgraph_inflight_nodes

# Peak concurrency over 5 minutes
max_over_time(langgraph_inflight_nodes[5m])
```

### 2. `langgraph_queue_depth`

Number of pending work items in the scheduler queue.

**Query examples:**
```promql
# Current queue depth
langgraph_queue_depth

# Queue saturation percentage
(langgraph_queue_depth / 64) * 100
```

### 3. `langgraph_step_latency_ms`

Node execution duration histogram.

**Query examples:**
```promql
# P95 latency for all nodes
histogram_quantile(0.95, rate(langgraph_step_latency_ms_bucket[5m]))

# P99 latency by node
histogram_quantile(0.99,
  sum by (node_id, le) (rate(langgraph_step_latency_ms_bucket[5m]))
)

# Average latency for slow node
rate(langgraph_step_latency_ms_sum{node_id="slow"}[5m]) /
rate(langgraph_step_latency_ms_count{node_id="slow"}[5m])
```

### 4. `langgraph_retries_total`

Cumulative retry attempts.

**Query examples:**
```promql
# Retry rate per second
rate(langgraph_retries_total[5m])

# Retries by node (top 5)
topk(5, sum by (node_id) (rate(langgraph_retries_total[5m])))
```

### 5. `langgraph_merge_conflicts_total`

State merge conflicts during concurrent execution.

**Query examples:**
```promql
# Conflict rate
rate(langgraph_merge_conflicts_total[5m])
```

### 6. `langgraph_backpressure_events_total`

Queue saturation events that occur when the scheduler queue reaches capacity (T033).

This counter tracks how many times the scheduler had to wait because the execution queue was full.
Backpressure is a natural throttling mechanism to prevent unbounded queue growth when nodes execute
faster than they can be drained.

**Labels:**
- `run_id`: Workflow execution identifier
- `reason`: Cause of backpressure event (currently `"queue_full"`)

**When backpressure occurs:**
1. A work item is about to be enqueued
2. Current queue depth >= queue capacity (default: 64)
3. The enqueuer waits for a slot to become available (blocking)
4. Metric increments to track the throttling event
5. When a slot opens, execution continues

**Query examples:**
```promql
# Backpressure event rate (events per second)
rate(langgraph_backpressure_events_total[5m])

# Total backpressure events by run
sum by (run_id) (langgraph_backpressure_events_total)

# Backpressure events by reason
sum by (reason) (rate(langgraph_backpressure_events_total[5m]))
```

**Example metric output:**
```
# When backpressure occurs (queue is at capacity)
langgraph_backpressure_events_total{reason="queue_full",run_id="run-1"} 3.0
langgraph_backpressure_events_total{reason="queue_full",run_id="run-2"} 7.0
```

**Interpreting backpressure metrics:**
- **0 events**: Queue never filled up during execution
- **Low rate** (<0.1/s): Normal, slight queueing under load
- **High rate** (>1/s): Queue frequently at capacity, consider:
  - Increasing `QueueDepth` (if memory allows)
  - Increasing `MaxConcurrent` (if CPU/resources allow)
  - Optimizing slow nodes
  - Distributing load across more instances

## Prometheus Configuration

### 1. Install Prometheus

**macOS:**
```bash
brew install prometheus
```

**Linux:**
```bash
wget https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.linux-amd64.tar.gz
tar xvfz prometheus-*.tar.gz
cd prometheus-*
```

**Docker:**
```bash
docker run -d -p 9091:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

### 2. Configure Prometheus Scraper

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s       # Scrape metrics every 15 seconds
  evaluation_interval: 15s   # Evaluate rules every 15 seconds

scrape_configs:
  - job_name: 'langgraph'
    static_configs:
      - targets: ['localhost:9090']  # Scrape LangGraph metrics endpoint
```

### 3. Start Prometheus

```bash
# Start Prometheus with config
prometheus --config.file=prometheus.yml

# Prometheus UI will be available at http://localhost:9091
```

## Grafana Dashboards

### Install Grafana

**macOS:**
```bash
brew install grafana
brew services start grafana
```

**Docker:**
```bash
docker run -d -p 3000:3000 grafana/grafana
```

### Configure Data Source

1. Open Grafana: http://localhost:3000 (admin/admin)
2. Add Prometheus data source:
   - URL: http://localhost:9091
   - Access: Browser
3. Save & Test

### Import Dashboard

Use the provided dashboard JSON (below) or create panels manually.

### Recommended Panels

**1. Workflow Execution Rate**
- Type: Graph
- Query: `rate(langgraph_step_latency_ms_count[5m])`
- Description: Workflows per second

**2. Node Latency Heatmap**
- Type: Heatmap
- Query: `histogram_quantile(0.95, rate(langgraph_step_latency_ms_bucket[5m]))`
- Description: P95 latency distribution by node

**3. Retry Rate by Node**
- Type: Bar chart
- Query: `sum by (node_id) (rate(langgraph_retries_total[5m]))`
- Description: Which nodes are retrying most

**4. Concurrency Gauge**
- Type: Gauge
- Query: `langgraph_inflight_nodes`
- Thresholds: Warning > 6, Critical > 7 (MaxConcurrent=8)

**5. Queue Depth Gauge**
- Type: Gauge
- Query: `langgraph_queue_depth`
- Thresholds: Warning > 50, Critical > 60 (QueueDepth=64)

**6. Error Rate**
- Type: Graph
- Query: `rate(langgraph_step_latency_ms_count{status="error"}[5m])`
- Description: Errors per second

**7. Backpressure Events (T033)**
- Type: Graph
- Query: `rate(langgraph_backpressure_events_total[5m])`
- Description: Queue saturation events per second
- Alert threshold: > 1 event/s indicates queue at capacity

### Dashboard JSON

Save this as `langgraph-dashboard.json` and import into Grafana:

```json
{
  "dashboard": {
    "title": "LangGraph Workflow Monitoring",
    "panels": [
      {
        "title": "Workflow Execution Rate",
        "targets": [
          {
            "expr": "rate(langgraph_step_latency_ms_count[5m])"
          }
        ],
        "type": "graph"
      },
      {
        "title": "Node Latency (P95)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, sum by (node_id, le) (rate(langgraph_step_latency_ms_bucket[5m])))"
          }
        ],
        "type": "graph"
      },
      {
        "title": "Retry Rate by Node",
        "targets": [
          {
            "expr": "sum by (node_id) (rate(langgraph_retries_total[5m]))"
          }
        ],
        "type": "bargauge"
      },
      {
        "title": "Concurrency",
        "targets": [
          {
            "expr": "langgraph_inflight_nodes"
          }
        ],
        "type": "gauge",
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                {"value": 0, "color": "green"},
                {"value": 6, "color": "yellow"},
                {"value": 7, "color": "red"}
              ]
            }
          }
        }
      },
      {
        "title": "Queue Depth",
        "targets": [
          {
            "expr": "langgraph_queue_depth"
          }
        ],
        "type": "gauge",
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                {"value": 0, "color": "green"},
                {"value": 50, "color": "yellow"},
                {"value": 60, "color": "red"}
              ]
            }
          }
        }
      },
      {
        "title": "Backpressure Events (T033)",
        "targets": [
          {
            "expr": "rate(langgraph_backpressure_events_total[5m])"
          }
        ],
        "type": "graph",
        "fieldConfig": {
          "defaults": {
            "thresholds": {
              "steps": [
                {"value": 0, "color": "green"},
                {"value": 1, "color": "red"}
              ]
            }
          }
        }
      }
    ]
  }
}
```

## Alert Rules

### Prometheus Alert Rules

Create `alerts.yml`:

```yaml
groups:
  - name: langgraph_alerts
    interval: 30s
    rules:
      # High latency alert
      - alert: HighNodeLatency
        expr: histogram_quantile(0.95, rate(langgraph_step_latency_ms_bucket[5m])) > 5000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High node latency detected"
          description: "P95 latency is {{ $value }}ms (threshold: 5000ms)"

      # High retry rate alert
      - alert: HighRetryRate
        expr: rate(langgraph_retries_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High retry rate detected"
          description: "Retry rate is {{ $value }}/s (threshold: 0.1/s)"

      # Queue saturation alert
      - alert: QueueSaturated
        expr: (langgraph_queue_depth / 64) > 0.8
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Workflow queue is saturated"
          description: "Queue depth is {{ $value }}% of capacity"

      # Backpressure alert
      - alert: FrequentBackpressure
        expr: rate(langgraph_backpressure_events_total[5m]) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Frequent backpressure events"
          description: "Backpressure rate is {{ $value }}/s"
```

Load alerts in Prometheus:

```yaml
# prometheus.yml
rule_files:
  - "alerts.yml"
```

## Troubleshooting

### Metrics Not Showing Up

1. **Check metrics endpoint:**
   ```bash
   curl http://localhost:9090/metrics | grep langgraph
   ```

2. **Verify Prometheus scraping:**
   - Go to http://localhost:9091/targets
   - Ensure `langgraph` target status is UP

3. **Check Prometheus logs:**
   ```bash
   # Look for scrape errors
   grep "error" prometheus.log
   ```

### High Queue Depth

If queue depth consistently high:

1. **Increase QueueDepth:**
   ```go
   Options{QueueDepth: 1024}  // Default is 64
   ```

2. **Increase MaxConcurrentNodes:**
   ```go
   Options{MaxConcurrentNodes: 16}  // Default is 8
   ```

3. **Optimize slow nodes:**
   - Check P95/P99 latencies
   - Add caching or batching
   - Consider async patterns

### High Retry Rate

If retry rate unexpectedly high:

1. **Check error types:**
   ```promql
   sum by (node_id, reason) (rate(langgraph_retries_total[5m]))
   ```

2. **Increase retry budget:**
   ```go
   RetryPolicy{MaxAttempts: 5}  // More retry attempts
   ```

3. **Add backoff:**
   ```go
   RetryPolicy{
       BaseDelay: 1 * time.Second,
       MaxDelay:  30 * time.Second,
   }
   ```

### High Backpressure Events (T033)

If `langgraph_backpressure_events_total` rate is consistently high (>1/s):

1. **Monitor queue saturation:**
   ```promql
   # Queue saturation percentage
   (langgraph_queue_depth / 64) * 100

   # When saturation is high, backpressure will trigger
   rate(langgraph_backpressure_events_total[5m]) > 1
   ```

2. **Increase queue capacity:**
   ```go
   // Increase from default 64 to 256
   graph.WithQueueDepth(256)
   ```

3. **Increase concurrent node capacity:**
   ```go
   // Increase from default 8 to 16
   graph.WithMaxConcurrent(16)
   ```

4. **Optimize slow nodes:**
   - Identify which nodes cause queue buildup
   - Check `langgraph_step_latency_ms` per node
   - Add caching or async patterns to slow operations

5. **Scale horizontally:**
   - Run multiple workflow instances
   - Use load balancer to distribute work
   - Each instance gets own queue

## Performance Tips

1. **Scrape interval**: 15s is good balance of granularity vs load
2. **Retention**: 15 days is typical (adjust based on disk space)
3. **Label cardinality**: Avoid high-cardinality labels like UUID run_ids
4. **Metric types**: Use histograms for latency, gauges for levels, counters for totals
5. **Query efficiency**: Use `rate()` for counters, avoid `avg()` on histograms

## See Also

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Alerting Rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
- [Main Documentation](../../docs/observability.md)
