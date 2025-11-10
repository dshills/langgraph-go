// Package main demonstrates sequential execution with retry policy configuration.
//
// This example shows how to configure per-node retry behavior with:
// 1. Sequential execution mode (MaxConcurrentNodes: 0)
// 2. Retry policy with exponential backoff
// 3. Error classification (retryable vs permanent)
// 4. Deterministic backoff via seeded RNG
// 5. Retry attempt tracking via context
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// WorkflowState tracks execution progress and results
type WorkflowState struct {
	Step       string
	Results    []string
	ErrorCount int
	LastError  string
}

// TransientError represents a temporary failure that should be retried
type TransientError struct {
	Message string
}

func (e TransientError) Error() string {
	return fmt.Sprintf("transient error: %s", e.Message)
}

// PermanentError represents a failure that should not be retried
type PermanentError struct {
	Message string
}

func (e PermanentError) Error() string {
	return fmt.Sprintf("permanent error: %s", e.Message)
}

// flakyNode simulates a node that fails transiently before succeeding
type flakyNode struct {
	failUntilAttempt int
}

func (n flakyNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	// Get current retry attempt from context
	attempt := 0
	if attemptVal := ctx.Value(graph.AttemptKey); attemptVal != nil {
		attempt = attemptVal.(int)
	}

	result := fmt.Sprintf("Attempt %d: ", attempt)

	// Fail transiently until reaching success threshold
	if attempt < n.failUntilAttempt {
		result += "Service temporarily unavailable (will retry)"
		return graph.NodeResult[WorkflowState]{
			Delta: WorkflowState{
				Step:    "flaky",
				Results: []string{result},
			},
			Err: TransientError{Message: "service unavailable"},
		}
	}

	// Success after retries
	result += "Success! Service is now available"
	return graph.NodeResult[WorkflowState]{
		Delta: WorkflowState{
			Step:    "flaky",
			Results: []string{result},
		},
		Route: graph.Goto("validator"),
	}
}

// Policy configures retry behavior for the flaky node
func (n flakyNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 5 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 5,                      // Up to 5 total attempts (initial + 4 retries)
			BaseDelay:   100 * time.Millisecond, // Start with 100ms delay
			MaxDelay:    2 * time.Second,        // Cap exponential backoff at 2s
			Retryable: func(err error) bool {
				// Only retry transient errors
				var transientErr TransientError
				return errors.As(err, &transientErr)
			},
		},
	}
}

// permanentFailureNode demonstrates non-retryable errors
type permanentFailureNode struct{}

func (n permanentFailureNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	attempt := 0
	if attemptVal := ctx.Value(graph.AttemptKey); attemptVal != nil {
		attempt = attemptVal.(int)
	}

	result := fmt.Sprintf("Attempt %d: Invalid configuration detected", attempt)
	return graph.NodeResult[WorkflowState]{
		Delta: WorkflowState{
			Step:    "permanent_failure",
			Results: []string{result},
		},
		Err: PermanentError{Message: "invalid configuration"},
	}
}

// Policy configures retry for permanent failure node
func (n permanentFailureNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			Retryable: func(err error) bool {
				// Permanent errors are not retryable
				var permErr PermanentError
				return !errors.As(err, &permErr)
			},
		},
	}
}

// maxAttemptsNode demonstrates exhausting retry attempts
type maxAttemptsNode struct{}

func (n maxAttemptsNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	attempt := 0
	if attemptVal := ctx.Value(graph.AttemptKey); attemptVal != nil {
		attempt = attemptVal.(int)
	}

	// Get RNG from context for deterministic behavior
	var seed int64
	if rngVal := ctx.Value(graph.RNGKey); rngVal != nil {
		// Note: This is for demonstration - in real code you'd use the RNG
		seed = 1 // placeholder
	}

	result := fmt.Sprintf("Attempt %d (seed=%d): Network timeout", attempt, seed)
	return graph.NodeResult[WorkflowState]{
		Delta: WorkflowState{
			Step:    "max_attempts",
			Results: []string{result},
		},
		Err: TransientError{Message: "network timeout"},
	}
}

// Policy configures limited retries
func (n maxAttemptsNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 2, // Only 1 retry allowed
			BaseDelay:   50 * time.Millisecond,
			MaxDelay:    500 * time.Millisecond,
			Retryable: func(err error) bool {
				var transientErr TransientError
				return errors.As(err, &transientErr)
			},
		},
	}
}

// validatorNode runs after successful retry
type validatorNode struct{}

