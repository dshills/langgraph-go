package mcp

import "encoding/json"

// JSON-RPC 2.0 standard error codes as defined in the specification.
const (
	// ErrCodeParseError indicates invalid JSON was received by the server.
	ErrCodeParseError = -32700

	// ErrCodeInvalidRequest indicates the JSON sent is not a valid Request object.
	ErrCodeInvalidRequest = -32600

	// ErrCodeMethodNotFound indicates the method does not exist or is not available.
	ErrCodeMethodNotFound = -32601

	// ErrCodeInvalidParams indicates invalid method parameter(s).
	ErrCodeInvalidParams = -32602

	// ErrCodeInternalError indicates internal JSON-RPC error.
	ErrCodeInternalError = -32603
)

// Request represents a JSON-RPC 2.0 request message as defined by the MCP specification.
// All MCP requests follow the JSON-RPC 2.0 protocol structure.
type Request struct {
	// JSONRPC specifies the JSON-RPC protocol version. Must be exactly "2.0".
	JSONRPC string `json:"jsonrpc"`

	// ID is a unique identifier for the request. Used to correlate requests with responses.
	// May be a string, number, or null. Omitted for notification requests.
	ID any `json:"id,omitempty"`

	// Method is the name of the method to be invoked.
	Method string `json:"method"`

	// Params holds the parameter values to be used during the invocation of the method.
	// May be omitted if the method requires no parameters.
	Params any `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response message.
// Either Result or Error must be present, but not both.
type Response struct {
	// JSONRPC specifies the JSON-RPC protocol version. Must be exactly "2.0".
	JSONRPC string `json:"jsonrpc"`

	// ID is the identifier from the corresponding request. Must match the request ID.
	ID any `json:"id"`

	// Result contains the result of the method invocation if successful.
	// Must be omitted if there was an error.
	Result any `json:"result,omitempty"`

	// Error contains error information if the method invocation failed.
	// Must be omitted if the method succeeded.
	Error *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	// Code is a number that indicates the error type that occurred.
	// Standard error codes are defined as constants in this package.
	Code int `json:"code"`

	// Message provides a short description of the error.
	Message string `json:"message"`

	// Data contains additional information about the error.
	// May be omitted. The value is defined by the server (e.g. detailed error information, nested errors etc.).
	Data any `json:"data,omitempty"`
}

// Error implements the error interface for Error.
func (e *Error) Error() string {
	if e.Data != nil {
		return e.Message + ": " + anyToString(e.Data)
	}
	return e.Message
}

// anyToString converts any value to string for error messages.
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case error:
		return val.Error()
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return "unable to serialize error data"
		}
		return string(data)
	}
}

// ClientInfo describes the client implementation making the MCP request.
// Used in the initialize method to identify the client.
type ClientInfo struct {
	// Name is the name of the client implementation.
	Name string `json:"name"`

	// Version is the version of the client implementation.
	Version string `json:"version"`
}

// ServerInfo describes the server implementation responding to MCP requests.
// Returned in the initialize response to identify the server.
type ServerInfo struct {
	// Name is the name of the server implementation.
	Name string `json:"name"`

	// Version is the version of the server implementation.
	Version string `json:"version"`
}

// Capabilities describes the optional features supported by a client or server.
// Used during the initialization handshake to negotiate supported features.
type Capabilities struct {
	// Tools indicates whether the implementation supports tool/function calling.
	Tools bool `json:"tools,omitempty"`

	// Resources indicates whether the implementation supports resource access.
	Resources bool `json:"resources,omitempty"`

	// Prompts indicates whether the implementation supports prompt templates.
	Prompts bool `json:"prompts,omitempty"`
}

// InitializeRequest contains the parameters for the initialize method.
// This is the first request sent by a client to establish a connection.
type InitializeRequest struct {
	// ProtocolVersion specifies the MCP protocol version the client supports.
	// Format: "YYYY-MM-DD" (e.g., "2025-06-18").
	ProtocolVersion string `json:"protocolVersion"`

	// ClientInfo describes the client making the request.
	ClientInfo ClientInfo `json:"clientInfo"`

	// Capabilities describes the optional features the client supports.
	Capabilities Capabilities `json:"capabilities"`
}

// InitializeResponse contains the result of a successful initialize request.
// Sent by the server to complete the initialization handshake.
type InitializeResponse struct {
	// ProtocolVersion specifies the MCP protocol version the server supports.
	// Must match the version requested by the client.
	ProtocolVersion string `json:"protocolVersion"`

	// ServerInfo describes the server implementation.
	ServerInfo ServerInfo `json:"serverInfo"`

	// Capabilities describes the optional features the server supports.
	Capabilities Capabilities `json:"capabilities"`
}

// MarshalRequest creates a JSON-RPC 2.0 request with the given parameters.
// This is a convenience function for creating well-formed requests.
func MarshalRequest(id any, method string, params any) ([]byte, error) {
	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	return json.Marshal(req)
}

// MarshalResponse creates a JSON-RPC 2.0 success response with the given result.
// This is a convenience function for creating well-formed responses.
func MarshalResponse(id any, result any) ([]byte, error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

// MarshalErrorResponse creates a JSON-RPC 2.0 error response with the given error.
// This is a convenience function for creating well-formed error responses.
func MarshalErrorResponse(id any, code int, message string, data any) ([]byte, error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(resp)
}

// UnmarshalRequest parses a JSON-RPC 2.0 request from the given data.
// Returns ErrCodeParseError if the JSON is invalid.
func UnmarshalRequest(data []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return Request{}, &Error{
			Code:    ErrCodeParseError,
			Message: "Parse error",
			Data:    err.Error(),
		}
	}

	// Validate required fields
	if req.JSONRPC != "2.0" {
		return Request{}, &Error{
			Code:    ErrCodeInvalidRequest,
			Message: "Invalid Request",
			Data:    "jsonrpc field must be '2.0'",
		}
	}

	if req.Method == "" {
		return Request{}, &Error{
			Code:    ErrCodeInvalidRequest,
			Message: "Invalid Request",
			Data:    "method field is required",
		}
	}

	return req, nil
}

// UnmarshalResponse parses a JSON-RPC 2.0 response from the given data.
// Returns ErrCodeParseError if the JSON is invalid.
func UnmarshalResponse(data []byte) (Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return Response{}, &Error{
			Code:    ErrCodeParseError,
			Message: "Parse error",
			Data:    err.Error(),
		}
	}

	// Validate required fields
	if resp.JSONRPC != "2.0" {
		return Response{}, &Error{
			Code:    ErrCodeInvalidRequest,
			Message: "Invalid Request",
			Data:    "jsonrpc field must be '2.0'",
		}
	}

	return resp, nil
}
