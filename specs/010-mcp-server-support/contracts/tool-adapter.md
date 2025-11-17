# Tool Adapter Contract

**Feature**: MCP Server Support
**Version**: MCP Specification 2025-06-18
**Date**: 2025-11-17

## Overview

This contract defines the MCP Tool operations that expose LangGraph tools to external clients. Tools represent invokable functions with typed inputs and outputs, allowing LLM applications to perform actions through the MCP server.

**Key Principles**:
- Tools wrap existing `graph/tool.Tool` implementations
- Input validation uses JSON Schema
- Tool execution respects LangGraph context cancellation
- Errors propagate as JSON-RPC error responses

---

## Tool Discovery

### `tools/list` Request

Retrieve the list of all tools available on the MCP server.

**Method**: `tools/list`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "tools/list",
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
  "method": "tools/list"
}
```

---

### `tools/list` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "tools": [
      {
        "name": string,
        "description": string,
        "inputSchema": object
      }
    ]
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tools` | `array` | Yes | List of available tools. Empty array `[]` if no tools registered. |
| `tools[].name` | `string` | Yes | Unique tool identifier. MUST match pattern `^[a-z][a-z0-9_]*$` (lowercase, underscores). |
| `tools[].description` | `string` | Yes | Human-readable description of tool purpose and behavior. |
| `tools[].inputSchema` | `object` | Yes | JSON Schema defining the tool's input parameters. |

**Tool Name Pattern**: `^[a-z][a-z0-9_]*$`
- MUST start with lowercase letter
- MAY contain lowercase letters, digits, underscores
- Examples: `weather`, `http_get`, `query_database`

**inputSchema Format**:

The `inputSchema` field MUST be a valid [JSON Schema Draft 7](https://json-schema.org/draft-07/json-schema-release-notes.html) object.

**Minimal Schema**:
```json
{
  "type": "object",
  "properties": {},
  "required": []
}
```

**Typical Schema**:
```json
{
  "type": "object",
  "properties": {
    "location": {
      "type": "string",
      "description": "City name or zip code"
    },
    "units": {
      "type": "string",
      "enum": ["celsius", "fahrenheit"],
      "default": "celsius"
    }
  },
  "required": ["location"]
}
```

**Validation Rules**:
- `tools` array MAY be empty (valid if no tools registered)
- Tool names MUST be unique within the array
- `description` MUST be non-empty string
- `inputSchema` MUST be a valid JSON Schema object
- `inputSchema.type` SHOULD be `"object"` (tools take object parameters)

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "get_weather",
        "description": "Retrieve current weather information for a specified location",
        "inputSchema": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "City name (e.g., 'San Francisco') or zip code"
            },
            "units": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"],
              "default": "celsius",
              "description": "Temperature units"
            }
          },
          "required": ["location"]
        }
      },
      {
        "name": "search_database",
        "description": "Query the workflow database with SQL",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "SQL SELECT statement"
            },
            "limit": {
              "type": "integer",
              "minimum": 1,
              "maximum": 1000,
              "default": 100,
              "description": "Maximum number of results"
            }
          },
          "required": ["query"]
        }
      }
    ]
  }
}
```

---

### `tools/list` Error Cases

**Error: Tools Capability Not Enabled**

**JSON-RPC Code**: `-32601` (Method not found)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found: 'tools/list'",
    "data": {
      "reason": "Tools capability not negotiated during initialization",
      "availableCapabilities": ["resources"]
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
    "message": "Invalid params: tools/list does not accept parameters",
    "data": {
      "received": {"filter": "weather"}
    }
  }
}
```

---

## Tool Invocation

### `tools/call` Request

Invoke a tool by name with input parameters.

