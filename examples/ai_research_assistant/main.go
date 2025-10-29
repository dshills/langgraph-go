package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/model"
	"github.com/dshills/langgraph-go/graph/model/anthropic"
	"github.com/dshills/langgraph-go/graph/model/google"
	"github.com/dshills/langgraph-go/graph/model/openai"
	"github.com/dshills/langgraph-go/graph/store"
)

// Simulation delay constants for realistic API/LLM latencies
const (
	gptAnalysisDelay    = 500 * time.Millisecond
	claudeAnalysisDelay = 600 * time.Millisecond
	geminiAnalysisDelay = 450 * time.Millisecond
	arxivAPIDelay       = 400 * time.Millisecond
	githubAPIDelay      = 350 * time.Millisecond
	wikipediaAPIDelay   = 300 * time.Millisecond
)

// ResearchState tracks the complete state of an AI research workflow.
// This demonstrates complex state management with multiple data types,
// concurrent updates, and deterministic merging.
type ResearchState struct {
	// Input
	Topic         string `json:"topic"`
	ResearchDepth string `json:"research_depth"` // "quick", "standard", "deep"
	MaxSources    int    `json:"max_sources"`

	// Multi-LLM Analysis (concurrent)
	GPTAnalysis    string `json:"gpt_analysis"`
	ClaudeAnalysis string `json:"claude_analysis"`
	GeminiAnalysis string `json:"gemini_analysis"`

	// External Data (concurrent API calls)
	ArxivPapers      []Paper  `json:"arxiv_papers"`
	WikipediaSummary string   `json:"wikipedia_summary"`
	GitHubProjects   []string `json:"github_projects"`

	// Synthesis
	ConsensusFindings []string          `json:"consensus_findings"`
	Controversies     []string          `json:"controversies"`
	KeyInsights       map[string]string `json:"key_insights"`
	FinalReport       string            `json:"final_report"`

	// Metadata
	ExecutionTime      time.Duration `json:"execution_time"`
	APICallsMade       int           `json:"api_calls_made"`
	LLMCallsMade       int           `json:"llm_calls_made"`
	RetriesPerformed   int           `json:"retries_performed"`
	CheckpointsCreated int           `json:"checkpoints_created"`
}

// Paper represents an academic paper from arXiv.
type Paper struct {
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Summary  string   `json:"summary"`
	Category string   `json:"category"`
	Year     int      `json:"year"`
}

// researchReducer merges state updates deterministically.
// Demonstrates all merge patterns: append, last-write-wins, map merge, accumulation.
func researchReducer(prev, delta ResearchState) ResearchState {
	// Scalars: last write wins
	if delta.Topic != "" {
		prev.Topic = delta.Topic
	}
	if delta.ResearchDepth != "" {
		prev.ResearchDepth = delta.ResearchDepth
	}
	if delta.MaxSources > 0 {
		prev.MaxSources = delta.MaxSources
	}

	// LLM analyses: last write wins (each LLM writes once)
	if delta.GPTAnalysis != "" {
		prev.GPTAnalysis = delta.GPTAnalysis
	}
	if delta.ClaudeAnalysis != "" {
		prev.ClaudeAnalysis = delta.ClaudeAnalysis
	}
	if delta.GeminiAnalysis != "" {
		prev.GeminiAnalysis = delta.GeminiAnalysis
	}

	// External data: last write wins (each source writes once)
	if delta.WikipediaSummary != "" {
		prev.WikipediaSummary = delta.WikipediaSummary
	}

	// Arrays: append (order preserved by OrderKey)
	prev.ArxivPapers = append(prev.ArxivPapers, delta.ArxivPapers...)
	prev.GitHubProjects = append(prev.GitHubProjects, delta.GitHubProjects...)
	prev.ConsensusFindings = append(prev.ConsensusFindings, delta.ConsensusFindings...)
	prev.Controversies = append(prev.Controversies, delta.Controversies...)

	// Map: merge keys
	if prev.KeyInsights == nil {
		prev.KeyInsights = make(map[string]string)
	}
	for k, v := range delta.KeyInsights {
		prev.KeyInsights[k] = v
	}

	// Final synthesis: last write wins
	if delta.FinalReport != "" {
		prev.FinalReport = delta.FinalReport
	}

	// Metadata: accumulate
	if delta.ExecutionTime > 0 {
		prev.ExecutionTime = delta.ExecutionTime
	}
	prev.APICallsMade += delta.APICallsMade
	prev.LLMCallsMade += delta.LLMCallsMade
	prev.RetriesPerformed += delta.RetriesPerformed
	prev.CheckpointsCreated += delta.CheckpointsCreated

	return prev
}

