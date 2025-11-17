# Quickstart: MCP Server Support

**Feature**: MCP Server Support
**Audience**: Developers integrating MCP server into LangGraph-Go workflows
**Time to Complete**: 10 minutes
**Prerequisites**: Basic familiarity with LangGraph-Go tool system

## Overview

This quickstart demonstrates how to expose a LangGraph workflow's tools, state, and prompts to external LLM applications via the Model Context Protocol (MCP). By the end, you'll have a running MCP server that can be accessed from Claude Desktop or other MCP clients.

**What You'll Build**:
- MCP server exposing a weather lookup tool
- Resource providing current workflow state
- Prompt template for starting the workflow

---

## Installation

```bash
# Add MCP server support to your project
go get github.com/dshills/langgraph-go/graph/mcp
```

---

## Step 1: Define Your Workflow State (2 minutes)

Create a simple workflow with a weather tool:

```go
// main.go
package main

import (
    "context"
    "fmt"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/tool"
    "github.com/dshills/langgraph-go/graph/store"
    "github.com/dshills/langgraph-go/graph/emit"
    "github.com/dshills/langgraph-go/graph/mcp"
)

// State represents workflow state
type State struct {
    Location    string
    Temperature int
    Conditions  string
    LastQuery   string
}

// Reducer merges state updates
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
    if delta.LastQuery != "" {
        prev.LastQuery = delta.LastQuery
    }
    return prev
}
```

---

## Step 2: Create a Tool (2 minutes)

Implement a simple weather tool:

```go
// WeatherTool fetches weather data (mock implementation)
type WeatherTool struct{}

func (w *WeatherTool) Name() string {
    return "get_weather"
}

func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    location, ok := input["location"].(string)
    if !ok || location == "" {
        return nil, fmt.Errorf("location parameter required")
    }

    // Mock weather data (replace with actual API call)
    return map[string]interface{}{
        "location":    location,
        "temperature": 72,
        "conditions":  "sunny",
        "humidity":    65,
    }, nil
}
```

---

## Step 3: Set Up MCP Server (3 minutes)

Create an MCP server and register your tool:

```go
func main() {
    ctx := context.Background()

    // Create LangGraph components
    st := store.NewMemStore[State]()
    emitter := emit.NewLogEmitter(os.Stdout, false)
    engine := graph.New(reducer, st, emitter)

    // Create weather tool
    weatherTool := &WeatherTool{}

    // Create MCP server
    mcpServer := mcp.NewServer(ctx, mcp.ServerConfig{
        Name:    "langgraph-weather",
        Version: "1.0.0",
    })

    // Register tool with MCP metadata
    err := mcpServer.RegisterTool("get_weather", weatherTool, mcp.ToolMetadata{
        Description: "Get current weather for a location",
        InputSchema: map[string]interface{}{
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
    if err != nil {
        log.Fatalf("Failed to register tool: %v", err)
    }

    // Register workflow state as a resource
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

    // Register a prompt template
    err = mcpServer.RegisterPrompt(mcp.PromptTemplate{
        Name:        "check_weather",
        Description: "Check weather for a location",
        Parameters: []mcp.PromptParameter{
            {
                Name:        "location",
                Description: "Location to check weather for",
                Required:    true,
            },
        },
        Template: "What's the weather like in {{location}}?",
    })
    if err != nil {
        log.Fatalf("Failed to register prompt: %v", err)
    }

    log.Println("MCP server starting on stdio...")

    // Start MCP server (runs until context cancelled)
    if err := mcpServer.Start(ctx); err != nil {
        log.Fatalf("MCP server error: %v", err)
    }
}
```

---

## Step 4: Run the Server (1 minute)

```bash
# Build the server
go build -o weather-server

# Run directly (for testing)
./weather-server
```

---

## Step 5: Connect from Claude Desktop (2 minutes)

Add your server to Claude Desktop's MCP configuration:

**macOS/Linux**: Edit `~/.config/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "langgraph-weather": {
      "command": "/path/to/your/weather-server"
    }
  }
}
```

**Windows**: Edit `%APPDATA%\Claude\claude_desktop_config.json`

Restart Claude Desktop, and your server will appear in the MCP tools menu.

---

## Step 6: Test the Integration (1 minute)

In Claude Desktop, try these prompts:

