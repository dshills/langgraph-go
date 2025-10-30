// Package main demonstrates usage of the LangGraph-Go framework.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// ApprovalState represents a workflow that requires human approval.
type ApprovalState struct {
	Request          string
	GeneratedOutput  string
	RequiresApproval bool
	Approved         *bool // nil = pending, true/false = decision made
	ApprovalComment  string
	Timestamp        time.Time
	Attempts         int
}

// Reducer merges state updates.
func reducer(prev, delta ApprovalState) ApprovalState {
	if delta.Request != "" {
		prev.Request = delta.Request
	}
	if delta.GeneratedOutput != "" {
		prev.GeneratedOutput = delta.GeneratedOutput
	}
	if delta.RequiresApproval {
		prev.RequiresApproval = delta.RequiresApproval
	}
	if delta.Approved != nil {
		prev.Approved = delta.Approved
	}
	if delta.ApprovalComment != "" {
		prev.ApprovalComment = delta.ApprovalComment
	}
	if !delta.Timestamp.IsZero() {
		prev.Timestamp = delta.Timestamp
	}
	if delta.Attempts > 0 {
		prev.Attempts = delta.Attempts
	}
	return prev
}

// GenerateOutputNode simulates generating output that needs approval.
func GenerateOutputNode(_ context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
	fmt.Printf("\n🤖 Generating output for request: %s\n", s.Request)

	// Simulate output generation.
	output := fmt.Sprintf("Generated response for '%s': This is an automated output that needs review.", s.Request)

	return graph.NodeResult[ApprovalState]{
		Delta: ApprovalState{
			GeneratedOutput:  output,
			RequiresApproval: true,
			Timestamp:        time.Now(),
			Attempts:         s.Attempts + 1,
		},
		Route: graph.Goto("approval-gate"),
	}
}

// ApprovalGateNode pauses execution until approval is received.
func ApprovalGateNode(_ context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
	// Check if approval has been provided.
	if s.Approved == nil {
		fmt.Printf("\n⏸️  Workflow paused - awaiting human approval\n")
		fmt.Printf("Generated Output:\n%s\n\n", s.GeneratedOutput)

		// Pause: Return Stop to halt execution.
		// Workflow will resume when state is updated with approval.
		return graph.NodeResult[ApprovalState]{
			Route: graph.Stop(),
		}
	}

	// Approval decision has been made.
	if *s.Approved {
		fmt.Printf("\n✅ Approved by human reviewer\n")
		if s.ApprovalComment != "" {
			fmt.Printf("Comment: %s\n", s.ApprovalComment)
		}
		return graph.NodeResult[ApprovalState]{
			Route: graph.Goto("finalize"),
		}
	}

	fmt.Printf("\n❌ Rejected by human reviewer\n")
	if s.ApprovalComment != "" {
		fmt.Printf("Reason: %s\n", s.ApprovalComment)
	}

	// Check if we should retry.
	if s.Attempts < 3 {
		fmt.Printf("Regenerating output (attempt %d/3)...\n", s.Attempts+1)
		// Reset approval state and regenerate.
		return graph.NodeResult[ApprovalState]{
			Delta: ApprovalState{
				Approved: nil, // Reset approval status
			},
			Route: graph.Goto("generate"),
		}
	}

	fmt.Printf("Max attempts reached. Workflow cancelled.\n")
	return graph.NodeResult[ApprovalState]{
		Route: graph.Stop(),
	}
}

// FinalizeNode completes the approved workflow.
func FinalizeNode(_ context.Context, s ApprovalState) graph.NodeResult[ApprovalState] {
	fmt.Printf("\n✨ Finalizing approved output...\n")
	fmt.Printf("Output: %s\n", s.GeneratedOutput)
	fmt.Printf("Status: Approved and published\n")

	return graph.NodeResult[ApprovalState]{
		Route: graph.Stop(),
	}
}

