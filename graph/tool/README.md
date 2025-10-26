# Tool System

Enable LLM agents to interact with external systems and perform actions through a flexible tool abstraction.

## Overview

The Tool system allows nodes in your LangGraph-Go workflows to call external services, APIs, and functions. Tools provide a standardized interface for:

- **Web Searches** - Query search engines and retrieve results
- **API Calls** - Interact with REST/GraphQL APIs
- **Database Queries** - Fetch and update data
- **File Operations** - Read, write, and process files
- **Calculations** - Perform complex computations
- **Code Execution** - Run scripts and programs

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/tool"
)

func main() {
    // Create HTTP tool
    httpTool := tool.NewHTTPTool()

    // Use tool in a node
    apiNode := graph.NodeFunc[MyState](func(ctx context.Context, state MyState) graph.NodeResult[MyState] {
        // Call external API using HTTP tool
        result, err := httpTool.Call(ctx, map[string]interface{}{
            "method": "GET",
            "url":    "https://api.example.com/data",
            "headers": map[string]interface{}{
                "Authorization": "Bearer " + state.APIKey,
            },
        })
        if err != nil {
            return graph.NodeResult[MyState]{Err: err}
        }

        // Process tool result
        statusCode := result["status_code"].(int)
        body := result["body"].(string)

        return graph.NodeResult[MyState]{
            Delta: MyState{
                Data:   body,
                Status: statusCode,
            },
            Route: graph.Stop(),
        }
    })

    // Build workflow with tool-enabled node
    // ... configure engine and run
}
```

## Tool Interface

All tools implement the `Tool` interface:

```go
type Tool interface {
    // Name returns the unique identifier for this tool
    Name() string

    // Call executes the tool with provided input
    Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}
```

### Method: `Name()`

Returns a unique identifier for the tool. Should be lowercase with underscores.

**Examples**: `"search_web"`, `"get_weather"`, `"calculate"`, `"http_request"`

### Method: `Call(ctx, input)`

Executes the tool with the provided parameters.

**Parameters**:
- `ctx` - Context for cancellation and timeout
- `input` - Tool parameters as key-value map (may be `nil`)

**Returns**:
- `map[string]interface{}` - Tool execution result
- `error` - Execution or validation errors

**Example**:
```go
result, err := tool.Call(ctx, map[string]interface{}{
    "query": "weather in San Francisco",
    "limit": 5,
})
```

## Built-in Tools

### HTTPTool

Make HTTP requests to external APIs and services.

**Create**:
```go
httpTool := tool.NewHTTPTool()
```

**Input Parameters**:
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `method` | string | No | HTTP method (default: "GET") |
| `url` | string | Yes | Target URL |
| `headers` | map | No | HTTP headers |
| `body` | string | No | Request body (for POST) |

**Output**:
| Field | Type | Description |
|-------|------|-------------|
| `status_code` | int | HTTP status code (200, 404, etc.) |
| `headers` | map | Response headers |
| `body` | string | Response body |

**Example - GET Request**:
```go
result, err := httpTool.Call(ctx, map[string]interface{}{
    "method": "GET",
    "url":    "https://api.github.com/repos/golang/go",
    "headers": map[string]interface{}{
        "Accept": "application/json",
    },
})

statusCode := result["status_code"].(int)
body := result["body"].(string)
```

**Example - POST Request**:
```go
requestBody := `{"name": "New Item", "price": 29.99}`

result, err := httpTool.Call(ctx, map[string]interface{}{
    "method": "POST",
    "url":    "https://api.example.com/items",
    "headers": map[string]interface{}{
        "Content-Type": "application/json",
    },
    "body": requestBody,
})
```

**Supported Methods**: GET, POST

**Error Handling**:
```go
result, err := httpTool.Call(ctx, input)
if err != nil {
    // Handle request errors (invalid URL, network issues, etc.)
    return graph.NodeResult[State]{Err: err}
}

// Check HTTP status code
if result["status_code"].(int) >= 400 {
    // Handle HTTP errors
    log.Printf("HTTP error: %d - %s",
        result["status_code"],
        result["body"])
}
```

## Creating Custom Tools

Implement the `Tool` interface to create custom tools:

```go
type WeatherTool struct {
    apiKey string
}

func NewWeatherTool(apiKey string) *WeatherTool {
    return &WeatherTool{apiKey: apiKey}
}

func (w *WeatherTool) Name() string {
    return "get_weather"
}

func (w *WeatherTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // 1. Validate input
    location, ok := input["location"].(string)
    if !ok || location == "" {
        return nil, fmt.Errorf("location parameter required")
    }

    // 2. Check context cancellation before expensive operations
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    // 3. Perform the operation
    weather, err := w.fetchWeather(ctx, location)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch weather: %w", err)
    }

    // 4. Return structured output
    return map[string]interface{}{
        "temperature": weather.Temp,
        "conditions":  weather.Conditions,
        "humidity":    weather.Humidity,
        "location":    location,
    }, nil
}

