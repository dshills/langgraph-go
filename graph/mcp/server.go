package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/tool"
	"github.com/sourcegraph/jsonrpc2"
)

// Server lifecycle states represent the operational state of the MCP server.
const (
	// StateUninitialized indicates the server has been created but not started.
	StateUninitialized int32 = iota

	// StateInitializing indicates the server is performing startup initialization.
	StateInitializing

	// StateRunning indicates the server is actively processing requests.
	StateRunning

	// StateStopped indicates the server has been stopped and cannot be restarted.
	StateStopped
)

// Server errors for resource registration and lifecycle management.
var (
	// ErrServerNotRunning is returned when attempting operations that require
	// the server to be in a specific state.
	ErrServerNotRunning = errors.New("server is not running")
)

// Placeholder types for capabilities not yet fully implemented.
// These will be expanded in later phases with full schema definitions.

// ToolMetadata contains metadata about a registered tool.
// This includes description, input schema, and other properties
// needed for tool discovery and validation.
type ToolMetadata struct {
	// Description provides a human-readable explanation of what the tool does.
	Description string `json:"description"`

	// Schema defines the input parameters expected by the tool.
	// This should be a JSON Schema object describing the tool's input structure.
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// MCPServer defines the interface for an MCP protocol server implementation.
//
// The server manages the lifecycle of MCP communication, including:
// - Starting and stopping the server.
// - Registering tools, resources, and prompt templates.
// - Handling client initialization and capability negotiation.
// - Processing JSON-RPC requests.
//
// Implementations must enforce lifecycle state transitions and thread safety.
//
// Example usage:
//
//	config := ServerConfig{
//	    Name:    "weather-server",
//	    Version: "1.0.0",
//	    Emitter: myEmitter,
//	}
//	server := NewServer(config)
//
//	// Register capabilities
//	server.RegisterTool("get_weather", weatherTool, ToolMetadata{
//	    Description: "Get current weather for a location",
//	    Schema: map[string]interface{}{
//	        "type": "object",
//	        "properties": map[string]interface{}{
//	            "location": map[string]interface{}{"type": "string"},
//	        },
//	        "required": []string{"location"},
//	    },
//	})
//
//	// Start server
//	if err := server.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Stop()
type MCPServer interface {
	// Start begins serving MCP requests and blocks until the context is cancelled
	// or an error occurs.
	//
	// Start performs the following:
	// - Transitions from Uninitialized to Initializing state.
	// - Sets up transport layer (stdin/stdout by default).
	// - Registers JSON-RPC method handlers.
	// - Transitions to Running state after initialization completes.
	// - Blocks until ctx.Done() or fatal error.
	//
	// Start can only be called once. Subsequent calls return an error.
	//
	// Parameters:
	//   - ctx: context for server lifetime management and cancellation.
	//
	// Returns:
	//   - error: nil on clean shutdown, error if startup or operation fails.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server and releases all resources.
	//
	// Stop performs the following:
	// - Transitions to Stopped state.
	// - Closes transport connections.
	// - Flushes observability events if Emitter is configured.
	// - Releases all registered capabilities.
	//
	// Stop can be called multiple times safely. Subsequent calls are no-ops.
	//
	// Returns:
	//   - error: nil on success, error if shutdown fails.
	Stop() error

	// RegisterTool registers a tool implementation with metadata for MCP clients.
	//
	// Tools enable LLM clients to perform actions via the server. Each tool must
	// have a unique name and provide metadata describing its purpose and parameters.
	//
	// RegisterTool should be called before Start() to ensure tools are available
	// during client initialization. Tools registered after Start() may not be
	// visible to already-connected clients.
	//
	// Parameters:
	//   - name: unique tool identifier (must match tool.Tool.Name()).
	//   - tool: tool implementation that executes the tool logic.
	//   - metadata: descriptive information about the tool and its parameters.
	//
	// Returns:
	//   - error: nil on success, error if name conflicts or server is stopped.
	RegisterTool(name string, tool tool.Tool, metadata ToolMetadata) error

	// RegisterResource is deprecated. Use RegisterStaticResource or RegisterDynamicResource instead.
	//
	// Deprecated: Use RegisterStaticResource for fixed content or RegisterDynamicResource for computed content.
	RegisterResource(uri string, resource Resource) error

	// RegisterStaticResource registers a resource with fixed content.
	//
	// Static resources have content that is fixed at registration time and never changes.
	// Use this for cached data, configuration files, or any content that doesn't need
	// to be computed on-demand.
	//
	// RegisterStaticResource should be called before Start() to ensure resources are
	// available during client initialization.
	//
	// Parameters:
	//   - uri: unique resource identifier (must match pattern ^[a-z][a-z0-9_/]*$).
	//   - name: human-readable resource name.
	//   - description: resource description for clients.
	//   - mimeType: MIME type of the content (e.g., "application/json", "text/plain").
	//   - content: the resource content as bytes.
	//
	// Returns:
	//   - error: nil on success, error if URI conflicts, invalid, or content exceeds size limit.
	RegisterStaticResource(uri, name, description, mimeType string, content []byte) error

	// RegisterDynamicResource registers a resource with computed content.
	//
	// Dynamic resources compute content on-demand using a generator function. Use this
	// for live metrics, current state, or any content that changes over time.
	//
	// The generator function is called each time a client reads the resource and should
	// respect context cancellation.
	//
	// RegisterDynamicResource should be called before Start() to ensure resources are
	// available during client initialization.
	//
	// Parameters:
	//   - uri: unique resource identifier (must match pattern ^[a-z][a-z0-9_/]*$).
	//   - name: human-readable resource name.
	//   - description: resource description for clients.
	//   - mimeType: MIME type of the content (e.g., "application/json", "text/plain").
	//   - generator: function that computes content on-demand, receives context for cancellation.
	//
	// Returns:
	//   - error: nil on success, error if URI conflicts, invalid, or generator is nil.
	RegisterDynamicResource(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) error

	// RegisterPrompt registers a prompt template that clients can invoke.
	//
	// Prompt templates provide reusable prompts with parameter substitution,
	// enabling clients to generate consistent prompts for common tasks.
	//
	// RegisterPrompt should be called before Start() to ensure prompts are
	// available during client initialization.
	//
	// Parameters:
	//   - template: prompt template implementation with name and rendering logic.
	//
	// Returns:
	//   - error: nil on success, error if name conflicts or server is stopped.
	RegisterPrompt(template PromptTemplate) error
}

// ServerConfig contains configuration for creating an MCP server instance.
type ServerConfig struct {
	// Name is the human-readable server name exposed to clients.
	// Example: "langgraph-weather", "database-tools".
	Name string

	// Version is the semantic version of the server implementation.
	// Example: "1.0.0", "2.1.3-beta".
	Version string

	// Emitter is an optional observability backend for emitting server events.
	// If nil, no events are emitted (useful for testing or minimal setups).
	Emitter emit.Emitter
}

// ConnectionSession represents an active MCP client connection.
//
// For stdio transport, there is typically one connection session per server instance.
// This struct tracks metadata about the connected client for observability and debugging.
type ConnectionSession struct {
	// ClientInfo contains information about the connected client (from initialize request).
	ClientInfo map[string]interface{}

	// ProtocolVersion is the MCP protocol version negotiated during initialization.
	ProtocolVersion string

	// Capabilities lists the client's declared capabilities (if provided).
	Capabilities map[string]interface{}

	// ConnectionTime records when the connection was established.
	ConnectionTime int64 // Unix timestamp in nanoseconds
}

// server is the default implementation of the MCPServer interface.
//
// It manages server lifecycle, capability registration, and JSON-RPC request handling
// with thread-safe state management.
type server struct {
	// config holds the server name, version, and optional emitter.
	config ServerConfig

	// state tracks the current lifecycle state (Uninitialized → Initializing → Running → Stopped).
	// Uses atomic operations for thread-safe state transitions.
	state atomic.Int32

	// mu protects toolRegistry, resourceRegistry, and promptRegistry from concurrent access.
	mu sync.RWMutex

	// toolRegistry maps tool names to their implementations and metadata.
	toolRegistry map[string]toolRegistration

	// toolAdapter is the adapter for tool invocation and validation.
	toolAdapter ToolRegistry

	// resourceProvider manages resource registration and access.
	resourceProvider ResourceProvider

	// promptRegistry manages prompt templates with thread-safe operations.
	promptRegistry *PromptRegistry

	// transport manages JSON-RPC communication (stdin/stdout by default).
	transport *jsonrpc2.Conn

	// ctx is the server's root context, cancelled when Stop() is called.
	ctx context.Context

	// cancel cancels the server's root context to trigger shutdown.
	cancel context.CancelFunc

	// session tracks the current connection session for observability (T064-T065).
	// For stdio transport, this represents the single client connection.
	session   *ConnectionSession
	sessionMu sync.RWMutex
}

// toolRegistration bundles a tool implementation with its metadata.
type toolRegistration struct {
	tool     tool.Tool
	metadata ToolMetadata
}

// NewServer creates a new MCP server with the given configuration.
//
// The server starts in Uninitialized state and must be started with Start().
// All capabilities (tools, resources, prompts) should be registered before calling Start().
//
// Parameters:
//   - config: server configuration with name, version, and optional emitter.
//
// Returns:
//   - MCPServer: configured server ready to register capabilities and start.
func NewServer(config ServerConfig) MCPServer {
	// Create tool adapter with optional emitter
	var toolAdapterOpts []RegistryOption
	if config.Emitter != nil {
		toolAdapterOpts = append(toolAdapterOpts, WithEmitter(config.Emitter))
	}

	// Create resource provider with optional emitter
	var resourceProviderOpts []ResourceProviderOption
	if config.Emitter != nil {
		resourceProviderOpts = append(resourceProviderOpts, WithResourceEmitter(config.Emitter))
	}

	s := &server{
		config:           config,
		toolRegistry:     make(map[string]toolRegistration),
		toolAdapter:      NewToolRegistry(toolAdapterOpts...),
		resourceProvider: NewResourceProvider(resourceProviderOpts...),
		promptRegistry:   NewPromptRegistry(),
	}
	s.state.Store(StateUninitialized)
	return s
}

// Start begins serving MCP requests and blocks until the context is cancelled.
func (s *server) Start(ctx context.Context) error {
	// Enforce state transition: can only start from Uninitialized state
	if !s.state.CompareAndSwap(StateUninitialized, StateInitializing) {
		currentState := s.state.Load()
		switch currentState {
		case StateInitializing:
			return errors.New("server is already initializing")
		case StateRunning:
			return errors.New("server is already running")
		case StateStopped:
			return errors.New("server has been stopped and cannot be restarted")
		default:
			return fmt.Errorf("server is in invalid state: %d", currentState)
		}
	}

	// Create server context with cancellation
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Emit server start event if emitter is configured
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "server_start",
			Meta: map[string]interface{}{
				"server_name":    s.config.Name,
				"server_version": s.config.Version,
			},
		})
	}

	// Set up JSON-RPC handler with method routing
	handler := jsonrpc2.HandlerWithError(s.handleRequest)

	// Create stdio transport (default for MCP servers)
	var err error
	s.transport, err = s.createTransport(s.ctx, handler)
	if err != nil {
		s.state.Store(StateStopped)
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Transition to running state
	s.state.Store(StateRunning)

	// Block until context is cancelled
	<-s.ctx.Done()

	// Graceful shutdown
	return s.Stop()
}