// setupEngine creates and configures the workflow engine.
func setupEngine() (*graph.Engine[ApprovalState], store.Store[ApprovalState]) {
	st := store.NewMemStore[ApprovalState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)

	engine := graph.New(
		reducer,
		st,
		emitter,
		graph.WithMaxSteps(20), // Prevent infinite loops
	)

	// Add nodes.
	if err := engine.Add("generate", graph.NodeFunc[ApprovalState](GenerateOutputNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("approval-gate", graph.NodeFunc[ApprovalState](ApprovalGateNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}
	if err := engine.Add("finalize", graph.NodeFunc[ApprovalState](FinalizeNode)); err != nil {
		log.Fatalf("failed to add node: %v", err)
	}

	// Define workflow.
	if err := engine.StartAt("generate"); err != nil {
		log.Fatalf("failed to set start node: %v", err)
	}

	return engine, st
}

// getApprovalFromUser prompts for user input.
func getApprovalFromUser() (bool, string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("\n👤 Approve this output? (y/n): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	approved := response == "y" || response == "yes"

	fmt.Print("Add comment (optional, press Enter to skip): ")
	comment, _ := reader.ReadString('\n')
	comment = strings.TrimSpace(comment)

	return approved, comment
}

// demonstrateApprovalWorkflow shows interactive approval workflow.
func demonstrateApprovalWorkflow() {
	fmt.Println("=" + strings.Repeat("=", 70))
	fmt.Println("Human-in-the-Loop: Approval Workflow Demo")
	fmt.Println("=" + strings.Repeat("=", 70))

	ctx := context.Background()
	engine, st := setupEngine()
	runID := "approval-demo-001"

	// Initial state.
	initialState := ApprovalState{
		Request: "Create a marketing email for new product launch",
	}

	// Start workflow.
	fmt.Println("\n🚀 Starting workflow...")
	final, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Check if paused for approval.
	if final.Approved == nil {
		fmt.Println("\n📋 Workflow paused at approval gate")

		// Get human approval.
		approved, comment := getApprovalFromUser()

		// Load the latest checkpoint.
		// LoadCheckpointV2 expects (runID, stepID) but we need the latest step.
		// For simplicity, we'll load the state directly and create a minimal checkpoint.
		latestState, latestStep, err := st.LoadLatest(ctx, runID)
		if err != nil {
			fmt.Printf("Error loading latest state: %v\n", err)
			return
		}

		// Update state with approval decision.
		latestState.Approved = &approved
		latestState.ApprovalComment = comment

		// Create a checkpoint to resume from.
		checkpoint := store.CheckpointV2[ApprovalState]{
			RunID:  runID,
			StepID: latestStep,
			State:  latestState,
		}

		// Resume workflow.
		fmt.Println("\n▶️  Resuming workflow...")
		final, err = engine.RunWithCheckpoint(ctx, checkpoint)
		if err != nil {
			fmt.Printf("Error resuming: %v\n", err)
			return
		}
	}

	// Show final result.
	fmt.Println("\n" + strings.Repeat("=", 70))
	if final.Approved != nil && *final.Approved {
		fmt.Println("✅ Workflow completed successfully - Output approved and finalized")
	} else if final.Approved != nil && !*final.Approved {
		fmt.Println("❌ Workflow terminated - Output rejected after max attempts")
	} else {
		fmt.Println("⏸️  Workflow paused - Awaiting approval")
	}
	fmt.Println(strings.Repeat("=", 70))
}

// demonstrateAutomaticResume shows resuming paused workflow later.
func demonstrateAutomaticResume() {
	fmt.Println("\n\n" + strings.Repeat("=", 70))
	fmt.Println("Demo: Automatic Resume from Saved Checkpoint")
	fmt.Println(strings.Repeat("=", 70))

	ctx := context.Background()
	engine, st := setupEngine()
	runID := "auto-resume-001"

	// Start workflow.
	fmt.Println("\n🚀 Starting workflow...")
	initialState := ApprovalState{
		Request: "Generate quarterly report summary",
	}

	final, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Workflow paused - simulate saving for later.
	if final.Approved == nil {
		fmt.Println("\n💾 Checkpoint saved. Workflow can be resumed later...")
		fmt.Println("(In production, approval could come from web UI, API, etc.)")

		// Simulate time passing.
		fmt.Println("\n⏰ [Simulating: approval received via external system]")
		time.Sleep(1 * time.Second)

		// Load the latest state.
		latestState, latestStep, err := st.LoadLatest(ctx, runID)
		if err != nil {
			fmt.Printf("Error loading latest state: %v\n", err)
			return
		}

		// Apply approval (simulating external approval system).
		approved := true
		latestState.Approved = &approved
		latestState.ApprovalComment = "Approved via external system"

		// Create checkpoint for resumption.
		checkpoint := store.CheckpointV2[ApprovalState]{
			RunID:  runID,
			StepID: latestStep,
			State:  latestState,
		}

		fmt.Println("\n▶️  Resuming workflow from checkpoint...")
		final, err = engine.RunWithCheckpoint(ctx, checkpoint)
		if err != nil {
			fmt.Printf("Error resuming: %v\n", err)
			return
		}

		fmt.Println("\n✅ Workflow resumed and completed successfully")
	}
}

// demonstrateRejectionAndRetry shows rejection with regeneration.
func demonstrateRejectionAndRetry() {
	fmt.Println("\n\n" + strings.Repeat("=", 70))
	fmt.Println("Demo: Rejection with Automatic Retry")
	fmt.Println(strings.Repeat("=", 70))

	ctx := context.Background()
	engine, st := setupEngine()
	runID := "rejection-demo-001"

	// Start workflow.
	fmt.Println("\n🚀 Starting workflow...")
	initialState := ApprovalState{
		Request: "Draft customer apology email",
	}

	final, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Simulate rejection.
	if final.Approved == nil {
		fmt.Println("\n📋 Simulating rejection by reviewer...")

		// Load latest state.
		latestState, latestStep, err := st.LoadLatest(ctx, runID)
		if err != nil {
			fmt.Printf("Error loading latest state: %v\n", err)
			return
		}

		// Reject with feedback.
		approved := false
		latestState.Approved = &approved
		latestState.ApprovalComment = "Tone is too formal - needs to be more personal"

		// Create checkpoint for resumption.
		checkpoint := store.CheckpointV2[ApprovalState]{
			RunID:  runID,
			StepID: latestStep,
			State:  latestState,
		}

		// Resume - will regenerate.
		fmt.Println("\n▶️  Resuming workflow...")
		final, err = engine.RunWithCheckpoint(ctx, checkpoint)
		if err != nil {
			fmt.Printf("Error resuming: %v\n", err)
			return
		}

		// Now at approval gate again after regeneration.
		if final.Approved == nil {
			fmt.Println("\n📋 New output generated - awaiting approval again...")

			// Load latest state again.
			latestState, latestStep, err = st.LoadLatest(ctx, runID)
			if err != nil {
				fmt.Printf("Error loading latest state: %v\n", err)
				return
			}

			// Approve the revised version.
			approved = true
			latestState.Approved = &approved
			latestState.ApprovalComment = "Much better - approved!"

			checkpoint = store.CheckpointV2[ApprovalState]{
				RunID:  runID,
				StepID: latestStep,
				State:  latestState,
			}

			final, err = engine.RunWithCheckpoint(ctx, checkpoint)
			if err != nil {
				fmt.Printf("Error resuming: %v\n", err)
				return
			}

			fmt.Println("\n✅ Revised output approved and finalized")
		}
	}
}

func main() {
	// Demo 1: Interactive approval workflow.
	demonstrateApprovalWorkflow()

	// Demo 2: Automatic resume from checkpoint.
	demonstrateAutomaticResume()

	// Demo 3: Rejection with retry.
	demonstrateRejectionAndRetry()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("All demos completed!")
	fmt.Println("See docs/human-in-the-loop.md for more patterns and production guidance.")
	fmt.Println(strings.Repeat("=", 70) + "\n")
}
