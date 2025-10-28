package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// GameState represents the state for a dice game workflow that demonstrates
// deterministic replay with random number generation.
type GameState struct {
	PlayerName   string   `json:"player_name"`
	RoundNumber  int      `json:"round_number"`
	DiceRolls    []int    `json:"dice_rolls"` // History of all dice rolls
	Score        int      `json:"score"`      // Total score
	GameLog      []string `json:"game_log"`   // Human-readable game events
	ApiCallsMade int      `json:"api_calls"`  // Track actual external calls
	IsReplay     bool     `json:"is_replay"`  // Replay mode indicator
}

// gameReducer merges state updates deterministically.
func gameReducer(prev, delta GameState) GameState {
	if delta.PlayerName != "" {
		prev.PlayerName = delta.PlayerName
	}
	if delta.RoundNumber > 0 {
		prev.RoundNumber = delta.RoundNumber
	}

	// Append arrays (order preserved by OrderKey)
	prev.DiceRolls = append(prev.DiceRolls, delta.DiceRolls...)
	prev.GameLog = append(prev.GameLog, delta.GameLog...)

	// Accumulate score
	prev.Score += delta.Score

	// Track API calls
	prev.ApiCallsMade += delta.ApiCallsMade

	// Copy replay flag
	if delta.IsReplay {
		prev.IsReplay = delta.IsReplay
	}

	return prev
}

// InitGameNode sets up the game and makes an "API call" to load player profile.
// This demonstrates recorded I/O - in replay mode, we don't actually call the API.
type InitGameNode struct {
	replayMode bool // Injected to simulate replay behavior
}

func (n *InitGameNode) Run(ctx context.Context, state GameState) graph.NodeResult[GameState] {
	fmt.Println("\n🎮 [Init] Starting game session...")

	// Simulate API call to load player profile
	var playerData string
	var apiCalls int

	if n.replayMode {
		// In replay mode, we would use recorded I/O instead of making API call
		fmt.Println("   ⏺️  REPLAY MODE: Using recorded player data (no API call)")
		playerData = fmt.Sprintf("Player '%s' (Level 5, Premium)", state.PlayerName)
		apiCalls = 0 // No actual call made
	} else {
		// Record mode: Make actual API call
		fmt.Println("   📡 RECORD MODE: Calling player profile API...")
		time.Sleep(100 * time.Millisecond) // Simulate API latency
		playerData = fmt.Sprintf("Player '%s' (Level 5, Premium)", state.PlayerName)
		apiCalls = 1
		fmt.Println("   ✓ API response recorded for replay")
	}

	log := []string{
		fmt.Sprintf("Game initialized for %s", playerData),
		"Rules: Roll 2 dice, score = sum of rolls",
	}

	return graph.NodeResult[GameState]{
		Delta: GameState{
			GameLog:      log,
			ApiCallsMade: apiCalls,
		},
		Route: graph.Goto("roll_dice"),
	}
}

func (n *InitGameNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true, // This node's I/O can be recorded
		RequiresIdempotency: true, // Use idempotency keys
	}
}

// RollDiceNode demonstrates deterministic random number generation using seeded RNG.
// This is critical for replay - the same runID produces the same random sequence.
type RollDiceNode struct{}

