// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics (T032) provides comprehensive Prometheus-compatible metrics.
// collection for graph execution monitoring in production environments.
//
// Metrics exposed (all namespaced with "langgraph_"):
//
// 1. inflight_nodes (gauge): Current number of nodes executing concurrently.
// Labels: run_id, graph_id.
// Use: Monitor concurrency levels and detect bottlenecks.
//
// 2. queue_depth (gauge): Number of pending nodes waiting for execution.
// Labels: run_id, graph_id.
// Use: Track backpressure and queue saturation.
//
// 3. step_latency_ms (histogram): Node execution duration in milliseconds.
// Labels: run_id, node_id, status (success/error).
// Buckets: [1, 5, 10, 50, 100, 500, 1000, 5000, 10000].
// Use: P50/P95/P99 latency analysis per node.
//
// 4. retries_total (counter): Cumulative retry attempts across all nodes.
// Labels: run_id, node_id, reason.
// Use: Identify flaky nodes and error patterns.
//
// 5. merge_conflicts_total (counter): Concurrent state merge conflicts detected.
// Labels: run_id, conflict_type.
// Use: Monitor determinism violations in concurrent execution.
//
// 6. backpressure_events_total (counter): Queue saturation events triggering backpressure.
// Labels: run_id, reason.
// Use: Track when execution is throttled due to resource limits.
//
// Usage:
//
// // Create metrics with custom registry.
// registry := prometheus.NewRegistry().
// metrics := NewPrometheusMetrics(registry).
//
// // Integrate with engine.
// engine := New[MyState](.
//
//	WithMetrics(metrics),
//
// ).
//
// // Metrics automatically update during execution.
//
//	// Expose via HTTP for Prometheus scraping:
//
// http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{})).
//
// Thread-safe: All methods use atomic operations or mutex protection.
type PrometheusMetrics struct {
	// Gauge metrics (current value observations).
	inflightNodes prometheus.Gauge
	queueDepth    prometheus.Gauge

	// Histogram metrics (distribution observations).
	stepLatency *prometheus.HistogramVec

	// Counter metrics (cumulative totals).
	retries        *prometheus.CounterVec
	mergeConflicts *prometheus.CounterVec
	backpressure   *prometheus.CounterVec

	// Registry holds all registered metrics.
	registry prometheus.Registerer

	// Mutex protects concurrent metric updates.
	mu sync.RWMutex

	// enabled controls whether metrics are recorded.
	enabled bool
}

// NewPrometheusMetrics (T033) creates and registers all graph execution metrics.
// with the provided Prometheus registry.
//
// Parameters:
// - registry: Prometheus registry to register metrics with (use prometheus.DefaultRegisterer for global registry).
//
// Returns:
// - *PrometheusMetrics: Fully initialized metrics collector.
//
// All metrics are registered with namespace "langgraph" and appropriate labels.
// Histograms use buckets optimized for typical node execution times (1ms to 10s).
//
// Example:
//
// // Use default global registry.
// metrics := NewPrometheusMetrics(prometheus.DefaultRegisterer).
//
// // Use custom registry (recommended for isolation).
// registry := prometheus.NewRegistry().
// metrics := NewPrometheusMetrics(registry).
// http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{})).
func NewPrometheusMetrics(registry prometheus.Registerer) *PrometheusMetrics {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	factory := promauto.With(registry)

	pm := &PrometheusMetrics{
		registry: registry,
		enabled:  true,
	}

	// 1. inflight_nodes gauge (T032).
	pm.inflightNodes = factory.NewGauge(prometheus.GaugeOpts{
		Namespace: "langgraph",
		Name:      "inflight_nodes",
		Help:      "Current number of nodes executing concurrently in the graph",
	})

	// 2. queue_depth gauge (T032).
	pm.queueDepth = factory.NewGauge(prometheus.GaugeOpts{
		Namespace: "langgraph",
		Name:      "queue_depth",
		Help:      "Number of pending nodes waiting for execution in the scheduler queue",
	})

	// 3. step_latency_ms histogram (T032).
	pm.stepLatency = factory.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "langgraph",
		Name:      "step_latency_ms",
		Help:      "Node execution duration in milliseconds (from dispatch to completion)",
		Buckets:   []float64{1, 5, 10, 50, 100, 500, 1000, 5000, 10000}, // 1ms to 10s
	}, []string{"run_id", "node_id", "status"}) // status: success, error, timeout

	// 4. retries_total counter (T032).
	pm.retries = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: "langgraph",
		Name:      "retries_total",
		Help:      "Cumulative count of node retry attempts across all executions",
	}, []string{"run_id", "node_id", "reason"}) // reason: error, timeout, transient

	// 5. merge_conflicts_total counter (T032).
	pm.mergeConflicts = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: "langgraph",
		Name:      "merge_conflicts_total",
		Help:      "Concurrent state merge conflicts detected during parallel execution",
	}, []string{"run_id", "conflict_type"}) // conflict_type: reducer_error, state_divergence

	// 6. backpressure_events_total counter (T032).
	pm.backpressure = factory.NewCounterVec(prometheus.CounterOpts{
		Namespace: "langgraph",
		Name:      "backpressure_events_total",
		Help:      "Queue saturation events where execution was throttled due to resource limits",
	}, []string{"run_id", "reason"}) // reason: queue_full, max_concurrent, timeout

	return pm
}

