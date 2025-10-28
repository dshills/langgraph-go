package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// ResearchState represents the workflow state for a concurrent research pipeline.
// This demonstrates real-world parallel data gathering where multiple independent
// sources are queried simultaneously, then merged into a unified result.
type ResearchState struct {
	Query           string            `json:"query"`            // User's research query
	Papers          []string          `json:"papers"`           // Academic papers found
	News            []string          `json:"news"`             // News articles found
	SocialMedia     []string          `json:"social_media"`     // Social media posts found
	Patents         []string          `json:"patents"`          // Patents found
	MarketData      map[string]string `json:"market_data"`      // Market analysis data
	Summary         string            `json:"summary"`          // Final synthesis
	ExecutionTime   time.Duration     `json:"execution_time"`   // Total time taken
	ParallelSpeedup float64           `json:"parallel_speedup"` // Speedup factor vs sequential
}

// researchReducer merges state updates from parallel branches deterministically.
//
// Key principles demonstrated:
// - Append-only for slices (order preserved by OrderKey, not arrival time)
// - Last-write-wins for scalar fields
// - Map merging for key-value data
//
// This reducer is commutative for parallel branches because array appends are
// ordered by the deterministic OrderKey, not by completion time.
func researchReducer(prev, delta ResearchState) ResearchState {
	// Scalar field: last write wins
	if delta.Query != "" {
		prev.Query = delta.Query
	}

	// Array fields: append deltas (order determined by OrderKey at merge time)
	prev.Papers = append(prev.Papers, delta.Papers...)
	prev.News = append(prev.News, delta.News...)
	prev.SocialMedia = append(prev.SocialMedia, delta.SocialMedia...)
	prev.Patents = append(prev.Patents, delta.Patents...)

	// Map field: merge keys
	if prev.MarketData == nil {
		prev.MarketData = make(map[string]string)
	}
	for k, v := range delta.MarketData {
		prev.MarketData[k] = v
	}

	// Summary: last write wins (merge node writes this)
	if delta.Summary != "" {
		prev.Summary = delta.Summary
	}

	// Timing fields
	if delta.ExecutionTime > 0 {
		prev.ExecutionTime = delta.ExecutionTime
	}
	if delta.ParallelSpeedup > 0 {
		prev.ParallelSpeedup = delta.ParallelSpeedup
	}

	return prev
}

// FetchPapersNode simulates calling an academic paper API (arXiv, Google Scholar, etc.).
// This demonstrates a recordable side effect - in replay mode, the recorded response
// would be used instead of making the actual API call.
type FetchPapersNode struct{}

func (n *FetchPapersNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("📚 [Papers] Querying academic databases...")

	// Simulate API latency (realistic 800-1200ms range)
	time.Sleep(1 * time.Second)

	// Simulate API response
	papers := []string{
		fmt.Sprintf("Paper: 'Advances in %s' (2024, Nature)", state.Query),
		fmt.Sprintf("Paper: '%s: A Survey' (2023, ACM)", state.Query),
		fmt.Sprintf("Paper: 'Practical Applications of %s' (2024, IEEE)", state.Query),
	}

	fmt.Printf("   ✓ Found %d academic papers\n", len(papers))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{Papers: papers},
		Route: graph.Stop(), // Terminal - no next node
	}
}

// Effects declares this node performs recordable I/O (API calls).
// In replay mode, recorded responses would be used instead of live execution.
func (n *FetchPapersNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true, // I/O can be captured for replay
		RequiresIdempotency: true, // Use idempotency keys for retries
	}
}

// FetchNewsNode simulates calling news APIs (NewsAPI, Google News, etc.).
type FetchNewsNode struct{}

func (n *FetchNewsNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("📰 [News] Querying news sources...")

	// Simulate API latency (realistic range)
	time.Sleep(900 * time.Millisecond)

	news := []string{
		fmt.Sprintf("Breaking: %s market sees 15%% growth", state.Query),
		fmt.Sprintf("Analysis: Future of %s in 2025", state.Query),
		fmt.Sprintf("Opinion: Why %s matters now", state.Query),
	}

	fmt.Printf("   ✓ Found %d news articles\n", len(news))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{News: news},
		Route: graph.Stop(),
	}
}

func (n *FetchNewsNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchSocialNode simulates querying social media APIs (Twitter/X, Reddit, etc.).
type FetchSocialNode struct{}

func (n *FetchSocialNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("💬 [Social] Analyzing social media trends...")

	// Simulate API latency
	time.Sleep(750 * time.Millisecond)

	social := []string{
		fmt.Sprintf("Trending: #%s with 12.5K mentions", strings.ReplaceAll(state.Query, " ", "")),
		fmt.Sprintf("Top post: 'Just implemented %s in production' (2.3K likes)", state.Query),
		fmt.Sprintf("Discussion: r/%s community insights", strings.ToLower(strings.ReplaceAll(state.Query, " ", ""))),
	}

	fmt.Printf("   ✓ Found %d social media posts\n", len(social))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{SocialMedia: social},
		Route: graph.Stop(),
	}
}

