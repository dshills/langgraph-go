package mcp

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestRequestMarshalUnmarshal tests marshaling and unmarshaling of Request messages.
func TestRequestMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name       string
		id         any
		method     string
		params     any
		wantErr    bool
		errCode    int
		errMessage string
	}{
		{
			name:   "request with string ID and params",
			id:     "req-123",
			method: "tools/list",
			params: map[string]string{"filter": "math"},
		},
		{
			name:   "request with numeric ID",
			id:     42,
			method: "initialize",
			params: InitializeRequest{
				ProtocolVersion: "2025-06-18",
				ClientInfo: ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
				Capabilities: Capabilities{
					Tools: true,
				},
			},
		},
		{
			name:   "request with nil ID (notification)",
			id:     nil,
			method: "notifications/cancelled",
			params: nil,
		},
		{
			name:   "request without params",
			id:     "req-456",
			method: "ping",
			params: nil,
		},
		{
			name:   "request with float ID",
			id:     123.456,
			method: "custom/method",
			params: map[string]any{"nested": map[string]int{"value": 10}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the request
			data, err := MarshalRequest(tt.id, tt.method, tt.params)
			if err != nil {
				t.Fatalf("MarshalRequest() error = %v", err)
			}

			// Unmarshal the request
			req, err := UnmarshalRequest(data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				var mcpErr *Error
				if !errors.As(err, &mcpErr) {
					t.Fatalf("expected *Error, got %T", err)
				}
				if mcpErr.Code != tt.errCode {
					t.Errorf("error code = %d, want %d", mcpErr.Code, tt.errCode)
				}
				if mcpErr.Message != tt.errMessage {
					t.Errorf("error message = %q, want %q", mcpErr.Message, tt.errMessage)
				}
				return
			}

			// Verify fields
			if req.JSONRPC != "2.0" {
				t.Errorf("JSONRPC = %q, want %q", req.JSONRPC, "2.0")
			}
			if req.Method != tt.method {
				t.Errorf("Method = %q, want %q", req.Method, tt.method)
			}

			// Compare IDs (handling nil and various types)
			if !compareAny(req.ID, tt.id) {
				t.Errorf("ID = %v, want %v", req.ID, tt.id)
			}

			// Verify params are present when expected
			if tt.params != nil && req.Params == nil {
				t.Error("Params = nil, want non-nil")
			}
		})
	}
}

// TestResponseMarshalUnmarshal tests marshaling and unmarshaling of Response messages.
func TestResponseMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name   string
		id     any
		result any
	}{
		{
			name:   "response with string ID and map result",
			id:     "resp-123",
			result: map[string]string{"status": "success"},
		},
		{
			name: "response with numeric ID",
			id:   42,
			result: InitializeResponse{
				ProtocolVersion: "2025-06-18",
				ServerInfo: ServerInfo{
					Name:    "test-server",
					Version: "1.0.0",
				},
				Capabilities: Capabilities{
					Tools:     true,
					Resources: true,
				},
			},
		},
		{
			name:   "response with nil result",
			id:     "resp-nil",
			result: nil,
		},
		{
			name:   "response with array result",
			id:     999,
			result: []string{"tool1", "tool2", "tool3"},
		},
		{
			name: "response with complex nested result",
			id:   "complex",
			result: map[string]any{
				"tools": []map[string]any{
					{"name": "calculator", "enabled": true},
					{"name": "weather", "enabled": false},
				},
				"count": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the response
			data, err := MarshalResponse(tt.id, tt.result)
			if err != nil {
				t.Fatalf("MarshalResponse() error = %v", err)
			}

			// Unmarshal the response
			resp, err := UnmarshalResponse(data)
			if err != nil {
				t.Fatalf("UnmarshalResponse() error = %v", err)
			}

			// Verify fields
			if resp.JSONRPC != "2.0" {
				t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, "2.0")
			}
			if !compareAny(resp.ID, tt.id) {
				t.Errorf("ID = %v, want %v", resp.ID, tt.id)
			}
			if resp.Error != nil {
				t.Errorf("Error = %+v, want nil", resp.Error)
			}

			// Verify result is present
			if tt.result != nil && resp.Result == nil {
				t.Error("Result = nil, want non-nil")
			}
		})
	}
}

