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

// ResearchState represents the state of a multi-agent research pipeline
type ResearchState struct {
	Topic            string
	ResearchQuestion string

	// Agent outputs
	HistoricalContext string
	TechnicalAnalysis string
	MarketTrends      string
	ExpertOpinions    string

	// Aggregated results
	Summary         string
	KeyFindings     []string
	Recommendations []string

	// Metadata
	AgentsCompleted   []string
	TotalResearchTime time.Duration
	StartTime         time.Time
}

// Reducer merges research state updates
func reducer(prev, delta ResearchState) ResearchState {
	if delta.Topic != "" {
		prev.Topic = delta.Topic
	}
	if delta.ResearchQuestion != "" {
		prev.ResearchQuestion = delta.ResearchQuestion
	}
	if delta.HistoricalContext != "" {
		prev.HistoricalContext = delta.HistoricalContext
	}
	if delta.TechnicalAnalysis != "" {
		prev.TechnicalAnalysis = delta.TechnicalAnalysis
	}
	if delta.MarketTrends != "" {
		prev.MarketTrends = delta.MarketTrends
	}
	if delta.ExpertOpinions != "" {
		prev.ExpertOpinions = delta.ExpertOpinions
	}
	if delta.Summary != "" {
		prev.Summary = delta.Summary
	}
	if len(delta.KeyFindings) > 0 {
		prev.KeyFindings = append(prev.KeyFindings, delta.KeyFindings...)
	}
	if len(delta.Recommendations) > 0 {
		prev.Recommendations = append(prev.Recommendations, delta.Recommendations...)
	}
	if len(delta.AgentsCompleted) > 0 {
		prev.AgentsCompleted = append(prev.AgentsCompleted, delta.AgentsCompleted...)
	}
	if delta.TotalResearchTime > 0 {
		prev.TotalResearchTime = delta.TotalResearchTime
	}
	if !delta.StartTime.IsZero() {
		prev.StartTime = delta.StartTime
	}
	return prev
}

// MockResearchTool simulates external research tools
type MockResearchTool struct {
	name string
}

func (m *MockResearchTool) Name() string {
	return m.name
}

func (m *MockResearchTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query, _ := input["query"].(string)

	// Simulate research delay
	time.Sleep(100 * time.Millisecond)

	results := map[string]interface{}{
		"source": m.name,
		"query":  query,
		"data":   fmt.Sprintf("Research data from %s about: %s", m.name, query),
	}

	return results, nil
}

