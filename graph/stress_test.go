package graph

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// ============================================================================
// Stress Test Framework (T006)
// ============================================================================

// StressTestConfig configures a stress test scenario with high concurrency,
// rapid execution, context cancellations, and error injection.
type StressTestConfig struct {
	// NumWorkers is the number of concurrent workers (MaxConcurrentNodes)
	NumWorkers int

	// NumExecutions is the total number of workflow executions to run
	NumExecutions int

	// NumNodesPerExecution is the number of nodes in each workflow
	NumNodesPerExecution int

	// ErrorInjectionRate is the probability (0.0-1.0) that a node will fail
	// 0.0 = no errors, 1.0 = all nodes fail
	ErrorInjectionRate float64

	// CancellationRate is the probability (0.0-1.0) that a workflow will be cancelled mid-execution
	// 0.0 = no cancellations, 1.0 = all workflows cancelled
	CancellationRate float64

	// NodeExecutionDelay is the time each node sleeps to simulate work
	// Use 0 for maximum stress (no artificial delays)
	NodeExecutionDelay time.Duration

	// ParallelExecutions determines if workflows run in parallel (true) or sequentially (false)
	ParallelExecutions bool

	// Timeout is the maximum time to wait for all executions to complete
	Timeout time.Duration
}

// StressTestStatistics captures detailed metrics from a stress test run.
type StressTestStatistics struct {
	// Total metrics
	TotalExecutions int
	TotalNodes      int
	TotalDuration   time.Duration
	StartTime       time.Time
	EndTime         time.Time

	// Success metrics
	SuccessfulExecutions int
	NodesExecuted        int64

	// Failure metrics
	FailedExecutions    int
	CancelledExecutions int
	TimeoutExecutions   int
	ErrorsInjected      int64
	ErrorsReported      int64

	// Performance metrics
	MinDuration    time.Duration
	MaxDuration    time.Duration
	AvgDuration    time.Duration
	Throughput     float64 // executions per second
	NodeThroughput float64 // nodes per second

	// Concurrency metrics
	PeakConcurrency int32
	RaceConditions  int // Detected race conditions (if any)
	Deadlocks       int // Detected deadlocks (timeouts)
}

// StressTestResult represents the outcome of a single workflow execution in a stress test.
type StressTestResult struct {
	ExecutionID int
	Success     bool
	Cancelled   bool
	Timeout     bool
	Error       error
	Duration    time.Duration
	NodesRan    int
}

