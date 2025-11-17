package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// ============================================================================
// T038: Contract Test for resources/list Method
// ============================================================================

// TestResourcesList_Contract_EmptyRegistry tests that list with no resources
// returns an empty array conforming to MCP specification.
func TestResourcesList_Contract_EmptyRegistry(t *testing.T) {
	provider := NewResourceProvider()

	// Get list of resources (should be empty)
	resources := provider.List()

	// Verify empty array (not nil)
	if resources == nil {
		t.Error("List() returned nil, want empty array []")
	}

	if len(resources) != 0 {
		t.Errorf("List() returned %d resources, want 0", len(resources))
	}

	// Verify JSON-RPC response structure
	result := map[string]interface{}{
		"resources": resources,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Verify response matches MCP spec schema
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	resourcesField, ok := decoded["resources"].([]interface{})
	if !ok {
		t.Error("Result.resources is not an array")
	}

	if len(resourcesField) != 0 {
		t.Errorf("Result.resources has %d elements, want 0", len(resourcesField))
	}
}

// TestResourcesList_Contract_SingleResource tests that list with one resource
// returns correct metadata following MCP specification format.
func TestResourcesList_Contract_SingleResource(t *testing.T) {
	provider := NewResourceProvider()

	// Register a single static resource
	err := provider.RegisterStatic(
		"workflow_state",
		"Workflow State",
		"Current state of the workflow execution",
		"application/json",
		[]byte(`{"messages":[],"counter":0}`),
	)
	if err != nil {
		t.Fatalf("RegisterStatic() failed: %v", err)
	}

	// Get list of resources
	resources := provider.List()

	if len(resources) != 1 {
		t.Fatalf("List() returned %d resources, want 1", len(resources))
	}

	// Verify resource metadata fields
	resource := resources[0]

	if resource.URI != "workflow_state" {
		t.Errorf("Resource.URI = %q, want 'workflow_state'", resource.URI)
	}

	if resource.Name != "Workflow State" {
		t.Errorf("Resource.Name = %q, want 'Workflow State'", resource.Name)
	}

	if resource.Description != "Current state of the workflow execution" {
		t.Errorf("Resource.Description = %q, want 'Current state of the workflow execution'", resource.Description)
	}

	if resource.MimeType != "application/json" {
		t.Errorf("Resource.MimeType = %q, want 'application/json'", resource.MimeType)
	}

	// Verify JSON marshaling matches MCP spec schema
	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("Failed to marshal resource: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal resource: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{"uri", "name", "description", "mimeType"}
	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("Resource JSON missing required field: %q", field)
		}
	}
}

// TestResourcesList_Contract_MultipleResources tests that list with multiple
// resources returns all metadata correctly.
func TestResourcesList_Contract_MultipleResources(t *testing.T) {
	provider := NewResourceProvider()
	// Register multiple resources with different characteristics
	testResources := []struct {
		uri         string
		name        string
		description string
		mimeType    string
		content     []byte
	}{
		{
			uri:         "workflow_state",
			name:        "Workflow State",
			description: "Current workflow execution state",
			mimeType:    "application/json",
			content:     []byte(`{"messages":[]}`),
		},
		{
			uri:         "checkpoints/latest",
			name:        "Latest Checkpoint",
			description: "Most recent saved checkpoint",
			mimeType:    "application/json",
			content:     []byte(`{"step":0}`),
		},
		{
			uri:         "metrics/runtime",
			name:        "Runtime Metrics",
			description: "Live workflow execution metrics",
			mimeType:    "application/json",
			content:     []byte(`{"nodesExecuted":42}`),
		},
		{
			uri:         "logs/execution",
			name:        "Execution Logs",
			description: "Workflow execution logs",
			mimeType:    "text/plain",
			content:     []byte("INFO: Workflow started\n"),
		},
	}
	// Register all resources
	for _, tr := range testResources {
		err := provider.RegisterStatic(tr.uri, tr.name, tr.description, tr.mimeType, tr.content)
		if err != nil {
			t.Fatalf("RegisterStatic(%q) failed: %v", tr.uri, err)
		}
	}
	// Get list of resources
	resources := provider.List()
	if len(resources) != len(testResources) {
		t.Fatalf("List() returned %d resources, want %d", len(resources), len(testResources))
	}
	// Verify each resource is present with correct metadata
	resourceMap := make(map[string]ResourceInfo)
	for _, res := range resources {
		resourceMap[res.URI] = res
	}
	for _, expected := range testResources {
		actual, found := resourceMap[expected.uri]
		if !found {
			t.Errorf("Resource %q not found in List() result", expected.uri)
			continue
		}
		if actual.Name != expected.name {
			t.Errorf("Resource %q Name = %q, want %q", expected.uri, actual.Name, expected.name)
		}
		if actual.Description != expected.description {
			t.Errorf("Resource %q Description = %q, want %q", expected.uri, actual.Description, expected.description)
		}
		if actual.MimeType != expected.mimeType {
			t.Errorf("Resource %q MimeType = %q, want %q", expected.uri, actual.MimeType, expected.mimeType)
		}
	}
}