// Stop gracefully shuts down the server and releases all resources.
func (s *server) Stop() error {
	// Can stop from any state except already stopped
	currentState := s.state.Load()
	if currentState == StateStopped {
		return nil // Already stopped, no-op
	}

	// Transition to stopped state
	s.state.Store(StateStopped)

	// Cancel server context
	if s.cancel != nil {
		s.cancel()
	}

	// Emit client_disconnect event if session exists (T070)
	s.sessionMu.RLock()
	hasSession := s.session != nil
	s.sessionMu.RUnlock()

	if hasSession && s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "client_disconnect",
			Meta: map[string]interface{}{
				"server_name": s.config.Name,
			},
		})
	}

	// Close transport connection
	var transportErr error
	if s.transport != nil {
		transportErr = s.transport.Close()
	}

	// Flush observability events
	var flushErr error
	if s.config.Emitter != nil {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*1000000000) // 5 seconds
		defer cancel()

		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "server_stop",
			Meta: map[string]interface{}{
				"server_name": s.config.Name,
			},
		})

		flushErr = s.config.Emitter.Flush(flushCtx)
	}

	// Combine errors if multiple failures occurred
	return errors.Join(transportErr, flushErr)
}

// RegisterTool registers a tool implementation with metadata.
func (s *server) RegisterTool(name string, t tool.Tool, metadata ToolMetadata) error {
	// Prevent registration after server is stopped
	if s.state.Load() == StateStopped {
		return errors.New("cannot register tool: server is stopped")
	}

	// Validate tool name matches
	if t.Name() != name {
		return fmt.Errorf("tool name mismatch: registry name %q != tool.Name() %q", name, t.Name())
	}

	// Convert server ToolMetadata to adapter ToolAdapterMetadata
	adapterMetadata := ToolAdapterMetadata{
		Name:        name,
		Description: metadata.Description,
		InputSchema: metadata.Schema,
	}

	// Register with tool adapter (validates according to MCP spec)
	if err := s.toolAdapter.Register(t, adapterMetadata); err != nil {
		return fmt.Errorf("failed to register tool with adapter: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store in server registry for tracking
	s.toolRegistry[name] = toolRegistration{
		tool:     t,
		metadata: metadata,
	}

	return nil
}

// RegisterResource is deprecated. Use RegisterStaticResource or RegisterDynamicResource instead.
func (s *server) RegisterResource(uri string, resource Resource) error {
	return errors.New("RegisterResource is deprecated, use RegisterStaticResource or RegisterDynamicResource")
}

// RegisterStaticResource registers a resource with fixed content.
func (s *server) RegisterStaticResource(uri, name, description, mimeType string, content []byte) error {
	// Prevent registration after server is stopped
	if s.state.Load() == StateStopped {
		return ErrServerNotRunning
	}

	// Delegate to resource provider
	return s.resourceProvider.RegisterStatic(uri, name, description, mimeType, content)
}

// RegisterDynamicResource registers a resource with computed content.
func (s *server) RegisterDynamicResource(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) error {
	// Prevent registration after server is stopped
	if s.state.Load() == StateStopped {
		return ErrServerNotRunning
	}

	// Delegate to resource provider
	return s.resourceProvider.RegisterDynamic(uri, name, description, mimeType, generator)
}

// RegisterPrompt registers a prompt template that clients can invoke.
func (s *server) RegisterPrompt(template PromptTemplate) error {
	// Prevent registration after server is stopped
	if s.state.Load() == StateStopped {
		return errors.New("cannot register prompt: server is stopped")
	}

	// Delegate to PromptRegistry.Register (which handles validation and thread safety)
	return s.promptRegistry.Register(template)
}

// createTransport sets up the JSON-RPC transport layer for the server.
// Currently uses stdio transport as per MCP specification.
func (s *server) createTransport(ctx context.Context, handler jsonrpc2.Handler) (*jsonrpc2.Conn, error) {
	// For now, we create a basic connection. Full stdio transport setup
	// would use transport.NewMCPStdioServer but we need direct access to conn.
	// This will be refined when integrating with transport package.
	return nil, errors.New("transport creation not yet implemented - will be completed in transport integration phase")
}

// handleRequest routes JSON-RPC requests to appropriate method handlers.
func (s *server) handleRequest(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
	// Route by method name
	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)
	case "tools/list":
		return s.handleToolsList(ctx, req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "resources/list":
		return s.handleResourcesList(ctx, req)
	case "resources/read":
		return s.handleResourcesRead(ctx, req)
	case "prompts/list":
		return s.handlePromptsList(ctx, req)
	case "prompts/get":
		return s.handlePromptsGet(ctx, req)
	default:
		return nil, &jsonrpc2.Error{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("method %q not found", req.Method),
		}
	}
}

// handleInitialize processes the initialize request and performs capability negotiation.
//
// This is the first request sent by clients to establish the MCP connection.
// The server responds with its capabilities and transitions to Running state.
func (s *server) handleInitialize(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure we're in initializing state (can re-initialize from running for reconnections)
	currentState := s.state.Load()
	if currentState != StateInitializing && currentState != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    ErrCodeInternalError,
			Message: "server not ready for initialization",
		}
	}

	// Parse initialize request parameters
	var initReq InitializeRequest
	if req.Params != nil {
		paramBytes, err := json.Marshal(req.Params)
		if err != nil {
			return nil, &jsonrpc2.Error{
				Code:    ErrCodeInvalidParams,
				Message: "failed to parse initialize parameters",
			}
		}
		if err := json.Unmarshal(paramBytes, &initReq); err != nil {
			return nil, &jsonrpc2.Error{
				Code:    ErrCodeInvalidParams,
				Message: "invalid initialize request format",
			}
		}
	}

	// Create connection session (T064-T065)
	connectionTime := time.Now().UnixNano()
	if ctxTime := ctx.Value("connection_time"); ctxTime != nil {
		if t, ok := ctxTime.(int64); ok {
			connectionTime = t
		}
	}

	s.sessionMu.Lock()
	s.session = &ConnectionSession{
		ClientInfo: map[string]interface{}{
			"name":    initReq.ClientInfo.Name,
			"version": initReq.ClientInfo.Version,
		},
		ProtocolVersion: initReq.ProtocolVersion,
		Capabilities: map[string]interface{}{
			"tools":     initReq.Capabilities.Tools,
			"resources": initReq.Capabilities.Resources,
			"prompts":   initReq.Capabilities.Prompts,
		},
		ConnectionTime: connectionTime,
	}
	s.sessionMu.Unlock()

	// Emit client_connect event (T070)
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "client_connect",
			Meta: map[string]interface{}{
				"protocol_version": initReq.ProtocolVersion,
				"client_name":      initReq.ClientInfo.Name,
				"client_version":   initReq.ClientInfo.Version,
				"client_capabilities": map[string]bool{
					"tools":     initReq.Capabilities.Tools,
					"resources": initReq.Capabilities.Resources,
					"prompts":   initReq.Capabilities.Prompts,
				},
			},
		})
	}

	// Determine server capabilities based on registrations
	s.mu.RLock()
	hasResources := len(s.resourceProvider.List()) > 0
	hasPrompts := len(s.promptRegistry.List()) > 0
	capabilities := Capabilities{
		Tools:     len(s.toolRegistry) > 0,
		Resources: hasResources,
		Prompts:   hasPrompts,
	}
	s.mu.RUnlock()

	// Transition to running state if coming from initializing
	if currentState == StateInitializing {
		s.state.Store(StateRunning)
	}

	// Build initialize response
	response := InitializeResponse{
		ProtocolVersion: initReq.ProtocolVersion,
		ServerInfo: ServerInfo{
			Name:    s.config.Name,
			Version: s.config.Version,
		},
		Capabilities: capabilities,
	}

	return response, nil
}

