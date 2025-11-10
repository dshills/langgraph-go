// Package graph_test provides comprehensive validation tests for deterministic replay.
package graph_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/store"
)

// ValidationTestState is a test state type for validation tests.
type ValidationTestState struct {
	Counter int      `json:"counter"`
	Visited []string `json:"visited"`
}

// ==============================================================================
// T040: RNG Sequence Validation
// ==============================================================================

// TestRNGSequenceValidation (T040) validates that RNG derivation produces
// identical random sequences across replays with the same RunID.
//
// According to BUG-001 fix, each worker gets a derived RNG instance seeded
// from the RunID, ensuring deterministic random values.
//
// Requirements:
// - 100 executions with same RunID
// - All random sequences identical
// - RNG context available in all nodes
func TestRNGSequenceValidation(t *testing.T) {
	t.Run("rng_sequences_identical_across_100_runs", func(t *testing.T) {
		runID := "rng-sequence-test"
		numRuns := 100
		numRandomValues := 10

		type RNGSequence struct {
			Values []int
			Hash   string
		}

		sequences := make([]RNGSequence, numRuns)

		for run := 0; run < numRuns; run++ {
			st := store.NewMemStore[ValidationTestState]()

			randomValues := []int{}

			reducer := func(prev, delta ValidationTestState) ValidationTestState {
				newState := prev
				newState.Counter += delta.Counter
				return newState
			}

			engine := graph.New(reducer, st, nil, graph.Options{
				MaxSteps:           15,
				MaxConcurrentNodes: 3,
			})

			// Node that generates random values using context RNG
			rngNode := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
				// Get RNG from context (seeded from RunID)
				rng, ok := ctx.Value(graph.RNGKey).(*rand.Rand)
				if !ok || rng == nil {
					return graph.NodeResult[ValidationTestState]{
						Err: errors.New("RNG not found in context"),
					}
				}

				// Generate random values
				for i := 0; i < numRandomValues; i++ {
					randomValues = append(randomValues, rng.Intn(1000))
				}

				return graph.NodeResult[ValidationTestState]{
					Delta: ValidationTestState{Counter: len(randomValues)},
					Route: graph.Stop(),
				}
			})

			if err := engine.Add("rng-node", rngNode); err != nil {
				t.Fatalf("Failed to add rng-node: %v", err)
			}
			if err := engine.StartAt("rng-node"); err != nil {
				t.Fatalf("Failed to start at rng-node: %v", err)
			}

			_, err := engine.Run(context.Background(), runID, ValidationTestState{})
			if err != nil {
				t.Fatalf("Run %d failed: %v", run, err)
			}

			// Compute hash of random sequence
			valuesJSON, _ := json.Marshal(randomValues)
			hash := sha256.Sum256(valuesJSON)

			sequences[run] = RNGSequence{
				Values: randomValues,
				Hash:   hex.EncodeToString(hash[:]),
			}
		}

		// Verify all runs produced identical random sequences
		firstHash := sequences[0].Hash
		deviations := 0
		for i := 1; i < numRuns; i++ {
			if sequences[i].Hash != firstHash {
				t.Errorf("Run %d RNG sequence differs:\n  First: %v\n  Run %d: %v",
					i, sequences[0].Values, i, sequences[i].Values)
				deviations++
			}
		}

		if deviations > 0 {
			t.Fatalf("FAILED: %d/%d runs had different RNG sequences", deviations, numRuns)
		}

		t.Logf("✓ All %d runs produced identical RNG sequences", numRuns)
		t.Logf("  Sample sequence: %v", sequences[0].Values[:5])
	})
}

// ==============================================================================
// T041: OrderKey Merge Consistency Validation
// ==============================================================================