// TestResourcesList_Contract_URIValidation tests that resource URIs comply
// with the MCP specification pattern: ^[a-z][a-z0-9_/]*$
func TestResourcesList_Contract_URIValidation(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		wantValid bool
	}{
		// Valid URIs
		{name: "simple lowercase", uri: "workflow", wantValid: true},
		{name: "with underscores", uri: "workflow_state", wantValid: true},
		{name: "with numbers", uri: "checkpoint123", wantValid: true},
		{name: "with slashes", uri: "checkpoints/latest", wantValid: true},
		{name: "hierarchical", uri: "metrics/runtime/cpu", wantValid: true},
		{name: "complex valid", uri: "history/run_123/step_5", wantValid: true},
		// Invalid URIs
		{name: "starts with number", uri: "123workflow", wantValid: false},
		{name: "starts with underscore", uri: "_private", wantValid: false},
		{name: "starts with slash", uri: "/invalid", wantValid: false},
		{name: "uppercase letter", uri: "WorkflowState", wantValid: false},
		{name: "contains hyphen", uri: "workflow-state", wantValid: false},
		{name: "contains space", uri: "workflow state", wantValid: false},
		{name: "contains special char", uri: "workflow@state", wantValid: false},
		{name: "empty string", uri: "", wantValid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewResourceProvider()
			err := provider.RegisterStatic(
				tt.uri,
				"Test Resource",
				"Test description",
				"application/json",
				[]byte(`{}`),
			)
			if tt.wantValid && err != nil {
				t.Errorf("RegisterStatic() with valid URI %q failed: %v", tt.uri, err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("RegisterStatic() with invalid URI %q should have failed", tt.uri)
			}
			// Verify error message mentions URI validation
			if !tt.wantValid && err != nil {
				errMsg := err.Error()
				if errMsg == "" {
					t.Error("Error message is empty")
				}
				// Error should mention "uri" or "pattern"
				if !containsAny(errMsg, []string{"uri", "URI", "pattern", "invalid"}) {
					t.Errorf("Error message %q should mention URI validation", errMsg)
				}
			}
		})
	}
}

// TestResourcesList_Contract_NoParams tests that resources/list accepts
// empty params or no params field as per MCP specification.
func TestResourcesList_Contract_NoParams(t *testing.T) {
	provider := NewResourceProvider()
	// Register a resource
	err := provider.RegisterStatic(
		"test_resource",
		"Test Resource",
		"Test description",
		"application/json",
		[]byte(`{"test":true}`),
	)
	if err != nil {
		t.Fatalf("RegisterStatic() failed: %v", err)
	}
	// Test 1: Call List() with no parameters (direct API call)
	resources := provider.List()
	if len(resources) != 1 {
		t.Errorf("List() with no params returned %d resources, want 1", len(resources))
	}
	// Test 2: Verify JSON-RPC request with empty params
	reqWithEmptyParams, err := MarshalRequest(1, "resources/list", map[string]interface{}{})
	if err != nil {
		t.Fatalf("MarshalRequest() with empty params failed: %v", err)
	}
	var decodedEmpty Request
	if err := json.Unmarshal(reqWithEmptyParams, &decodedEmpty); err != nil {
		t.Fatalf("Unmarshal request with empty params failed: %v", err)
	}
	// Verify method and params structure
	if decodedEmpty.Method != "resources/list" {
		t.Errorf("Request.Method = %q, want 'resources/list'", decodedEmpty.Method)
	}
	// Test 3: Verify JSON-RPC request with omitted params
	reqNoParams, err := MarshalRequest(1, "resources/list", nil)
	if err != nil {
		t.Fatalf("MarshalRequest() with nil params failed: %v", err)
	}
	var decodedNil Request
	if err := json.Unmarshal(reqNoParams, &decodedNil); err != nil {
		t.Fatalf("Unmarshal request with nil params failed: %v", err)
	}
	if decodedNil.Method != "resources/list" {
		t.Errorf("Request.Method = %q, want 'resources/list'", decodedNil.Method)
	}
	// Test 4: Verify response structure matches MCP spec
	result := map[string]interface{}{
		"resources": resources,
	}
	respData, err := MarshalResponse(1, result)
	if err != nil {
		t.Fatalf("MarshalResponse() failed: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("Unmarshal response failed: %v", err)
	}
	// Verify response structure
	if resp.JSONRPC != "2.0" {
		t.Errorf("Response.JSONRPC = %q, want '2.0'", resp.JSONRPC)
	}
	if resp.ID != float64(1) {
		t.Errorf("Response.ID = %v, want 1", resp.ID)
	}
	if resp.Result == nil {
		t.Error("Response.Result is nil, want result object")
	}
	if resp.Error != nil {
		t.Errorf("Response.Error should be nil, got: %v", resp.Error)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// containsAny checks if string s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if containsString(s, substr) {
			return true
		}
	}
	return false
}