// runStressTest executes a configurable stress test framework that validates
// the engine under extreme conditions.
//
// The framework supports:
// - Rapid execution with 100+ concurrent workers
// - Context cancellations at various points
// - Simultaneous error injection across nodes
// - Parallel or sequential execution patterns
// - Comprehensive statistics collection
//
// This is designed to expose race conditions, deadlocks, and resource leaks
// that might not appear in normal testing.
//
// Example usage:
//
//	config := StressTestConfig{
//	    NumWorkers: 100,
//	    NumExecutions: 500,
//	    NumNodesPerExecution: 20,
//	    ErrorInjectionRate: 0.1,  // 10% of nodes fail
//	    CancellationRate: 0.05,    // 5% of workflows cancelled
//	    ParallelExecutions: true,
//	    Timeout: 60 * time.Second,
//	}
//	stats := runStressTest(t, config)
//	t.Logf("Throughput: %.2f executions/sec", stats.Throughput)
//	if stats.Deadlocks > 0 {
//	    t.Errorf("Detected %d deadlocks", stats.Deadlocks)
//	}
func runStressTest(t *testing.T, config StressTestConfig) StressTestStatistics {
	t.Helper()

	// Apply defaults
	if config.NumWorkers <= 0 {
		config.NumWorkers = 10
	}
	if config.NumExecutions <= 0 {
		config.NumExecutions = 100
	}
	if config.NumNodesPerExecution <= 0 {
		config.NumNodesPerExecution = 10
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	// Initialize statistics
	stats := StressTestStatistics{
		TotalExecutions: config.NumExecutions,
		TotalNodes:      config.NumExecutions * config.NumNodesPerExecution,
		StartTime:       time.Now(),
		MinDuration:     time.Hour, // Will be updated
	}

	// Shared counters (thread-safe)
	var (
		nodesExecuted     atomic.Int64
		errorsInjected    atomic.Int64
		errorsReported    atomic.Int64
		activeConcurrency atomic.Int32
		peakConcurrency   atomic.Int32
	)

	// Results channel
	results := make(chan StressTestResult, config.NumExecutions)

	// Execution function
	executeWorkflow := func(executionID int) {
		// Track concurrency
		current := activeConcurrency.Add(1)
		defer activeConcurrency.Add(-1)

		// Update peak concurrency
		for {
			peak := peakConcurrency.Load()
			if current <= peak || peakConcurrency.CompareAndSwap(peak, current) {
				break
			}
		}

		result := StressTestResult{
			ExecutionID: executionID,
			Success:     true,
		}

		// Setup reducer (immutable - create new state)
		reducer := func(prev, delta TestState) TestState {
			newState := prev // Create new state instead of mutating prev
			newState.Counter += delta.Counter
			if delta.Value != "" {
				newState.Value = delta.Value
			}
			return newState
		}

		// Create engine for this execution
		st := store.NewMemStore[TestState]()
		emitter := emit.NewBufferedEmitter()
		opts := Options{
			MaxSteps:           config.NumNodesPerExecution * 2,
			MaxConcurrentNodes: config.NumWorkers,
		}
		engine := New(reducer, st, emitter, opts)

		// Track nodes executed in this workflow
		var executionNodeCount atomic.Int32

		// Create nodes with error injection
		for i := 0; i < config.NumNodesPerExecution; i++ {
			nodeID := fmt.Sprintf("node_%d", i)
			idx := i // Capture for closure

			nodeFunc := NodeFunc[TestState](func(ctx context.Context, state TestState) NodeResult[TestState] {
				// Simulate work
				if config.NodeExecutionDelay > 0 {
					time.Sleep(config.NodeExecutionDelay)
				}

				executionNodeCount.Add(1)
				nodesExecuted.Add(1)

				// Error injection
				if config.ErrorInjectionRate > 0 {
					// Use deterministic pseudo-random for consistent behavior
					if float64(idx%100)/100.0 < config.ErrorInjectionRate {
						errorsInjected.Add(1)
						return NodeResult[TestState]{
							Err: fmt.Errorf("injected error in node %d", idx),
						}
					}
				}

				return NodeResult[TestState]{
					Delta: TestState{Counter: 1},
					Route: Stop(),
				}
			})

			if err := engine.Add(nodeID, nodeFunc); err != nil {
				result.Success = false
				result.Error = fmt.Errorf("failed to add node: %w", err)
				results <- result
				return
			}
		}

		// Create start node (fan-out pattern for maximum concurrency)
		startNode := NodeFunc[TestState](func(_ context.Context, _ TestState) NodeResult[TestState] {
			nextNodes := make([]string, config.NumNodesPerExecution)
			for i := 0; i < config.NumNodesPerExecution; i++ {
				nextNodes[i] = fmt.Sprintf("node_%d", i)
			}
			return NodeResult[TestState]{
				Route: Next{Many: nextNodes},
			}
		})

		if err := engine.Add("start", startNode); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to add start node: %w", err)
			results <- result
			return
		}

		if err := engine.StartAt("start"); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to set start node: %w", err)
			results <- result
			return
		}

		// Determine if this execution should be cancelled
		shouldCancel := config.CancellationRate > 0 &&
			float64(executionID%100)/100.0 < config.CancellationRate

		// Create context with cancellation support
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Schedule cancellation if configured
		if shouldCancel {
			// Cancel after a random short delay to catch mid-execution
			go func() {
				time.Sleep(time.Millisecond * time.Duration(executionID%50))
				cancel()
			}()
		}

		// Execute workflow
		startTime := time.Now()
		runID := fmt.Sprintf("stress-test-%d", executionID)

		_, err := engine.Run(ctx, runID, TestState{})
		duration := time.Since(startTime)

		result.Duration = duration
		result.NodesRan = int(executionNodeCount.Load())

		// Classify result
		if err != nil {
			result.Success = false
			result.Error = err

			if errors.Is(err, context.Canceled) {
				result.Cancelled = true
			} else if errors.Is(err, context.DeadlineExceeded) {
				result.Timeout = true
			} else {
				errorsReported.Add(1)
			}
		}

		results <- result
	}

	// Execute workflows
	if config.ParallelExecutions {
		// Parallel execution for maximum stress
		var wg sync.WaitGroup
		for i := 0; i < config.NumExecutions; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				executeWorkflow(id)
			}(i)
		}

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All executions completed
		case <-time.After(config.Timeout):
			t.Logf("WARNING: Stress test timeout reached after %v", config.Timeout)
			// Still wait for all workers to finish to avoid closing channel while workers are sending
			<-done
		}
	} else {
		// Sequential execution
		for i := 0; i < config.NumExecutions; i++ {
			executeWorkflow(i)
		}
	}

	// Close results channel only after all workers have finished
	close(results)
	stats.EndTime = time.Now()
	stats.TotalDuration = stats.EndTime.Sub(stats.StartTime)

	// Collect results
	var totalDuration time.Duration
	for result := range results {
		if result.Success {
			stats.SuccessfulExecutions++
		} else {
			stats.FailedExecutions++
			if result.Cancelled {
				stats.CancelledExecutions++
			} else if result.Timeout {
				stats.TimeoutExecutions++
				stats.Deadlocks++ // Timeouts may indicate deadlocks
			}
		}

		// Update duration statistics
		if result.Duration < stats.MinDuration {
			stats.MinDuration = result.Duration
		}
		if result.Duration > stats.MaxDuration {
			stats.MaxDuration = result.Duration
		}
		totalDuration += result.Duration
	}

	// Finalize statistics
	stats.NodesExecuted = nodesExecuted.Load()
	stats.ErrorsInjected = errorsInjected.Load()
	stats.ErrorsReported = errorsReported.Load()
	stats.PeakConcurrency = peakConcurrency.Load()

	if stats.SuccessfulExecutions+stats.FailedExecutions > 0 {
		stats.AvgDuration = totalDuration / time.Duration(stats.SuccessfulExecutions+stats.FailedExecutions)
	}

	if stats.TotalDuration.Seconds() > 0 {
		stats.Throughput = float64(stats.SuccessfulExecutions+stats.FailedExecutions) / stats.TotalDuration.Seconds()
		stats.NodeThroughput = float64(stats.NodesExecuted) / stats.TotalDuration.Seconds()
	}

	return stats
}