// Stub handlers for other MCP methods (to be implemented in later phases)

func (s *server) handleToolsList(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Validate params (should be empty or omitted)
	if req.Params != nil {
		// Check if params is an empty object
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: failed to parse parameters",
			}
		}

		var paramsMap map[string]interface{}
		if err := json.Unmarshal(paramsBytes, &paramsMap); err == nil && len(paramsMap) > 0 {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"received": req.Params,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: tools/list does not accept parameters",
				Data:    &rawData,
			}
		}
	}

	// Get tools from adapter
	toolMetadataList := s.toolAdapter.List()

	// Convert to MCP tool list format
	type ToolListItem struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		InputSchema map[string]interface{} `json:"inputSchema"`
	}

	type ToolsListResponse struct {
		Tools []ToolListItem `json:"tools"`
	}

	tools := make([]ToolListItem, 0, len(toolMetadataList))
	for _, meta := range toolMetadataList {
		tools = append(tools, ToolListItem{
			Name:        meta.Name,
			Description: meta.Description,
			InputSchema: meta.InputSchema,
		})
	}

	return ToolsListResponse{Tools: tools}, nil
}

func (s *server) handleToolsCall(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Parse request parameters
	type ToolCallParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	var params ToolCallParams
	if req.Params == nil {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: missing required parameter 'name'",
		}
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: failed to parse parameters",
		}
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"error": err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: invalid parameter format",
			Data:    &rawData,
		}
	}

	// Validate tool name is provided
	if params.Name == "" {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: missing required parameter 'name'",
		}
	}

	// Default to empty arguments if not provided
	if params.Arguments == nil {
		params.Arguments = make(map[string]interface{})
	}

	// Invoke tool through adapter
	result, err := s.toolAdapter.Invoke(ctx, params.Name, params.Arguments)
	if err != nil {
		// Check if it's already an MCP error
		if mcpErr, ok := err.(*Error); ok {
			// Convert MCP Error to jsonrpc2.Error
			var rawData *json.RawMessage
			if mcpErr.Data != nil {
				dataBytes, _ := json.Marshal(mcpErr.Data)
				raw := json.RawMessage(dataBytes)
				rawData = &raw
			}
			return nil, &jsonrpc2.Error{
				Code:    int64(mcpErr.Code),
				Message: mcpErr.Message,
				Data:    rawData,
			}
		}

		// Check for context errors
		if errors.Is(err, context.Canceled) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"toolName": params.Name,
				"error":    "context canceled",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: "Internal error: tool execution cancelled",
				Data:    &rawData,
			}
		}

		if errors.Is(err, context.DeadlineExceeded) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"toolName": params.Name,
				"error":    "context deadline exceeded",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: "Internal error: tool execution timeout",
				Data:    &rawData,
			}
		}

		// Generic internal error
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"toolName": params.Name,
			"error":    err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "Internal error: tool execution failed",
			Data:    &rawData,
		}
	}

	// Return tool result in MCP format
	return result, nil
}