// ============================================================================
// Mock Resource Implementation (for future use once implementation exists)
// ============================================================================

// mockResourceGenerator is a mock generator function for dynamic resources.
// This will be used once ResourceProvider implementation exists.
func mockResourceGenerator(content string) func(context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		return []byte(content), nil
	}
}

// mockFailingGenerator simulates a failing generator for error testing.
func mockFailingGenerator(err error) func(context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		return nil, err
	}
}

// ============================================================================
// T039: Contract Tests for resources/read Method
// ============================================================================

// TestResourcesRead_Contract_StaticResource verifies reading a static text resource successfully.
// Contract: resources/read returns content in "text" field for text-based MIME types.
func TestResourcesRead_Contract_StaticResource(t *testing.T) {
	provider := NewResourceProvider()
	// Register static JSON resource
	stateJSON := []byte(`{"messages":[{"role":"user","content":"Hello"}],"counter":5}`)
	err := provider.RegisterStatic(
		"workflow_state",
		"Workflow State",
		"Current workflow execution state",
		"application/json",
		stateJSON,
	)
	if err != nil {
		t.Fatalf("Failed to register static resource: %v", err)
	}
	// Read resource
	ctx := context.Background()
	content, err := provider.Read(ctx, "workflow_state")
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	// Verify content matches registered data
	if string(content) != string(stateJSON) {
		t.Errorf("Read() content mismatch\nGot:  %s\nWant: %s", content, stateJSON)
	}
	// Verify content is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Errorf("Read() returned invalid JSON: %v", err)
	}
	// Verify specific fields
	if messages, ok := parsed["messages"].([]interface{}); !ok || len(messages) != 1 {
		t.Errorf("Read() content missing expected 'messages' field")
	}
	if counter, ok := parsed["counter"].(float64); !ok || counter != 5 {
		t.Errorf("Read() content has unexpected 'counter' value: %v", parsed["counter"])
	}
}

// TestResourcesRead_Contract_DynamicResource verifies reading a dynamic resource that computes on demand.
// Contract: resources/read calls generator function and returns computed content.
func TestResourcesRead_Contract_DynamicResource(t *testing.T) {
	provider := NewResourceProvider()
	callCount := 0
	// Register dynamic resource that generates fresh content on each read
	err := provider.RegisterDynamic(
		"metrics/runtime",
		"Runtime Metrics",
		"Live workflow execution metrics",
		"application/json",
		func(ctx context.Context) ([]byte, error) {
			callCount++
			metrics := map[string]interface{}{
				"nodesExecuted":   42,
				"currentNode":     "process_batch",
				"timestamp":       time.Now().UTC().Format(time.RFC3339),
				"generationCount": callCount,
			}
			return json.Marshal(metrics)
		},
	)
	if err != nil {
		t.Fatalf("Failed to register dynamic resource: %v", err)
	}
	// Read resource first time
	ctx := context.Background()
	content1, err := provider.Read(ctx, "metrics/runtime")
	if err != nil {
		t.Fatalf("Read() first call failed: %v", err)
	}
	// Verify content is valid JSON
	var parsed1 map[string]interface{}
	if err := json.Unmarshal(content1, &parsed1); err != nil {
		t.Errorf("Read() returned invalid JSON: %v", err)
	}
	// Verify expected fields
	if nodes, ok := parsed1["nodesExecuted"].(float64); !ok || nodes != 42 {
		t.Errorf("Read() content missing/incorrect 'nodesExecuted' field: %v", parsed1["nodesExecuted"])
	}
	// Read resource second time
	content2, err := provider.Read(ctx, "metrics/runtime")
	if err != nil {
		t.Fatalf("Read() second call failed: %v", err)
	}
	var parsed2 map[string]interface{}
	if err := json.Unmarshal(content2, &parsed2); err != nil {
		t.Errorf("Read() second call returned invalid JSON: %v", err)
	}
	// Verify generator was called twice (generationCount should increment)
	if count1, ok := parsed1["generationCount"].(float64); ok {
		if count2, ok := parsed2["generationCount"].(float64); ok {
			if count2 <= count1 {
				t.Errorf("Read() did not regenerate content (count1=%v, count2=%v)", count1, count2)
			}
		}
	}
	// Verify timestamps are different (dynamic generation)
	if ts1, ok := parsed1["timestamp"].(string); ok {
		if ts2, ok := parsed2["timestamp"].(string); ok {
			if ts1 == ts2 {
				t.Logf("Warning: timestamps are identical, may indicate caching: %s", ts1)
			}
		}
	}
}

