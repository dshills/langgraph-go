# MCP Protocol Contract

**Feature**: MCP Server Support
**Version**: MCP Specification 2025-06-18
**Protocol**: JSON-RPC 2.0
**Date**: 2025-11-17

## Overview

This contract defines the core Model Context Protocol (MCP) messaging format and initialization handshake. MCP uses JSON-RPC 2.0 as the base protocol for all client-server communication over stdio transport.

**Transport**: Stdio with Content-Length headers (VSCodeObjectCodec format)
**Encoding**: UTF-8 JSON

---

## JSON-RPC 2.0 Base Format

All MCP messages follow the JSON-RPC 2.0 specification.

### Request Message

**Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": string | number,
  "method": string,
  "params": object | array | null
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `jsonrpc` | `string` | Yes | MUST be exactly `"2.0"` |
| `id` | `string \| number` | Yes | Request identifier for response correlation. MUST be unique per request. |
| `method` | `string` | Yes | MCP method name (e.g., `"initialize"`, `"tools/list"`) |
| `params` | `object \| array \| null` | No | Method-specific parameters. Omit or use `null` if no parameters. |

**Validation Rules**:
- `jsonrpc` MUST be the string `"2.0"` exactly
- `id` MUST be unique within the connection session
- `id` MUST NOT be `null` (use notification format if no response needed)
- `method` MUST be a non-empty string
- `params` MUST be structured data (object or array) or omitted/null

**Example (Valid Request)**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "tools": {}
    },
    "clientInfo": {
      "name": "Claude Desktop",
      "version": "0.7.0"
    }
  }
}
```

---

### Response Message (Success)

**Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": string | number,
  "result": any
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `jsonrpc` | `string` | Yes | MUST be exactly `"2.0"` |
| `id` | `string \| number` | Yes | MUST match the `id` of the corresponding request |
| `result` | `any` | Yes | Method-specific result data. Can be any JSON type (object, array, string, number, boolean, null). |

**Validation Rules**:
- `id` MUST match the request `id` exactly
- `result` field MUST be present (even if `null`)
- `error` field MUST NOT be present in success responses

**Example (Valid Response)**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "tools": {},
      "resources": {}
    },
    "serverInfo": {
      "name": "LangGraph MCP Server",
      "version": "0.1.0"
    }
  }
}
```

---

### Response Message (Error)

**Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": string | number | null,
  "error": {
    "code": number,
    "message": string,
    "data": any
  }
}
```

**Error Object Schema**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `code` | `number` | Yes | Standard JSON-RPC error code (integer) |
| `message` | `string` | Yes | Human-readable error description |
| `data` | `any` | No | Additional error context (optional) |

**Standard Error Codes**:

| Code | Name | Meaning | When to Use |
|------|------|---------|-------------|
| `-32700` | Parse error | Invalid JSON received | JSON syntax errors, malformed UTF-8 |
| `-32600` | Invalid Request | JSON-RPC format violation | Missing required fields, wrong types |
| `-32601` | Method not found | Unknown method | Unsupported or misspelled method name |
| `-32602` | Invalid params | Invalid method parameters | Missing required params, type mismatches |
| `-32603` | Internal error | Server-side failure | Tool execution errors, internal exceptions |

**Validation Rules**:
- `id` MUST match request `id`, or `null` if request `id` was invalid
- `result` field MUST NOT be present in error responses
- `error.code` MUST be an integer (standard codes are negative)
- `error.message` MUST be a non-empty string
- `error.data` is OPTIONAL and can contain any JSON type

**Example (Invalid Params Error)**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": -32602,
    "message": "Invalid params: missing required parameter 'name'",
    "data": {
      "parameter": "name",
      "expected": "string",
      "received": null
    }
  }
}
```

**Example (Method Not Found Error)**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "error": {
    "code": -32601,
    "message": "Method not found: 'unknown/method'",
    "data": {
      "method": "unknown/method",
      "supportedMethods": ["initialize", "tools/list", "tools/call"]
    }
  }
}
```

---

### Notification Message

**Schema**:
```json
{
  "jsonrpc": "2.0",
  "method": string,
  "params": object | array | null
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `jsonrpc` | `string` | Yes | MUST be exactly `"2.0"` |
| `method` | `string` | Yes | Notification method name |
| `params` | `object \| array \| null` | No | Method-specific parameters |

**Validation Rules**:
- Notifications MUST NOT include an `id` field
- Server MUST NOT send a response to notifications
- Clients MUST NOT expect responses to notifications

**Example (Notification)**:
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/progress",
  "params": {
    "progressToken": "task-123",
    "progress": 50,
    "total": 100
  }
}
```

**Note**: MCP 2025-06-18 does not currently define standard notifications, but the format is reserved for future extensions.

---

## MCP Initialization

### `initialize` Request

The `initialize` method MUST be the first method called by a client. It establishes the protocol version and negotiates capabilities between client and server.

**Method**: `initialize`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "initialize",
  "params": {
    "protocolVersion": string,
    "capabilities": {
      "tools": object | null,
      "resources": object | null,
      "prompts": object | null
    },
    "clientInfo": {
      "name": string,
      "version": string
    }
  }
}
```