func (n validatorNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	result := "Validation complete - workflow succeeded after retries"
	return graph.NodeResult[WorkflowState]{
		Delta: WorkflowState{
			Step:    "validator",
			Results: []string{result},
		},
		Route: graph.Stop(),
	}
}

// successNode demonstrates no-retry configuration
type successNode struct{}

func (n successNode) Run(ctx context.Context, state WorkflowState) graph.NodeResult[WorkflowState] {
	return graph.NodeResult[WorkflowState]{
		Delta: WorkflowState{
			Step:    "success",
			Results: []string{"No retry policy - completes on first attempt"},
		},
		Route: graph.Stop(),
	}
}

func main() {
	fmt.Println("=== Sequential Retry Policy Example ===\n")

	// Create reducer that merges state updates
	reducer := func(prev, delta WorkflowState) WorkflowState {
		if delta.Step != "" {
			prev.Step = delta.Step
		}
		if delta.LastError != "" {
			prev.LastError = delta.LastError
		}
		prev.ErrorCount += delta.ErrorCount
		prev.Results = append(prev.Results, delta.Results...)
		return prev
	}

	// Scenario 1: Successful retry after transient failures
	fmt.Println("=== Scenario 1: Transient Failure with Successful Retry ===")
	runScenario1(reducer)

	// Scenario 2: Permanent failure (no retry)
	fmt.Println("\n=== Scenario 2: Permanent Failure (Non-Retryable) ===")
	runScenario2(reducer)

	// Scenario 3: Max attempts exceeded
	fmt.Println("\n=== Scenario 3: Max Retry Attempts Exceeded ===")
	runScenario3(reducer)

	// Scenario 4: No retry policy (immediate failure)
	fmt.Println("\n=== Scenario 4: Node Without Retry Policy ===")
	runScenario4(reducer)

	// Summary
	fmt.Println("\n=== Key Takeaways ===")
	printTakeaways()
}

func runScenario1(reducer func(WorkflowState, WorkflowState) WorkflowState) {
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{
		MaxSteps:           20,
		MaxConcurrentNodes: 0, // Sequential execution (nodes run one at a time)
	}
	engine := graph.New(reducer, st, emitter, opts)

	// Add nodes
	_ = engine.Add("flaky", flakyNode{failUntilAttempt: 2}) // Fails twice, succeeds on attempt 2
	_ = engine.Add("validator", validatorNode{})
	_ = engine.StartAt("flaky")

	// Run workflow
	ctx := context.Background()
	fmt.Println("Configuration:")
	fmt.Println("- Node will fail on attempts 0 and 1 (transient errors)")
	fmt.Println("- Node will succeed on attempt 2")
	fmt.Println("- MaxAttempts: 5, BaseDelay: 100ms, MaxDelay: 2s")
	fmt.Println("- Sequential execution (MaxConcurrentNodes: 0)")
	fmt.Println()

	start := time.Now()
	finalState, err := engine.Run(ctx, "retry-scenario1", WorkflowState{})
	duration := time.Since(start)

	// Display results
	fmt.Printf("Execution time: %v\n", duration)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Status: SUCCESS")
	}
	fmt.Println("\nExecution trace:")
	for i, result := range finalState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}

	// Expected backoff delays:
	// - Attempt 0 → fails → backoff: 100ms * 2^0 + jitter = ~100-200ms
	// - Attempt 1 → fails → backoff: 100ms * 2^1 + jitter = ~200-300ms
	// - Attempt 2 → succeeds
	// Total expected delay: ~300-500ms + execution time
	fmt.Printf("\nExpected behavior:")
	fmt.Printf("\n- Attempt 0 fails → backoff ~100-200ms")
	fmt.Printf("\n- Attempt 1 fails → backoff ~200-300ms")
	fmt.Printf("\n- Attempt 2 succeeds")
	fmt.Printf("\n- Total backoff: ~300-500ms (deterministic based on runID)\n")
}

func runScenario2(reducer func(WorkflowState, WorkflowState) WorkflowState) {
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewNullEmitter() // Suppress logs for clarity
	opts := graph.Options{
		MaxSteps:           10,
		MaxConcurrentNodes: 0,
	}
	engine := graph.New(reducer, st, emitter, opts)

	_ = engine.Add("permanent", permanentFailureNode{})
	_ = engine.StartAt("permanent")

	ctx := context.Background()
	fmt.Println("Configuration:")
	fmt.Println("- Node returns PermanentError (non-retryable)")
	fmt.Println("- RetryPolicy.Retryable returns false for PermanentError")
	fmt.Println("- Expected: Immediate failure without retries")
	fmt.Println()

	start := time.Now()
	finalState, err := engine.Run(ctx, "retry-scenario2", WorkflowState{})
	duration := time.Since(start)

	fmt.Printf("Execution time: %v\n", duration)
	if err != nil {
		fmt.Printf("Error (expected): %v\n", err)
	}
	fmt.Println("\nExecution trace:")
	for i, result := range finalState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}
	fmt.Println("\nResult: Failed immediately (no retries for permanent errors)")
}

