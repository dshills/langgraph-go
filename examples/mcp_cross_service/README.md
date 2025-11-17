# Cross-Service MCP Example

This example demonstrates how to configure LangGraph MCP servers for robust cross-service communication.

## Overview

While this implementation focuses on **server-side** capabilities (MCP client is out of scope), the server is designed to handle production cross-service scenarios where:

1. Multiple LangGraph workflows run as separate MCP servers
2. External MCP clients (Claude Desktop, custom clients, etc.) connect to these servers
3. Servers handle concurrent requests, timeouts, and disconnections gracefully

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│  MCP Server A   │         │  MCP Server B   │
│  (Tools +       │         │  (Resources +   │
│   Resources)    │         │   Tools)        │
└────────┬────────┘         └────────┬────────┘
         │                           │
         │                           │
         └───────────┬───────────────┘
                     │
              ┌──────▼────────┐
              │  External     │
              │  MCP Client   │
              │  (Claude, etc)│
              └───────────────┘
```

## Key Features

### 1. Connection Lifecycle Management

The server tracks connection sessions and emits observability events:

```go
// Emitted when client connects
"client_connect" {
    "protocol_version": "2025-06-18",
    "client_name": "claude-desktop",
    "client_capabilities": {...}
}

// Emitted when client disconnects
"client_disconnect" {
    "server_name": "langgraph-service-a"
}
```

### 2. Context Cancellation Support

All tool invocations respect context cancellation:

```go
tool.Call(ctx, input)  // Respects ctx.Done()
```

This ensures that when clients disconnect or timeout occurs, in-flight operations are properly cancelled.

### 3. Graceful Shutdown

The server handles shutdown gracefully:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Start server (blocks until ctx is cancelled)
if err := server.Start(ctx); err != nil {
    log.Fatal(err)
}

// On shutdown:
// 1. Emits client_disconnect event
// 2. Closes transport connection
// 3. Flushes observability events
// 4. Cleans up resources
```

### 4. Thread-Safe Concurrent Access

Tools and resources can be safely invoked concurrently:

```go
// Multiple clients can call the same tool simultaneously
// - ToolRegistry uses sync.RWMutex for thread safety
// - Resource provider uses sync.RWMutex
// - Tool implementations should also be thread-safe
```

## Running the Example

```bash
# Build the servers
go build -o service-a ./service_a.go
go build -o service-b ./service_b.go

# Run Service A (exposes weather tools)
./service-a

# Run Service B (exposes state resources)
./service-b
```

## Connecting External Clients

### Claude Desktop Configuration

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "langgraph-service-a": {
      "command": "/path/to/service-a"
    },
    "langgraph-service-b": {
      "command": "/path/to/service-b"
    }
  }
}
```

Claude will connect to both servers independently and can use tools/resources from either.

### Custom MCP Clients

External MCP clients should:

1. **Handle stdio transport** - communicate via stdin/stdout
2. **Implement JSON-RPC 2.0** - send properly formatted requests
3. **Respect protocol negotiation** - send `initialize` request first
4. **Handle disconnection gracefully** - server will emit events and clean up

## Observability

Monitor cross-service health using emitted events:

```go
emitter := emit.NewLogEmitter(os.Stdout, false)

server := mcp.NewServer(mcp.ServerConfig{
    Name:    "service-a",
    Version: "1.0.0",
    Emitter: emitter,
})

// Events emitted:
// - server_start: Server begins accepting requests
// - client_connect: Client successfully initialized
// - tool_call_start/tool_call_end: Tool invocation lifecycle
// - resource_read_start/resource_read_complete: Resource access
// - client_disconnect: Client disconnected
// - server_stop: Server shutdown complete
```

## Production Deployment

For production cross-service deployments:

1. **Use persistent storage** - Replace MemStore with MySQL/PostgreSQL Store
2. **Add monitoring** - Integrate with Prometheus/OpenTelemetry via Emitter
3. **Configure timeouts** - Set appropriate context deadlines for operations
4. **Implement health checks** - Monitor server state and connection health
5. **Network security** - Deploy in trusted network environment or add authentication layer

## Testing Cross-Service Scenarios

See `tests/integration/mcp_integration_test.go` for:

- Connection timeout handling tests
- Graceful disconnection tests
- Concurrent connection simulation

Run tests:

```bash
go test ./tests/integration/... -v
```

## Future Enhancements

When MCP client implementation is added (currently out of scope):

- Direct workflow-to-workflow tool invocation
- Cross-service state sharing via resources
- Service discovery and dynamic routing
- Application-level authentication between services