func (s *server) handleResourcesList(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Validate params (should be empty or omitted)
	if req.Params != nil {
		// Check if params is an empty object
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: failed to parse parameters",
			}
		}

		var paramsMap map[string]interface{}
		if err := json.Unmarshal(paramsBytes, &paramsMap); err == nil && len(paramsMap) > 0 {
			// Params is a non-empty object - reject
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"received": paramsMap,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: resources/list does not accept parameters",
				Data:    &rawData,
			}
		}
	}

	// Get list of resources from provider
	resources := s.resourceProvider.List()

	// Format result according to MCP spec
	return MarshalResourcesListResult(resources)
}

func (s *server) handleResourcesRead(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Parse parameters
	var params ResourcesReadParams
	if req.Params == nil {
		rawData := json.RawMessage([]byte(`{"parameter":"uri","received":null}`))
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: 'uri' parameter is required",
			Data:    &rawData,
		}
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: failed to parse parameters",
		}
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"error": err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: invalid parameter format",
			Data:    &rawData,
		}
	}

	// Validate URI parameter
	if params.URI == "" {
		rawData := json.RawMessage([]byte(`{"parameter":"uri","received":""}`))
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: 'uri' parameter is required",
			Data:    &rawData,
		}
	}

	// Read resource content
	content, err := s.resourceProvider.Read(ctx, params.URI)
	if err != nil {
		// Check for specific error types
		if errors.Is(err, ErrResourceNotFound) {
			// Get list of available resources for error message
			availableResources := s.resourceProvider.List()
			uris := make([]string, 0, len(availableResources))
			for _, r := range availableResources {
				uris = append(uris, r.URI)
			}
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"uri":                params.URI,
				"availableResources": uris,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: fmt.Sprintf("Invalid params: resource '%s' not found", params.URI),
				Data:    &rawData,
			}
		}

		// Check for size limit errors
		if errors.Is(err, ErrResourceTooLarge) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"uri":   params.URI,
				"error": err.Error(),
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: "Internal error: resource exceeds size limit",
				Data:    &rawData,
			}
		}

		// Check for context errors
		if errors.Is(err, context.Canceled) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"uri":   params.URI,
				"error": "context canceled",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: "Internal error: resource read cancelled",
				Data:    &rawData,
			}
		}

		if errors.Is(err, context.DeadlineExceeded) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"uri":     params.URI,
				"error":   "context deadline exceeded",
				"timeout": "5s",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: fmt.Sprintf("Internal error: failed to read resource '%s'", params.URI),
				Data:    &rawData,
			}
		}

		// Generic internal error
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"uri":   params.URI,
			"error": err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: fmt.Sprintf("Internal error: failed to read resource '%s'", params.URI),
			Data:    &rawData,
		}
	}

	// Get resource metadata for MIME type
	resource, err := s.resourceProvider.Get(params.URI)
	if err != nil {
		// This shouldn't happen since Read succeeded, but handle defensively
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "Internal error: resource metadata not found",
		}
	}

	// Format result according to MCP spec
	return MarshalResourcesReadResult(params.URI, resource.MimeType(), content)
}

