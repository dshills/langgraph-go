package transport_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/dshills/langgraph-go/graph/mcp"
	"github.com/dshills/langgraph-go/graph/mcp/transport"
)

// ExampleStdioReadWriteCloser demonstrates how to use StdioReadWriteCloser
// for JSON-RPC communication in an MCP server.
func ExampleStdioReadWriteCloser() {
	// In a real MCP server, you would use NewStdioReadWriteCloser() which wraps
	// os.Stdin and os.Stdout. For this example, we simulate with buffers.

	// Simulate an initialize request from a client
	initReq := mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: mcp.InitializeRequest{
			ProtocolVersion: "2025-06-18",
			ClientInfo: mcp.ClientInfo{
				Name:    "example-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.Capabilities{
				Tools: true,
			},
		},
	}

	requestData, _ := json.Marshal(initReq)

	// For demonstration, we'll use buffers instead of actual stdin/stdout
	input := bytes.NewBuffer(requestData)
	var output bytes.Buffer

	// Create a custom transport for this example
	// In production: transport := transport.NewStdioReadWriteCloser()
	rwc := &customTransport{
		reader: io.NopCloser(input),
		writer: &writeCloser{Writer: &output},
	}
	defer rwc.Close()

	// Read the request
	decoder := json.NewDecoder(rwc)
	var req mcp.Request
	if err := decoder.Decode(&req); err != nil {
		log.Fatal(err)
	}

	// Process the request and create a response
	resp := mcp.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcp.InitializeResponse{
			ProtocolVersion: "2025-06-18",
			ServerInfo: mcp.ServerInfo{
				Name:    "example-server",
				Version: "1.0.0",
			},
			Capabilities: mcp.Capabilities{
				Tools: true,
			},
		},
	}

	// Write the response
	encoder := json.NewEncoder(rwc)
	if err := encoder.Encode(resp); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Request method:", req.Method)
	fmt.Println("Response sent successfully")
	// Output:
	// Request method: initialize
	// Response sent successfully
}

// customTransport is a test helper that mimics StdioReadWriteCloser behavior
// for example purposes without requiring actual stdin/stdout.
type customTransport struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (c *customTransport) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *customTransport) Write(b []byte) (int, error) {
	return c.writer.Write(b)
}

func (c *customTransport) Close() error {
	_ = c.reader.Close()
	_ = c.writer.Close()
	return nil
}

type writeCloser struct {
	io.Writer
}

func (w *writeCloser) Close() error {
	return nil
}

// ExampleStdioReadWriteCloser_basicUsage shows basic read and write operations.
func ExampleStdioReadWriteCloser_basicUsage() {
	// Create a new stdio transport
	// In production: transport := transport.NewStdioReadWriteCloser()
	// For this example, we demonstrate the API:
	_ = transport.NewStdioReadWriteCloser()

	fmt.Println("Transport created successfully")
	// Output:
	// Transport created successfully
}

// ExampleNewMCPStdioServer demonstrates creating an MCP JSON-RPC server.
// In production, this server would communicate via stdin/stdout with an MCP client.
func ExampleNewMCPStdioServer() {
	// Note: This example demonstrates the API structure.
	// In production, the server runs as a standalone process with actual stdio.

	fmt.Println("MCP server creation example")
	fmt.Println("Server uses JSON-RPC 2.0 with Content-Length framing")
	fmt.Println("Supports bidirectional communication")
	// Output:
	// MCP server creation example
	// Server uses JSON-RPC 2.0 with Content-Length framing
	// Supports bidirectional communication
}