**Method**: `tools/call`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "tools/call",
  "params": {
    "name": string,
    "arguments": object
  }
}
```

**Request Parameters**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Tool name (MUST match a tool from `tools/list`) |
| `arguments` | `object` | No | Tool input parameters. Validated against tool's `inputSchema`. If omitted, defaults to `{}`. |

**Validation Rules**:
- `name` MUST match a registered tool name exactly
- `arguments` MUST conform to the tool's `inputSchema`
- `arguments` MUST be an object (not array, string, or primitive)
- Required parameters (per `inputSchema.required`) MUST be present in `arguments`
- Extra properties (not in `inputSchema.properties`) MAY be allowed depending on schema's `additionalProperties` setting

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_weather",
    "arguments": {
      "location": "San Francisco",
      "units": "fahrenheit"
    }
  }
}
```

**Example Request (Minimal)**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "list_workflows"
  }
}
```
*(Arguments omitted when tool has no required parameters)*

---

### `tools/call` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "content": [
      {
        "type": "text" | "image" | "resource",
        "text": string,
        "data": string,
        "resource": string,
        "mimeType": string
      }
    ],
    "isError": boolean
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | `array` | Yes | Array of content blocks representing tool output. MUST have at least one element. |
| `content[].type` | `string` | Yes | Content type: `"text"`, `"image"`, or `"resource"` |
| `isError` | `boolean` | No | If `true`, indicates tool executed but returned error content. Defaults to `false`. |

**Content Types**:

**Text Content**:
```json
{
  "type": "text",
  "text": string
}
```
- `text`: Tool output as plain text or JSON string
- Most common content type for tool results

**Image Content**:
```json
{
  "type": "image",
  "data": string,
  "mimeType": string
}
```
- `data`: Base64-encoded image data
- `mimeType`: MIME type (e.g., `"image/png"`, `"image/jpeg"`)

**Resource Content**:
```json
{
  "type": "resource",
  "resource": string
}
```
- `resource`: URI of a resource (reference to MCP resource, see `resource-provider.md`)

**Validation Rules**:
- `content` array MUST NOT be empty
- Each content item MUST have a `type` field
- `type: "text"` REQUIRES `text` field
- `type: "image"` REQUIRES `data` and `mimeType` fields
- `type: "resource"` REQUIRES `resource` field
- `isError: true` indicates business logic error (tool executed but failed its operation)
- Tool execution errors (crashes, timeouts) MUST use JSON-RPC error responses instead

**Example Response (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"temperature\": 72, \"conditions\": \"sunny\", \"humidity\": 45}"
      }
    ]
  }
}
```

**Example Response (Multi-Content)**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Query returned 5 results"
      },
      {
        "type": "resource",
        "resource": "query_results/abc123"
      }
    ]
  }
}
```

**Example Response (Business Logic Error)**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Error: Location 'InvalidCity' not found in weather database"
      }
    ],
    "isError": true
  }
}
```

---

### `tools/call` Error Cases

**Error: Tool Not Found**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": -32602,
    "message": "Invalid params: tool 'unknown_tool' not found",
    "data": {
      "toolName": "unknown_tool",
      "availableTools": ["get_weather", "search_database"]
    }
  }
}
```

**Error: Missing Required Parameter**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "error": {
    "code": -32602,
    "message": "Invalid params: missing required parameter 'location'",
    "data": {
      "parameter": "location",
      "schema": {
        "type": "string",
        "description": "City name or zip code"
      }
    }
  }
}
```

**Error: Parameter Type Mismatch**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32602,
    "message": "Invalid params: parameter 'limit' must be integer",
    "data": {
      "parameter": "limit",
      "expected": "integer",
      "received": "string",
      "value": "not-a-number"
    }
  }
}
```

**Error: Tool Execution Failure**

**JSON-RPC Code**: `-32603` (Internal error)

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "error": {
    "code": -32603,
    "message": "Internal error: tool execution failed",
    "data": {
      "toolName": "get_weather",
      "error": "context deadline exceeded",
      "duration": "30s"
    }
  }
}
```

**Error: Tool Timeout**