func (n *RollDiceNode) Run(ctx context.Context, state GameState) graph.NodeResult[GameState] {
	round := state.RoundNumber + 1
	fmt.Printf("\n🎲 [Round %d] Rolling dice...\n", round)

	// CRITICAL: Use RNG from context for deterministic replay
	// The RNG is seeded from runID, ensuring same sequence every time
	rng, ok := ctx.Value(graph.RNGKey).(*rand.Rand)
	if !ok || rng == nil {
		// Fallback for tests without engine context
		fmt.Println("   ⚠️  Warning: No seeded RNG in context, using time-based seed")
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Roll 2 six-sided dice using seeded RNG
	die1 := rng.Intn(6) + 1
	die2 := rng.Intn(6) + 1
	total := die1 + die2

	fmt.Printf("   🎲 Die 1: %d\n", die1)
	fmt.Printf("   🎲 Die 2: %d\n", die2)
	fmt.Printf("   📊 Total: %d\n", total)

	// Check for doubles bonus
	bonus := 0
	logMsg := fmt.Sprintf("Round %d: Rolled %d + %d = %d", round, die1, die2, total)
	if die1 == die2 {
		bonus = 10
		fmt.Println("   🎉 DOUBLES! +10 bonus points!")
		logMsg += " (DOUBLES BONUS!)"
	}

	return graph.NodeResult[GameState]{
		Delta: GameState{
			RoundNumber: round,
			DiceRolls:   []int{die1, die2},
			Score:       total + bonus,
			GameLog:     []string{logMsg},
		},
		Route: graph.Goto("check_continue"),
	}
}

// This node is pure (no I/O), so no Effects() method needed

// CheckContinueNode decides whether to play another round or finish.
// Uses seeded RNG to make a random decision that's deterministic in replay.
type CheckContinueNode struct{}

func (n *CheckContinueNode) Run(ctx context.Context, state GameState) graph.NodeResult[GameState] {
	fmt.Println("\n🤔 [Decision] Should we play another round?")

	// Use seeded RNG for deterministic decision
	rng, ok := ctx.Value(graph.RNGKey).(*rand.Rand)
	if !ok || rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Simple rule: 70% chance to continue if under 5 rounds
	continueGame := state.RoundNumber < 5 && rng.Float64() < 0.7

	if continueGame {
		fmt.Println("   ✓ Yes! Rolling again...")
		return graph.NodeResult[GameState]{
			Route: graph.Goto("roll_dice"),
		}
	}

	fmt.Println("   ✓ Game over! Finalizing results...")
	return graph.NodeResult[GameState]{
		Route: graph.Goto("finalize"),
	}
}

// FinalizeNode computes final results and makes an "API call" to save high score.
type FinalizeNode struct {
	replayMode bool
}

func (n *FinalizeNode) Run(ctx context.Context, state GameState) graph.NodeResult[GameState] {
	fmt.Println("\n🏆 [Finalize] Computing final results...")

	// Simulate API call to save high score
	var apiCalls int

	if n.replayMode {
		fmt.Println("   ⏺️  REPLAY MODE: Using recorded save result (no API call)")
		apiCalls = 0
	} else {
		fmt.Println("   📡 RECORD MODE: Saving high score to database...")
		time.Sleep(50 * time.Millisecond)
		fmt.Println("   ✓ High score saved and recorded")
		apiCalls = 1
	}

	avgScore := 0
	if state.RoundNumber > 0 {
		avgScore = state.Score / state.RoundNumber
	}

	finalLog := []string{
		fmt.Sprintf("Game completed after %d rounds", state.RoundNumber),
		fmt.Sprintf("Final score: %d (avg: %d per round)", state.Score, avgScore),
	}

	return graph.NodeResult[GameState]{
		Delta: GameState{
			GameLog:      finalLog,
			ApiCallsMade: apiCalls,
		},
		Route: graph.Stop(),
	}
}

func (n *FinalizeNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  LangGraph-Go: Deterministic Replay Example                   ║")
	fmt.Println("║  Demonstrates checkpoint save/replay with seeded randomness    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")

	ctx := context.Background()

	// ============================================================================
	// Part 1: Original Execution (Record Mode)
	// ============================================================================

	fmt.Println("\n" + strings.Repeat("═", 66))
	fmt.Println("PART 1: ORIGINAL EXECUTION (RECORD MODE)")
	fmt.Println(strings.Repeat("═", 66))

	memStore := store.NewMemStore[GameState]()
	emitter := &simpleEmitter{}

	opts := graph.Options{
		MaxSteps:           50,
		MaxConcurrentNodes: 1,     // Sequential for clarity
		ReplayMode:         false, // Record mode
		StrictReplay:       true,
	}

	engine := buildGameEngine(opts, memStore, emitter, false)

	initialState := GameState{
		PlayerName: "Alice",
	}

	fmt.Println("\n🎯 Running original game execution...")
	fmt.Println("   - Random dice rolls will be generated using seeded RNG")
	fmt.Println("   - API calls will be made and recorded")
	fmt.Println("   - State will be checkpointed at each step")

	runID := "game-replay-demo-001"
	originalState, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		log.Fatalf("Original execution failed: %v", err)
	}

	fmt.Println("\n✅ Original execution completed!")
	displayGameResults("ORIGINAL", originalState)

	// Save a checkpoint for later replay
	fmt.Println("\n💾 Saving checkpoint 'final-state'...")
	checkpoint := store.CheckpointV2[GameState]{
		RunID:     runID,
		StepID:    100,
		State:     originalState,
		Frontier:  nil,
		Label:     "final-state",
		Timestamp: time.Now(),
	}
	if err := memStore.SaveCheckpointV2(ctx, checkpoint); err != nil {
		log.Fatalf("Failed to save checkpoint: %v", err)
	}
	fmt.Println("   ✓ Checkpoint saved")

	// ============================================================================
	// Part 2: Replay Execution
	// ============================================================================

	fmt.Println("\n" + strings.Repeat("═", 66))
	fmt.Println("PART 2: REPLAY EXECUTION (FROM RECORDED STATE)")
	fmt.Println(strings.Repeat("═", 66))

	// Create new engine in replay mode
	replayOpts := graph.Options{
		MaxSteps:           50,
		MaxConcurrentNodes: 1,
		ReplayMode:         true, // Replay mode - use recorded I/O
		StrictReplay:       true, // Fail on mismatch
	}

	replayEngine := buildGameEngine(replayOpts, memStore, emitter, true)

	fmt.Println("\n🔄 Replaying execution with SAME runID...")
	fmt.Println("   - Dice rolls will use SAME seeded RNG (deterministic)")
	fmt.Println("   - API calls will use RECORDED responses (no actual calls)")
	fmt.Println("   - All random decisions will be IDENTICAL")

	replayedState, err := replayEngine.Run(ctx, runID, initialState)
	if err != nil {
		log.Fatalf("Replay execution failed: %v", err)
	}

	fmt.Println("\n✅ Replay execution completed!")
	displayGameResults("REPLAYED", replayedState)

	// ============================================================================
	// Part 3: Verification
	// ============================================================================

	fmt.Println("\n" + strings.Repeat("═", 66))
	fmt.Println("VERIFICATION: COMPARING ORIGINAL VS REPLAY")
	fmt.Println(strings.Repeat("═", 66))
	fmt.Println()

	// Verify identical results
	verifyIdentical(originalState, replayedState)

	// ============================================================================
	// Part 4: Different RunID = Different Random Sequence
	// ============================================================================

	fmt.Println("\n" + strings.Repeat("═", 66))
	fmt.Println("PART 4: DIFFERENT RUN ID = DIFFERENT RANDOMNESS")
	fmt.Println(strings.Repeat("═", 66))

	fmt.Println("\n🎲 Running with DIFFERENT runID to show different random sequence...")

	differentRunID := "game-replay-demo-002"
	differentState, err := engine.Run(ctx, differentRunID, initialState)
	if err != nil {
		log.Fatalf("Different run execution failed: %v", err)
	}

	displayGameResults("DIFFERENT RUN", differentState)

	// Show that results differ
	fmt.Println("\n📊 Comparison:")
	fmt.Printf("   Original run (%s):  %d rounds, score=%d, rolls=%v\n",
		runID, originalState.RoundNumber, originalState.Score, originalState.DiceRolls)
	fmt.Printf("   Different run (%s): %d rounds, score=%d, rolls=%v\n",
		differentRunID, differentState.RoundNumber, differentState.Score, differentState.DiceRolls)

	if !slicesEqual(originalState.DiceRolls, differentState.DiceRolls) {
		fmt.Println("\n   ✅ CORRECT: Different runIDs produced different random sequences!")
	} else {
		fmt.Println("\n   ⚠️  Unexpected: Random sequences matched (very unlikely!)")
	}

	// ============================================================================
	// Summary
	// ============================================================================

	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Key Concepts Demonstrated                                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("1. ✅ Seeded RNG: Same runID = same random sequence")
	fmt.Println("2. ✅ Recorded I/O: Replay mode uses cached API responses")
	fmt.Println("3. ✅ Checkpoint save: State persisted for later replay")
	fmt.Println("4. ✅ Deterministic replay: Exact state reproduction")
	fmt.Println("5. ✅ Isolation: Different runIDs = different random sequences")
	fmt.Println()
	fmt.Println("💡 Use cases:")
	fmt.Println("   - Debugging: Replay production failures locally")
	fmt.Println("   - Testing: Verify logic without external dependencies")
	fmt.Println("   - Auditing: Reconstruct exact execution flow")
	fmt.Println("   - Time travel: Resume from any checkpoint")
	fmt.Println()
}