// printStressTestStatistics outputs a formatted summary of stress test results.
// This is a helper function to provide consistent reporting across stress tests.
func printStressTestStatistics(t *testing.T, stats StressTestStatistics) {
	t.Helper()

	t.Logf("\n=== Stress Test Results ===")
	t.Logf("Total Duration: %v", stats.TotalDuration)
	t.Logf("")
	t.Logf("Executions:")
	t.Logf("  Total:      %d", stats.TotalExecutions)
	t.Logf("  Successful: %d (%.1f%%)", stats.SuccessfulExecutions,
		float64(stats.SuccessfulExecutions)/float64(stats.TotalExecutions)*100)
	t.Logf("  Failed:     %d (%.1f%%)", stats.FailedExecutions,
		float64(stats.FailedExecutions)/float64(stats.TotalExecutions)*100)
	t.Logf("  Cancelled:  %d", stats.CancelledExecutions)
	t.Logf("  Timeout:    %d", stats.TimeoutExecutions)
	t.Logf("")
	t.Logf("Nodes:")
	t.Logf("  Total expected: %d", stats.TotalNodes)
	t.Logf("  Executed:       %d (%.1f%%)", stats.NodesExecuted,
		float64(stats.NodesExecuted)/float64(stats.TotalNodes)*100)
	t.Logf("")
	t.Logf("Errors:")
	t.Logf("  Injected: %d", stats.ErrorsInjected)
	t.Logf("  Reported: %d", stats.ErrorsReported)
	t.Logf("")
	t.Logf("Performance:")
	t.Logf("  Throughput:      %.2f executions/sec", stats.Throughput)
	t.Logf("  Node throughput: %.2f nodes/sec", stats.NodeThroughput)
	t.Logf("  Min duration:    %v", stats.MinDuration)
	t.Logf("  Max duration:    %v", stats.MaxDuration)
	t.Logf("  Avg duration:    %v", stats.AvgDuration)
	t.Logf("")
	t.Logf("Concurrency:")
	t.Logf("  Peak concurrent executions: %d", stats.PeakConcurrency)
	t.Logf("")
	t.Logf("Issues:")
	t.Logf("  Deadlocks detected:  %d", stats.Deadlocks)
	t.Logf("  Race conditions:     %d", stats.RaceConditions)
	t.Logf("===========================\n")
}
