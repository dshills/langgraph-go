# MCP Transport Package

This package provides transport implementations for Model Context Protocol (MCP) JSON-RPC 2.0 communication.

## Overview

The transport package implements the underlying communication layer for MCP servers and clients. It provides abstractions for bidirectional JSON-RPC message exchange over different transport mechanisms.

## Transports

### JSON-RPC Server (MCPStdioServer)

The `MCPStdioServer` provides a complete JSON-RPC 2.0 server implementation for MCP protocol communication over stdin/stdout. It wraps the `sourcegraph/jsonrpc2` library with Content-Length framing (LSP-compatible) and handles connection lifecycle management.

**Features:**
- JSON-RPC 2.0 protocol compliance with Content-Length header framing
- Bidirectional communication (requests and notifications)
- Context-based lifecycle management
- Graceful shutdown with proper resource cleanup
- Support for concurrent request handling

**Example:**

```go
package main

import (
    "context"
    "log"

    "github.com/dshills/langgraph-go/graph/mcp/transport"
    "github.com/sourcegraph/jsonrpc2"
)

type MyHandler struct{}

func (h *MyHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
    switch req.Method {
    case "initialize":
        // Handle initialization
        conn.Reply(ctx, req.ID, map[string]string{"status": "ready"})
    case "tools/list":
        // Handle tool listing
        conn.Reply(ctx, req.ID, []string{"tool1", "tool2"})
    default:
        conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
            Code:    -32601,
            Message: "Method not found",
        })
    }
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    handler := &MyHandler{}
    server, err := transport.NewMCPStdioServer(ctx, handler)
    if err != nil {
        log.Fatal(err)
    }
    defer server.Close()

    // Start serving requests (blocks until context cancelled)
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Stdio Transport

The `StdioReadWriteCloser` wraps standard input and output streams (`os.Stdin` and `os.Stdout`) to provide an `io.ReadWriteCloser` interface for JSON-RPC communication.

**Use Cases:**
- MCP server implementations that communicate via stdin/stdout
- Command-line tools that use JSON-RPC over standard streams
- Process-based IPC using standard I/O redirection

**Example:**

```go
package main

import (
    "encoding/json"
    "log"

    "github.com/dshills/langgraph-go/graph/mcp"
    "github.com/dshills/langgraph-go/graph/mcp/transport"
)

func main() {
    // Create stdio transport
    t := transport.NewStdioReadWriteCloser()
    defer t.Close()

    // Set up JSON encoder/decoder
    decoder := json.NewDecoder(t)
    encoder := json.NewEncoder(t)

    // Read incoming requests
    var req mcp.Request
    if err := decoder.Decode(&req); err != nil {
        log.Fatal(err)
    }

    // Process request...

    // Send response
    resp := mcp.Response{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result:  processResult,
    }
    if err := encoder.Encode(resp); err != nil {
        log.Fatal(err)
    }
}
```

## Interface

All transports implement `io.ReadWriteCloser`:

```go
type ReadWriteCloser interface {
    Read(b []byte) (n int, err error)
    Write(b []byte) (n int, err error)
    Close() error
}
```

This allows for seamless integration with `encoding/json` and other standard library packages.

## Testing

The package includes comprehensive tests:

```bash
# Run tests (skip stdio tests that require actual stdin/stdout)
go test -short ./graph/mcp/transport/

# Run with coverage
go test -short -cover ./graph/mcp/transport/

# Run with race detector
go test -short -race ./graph/mcp/transport/

# Run full tests including stdio integration tests
# Note: These may produce stderr output due to stdin/stdout contention
go test -v ./graph/mcp/transport/
```

**Test Limitations:**

Due to stdin/stdout being global resources, tests that create actual `MCPStdioServer` instances are skipped in `-short` mode. These tests verify:
- Server creation and initialization
- Connection lifecycle management
- Graceful shutdown behavior
- Context cancellation handling

In production, each MCP server runs in isolation with dedicated stdio streams, avoiding the contention issues present in test environments.

## Error Handling

The stdio transport properly handles errors:

- **Read errors**: Propagated from the underlying reader
- **Write errors**: Propagated from the underlying writer
- **Close errors**: Both reader and writer close errors are joined using `errors.Join`

This ensures that all error information is preserved when closing the transport.