**JSON-RPC Code**: `-32603` (Internal error)

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "error": {
    "code": -32603,
    "message": "Internal error: tool execution timeout",
    "data": {
      "toolName": "slow_operation",
      "timeout": "10s",
      "error": "context deadline exceeded"
    }
  }
}
```

---

## Tool Registration (Server-Side)

This section documents the Go API for registering tools with the MCP server (not part of JSON-RPC protocol).

**Go Interface** (`graph/mcp/tool_adapter.go`):

```go
type ToolMetadata struct {
    Name        string                 // Tool name (validated against pattern)
    Description string                 // Human-readable description
    InputSchema map[string]interface{} // JSON Schema for input validation
}

type ToolRegistry interface {
    // Register a LangGraph tool with MCP metadata
    Register(tool tool.Tool, metadata ToolMetadata) error

    // Get registered tool by name
    Get(name string) (*RegisteredTool, error)

    // List all tool metadata
    List() []ToolMetadata

    // Invoke tool with validated input
    Invoke(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
}

type RegisteredTool struct {
    Tool     tool.Tool
    Metadata ToolMetadata
}

type ToolResult struct {
    Content []ContentBlock
    IsError bool
}

type ContentBlock struct {
    Type     string // "text", "image", "resource"
    Text     string
    Data     string // Base64 for images
    MimeType string
    Resource string // Resource URI
}
```

**Registration Example**:

```go
registry := mcp.NewToolRegistry()

// Register weather tool
err := registry.Register(weatherTool, mcp.ToolMetadata{
    Name:        "get_weather",
    Description: "Get current weather for a location",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City name or zip code",
            },
            "units": map[string]interface{}{
                "type":        "string",
                "enum":        []string{"celsius", "fahrenheit"},
                "default":     "celsius",
            },
        },
        "required": []string{"location"},
    },
})
```

**Validation at Registration Time**:

The server MUST validate at registration:
- Tool name matches pattern `^[a-z][a-z0-9_]*$`
- Tool name is unique (no duplicates)
- Description is non-empty
- InputSchema is valid JSON Schema

Invalid registrations MUST panic (fail-fast during server initialization).

---

## Input Validation

### JSON Schema Validation

Server MUST validate `arguments` against tool's `inputSchema` before invocation.

**Validation Steps**:

1. **Type Checking**: Validate argument types match schema types
2. **Required Fields**: Ensure all `required` fields are present
3. **Enum Values**: If `enum` specified, value MUST be in enum list
4. **Number Constraints**: Validate `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`
5. **String Constraints**: Validate `minLength`, `maxLength`, `pattern`
6. **Array Constraints**: Validate `minItems`, `maxItems`, `uniqueItems`
7. **Additional Properties**: Respect `additionalProperties` setting

**Common Validation Errors**:

| Validation Failure | Error Message Example |
|-------------------|----------------------|
| Missing required field | `"missing required parameter 'location'"` |
| Type mismatch | `"parameter 'limit' must be integer, got string"` |
| Enum violation | `"parameter 'units' must be one of [celsius, fahrenheit], got 'kelvin'"` |
| Range violation | `"parameter 'limit' must be between 1 and 1000, got 5000"` |
| Pattern mismatch | `"parameter 'email' must match pattern '^[^@]+@[^@]+$'"` |

---

## Tool Execution Lifecycle

```
┌─────────────────┐
│ tools/call      │
│ request         │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Validate tool   │
│ name exists     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Validate        │
│ arguments       │
│ against schema  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Invoke          │
│ tool.Call()     │
│ with context    │
└────────┬────────┘
         │
         ├─────────────────┐
         │ Success         │ Error
         ▼                 ▼
┌─────────────────┐  ┌─────────────────┐
│ Format result   │  │ Return -32603   │
│ as content      │  │ Internal error  │
│ blocks          │  │                 │
└────────┬────────┘  └─────────────────┘
         │
         ▼