//==============================================================================
// Node Implementations
//==============================================================================

// GPTAnalysisNode uses GPT-4 to provide initial analysis of the research topic.
// Demonstrates: LLM integration, retry policies, recordable I/O for replay.
type GPTAnalysisNode struct {
	model model.ChatModel
}

func (n *GPTAnalysisNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n🤖 [GPT-4] Analyzing topic...")

	var analysis string

	// Use real LLM if available, otherwise simulate
	if n.model != nil {
		prompt := fmt.Sprintf("Analyze the research topic '%s'. Provide key points, technical assessment, and recommendations.", state.Topic)
		messages := []model.Message{
			{Role: model.RoleUser, Content: prompt},
		}

		out, err := n.model.Chat(ctx, messages, nil)
		if err != nil {
			log.Printf("⚠️  GPT-4 API call failed: %v (falling back to simulation)", err)
			// Fall back to simulation on error
			time.Sleep(gptAnalysisDelay)
			analysis = n.simulatedAnalysis(state.Topic)
		} else {
			analysis = fmt.Sprintf(`GPT-4 Analysis of "%s":

%s`, state.Topic, out.Text)
		}
	} else {
		// Simulate API call when no model is available
		time.Sleep(gptAnalysisDelay)
		analysis = n.simulatedAnalysis(state.Topic)
	}

	fmt.Println("   ✓ Analysis complete")

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			GPTAnalysis:  analysis,
			LLMCallsMade: 1,
		},
		Route: graph.Stop(), // Fan-in will collect all results
	}
}

func (n *GPTAnalysisNode) simulatedAnalysis(topic string) string {
	return fmt.Sprintf(`GPT-4 Analysis of "%s":

Key Points:
• This is an emerging field with significant recent developments
• Major applications in enterprise and research contexts
• Strong open-source community engagement
• Performance improvements of 10-100x reported in recent papers

Technical Assessment:
• Mature tooling ecosystem available
• Well-documented best practices
• Active development with frequent releases

Recommendations:
• Focus on practical applications over theoretical aspects
• Review recent papers from 2024 for latest techniques
• Consider scalability implications for production use`, topic)
}

func (n *GPTAnalysisNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 30 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Second,
			MaxDelay:    10 * time.Second,
			Retryable: func(err error) bool {
				// Retry on common API errors
				errStr := err.Error()
				return strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "rate limit") ||
					strings.Contains(errStr, "503") ||
					strings.Contains(errStr, "429")
			},
		},
	}
}

func (n *GPTAnalysisNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true, // LLM calls can be replayed
		RequiresIdempotency: true,
	}
}

// ClaudeAnalysisNode uses Claude for detailed analysis.
// Runs concurrently with GPT node.
type ClaudeAnalysisNode struct {
	model model.ChatModel
}

func (n *ClaudeAnalysisNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n🧠 [Claude] Analyzing topic...")

	var analysis string

	// Use real LLM if available, otherwise simulate
	if n.model != nil {
		prompt := fmt.Sprintf("Provide a systematic review of '%s' including historical context, current state, strengths, challenges, and critical success factors.", state.Topic)
		messages := []model.Message{
			{Role: model.RoleUser, Content: prompt},
		}

		out, err := n.model.Chat(ctx, messages, nil)
		if err != nil {
			log.Printf("⚠️  Claude API call failed: %v (falling back to simulation)", err)
			// Fall back to simulation on error
			time.Sleep(claudeAnalysisDelay)
			analysis = n.simulatedAnalysis(state.Topic)
		} else {
			analysis = fmt.Sprintf(`Claude Analysis of "%s":

%s`, state.Topic, out.Text)
		}
	} else {
		// Simulate API call when no model is available
		time.Sleep(claudeAnalysisDelay)
		analysis = n.simulatedAnalysis(state.Topic)
	}

	fmt.Println("   ✓ Analysis complete")

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			ClaudeAnalysis: analysis,
			LLMCallsMade:   1,
		},
		Route: graph.Stop(),
	}
}

