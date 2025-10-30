// Package main demonstrates an interactive approval workflow using LangGraph-Go.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// ApprovalState represents an approval workflow state
type ApprovalState struct {
	RequestID       string
	RequestType     string
	Requester       string
	Amount          float64
	Description     string
	Status          string
	ApprovalLevel   int
	Approvers       []string
	Comments        []string
	PendingApproval bool
	Approved        bool
	Rejected        bool
	CheckpointID    string
}

func reducer(prev, delta ApprovalState) ApprovalState {
	if delta.Status != "" {
		prev.Status = delta.Status
	}
	if delta.ApprovalLevel > 0 {
		prev.ApprovalLevel = delta.ApprovalLevel
	}
	if len(delta.Approvers) > 0 {
		prev.Approvers = append(prev.Approvers, delta.Approvers...)
	}
	if len(delta.Comments) > 0 {
		prev.Comments = append(prev.Comments, delta.Comments...)
	}
	if delta.PendingApproval {
		prev.PendingApproval = true
	}
	if delta.Approved {
		prev.Approved = true
		prev.PendingApproval = false
	}
	if delta.Rejected {
		prev.Rejected = true
		prev.PendingApproval = false
	}
	if delta.CheckpointID != "" {
		prev.CheckpointID = delta.CheckpointID
	}
	return prev
}