func runScenario3(reducer func(WorkflowState, WorkflowState) WorkflowState) {
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewNullEmitter()
	opts := graph.Options{
		MaxSteps:           10,
		MaxConcurrentNodes: 0,
	}
	engine := graph.New(reducer, st, emitter, opts)

	_ = engine.Add("max_attempts", maxAttemptsNode{})
	_ = engine.StartAt("max_attempts")

	ctx := context.Background()
	fmt.Println("Configuration:")
	fmt.Println("- Node always fails with TransientError (retryable)")
	fmt.Println("- MaxAttempts: 2 (initial attempt + 1 retry)")
	fmt.Println("- Expected: Fails with ErrMaxAttemptsExceeded after 2 attempts")
	fmt.Println()

	start := time.Now()
	finalState, err := engine.Run(ctx, "retry-scenario3", WorkflowState{})
	duration := time.Since(start)

	fmt.Printf("Execution time: %v\n", duration)
	if errors.Is(err, graph.ErrMaxAttemptsExceeded) {
		fmt.Println("Error (expected): max retry attempts exceeded")
	} else {
		fmt.Printf("Error: %v\n", err)
	}
	fmt.Println("\nExecution trace:")
	for i, result := range finalState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}
	fmt.Println("\nResult: Max attempts reached (2 total attempts)")
}

func runScenario4(reducer func(WorkflowState, WorkflowState) WorkflowState) {
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewNullEmitter()
	opts := graph.Options{
		MaxSteps:           10,
		MaxConcurrentNodes: 0,
	}
	engine := graph.New(reducer, st, emitter, opts)

	_ = engine.Add("success", successNode{})
	_ = engine.StartAt("success")

	ctx := context.Background()
	fmt.Println("Configuration:")
	fmt.Println("- Node has no RetryPolicy")
	fmt.Println("- Expected: Single execution (no retries)")
	fmt.Println()

	start := time.Now()
	finalState, err := engine.Run(ctx, "retry-scenario4", WorkflowState{})
	duration := time.Since(start)

	fmt.Printf("Execution time: %v\n", duration)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Status: SUCCESS")
	}
	fmt.Println("\nExecution trace:")
	for i, result := range finalState.Results {
		fmt.Printf("  %d. %s\n", i+1, result)
	}
	fmt.Println("\nResult: Completed immediately (no retry policy)")
}

func printTakeaways() {
	fmt.Println("1. Sequential Execution:")
	fmt.Println("   - Set MaxConcurrentNodes: 0 for sequential mode")
	fmt.Println("   - Nodes execute one at a time in deterministic order")
	fmt.Println()
	fmt.Println("2. RetryPolicy Configuration:")
	fmt.Println("   - MaxAttempts: Total attempts (includes initial execution)")
	fmt.Println("   - BaseDelay: Starting backoff delay")
	fmt.Println("   - MaxDelay: Cap for exponential backoff")
	fmt.Println("   - Retryable: Predicate to classify errors")
	fmt.Println()
	fmt.Println("3. Exponential Backoff:")
	fmt.Println("   - Formula: min(BaseDelay * 2^attempt, MaxDelay) + jitter")
	fmt.Println("   - Jitter adds randomness to prevent thundering herd")
	fmt.Println("   - Deterministic when using same runID (seeded RNG)")
	fmt.Println()
	fmt.Println("4. Accessing Retry Context:")
	fmt.Println("   - Use ctx.Value(graph.AttemptKey) for attempt number")
	fmt.Println("   - Use ctx.Value(graph.RNGKey) for deterministic RNG")
	fmt.Println("   - Attempt 0 = initial execution, 1+ = retries")
	fmt.Println()
	fmt.Println("5. Error Classification:")
	fmt.Println("   - Retryable: Transient failures (network, timeout, 503)")
	fmt.Println("   - Non-retryable: Permanent failures (validation, 404, auth)")
	fmt.Println("   - Use errors.As() to check error types")
	fmt.Println()
	fmt.Println("6. Deterministic Replay:")
	fmt.Println("   - Same runID → same RNG seed → same backoff jitter")
	fmt.Println("   - Enables exact replay for debugging and testing")
	fmt.Println("   - Critical for reproducible workflow execution")
}
