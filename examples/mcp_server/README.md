# MCP Server Example: Weather Tool

This example demonstrates how to expose LangGraph-Go tools to external LLM applications via the Model Context Protocol (MCP). The server implements a simple weather lookup tool that can be accessed from Claude Desktop or other MCP clients.

## Overview

The Model Context Protocol (MCP) enables LLM applications to discover and invoke tools exposed by servers over a standardized JSON-RPC interface. This example shows:

- Creating an MCP server with proper configuration
- Implementing a tool that follows the `Tool` interface
- Registering tools with metadata and JSON Schema for input validation
- Server lifecycle management with graceful shutdown
- Integration patterns for connecting to Claude Desktop

## Architecture

```
┌─────────────────────────────────────────────┐
│         Claude Desktop (MCP Client)         │
│  - Discovers available tools                │
│  - Validates input against schema           │
│  - Invokes tools with parameters            │
└────────────────┬────────────────────────────┘
                 │
                 │ JSON-RPC 2.0 over stdio
                 │
┌────────────────▼────────────────────────────┐
│    MCP Server (weather-server)              │
│  - Exposes tool metadata                    │
│  - Validates tool inputs                    │
│  - Executes tool logic                      │
│  - Returns structured results               │
└────────────────┬────────────────────────────┘
                 │
                 │ Tool interface
                 │
┌────────────────▼────────────────────────────┐
│         WeatherTool Implementation          │
│  - Accepts location parameter               │
│  - Returns mock weather data                │
│  - (Replace with real API in production)    │
└─────────────────────────────────────────────┘
```

## Features

### Current Implementation

- **Tool Registration**: Weather lookup tool with JSON Schema validation
- **Server Configuration**: Name, version, and observability settings
- **Graceful Shutdown**: Signal handling for clean termination
- **Input Validation**: JSON Schema enforcement for tool parameters
- **Error Handling**: Proper error reporting via JSON-RPC error codes
- **Logging**: Request/response logging for debugging

### Future Capabilities (In Development)

- **Resources**: Expose workflow state and checkpoints as readable resources
- **Prompts**: Reusable prompt templates with parameter substitution
- **Stdio Transport**: Full JSON-RPC 2.0 communication over stdin/stdout
- **Multiple Tools**: Register and manage multiple tools simultaneously

## Building and Running

### Prerequisites

- Go 1.21+ (requires generics support)
- LangGraph-Go framework installed

### Build the Server

```bash
# From the examples/mcp_server directory
go build -o weather-server weather_server.go

# Or from the project root
go build -o examples/mcp_server/weather-server examples/mcp_server/weather_server.go
```

### Run the Server

```bash
# Run directly (for testing and development)
./weather-server

# Expected output:
# Weather tool registered successfully
# MCP server starting on stdio...
# Server name: langgraph-weather
# Available tools: get_weather
```

### Development Status

**Note**: The stdio transport layer is currently in development. The server will complete initialization and display configuration information, but full client communication will be available in the next release. This example demonstrates the API for server configuration and tool registration.

## Connecting to Claude Desktop

Once the stdio transport is complete, you can connect the server to Claude Desktop by adding it to the MCP configuration file.

### Configuration File Location

**macOS/Linux**:
```
~/.config/Claude/claude_desktop_config.json
```

**Windows**:
```
%APPDATA%\Claude\claude_desktop_config.json
```

### Configuration Example

Edit the configuration file and add your server:

```json
{
  "mcpServers": {
    "langgraph-weather": {
      "command": "/absolute/path/to/weather-server"
    }
  }
}
```

**Important**: Use the absolute path to the compiled binary. The configuration will not work with relative paths.

### Example Configuration

```json
{
  "mcpServers": {
    "langgraph-weather": {
      "command": "/Users/username/langgraph-go/examples/mcp_server/weather-server"
    }
  }
}
```

### Restart Claude Desktop

After updating the configuration:

1. Save the `claude_desktop_config.json` file
2. Quit Claude Desktop completely (not just close the window)
3. Restart Claude Desktop
4. Your server should appear in the MCP tools menu

## Testing the Integration

Once connected to Claude Desktop, you can test the weather tool with these prompts:

### List Available Tools

```
What MCP tools do you have access to?
```

**Expected Response**: Claude should list "get_weather" with its description.

### Invoke the Weather Tool

```
Check the weather in San Francisco
```