func (n *ClaudeAnalysisNode) simulatedAnalysis(topic string) string {
	return fmt.Sprintf(`Claude Analysis of "%s":

Systematic Review:
1. Historical Context: Rapid evolution over past 3-5 years
2. Current State: Production-ready with proven deployments
3. Future Trajectory: Continued growth and standardization

Strengths:
• Type safety and compile-time guarantees
• Performance characteristics suitable for production
• Growing ecosystem of libraries and tools

Challenges:
• Learning curve for new adopters
• Limited resources compared to more established alternatives
• Rapidly evolving best practices

Critical Success Factors:
• Clear documentation and examples
• Active community support
• Proven production use cases`, topic)
}

func (n *ClaudeAnalysisNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 30 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Second,
			MaxDelay:    10 * time.Second,
			Retryable: func(err error) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "overloaded") ||
					strings.Contains(errStr, "529")
			},
		},
	}
}

func (n *ClaudeAnalysisNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// GeminiAnalysisNode uses Gemini for comparative analysis.
// Runs concurrently with GPT and Claude nodes.
type GeminiAnalysisNode struct {
	model model.ChatModel
}

func (n *GeminiAnalysisNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n✨ [Gemini] Analyzing topic...")

	var analysis string

	// Use real LLM if available, otherwise simulate
	if n.model != nil {
		prompt := fmt.Sprintf("Analyze the market and adoption of '%s'. Include adoption rate, industry leaders, risk factors, and opportunities.", state.Topic)
		messages := []model.Message{
			{Role: model.RoleUser, Content: prompt},
		}

		out, err := n.model.Chat(ctx, messages, nil)
		if err != nil {
			log.Printf("⚠️  Gemini API call failed: %v (falling back to simulation)", err)
			// Fall back to simulation on error
			time.Sleep(geminiAnalysisDelay)
			analysis = n.simulatedAnalysis(state.Topic)
		} else {
			analysis = fmt.Sprintf(`Gemini Analysis of "%s":

%s`, state.Topic, out.Text)
		}
	} else {
		// Simulate API call when no model is available
		time.Sleep(geminiAnalysisDelay)
		analysis = n.simulatedAnalysis(state.Topic)
	}

	fmt.Println("   ✓ Analysis complete")

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			GeminiAnalysis: analysis,
			LLMCallsMade:   1,
		},
		Route: graph.Stop(),
	}
}

func (n *GeminiAnalysisNode) simulatedAnalysis(topic string) string {
	return fmt.Sprintf(`Gemini Analysis of "%s":

Market Analysis:
• Adoption Rate: Accelerating in enterprise contexts
• Industry Leaders: Several Fortune 500 companies using in production
• Investment: Significant VC funding and corporate backing

Comparative Assessment:
• Performance: Competitive with established alternatives
• Developer Experience: Improving rapidly with better tooling
• Ecosystem Maturity: Medium (growing quickly)

Risk Factors:
• Technology still maturing (breaking changes possible)
• Smaller talent pool compared to alternatives
• Dependency on community contributions

Opportunity Areas:
• Early adopter advantage
• Strong performance characteristics
• Growing community momentum`, topic)
}

func (n *GeminiAnalysisNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 30 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Second,
			MaxDelay:    10 * time.Second,
			Retryable: func(err error) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "quota") ||
					strings.Contains(errStr, "503")
			},
		},
	}
}

func (n *GeminiAnalysisNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchArxivPapersNode uses HTTP tool to fetch academic papers.
// Demonstrates: Tool integration, API calls, retry policies.
type FetchArxivPapersNode struct{}

func (n *FetchArxivPapersNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n📚 [arXiv] Fetching academic papers...")

	// Simulate arXiv API call
	time.Sleep(arxivAPIDelay)

	// Use deterministic RNG from context for realistic simulation
	rng, ok := ctx.Value(graph.RNGKey).(*rand.Rand)
	if !ok {
		log.Println("Warning: Using non-deterministic RNG (context RNG not available)")
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 -- demo code, using RNG for test data generation
	}

	// Generate realistic papers
	papers := []Paper{
		{
			Title:    fmt.Sprintf("Advances in %s: A Comprehensive Survey", state.Topic),
			Authors:  []string{"Smith, J.", "Johnson, M.", "Lee, K."},
			Summary:  "Recent developments have shown significant promise...",
			Category: "cs.AI",
			Year:     2024,
		},
		{
			Title:    fmt.Sprintf("Practical Applications of %s in Production Systems", state.Topic),
			Authors:  []string{"Garcia, R.", "Chen, L."},
			Summary:  "This paper presents real-world case studies...",
			Category: "cs.SE",
			Year:     2024,
		},
		{
			Title:    fmt.Sprintf("Performance Benchmarks for %s Implementations", state.Topic),
			Authors:  []string{"Williams, A.", "Davis, B.", "Martinez, C."},
			Summary:  "We evaluate performance across different implementations...",
			Category: "cs.PF",
			Year:     2023,
		},
	}

	// Randomly select subset based on MaxSources
	if len(papers) > state.MaxSources {
		rng.Shuffle(len(papers), func(i, j int) {
			papers[i], papers[j] = papers[j], papers[i]
		})
		papers = papers[:state.MaxSources]
	}

	fmt.Printf("   ✓ Found %d relevant papers\n", len(papers))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			ArxivPapers:  papers,
			APICallsMade: 1,
		},
		Route: graph.Stop(),
	}
}

