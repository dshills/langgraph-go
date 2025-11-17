package transport

import (
	"context"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// MCPStdioServer wraps a JSON-RPC 2.0 connection for MCP protocol communication
// over standard input/output streams. It manages the lifecycle of the connection
// and provides graceful shutdown capabilities.
//
// The server uses Content-Length framing (LSP-compatible) via VSCodeObjectCodec
// for robust message parsing and handles JSON-RPC 2.0 requests through the
// provided handler.
//
// Example usage:
//
//	handler := &MyMCPHandler{}
//	server, err := NewMCPStdioServer(ctx, handler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Close()
//
//	if err := server.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
type MCPStdioServer struct {
	conn *jsonrpc2.Conn
}

// NewMCPStdioServer creates a new MCP server that communicates over stdin/stdout
// using JSON-RPC 2.0 protocol with Content-Length header framing.
//
// The server uses:
//   - StdioReadWriteCloser for bidirectional stdin/stdout communication
//   - BufferedStream for efficient I/O operations
//   - VSCodeObjectCodec for Content-Length header framing (LSP-compatible)
//
// The provided handler is invoked for all incoming JSON-RPC requests. The handler
// must implement the jsonrpc2.Handler interface to process method calls and return
// appropriate responses.
//
// The context is used to manage the connection lifecycle. When the context is
// cancelled, the connection will be closed gracefully.
//
// Parameters:
//   - ctx: context for connection lifecycle management
//   - handler: JSON-RPC 2.0 request handler implementing jsonrpc2.Handler
//
// Returns:
//   - *MCPStdioServer: configured server ready to start
//   - error: nil on success, error if connection setup fails
func NewMCPStdioServer(ctx context.Context, handler jsonrpc2.Handler) (*MCPStdioServer, error) {
	// Create stdio transport for bidirectional communication
	stdio := NewStdioReadWriteCloser()

	// Use BufferedStream with VSCodeObjectCodec for Content-Length framing
	// This provides LSP-compatible message framing with headers like:
	// Content-Length: 123\r\n\r\n{...json...}
	stream := jsonrpc2.NewBufferedStream(stdio, jsonrpc2.VSCodeObjectCodec{})

	// Create JSON-RPC connection with provided handler
	// The connection will automatically handle incoming requests and route
	// them to the handler's Handle method
	conn := jsonrpc2.NewConn(ctx, stream, handler)

	return &MCPStdioServer{conn: conn}, nil
}

// Start begins serving JSON-RPC requests and blocks until the provided context
// is cancelled or the connection is closed.
//
// This method:
//   - Blocks until ctx.Done() is signalled
//   - Automatically closes the connection on context cancellation
//   - Returns any error that occurred during connection closure
//
// The underlying JSON-RPC connection handles incoming requests concurrently
// through the handler provided during construction. Multiple requests may be
// processed simultaneously depending on the handler implementation.
//
// Call this method in a goroutine or as the last statement in main() to keep
// the server running:
//
//	go server.Start(ctx)
//	// or
//	if err := server.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// Parameters:
//   - ctx: context for controlling server lifetime
//
// Returns:
//   - error: nil on clean shutdown, error if connection closure fails
func (s *MCPStdioServer) Start(ctx context.Context) error {
	// Block until context is cancelled
	<-ctx.Done()

	// Close the connection gracefully
	// This ensures any in-flight requests are completed and resources are cleaned up
	return s.conn.Close()
}

// Close gracefully shuts down the JSON-RPC connection and releases all resources.
//
// This method should be called when the server is no longer needed, typically
// using defer after server creation:
//
//	server, err := NewMCPStdioServer(ctx, handler)
//	if err != nil {
//	    return err
//	}
//	defer server.Close()
//
// Close can be called multiple times safely. Subsequent calls after the first
// will have no effect.
//
// Returns:
//   - error: nil on success, error if connection closure fails
func (s *MCPStdioServer) Close() error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Wait blocks until the underlying JSON-RPC connection is closed.
//
// This method is useful when you need to wait for the server to finish processing
// without using a context. It blocks until:
//   - The connection is closed explicitly via Close()
//   - The connection is terminated due to an error
//   - The peer closes the connection
//
// Example usage:
//
//	server, err := NewMCPStdioServer(context.Background(), handler)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Start processing in background
//	go func() {
//	    // ... do work that eventually closes the server
//	}()
//
//	// Block until server is done
//	<-server.Wait()
//
// Returns:
//   - <-chan struct{}: channel that closes when the connection terminates
func (s *MCPStdioServer) Wait() <-chan struct{} {
	if s.conn == nil {
		// Return closed channel if no connection
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return s.conn.DisconnectNotify()
}

// Call invokes a JSON-RPC method on the connected peer and waits for the response.
//
// This method allows the server to act as a client and make outgoing requests
// to the peer (bidirectional communication). It blocks until the response is
// received or the context is cancelled.
//
// Parameters:
//   - ctx: context for request timeout and cancellation
//   - method: JSON-RPC method name to invoke
//   - params: parameters to pass to the method (will be JSON-encoded)
//   - result: pointer to store the result (will be JSON-decoded)
//
// Returns:
//   - error: nil on success, error if the call fails or context is cancelled
//
// Example:
//
//	var result InitializeResponse
//	err := server.Call(ctx, "initialize", initParams, &result)
func (s *MCPStdioServer) Call(ctx context.Context, method string, params, result interface{}) error {
	if s.conn == nil {
		return fmt.Errorf("connection not initialized")
	}
	return s.conn.Call(ctx, method, params, result)
}

// Notify sends a JSON-RPC notification to the connected peer without expecting a response.
//
// Notifications are fire-and-forget messages that do not have a request ID and
// do not generate responses. They are useful for event notifications and status
// updates that don't require acknowledgment.
//
// Parameters:
//   - ctx: context for send timeout and cancellation
//   - method: JSON-RPC method name to invoke
//   - params: parameters to pass to the method (will be JSON-encoded)
//
// Returns:
//   - error: nil on success, error if the notification fails to send
//
// Example:
//
//	err := server.Notify(ctx, "notifications/progress", progressUpdate)
func (s *MCPStdioServer) Notify(ctx context.Context, method string, params interface{}) error {
	if s.conn == nil {
		return fmt.Errorf("connection not initialized")
	}
	return s.conn.Notify(ctx, method, params)
}