**Expected Response**: Claude will call the `get_weather` tool and display:
- Location: San Francisco
- Temperature: 72°F
- Conditions: sunny
- Humidity: 65%

### Test Input Validation

```
Get the weather without specifying a location
```

**Expected Response**: Claude should receive a validation error about the missing `location` parameter.

### Multiple Locations

```
What's the weather in New York, London, and Tokyo?
```

**Expected Response**: Claude will make three separate tool calls and compare the results.

## Tool Schema

The weather tool accepts the following input:

```json
{
  "type": "object",
  "properties": {
    "location": {
      "type": "string",
      "description": "City name or zip code (e.g., 'San Francisco' or '94102')"
    }
  },
  "required": ["location"]
}
```

And returns this output structure:

```json
{
  "location": "San Francisco",
  "temperature": 72,
  "conditions": "sunny",
  "humidity": 65
}
```

## Implementation Details

### WeatherTool Structure

```go
type WeatherTool struct{}

func (w *WeatherTool) Name() string {
    return "get_weather"
}

func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Validate required parameter
    location, ok := input["location"].(string)
    if !ok || location == "" {
        return nil, fmt.Errorf("location parameter required")
    }

    // Check context cancellation
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }

    // Return mock weather data
    return map[string]interface{}{
        "location":    location,
        "temperature": 72,
        "conditions":  "sunny",
        "humidity":    65,
    }, nil
}
```

### Server Configuration

```go
mcpServer := mcp.NewServer(mcp.ServerConfig{
    Name:    "langgraph-weather",
    Version: "1.0.0",
    Emitter: emitter, // Optional observability
})
```

### Tool Registration

```go
err := mcpServer.RegisterTool("get_weather", weatherTool, mcp.ToolMetadata{
    Description: "Get current weather for a location",
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City name or zip code",
            },
        },
        "required": []string{"location"},
    },
})
```

## Extending the Example

### Add a Real Weather API

Replace the mock data with a real weather API:

```go
func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    location := input["location"].(string)

    // Call OpenWeatherMap API
    url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=imperial",
        location, os.Getenv("OPENWEATHER_API_KEY"))

    resp, err := http.Get(url)
    if err != nil {
        return nil, fmt.Errorf("API call failed: %w", err)
    }
    defer resp.Body.Close()

    var result map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    // Extract relevant data
    return map[string]interface{}{
        "location":    location,
        "temperature": int(result["main"].(map[string]interface{})["temp"].(float64)),
        "conditions":  result["weather"].([]interface{})[0].(map[string]interface{})["main"].(string),
        "humidity":    int(result["main"].(map[string]interface{})["humidity"].(float64)),
    }, nil
}
```

### Add More Tools

Register additional tools to expand functionality:

```go
// Add a forecast tool
forecastTool := &ForecastTool{}
mcpServer.RegisterTool("get_forecast", forecastTool, mcp.ToolMetadata{
    Description: "Get 5-day weather forecast",
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{"type": "string"},
            "days": map[string]interface{}{
                "type": "number",
                "minimum": 1,
                "maximum": 5,
            },
        },
        "required": []string{"location"},
    },
})

// Add an alerts tool
alertsTool := &AlertsTool{}
mcpServer.RegisterTool("get_weather_alerts", alertsTool, mcp.ToolMetadata{
    Description: "Get active weather alerts for a location",
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{"type": "string"},
        },
        "required": []string{"location"},
    },
})
```

### Add Caching

Implement caching to reduce API calls:

```go
type WeatherTool struct {
    cache map[string]cacheEntry
    mu    sync.RWMutex
}

type cacheEntry struct {
    data      map[string]interface{}
    timestamp time.Time
}

func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    location := input["location"].(string)

    // Check cache
    w.mu.RLock()
    if entry, exists := w.cache[location]; exists {
        if time.Since(entry.timestamp) < 10*time.Minute {
            w.mu.RUnlock()
            return entry.data, nil
        }
    }
    w.mu.RUnlock()

    // Fetch fresh data
    data, err := w.fetchWeather(ctx, location)
    if err != nil {
        return nil, err
    }

    // Update cache
    w.mu.Lock()
    w.cache[location] = cacheEntry{
        data:      data,
        timestamp: time.Now(),
    }
    w.mu.Unlock()

    return data, nil
}
```

## Troubleshooting

### Server Won't Start

**Problem**: Server exits immediately or with error