// TestErrorResponseMarshalUnmarshal tests marshaling and unmarshaling of error responses.
func TestErrorResponseMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		id      any
		code    int
		message string
		data    any
	}{
		{
			name:    "parse error",
			id:      nil,
			code:    ErrCodeParseError,
			message: "Parse error",
			data:    "unexpected token at position 5",
		},
		{
			name:    "method not found",
			id:      "req-404",
			code:    ErrCodeMethodNotFound,
			message: "Method not found",
			data:    nil,
		},
		{
			name:    "invalid params",
			id:      42,
			code:    ErrCodeInvalidParams,
			message: "Invalid params",
			data:    map[string]string{"field": "toolId", "issue": "required"},
		},
		{
			name:    "internal error",
			id:      "internal-err",
			code:    ErrCodeInternalError,
			message: "Internal error",
			data:    "database connection failed",
		},
		{
			name:    "custom error code",
			id:      999,
			code:    -32000,
			message: "Custom server error",
			data:    []string{"reason1", "reason2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the error response
			data, err := MarshalErrorResponse(tt.id, tt.code, tt.message, tt.data)
			if err != nil {
				t.Fatalf("MarshalErrorResponse() error = %v", err)
			}

			// Unmarshal the response
			resp, err := UnmarshalResponse(data)
			if err != nil {
				t.Fatalf("UnmarshalResponse() error = %v", err)
			}

			// Verify fields
			if resp.JSONRPC != "2.0" {
				t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, "2.0")
			}
			if !compareAny(resp.ID, tt.id) {
				t.Errorf("ID = %v, want %v", resp.ID, tt.id)
			}
			if resp.Result != nil {
				t.Errorf("Result = %v, want nil", resp.Result)
			}

			// Verify error object
			if resp.Error == nil {
				t.Fatal("Error = nil, want non-nil")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("Error.Code = %d, want %d", resp.Error.Code, tt.code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("Error.Message = %q, want %q", resp.Error.Message, tt.message)
			}

			// Verify data field
			if tt.data != nil && resp.Error.Data == nil {
				t.Error("Error.Data = nil, want non-nil")
			}
			if tt.data == nil && resp.Error.Data != nil {
				t.Errorf("Error.Data = %v, want nil", resp.Error.Data)
			}
		})
	}
}