1. **List available tools**:
   ```
   What MCP tools do you have access to?
   ```

2. **Invoke the weather tool**:
   ```
   Check the weather in San Francisco
   ```

3. **Read workflow state**:
   ```
   Show me the current workflow state
   ```

4. **Use prompt template**:
   ```
   Use the check_weather prompt for New York
   ```

---

## What Just Happened?

1. **Tool Exposure**: Your `WeatherTool` is now discoverable by Claude Desktop
2. **Tool Invocation**: Claude can call `get_weather` with location parameters
3. **State Access**: Claude can read your workflow's current state via resources
4. **Guided Interactions**: Prompt templates guide Claude through common operations

---

## Next Steps

### Add More Tools

```go
// Register multiple tools
mcpServer.RegisterTool("database_query", dbTool, metadata1)
mcpServer.RegisterTool("file_processor", fileool, metadata2)
mcpServer.RegisterTool("api_call", httpTool, metadata3)
```

### Expose Checkpoints as Resources

```go
// List all checkpoints as resources
checkpoints, err := st.ListCheckpoints(ctx, "run-123")
for _, cp := range checkpoints {
    uri := fmt.Sprintf("checkpoints/%s", cp.Label)
    mcpServer.RegisterStaticResource(uri, "application/json", cp.StateData)
}
```

### Create Custom Prompt Templates

```go
mcpServer.RegisterPrompt(mcp.PromptTemplate{
    Name:        "analyze_results",
    Description: "Analyze workflow execution results",
    Parameters: []mcp.PromptParameter{
        {Name: "run_id", Required: true},
        {Name: "focus_area", Required: false, DefaultValue: "performance"},
    },
    Template: "Analyze the results of run {{run_id}}, focusing on {{focus_area}}",
})
```

### Add Observability

```go
// MCP server emits events through LangGraph's Emitter interface
mcpServer := mcp.NewServer(ctx, mcp.ServerConfig{
    Name:    "my-server",
    Emitter: emitter, // Use existing emitter
})

// Events emitted:
// - server_start, server_stop
// - tool_call_start, tool_call_end
// - resource_read
// - prompt_render
```

---

## Testing Without Claude Desktop

### Option 1: Use MCP Inspector (Recommended)

```bash
# Install MCP Inspector
npm install -g @modelcontextprotocol/inspector

# Test your server
mcp-inspector ./weather-server
```

### Option 2: Manual JSON-RPC Testing

```bash
# Start server
./weather-server

# In another terminal, send JSON-RPC requests to stdin:
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./weather-server

echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./weather-server

echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_weather","input":{"location":"SF"}}}' | ./weather-server
```

---

## Troubleshooting

### Server Won't Start

**Problem**: Server exits immediately or with error

