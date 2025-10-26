package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// ChatState represents customer support conversation state
type ChatState struct {
	ConversationID      string
	UserMessage         string
	Intent              string
	Context             map[string]interface{}
	Response            string
	ConversationHistory []string
	Resolved            bool
	NeedsEscalation     bool
}

func reducer(prev, delta ChatState) ChatState {
	if delta.UserMessage != "" {
		prev.UserMessage = delta.UserMessage
	}
	if delta.Intent != "" {
		prev.Intent = delta.Intent
	}
	if delta.Response != "" {
		prev.Response = delta.Response
		prev.ConversationHistory = append(prev.ConversationHistory, fmt.Sprintf("Bot: %s", delta.Response))
	}
	if delta.Context != nil {
		if prev.Context == nil {
			prev.Context = make(map[string]interface{})
		}
		for k, v := range delta.Context {
			prev.Context[k] = v
		}
	}
	if len(delta.ConversationHistory) > 0 {
		prev.ConversationHistory = append(prev.ConversationHistory, delta.ConversationHistory...)
	}
	if delta.Resolved {
		prev.Resolved = true
	}
	if delta.NeedsEscalation {
		prev.NeedsEscalation = true
	}
	return prev
}

func main() {
	fmt.Println("=== Customer Support Chatbot with Checkpointing ===")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("• Conditional routing based on user intent classification")
	fmt.Println("• Conversation state management and history tracking")
	fmt.Println("• Checkpoint-based conversation persistence")
	fmt.Println("• Knowledge base lookups and context-aware responses")
	fmt.Println("• Escalation handling for complex issues")
	fmt.Println()

	st := store.NewMemStore[ChatState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{MaxSteps: 15}
	engine := graph.New(reducer, st, emitter, opts)

	// Node 1: Classify user intent
	engine.Add("classify", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Printf("🤖 Classifying intent for: '%s'\n", state.UserMessage)

		// Simple intent classification based on keywords
		msg := strings.ToLower(state.UserMessage)
		var intent string

		if strings.Contains(msg, "refund") || strings.Contains(msg, "money back") {
			intent = "refund_request"
		} else if strings.Contains(msg, "shipping") || strings.Contains(msg, "delivery") {
			intent = "shipping_inquiry"
		} else if strings.Contains(msg, "broken") || strings.Contains(msg, "not working") {
			intent = "technical_support"
		} else if strings.Contains(msg, "cancel") {
			intent = "cancellation"
		} else {
			intent = "general_inquiry"
		}

		fmt.Printf("📋 Intent classified as: %s\n", intent)

		// Save checkpoint after classification
		checkpointID := fmt.Sprintf("%s-classified", state.ConversationID)
		err := st.SaveCheckpoint(ctx, checkpointID, state, 1)
		if err == nil {
			fmt.Printf("💾 Checkpoint saved: %s\n", checkpointID)
		}

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Intent:              intent,
				ConversationHistory: []string{fmt.Sprintf("User: %s", state.UserMessage)},
			},
			Route: graph.Goto("route_intent"),
		}
	}))

	// Node 2: Route based on intent
	engine.Add("route_intent", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Printf("🔀 Routing to handler for: %s\n", state.Intent)

		switch state.Intent {
		case "refund_request":
			return graph.NodeResult[ChatState]{Route: graph.Goto("handle_refund")}
		case "shipping_inquiry":
			return graph.NodeResult[ChatState]{Route: graph.Goto("handle_shipping")}
		case "technical_support":
			return graph.NodeResult[ChatState]{Route: graph.Goto("handle_technical")}
		case "cancellation":
			return graph.NodeResult[ChatState]{Route: graph.Goto("handle_cancellation")}
		default:
			return graph.NodeResult[ChatState]{Route: graph.Goto("handle_general")}
		}
	}))

	// Handler: Refund requests
	engine.Add("handle_refund", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("💰 Processing refund request...")

		response := "I understand you'd like a refund. I can help with that. " +
			"Our refund policy allows returns within 30 days of purchase. " +
			"Could you please provide your order number?"

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Response: response,
				Context: map[string]interface{}{
					"awaiting": "order_number",
					"action":   "refund",
				},
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Handler: Shipping inquiries
	engine.Add("handle_shipping", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("📦 Processing shipping inquiry...")

		response := "I can help you track your order! " +
			"Standard shipping takes 3-5 business days, while express shipping arrives in 1-2 days. " +
			"Do you have a tracking number, or would you like me to look up your order?"

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Response: response,
				Context: map[string]interface{}{
					"awaiting": "tracking_or_order",
				},
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Handler: Technical support
	engine.Add("handle_technical", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("🔧 Processing technical support request...")

		// Check if issue is complex enough to escalate
		needsEscalation := strings.Contains(strings.ToLower(state.UserMessage), "broken") ||
			strings.Contains(strings.ToLower(state.UserMessage), "not working")

		var response string
		if needsEscalation {
			response = "I'm sorry to hear you're experiencing technical issues. " +
				"This sounds like it might need specialist attention. " +
				"Let me connect you with our technical support team who can assist you better."
			return graph.NodeResult[ChatState]{
				Delta: ChatState{
					Response:        response,
					NeedsEscalation: true,
				},
				Route: graph.Goto("escalate"),
			}
		}

		response = "I can help troubleshoot that. Have you tried restarting the device? " +
			"Also, please make sure you're using the latest version of our software."

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Response: response,
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Handler: Cancellation
	engine.Add("handle_cancellation", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("❌ Processing cancellation request...")

		response := "I can help you cancel your order. " +
			"If the order hasn't shipped yet, we can cancel it immediately. " +
			"Please provide your order number so I can check the status."

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Response: response,
				Context: map[string]interface{}{
					"awaiting": "order_number",
					"action":   "cancel",
				},
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Handler: General inquiries
	engine.Add("handle_general", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("❓ Processing general inquiry...")

		response := "Thank you for contacting support! " +
			"I'm here to help with orders, shipping, returns, and technical issues. " +
			"Could you please provide more details about what you need assistance with?"

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Response: response,
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Node: Escalate to human agent
	engine.Add("escalate", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("🚨 Escalating to human agent...")

		// Save escalation checkpoint
		checkpointID := fmt.Sprintf("%s-escalated", state.ConversationID)
		_ = st.SaveCheckpoint(ctx, checkpointID, state, 5)

		return graph.NodeResult[ChatState]{
			Delta: ChatState{
				Resolved: false,
			},
			Route: graph.Goto("finalize"),
		}
	}))

	// Node: Finalize conversation
	engine.Add("finalize", graph.NodeFunc[ChatState](func(ctx context.Context, state ChatState) graph.NodeResult[ChatState] {
		fmt.Println("✅ Finalizing conversation...")

		// Save final checkpoint
		checkpointID := fmt.Sprintf("%s-final", state.ConversationID)
		_ = st.SaveCheckpoint(ctx, checkpointID, state, 10)
		fmt.Printf("💾 Final checkpoint saved: %s\n", checkpointID)

		if !state.NeedsEscalation {
			return graph.NodeResult[ChatState]{
				Delta: ChatState{Resolved: true},
				Route: graph.Stop(),
			}
		}

		return graph.NodeResult[ChatState]{
			Route: graph.Stop(),
		}
	}))

	engine.StartAt("classify")

	// Simulate multiple conversations with checkpointing
	conversations := []struct {
		id      string
		message string
	}{
		{"conv-001", "I want a refund for my order"},
		{"conv-002", "Where is my package?"},
		{"conv-003", "The product is broken and not working at all"},
		{"conv-004", "I need to cancel my order"},
	}

	fmt.Println("Processing customer support conversations...")
	fmt.Println()

	for _, conv := range conversations {
		fmt.Println(strings.Repeat("=", 70))
		fmt.Printf("Conversation ID: %s\n", conv.id)
		fmt.Println(strings.Repeat("=", 70))

		ctx := context.Background()
		initialState := ChatState{
			ConversationID: conv.id,
			UserMessage:    conv.message,
			Context:        make(map[string]interface{}),
		}

		final, err := engine.Run(ctx, conv.id, initialState)
		if err != nil {
			fmt.Printf("❌ Conversation failed: %v\n", err)
			continue
		}

		// Display conversation summary
		fmt.Println()
		fmt.Println("───────────────────────────────────────────────────────────")
		fmt.Println("CONVERSATION SUMMARY")
		fmt.Println("───────────────────────────────────────────────────────────")
		fmt.Printf("Intent: %s\n", final.Intent)
		fmt.Printf("Resolved: %v\n", final.Resolved)
		fmt.Printf("Escalated: %v\n", final.NeedsEscalation)
		fmt.Println()
		fmt.Println("Conversation History:")
		for _, msg := range final.ConversationHistory {
			fmt.Printf("  %s\n", msg)
		}
		fmt.Println()

		// Simulate checkpoint resume
		checkpointID := fmt.Sprintf("%s-final", conv.id)
		restored, _, err := st.LoadCheckpoint(ctx, checkpointID)
		if err == nil {
			fmt.Printf("✅ Conversation can be resumed from checkpoint: %s\n", checkpointID)
			fmt.Printf("   Restored state has %d messages in history\n", len(restored.ConversationHistory))
		}
		fmt.Println()

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("✅ All conversations processed successfully!")
	fmt.Println(strings.Repeat("=", 70))
}