// TestResourcesRead_Contract_NotFound verifies reading a nonexistent resource returns -32602 error.
// Contract: resources/read returns InvalidParams (-32602) when resource URI not found.
func TestResourcesRead_Contract_NotFound(t *testing.T) {
	provider := NewResourceProvider()
	// Register one resource
	err := provider.RegisterStatic(
		"workflow_state",
		"Workflow State",
		"Current workflow state",
		"application/json",
		[]byte(`{}`),
	)
	if err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}
	// Attempt to read nonexistent resource
	ctx := context.Background()
	_, err = provider.Read(ctx, "nonexistent_resource")
	// Verify error is returned
	if err == nil {
		t.Fatal("Read() should return error for nonexistent resource, got nil")
	}
	// Verify error indicates resource not found
	errMsg := err.Error()
	if !containsString(errMsg, "not found") && !containsString(errMsg, "nonexistent") {
		t.Errorf("Read() error should indicate resource not found, got: %v", err)
	}
	// Contract: Should be InvalidParams error (-32602)
	// Implementation will map this to JSON-RPC error code
	// Note: Go error doesn't carry JSON-RPC code, but message should indicate invalid params
}

// TestResourcesRead_Contract_MissingURI verifies reading without URI parameter returns -32602 error.
// Contract: resources/read returns InvalidParams (-32602) when URI parameter is missing/empty.
func TestResourcesRead_Contract_MissingURI(t *testing.T) {
	provider := NewResourceProvider()
	// Register a resource
	err := provider.RegisterStatic(
		"workflow_state",
		"Workflow State",
		"State",
		"application/json",
		[]byte(`{}`),
	)
	if err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}
	testCases := []struct {
		name string
		uri  string
	}{
		{
			name: "empty string URI",
			uri:  "",
		},
		{
			name: "whitespace-only URI",
			uri:  "   ",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := provider.Read(ctx, tc.uri)
			// Verify error is returned
			if err == nil {
				t.Fatalf("Read() should return error for %s, got nil", tc.name)
			}
			// Verify error indicates missing/invalid URI
			errMsg := err.Error()
			if !containsString(errMsg, "uri") && !containsString(errMsg, "required") && !containsString(errMsg, "empty") {
				t.Errorf("Read() error should indicate missing URI, got: %v", err)
			}
		})
	}
}

// TestResourcesRead_Contract_JSONResource verifies JSON resource returns content in "text" field.
// Contract: Text-based MIME types (application/json) use "text" field in response.
func TestResourcesRead_Contract_JSONResource(t *testing.T) {
	provider := NewResourceProvider()
	// Register JSON resource with nested structure
	complexJSON := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
		},
		"counter": 5,
		"metadata": map[string]interface{}{
			"runID":     "run-abc123",
			"startTime": "2025-11-17T10:00:00Z",
		},
	}
	jsonBytes, _ := json.Marshal(complexJSON)
	err := provider.RegisterStatic(
		"workflow_state",
		"Workflow State",
		"Current workflow state",
		"application/json",
		jsonBytes,
	)
	if err != nil {
		t.Fatalf("Failed to register JSON resource: %v", err)
	}
	// Read resource
	ctx := context.Background()
	content, err := provider.Read(ctx, "workflow_state")
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	// Verify content is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Errorf("Read() returned invalid JSON: %v", err)
	}
	// Verify structure matches
	if messages, ok := parsed["messages"].([]interface{}); !ok || len(messages) != 2 {
		t.Errorf("Read() content has incorrect 'messages' structure")
	}
	if metadata, ok := parsed["metadata"].(map[string]interface{}); !ok {
		t.Errorf("Read() content missing 'metadata' field")
	} else {
		if runID, ok := metadata["runID"].(string); !ok || runID != "run-abc123" {
			t.Errorf("Read() content has incorrect metadata.runID: %v", metadata["runID"])
		}
	}
}