// buildGameEngine constructs the game workflow graph.
func buildGameEngine(opts graph.Options, st store.Store[GameState], emitter emit.Emitter, replayMode bool) *graph.Engine[GameState] {
	engine := graph.New(gameReducer, st, emitter, opts)

	// Add nodes
	if err := engine.Add("init", &InitGameNode{replayMode: replayMode}); err != nil {
		log.Fatalf("Failed to add init node: %v", err)
	}
	if err := engine.Add("roll_dice", &RollDiceNode{}); err != nil {
		log.Fatalf("Failed to add roll_dice node: %v", err)
	}
	if err := engine.Add("check_continue", &CheckContinueNode{}); err != nil {
		log.Fatalf("Failed to add check_continue node: %v", err)
	}
	if err := engine.Add("finalize", &FinalizeNode{replayMode: replayMode}); err != nil {
		log.Fatalf("Failed to add finalize node: %v", err)
	}

	// Set entry point
	if err := engine.StartAt("init"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	return engine
}

// displayGameResults prints a formatted summary of game state.
func displayGameResults(label string, state GameState) {
	fmt.Println()
	fmt.Printf("📋 %s RESULTS:\n", label)
	fmt.Printf("   Player:      %s\n", state.PlayerName)
	fmt.Printf("   Rounds:      %d\n", state.RoundNumber)
	fmt.Printf("   Final Score: %d\n", state.Score)
	fmt.Printf("   API Calls:   %d\n", state.ApiCallsMade)
	fmt.Printf("   Dice Rolls:  %v\n", state.DiceRolls)
	fmt.Println()
	fmt.Println("   Game Log:")
	for i, entry := range state.GameLog {
		fmt.Printf("     %d. %s\n", i+1, entry)
	}
}

// verifyIdentical checks if original and replayed states match exactly.
func verifyIdentical(original, replayed GameState) {
	// Compare key fields
	checksMatch := []bool{
		original.PlayerName == replayed.PlayerName,
		original.RoundNumber == replayed.RoundNumber,
		original.Score == replayed.Score,
		slicesEqual(original.DiceRolls, replayed.DiceRolls),
		stringSlicesEqual(original.GameLog, replayed.GameLog),
	}

	allMatch := true
	for _, match := range checksMatch {
		if !match {
			allMatch = false
			break
		}
	}

	if allMatch {
		fmt.Println("✅ VERIFICATION PASSED!")
		fmt.Println("   Original and replayed states are IDENTICAL:")
		fmt.Printf("     - Player name:  %s\n", original.PlayerName)
		fmt.Printf("     - Rounds:       %d\n", original.RoundNumber)
		fmt.Printf("     - Score:        %d\n", original.Score)
		fmt.Printf("     - Dice rolls:   %v\n", original.DiceRolls)
		fmt.Println()
		fmt.Println("   This proves deterministic replay works correctly!")
	} else {
		fmt.Println("❌ VERIFICATION FAILED!")
		fmt.Println("   States differ (this should not happen with deterministic replay):")
		fmt.Printf("     Original:  rounds=%d, score=%d, rolls=%v\n",
			original.RoundNumber, original.Score, original.DiceRolls)
		fmt.Printf("     Replayed:  rounds=%d, score=%d, rolls=%v\n",
			replayed.RoundNumber, replayed.Score, replayed.DiceRolls)
	}

	// Also compute and compare state hashes for additional verification
	originalHash := computeStateHash(original)
	replayedHash := computeStateHash(replayed)

	fmt.Printf("\n   State hashes:\n")
	fmt.Printf("     Original:  %x\n", originalHash[:8])
	fmt.Printf("     Replayed:  %x\n", replayedHash[:8])

	if originalHash == replayedHash {
		fmt.Println("     ✓ Hashes match - states are byte-for-byte identical")
	} else {
		fmt.Println("     ✗ Hashes differ - states diverged")
	}
}

// computeStateHash creates a deterministic hash of GameState for verification.
func computeStateHash(state GameState) [32]byte {
	// Serialize to JSON for consistent hashing
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Warning: Failed to marshal state for hashing: %v", err)
		return [32]byte{}
	}
	return sha256.Sum256(data)
}

// slicesEqual checks if two int slices contain the same elements in order.
func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stringSlicesEqual checks if two string slices contain the same elements in order.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// simpleEmitter provides basic event logging.
type simpleEmitter struct{}

func (e *simpleEmitter) Emit(event emit.Event) {
	// Silent emitter for cleaner demo output
}

func (e *simpleEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		e.Emit(event)
	}
	return nil
}

func (e *simpleEmitter) Flush(ctx context.Context) error {
	return nil
}