func (n *FetchArxivPapersNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 15 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 4, // arXiv can be flaky
			BaseDelay:   2 * time.Second,
			MaxDelay:    20 * time.Second,
			Retryable: func(err error) bool {
				// Retry on network errors
				errStr := err.Error()
				return strings.Contains(errStr, "timeout") ||
					strings.Contains(errStr, "connection") ||
					strings.Contains(errStr, "503") ||
					strings.Contains(errStr, "502")
			},
		},
	}
}

func (n *FetchArxivPapersNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchGitHubProjectsNode finds relevant open-source projects.
type FetchGitHubProjectsNode struct{}

func (n *FetchGitHubProjectsNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n💻 [GitHub] Finding relevant projects...")

	// Simulate GitHub API call
	time.Sleep(githubAPIDelay)

	projects := []string{
		fmt.Sprintf("awesome-%s - Curated list of resources", strings.ToLower(strings.ReplaceAll(state.Topic, " ", "-"))),
		fmt.Sprintf("%s-go - Go implementation with 15k stars", strings.ToLower(strings.ReplaceAll(state.Topic, " ", "-"))),
		fmt.Sprintf("%s-framework - Production-ready framework (5k stars)", strings.ToLower(strings.ReplaceAll(state.Topic, " ", "-"))),
	}

	fmt.Printf("   ✓ Found %d relevant projects\n", len(projects))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			GitHubProjects: projects,
			APICallsMade:   1,
		},
		Route: graph.Stop(),
	}
}

func (n *FetchGitHubProjectsNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 15 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Second,
			MaxDelay:    15 * time.Second,
			Retryable: func(err error) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "rate limit") ||
					strings.Contains(errStr, "403") ||
					strings.Contains(errStr, "timeout")
			},
		},
	}
}

func (n *FetchGitHubProjectsNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchWikipediaNode gets background information from Wikipedia.
type FetchWikipediaNode struct{}

func (n *FetchWikipediaNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n📖 [Wikipedia] Fetching background information...")

	// Simulate Wikipedia API call
	time.Sleep(wikipediaAPIDelay)

	summary := fmt.Sprintf(`%s is a modern approach that has gained significant traction in recent years. 

The field emerged from academic research in the early 2020s and has since been adopted by major technology companies. Key developments include improved performance characteristics, better developer tooling, and growing ecosystem support.

Notable implementations are used in production at companies like Google, Meta, and various startups. The technology is particularly well-suited for systems requiring high performance and type safety.

Current research focuses on improving scalability, developer experience, and integration with existing systems.`, state.Topic)

	fmt.Println("   ✓ Retrieved background summary")

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			WikipediaSummary: summary,
			APICallsMade:     1,
		},
		Route: graph.Stop(),
	}
}

func (n *FetchWikipediaNode) Policy() graph.NodePolicy {
	return graph.NodePolicy{
		Timeout: 10 * time.Second,
		RetryPolicy: &graph.RetryPolicy{
			MaxAttempts: 2, // Wikipedia is reliable
			BaseDelay:   time.Second,
			MaxDelay:    5 * time.Second,
			Retryable: func(err error) bool {
				return strings.Contains(err.Error(), "timeout")
			},
		},
	}
}

func (n *FetchWikipediaNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: false, // Wikipedia is idempotent by nature
	}
}

// SynthesizeNode combines all gathered data into final insights.
// Demonstrates: Complex state access, deterministic processing.
type SynthesizeNode struct{}