**Solutions**:
1. Check that stdio is available (don't redirect stdin/stdout in shell)
2. Verify Go version (requires Go 1.21+)
3. Check server logs for initialization errors

### Tools Not Appearing in Claude Desktop

**Problem**: MCP server listed but no tools visible

**Solutions**:
1. Verify `claude_desktop_config.json` has correct path to executable
2. Restart Claude Desktop after config changes
3. Check that tool metadata includes valid JSON Schema
4. Ensure tool names follow pattern: `^[a-z][a-z0-9_]*$`

### Tool Invocations Fail

**Problem**: Tool calls return errors

**Solutions**:
1. Verify input schema matches tool expectations
2. Check tool error messages in logs
3. Ensure tool respects context cancellation
4. Validate that tool returns `map[string]interface{}` not string

### Resource Reads Fail

**Problem**: Resources return errors or empty data

**Solutions**:
1. Check that Store has data for the runID
2. Verify resource generator functions don't panic
3. Ensure resource URIs are valid (lowercase, underscores only)
4. Check resource size is under 10MB limit

---

## Common Patterns

### Pattern 1: Workflow Control Tools

Expose workflow control operations as tools:

```go
// Start workflow tool
startTool := &tool.NodeFunc[State](func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    runID := input["run_id"].(string)
    initialState := State{LastQuery: "started"}

    finalState, err := engine.Run(ctx, runID, initialState)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "status":      "completed",
        "final_state": finalState,
    }, nil
})

mcpServer.RegisterTool("start_workflow", startTool, metadata)
```

### Pattern 2: Real-Time Metrics

Expose live metrics as dynamic resources:

```go
mcpServer.RegisterDynamicResource("metrics/runtime", "application/json", func(ctx context.Context) ([]byte, error) {
    metrics := map[string]interface{}{
        "uptime":        time.Since(startTime).Seconds(),
        "tool_calls":    callCounter.Load(),
        "active_runs":   len(activeRuns),
        "memory_usage":  runtime.MemStats.Alloc,
    }
    return json.Marshal(metrics)
})
```

### Pattern 3: Multi-Step Prompts

Create prompts that guide multi-turn conversations:

```go
mcpServer.RegisterPrompt(mcp.PromptTemplate{
    Name:        "debug_workflow",
    Description: "Debug a failed workflow run",
    Parameters: []mcp.PromptParameter{
        {Name: "run_id", Required: true},
    },
    Messages: []mcp.PromptMessage{
        {
            Role: "user",
            Content: "I need help debugging run {{run_id}}",
        },
        {
            Role: "assistant",
            Content: "I'll help you debug that. First, let me check the workflow state.",
        },
        {
            Role: "user",
            Content: "Please read the workflow_state resource and analyze the error.",
        },
    },
})
```

---

## Performance Tips

1. **Cache Static Resources**: Don't regenerate fixed content on every read
2. **Limit Resource Size**: Keep resources under 1MB for fast reads
3. **Batch Tool Registrations**: Register all tools during initialization, not at runtime
4. **Use Context Timeouts**: Set reasonable timeouts for tool execution
5. **Monitor Memory**: Track resource memory usage with Emitter events

---

## Security Considerations

1. **Validate Tool Inputs**: Always validate against JSON Schema
2. **Sanitize Resource URIs**: Check for path traversal attempts
3. **Rate Limit**: Consider adding rate limiting for production deployments
4. **Audit Logging**: Log all tool invocations and resource reads
5. **Sensitive Data**: Don't expose credentials or secrets as resources

---

## Full Example

See `examples/mcp_server/` for a complete working example with:
- Multiple tools (weather, database, file operations)
- Dynamic resources (state, checkpoints, metrics)
- Prompt templates (start, resume, analyze)
- Claude Desktop configuration
- Integration tests

---

## API Reference

### Server Creation

```go
func NewServer(ctx context.Context, config ServerConfig) *Server

type ServerConfig struct {
    Name    string        // Server name (e.g., "langgraph-weather")
    Version string        // Server version (e.g., "1.0.0")
    Emitter emit.Emitter  // Optional observability emitter
}
```

### Tool Registration

```go
func (s *Server) RegisterTool(name string, tool tool.Tool, metadata ToolMetadata) error

type ToolMetadata struct {
    Description string                 // Human-readable description
    InputSchema map[string]interface{} // JSON Schema for input parameters
}
```

### Resource Registration

```go
// Static resource (fixed content)
func (s *Server) RegisterStaticResource(uri string, mimeType string, content []byte) error

// Dynamic resource (computed on-demand)
func (s *Server) RegisterDynamicResource(uri string, mimeType string, generator func(context.Context) ([]byte, error)) error
```

### Prompt Registration

```go
func (s *Server) RegisterPrompt(template PromptTemplate) error

type PromptTemplate struct {
    Name        string
    Description string
    Parameters  []PromptParameter
    Template    string  // Template with {{param}} placeholders
}

type PromptParameter struct {
    Name         string
    Description  string
    Required     bool
    DefaultValue string
}
```

### Server Lifecycle

```go
// Start server (blocks until context cancelled)
func (s *Server) Start(ctx context.Context) error

// Graceful shutdown (called automatically on context cancellation)
func (s *Server) Stop() error
```

---

## Further Reading

- [MCP Specification](https://modelcontextprotocol.io/specification/2025-06-18)
- [JSON-RPC 2.0 Spec](https://www.jsonrpc.org/specification)
- [Tool Design Best Practices](../../docs/guides/tools.md)
- [Resource Provider Patterns](../../docs/guides/resources.md)
- [Prompt Template Guide](../../docs/guides/prompts.md)

---

## Support

- **Issues**: https://github.com/dshills/langgraph-go/issues
- **Examples**: `examples/mcp_server/`
- **API Docs**: See contracts/ directory for detailed protocol specifications