func (n *FetchSocialNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchPatentsNode simulates querying patent databases (USPTO, Google Patents, etc.).
type FetchPatentsNode struct{}

func (n *FetchPatentsNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("⚖️  [Patents] Searching patent databases...")

	// Simulate API latency
	time.Sleep(1100 * time.Millisecond)

	patents := []string{
		fmt.Sprintf("US Patent 11,234,567: Method for %s optimization", state.Query),
		fmt.Sprintf("US Patent 11,345,678: System and apparatus for %s", state.Query),
	}

	fmt.Printf("   ✓ Found %d patents\n", len(patents))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{Patents: patents},
		Route: graph.Stop(),
	}
}

func (n *FetchPatentsNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

// FetchMarketDataNode simulates querying financial/market data APIs.
type FetchMarketDataNode struct{}

func (n *FetchMarketDataNode) Run(ctx context.Context, state ResearchState) graph.NodeResult[ResearchState] {
	fmt.Println("📈 [Market] Fetching market intelligence...")

	// Simulate API latency
	time.Sleep(850 * time.Millisecond)

	marketData := map[string]string{
		"market_size":    "$2.4B (2024)",
		"growth_rate":    "23% CAGR",
		"top_competitor": "CompanyX Inc.",
		"funding_raised": "$150M Series C",
		"adoption_rate":  "67% enterprise adoption",
	}

	fmt.Printf("   ✓ Retrieved %d market data points\n", len(marketData))

	return graph.NodeResult[ResearchState]{
		Delta: ResearchState{MarketData: marketData},
		Route: graph.Stop(),
	}
}

func (n *FetchMarketDataNode) Effects() graph.SideEffectPolicy {
	return graph.SideEffectPolicy{
		Recordable:          true,
		RequiresIdempotency: true,
	}
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  LangGraph-Go: Concurrent Workflow Example                    ║")
	fmt.Println("║  Demonstrates parallel node execution with deterministic merge ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ctx := context.Background()

	// Configure concurrent execution with tuned parameters
	opts := graph.Options{
		MaxSteps:            100,
		MaxConcurrentNodes:  runtime.NumCPU(), // Use all CPU cores for I/O-bound work
		QueueDepth:          1024,
		DefaultNodeTimeout:  30 * time.Second,
		RunWallClockBudget:  5 * time.Minute,
		BackpressureTimeout: 30 * time.Second,
	}

	fmt.Printf("⚙️  Configuration:\n")
	fmt.Printf("   - Max concurrent nodes: %d (CPU cores: %d)\n", opts.MaxConcurrentNodes, runtime.NumCPU())
	fmt.Printf("   - Queue depth: %d\n", opts.QueueDepth)
	fmt.Printf("   - Node timeout: %v\n", opts.DefaultNodeTimeout)
	fmt.Println()

	// Create engine components
	memStore := store.NewMemStore[ResearchState]()
	logEmitter := &simpleEmitter{verbose: false} // Quiet mode for cleaner output
	engine := graph.New(researchReducer, memStore, logEmitter, opts)

	// Add fan-out entry node that triggers 5 parallel fetches
	fanout := graph.NodeFunc[ResearchState](func(ctx context.Context, s ResearchState) graph.NodeResult[ResearchState] {
		return graph.NodeResult[ResearchState]{
			Route: graph.Next{
				Many: []string{"fetch_papers", "fetch_news", "fetch_social", "fetch_patents", "fetch_market"},
			},
		}
	})

	// Add all nodes
	if err := engine.Add("fanout", fanout); err != nil {
		log.Fatalf("Failed to add fanout node: %v", err)
	}
	if err := engine.Add("fetch_papers", &FetchPapersNode{}); err != nil {
		log.Fatalf("Failed to add papers node: %v", err)
	}
	if err := engine.Add("fetch_news", &FetchNewsNode{}); err != nil {
		log.Fatalf("Failed to add news node: %v", err)
	}
	if err := engine.Add("fetch_social", &FetchSocialNode{}); err != nil {
		log.Fatalf("Failed to add social node: %v", err)
	}
	if err := engine.Add("fetch_patents", &FetchPatentsNode{}); err != nil {
		log.Fatalf("Failed to add patents node: %v", err)
	}
	if err := engine.Add("fetch_market", &FetchMarketDataNode{}); err != nil {
		log.Fatalf("Failed to add market node: %v", err)
	}

	// Set fan-out starting point: all 5 nodes launch in parallel via Next.Many
	if err := engine.StartAt("fanout"); err != nil {
		log.Fatalf("Failed to set start node: %v", err)
	}

	// Execute workflow
	query := "Large Language Models"
	initialState := ResearchState{Query: query}

	fmt.Printf("🔍 Research Query: \"%s\"\n", query)
	fmt.Println()
	fmt.Println("🚀 Launching 5 parallel data fetching operations...")
	fmt.Println("   (Sequential would take ~4.5 seconds, parallel should take ~1.1 seconds)")
	fmt.Println()

	startTime := time.Now()
	finalState, err := engine.Run(ctx, "research-concurrent-001", initialState)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Fatalf("Workflow execution failed: %v", err)
	}

	// Calculate speedup
	// Sequential time = sum of all node times (1.0s + 0.9s + 0.75s + 1.1s + 0.85s = 4.6s)
	sequentialTime := time.Duration(4600) * time.Millisecond
	parallelSpeedup := sequentialTime.Seconds() / elapsed.Seconds()

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Results                                                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("⏱️  Execution Time:\n")
	fmt.Printf("   - Sequential estimate: %.2f seconds\n", sequentialTime.Seconds())
	fmt.Printf("   - Actual (parallel):   %.2f seconds\n", elapsed.Seconds())
	fmt.Printf("   - Speedup factor:      %.2fx\n", parallelSpeedup)
	fmt.Println()

	if parallelSpeedup > 2.0 {
		fmt.Println("✅ SUCCESS: Achieved significant parallel speedup!")
	} else if parallelSpeedup > 1.2 {
		fmt.Println("⚠️  Partial parallelism achieved (may be limited by CPU cores or system load)")
	} else {
		fmt.Println("❌ Warning: Nodes may have executed sequentially")
	}
	fmt.Println()

	// Display aggregated results
	fmt.Println("📊 Aggregated Research Data:")
	fmt.Println()

	if len(finalState.Papers) > 0 {
		fmt.Printf("📚 Academic Papers (%d):\n", len(finalState.Papers))
		for _, paper := range finalState.Papers {
			fmt.Printf("   • %s\n", paper)
		}
		fmt.Println()
	}

	if len(finalState.News) > 0 {
		fmt.Printf("📰 News Articles (%d):\n", len(finalState.News))
		for _, article := range finalState.News {
			fmt.Printf("   • %s\n", article)
		}
		fmt.Println()
	}

	if len(finalState.SocialMedia) > 0 {
		fmt.Printf("💬 Social Media Posts (%d):\n", len(finalState.SocialMedia))
		for _, post := range finalState.SocialMedia {
			fmt.Printf("   • %s\n", post)
		}
		fmt.Println()
	}

	if len(finalState.Patents) > 0 {
		fmt.Printf("⚖️  Patents (%d):\n", len(finalState.Patents))
		for _, patent := range finalState.Patents {
			fmt.Printf("   • %s\n", patent)
		}
		fmt.Println()
	}

	if len(finalState.MarketData) > 0 {
		fmt.Printf("📈 Market Data (%d points):\n", len(finalState.MarketData))
		// Sort keys for deterministic display
		keys := make([]string, 0, len(finalState.MarketData))
		for k := range finalState.MarketData {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("   • %s: %s\n", k, finalState.MarketData[k])
		}
		fmt.Println()
	}

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Key Concepts Demonstrated                                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("1. ✅ Fan-out parallelism: 5 nodes launched simultaneously")
	fmt.Println("2. ✅ Deterministic merge: Results combined in consistent order")
	fmt.Println("3. ✅ Speedup measurement: Parallel vs sequential execution time")
	fmt.Println("4. ✅ Side effect policies: Nodes declare recordable I/O")
	fmt.Println("5. ✅ Resource control: MaxConcurrentNodes limits parallelism")
	fmt.Println()
	fmt.Println("💡 Try this:")
	fmt.Println("   - Run multiple times: results merge in same order every time")
	fmt.Println("   - Change MaxConcurrentNodes to 2: observe constrained parallelism")
	fmt.Println("   - Add replay mode: reuse recorded API responses (see replay_demo)")
	fmt.Println()
}

// simpleEmitter provides basic event logging with optional verbosity.
type simpleEmitter struct {
	verbose bool
}

func (e *simpleEmitter) Emit(event emit.Event) {
	if !e.verbose {
		return // Silent mode for cleaner demo output
	}
	if event.Msg != "" {
		fmt.Printf("   [Event] Step %d, Node %s: %s\n", event.Step, event.NodeID, event.Msg)
	}
}

func (e *simpleEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		e.Emit(event)
	}
	return nil
}

func (e *simpleEmitter) Flush(ctx context.Context) error {
	return nil // No-op for simple emitter
}