┌─────────────────┐
│ Return result   │
│ or isError      │
└─────────────────┘
```

**Key Points**:
- Schema validation happens BEFORE tool execution
- Tool execution errors (panics, timeouts, context cancellation) map to `-32603` Internal error
- Business logic errors (tool returned error result) use `isError: true` in result

---

## Context Cancellation

Tool execution MUST respect context cancellation:

**Scenario 1: Client Disconnects During Tool Execution**

```go
func (r *ToolRegistry) Invoke(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
    tool, err := r.Get(name)
    if err != nil {
        return nil, err
    }

    // Tool.Call receives cancellable context
    output, err := tool.Tool.Call(ctx, args)
    if err != nil {
        // Check for cancellation
        if ctx.Err() != nil {
            return nil, fmt.Errorf("tool execution cancelled: %w", ctx.Err())
        }
        return nil, err
    }

    return formatToolOutput(output), nil
}
```

**Error Response (Context Cancelled)**:
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "error": {
    "code": -32603,
    "message": "Internal error: tool execution cancelled",
    "data": {
      "toolName": "long_running_task",
      "error": "context canceled"
    }
  }
}
```

---

## Examples

### Example 1: List Tools

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "http_get",
        "description": "Perform HTTP GET request to a URL",
        "inputSchema": {
          "type": "object",
          "properties": {
            "url": {"type": "string", "format": "uri"},
            "headers": {"type": "object", "additionalProperties": {"type": "string"}}
          },
          "required": ["url"]
        }
      }
    ]
  }
}
```

---

### Example 2: Call Tool (Success)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "http_get",
    "arguments": {
      "url": "https://api.example.com/status",
      "headers": {
        "Authorization": "Bearer token123"
      }
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"status\": \"ok\", \"uptime\": 12345}"
      }
    ]
  }
}
```

---

### Example 3: Call Tool (Validation Error)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "http_get",
    "arguments": {
      "url": "not-a-valid-url"
    }
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
    "message": "Invalid params: parameter 'url' must be valid URI",
    "data": {
      "parameter": "url",
      "expected": "string (format: uri)",
      "received": "not-a-valid-url"
    }
  }
}
```

---

### Example 4: Call Tool (Execution Error)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "http_get",
    "arguments": {
      "url": "https://nonexistent.example.com/status"
    }
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
    "message": "Internal error: tool execution failed",
    "data": {
      "toolName": "http_get",
      "error": "Get \"https://nonexistent.example.com/status\": dial tcp: lookup nonexistent.example.com: no such host"
    }
  }
}
```

---

## Security Considerations

### Input Sanitization

- Tool inputs MUST be validated against JSON Schema before execution
- Server SHOULD reject excessively large input objects (>1MB recommended limit)
- Server SHOULD enforce timeouts on tool execution (prevent infinite loops)

### Tool Isolation

- Tools MUST NOT access global state unless explicitly designed to do so
- Tool execution errors MUST NOT leak sensitive internal details (stack traces, file paths)
- Tool descriptions SHOULD NOT expose sensitive system information

### Error Messages

**Safe Error**:
```json
{
  "code": -32603,
  "message": "Internal error: database query failed"
}
```

**Unsafe Error** (DO NOT):
```json
{
  "code": -32603,
  "message": "Internal error: SQL query failed at /home/user/app/db.go:42 with connection string postgres://user:password@localhost/db"
}
```

---

## Performance Considerations

- Tool listing (`tools/list`) SHOULD complete in <100ms
- Tool invocation overhead (validation + marshalling) SHOULD be <50ms
- Long-running tools (>10s) SHOULD be explicitly documented in descriptions
- Server SHOULD limit concurrent tool executions (recommend 100 max)

---

## Reference

- **JSON Schema Draft 7**: https://json-schema.org/draft-07/json-schema-release-notes.html
- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **LangGraph Tool Interface**: `graph/tool/tool.go`