**Solutions**:
1. Check that Go version is 1.21+ (`go version`)
2. Verify all dependencies are installed (`go mod tidy`)
3. Check server logs for initialization errors
4. Ensure no other process is using stdio

### Tools Not Appearing in Claude Desktop

**Problem**: MCP server listed but tools not visible

**Solutions**:
1. Verify `claude_desktop_config.json` has correct absolute path to executable
2. Check that tool names follow pattern: `^[a-z][a-z0-9_]*$` (lowercase, underscores only)
3. Restart Claude Desktop after config changes (full quit, not just close window)
4. Check that tool metadata includes valid JSON Schema
5. Look for errors in Claude Desktop logs

### Tool Invocations Fail

**Problem**: Tool calls return errors

**Solutions**:
1. Verify input schema matches tool expectations
2. Check tool error messages in server logs
3. Ensure tool respects context cancellation (`if ctx.Err() != nil`)
4. Validate that tool returns `map[string]interface{}` not other types
5. Check for nil pointer dereferences in tool logic

### Transport Not Implemented

**Problem**: Server shows "transport not yet implemented" message

**Status**: This is expected in the current release. The MCP transport layer is in development and will be available in the next release. The example demonstrates the server configuration API and tool registration patterns.

**Workaround**: Use the existing API to understand how to structure your tools and metadata. When the transport layer is complete, your tools will work without code changes.

## Testing Without Claude Desktop

### Option 1: Unit Tests

Write tests for your tool implementation:

```go
func TestWeatherTool(t *testing.T) {
    tool := &WeatherTool{}

    // Test successful call
    input := map[string]interface{}{"location": "San Francisco"}
    output, err := tool.Call(context.Background(), input)
    if err != nil {
        t.Fatalf("Call failed: %v", err)
    }

    if output["location"] != "San Francisco" {
        t.Errorf("Expected location San Francisco, got %v", output["location"])
    }

    // Test missing parameter
    _, err = tool.Call(context.Background(), map[string]interface{}{})
    if err == nil {
        t.Error("Expected error for missing location parameter")
    }
}
```

### Option 2: MCP Inspector (Future)

Once the transport layer is complete, use the MCP Inspector tool:

```bash
# Install MCP Inspector
npm install -g @modelcontextprotocol/inspector

# Test your server
mcp-inspector ./weather-server
```

### Option 3: Manual JSON-RPC Testing (Future)

Once the transport layer is complete, test with raw JSON-RPC:

```bash
# Initialize connection
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./weather-server

# List tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./weather-server

# Call tool
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_weather","arguments":{"location":"SF"}}}' | ./weather-server
```

## Security Considerations

### Input Validation

- Always validate tool inputs against JSON Schema
- Sanitize location strings to prevent injection attacks
- Limit input sizes to prevent resource exhaustion

### API Keys

- Store API keys in environment variables, not code
- Use `.env` files for local development (add to `.gitignore`)
- Rotate API keys regularly in production
- Monitor API usage for anomalies

### Rate Limiting

- Implement rate limiting for production deployments
- Track tool invocations per client/user
- Set reasonable timeout limits
- Cache results to reduce API load

### Error Handling

- Don't expose internal errors or stack traces to clients
- Log sensitive errors server-side only
- Return generic error messages to clients
- Sanitize error messages before returning

## Performance Tips

1. **Cache Aggressively**: Weather data doesn't change every second
2. **Use Context Timeouts**: Set reasonable timeouts for API calls
3. **Batch Requests**: Consider supporting multiple locations in one call
4. **Monitor Memory**: Track cache size and evict old entries
5. **Async Operations**: Use goroutines for independent operations

## Related Examples

- **`examples/llm/`**: LLM integration patterns
- **`examples/tools/`**: Additional tool implementations
- **`examples/ai_research_assistant/`**: Complex multi-tool workflows
- **`examples/chatbot/`**: Interactive tool usage

## Further Reading

- [MCP Specification](https://modelcontextprotocol.io/specification/2025-06-18)
- [JSON-RPC 2.0 Spec](https://www.jsonrpc.org/specification)
- [LangGraph-Go Quickstart](../../specs/010-mcp-server-support/quickstart.md)
- [Tool Design Best Practices](../../CLAUDE.md)

## Support

- **Issues**: https://github.com/dshills/langgraph-go/issues
- **API Documentation**: See `graph/mcp/` package for detailed protocol specifications
- **Questions**: Open a discussion on GitHub

## License

This example is part of the LangGraph-Go project and follows the same license.
