# Resource Provider Contract

**Feature**: MCP Server Support
**Version**: MCP Specification 2025-06-18
**Date**: 2025-11-17

## Overview

This contract defines the MCP Resource operations that expose read-only data from LangGraph workflows to external clients. Resources represent data entities (workflow state, checkpoints, metrics, logs) accessible via unique URIs.

**Key Principles**:
- Resources are read-only (no mutations)
- Resources identified by unique URIs
- Support both static (cached) and dynamic (computed on-demand) resources
- Resources can reference workflow state, checkpoints, or arbitrary data

---

## Resource Discovery

### `resources/list` Request

Retrieve the list of all resources available on the MCP server.

**Method**: `resources/list`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "resources/list",
  "params": {}
}
```

**Request Parameters**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| (none) | - | - | This method takes no parameters. Use empty object `{}` or omit `params`. |

**Validation Rules**:
- Server MUST accept empty `params` object or omitted `params` field
- Server MUST reject non-empty `params` with `-32602` Invalid params

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "resources/list"
}
```

---

### `resources/list` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "resources": [
      {
        "uri": string,
        "name": string,
        "description": string,
        "mimeType": string
      }
    ]
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `resources` | `array` | Yes | List of available resources. Empty array `[]` if no resources registered. |
| `resources[].uri` | `string` | Yes | Unique resource identifier. MUST match pattern `^[a-z][a-z0-9_/]*$` (lowercase, underscores, slashes). |
| `resources[].name` | `string` | Yes | Human-readable resource name (e.g., `"Workflow State"`, `"Latest Checkpoint"`). |
| `resources[].description` | `string` | Yes | Description of resource content and purpose. |
| `resources[].mimeType` | `string` | Yes | MIME type of resource content (e.g., `"application/json"`, `"text/plain"`). |

**Resource URI Pattern**: `^[a-z][a-z0-9_/]*$`
- MUST start with lowercase letter
- MAY contain lowercase letters, digits, underscores, forward slashes
- Slashes denote hierarchical organization
- Examples: `workflow_state`, `checkpoints/latest`, `metrics/runtime`, `history/run-123`

**Common MIME Types**:

| MIME Type | Usage |
|-----------|-------|
| `application/json` | Structured data (state, checkpoints, metrics) |
| `text/plain` | Plain text (logs, configuration) |
| `text/markdown` | Markdown documentation |
| `application/octet-stream` | Binary data |

**Validation Rules**:
- `resources` array MAY be empty (valid if no resources registered)
- Resource URIs MUST be unique within the array
- `name` and `description` MUST be non-empty strings
- `mimeType` MUST be a valid MIME type string

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "resources": [
      {
        "uri": "workflow_state",
        "name": "Workflow State",
        "description": "Current state of the workflow execution",
        "mimeType": "application/json"
      },
      {
        "uri": "checkpoints/latest",
        "name": "Latest Checkpoint",
        "description": "Most recent saved checkpoint",
        "mimeType": "application/json"
      },
      {
        "uri": "checkpoints/iteration-5",
        "name": "Checkpoint: Iteration 5",
        "description": "Checkpoint saved after 5th iteration",
        "mimeType": "application/json"
      },
      {
        "uri": "metrics/runtime",
        "name": "Runtime Metrics",
        "description": "Current workflow execution metrics",
        "mimeType": "application/json"
      }
    ]
  }
}
```

---

### `resources/list` Error Cases

**Error: Resources Capability Not Enabled**

**JSON-RPC Code**: `-32601` (Method not found)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found: 'resources/list'",
    "data": {
      "reason": "Resources capability not negotiated during initialization",
      "availableCapabilities": ["tools"]
    }
  }
}
```

**Error: Invalid Parameters**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: resources/list does not accept parameters",
    "data": {
      "received": {"filter": "checkpoints"}
    }
  }
}
```

---

## Resource Reading

### `resources/read` Request

Read the content of a specific resource by URI.

**Method**: `resources/read`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "resources/read",
  "params": {
    "uri": string
  }
}
```

**Request Parameters**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `uri` | `string` | Yes | Resource URI (MUST match a resource from `resources/list`) |