**Request Parameters**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `protocolVersion` | `string` | Yes | MCP protocol version. MUST be `"2025-06-18"` or compatible. |
| `capabilities` | `object` | Yes | Client capabilities declaration |
| `capabilities.tools` | `object \| null` | No | Presence indicates client supports tools. Value is reserved for future options. Use `{}` to indicate support. |
| `capabilities.resources` | `object \| null` | No | Presence indicates client supports resources. Use `{}` to indicate support. |
| `capabilities.prompts` | `object \| null` | No | Presence indicates client supports prompts. Use `{}` to indicate support. |
| `clientInfo` | `object` | Yes | Client identification |
| `clientInfo.name` | `string` | Yes | Client name (e.g., `"Claude Desktop"`) |
| `clientInfo.version` | `string` | Yes | Client version (e.g., `"0.7.0"`) |

**Validation Rules**:
- `protocolVersion` MUST match server-supported versions
- At least one capability MUST be declared (empty capabilities object is invalid)
- `clientInfo.name` and `clientInfo.version` MUST be non-empty strings

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "tools": {},
      "resources": {},
      "prompts": {}
    },
    "clientInfo": {
      "name": "Claude Desktop",
      "version": "0.7.0"
    }
  }
}
```

---

### `initialize` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "protocolVersion": string,
    "capabilities": {
      "tools": object | null,
      "resources": object | null,
      "prompts": object | null
    },
    "serverInfo": {
      "name": string,
      "version": string
    }
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `protocolVersion` | `string` | Yes | MCP protocol version supported by server. MUST be `"2025-06-18"`. |
| `capabilities` | `object` | Yes | Server capabilities declaration |
| `capabilities.tools` | `object \| null` | No | Server supports tool operations if present |
| `capabilities.resources` | `object \| null` | No | Server supports resource operations if present |
| `capabilities.prompts` | `object \| null` | No | Server supports prompt operations if present |
| `serverInfo` | `object` | Yes | Server identification |
| `serverInfo.name` | `string` | Yes | Server name (e.g., `"LangGraph MCP Server"`) |
| `serverInfo.version` | `string` | Yes | Server version (e.g., `"0.1.0"`) |

**Validation Rules**:
- `protocolVersion` MUST match client request version or be a compatible version
- Server MUST only declare capabilities it actually supports
- `serverInfo.name` and `serverInfo.version` MUST be non-empty strings

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "tools": {},
      "resources": {}
    },
    "serverInfo": {
      "name": "LangGraph MCP Server",
      "version": "0.1.0"
    }
  }
}
```

---

### `initialize` Error Cases

**Error: Unsupported Protocol Version**

**HTTP-like Code**: 505 Version Not Supported
**JSON-RPC Code**: `-32600` (Invalid Request)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Unsupported protocol version",
    "data": {
      "requested": "2024-01-01",
      "supported": ["2025-06-18"]
    }
  }
}
```

**Error: Missing Required Capabilities**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: no capabilities declared",
    "data": {
      "capabilities": {},
      "reason": "At least one capability (tools, resources, or prompts) must be declared"
    }
  }
}
```

**Error: Invalid Client Info**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params: clientInfo.name is required",
    "data": {
      "field": "clientInfo.name",
      "expected": "non-empty string",
      "received": ""
    }
  }
}
```

---

## Capability Negotiation

After successful `initialize`, the **effective capabilities** are the **intersection** of client and server declared capabilities.

**Negotiation Logic**:

| Client Declares | Server Declares | Result |
|----------------|-----------------|--------|
| `tools: {}` | `tools: {}` | Tools enabled |
| `tools: {}` | (not present) | Tools disabled |
| (not present) | `tools: {}` | Tools disabled |
| `resources: {}` | `resources: {}` | Resources enabled |

**Example Negotiation**:

**Client Request**:
```json
{
  "capabilities": {
    "tools": {},
    "prompts": {}
  }
}
```

**Server Response**:
```json
{
  "capabilities": {
    "tools": {},
    "resources": {}
  }
}
```

**Effective Capabilities**: Only `tools` enabled (both client and server support it). `resources` disabled (client doesn't support). `prompts` disabled (server doesn't support).

---

## Message Framing (Stdio Transport)

MCP over stdio uses **Content-Length** headers for message framing (VSCodeObjectCodec format).

**Message Format**:
```
Content-Length: <byte-count>\r\n
\r\n
<JSON-RPC message>
```

**Example**:
```
Content-Length: 158\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"clientInfo":{"name":"Test","version":"1.0"}}}
```

**Validation Rules**:
- Header MUST be `Content-Length: <decimal-number>\r\n`
- Header MUST be followed by `\r\n` (blank line)
- `<byte-count>` MUST match UTF-8 encoded JSON byte length exactly
- Message body MUST be valid UTF-8 encoded JSON

**Error Handling**:
- Invalid header format → Close connection (unrecoverable)
- Content-Length mismatch → Close connection (unrecoverable)
- Invalid JSON → Return `-32700` Parse error

---

## Protocol State Machine

```
┌─────────────┐
│ Unconnected │
└──────┬──────┘
       │ (connection established)
       ▼