func main() {
	fmt.Println("=== Interactive Approval Workflow with Checkpoints ===")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("• Pause/resume workflow at specific points")
	fmt.Println("• Human-in-the-loop decision making (simulated)")
	fmt.Println("• Checkpoint-based state persistence")
	fmt.Println("• Multi-level approval routing")
	fmt.Println("• Resumable workflow execution after pauses")
	fmt.Println()

	st := store.NewMemStore[ApprovalState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	engine := graph.New(reducer, st, emitter, graph.WithMaxSteps(20))

	// Node 1: Initialize request
	if err := engine.Add("initialize", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		fmt.Printf("📝 Initializing approval request: %s\n", state.RequestID)
		fmt.Printf("   Type: %s | Amount: $%.2f\n", state.RequestType, state.Amount)
		fmt.Printf("   Requester: %s\n", state.Requester)

		// Determine approval level based on amount
		var level int
		if state.Amount < 1000 {
			level = 1 // Manager approval
		} else if state.Amount < 10000 {
			level = 2 // Director approval
		} else {
			level = 3 // VP approval
		}

		fmt.Printf("   Required approval level: %d\n", level)

		return graph.NodeResult[ApprovalState]{
			Delta: ApprovalState{
				ApprovalLevel: level,
				Status:        "pending_approval",
			},
			Route: graph.Goto("request_approval"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add initialize node: %v\n", err)
		os.Exit(1)
	}

	// Node 2: Request approval (pause point)
	if err := engine.Add("request_approval", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		var approverTitle string
		switch state.ApprovalLevel {
		case 1:
			approverTitle = "Manager"
		case 2:
			approverTitle = "Director"
		case 3:
			approverTitle = "VP"
		}

		fmt.Printf("⏸️  Requesting approval from: %s (Level %d)\n", approverTitle, state.ApprovalLevel)
		fmt.Println("   Workflow pausing... saving checkpoint")

		// Save checkpoint for resumption
		checkpointID := fmt.Sprintf("%s-awaiting-approval", state.RequestID)
		err := st.SaveCheckpoint(ctx, checkpointID, state, 2)
		if err != nil {
			fmt.Printf("❌ Failed to save checkpoint: %v\n", err)
		} else {
			fmt.Printf("💾 Checkpoint saved: %s\n", checkpointID)
		}

		return graph.NodeResult[ApprovalState]{
			Delta: ApprovalState{
				PendingApproval: true,
				CheckpointID:    checkpointID,
			},
			Route: graph.Goto("await_decision"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add request_approval node: %v\n", err)
		os.Exit(1)
	}

	// Node 3: Await human decision (simulated)
	if err := engine.Add("await_decision", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		fmt.Println("⏳ Waiting for human decision...")

		// Simulate human review time
		time.Sleep(300 * time.Millisecond)

		// Simulate approval decision (80% approval rate)
		approved := state.Amount < 15000 // Auto-approve under $15k for demo

		if approved {
			fmt.Println("✅ Request APPROVED")
			return graph.NodeResult[ApprovalState]{
				Delta: ApprovalState{
					Approved:  true,
					Status:    "approved",
					Approvers: []string{fmt.Sprintf("Level_%d_Approver", state.ApprovalLevel)},
					Comments:  []string{"Approved - request looks good"},
				},
				Route: graph.Goto("process_approval"),
			}
		}

		fmt.Println("❌ Request REJECTED")
		return graph.NodeResult[ApprovalState]{
			Delta: ApprovalState{
				Rejected: true,
				Status:   "rejected",
				Comments: []string{"Rejected - amount exceeds threshold"},
			},
			Route: graph.Goto("process_rejection"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add await_decision node: %v\n", err)
		os.Exit(1)
	}

	// Node 4: Process approval
	if err := engine.Add("process_approval", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		fmt.Println("🎉 Processing approved request...")
		fmt.Printf("   Approved by: %v\n", state.Approvers)

		// Save completion checkpoint
		checkpointID := fmt.Sprintf("%s-approved", state.RequestID)
		_ = st.SaveCheckpoint(ctx, checkpointID, state, 5)

		return graph.NodeResult[ApprovalState]{
			Route: graph.Goto("complete"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add process_approval node: %v\n", err)
		os.Exit(1)
	}

	// Node 5: Process rejection
	if err := engine.Add("process_rejection", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		fmt.Println("🚫 Processing rejected request...")
		fmt.Printf("   Reason: %v\n", state.Comments)

		// Save rejection checkpoint
		checkpointID := fmt.Sprintf("%s-rejected", state.RequestID)
		_ = st.SaveCheckpoint(ctx, checkpointID, state, 5)

		return graph.NodeResult[ApprovalState]{
			Route: graph.Goto("complete"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add process_rejection node: %v\n", err)
		os.Exit(1)
	}

	// Node 6: Complete
	if err := engine.Add("complete", graph.NodeFunc[ApprovalState](func(ctx context.Context, state ApprovalState) graph.NodeResult[ApprovalState] {
		fmt.Println()
		fmt.Println("📊 Workflow Summary:")
		fmt.Printf("   Request ID: %s\n", state.RequestID)
		fmt.Printf("   Final Status: %s\n", state.Status)
		fmt.Printf("   Approval Level: %d\n", state.ApprovalLevel)
		fmt.Printf("   Total Approvers: %d\n", len(state.Approvers))

		return graph.NodeResult[ApprovalState]{
			Route: graph.Stop(),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add complete node: %v\n", err)
		os.Exit(1)
	}

	if err := engine.StartAt("initialize"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set start node to initialize: %v\n", err)
		os.Exit(1)
	}

	// Simulate multiple approval requests
	requests := []struct {
		id          string
		requestType string
		requester   string
		amount      float64
		description string
	}{
		{"REQ-001", "Equipment Purchase", "Alice", 500.00, "New laptop"},
		{"REQ-002", "Travel Expense", "Bob", 2500.00, "Conference trip"},
		{"REQ-003", "Software License", "Carol", 8000.00, "Enterprise license"},
		{"REQ-004", "Marketing Campaign", "Dave", 20000.00, "Q4 campaign"},
	}

	fmt.Println("Processing approval requests...")
	fmt.Println()

	for _, req := range requests {
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("Request: %s\n", req.id)
		fmt.Println("═══════════════════════════════════════════════════════════")

		ctx := context.Background()
		initialState := ApprovalState{
			RequestID:   req.id,
			RequestType: req.requestType,
			Requester:   req.requester,
			Amount:      req.amount,
			Description: req.description,
		}

		// Execute workflow
		final, err := engine.Run(ctx, req.id, initialState)
		if err != nil {
			fmt.Printf("❌ Workflow failed: %v\n", err)
			continue
		}

		// Demonstrate checkpoint resumption
		if final.CheckpointID != "" {
			fmt.Println()
			fmt.Println("🔄 Demonstrating checkpoint resume capability...")
			fmt.Printf("   Checkpoint: %s\n", final.CheckpointID)

			// Load from checkpoint
			restored, step, err := st.LoadCheckpoint(ctx, final.CheckpointID)
			if err == nil {
				fmt.Printf("✅ Successfully restored state from step %d\n", step)
				fmt.Printf("   Restored status: %s\n", restored.Status)
				fmt.Printf("   Workflow could be resumed from this point\n")
			}
		}

		fmt.Println()
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("✅ All approval workflows completed!")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