func (w *WeatherTool) fetchWeather(ctx context.Context, location string) (*WeatherData, error) {
    // Implementation details...
    return &WeatherData{}, nil
}
```

### Best Practices for Custom Tools

1. **Validate Input**:
```go
func (t *MyTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    // Validate required parameters
    param, ok := input["param"].(string)
    if !ok {
        return nil, fmt.Errorf("param required (string)")
    }

    // Validate optional parameters
    limit := 10 // default
    if l, ok := input["limit"].(int); ok {
        limit = l
    }

    // ... execute tool
}
```

2. **Respect Context**:
```go
// Check before expensive operations
if err := ctx.Err(); err != nil {
    return nil, err
}

// Pass context to child operations
result, err := http.Get(ctx, url)
```

3. **Return Structured Output**:
```go
// ✅ Good: Structured output
return map[string]interface{}{
    "results": items,
    "count":   len(items),
    "next":    nextPageToken,
}, nil

// ❌ Bad: Raw string output
return map[string]interface{}{
    "data": jsonString,
}, nil
```

4. **Handle Errors Gracefully**:
```go
if err != nil {
    // Wrap errors with context
    return nil, fmt.Errorf("database query failed: %w", err)
}
```

5. **Be Idempotent When Possible**:
```go
// Safe to call multiple times with same input
func (t *ReadTool) Call(ctx context.Context, input map[string]interface{}) {
    // Read-only operations are naturally idempotent
}
```

## Using Tools in Nodes

### Single Tool Call

```go
fetchNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    // Call tool
    result, err := weatherTool.Call(ctx, map[string]interface{}{
        "location": state.Location,
    })
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Use result in state
    temp := result["temperature"].(int)
    return graph.NodeResult[State]{
        Delta: State{Temperature: temp},
        Route: graph.Goto("display"),
    }
})
```

### Multiple Tool Calls

```go
processNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    // First tool: fetch user data
    userData, err := userTool.Call(ctx, map[string]interface{}{
        "user_id": state.UserID,
    })
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Second tool: enrich data
    enrichedData, err := enrichTool.Call(ctx, map[string]interface{}{
        "data": userData["profile"],
    })
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Use combined results
    return graph.NodeResult[State]{
        Delta: State{Profile: enrichedData},
        Route: graph.Stop(),
    }
})
```

### Conditional Tool Execution

```go
smartNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    var result map[string]interface{}
    var err error

    // Choose tool based on state
    if state.NeedsWeather {
        result, err = weatherTool.Call(ctx, map[string]interface{}{
            "location": state.Location,
        })
    } else {
        result, err = newsTool.Call(ctx, map[string]interface{}{
            "topic": state.Topic,
        })
    }

    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Data: result},
        Route: graph.Stop(),
    }
})
```

### Error Handling Patterns

```go
robustNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    result, err := externalTool.Call(ctx, state.Input)
    if err != nil {
        // Option 1: Return error (workflow stops)
        return graph.NodeResult[State]{Err: err}

        // Option 2: Handle error gracefully
        return graph.NodeResult[State]{
            Delta: State{
                Error:  err.Error(),
                Status: "failed",
            },
            Route: graph.Goto("error_handler"),
        }

        // Option 3: Retry logic
        for attempt := 0; attempt < 3; attempt++ {
            result, err = externalTool.Call(ctx, state.Input)
            if err == nil {
                break
            }
            time.Sleep(time.Second * time.Duration(attempt+1))
        }
        if err != nil {
            return graph.NodeResult[State]{Err: err}
        }
    }

    // Success path
    return graph.NodeResult[State]{
        Delta: State{Result: result},
        Route: graph.Stop(),
    }
})
```

## Tool Registry Pattern

For workflows with many tools, use a registry:

```go
type ToolRegistry struct {
    tools map[string]tool.Tool
}

func NewToolRegistry() *ToolRegistry {
    return &ToolRegistry{
        tools: make(map[string]tool.Tool),
    }
}

func (r *ToolRegistry) Register(t tool.Tool) {
    r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (tool.Tool, error) {
    t, ok := r.tools[name]
    if !ok {
        return nil, fmt.Errorf("tool not found: %s", name)
    }
    return t, nil
}

// Usage in workflow
registry := NewToolRegistry()
registry.Register(tool.NewHTTPTool())
registry.Register(NewWeatherTool(apiKey))

dynamicNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    // Get tool by name from state
    t, err := registry.Get(state.ToolName)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    result, err := t.Call(ctx, state.ToolInput)
    // ... handle result
})
```

## Integration with LLM Tool Calling

LangGraph-Go tools work seamlessly with LLM tool calling:

```go
// Define tools for LLM
weatherSpec := model.ToolSpec{
    Name:        "get_weather",
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
}