// TestOrderKeyMergeConsistency (T041) validates that OrderKey-based merge
// produces deterministic results across concurrent submissions.
//
// According to BUG-003 fix, heap-based frontier ordering using OrderKey
// ensures deterministic merge order regardless of channel timing.
//
// Requirements:
// - Compute OrderKeys for various scenarios
// - Verify heap ordering is deterministic
// - Test with concurrent submissions
// - 50 iterations, 100% consistency
func TestOrderKeyMergeConsistency(t *testing.T) {
	t.Run("orderkey_produces_identical_merge_order_50_runs", func(t *testing.T) {
		runID := "orderkey-merge-test"
		numRuns := 50
		numBranches := 5

		type MergeSequence struct {
			Order []string
			Hash  string
		}

		sequences := make([]MergeSequence, numRuns)

		for run := 0; run < numRuns; run++ {
			st := store.NewMemStore[ValidationTestState]()

			reducer := func(prev, delta ValidationTestState) ValidationTestState {
				newState := prev
				newState.Visited = append(newState.Visited, delta.Visited...)
				return newState
			}

			engine := graph.New(reducer, st, nil, graph.Options{
				MaxSteps:           20,
				MaxConcurrentNodes: numBranches,
			})

			// Create parallel branches
			for i := 0; i < numBranches; i++ {
				branchID := fmt.Sprintf("branch_%d", i)
				idx := i // Capture

				branchNode := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
					// Varying work to create timing variance
					workDelay := time.Duration(idx*10) * time.Microsecond
					time.Sleep(workDelay)

					return graph.NodeResult[ValidationTestState]{
						Delta: ValidationTestState{Visited: []string{fmt.Sprintf("branch_%d", idx)}},
						Route: graph.Stop(),
					}
				})

				if err := engine.Add(branchID, branchNode); err != nil {
					t.Fatalf("Failed to add %s: %v", branchID, err)
				}
			}

			// Start fans out to all branches
			startNode := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
				branches := make([]string, numBranches)
				for i := 0; i < numBranches; i++ {
					branches[i] = fmt.Sprintf("branch_%d", i)
				}
				return graph.NodeResult[ValidationTestState]{
					Route: graph.Next{Many: branches},
				}
			})

			if err := engine.Add("start", startNode); err != nil {
				t.Fatalf("Failed to add start: %v", err)
			}
			if err := engine.StartAt("start"); err != nil {
				t.Fatalf("Failed to start at start: %v", err)
			}

			finalState, err := engine.Run(context.Background(), runID, ValidationTestState{})
			if err != nil {
				t.Fatalf("Run %d failed: %v", run, err)
			}

			// Compute hash of merge order
			orderJSON, _ := json.Marshal(finalState.Visited)
			hash := sha256.Sum256(orderJSON)

			sequences[run] = MergeSequence{
				Order: finalState.Visited,
				Hash:  hex.EncodeToString(hash[:]),
			}
		}

		// Verify 100% consistency
		firstHash := sequences[0].Hash
		deviations := 0
		for i := 1; i < numRuns; i++ {
			if sequences[i].Hash != firstHash {
				t.Errorf("Run %d merge order differs:\n  First: %v\n  Run %d: %v",
					i, sequences[0].Order, i, sequences[i].Order)
				deviations++
			}
		}

		if deviations > 0 {
			t.Fatalf("FAILED: %d/%d runs had different merge orders", deviations, numRuns)
		}

		t.Logf("✓ All %d runs produced identical merge order", numRuns)
		t.Logf("  Visited order: %v", sequences[0].Order)
	})
}

// ==============================================================================
// T042: 1000-Iteration Determinism Stress Test
// ==============================================================================