**Validation Rules**:
- `uri` MUST be non-empty string
- `uri` MUST match a registered resource URI exactly
- `uri` is case-sensitive

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "resources/read",
  "params": {
    "uri": "workflow_state"
  }
}
```

---

### `resources/read` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "contents": [
      {
        "uri": string,
        "mimeType": string,
        "text": string,
        "blob": string
      }
    ]
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `contents` | `array` | Yes | Array of content items. Typically contains one element (the requested resource). |
| `contents[].uri` | `string` | Yes | Resource URI (matches request URI) |
| `contents[].mimeType` | `string` | Yes | MIME type of content |
| `contents[].text` | `string` | Conditional | Text content (required if content is text-based) |
| `contents[].blob` | `string` | Conditional | Base64-encoded binary content (required if content is binary) |

**Content Format Rules**:

| MIME Type | Field Used | Encoding |
|-----------|------------|----------|
| `application/json` | `text` | UTF-8 JSON string |
| `text/plain` | `text` | UTF-8 plain text |
| `text/markdown` | `text` | UTF-8 markdown |
| `application/octet-stream` | `blob` | Base64 encoding |
| `image/*` | `blob` | Base64 encoding |

**Validation Rules**:
- `contents` array MUST contain exactly one element for single resource reads
- Either `text` OR `blob` MUST be present (not both)
- `text` field used for text-based MIME types
- `blob` field used for binary MIME types
- `blob` MUST be valid Base64 encoding

**Example Response (JSON Resource)**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "contents": [
      {
        "uri": "workflow_state",
        "mimeType": "application/json",
        "text": "{\"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}], \"counter\": 5, \"lastUpdate\": \"2025-11-17T10:30:00Z\"}"
      }
    ]
  }
}
```

**Example Response (Text Resource)**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "contents": [
      {
        "uri": "logs/execution",
        "mimeType": "text/plain",
        "text": "2025-11-17 10:30:00 INFO: Workflow started\n2025-11-17 10:30:05 INFO: Node 'process' completed\n2025-11-17 10:30:10 INFO: Workflow finished"
      }
    ]
  }
}
```

**Example Response (Binary Resource)**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "contents": [
      {
        "uri": "graph_visualization",
        "mimeType": "image/png",
        "blob": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
      }
    ]
  }
}
```

---

### `resources/read` Error Cases

**Error: Resource Not Found**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": -32602,
    "message": "Invalid params: resource 'nonexistent' not found",
    "data": {
      "uri": "nonexistent",
      "availableResources": ["workflow_state", "checkpoints/latest", "metrics/runtime"]
    }
  }
}
```

**Error: Missing URI Parameter**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "error": {
    "code": -32602,
    "message": "Invalid params: 'uri' parameter is required",
    "data": {
      "parameter": "uri",
      "received": null
    }
  }
}
```

**Error: Resource Read Failure**

**JSON-RPC Code**: `-32603` (Internal error)

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32603,
    "message": "Internal error: failed to read resource 'workflow_state'",
    "data": {
      "uri": "workflow_state",
      "error": "context deadline exceeded",
      "timeout": "5s"
    }
  }
}
```

**Error: Resource Too Large**

**JSON-RPC Code**: `-32603` (Internal error)

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "error": {
    "code": -32603,
    "message": "Internal error: resource exceeds size limit",
    "data": {
      "uri": "large_dataset",
      "size": 15728640,
      "maxSize": 10485760,
      "hint": "Consider pagination or streaming (future feature)"
    }
  }
}
```

---

## Resource Registration (Server-Side)

This section documents the Go API for registering resources with the MCP server (not part of JSON-RPC protocol).

**Go Interface** (`graph/mcp/resource_provider.go`):

```go
type ResourceInfo struct {
    URI         string // Resource URI (validated against pattern)
    Name        string // Human-readable name
    Description string // Resource description
    MimeType    string // MIME type
}

type Resource interface {
    // URI returns the resource identifier
    URI() string

    // MimeType returns the content type
    MimeType() string

    // Read fetches resource content (may be cached or computed)
    Read(ctx context.Context) ([]byte, error)
}

type ResourceProvider interface {
    // RegisterStatic registers a resource with fixed content
    RegisterStatic(uri, name, description, mimeType string, content []byte) error

    // RegisterDynamic registers a resource with computed content
    RegisterDynamic(uri, name, description, mimeType string, generator func(context.Context) ([]byte, error)) error

    // Get retrieves a resource by URI
    Get(uri string) (Resource, error)

    // List returns all resource metadata
    List() []ResourceInfo

    // Read fetches resource content by URI
    Read(ctx context.Context, uri string) ([]byte, error)
}

type StaticResource struct {
    uri      string
    mimeType string
    content  []byte
}

type DynamicResource struct {
    uri       string
    mimeType  string
    generator func(context.Context) ([]byte, error)
}
```

**Registration Example (Static Resource)**:

```go
provider := mcp.NewResourceProvider()

// Register static resource (fixed content)
stateJSON := []byte(`{"messages": [], "counter": 0}`)
err := provider.RegisterStatic(
    "workflow_state",
    "Workflow State",
    "Current state of workflow execution",
    "application/json",
    stateJSON,
)
```

**Registration Example (Dynamic Resource)**:

```go
// Register dynamic resource (computed on-demand)
err := provider.RegisterDynamic(
    "metrics/runtime",
    "Runtime Metrics",
    "Live workflow execution metrics",
    "application/json",
    func(ctx context.Context) ([]byte, error) {
        metrics := getCurrentMetrics()
        return json.Marshal(metrics)
    },
)
```

**Validation at Registration Time**:

The server MUST validate at registration:
- Resource URI matches pattern `^[a-z][a-z0-9_/]*$`
- Resource URI is unique (no duplicates)
- Name and description are non-empty
- MIME type is valid
- Content size is under limit (default 10MB, configurable)

Invalid registrations MUST panic (fail-fast during server initialization).

---

## Common Resource URI Patterns

### Workflow State

**URI**: `workflow_state`
**MIME Type**: `application/json`
**Description**: Current workflow state snapshot

**Example Content**:
```json
{
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi there!"}
  ],
  "counter": 5,
  "metadata": {
    "runID": "run-abc123",
    "startTime": "2025-11-17T10:00:00Z"
  }
}
```

---

### Checkpoints

**URI Pattern**: `checkpoints/{label}`
**MIME Type**: `application/json`
**Description**: Saved checkpoint at specific label

**Example URIs**:
- `checkpoints/latest` - Most recent checkpoint
- `checkpoints/iteration-5` - Checkpoint at iteration 5
- `checkpoints/pre-deploy` - Named checkpoint before deployment

**Example Content**:
```json
{
  "label": "iteration-5",
  "step": 10,
  "nodeID": "process",
  "state": {
    "messages": [...],
    "counter": 5
  },
  "timestamp": "2025-11-17T10:05:00Z"
}
```

---

### Execution History

**URI Pattern**: `history/{runID}`
**MIME Type**: `application/json`
**Description**: Full execution history for a workflow run

**Example URI**: `history/run-abc123`

**Example Content**:
```json
{
  "runID": "run-abc123",
  "startTime": "2025-11-17T10:00:00Z",
  "endTime": "2025-11-17T10:10:00Z",
  "steps": [
    {"step": 0, "nodeID": "start", "timestamp": "2025-11-17T10:00:00Z"},
    {"step": 1, "nodeID": "process", "timestamp": "2025-11-17T10:00:05Z"},
    {"step": 2, "nodeID": "finish", "timestamp": "2025-11-17T10:10:00Z"}
  ],
  "finalState": {...}
}
```

---

### Runtime Metrics

**URI**: `metrics/runtime`
**MIME Type**: `application/json`
**Description**: Live workflow execution metrics (dynamic)

**Example Content**:
```json
{
  "nodesExecuted": 42,
  "totalExecutionTime": "5m30s",
  "averageNodeTime": "7.8s",
  "errorsEncountered": 0,
  "checkpointsSaved": 5,
  "currentNode": "process_batch",
  "timestamp": "2025-11-17T10:30:00Z"
}
```

---

### Logs

**URI**: `logs/execution`
**MIME Type**: `text/plain`
**Description**: Workflow execution logs

**Example Content**:
```
2025-11-17 10:00:00 INFO: Workflow started (runID: run-abc123)
2025-11-17 10:00:05 INFO: Node 'start' completed in 5s
2025-11-17 10:00:12 INFO: Node 'process' completed in 7s
2025-11-17 10:00:15 WARN: Node 'validate' retrying (attempt 2/3)
2025-11-17 10:00:18 INFO: Node 'validate' completed in 6s
2025-11-17 10:10:00 INFO: Workflow finished successfully
```

---

## Static vs Dynamic Resources

### Static Resources

**Use Cases**:
- Fixed configuration data
- Cached state snapshots
- Historical checkpoints (immutable)
- Precomputed reports

**Characteristics**:
- Content computed once at registration
- Fast reads (in-memory cache)
- Content never changes during server lifetime

**Example**:
```go
configJSON := []byte(`{"maxRetries": 3, "timeout": "30s"}`)
provider.RegisterStatic("config", "Configuration", "Server config", "application/json", configJSON)
```

---

### Dynamic Resources

**Use Cases**:
- Live metrics and monitoring data
- Current workflow state
- Real-time logs
- On-demand computed data

**Characteristics**:
- Content computed on each read
- Reflects current system state
- May have higher latency than static resources

**Example**:
```go
provider.RegisterDynamic("workflow_state", "State", "Current state", "application/json",
    func(ctx context.Context) ([]byte, error) {
        state, err := store.LoadLatest(ctx, runID)
        if err != nil {
            return nil, err
        }
        return json.Marshal(state)
    },
)
```

---

## Resource Access from LangGraph Store

Resources can expose data from LangGraph's Store interface.

**Example (Workflow State)**:

```go
provider.RegisterDynamic("workflow_state", "Workflow State", "Current workflow state", "application/json",
    func(ctx context.Context) ([]byte, error) {
        state, _, _, err := store.LoadLatest(ctx, runID)
        if err != nil {
            return nil, fmt.Errorf("failed to load state: %w", err)
        }
        return json.Marshal(state)
    },
)
```

**Example (Checkpoint)**:

```go
provider.RegisterDynamic("checkpoints/latest", "Latest Checkpoint", "Most recent checkpoint", "application/json",
    func(ctx context.Context) ([]byte, error) {
        // LoadLatest returns state, step, nodeID
        state, step, nodeID, err := store.LoadLatest(ctx, runID)
        if err != nil {
            return nil, err
        }

        checkpoint := map[string]interface{}{
            "step":   step,
            "nodeID": nodeID,
            "state":  state,
        }
        return json.Marshal(checkpoint)
    },
)
```

---

## Context Cancellation

Resource reads MUST respect context cancellation:

**Example**:
```go
func (r *DynamicResource) Read(ctx context.Context) ([]byte, error) {
    data, err := r.generator(ctx)
    if err != nil {
        // Check for cancellation
        if ctx.Err() != nil {
            return nil, fmt.Errorf("resource read cancelled: %w", ctx.Err())
        }
        return nil, err
    }
    return data, nil
}
```

**Error Response (Context Cancelled)**:
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "error": {
    "code": -32603,
    "message": "Internal error: resource read cancelled",
    "data": {
      "uri": "slow_computation",
      "error": "context canceled"
    }
  }
}
```

---

## Size Limits

Resources SHOULD enforce size limits to prevent memory exhaustion.

**Recommended Limits**:
- Default: 10MB per resource
- Configurable via server options

**Handling Large Resources**:

If resource exceeds limit, server SHOULD return error:

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "error": {
    "code": -32603,
    "message": "Internal error: resource exceeds size limit",
    "data": {
      "uri": "large_dataset",
      "size": 15728640,
      "maxSize": 10485760
    }
  }
}
```

**Future Enhancement**: Pagination or streaming for large resources (deferred to future MCP specification versions).

---

## Examples

### Example 1: List Resources

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "resources/list"
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "resources": [
      {
        "uri": "workflow_state",
        "name": "Workflow State",
        "description": "Current workflow execution state",
        "mimeType": "application/json"
      },
      {
        "uri": "checkpoints/latest",
        "name": "Latest Checkpoint",
        "description": "Most recent saved checkpoint",
        "mimeType": "application/json"
      }
    ]
  }
}
```

---

### Example 2: Read Resource (Success)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "resources/read",
  "params": {
    "uri": "workflow_state"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "contents": [
      {
        "uri": "workflow_state",
        "mimeType": "application/json",
        "text": "{\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}],\"counter\":5}"
      }
    ]
  }
}
```

---

### Example 3: Read Resource (Not Found)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "resources/read",
  "params": {
    "uri": "nonexistent_resource"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "error": {
    "code": -32602,
    "message": "Invalid params: resource 'nonexistent_resource' not found",
    "data": {
      "uri": "nonexistent_resource",
      "availableResources": ["workflow_state", "checkpoints/latest"]
    }
  }
}
```

---

### Example 4: Read Large Resource (Error)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/read",
  "params": {
    "uri": "large_logs"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32603,
    "message": "Internal error: resource exceeds size limit",
    "data": {
      "uri": "large_logs",
      "size": 15728640,
      "maxSize": 10485760
    }
  }
}
```

---

## Security Considerations

### Data Exposure

- Resources MUST NOT expose sensitive credentials or secrets
- Resource descriptions SHOULD clearly indicate if data is sensitive
- Server SHOULD allow filtering which resources to expose (allowlist pattern)

### Access Control

- Current MCP specification does not define authentication/authorization
- Deployment-level security (network isolation, process permissions) SHOULD be used
- Future enhancement: Resource-level access control

### Error Messages

**Safe Error**:
```json
{
  "code": -32602,
  "message": "Invalid params: resource 'admin/config' not found"
}
```

**Unsafe Error** (DO NOT):
```json
{
  "code": -32602,
  "message": "Invalid params: resource 'admin/config' exists but access denied for user 'guest' with IP 192.168.1.100"
}
```

---

## Performance Considerations

- Resource listing (`resources/list`) SHOULD complete in <100ms
- Static resource reads SHOULD complete in <50ms (in-memory cache)
- Dynamic resource reads SHOULD complete in <500ms for 10MB datasets
- Server SHOULD limit concurrent resource reads (recommend 100 max)
- Dynamic resources SHOULD implement caching if computation is expensive

---

## Reference

- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **LangGraph Store Interface**: `graph/store/store.go`
- **MIME Types**: https://www.iana.org/assignments/media-types/media-types.xhtml
