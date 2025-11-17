// Package main implements an MCP server that exposes a weather lookup tool
// to external LLM applications via the Model Context Protocol.
//
// This example demonstrates:
// - Creating an MCP server with proper configuration
// - Implementing a tool that follows the Tool interface
// - Registering tools with metadata and JSON Schema
// - Server lifecycle management with graceful shutdown
//
// Usage:
//
//	go build -o weather-server
//	./weather-server
//
// The server communicates over stdio and can be connected to Claude Desktop
// or other MCP clients by adding it to the MCP configuration.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/mcp"
	"github.com/dshills/langgraph-go/graph/store"
)

// State represents the workflow state for weather queries.
type State struct {
	Location    string // Last queried location
	Temperature int    // Temperature in Fahrenheit
	Conditions  string // Weather conditions (sunny, cloudy, rainy, etc.)
	Humidity    int    // Humidity percentage
	LastQuery   string // Timestamp or identifier of last query
}

// reducer merges state updates deterministically.
// This enables the workflow state to be tracked and checkpointed.
func reducer(prev, delta State) State {
	if delta.Location != "" {
		prev.Location = delta.Location
	}
	if delta.Temperature != 0 {
		prev.Temperature = delta.Temperature
	}
	if delta.Conditions != "" {
		prev.Conditions = delta.Conditions
	}
	if delta.Humidity != 0 {
		prev.Humidity = delta.Humidity
	}
	if delta.LastQuery != "" {
		prev.LastQuery = delta.LastQuery
	}
	return prev
}

// WeatherTool implements the Tool interface to provide weather lookups.
// In a real implementation, this would call an actual weather API.
type WeatherTool struct{}

// Name returns the unique identifier for this tool.
// Must match the name used during registration.
func (w *WeatherTool) Name() string {
	return "get_weather"
}

// Call executes the weather lookup with the provided location parameter.
//
// Input schema:
//
//	{
//	  "location": string (required) - City name or zip code
//	}
//
// Output schema:
//
//	{
//	  "location": string - Queried location
//	  "temperature": int - Temperature in Fahrenheit
//	  "conditions": string - Weather conditions description
//	  "humidity": int - Humidity percentage
//	}
func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate required parameter
	location, ok := input["location"].(string)
	if !ok || location == "" {
		return nil, fmt.Errorf("location parameter required (must be non-empty string)")
	}

	// Check context cancellation before expensive operations
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Mock weather data (replace with actual API call in production)
	// In a real implementation, you would:
	// 1. Call a weather API (e.g., OpenWeatherMap, Weather.gov)
	// 2. Parse the response
	// 3. Handle API errors and rate limits
	// 4. Cache results to reduce API calls
	weatherData := map[string]interface{}{
		"location":    location,
		"temperature": 72,
		"conditions":  "sunny",
		"humidity":    65,
	}

	log.Printf("Weather query for %s: %d°F, %s, %d%% humidity",
		location,
		weatherData["temperature"],
		weatherData["conditions"],
		weatherData["humidity"])

	return weatherData, nil
}