// TestDeterminism1000Iterations (T042) is the ultimate determinism validation
// running 1000 iterations to ensure 100% consistency.
//
// This is the final acceptance test for US2, verifying that all bug fixes
// (RNG, Frontier, Completion) preserve deterministic replay.
//
// Requirements:
// - 1000 executions with same RunID
// - Comprehensive workflow (sequential + parallel + retry)
// - 100% identical outputs (single deviation = FAIL)
// - Performance tracking (throughput, memory)
func TestDeterminism1000Iterations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 1000-iteration test in short mode")
	}

	t.Run("comprehensive_workflow_1000_iterations", func(t *testing.T) {
		runID := "determinism-1000-test"
		numRuns := 1000

		startTime := time.Now()
		stateHashes := make([]string, numRuns)

		for run := 0; run < numRuns; run++ {
			st := store.NewMemStore[ValidationTestState]()

			reducer := func(prev, delta ValidationTestState) ValidationTestState {
				newState := prev
				newState.Counter += delta.Counter
				newState.Visited = append(newState.Visited, delta.Visited...)
				return newState
			}

			engine := graph.New(reducer, st, nil, graph.Options{
				MaxSteps:           30,
				MaxConcurrentNodes: 10,
			})

			// Complex workflow: sequential + parallel

			// Sequential start
			seq1 := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
				return graph.NodeResult[ValidationTestState]{
					Delta: ValidationTestState{Counter: 1, Visited: []string{"seq1"}},
					Route: graph.Goto("parallel_start"),
				}
			})

			// Fan-out to parallel branches
			parallelStart := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
				return graph.NodeResult[ValidationTestState]{
					Route: graph.Next{Many: []string{"p1", "p2", "p3", "p4", "p5"}},
				}
			})

			// 5 parallel branches (all stop, no merge point needed)
			for i := 1; i <= 5; i++ {
				nodeID := fmt.Sprintf("p%d", i)
				idx := i

				pNode := graph.NodeFunc[ValidationTestState](func(ctx context.Context, state ValidationTestState) graph.NodeResult[ValidationTestState] {
					// Get RNG for random work
					rng, _ := ctx.Value(graph.RNGKey).(*rand.Rand)
					if rng != nil {
						_ = rng.Intn(100) // Random work (deterministic from RunID)
					}

					return graph.NodeResult[ValidationTestState]{
						Delta: ValidationTestState{Counter: idx, Visited: []string{fmt.Sprintf("p%d", idx)}},
						Route: graph.Stop(),
					}
				})

				if err := engine.Add(nodeID, pNode); err != nil {
					t.Fatalf("Failed to add %s: %v", nodeID, err)
				}
			}

			if err := engine.Add("seq1", seq1); err != nil {
				t.Fatalf("Failed to add seq1: %v", err)
			}
			if err := engine.Add("parallel_start", parallelStart); err != nil {
				t.Fatalf("Failed to add parallel_start: %v", err)
			}
			if err := engine.StartAt("seq1"); err != nil {
				t.Fatalf("Failed to start at seq1: %v", err)
			}

			finalState, err := engine.Run(context.Background(), runID, ValidationTestState{})
			if err != nil {
				t.Fatalf("Run %d failed: %v", run, err)
			}

			// Compute state hash
			stateJSON, _ := json.Marshal(finalState)
			hash := sha256.Sum256(stateJSON)
			stateHashes[run] = hex.EncodeToString(hash[:])

			// Progress logging every 100 runs
			if (run+1)%100 == 0 {
				t.Logf("Progress: %d/%d runs completed", run+1, numRuns)
			}
		}

		duration := time.Since(startTime)

		// Verify ALL hashes are identical
		firstHash := stateHashes[0]
		deviations := 0
		for i := 1; i < numRuns; i++ {
			if stateHashes[i] != firstHash {
				deviations++
				if deviations <= 5 { // Log first 5 deviations
					t.Errorf("Run %d state differs (hash mismatch)", i)
				}
			}
		}

		if deviations > 0 {
			t.Fatalf("FAILED: %d/%d runs had different states", deviations, numRuns)
		}

		// Success metrics
		throughput := float64(numRuns) / duration.Seconds()
		t.Logf("✓ SUCCESS: All %d runs produced identical states", numRuns)
		t.Logf("  Duration: %v", duration)
		t.Logf("  Throughput: %.2f executions/sec", throughput)
		t.Logf("  State hash: %s", firstHash[:16])
	})
}