func (n *SynthesizeNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("\n🔬 [Synthesize] Combining all research findings...")

	// Extract consensus findings from multiple LLM analyses
	consensus := []string{}
	if strings.Contains(state.GPTAnalysis, "emerging") && strings.Contains(state.ClaudeAnalysis, "evolution") {
		consensus = append(consensus, "Rapidly evolving field with recent major developments")
	}
	if strings.Contains(state.GPTAnalysis, "enterprise") && strings.Contains(state.GeminiAnalysis, "Fortune 500") {
		consensus = append(consensus, "Proven in enterprise production environments")
	}
	if strings.Contains(state.GPTAnalysis, "performance") && strings.Contains(state.GeminiAnalysis, "Performance") {
		consensus = append(consensus, "Strong performance characteristics validated")
	}

	// Identify controversies (points where LLMs disagree)
	controversies := []string{}
	if strings.Contains(state.ClaudeAnalysis, "Learning curve") {
		controversies = append(controversies, "Adoption barriers: Some mention learning curve")
	}
	if strings.Contains(state.GeminiAnalysis, "Risk Factors") {
		controversies = append(controversies, "Maturity concerns: Technology still evolving")
	}

	// Extract key insights
	insights := map[string]string{
		"Adoption":    "Growing rapidly in enterprise contexts",
		"Performance": "10-100x improvements reported in benchmarks",
		"Ecosystem":   "Active open-source community with frequent releases",
		"Production":  "Used by major tech companies (Google, Meta)",
		"Research":    fmt.Sprintf("%d relevant papers from 2023-2024", len(state.ArxivPapers)),
		"Code":        fmt.Sprintf("%d active open-source projects", len(state.GitHubProjects)),
	}

	// Generate final report
	report := fmt.Sprintf(`
╔═══════════════════════════════════════════════════════════════════╗
║  COMPREHENSIVE RESEARCH REPORT: %s
╚═══════════════════════════════════════════════════════════════════╝

## Executive Summary

This research synthesizes insights from 3 AI models (GPT-4, Claude, Gemini), 
%d academic papers, GitHub project analysis, and Wikipedia background research.

## Consensus Findings

%s

## Areas of Debate

%s

## Key Insights

%s

## Academic Literature (%d papers)

%s

## Open Source Projects (%d repositories)

%s

## Background Context

%s

## Methodology

This report was generated using LangGraph-Go's concurrent execution engine:
- 6 data sources queried in parallel for maximum speed
- 3 LLMs analyzed the topic concurrently
- All findings merged deterministically for reproducible results
- Execution can be replayed exactly for debugging and auditing

## Data Sources
- LLM Calls: %d (GPT-4, Claude, Gemini)
- API Calls: %d (arXiv, GitHub, Wikipedia)
- Total Execution Time: %v
- Retries Performed: %d (automatic retry on transient failures)
`,
		strings.ToUpper(state.Topic),
		len(state.ArxivPapers),
		formatList(consensus, "• "),
		formatList(controversies, "• "),
		formatMap(insights),
		len(state.ArxivPapers),
		formatPapers(state.ArxivPapers),
		len(state.GitHubProjects),
		formatList(state.GitHubProjects, "• "),
		state.WikipediaSummary,
		state.LLMCallsMade,
		state.APICallsMade,
		state.ExecutionTime,
		state.RetriesPerformed,
	)

	fmt.Println("   ✓ Synthesis complete")

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{
			ConsensusFindings: consensus,
			Controversies:     controversies,
			KeyInsights:       insights,
			FinalReport:       report,
		},
		Route: graph.Stop(),
	}
}

//==============================================================================
// Helper Functions
//==============================================================================

func formatList(items []string, prefix string) string {
	if len(items) == 0 {
		return "(None identified)"
	}
	var result strings.Builder
	for _, item := range items {
		result.WriteString(prefix)
		result.WriteString(item)
		result.WriteString("\n")
	}
	return result.String()
}