func main() {
	fmt.Println("=== Multi-Agent Research Pipeline ===")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("• Multi-agent workflow with specialized researchers")
	fmt.Println("• State aggregation from multiple sources")
	fmt.Println("• Sequential processing of research domains")
	fmt.Println("• Tool integration for data gathering")
	fmt.Println("• Complex workflow with synthesis and reporting")
	fmt.Println()

	// Create research tools
	historicalDB := &MockResearchTool{name: "historical_database"}
	technicalAPI := &MockResearchTool{name: "technical_api"}
	marketData := &MockResearchTool{name: "market_data_service"}
	expertNetwork := &MockResearchTool{name: "expert_network"}

	// Setup workflow
	st := store.NewMemStore[ResearchState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{MaxSteps: 20}
	engine := graph.New(reducer, st, emitter, opts)

	// Node 1: Initialize research
	engine.Add("initialize", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Printf("🔍 Initializing research on: %s\n", state.Topic)
		fmt.Printf("📋 Research question: %s\n", state.ResearchQuestion)
		fmt.Println()

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				StartTime: time.Now(),
			},
			Route: graph.Goto("historical_agent"),
		}
	}))

	// Agent 1: Historical Context Researcher
	engine.Add("historical_agent", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Println("📚 Historical Agent: Researching historical context...")

		// Use research tool
		result, err := historicalDB.Call(ctx, map[string]interface{}{
			"query": fmt.Sprintf("historical context of %s", state.Topic),
		})
		if err != nil {
			return graph.NodeResult[ResearchState]{Err: err}
		}

		context := fmt.Sprintf(
			"Historical Context Analysis:\n"+
				"The topic '%s' has evolved significantly over the past decades. "+
				"Early developments began in the 1990s with foundational research. "+
				"Major breakthroughs occurred in 2010-2015. Recent trends show "+
				"accelerated innovation and widespread adoption.\n"+
				"Source: %s",
			state.Topic,
			result["source"],
		)

		fmt.Println("✅ Historical research complete")

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				HistoricalContext: context,
				AgentsCompleted:   []string{"historical_agent"},
			},
			Route: graph.Goto("technical_agent"),
		}
	}))

	// Agent 2: Technical Analysis Researcher
	engine.Add("technical_agent", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Println("🔧 Technical Agent: Analyzing technical aspects...")

		result, err := technicalAPI.Call(ctx, map[string]interface{}{
			"query": fmt.Sprintf("technical analysis of %s", state.Topic),
		})
		if err != nil {
			return graph.NodeResult[ResearchState]{Err: err}
		}

		analysis := fmt.Sprintf(
			"Technical Analysis:\n"+
				"Core technologies: Advanced algorithms, distributed systems, cloud infrastructure\n"+
				"Performance metrics: 99.9%% uptime, <10ms latency, scalable to millions of users\n"+
				"Technical challenges: Integration complexity, data consistency, security\n"+
				"Architecture: Microservices-based, event-driven, containerized deployment\n"+
				"Source: %s",
			result["source"],
		)

		fmt.Println("✅ Technical analysis complete")

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				TechnicalAnalysis: analysis,
				AgentsCompleted:   []string{"technical_agent"},
			},
			Route: graph.Goto("market_agent"),
		}
	}))

	// Agent 3: Market Trends Researcher
	engine.Add("market_agent", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Println("📈 Market Agent: Analyzing market trends...")

		result, err := marketData.Call(ctx, map[string]interface{}{
			"query": fmt.Sprintf("market trends for %s", state.Topic),
		})
		if err != nil {
			return graph.NodeResult[ResearchState]{Err: err}
		}

		trends := fmt.Sprintf(
			"Market Trends Analysis:\n"+
				"Market size: $50B+ globally, growing at 25%% CAGR\n"+
				"Key players: Leading tech companies, innovative startups, enterprise adopters\n"+
				"Adoption rate: 60%% of Fortune 500 companies actively exploring\n"+
				"Investment trends: $10B+ in venture funding over past 3 years\n"+
				"Regional leaders: North America (40%%), Europe (30%%), Asia-Pacific (25%%)\n"+
				"Source: %s",
			result["source"],
		)

		fmt.Println("✅ Market analysis complete")

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				MarketTrends:    trends,
				AgentsCompleted: []string{"market_agent"},
			},
			Route: graph.Goto("expert_agent"),
		}
	}))

	// Agent 4: Expert Opinions Researcher
	engine.Add("expert_agent", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Println("👥 Expert Agent: Gathering expert opinions...")

		result, err := expertNetwork.Call(ctx, map[string]interface{}{
			"query": fmt.Sprintf("expert opinions on %s", state.Topic),
		})
		if err != nil {
			return graph.NodeResult[ResearchState]{Err: err}
		}

		opinions := fmt.Sprintf(
			"Expert Opinions:\n"+
				"Dr. Sarah Chen (MIT): 'This technology represents a paradigm shift'\n"+
				"Prof. James Wilson (Stanford): 'We're seeing unprecedented innovation rates'\n"+
				"Industry Leader, Tech Corp: 'Early adopters gain significant competitive advantage'\n"+
				"Chief Scientist, Research Lab: 'The next 5 years will be transformative'\n"+
				"Source: %s",
			result["source"],
		)

		fmt.Println("✅ Expert opinions gathered")

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				ExpertOpinions:  opinions,
				AgentsCompleted: []string{"expert_agent"},
			},
			Route: graph.Goto("synthesize"),
		}
	}))

	// Node 5: Synthesize all research (Join point for parallel execution)
	engine.Add("synthesize", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		// Count available research
		count := 0
		if state.HistoricalContext != "" {
			count++
		}
		if state.TechnicalAnalysis != "" {
			count++
		}
		if state.MarketTrends != "" {
			count++
		}
		if state.ExpertOpinions != "" {
			count++
		}

		fmt.Printf("📊 Synthesize check: %d/4 agents complete\n", count)

		if count < 4 {
			// Not all agents done yet, wait
			return graph.NodeResult[ResearchState]{
				Route: graph.Goto("synthesize"),
			}
		}

		// All research complete, proceed with synthesis
		fmt.Println()
		fmt.Println("🔄 Synthesizing research from all agents...")

		// Simulate synthesis time
		time.Sleep(200 * time.Millisecond)

		// Create comprehensive summary
		summary := fmt.Sprintf(
			"Comprehensive Research Summary: %s\n\n"+
				"This multi-agent research analysis combines historical context, "+
				"technical evaluation, market trends, and expert insights to provide "+
				"a holistic view of %s.\n\n"+
				"The research reveals significant growth potential, technical maturity, "+
				"and strong market validation. Expert consensus indicates this is a "+
				"transformative technology with substantial near-term impact.",
			state.Topic,
			state.Topic,
		)

		// Extract key findings
		keyFindings := []string{
			"Strong historical foundation with accelerating recent progress",
			"Technically mature with proven scalability and performance",
			"Rapid market growth with $50B+ global market size",
			"Expert consensus on transformative potential",
			"High adoption rate among leading organizations",
		}

		// Generate recommendations
		recommendations := []string{
			"Immediate action: Begin pilot implementation to gain early-mover advantage",
			"Strategic priority: Allocate resources for skill development and training",
			"Risk mitigation: Establish clear governance and security frameworks",
			"Partnership opportunities: Engage with leading vendors and consultants",
			"Timeline: Aim for production deployment within 12-18 months",
		}

		duration := time.Since(state.StartTime)

		fmt.Println("✅ Synthesis complete")

		return graph.NodeResult[ResearchState]{
			Delta: ResearchState{
				Summary:           summary,
				KeyFindings:       keyFindings,
				Recommendations:   recommendations,
				TotalResearchTime: duration,
			},
			Route: graph.Goto("report"),
		}
	}))

	// Node 6: Generate final report
	engine.Add("report", graph.NodeFunc[ResearchState](func(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
		fmt.Println()
		fmt.Println("📊 Generating final research report...")
		fmt.Println()

		return graph.NodeResult[ResearchState]{
			Route: graph.Stop(),
		}
	}))

	engine.StartAt("initialize")

	// Execute research pipeline
	ctx := context.Background()
	initialState := ResearchState{
		Topic:            "Artificial Intelligence in Healthcare",
		ResearchQuestion: "What is the current state and future potential of AI in healthcare?",
	}

	final, err := engine.Run(ctx, "research-001", initialState)
	if err != nil {
		fmt.Printf("❌ Research pipeline failed: %v\n", err)
		os.Exit(1)
	}

	// Display final report
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                    RESEARCH REPORT")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Topic: %s\n", final.Topic)
	fmt.Printf("Question: %s\n", final.ResearchQuestion)
	fmt.Printf("Research Duration: %v\n", final.TotalResearchTime)
	fmt.Println()

	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("EXECUTIVE SUMMARY")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println(final.Summary)
	fmt.Println()

	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("KEY FINDINGS")
	fmt.Println("───────────────────────────────────────────────────────────")
	for i, finding := range final.KeyFindings {
		fmt.Printf("%d. %s\n", i+1, finding)
	}
	fmt.Println()

	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("DETAILED ANALYSIS")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println()

	fmt.Println("Historical Context:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println(final.HistoricalContext)
	fmt.Println()

	fmt.Println("Technical Analysis:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println(final.TechnicalAnalysis)
	fmt.Println()

	fmt.Println("Market Trends:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println(final.MarketTrends)
	fmt.Println()

	fmt.Println("Expert Opinions:")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println(final.ExpertOpinions)
	fmt.Println()

	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("RECOMMENDATIONS")
	fmt.Println("───────────────────────────────────────────────────────────")
	for i, rec := range final.Recommendations {
		fmt.Printf("%d. %s\n", i+1, rec)
	}
	fmt.Println()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("✅ Research completed with %d agents in %v\n", len(final.AgentsCompleted), final.TotalResearchTime)
	fmt.Println("═══════════════════════════════════════════════════════════")
}