// TestErrorStruct tests the Error struct creation and serialization.
func TestErrorStruct(t *testing.T) {
	tests := []struct {
		name    string
		err     *Error
		wantStr string
	}{
		{
			name: "error with all fields",
			err: &Error{
				Code:    ErrCodeParseError,
				Message: "Parse error",
				Data:    "invalid JSON",
			},
			wantStr: `{"code":-32700,"message":"Parse error","data":"invalid JSON"}`,
		},
		{
			name: "error without data",
			err: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: "Method not found",
			},
			wantStr: `{"code":-32601,"message":"Method not found"}`,
		},
		{
			name: "error with structured data",
			err: &Error{
				Code:    ErrCodeInvalidParams,
				Message: "Invalid params",
				Data:    map[string]string{"field": "name", "reason": "required"},
			},
			wantStr: `{"code":-32602,"message":"Invalid params","data":{"field":"name","reason":"required"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the error
			data, err := json.Marshal(tt.err)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Compare JSON (order-independent)
			var got, want map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantStr), &want); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}

			if !compareMaps(got, want) {
				t.Errorf("Error JSON = %s, want %s", data, tt.wantStr)
			}
		})
	}
}

// TestInitializeRequestSerialization tests InitializeRequest marshaling/unmarshaling.
func TestInitializeRequestSerialization(t *testing.T) {
	tests := []struct {
		name string
		req  InitializeRequest
	}{
		{
			name: "basic initialization",
			req: InitializeRequest{
				ProtocolVersion: "2025-06-18",
				ClientInfo: ClientInfo{
					Name:    "langgraph-go",
					Version: "0.1.0",
				},
				Capabilities: Capabilities{
					Tools: true,
				},
			},
		},
		{
			name: "full capabilities",
			req: InitializeRequest{
				ProtocolVersion: "2025-06-18",
				ClientInfo: ClientInfo{
					Name:    "advanced-client",
					Version: "2.0.0",
				},
				Capabilities: Capabilities{
					Tools:     true,
					Resources: true,
					Prompts:   true,
				},
			},
		},
		{
			name: "minimal capabilities",
			req: InitializeRequest{
				ProtocolVersion: "2025-06-18",
				ClientInfo: ClientInfo{
					Name:    "basic-client",
					Version: "0.0.1",
				},
				Capabilities: Capabilities{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Unmarshal back
			var got InitializeRequest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify fields
			if got.ProtocolVersion != tt.req.ProtocolVersion {
				t.Errorf("ProtocolVersion = %q, want %q", got.ProtocolVersion, tt.req.ProtocolVersion)
			}
			if got.ClientInfo.Name != tt.req.ClientInfo.Name {
				t.Errorf("ClientInfo.Name = %q, want %q", got.ClientInfo.Name, tt.req.ClientInfo.Name)
			}
			if got.ClientInfo.Version != tt.req.ClientInfo.Version {
				t.Errorf("ClientInfo.Version = %q, want %q", got.ClientInfo.Version, tt.req.ClientInfo.Version)
			}
			if got.Capabilities.Tools != tt.req.Capabilities.Tools {
				t.Errorf("Capabilities.Tools = %v, want %v", got.Capabilities.Tools, tt.req.Capabilities.Tools)
			}
			if got.Capabilities.Resources != tt.req.Capabilities.Resources {
				t.Errorf("Capabilities.Resources = %v, want %v", got.Capabilities.Resources, tt.req.Capabilities.Resources)
			}
			if got.Capabilities.Prompts != tt.req.Capabilities.Prompts {
				t.Errorf("Capabilities.Prompts = %v, want %v", got.Capabilities.Prompts, tt.req.Capabilities.Prompts)
			}
		})
	}
}

// TestInitializeResponseSerialization tests InitializeResponse marshaling/unmarshaling.
func TestInitializeResponseSerialization(t *testing.T) {
	tests := []struct {
		name string
		resp InitializeResponse
	}{
		{
			name: "basic server response",
			resp: InitializeResponse{
				ProtocolVersion: "2025-06-18",
				ServerInfo: ServerInfo{
					Name:    "langgraph-mcp-server",
					Version: "1.0.0",
				},
				Capabilities: Capabilities{
					Tools: true,
				},
			},
		},
		{
			name: "full capabilities server",
			resp: InitializeResponse{
				ProtocolVersion: "2025-06-18",
				ServerInfo: ServerInfo{
					Name:    "full-feature-server",
					Version: "3.2.1",
				},
				Capabilities: Capabilities{
					Tools:     true,
					Resources: true,
					Prompts:   true,
				},
			},
		},
		{
			name: "minimal server",
			resp: InitializeResponse{
				ProtocolVersion: "2025-06-18",
				ServerInfo: ServerInfo{
					Name:    "minimal-server",
					Version: "0.1.0-beta",
				},
				Capabilities: Capabilities{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// Unmarshal back
			var got InitializeResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify fields
			if got.ProtocolVersion != tt.resp.ProtocolVersion {
				t.Errorf("ProtocolVersion = %q, want %q", got.ProtocolVersion, tt.resp.ProtocolVersion)
			}
			if got.ServerInfo.Name != tt.resp.ServerInfo.Name {
				t.Errorf("ServerInfo.Name = %q, want %q", got.ServerInfo.Name, tt.resp.ServerInfo.Name)
			}
			if got.ServerInfo.Version != tt.resp.ServerInfo.Version {
				t.Errorf("ServerInfo.Version = %q, want %q", got.ServerInfo.Version, tt.resp.ServerInfo.Version)
			}
			if got.Capabilities.Tools != tt.resp.Capabilities.Tools {
				t.Errorf("Capabilities.Tools = %v, want %v", got.Capabilities.Tools, tt.resp.Capabilities.Tools)
			}
			if got.Capabilities.Resources != tt.resp.Capabilities.Resources {
				t.Errorf("Capabilities.Resources = %v, want %v", got.Capabilities.Resources, tt.resp.Capabilities.Resources)
			}
			if got.Capabilities.Prompts != tt.resp.Capabilities.Prompts {
				t.Errorf("Capabilities.Prompts = %v, want %v", got.Capabilities.Prompts, tt.resp.Capabilities.Prompts)
			}
		})
	}
}

// TestInvalidJSON tests handling of invalid JSON in unmarshal functions.
func TestInvalidJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		unmarshal func([]byte) (any, error)
		wantCode  int
	}{
		{
			name: "invalid request JSON",
			json: `{"jsonrpc": "2.0", "method": "test", invalid}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantCode: ErrCodeParseError,
		},
		{
			name: "invalid response JSON",
			json: `{"jsonrpc": "2.0", "id": 1, "result": {broken`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalResponse(data)
			},
			wantCode: ErrCodeParseError,
		},
		{
			name: "completely malformed JSON",
			json: `not json at all`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantCode: ErrCodeParseError,
		},
		{
			name: "empty JSON",
			json: ``,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantCode: ErrCodeParseError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.unmarshal([]byte(tt.json))
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var mcpErr *Error
			if !errors.As(err, &mcpErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}

			if mcpErr.Code != tt.wantCode {
				t.Errorf("error code = %d, want %d", mcpErr.Code, tt.wantCode)
			}
		})
	}
}