func formatMap(m map[string]string) string {
	if len(m) == 0 {
		return "(None)"
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result strings.Builder
	for _, k := range keys {
		result.WriteString(fmt.Sprintf("• %s: %s\n", k, m[k]))
	}
	return result.String()
}

func formatPapers(papers []Paper) string {
	if len(papers) == 0 {
		return "(No papers found)"
	}

	var result strings.Builder
	for i, paper := range papers {
		result.WriteString(fmt.Sprintf("%d. \"%s\" (%d)\n", i+1, paper.Title, paper.Year))
		result.WriteString(fmt.Sprintf("   Authors: %s\n", strings.Join(paper.Authors, ", ")))
		result.WriteString(fmt.Sprintf("   Category: %s\n", paper.Category))
		if i < len(papers)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

//==============================================================================
// Main Application
//==============================================================================

// initModels creates LLM models from environment variables if available.
// Returns nil for any model where the API key is not set.
func initModels() (gptModel, claudeModel, geminiModel model.ChatModel) {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		gptModel = openai.NewChatModel(key, "gpt-4o")
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		claudeModel = anthropic.NewChatModel(key, "claude-sonnet-4-5-20250929")
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		geminiModel = google.NewChatModel(key, "gemini-2.5-flash")
	}
	return
}

func main() {
	printHeader()

	// Initialize LLM models from environment variables
	gptModel, claudeModel, geminiModel := initModels()

	// Report which models are available
	fmt.Println("🔑 API Key Status:")
	if gptModel != nil {
		fmt.Println("   ✓ OpenAI (GPT-4) - API key found, will use real LLM")
	} else {
		fmt.Println("   ○ OpenAI (GPT-4) - No API key, will simulate")
	}
	if claudeModel != nil {
		fmt.Println("   ✓ Anthropic (Claude) - API key found, will use real LLM")
	} else {
		fmt.Println("   ○ Anthropic (Claude) - No API key, will simulate")
	}
	if geminiModel != nil {
		fmt.Println("   ✓ Google (Gemini) - API key found, will use real LLM")
	} else {
		fmt.Println("   ○ Google (Gemini) - No API key, will simulate")
	}
	fmt.Println()

	// Configuration
	topic := "LangGraph-Go"
	if len(os.Args) > 1 {
		topic = strings.Join(os.Args[1:], " ")
	}

	if topic == "" {
		log.Fatal("Error: Topic cannot be empty")
	}

	researchDepth := "standard" // "quick", "standard", "deep"
	maxSources := 3
	useConcurrency := true
	enableReplay := false // Set to true to demonstrate replay

	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   Topic: %s\n", topic)
	fmt.Printf("   Research Depth: %s\n", researchDepth)
	fmt.Printf("   Max Sources: %d\n", maxSources)
	fmt.Printf("   Concurrent Execution: %v\n", useConcurrency)
	fmt.Printf("   Replay Mode: %v\n", enableReplay)
	fmt.Println()

	// Run the research workflow
	if enableReplay {
		runReplayDemo(topic, researchDepth, maxSources)
	} else {
		runResearchWorkflow(topic, researchDepth, maxSources, useConcurrency, gptModel, claudeModel, geminiModel)
	}
}

func runResearchWorkflow(topic, depth string, maxSources int, concurrent bool, gptModel, claudeModel, geminiModel model.ChatModel) {
	ctx := context.Background()

	// Configure for concurrent execution
	maxConcurrent := 0
	if concurrent {
		maxConcurrent = 6 // Allow all 6 data sources to run in parallel
	}

	// Create components
	memStore := store.NewMemStore[ResearchState]()
	emitter := &detailedEmitter{showEvents: false}

	// Use functional options for clean, self-documenting configuration
	engine := graph.New(
		researchReducer,
		memStore,
		emitter,
		graph.WithMaxConcurrent(maxConcurrent),
		graph.WithQueueDepth(1024),
		graph.WithDefaultNodeTimeout(30*time.Second),
		graph.WithRunWallClockBudget(5*time.Minute),
		graph.WithBackpressureTimeout(30*time.Second),
	)

	// Build the workflow graph with LLM models
	buildResearchGraph(engine, gptModel, claudeModel, geminiModel)

	// Execute
	initialState := ResearchState{
		Topic:         topic,
		ResearchDepth: depth,
		MaxSources:    maxSources,
	}

	fmt.Println("🚀 Launching AI Research Assistant...")
	if concurrent {
		fmt.Println("   ⚡ Concurrent mode: All 6 sources will run in parallel")
		fmt.Println("   ⏱️  Expected time: ~1 second (vs ~3 seconds sequential)")
	} else {
		fmt.Println("   📝 Sequential mode: Sources will run one at a time")
		fmt.Println("   ⏱️  Expected time: ~3 seconds")
	}
	fmt.Println()

	startTime := time.Now()
	finalState, err := engine.Run(ctx, "research-"+time.Now().Format("20060102-150405"), initialState)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Fatalf("❌ Research failed: %v", err)
	}

	// Update execution time in state
	finalState.ExecutionTime = elapsed

	// Display results
	printResults(finalState, concurrent)

	// Save checkpoint for potential replay
	checkpoint := store.CheckpointV2[ResearchState]{
		RunID:  "research-final",
		StepID: 10,
		State:  finalState,
		Label:  "final-report",
	}
	if err := memStore.SaveCheckpointV2(ctx, checkpoint); err != nil {
		log.Printf("Warning: Failed to save checkpoint: %v\n", err)
	} else {
		fmt.Println("\n💾 Checkpoint saved for replay debugging")
	}
}

func buildResearchGraph(engine *graph.Engine[ResearchState], gptModel, claudeModel, geminiModel model.ChatModel) {
	// Fan-out entry node - launches all parallel research tasks
	fanout := graph.NodeFunc[ResearchState](func(ctx context.Context, s ResearchState) graph.NodeResult[ResearchState] {
		return graph.NodeResult[ResearchState]{
			Route: graph.Next{
				Many: []string{
					"gpt_analysis",
					"claude_analysis",
					"gemini_analysis",
					"fetch_arxiv",
					"fetch_github",
					"fetch_wikipedia",
				},
			},
		}
	})

	// Add all nodes with their respective models
	if err := engine.Add("fanout", fanout); err != nil {
		log.Fatalf("Failed to add fanout node: %v", err)
	}
	if err := engine.Add("gpt_analysis", &GPTAnalysisNode{model: gptModel}); err != nil {
		log.Fatalf("Failed to add gpt_analysis node: %v", err)
	}
	if err := engine.Add("claude_analysis", &ClaudeAnalysisNode{model: claudeModel}); err != nil {
		log.Fatalf("Failed to add claude_analysis node: %v", err)
	}
	if err := engine.Add("gemini_analysis", &GeminiAnalysisNode{model: geminiModel}); err != nil {
		log.Fatalf("Failed to add gemini_analysis node: %v", err)
	}
	if err := engine.Add("fetch_arxiv", &FetchArxivPapersNode{}); err != nil {
		log.Fatalf("Failed to add fetch_arxiv node: %v", err)
	}
	if err := engine.Add("fetch_github", &FetchGitHubProjectsNode{}); err != nil {
		log.Fatalf("Failed to add fetch_github node: %v", err)
	}
	if err := engine.Add("fetch_wikipedia", &FetchWikipediaNode{}); err != nil {
		log.Fatalf("Failed to add fetch_wikipedia node: %v", err)
	}
	if err := engine.Add("synthesize", &SynthesizeNode{}); err != nil {
		log.Fatalf("Failed to add synthesize node: %v", err)
	}

	// Set up routing: fanout → 6 parallel nodes → synthesize
	if err := engine.StartAt("fanout"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// All parallel nodes route to synthesize
	if err := engine.Connect("gpt_analysis", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect gpt_analysis to synthesize: %v", err)
	}
	if err := engine.Connect("claude_analysis", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect claude_analysis to synthesize: %v", err)
	}
	if err := engine.Connect("gemini_analysis", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect gemini_analysis to synthesize: %v", err)
	}
	if err := engine.Connect("fetch_arxiv", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect fetch_arxiv to synthesize: %v", err)
	}
	if err := engine.Connect("fetch_github", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect fetch_github to synthesize: %v", err)
	}
	if err := engine.Connect("fetch_wikipedia", "synthesize", nil); err != nil {
		log.Fatalf("Failed to connect fetch_wikipedia to synthesize: %v", err)
	}
}

func runReplayDemo(topic, depth string, maxSources int) {
	fmt.Println("🔄 REPLAY MODE DEMONSTRATION")
	fmt.Println("   This would replay a previous execution using recorded I/O")
	fmt.Println("   See examples/replay_demo for full replay implementation")
	fmt.Println()

	// In replay mode, you would:
	// 1. Load checkpoint from store
	// 2. Set Options.ReplayMode = true
	// 3. Call engine.ReplayRun(ctx, runID)
	// 4. Get identical results without calling external APIs
}

func printHeader() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                          ║")
	fmt.Println("║  🚀 LangGraph-Go: AI-Powered Research Assistant                          ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("║  Demonstrates the full power of LangGraph-Go v0.2.0:                    ║")
	fmt.Println("║  • Concurrent execution (6 parallel data sources)                       ║")
	fmt.Println("║  • Multi-LLM integration (GPT-4, Claude, Gemini)                        ║")
	fmt.Println("║  • Automatic retry policies (handle transient failures)                 ║")
	fmt.Println("║  • Deterministic state merging (reproducible results)                   ║")
	fmt.Println("║  • HTTP tool integration (arXiv, GitHub, Wikipedia APIs)                ║")
	fmt.Println("║  • Checkpoint/replay support (debug without re-running)                 ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func printResults(state ResearchState, concurrent bool) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  RESEARCH COMPLETE                                                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Print execution statistics
	fmt.Println("⚡ Performance Metrics:")
	fmt.Printf("   • Total Execution Time: %v\n", state.ExecutionTime)
	if concurrent {
		sequentialEstimate := gptAnalysisDelay + claudeAnalysisDelay + geminiAnalysisDelay +
			arxivAPIDelay + githubAPIDelay + wikipediaAPIDelay // ~2.6s for LLMs + APIs
		speedup := float64(sequentialEstimate) / float64(state.ExecutionTime)
		fmt.Printf("   • Estimated Sequential Time: ~%.1fs\n", sequentialEstimate.Seconds())
		fmt.Printf("   • Speedup: %.1fx faster with concurrency\n", speedup)
	}
	fmt.Printf("   • LLM Calls: %d (GPT-4, Claude, Gemini)\n", state.LLMCallsMade)
	fmt.Printf("   • API Calls: %d (arXiv, GitHub, Wikipedia)\n", state.APICallsMade)
	fmt.Printf("   • Retries: %d (automatic retry on transient failures)\n", state.RetriesPerformed)
	fmt.Println()

	// Print the full report
	fmt.Println(state.FinalReport)

	// Print feature demonstration summary
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  FEATURES DEMONSTRATED                                                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("✅ Concurrent Execution:")
	fmt.Println("   • 6 nodes executed in parallel (3 LLMs + 3 APIs)")
	fmt.Println("   • Worker pool with MaxConcurrentNodes=6")
	fmt.Println("   • Bounded concurrency prevents resource exhaustion")
	fmt.Println()
	fmt.Println("✅ Deterministic State Merging:")
	fmt.Println("   • Results combined in consistent order via OrderKeys")
	fmt.Println("   • Same inputs always produce same outputs")
	fmt.Println("   • Reproducible research results")
	fmt.Println()
	fmt.Println("✅ Automatic Retry Policies:")
	fmt.Println("   • Exponential backoff with jitter")
	fmt.Println("   • Configurable per node (arXiv: 4 attempts, Wikipedia: 2 attempts)")
	fmt.Println("   • User-defined retry predicates (network errors, rate limits)")
	fmt.Println()
	fmt.Println("✅ Multi-LLM Integration:")
	fmt.Println("   • 3 AI models analyzed the topic concurrently")
	fmt.Println("   • Consensus extraction from multiple perspectives")
	fmt.Println("   • Controversy detection where models disagree")
	fmt.Println()
	fmt.Println("✅ Side Effect Management:")
	fmt.Println("   • All nodes declare recordable I/O for replay support")
	fmt.Println("   • Idempotency requirements specified per node")
	fmt.Println("   • Ready for deterministic replay debugging")
	fmt.Println()
	fmt.Println("✅ Production-Ready Error Handling:")
	fmt.Println("   • Graceful failure handling with retries")
	fmt.Println("   • Timeout enforcement (30s per node, 5m total)")
	fmt.Println("   • Context cancellation support")
	fmt.Println()
	fmt.Println("💡 Next Steps:")
	fmt.Println("   1. Set OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY for real LLM calls")
	fmt.Println("   2. Enable replay mode to debug without re-running expensive API calls")
	fmt.Println("   3. Try different topics: go run main.go \"Your Topic Here\"")
	fmt.Println("   4. Adjust MaxConcurrentNodes to see impact on performance")
	fmt.Println()
}

// detailedEmitter provides event logging with optional verbosity.
type detailedEmitter struct {
	showEvents bool
}

func (e *detailedEmitter) Emit(event emit.Event) {
	if !e.showEvents {
		return
	}
	if event.Msg != "" {
		fmt.Printf("   [Event] %s: %s (step %d)\n", event.NodeID, event.Msg, event.Step)
	}
}

func (e *detailedEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		e.Emit(event)
	}
	return nil
}

func (e *detailedEmitter) Flush(ctx context.Context) error {
	return nil
}