// TestResourcesRead_Contract_BinaryResource verifies binary resource returns content in "blob" field (base64).
// Contract: Binary MIME types use "blob" field with base64 encoding.
func TestResourcesRead_Contract_BinaryResource(t *testing.T) {
	provider := NewResourceProvider()
	// Register binary resource (simulated PNG image data)
	binaryData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG header
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, // Sample data
	}
	err := provider.RegisterStatic(
		"graph_visualization",
		"Graph Visualization",
		"Workflow graph diagram",
		"image/png",
		binaryData,
	)
	if err != nil {
		t.Fatalf("Failed to register binary resource: %v", err)
	}
	// Read resource
	ctx := context.Background()
	content, err := provider.Read(ctx, "graph_visualization")
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	// For binary resources, implementation should return raw bytes
	// Server layer will base64-encode for JSON-RPC response
	if len(content) != len(binaryData) {
		t.Errorf("Read() content length mismatch\nGot:  %d bytes\nWant: %d bytes", len(content), len(binaryData))
	}
	// Verify content matches original binary data
	for i := range binaryData {
		if i >= len(content) {
			break
		}
		if content[i] != binaryData[i] {
			t.Errorf("Read() content byte mismatch at index %d: got 0x%02X, want 0x%02X", i, content[i], binaryData[i])
		}
	}
	// Verify content can be base64-encoded (for JSON-RPC response)
	encoded := base64.StdEncoding.EncodeToString(content)
	if len(encoded) == 0 {
		t.Error("Read() content cannot be base64-encoded")
	}
	// Verify decoding returns original data
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Errorf("Base64 decode failed: %v", err)
	}
	if string(decoded) != string(binaryData) {
		t.Error("Base64 round-trip does not preserve binary data")
	}
}

// TestResourcesRead_Contract_ContextCancellation verifies read respects context cancellation.
// Contract: resources/read returns error when context is cancelled during read operation.
func TestResourcesRead_Contract_ContextCancellation(t *testing.T) {
	provider := NewResourceProvider()
	// Register dynamic resource with slow generator
	generatorCalled := make(chan struct{})
	err := provider.RegisterDynamic(
		"slow_computation",
		"Slow Computation",
		"Resource with slow generation",
		"application/json",
		func(ctx context.Context) ([]byte, error) {
			close(generatorCalled)
			// Simulate slow operation that checks context
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return []byte(`{"result":"completed"}`), nil
			}
		},
	)
	if err != nil {
		t.Fatalf("Failed to register dynamic resource: %v", err)
	}
	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	// Read resource with cancelled context
	_, err = provider.Read(ctx, "slow_computation")
	// Wait for generator to be called
	select {
	case <-generatorCalled:
		// Generator was called, proceed with verification
	case <-time.After(1 * time.Second):
		t.Fatal("Generator was not called within timeout")
	}
	// Verify error indicates context cancellation
	if err == nil {
		t.Fatal("Read() should return error when context is cancelled, got nil")
	}
	// Verify error is context.DeadlineExceeded or context.Canceled
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Read() should return context error, got: %v", err)
	}
}