// TestProtocolVersionValidation tests validation of the jsonrpc version field.
func TestProtocolVersionValidation(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		unmarshal func([]byte) (any, error)
		wantErr   bool
		wantCode  int
	}{
		{
			name: "missing jsonrpc in request",
			json: `{"method": "test"}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
		},
		{
			name: "wrong jsonrpc version in request",
			json: `{"jsonrpc": "1.0", "method": "test"}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
		},
		{
			name: "missing jsonrpc in response",
			json: `{"id": 1, "result": {}}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalResponse(data)
			},
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
		},
		{
			name: "wrong jsonrpc version in response",
			json: `{"jsonrpc": "3.0", "id": 1, "result": {}}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalResponse(data)
			},
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
		},
		{
			name: "valid jsonrpc version",
			json: `{"jsonrpc": "2.0", "method": "test"}`,
			unmarshal: func(data []byte) (any, error) {
				return UnmarshalRequest(data)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.unmarshal([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				var mcpErr *Error
				if !errors.As(err, &mcpErr) {
					t.Fatalf("expected *Error, got %T: %v", err, err)
				}

				if mcpErr.Code != tt.wantCode {
					t.Errorf("error code = %d, want %d", mcpErr.Code, tt.wantCode)
				}
			}
		})
	}
}

// TestRequiredFieldValidation tests validation of required fields in requests.
func TestRequiredFieldValidation(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantErr  bool
		wantCode int
		wantData string
	}{
		{
			name:     "missing method field",
			json:     `{"jsonrpc": "2.0", "id": 1}`,
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
			wantData: "method field is required",
		},
		{
			name:     "empty method field",
			json:     `{"jsonrpc": "2.0", "id": 1, "method": ""}`,
			wantErr:  true,
			wantCode: ErrCodeInvalidRequest,
			wantData: "method field is required",
		},
		{
			name:    "valid request with all required fields",
			json:    `{"jsonrpc": "2.0", "id": 1, "method": "test"}`,
			wantErr: false,
		},
		{
			name:    "valid request without optional ID",
			json:    `{"jsonrpc": "2.0", "method": "notification"}`,
			wantErr: false,
		},
		{
			name:    "valid request without optional params",
			json:    `{"jsonrpc": "2.0", "id": 1, "method": "ping"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalRequest([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				var mcpErr *Error
				if !errors.As(err, &mcpErr) {
					t.Fatalf("expected *Error, got %T: %v", err, err)
				}

				if mcpErr.Code != tt.wantCode {
					t.Errorf("error code = %d, want %d", mcpErr.Code, tt.wantCode)
				}

				if dataStr, ok := mcpErr.Data.(string); ok {
					if dataStr != tt.wantData {
						t.Errorf("error data = %q, want %q", dataStr, tt.wantData)
					}
				} else if tt.wantData != "" {
					t.Errorf("error data type = %T, want string with %q", mcpErr.Data, tt.wantData)
				}
			}
		})
	}
}

// TestErrorInterface tests the Error type's error interface implementation.
func TestErrorInterface(t *testing.T) {
	tests := []struct {
		name    string
		err     *Error
		wantMsg string
	}{
		{
			name: "error with string data",
			err: &Error{
				Code:    ErrCodeParseError,
				Message: "Parse error",
				Data:    "invalid JSON at position 10",
			},
			wantMsg: "Parse error: invalid JSON at position 10",
		},
		{
			name: "error without data",
			err: &Error{
				Code:    ErrCodeMethodNotFound,
				Message: "Method not found",
			},
			wantMsg: "Method not found",
		},
		{
			name: "error with map data",
			err: &Error{
				Code:    ErrCodeInvalidParams,
				Message: "Invalid params",
				Data:    map[string]string{"field": "name", "reason": "required"},
			},
			wantMsg: "Invalid params: {\"field\":\"name\",\"reason\":\"required\"}",
		},
		{
			name: "error with array data",
			err: &Error{
				Code:    ErrCodeInternalError,
				Message: "Internal error",
				Data:    []string{"reason1", "reason2"},
			},
			wantMsg: "Internal error: [\"reason1\",\"reason2\"]",
		},
		{
			name: "error with number data",
			err: &Error{
				Code:    -32000,
				Message: "Custom error",
				Data:    42,
			},
			wantMsg: "Custom error: 42",
		},
		{
			name: "error with bool data",
			err: &Error{
				Code:    -32001,
				Message: "Boolean error",
				Data:    true,
			},
			wantMsg: "Boolean error: true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}

			// Verify it can be used as error interface
			var err error = tt.err
			if err.Error() != tt.wantMsg {
				t.Errorf("error interface Error() = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

// TestErrorAsError tests that Error can be used with errors.As.
func TestErrorAsError(t *testing.T) {
	mcpErr := &Error{
		Code:    ErrCodeMethodNotFound,
		Message: "Method not found",
		Data:    "tools/execute",
	}

	// Test errors.As
	var targetErr *Error
	if !errors.As(mcpErr, &targetErr) {
		t.Fatal("errors.As failed to match *Error")
	}

	if targetErr.Code != mcpErr.Code {
		t.Errorf("Code = %d, want %d", targetErr.Code, mcpErr.Code)
	}
	if targetErr.Message != mcpErr.Message {
		t.Errorf("Message = %q, want %q", targetErr.Message, mcpErr.Message)
	}
}

// TestErrorWithErrorData tests Error with error type as Data field.
func TestErrorWithErrorData(t *testing.T) {
	innerErr := errors.New("connection refused")
	err := &Error{
		Code:    ErrCodeInternalError,
		Message: "Internal error",
		Data:    innerErr,
	}

	wantMsg := "Internal error: connection refused"
	if err.Error() != wantMsg {
		t.Errorf("Error() = %q, want %q", err.Error(), wantMsg)
	}
}

// TestErrorWithUnserializableData tests Error with data that cannot be JSON serialized.
func TestErrorWithUnserializableData(t *testing.T) {
	// Create a channel, which cannot be JSON serialized
	unserialized := make(chan int)
	err := &Error{
		Code:    ErrCodeInternalError,
		Message: "Internal error",
		Data:    unserialized,
	}

	gotMsg := err.Error()
	wantMsg := "Internal error: unable to serialize error data"
	if gotMsg != wantMsg {
		t.Errorf("Error() = %q, want %q", gotMsg, wantMsg)
	}
}

// Helper function to compare any values, handling JSON number conversions.
func compareAny(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle JSON number conversions (int to float64)
	aFloat, aIsNum := a.(float64)
	bFloat, bIsNum := b.(float64)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}

	// Handle int to float64 comparison
	if aIsNum {
		if bInt, ok := b.(int); ok {
			return aFloat == float64(bInt)
		}
	}
	if bIsNum {
		if aInt, ok := a.(int); ok {
			return bFloat == float64(aInt)
		}
	}

	return a == b
}

// Helper function to deep compare maps.
func compareMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}

		// Recursively compare nested maps
		if aMap, aIsMap := v.(map[string]any); aIsMap {
			if bMap, bIsMap := bv.(map[string]any); bIsMap {
				if !compareMaps(aMap, bMap) {
					return false
				}
				continue
			}
			return false
		}

		if !compareAny(v, bv) {
			return false
		}
	}

	return true
}