┌─────────────────┐
│ Waiting for     │
│ 'initialize'    │
└──────┬──────────┘
       │ (initialize request)
       ▼
┌─────────────────┐
│ Initializing    │
│ (negotiating)   │
└──────┬──────────┘
       │ (initialize response sent)
       ▼
┌─────────────────┐
│ Ready           │◄───┐
│ (accepting      │    │
│  method calls)  │    │ (method calls)
└──────┬──────────┘────┘
       │ (connection closed or error)
       ▼
┌─────────────┐
│ Disconnected│
└─────────────┘
```

**State Transitions**:

1. **Unconnected → Waiting for 'initialize'**: Connection established via stdio
2. **Waiting → Initializing**: `initialize` request received
3. **Initializing → Ready**: `initialize` response sent successfully
4. **Ready → Ready**: Method calls processed (tools/list, tools/call, etc.)
5. **Any → Disconnected**: Connection closed, fatal error, or context cancellation

**Rules**:
- First message from client MUST be `initialize`
- Clients MUST NOT send other method calls before `initialize` completes
- Server MAY reject non-initialize methods with `-32600` Invalid Request if sent before initialization

---

## Transport Error Handling

### Recoverable Errors

Errors that can be reported via JSON-RPC error responses:

| Error Type | JSON-RPC Code | Recovery |
|------------|---------------|----------|
| Parse error (invalid JSON) | `-32700` | Send error response, continue |
| Invalid request format | `-32600` | Send error response, continue |
| Method not found | `-32601` | Send error response, continue |
| Invalid parameters | `-32602` | Send error response, continue |
| Internal error | `-32603` | Send error response, continue |

### Unrecoverable Errors

Errors that require connection termination:

| Error Type | Action |
|------------|--------|
| Content-Length header malformed | Close connection immediately |
| Content-Length mismatch | Close connection immediately |
| Connection context cancelled | Close connection gracefully |
| Stdio stream errors (pipe broken) | Close connection gracefully |

---

## Examples

### Example 1: Successful Initialization

**Client → Server**:
```
Content-Length: 185\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{"tools":{},"resources":{}},"clientInfo":{"name":"Test Client","version":"1.0.0"}}}
```

**Server → Client**:
```
Content-Length: 165\r\n
\r\n
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{},"resources":{}},"serverInfo":{"name":"LangGraph","version":"0.1.0"}}}
```

---

### Example 2: Parse Error

**Client → Server** (invalid JSON):
```
Content-Length: 50\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize",INVALID
```

**Server → Client**:
```
Content-Length: 107\r\n
\r\n
{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error: invalid JSON","data":{"position":45}}}
```

---

### Example 3: Method Not Found

**Client → Server**:
```
Content-Length: 68\r\n
\r\n
{"jsonrpc":"2.0","id":2,"method":"unknown/method","params":{"foo":"bar"}}
```

**Server → Client**:
```
Content-Length: 151\r\n
\r\n
{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"Method not found: 'unknown/method'","data":{"method":"unknown/method","available":["tools/list"]}}}
```

---

## Validation Checklist

Server implementations MUST validate:

- [ ] `jsonrpc` field is exactly `"2.0"`
- [ ] `id` is present for requests (string or number)
- [ ] `id` is NOT `null` for requests
- [ ] `method` is a non-empty string
- [ ] `params` is object, array, or omitted/null
- [ ] `initialize` is called before other methods
- [ ] `protocolVersion` matches supported versions
- [ ] At least one capability declared in `initialize`
- [ ] `clientInfo.name` and `clientInfo.version` are non-empty
- [ ] Content-Length header matches message byte length
- [ ] JSON is valid UTF-8 encoded

Server implementations MUST NOT:

- [ ] Accept methods before `initialize` completes
- [ ] Send both `result` and `error` in same response
- [ ] Use `id: null` in error responses unless request `id` was invalid
- [ ] Continue connection after unrecoverable transport errors

---

## Reference

- **JSON-RPC 2.0 Specification**: https://www.jsonrpc.org/specification
- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **VSCode LSP Object Codec**: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#baseProtocol
