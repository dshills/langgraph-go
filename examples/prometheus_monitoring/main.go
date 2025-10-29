// Package main demonstrates usage of the LangGraph-Go framework.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// State represents the workflow state with metrics-relevant fields.
type State struct {
	Counter       int
	LastNodeID    string
	ExecutionPath []string
	TotalLatency  time.Duration
	RetryCount    int
}

// Reducer merges state updates.
func reducer(prev, delta State) State {
	if delta.LastNodeID != "" {
		prev.LastNodeID = delta.LastNodeID
		prev.ExecutionPath = append(prev.ExecutionPath, delta.LastNodeID)
	}
	prev.Counter += delta.Counter
	prev.TotalLatency += delta.TotalLatency
	prev.RetryCount += delta.RetryCount
	return prev
}

// Node implementations demonstrating metrics collection.

// FastNode executes quickly (1-10ms).
func FastNode(_ context.Context, s State) graph.NodeResult[State] {
	start := time.Now()
	time.Sleep(time.Duration(1+rand.Intn(10)) * time.Millisecond)

	return graph.NodeResult[State]{
		Delta: State{
			LastNodeID:   "fast",
			Counter:      1,
			TotalLatency: time.Since(start),
		},
		Route: graph.Next{To: "medium"},
	}
}

// MediumNode executes with medium latency (50-100ms).
func MediumNode(_ context.Context, s State) graph.NodeResult[State] {
	start := time.Now()
	time.Sleep(time.Duration(50+rand.Intn(50)) * time.Millisecond)

	return graph.NodeResult[State]{
		Delta: State{
			LastNodeID:   "medium",
			Counter:      1,
			TotalLatency: time.Since(start),
		},
		Route: graph.Next{To: "slow"},
	}
}

// SlowNode executes slowly (500-1000ms).
func SlowNode(_ context.Context, s State) graph.NodeResult[State] {
	start := time.Now()
	time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)

	return graph.NodeResult[State]{
		Delta: State{
			LastNodeID:   "slow",
			Counter:      1,
			TotalLatency: time.Since(start),
		},
		Route: graph.Next{To: "parallel"},
	}
}

// ParallelNode demonstrates fan-out (triggers parallel branches).
func ParallelNode(_ context.Context, s State) graph.NodeResult[State] {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	return graph.NodeResult[State]{
		Delta: State{
			LastNodeID:   "parallel",
			Counter:      1,
			TotalLatency: time.Since(start),
		},
		Route: graph.Next{Many: []string{"branchA", "branchB", "branchC"}},
	}
}

// BranchNode processes parallel branch.
func BranchNode(name string) graph.NodeFunc[State] {
	return func(ctx context.Context, s State) graph.NodeResult[State] {
		start := time.Now()
		time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond)

		return graph.NodeResult[State]{
			Delta: State{
				LastNodeID:   name,
				Counter:      1,
				TotalLatency: time.Since(start),
			},
			Route: graph.Next{Terminal: true},
		}
	}
}

// FlakyNode demonstrates retry metrics (fails 30% of the time).
type FlakyNode struct{}

func (f *FlakyNode) Run(_ context.Context, s State) graph.NodeResult[State] {
	start := time.Now()
	time.Sleep(50 * time.Millisecond)

	// Fail 30% of attempts.
	if rand.Float64() < 0.3 {
		return graph.NodeResult[State]{
			Delta: State{
				RetryCount: 1,
			},
			Err: fmt.Errorf("transient error"),
		}
	}

	return graph.NodeResult[State]{
		Delta: State{
			LastNodeID:   "flaky",
			Counter:      1,
			TotalLatency: time.Since(start),
		},
		Route: graph.Next{Terminal: true},
	}
}

// Policy configures retry behavior for FlakyNode.
func (f *FlakyNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Retryable: func(err error) bool {
				return true // Retry all errors
			},
		},
	}
}

func main() {
	// 1. Setup Prometheus metrics with custom registry.
	log.Println("Setting up Prometheus metrics...")
	registry := prometheus.NewRegistry()
	metrics := graph.NewPrometheusMetrics(registry)

	// Expose /metrics endpoint for Prometheus scraping.
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	go func() {
		log.Println("Metrics server listening on :9090")
		log.Println("Prometheus metrics: http://localhost:9090/metrics")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Metrics server error: %v\n", err)
		}
	}()

	// 2. Setup cost tracker.
	tracker := graph.NewCostTracker("run-001", "USD")

	// 3. Create engine with full observability.
	log.Println("Creating workflow engine with observability...")
	engine := graph.New(
		reducer,
		store.NewMemStore[State](),
		emit.NewLogEmitter(os.Stdout, false), // Verbose logging disabled
		graph.Options{
			Metrics:            metrics,
			CostTracker:        tracker,
			MaxSteps:           100,
			MaxConcurrentNodes: 8,
			QueueDepth:         64,
		},
	)

	// 4. Build workflow graph.
	log.Println("Building workflow graph...")
	if err := engine.Add("fast", graph.NodeFunc[State](FastNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("medium", graph.NodeFunc[State](MediumNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("slow", graph.NodeFunc[State](SlowNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("parallel", graph.NodeFunc[State](ParallelNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("branchA", BranchNode("branchA")); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("branchB", BranchNode("branchB")); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("branchC", BranchNode("branchC")); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("flaky", &FlakyNode{}); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.StartAt("fast"); err != nil {
		log.Fatalf("failed to set start node: %v", err)
	}

	// 5. Setup graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 6. Run workflow continuously to generate metrics.
	log.Println("Starting continuous workflow execution...")
	log.Println("Press Ctrl+C to stop")
	log.Println("")

	runCount := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return

		case <-sigChan:
			log.Println("\nReceived interrupt signal")
			cancel()

		case <-ticker.C:
			runCount++
			runID := fmt.Sprintf("run-%d", runCount)

			log.Printf("Starting workflow execution: %s\n", runID)

			// Execute workflow.
			result, err := engine.Run(ctx, runID, State{})
			if err != nil {
				log.Printf("  Error: %v\n", err)
				continue
			}

			// Print execution summary.
			log.Printf("  Completed: %d steps, %d nodes executed\n",
				result.Counter, len(result.ExecutionPath))
			log.Printf("  Total latency: %v\n", result.TotalLatency)
			log.Printf("  Execution path: %v\n", result.ExecutionPath)
			log.Printf("  Retries: %d\n", result.RetryCount)
			log.Println("")

			// Every 10 runs, print metrics summary.
			if runCount%10 == 0 {
				printMetricsSummary(runCount)
			}
		}
	}
}

func printMetricsSummary(runCount int) {
	log.Println("===============================================")
	log.Printf("METRICS SUMMARY (after %d runs)\n", runCount)
	log.Println("===============================================")
	log.Println("View detailed metrics at: http://localhost:9090/metrics")
	log.Println("")
	log.Println("Key metrics to monitor:")
	log.Println("  - langgraph_inflight_nodes: Current concurrency")
	log.Println("  - langgraph_queue_depth: Pending work items")
	log.Println("  - langgraph_step_latency_ms: Node execution times")
	log.Println("  - langgraph_retries_total: Retry attempts")
	log.Println("")
	log.Println("Example Prometheus queries:")
	log.Println("  - histogram_quantile(0.95, rate(langgraph_step_latency_ms_bucket[5m]))")
	log.Println("  - rate(langgraph_retries_total[5m])")
	log.Println("  - langgraph_queue_depth")
	log.Println("===============================================")
	log.Println("")
}
