// Package main demonstrates usage of the LangGraph-Go framework.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// State represents the workflow state.
type State struct {
	UserQuery    string
	Location     string
	WeatherData  map[string]interface{}
	NewsArticles []string
	Summary      string
}

// Reducer merges state updates.
func reducer(prev, delta State) State {
	if delta.UserQuery != "" {
		prev.UserQuery = delta.UserQuery
	}
	if delta.Location != "" {
		prev.Location = delta.Location
	}
	if delta.WeatherData != nil {
		prev.WeatherData = delta.WeatherData
	}
	if len(delta.NewsArticles) > 0 {
		prev.NewsArticles = append(prev.NewsArticles, delta.NewsArticles...)
	}
	if delta.Summary != "" {
		prev.Summary = delta.Summary
	}
	return prev
}

// DemoWeatherTool is a demonstration tool that simulates a weather API.
type DemoWeatherTool struct{}

func (d *DemoWeatherTool) Name() string {
	return "get_weather"
}

func (d *DemoWeatherTool) Call(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	location, ok := input["location"].(string)
	if !ok || location == "" {
		return nil, fmt.Errorf("location parameter required (string)")
	}

	// Simulate API call with mock data.
	weatherData := map[string]interface{}{
		"location":    location,
		"temperature": 72,
		"conditions":  "sunny",
		"humidity":    65,
		"wind_speed":  8,
	}

	return weatherData, nil
}

// DemoNewsTool is a demonstration tool that simulates a news API.
type DemoNewsTool struct{}

func (d *DemoNewsTool) Name() string {
	return "get_news"
}

func (d *DemoNewsTool) Call(_ context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	location, ok := input["location"].(string)
	if !ok || location == "" {
		return nil, fmt.Errorf("location parameter required (string)")
	}

	articles := []string{
		fmt.Sprintf("Breaking: %s city council approves new budget", location),
		fmt.Sprintf("Local %s startup raises $10M in Series A funding", location),
		fmt.Sprintf("Tech conference coming to %s next month", location),
		fmt.Sprintf("%s weather forecast: sunny week ahead", location),
	}

	return map[string]interface{}{
		"location": location,
		"articles": articles,
		"count":    len(articles),
	}, nil
}

func main() {
	fmt.Println("=== LangGraph-Go Tool Invocation Example ===")
	fmt.Println()

	// Create demo tools (custom implementations).
	weatherTool := &DemoWeatherTool{}
	newsTool := &DemoNewsTool{}

	// Node 1: Extract location from query.
	parseNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		fmt.Printf("📝 Parsing query: %q\n", state.UserQuery)

		// Simplified demo: Extract location from query.
		// Production implementation should use NLP or regex.
		location := "San Francisco" // Default
		if state.UserQuery != "" {
			// Simplified extraction - always returns New York for demo.
			location = "New York"
		}

		fmt.Printf("📍 Extracted location: %s\n\n", location)

		return graph.NodeResult[State]{
			Delta: State{Location: location},
			Route: graph.Goto("fetch_weather"),
		}
	})

	// Node 2: Fetch weather data using custom DemoWeatherTool.
	fetchWeatherNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		fmt.Printf("🌤️  Fetching weather for %s...\n", state.Location)

		// Call weather tool.
		result, err := weatherTool.Call(ctx, map[string]interface{}{
			"location": state.Location,
		})
		if err != nil {
			fmt.Printf("❌ Weather tool error: %v\n", err)
			return graph.NodeResult[State]{Err: err}
		}

		fmt.Printf("✅ Weather data received:\n")
		fmt.Printf("   Temperature: %v°F\n", result["temperature"])
		fmt.Printf("   Conditions: %v\n", result["conditions"])
		fmt.Printf("   Humidity: %v%%\n\n", result["humidity"])

		return graph.NodeResult[State]{
			Delta: State{WeatherData: result},
			Route: graph.Goto("fetch_news"),
		}
	})

	// Node 3: Fetch news articles using custom DemoNewsTool.
	fetchNewsNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		fmt.Printf("📰 Fetching news for %s...\n", state.Location)

		// Call news tool.
		result, err := newsTool.Call(ctx, map[string]interface{}{
			"location": state.Location,
		})
		if err != nil {
			fmt.Printf("❌ News tool error: %v\n", err)
			return graph.NodeResult[State]{Err: err}
		}

		articles, ok := result["articles"].([]string)
		if !ok {
			err := fmt.Errorf("invalid articles type from news tool")
			fmt.Printf("❌ %v\n", err)
			return graph.NodeResult[State]{Err: err}
		}

		fmt.Printf("✅ Found %d news articles\n\n", len(articles))

		return graph.NodeResult[State]{
			Delta: State{NewsArticles: articles},
			Route: graph.Goto("summarize"),
		}
	})

	// Node 4: Summarize results.
	summarizeNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
		fmt.Println("📊 Generating summary...")

		// Extract weather data with proper type checking.
		weatherTemp := 0
		weatherCond := "unknown"
		if state.WeatherData != nil {
			if temp, ok := state.WeatherData["temperature"].(int); ok {
				weatherTemp = temp
			}
			if cond, ok := state.WeatherData["conditions"].(string); ok {
				weatherCond = cond
			}
		}

		summary := fmt.Sprintf(
			"Weather and News for %s:\n"+
				"- Current temperature: %d°F\n"+
				"- Conditions: %s\n"+
				"- Found %d news articles\n"+
				"- All tools executed successfully",
			state.Location,
			weatherTemp,
			weatherCond,
			len(state.NewsArticles),
		)

		fmt.Printf("✅ Summary generated\n\n")

		return graph.NodeResult[State]{
			Delta: State{Summary: summary},
			Route: graph.Stop(),
		}
	})

	// Build workflow.
	st := store.NewMemStore[State]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{MaxSteps: 20}

	engine := graph.New(reducer, st, emitter, opts)
	if err := engine.Add("parse", parseNode); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add parse node: %v\n", err)
		os.Exit(1)
	}
	if err := engine.Add("fetch_weather", fetchWeatherNode); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add fetch_weather node: %v\n", err)
		os.Exit(1)
	}
	if err := engine.Add("fetch_news", fetchNewsNode); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add fetch_news node: %v\n", err)
		os.Exit(1)
	}
	if err := engine.Add("summarize", summarizeNode); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add summarize node: %v\n", err)
		os.Exit(1)
	}
	if err := engine.StartAt("parse"); err != nil {
		log.Fatalf("failed to set start node: %v", err)
	}

	// Run workflow.
	ctx := context.Background()
	initialState := State{
		UserQuery: "What's happening in New York today?",
	}

	fmt.Println("🚀 Starting workflow...")
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()

	final, err := engine.Run(ctx, "tool-example-001", initialState)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	// Display results.
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("🎉 Workflow Complete!")
	fmt.Println()
	fmt.Println("=== Final State ===")
	fmt.Printf("Location: %s\n", final.Location)
	fmt.Printf("Weather Data: %v\n", final.WeatherData != nil)
	fmt.Printf("News Articles: %d\n", len(final.NewsArticles))
	fmt.Println("\n=== Summary ===")
	fmt.Println(final.Summary)
	fmt.Println("\n=== News Headlines ===")
	for i, article := range final.NewsArticles {
		fmt.Printf("%d. %s\n", i+1, article)
	}
}