// LLM decides which tool to call
llmNode := graph.NodeFunc[State](func(ctx context.Context, state State) graph.NodeResult[State] {
    chatOut, err := llm.Chat(ctx, state.Messages, []model.ToolSpec{weatherSpec})
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Execute tool if LLM requested it
    if chatOut.ToolCall != nil {
        result, err := weatherTool.Call(ctx, chatOut.ToolCall.Arguments)
        if err != nil {
            return graph.NodeResult[State]{Err: err}
        }

        // Add tool result to conversation
        return graph.NodeResult[State]{
            Delta: State{
                Messages: append(state.Messages, model.Message{
                    Role:    "tool",
                    Content: fmt.Sprintf("%v", result),
                }),
            },
            Route: graph.Goto("llm"), // Continue conversation
        }
    }

    // No tool call - final answer
    return graph.NodeResult[State]{
        Delta: State{Answer: chatOut.Message.Content},
        Route: graph.Stop(),
    }
})
```

## Testing Tools

### Unit Testing Custom Tools

```go
func TestWeatherTool_Call(t *testing.T) {
    tool := NewWeatherTool("test-api-key")

    ctx := context.Background()
    input := map[string]interface{}{
        "location": "San Francisco",
    }

    result, err := tool.Call(ctx, input)
    if err != nil {
        t.Fatalf("Call() error = %v", err)
    }

    // Verify output structure
    if _, ok := result["temperature"]; !ok {
        t.Error("result missing temperature field")
    }

    if loc := result["location"]; loc != "San Francisco" {
        t.Errorf("location = %v, want San Francisco", loc)
    }
}
```

### Testing with Mock Tools

```go
type mockTool struct {
    name   string
    output map[string]interface{}
    err    error
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.output, nil
}

func TestNodeWithMockTool(t *testing.T) {
    mock := &mockTool{
        name: "test_tool",
        output: map[string]interface{}{
            "result": "success",
        },
    }

    // Test node with mock tool
    // ...
}
```

## Performance Considerations

### Tool Timeouts

```go
// Set timeout for tool execution
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := tool.Call(ctx, input)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Tool execution timed out")
    }
}
```

### Caching Tool Results

```go
type CachedTool struct {
    tool  tool.Tool
    cache map[string]map[string]interface{}
    mu    sync.RWMutex
}

func (c *CachedTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
    key := computeCacheKey(input)

    // Check cache
    c.mu.RLock()
    if cached, ok := c.cache[key]; ok {
        c.mu.RUnlock()
        return cached, nil
    }
    c.mu.RUnlock()

    // Call underlying tool
    result, err := c.tool.Call(ctx, input)
    if err != nil {
        return nil, err
    }

    // Store in cache
    c.mu.Lock()
    c.cache[key] = result
    c.mu.Unlock()

    return result, nil
}
```

## Troubleshooting

### Tool Not Found

```
Error: tool not found: search_web
```

**Solution**: Ensure tool is registered before use:
```go
registry.Register(searchTool)
```

### Invalid Input Type

```
Error: location parameter required (string)
```

**Solution**: Check input parameter types:
```go
location, ok := input["location"].(string)
if !ok {
    return nil, fmt.Errorf("location must be string, got %T", input["location"])
}
```

### Context Timeout

```
Error: context deadline exceeded
```

**Solution**: Increase timeout or optimize tool:
```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

### Network Errors

```
Error: failed to execute request: dial tcp: lookup failed
```

**Solution**: Add retry logic and error handling:
```go
for i := 0; i < 3; i++ {
    result, err = httpTool.Call(ctx, input)
    if err == nil {
        break
    }
    time.Sleep(time.Second * time.Duration(i+1))
}
```

## Examples

See [`examples/tools/`](../../examples/tools/) for complete working examples:

- **Basic Tool Usage** - Simple tool invocation in nodes
- **Multi-Tool Workflow** - Orchestrating multiple tools
- **LLM Tool Calling** - Integration with LLM-driven tool selection
- **Custom Tool Implementation** - Building domain-specific tools

## API Reference

- [Tool Interface](./tool.go) - Core tool interface definition
- [HTTPTool](./http.go) - HTTP request tool implementation

## Related Documentation

- [Node Documentation](../node.go) - Using tools within nodes
- [LLM Integration Guide](../model/) - Tool calling with LLMs
- [State Management](../../docs/guides/03-state-management.md) - Passing tool results via state

## Support

For issues or questions:
- [GitHub Issues](https://github.com/dshills/langgraph-go/issues)
- [Examples](../../examples/tools/)
- [FAQ](../../docs/FAQ.md)
