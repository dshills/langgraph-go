package tool

import "context"

// Tool defines the interface for executable tools that LLMs can invoke.
//
// Tools enable LLMs to interact with external systems and perform actions:
//   - Web searches
//   - Database queries
//   - API calls
//   - File operations
//   - Calculations
//   - Code execution
//
// Implementations should:
//   - Validate input parameters
//   - Respect context cancellation and timeouts
//   - Return structured output as map[string]interface{}
//   - Handle errors gracefully with clear error messages
//   - Be idempotent when possible
//
// Example implementation:
//
//	type WeatherTool struct{}
//
//	func (w *WeatherTool) Name() string {
//	    return "get_weather"
//	}
//
//	func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
//	    location, ok := input["location"].(string)
//	    if !ok {
//	        return nil, errors.New("location parameter required")
//	    }
//
//	    // Fetch weather data...
//	    temp := 72.5
//
//	    return map[string]interface{}{
//	        "temperature": temp,
//	        "conditions":  "sunny",
//	        "location":    location,
//	    }, nil
//	}
//
// Example usage in a workflow:
//
//	weatherTool := &WeatherTool{}
//	input := map[string]interface{}{"location": "San Francisco"}
//	output, err := weatherTool.Call(ctx, input)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Temperature: %v\n", output["temperature"])
type Tool interface {
	// Name returns the unique identifier for this tool.
	//
	// The name must match the tool name in ToolSpec used by the LLM.
	// Names should be lowercase with underscores, following function naming conventions.
	//
	// Examples: "search_web", "get_weather", "calculate", "send_email"
	Name() string

	// Call executes the tool with the provided input and returns the result.
	//
	// Parameters:
	//   - ctx: Context for cancellation, timeout, and metadata propagation
	//   - input: Tool parameters as key-value pairs (may be nil for parameterless tools)
	//
	// Returns:
	//   - map[string]interface{}: Tool execution result
	//   - error: Execution errors, validation errors, or context cancellation
	//
	// The input structure should match the Schema defined in the corresponding ToolSpec.
	// The output can be any structured data that the LLM can process.
	//
	// Implementations should:
	//   - Check ctx.Err() before expensive operations
	//   - Validate required input parameters
	//   - Return descriptive errors for invalid inputs
	//   - Include relevant metadata in the output
	Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}