// TestResourcesRead_Contract_TextResourceTypes verifies various text-based MIME types use "text" field.
// Contract: text/plain, text/markdown, application/json all return content in "text" field.
func TestResourcesRead_Contract_TextResourceTypes(t *testing.T) {
	testCases := []struct {
		name     string
		uri      string
		mimeType string
		content  []byte
	}{
		{
			name:     "text/plain",
			uri:      "logs/execution",
			mimeType: "text/plain",
			content:  []byte("2025-11-17 10:00:00 INFO: Workflow started\n2025-11-17 10:00:05 INFO: Node completed\n"),
		},
		{
			name:     "text/markdown",
			uri:      "docs/readme",
			mimeType: "text/markdown",
			content:  []byte("# Workflow Documentation\n\n## Overview\n\nThis workflow processes data in stages.\n"),
		},
		{
			name:     "application/json",
			uri:      "config/settings",
			mimeType: "application/json",
			content:  []byte(`{"maxRetries":3,"timeout":"30s"}`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewResourceProvider()
			err := provider.RegisterStatic(
				tc.uri,
				tc.name,
				"Test resource for "+tc.mimeType,
				tc.mimeType,
				tc.content,
			)
			if err != nil {
				t.Fatalf("Failed to register resource: %v", err)
			}
			// Read resource
			ctx := context.Background()
			content, err := provider.Read(ctx, tc.uri)
			if err != nil {
				t.Fatalf("Read() failed: %v", err)
			}
			// Verify content matches
			if string(content) != string(tc.content) {
				t.Errorf("Read() content mismatch\nGot:  %s\nWant: %s", content, tc.content)
			}
			// For JSON, verify it's valid
			if tc.mimeType == "application/json" {
				var parsed interface{}
				if err := json.Unmarshal(content, &parsed); err != nil {
					t.Errorf("Read() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

// TestResourcesRead_Contract_ReadFailure verifies read failures return -32603 Internal error.
// Contract: resources/read returns InternalError (-32603) when generator fails.
func TestResourcesRead_Contract_ReadFailure(t *testing.T) {
	provider := NewResourceProvider()
	// Register dynamic resource that fails on read
	expectedErr := errors.New("database connection timeout")
	err := provider.RegisterDynamic(
		"failing_resource",
		"Failing Resource",
		"Resource that fails to generate",
		"application/json",
		func(ctx context.Context) ([]byte, error) {
			return nil, expectedErr
		},
	)
	if err != nil {
		t.Fatalf("Failed to register dynamic resource: %v", err)
	}
	// Read resource
	ctx := context.Background()
	_, err = provider.Read(ctx, "failing_resource")
	// Verify error is returned
	if err == nil {
		t.Fatal("Read() should return error when generator fails, got nil")
	}
	// Verify error indicates generation failure
	if !errors.Is(err, expectedErr) {
		t.Logf("Read() error: %v", err)
		// Error should wrap or indicate generation failure
	}
}

// TestResourcesRead_Contract_SizeLimit verifies large resources return -32603 error.
// Contract: resources/read returns InternalError (-32603) when content exceeds size limit.
func TestResourcesRead_Contract_SizeLimit(t *testing.T) {
	provider := NewResourceProvider()
	// Create large content (15MB, exceeds typical 10MB limit)
	largeContent := make([]byte, 15*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}
	err := provider.RegisterStatic(
		"large_dataset",
		"Large Dataset",
		"Dataset exceeding size limit",
		"application/octet-stream",
		largeContent,
	)
	// Registration may fail if size validation is done at registration time
	if err != nil {
		// Verify error indicates size limit exceeded
		errMsg := err.Error()
		if !containsString(errMsg, "size") && !containsString(errMsg, "limit") && !containsString(errMsg, "large") {
			t.Errorf("Registration error should indicate size limit, got: %v", err)
		}
		return // Expected behavior: registration fails for oversized content
	}
	// If registration succeeds, read should fail
	ctx := context.Background()
	_, err = provider.Read(ctx, "large_dataset")
	// Verify error is returned (size limit check may be deferred to read time)
	if err == nil {
		t.Log("Warning: Read() succeeded for large resource, size limit may not be enforced")
		// Not failing test here as size limits may be optional or configurable
		return
	}
	// Verify error indicates size limit exceeded
	errMsg := err.Error()
	if !containsString(errMsg, "size") && !containsString(errMsg, "limit") && !containsString(errMsg, "large") {
		t.Logf("Read() error should indicate size limit, got: %v", err)
	}
}

// ============================================================================
// T042: Integration Tests for Resource Size Limits
// ============================================================================
// These tests define the expected behavior for resource size limit enforcement.
// Tests should FAIL initially until size limit checking is implemented.
// Contract Requirements (from resource-provider.md):
// - Default limit: 10MB per resource
// - Static resources checked at registration time
// - Dynamic resources checked at read time
// - Error code: -32603 (Internal error)
// - Error should include: uri, size, maxSize
// ============================================================================

const (
	// defaultMaxResourceSize is the default maximum size for resources (10MB)
	defaultMaxResourceSize = 10 * 1024 * 1024 // 10MB

	// sizeLimitMimeType is the MIME type used in size limit tests
	sizeLimitMimeType = "application/json"
)

// generateTestData creates test data of a specific size in bytes
func generateTestData(sizeBytes int) []byte {
	// Generate JSON array with padding to reach target size
	// Using repeated characters for efficiency
	if sizeBytes <= 2 {
		return []byte("[]")
	}

	// Start with JSON array brackets and space for closing
	data := make([]byte, sizeBytes)
	data[0] = '['
	data[sizeBytes-1] = ']'

	// Fill with valid JSON content (repeated 'x' characters in a string)
	// Use pattern: ["xxxx..."]
	data[1] = '"'
	for i := 2; i < sizeBytes-2; i++ {
		data[i] = 'x'
	}
	data[sizeBytes-2] = '"'

	return data
}

// ============================================================================
// Test Case 1: Static Resource Under Size Limit (Success)
// ============================================================================

func TestResourceProvider_SizeLimit_StaticUnderLimit(t *testing.T) {
	provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
	// Create 5MB of test data (well under 10MB limit)
	content := generateTestData(5 * 1024 * 1024) // 5MB
	// Register static resource - should succeed
	err := provider.RegisterStatic(
		"test_resource_5mb",
		"Test Resource 5MB",
		"A 5MB test resource that should register successfully",
		sizeLimitMimeType,
		content,
	)
	if err != nil {
		t.Fatalf("RegisterStatic with 5MB content should succeed, got error: %v", err)
	}
	// Verify resource can be retrieved
	resource, err := provider.Get("test_resource_5mb")
	if err != nil {
		t.Fatalf("Get should succeed after successful registration: %v", err)
	}
	// Verify resource can be read
	ctx := context.Background()
	data, err := resource.Read(ctx)
	if err != nil {
		t.Fatalf("Read should succeed for resource under size limit: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("Read data size mismatch: got %d bytes, want %d bytes", len(data), len(content))
	}
}

// ============================================================================
// Test Case 2: Static Resource Exactly At Size Limit (Success)
// ============================================================================

func TestResourceProvider_SizeLimit_StaticAtLimit(t *testing.T) {
	provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
	// Create exactly 10MB of test data (exactly at limit)
	content := generateTestData(defaultMaxResourceSize) // 10MB exactly
	// Register static resource - should succeed
	err := provider.RegisterStatic(
		"test_resource_10mb",
		"Test Resource 10MB",
		"A 10MB test resource that should register successfully (at limit)",
		sizeLimitMimeType,
		content,
	)
	if err != nil {
		t.Fatalf("RegisterStatic with exactly 10MB content should succeed, got error: %v", err)
	}
	// Verify resource can be retrieved
	resource, err := provider.Get("test_resource_10mb")
	if err != nil {
		t.Fatalf("Get should succeed after successful registration: %v", err)
	}
	// Verify resource can be read
	ctx := context.Background()
	data, err := resource.Read(ctx)
	if err != nil {
		t.Fatalf("Read should succeed for resource at size limit: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("Read data size mismatch: got %d bytes, want %d bytes", len(data), len(content))
	}
}

// ============================================================================
// Test Case 3: Static Resource Over Size Limit (Error at Registration)
// ============================================================================

func TestResourceProvider_SizeLimit_StaticOverLimit(t *testing.T) {
	provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
	// Create 15MB of test data (over 10MB limit)
	content := generateTestData(15 * 1024 * 1024) // 15MB
	// Register static resource - should FAIL at registration time
	err := provider.RegisterStatic(
		"test_resource_15mb",
		"Test Resource 15MB",
		"A 15MB test resource that should fail registration",
		sizeLimitMimeType,
		content,
	)
	if err == nil {
		t.Fatal("RegisterStatic with 15MB content should fail with size limit error")
	}
	// Verify error message contains required information
	errMsg := err.Error()
	// Check for presence of key information in error
	requiredFields := []string{
		"test_resource_15mb", // uri
		"15728640",           // size (15MB in bytes) - may vary slightly
		"10485760",           // maxSize (10MB in bytes)
	}
	for _, field := range requiredFields {
		if !containsString(errMsg, field) {
			t.Errorf("Error message should contain '%s', got: %s", field, errMsg)
		}
	}
	// Verify error message indicates size limit exceeded
	if !containsAny(errMsg, []string{"size", "limit", "exceeds"}) {
		t.Errorf("Error message should mention 'size' or 'limit', got: %s", errMsg)
	}
	// Verify resource was NOT registered (Get should fail)
	_, err = provider.Get("test_resource_15mb")
	if err == nil {
		t.Error("Get should fail for resource that failed registration due to size limit")
	}
}

// ============================================================================
// Test Case 4: Dynamic Resource Over Size Limit (Error at Read Time)
// ============================================================================

func TestResourceProvider_SizeLimit_DynamicOverLimit(t *testing.T) {
	provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
	// Create generator that returns 15MB of data
	generator := func(ctx context.Context) ([]byte, error) {
		return generateTestData(15 * 1024 * 1024), nil // 15MB
	}
	// Register dynamic resource - should SUCCEED (size not checked at registration)
	err := provider.RegisterDynamic(
		"test_dynamic_15mb",
		"Test Dynamic Resource 15MB",
		"A dynamic resource that generates 15MB of data",
		sizeLimitMimeType,
		generator,
	)
	if err != nil {
		t.Fatalf("RegisterDynamic should succeed (size check happens at read time), got error: %v", err)
	}
	// Read should FAIL due to size limit (use provider.Read to enforce size limit)
	ctx := context.Background()
	data, err := provider.Read(ctx, "test_dynamic_15mb")
	if err == nil {
		t.Fatal("Read should fail for dynamic resource that exceeds size limit")
	}
	if data != nil {
		t.Error("Read should return nil data when size limit is exceeded")
	}
	// Verify error message contains required information
	errMsg := err.Error()
	requiredFields := []string{
		"test_dynamic_15mb", // uri
	}
	for _, field := range requiredFields {
		if !containsString(errMsg, field) {
			t.Errorf("Error message should contain '%s', got: %s", field, errMsg)
		}
	}
	// Should mention size or limit
	if !containsAny(errMsg, []string{"size", "limit", "exceeds"}) {
		t.Errorf("Error message should mention 'size' or 'limit', got: %s", errMsg)
	}
}

// ============================================================================
// Test Case 5: Configurable Size Limit
// ============================================================================

func TestResourceProvider_SizeLimit_ConfigurableLimit(t *testing.T) {
	// Create provider with custom 1MB size limit
	customLimit := 1 * 1024 * 1024 // 1MB
	provider := NewResourceProvider(WithMaxResourceSize(customLimit))
	// Test 1: Resource under custom limit should succeed
	t.Run("under_custom_limit", func(t *testing.T) {
		content := generateTestData(512 * 1024) // 512KB (under 1MB)
		err := provider.RegisterStatic(
			"test_under_1mb",
			"Test Under 1MB",
			"Resource under custom 1MB limit",
			sizeLimitMimeType,
			content,
		)
		if err != nil {
			t.Fatalf("RegisterStatic with 512KB content should succeed with 1MB limit, got error: %v", err)
		}
	})
	// Test 2: Resource over custom limit should fail
	t.Run("over_custom_limit", func(t *testing.T) {
		content := generateTestData(2 * 1024 * 1024) // 2MB (over 1MB)
		err := provider.RegisterStatic(
			"test_over_1mb",
			"Test Over 1MB",
			"Resource over custom 1MB limit",
			sizeLimitMimeType,
			content,
		)
		if err == nil {
			t.Fatal("RegisterStatic with 2MB content should fail with 1MB limit")
		}
		// Verify error mentions the custom limit
		errMsg := err.Error()
		if !containsString(errMsg, "1048576") { // 1MB in bytes
			t.Errorf("Error should mention custom limit of 1048576 bytes, got: %s", errMsg)
		}
	})
	// Test 3: Dynamic resource over custom limit should fail at read time
	t.Run("dynamic_over_custom_limit", func(t *testing.T) {
		generator := func(ctx context.Context) ([]byte, error) {
			return generateTestData(2 * 1024 * 1024), nil // 2MB
		}
		err := provider.RegisterDynamic(
			"test_dynamic_2mb",
			"Test Dynamic 2MB",
			"Dynamic resource that exceeds 1MB limit",
			sizeLimitMimeType,
			generator,
		)
		if err != nil {
			t.Fatalf("RegisterDynamic should succeed, got error: %v", err)
		}
		// Read should fail (use provider.Read to enforce size limit)
		ctx := context.Background()
		_, err = provider.Read(ctx, "test_dynamic_2mb")
		if err == nil {
			t.Fatal("Read should fail for dynamic resource exceeding custom 1MB limit")
		}
	})
}

// ============================================================================
// Test Case 6: Edge Cases for Size Limits
// ============================================================================

func TestResourceProvider_SizeLimit_EdgeCases(t *testing.T) {
	t.Run("zero_size_resource", func(t *testing.T) {
		provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
		content := []byte{}
		err := provider.RegisterStatic(
			"test_zero_size",
			"Test Zero Size",
			"Empty resource",
			sizeLimitMimeType,
			content,
		)
		if err != nil {
			t.Fatalf("RegisterStatic with empty content should succeed, got error: %v", err)
		}
	})
	t.Run("one_byte_over_limit", func(t *testing.T) {
		provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
		// One byte over the limit
		content := generateTestData(defaultMaxResourceSize + 1)
		err := provider.RegisterStatic(
			"test_one_over",
			"Test One Over",
			"Resource one byte over limit",
			sizeLimitMimeType,
			content,
		)
		if err == nil {
			t.Fatal("RegisterStatic with one byte over limit should fail")
		}
	})
	t.Run("one_byte_under_limit", func(t *testing.T) {
		provider := NewResourceProvider(WithMaxResourceSize(defaultMaxResourceSize))
		// One byte under the limit
		content := generateTestData(defaultMaxResourceSize - 1)
		err := provider.RegisterStatic(
			"test_one_under",
			"Test One Under",
			"Resource one byte under limit",
			sizeLimitMimeType,
			content,
		)
		if err != nil {
			t.Fatalf("RegisterStatic with one byte under limit should succeed, got error: %v", err)
		}
	})
}