// T085 [US3] Implement prompts/list handler
//
// handlePromptsList retrieves the list of all registered prompt templates.
//
// MCP Spec: Returns array of prompts with name, description, and parameter metadata.
// Contract: contracts/prompt-registry.md#prompts/list
func (s *server) handlePromptsList(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Validate params (should be empty or omitted)
	if req.Params != nil {
		// Check if params is an empty object
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: failed to parse parameters",
			}
		}

		var paramsMap map[string]interface{}
		if err := json.Unmarshal(paramsBytes, &paramsMap); err == nil && len(paramsMap) > 0 {
			// Params is a non-empty object - reject
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"received": paramsMap,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: "Invalid params: prompts/list does not accept parameters",
				Data:    &rawData,
			}
		}
	}

	// Emit prompt_list_start event (T089)
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "prompt_list_start",
			Meta:   map[string]interface{}{},
		})
	}

	// Get list of prompts from registry
	s.mu.RLock()
	promptInfos := s.promptRegistry.List()
	s.mu.RUnlock()

	// Emit prompt_list_end event (T089)
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "prompt_list_end",
			Meta: map[string]interface{}{
				"prompt_count": len(promptInfos),
			},
		})
	}

	// Format result according to MCP spec
	// Response schema: {"prompts": [...]}
	type PromptsListResponse struct {
		Prompts []PromptInfo `json:"prompts"`
	}

	return PromptsListResponse{Prompts: promptInfos}, nil
}