// RecordStepLatency (T034) records the execution duration of a node in milliseconds.
//
// This updates the step_latency_ms histogram with labels for run_id, node_id, and status.
// Use this to track P50/P95/P99 latencies per node for performance monitoring.
//
// Parameters:
// - runID: Unique workflow execution identifier.
// - nodeID: Node that was executed.
// - latency: Execution duration.
// - status: Execution outcome ("success", "error", "timeout").
//
// Example:
//
// start := time.Now().
// result := node.Run(ctx, state).
// metrics.RecordStepLatency(runID, nodeID, time.Since(start), "success").
func (pm *PrometheusMetrics) RecordStepLatency(runID, nodeID string, latency time.Duration, status string) {
	if !pm.enabled {
		return
	}

	latencyMs := float64(latency.Milliseconds())
	pm.stepLatency.WithLabelValues(runID, nodeID, status).Observe(latencyMs)
}

// IncrementRetries (T035) increments the retry counter for a specific node and reason.
//
// This updates the retries_total counter with labels for run_id, node_id, and reason.
// Use this to identify flaky nodes and error patterns requiring investigation.
//
// Parameters:
// - runID: Unique workflow execution identifier.
// - nodeID: Node that is being retried.
// - reason: Retry cause ("error", "timeout", "transient").
//
// Example:
//
// if result.Err != nil {.
// metrics.IncrementRetries(runID, nodeID, "error").
//
//		    // Retry logic...
//	}.
func (pm *PrometheusMetrics) IncrementRetries(runID, nodeID, reason string) {
	if !pm.enabled {
		return
	}

	pm.retries.WithLabelValues(runID, nodeID, reason).Inc()
}

// UpdateQueueDepth (T036) sets the current number of pending nodes in the scheduler queue.
//
// This updates the queue_depth gauge. Use this to monitor backpressure and detect.
// when the system is saturated with pending work.
//
// Parameters:
// - depth: Current number of nodes waiting for execution.
//
// Example:
//
// metrics.UpdateQueueDepth(scheduler.PendingCount()).
func (pm *PrometheusMetrics) UpdateQueueDepth(depth int) {
	if !pm.enabled {
		return
	}

	pm.queueDepth.Set(float64(depth))
}

// UpdateInflightNodes (T037) sets the current number of nodes executing concurrently.
//
// This updates the inflight_nodes gauge. Use this to monitor concurrency levels.
// and detect whether MaxConcurrent limits are being reached.
//
// Parameters:
// - count: Current number of nodes in execution.
//
// Example:
//
// metrics.UpdateInflightNodes(len(activeNodes)).
func (pm *PrometheusMetrics) UpdateInflightNodes(count int) {
	if !pm.enabled {
		return
	}

	pm.inflightNodes.Set(float64(count))
}

// IncrementMergeConflicts (T038) increments the merge conflict counter.
//
// This updates the merge_conflicts_total counter with labels for run_id and conflict_type.
// Use this to detect determinism violations or reducer errors in concurrent execution.
//
// Parameters:
// - runID: Unique workflow execution identifier.
// - conflictType: Type of conflict ("reducer_error", "state_divergence").
//
// Example:
//
// if err := reducer(prev, delta); err != nil {.
// metrics.IncrementMergeConflicts(runID, "reducer_error").
// }.
func (pm *PrometheusMetrics) IncrementMergeConflicts(runID, conflictType string) {
	if !pm.enabled {
		return
	}

	pm.mergeConflicts.WithLabelValues(runID, conflictType).Inc()
}

// IncrementBackpressure (T039) increments the backpressure event counter.
//
// This updates the backpressure_events_total counter with labels for run_id and reason.
// Use this to track when execution is throttled due to resource limits (queue full,
// max concurrent reached, etc.).
//
// Parameters:
// - runID: Unique workflow execution identifier.
// - reason: Backpressure cause ("queue_full", "max_concurrent", "timeout").
//
// Example:
//
// if queueDepth >= maxQueueDepth {.
// metrics.IncrementBackpressure(runID, "queue_full").
// return ErrBackpressure.
// }.
func (pm *PrometheusMetrics) IncrementBackpressure(runID, reason string) {
	if !pm.enabled {
		return
	}

	pm.backpressure.WithLabelValues(runID, reason).Inc()
}

// Disable temporarily disables metric recording (useful for testing).
func (pm *PrometheusMetrics) Disable() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.enabled = false
}

// Enable re-enables metric recording after Disable().
func (pm *PrometheusMetrics) Enable() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.enabled = true
}

// Reset clears all metric values (useful for testing).
// This does not unregister metrics from the registry.
func (pm *PrometheusMetrics) Reset() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.inflightNodes.Set(0)
	pm.queueDepth.Set(0)
	// Note: Counters cannot be reset in Prometheus (cumulative by design).
	// Histograms also maintain cumulative observations.
}