func main() {
	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	// Create LangGraph components
	// In a real application, you would use a persistent store (e.g., MySQL)
	_ = store.NewMemStore[State]() // Store for future resource registration
	emitter := emit.NewLogEmitter(os.Stdout, false)

	// Create MCP server with configuration
	mcpServer := mcp.NewServer(mcp.ServerConfig{
		Name:    "langgraph-weather",
		Version: "1.0.0",
		Emitter: emitter, // Optional: enables observability events
	})

	// Create and register the weather tool
	weatherTool := &WeatherTool{}

	// Register tool with MCP metadata and JSON Schema
	// The schema follows JSON Schema specification for input validation
	err := mcpServer.RegisterTool("get_weather", weatherTool, mcp.ToolMetadata{
		Description: "Get current weather for a location",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "City name or zip code (e.g., 'San Francisco' or '94102')",
				},
			},
			"required": []string{"location"},
		},
	})
	if err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}

	log.Println("Weather tool registered successfully")

	// NOTE: The following sections demonstrate the intended API for resources
	// and prompts. These capabilities are currently in development and will
	// return "not yet implemented" errors if invoked by clients.

	// Register workflow state as a resource (future capability)
	// This would allow clients to read the current workflow state
	/*
		err = mcpServer.RegisterDynamicResource("workflow_state", "application/json", func(ctx context.Context) ([]byte, error) {
			state, _, _, err := st.LoadLatest(ctx, "current-run")
			if err != nil {
				return nil, err
			}
			return json.Marshal(state)
		})
		if err != nil {
			log.Fatalf("Failed to register resource: %v", err)
		}
	*/

	// Register prompt templates for weather queries
	// These provide reusable prompts that clients can render with parameters
	err = mcpServer.RegisterPrompt(mcp.PromptTemplate{
		Name:        "check_weather",
		Description: "Check weather for a specific location",
		Parameters: []mcp.PromptParameter{
			{
				Name:        "location",
				Description: "City name or zip code to check weather for",
				Required:    true,
			},
		},
		Template: "What's the weather like in {{location}}? Please use the get_weather tool to find out.",
	})
	if err != nil {
		log.Fatalf("Failed to register check_weather prompt: %v", err)
	}

	err = mcpServer.RegisterPrompt(mcp.PromptTemplate{
		Name:        "compare_weather",
		Description: "Compare weather between two locations",
		Parameters: []mcp.PromptParameter{
			{
				Name:        "location1",
				Description: "First location to compare",
				Required:    true,
			},
			{
				Name:        "location2",
				Description: "Second location to compare",
				Required:    true,
			},
		},
		Template: "Compare the weather between {{location1}} and {{location2}}. Use the get_weather tool for both locations and provide a summary of the differences.",
	})
	if err != nil {
		log.Fatalf("Failed to register compare_weather prompt: %v", err)
	}

	err = mcpServer.RegisterPrompt(mcp.PromptTemplate{
		Name:        "weather_advice",
		Description: "Get weather-appropriate advice for activities",
		Parameters: []mcp.PromptParameter{
			{
				Name:        "location",
				Description: "Location to check weather for",
				Required:    true,
			},
			{
				Name:         "activity",
				Description:  "Planned activity (e.g., hiking, picnic, sports)",
				Required:     false,
				DefaultValue: "outdoor activities",
			},
		},
		Template: "Check the weather in {{location}} and advise whether it's suitable for {{activity}}. Use the get_weather tool to get current conditions.",
	})
	if err != nil {
		log.Fatalf("Failed to register weather_advice prompt: %v", err)
	}

	log.Println("Prompt templates registered successfully")

	log.Println("MCP server starting on stdio...")
	log.Println("Server name: langgraph-weather")
	log.Println("Available tools: get_weather")
	log.Println("Available prompts: check_weather, compare_weather, weather_advice")
	log.Println("")
	log.Println("Configure Claude Desktop with:")
	log.Println(`  "langgraph-weather": {`)
	log.Printf(`    "command": "%s"`, getExecutablePath())
	log.Println(`  }`)
	log.Println("")

	// Start MCP server (blocks until context is cancelled)
	// NOTE: Transport layer implementation is in progress.
	// The server will currently return an error about transport not being implemented.
	// Once the transport layer is complete, this will handle stdio communication
	// and process JSON-RPC requests from MCP clients.
	if err := mcpServer.Start(ctx); err != nil {
		// Check if this is the expected "not yet implemented" error
		errMsg := err.Error()
		if errMsg == "failed to create transport: transport creation not yet implemented - will be completed in transport integration phase" {
			log.Println("")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("Note: MCP server transport layer is in development.")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("")
			log.Println("This example demonstrates the server configuration and tool")
			log.Println("registration API. Full stdio transport support will be available")
			log.Println("in the next release.")
			log.Println("")
			log.Println("Server configuration complete. Tools registered:")
			log.Println("  ✓ get_weather: Get current weather for a location")
			log.Println("")
			log.Println("Prompts registered:")
			log.Println("  ✓ check_weather: Check weather for a specific location")
			log.Println("  ✓ compare_weather: Compare weather between two locations")
			log.Println("  ✓ weather_advice: Get weather-appropriate advice for activities")
			log.Println("")
			log.Println("Once transport is implemented, this server will:")
			log.Println("  • Communicate with Claude Desktop via stdio")
			log.Println("  • Process JSON-RPC 2.0 requests")
			log.Println("  • Validate tool inputs against JSON Schema")
			log.Println("  • Return structured tool results")
			log.Println("  • Render prompt templates with parameters")
			return
		}
		log.Fatalf("MCP server error: %v", err)
	}

	log.Println("MCP server stopped cleanly")
}

// getExecutablePath returns the absolute path to the current executable.
// This is used to generate the Claude Desktop configuration example.
func getExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "/path/to/weather-server"
	}
	return exe
}