// T086 [US3] Implement prompts/get handler
//
// handlePromptsGet renders a prompt template with provided argument values.
//
// MCP Spec: Substitutes {{parameter}} placeholders and returns rendered messages.
// Contract: contracts/prompt-registry.md#prompts/get
func (s *server) handlePromptsGet(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	// Ensure server is running
	if s.state.Load() != StateRunning {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: "server not ready",
		}
	}

	// Parse request parameters
	type PromptsGetParams struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}

	var params PromptsGetParams
	if req.Params == nil {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: missing required parameter 'name'",
		}
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: failed to parse parameters",
		}
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"error": err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: invalid parameter format",
			Data:    &rawData,
		}
	}

	// Validate prompt name is provided
	if params.Name == "" {
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInvalidParams),
			Message: "Invalid params: missing required parameter 'name'",
		}
	}

	// Default to empty arguments if not provided
	if params.Arguments == nil {
		params.Arguments = make(map[string]string)
	}

	// Emit prompt_render_start event (T089)
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "prompt_render_start",
			Meta: map[string]interface{}{
				"prompt_name":    params.Name,
				"argument_count": len(params.Arguments),
			},
		})
	}

	// Render prompt through registry
	s.mu.RLock()
	rendered, err := s.promptRegistry.Render(params.Name, params.Arguments)
	s.mu.RUnlock()

	if err != nil {
		// Emit prompt_render_error event (T089)
		if s.config.Emitter != nil {
			s.config.Emitter.Emit(emit.Event{
				RunID:  "server",
				NodeID: s.config.Name,
				Msg:    "prompt_render_error",
				Meta: map[string]interface{}{
					"prompt_name": params.Name,
					"error":       err.Error(),
				},
			})
		}
		// Check for specific error types
		if errors.Is(err, ErrPromptNotFound) {
			// Get list of available prompts for error message
			s.mu.RLock()
			availablePrompts := s.promptRegistry.List()
			s.mu.RUnlock()

			promptNames := make([]string, 0, len(availablePrompts))
			for _, p := range availablePrompts {
				promptNames = append(promptNames, p.Name)
			}

			dataBytes, _ := json.Marshal(map[string]interface{}{
				"promptName":       params.Name,
				"availablePrompts": promptNames,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: fmt.Sprintf("Invalid params: prompt '%s' not found", params.Name),
				Data:    &rawData,
			}
		}

		// Check for missing required parameter (T088 - enhanced error messages)
		if errors.Is(err, ErrMissingParameter) {
			// Extract parameter name from error message
			paramName := extractParamName(err.Error())
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"promptName": params.Name,
				"parameter":  paramName,
				"required":   true,
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInvalidParams),
				Message: fmt.Sprintf("Invalid params: missing required parameter '%s'", paramName),
				Data:    &rawData,
			}
		}

		// Check for context errors
		if errors.Is(err, context.Canceled) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"promptName": params.Name,
				"error":      "context canceled",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: "Internal error: prompt rendering cancelled",
				Data:    &rawData,
			}
		}

		if errors.Is(err, context.DeadlineExceeded) {
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"promptName": params.Name,
				"error":      "context deadline exceeded",
			})
			rawData := json.RawMessage(dataBytes)
			return nil, &jsonrpc2.Error{
				Code:    int64(ErrCodeInternalError),
				Message: fmt.Sprintf("Internal error: failed to render prompt '%s'", params.Name),
				Data:    &rawData,
			}
		}

		// Generic internal error
		dataBytes, _ := json.Marshal(map[string]interface{}{
			"promptName": params.Name,
			"error":      err.Error(),
		})
		rawData := json.RawMessage(dataBytes)
		return nil, &jsonrpc2.Error{
			Code:    int64(ErrCodeInternalError),
			Message: fmt.Sprintf("Internal error: failed to render prompt '%s'", params.Name),
			Data:    &rawData,
		}
	}

	// Emit prompt_render_complete event (T089)
	if s.config.Emitter != nil {
		s.config.Emitter.Emit(emit.Event{
			RunID:  "server",
			NodeID: s.config.Name,
			Msg:    "prompt_render_complete",
			Meta: map[string]interface{}{
				"prompt_name":    params.Name,
				"message_count":  len(rendered.Messages),
				"argument_count": len(params.Arguments),
			},
		})
	}

	// Return rendered prompt (format matches RenderedPrompt struct)
	return rendered, nil
}

// extractParamName extracts the parameter name from a "missing required parameter: X" error message.
// Returns empty string if parameter name cannot be extracted.
func extractParamName(errMsg string) string {
	// Error format from Render(): "missing required parameter: workflow_id"
	const prefix = "missing required parameter: "
	if idx := indexOfString(errMsg, prefix); idx >= 0 {
		return errMsg[idx+len(prefix):]
	}
	return ""
}

// indexOfString returns the index of the first occurrence of substr in s, or -1 if not found.
func indexOfString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
