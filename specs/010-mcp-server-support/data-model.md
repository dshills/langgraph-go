# Data Model: MCP Server Support

**Feature**: MCP Server Support
**Date**: 2025-11-17
**Status**: Draft

## Overview

This document defines the data entities and their relationships for the MCP (Model Context Protocol) server implementation. The MCP server acts as an adapter layer between external LLM applications and LangGraph workflows, exposing tools, resources, and prompts via JSON-RPC 2.0 protocol.

**Key Principle**: MCP server does NOT introduce new state management. It exposes existing LangGraph entities (Tools, State, Checkpoints) through protocol adapters.

---

## Entity Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         MCP Client                               │
│                  (Claude Desktop, VS Code, etc.)                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │ JSON-RPC 2.0 over stdio
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                        MCPServer                                 │
│  ┌──────────────┐  ┌─────────────────┐  ┌─────────────────┐    │
│  │ ToolRegistry │  │ResourceProvider │  │PromptRegistry   │    │
│  └──────┬───────┘  └────────┬────────┘  └────────┬────────┘    │
│         │                   │                     │             │
└─────────┼───────────────────┼─────────────────────┼─────────────┘
          │                   │                     │
          ▼                   ▼                     ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│  LangGraph Tool  │ │ LangGraph Store  │ │  Prompt Template │
│  (graph/tool/)   │ │  (graph/store/)  │ │  (in-memory)     │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

---

## Core Entities

### 1. MCPServer

**Description**: The main server component that implements the Model Context Protocol. Manages lifecycle, connection handling, and routing of JSON-RPC messages to appropriate handlers.

**Attributes**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `toolRegistry` | `*ToolRegistry` | Yes | Registry of available tools |
| `resourceProvider` | `ResourceProvider` | Yes | Provider for state/checkpoint resources |
| `promptRegistry` | `*PromptRegistry` | Yes | Registry of prompt templates |
| `conn` | `*jsonrpc2.Conn` | Yes | JSON-RPC connection |
| `ctx` | `context.Context` | Yes | Server lifecycle context |
| `emitter` | `emit.Emitter` | No | Observability emitter (optional) |
| `config` | `ServerConfig` | Yes | Server configuration |

**Lifecycle States**:

1. **Uninitialized**: Server created but not started
2. **Initializing**: Capability negotiation with client
3. **Running**: Accepting and processing requests
4. **Stopping**: Graceful shutdown in progress
5. **Stopped**: Server terminated

**State Transitions**:

```
Uninitialized → [Start()] → Initializing → [initialize request] → Running
Running → [Stop()] → Stopping → [cleanup complete] → Stopped
Running → [context cancelled] → Stopping → Stopped
```

**Relationships**:

- **HAS-ONE** ToolRegistry
- **HAS-ONE** ResourceProvider
- **HAS-ONE** PromptRegistry
- **CONNECTS-TO** MCP Client (via JSON-RPC connection)

**Validation Rules**:

- Must have at least one registered tool, resource, or prompt (empty server is invalid)
- Cannot start if already running
- Cannot register tools/resources/prompts while in Running state (registration during initialization only)

---

### 2. ToolRegistry

**Description**: Maintains a mapping of tool names to LangGraph Tool implementations and their MCP metadata. Handles tool discovery and invocation.

**Attributes**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tools` | `map[string]*RegisteredTool` | Yes | Map of tool name → registered tool |
| `mu` | `sync.RWMutex` | Yes | Protects concurrent access to tools map |

**RegisteredTool** (nested entity):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `tool` | `tool.Tool` | Yes | LangGraph Tool implementation |
| `metadata` | `ToolMetadata` | Yes | MCP-specific metadata |

**ToolMetadata** (nested entity):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Tool identifier (must be unique) |
| `description` | `string` | Yes | Human-readable description |
| `inputSchema` | `map[string]interface{}` | Yes | JSON Schema for tool parameters |

**Operations**:

- `Register(name string, tool tool.Tool, metadata ToolMetadata) error`: Register a tool
- `Get(name string) (*RegisteredTool, error)`: Retrieve tool by name
- `List() []ToolMetadata`: List all tool metadata
- `Invoke(ctx context.Context, name string, input map[string]interface{}) (map[string]interface{}, error)`: Execute tool

**Relationships**:

- **WRAPS** many LangGraph Tools (graph/tool.Tool)
- **OWNED-BY** MCPServer

**Validation Rules**:

- Tool names must be unique within registry
- Tool names must match pattern: `^[a-z][a-z0-9_]*$` (lowercase, underscores)
- Input schema must be valid JSON Schema
- Tool description must be non-empty

**Invariants**:

- Once registered, tools cannot be unregistered (immutable after initialization)
- Tool execution must respect context cancellation
- Tool errors must be mapped to JSON-RPC error codes

---

### 3. ResourceProvider

**Description**: Interface for providing read-only access to workflow data (state, checkpoints, metrics). Supports both static and dynamic resources.

**Attributes**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `resources` | `map[string]Resource` | Yes | Map of URI → resource |
| `mu` | `sync.RWMutex` | Yes | Protects concurrent access |

**Resource** (interface):

| Method | Returns | Description |
|--------|---------|-------------|
| `URI() string` | `string` | Unique resource identifier |
| `MIMEType() string` | `string` | Content type (e.g., "application/json") |
| `Read(ctx context.Context) ([]byte, error)` | `[]byte, error` | Fetch resource content |

**Resource Types**:

1. **StaticResource**: Fixed content, computed once
2. **DynamicResource**: Computed on each read (e.g., current metrics)

**StaticResource** (implementation):

| Field | Type | Description |
|-------|------|-------------|
| `uri` | `string` | Resource URI |
| `mimeType` | `string` | Content type |
| `content` | `[]byte` | Cached content |

**DynamicResource** (implementation):

| Field | Type | Description |
|-------|------|-------------|
| `uri` | `string` | Resource URI |
| `mimeType` | `string` | Content type |
| `generator` | `func(context.Context) ([]byte, error)` | Function to generate content |

**Operations**:

- `RegisterStatic(uri string, mimeType string, content []byte) error`: Register static resource
- `RegisterDynamic(uri string, mimeType string, generator func(context.Context) ([]byte, error)) error`: Register dynamic resource
- `Get(uri string) (Resource, error)`: Retrieve resource by URI
- `List() []ResourceInfo`: List all resources (URI + MIME type)
- `Read(ctx context.Context, uri string) ([]byte, error)`: Read resource content

**Relationships**:

- **READS-FROM** LangGraph Store (for state/checkpoint resources)
- **OWNED-BY** MCPServer

**Validation Rules**:

- URIs must be unique
- URIs must match pattern: `^[a-z][a-z0-9_/]*$` (lowercase, underscores, slashes)
- MIME types must be valid (e.g., "application/json", "text/plain")
- Resource content must be under 10MB (configurable limit)

**Common Resource URI Patterns**:

| Pattern | Example | Description |
|---------|---------|-------------|
| `workflow_state` | `workflow_state` | Current workflow state |
| `checkpoints/{label}` | `checkpoints/iteration-5` | Specific checkpoint |
| `checkpoints/latest` | `checkpoints/latest` | Most recent checkpoint |
| `metrics/runtime` | `metrics/runtime` | Runtime metrics |
| `history/{runID}` | `history/run-123` | Execution history |

---

### 4. PromptRegistry

**Description**: Manages prompt templates that guide LLM interactions with workflows. Templates support parameter substitution.

**Attributes**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompts` | `map[string]*PromptTemplate` | Yes | Map of prompt name → template |
| `mu` | `sync.RWMutex` | Yes | Protects concurrent access |

**PromptTemplate** (nested entity):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Prompt identifier (unique) |
| `description` | `string` | Yes | Human-readable description |
| `parameters` | `[]PromptParameter` | No | Template parameters |
| `template` | `string` | Yes | Template string with `{{param}}` placeholders |

**PromptParameter** (nested entity):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Parameter name |
| `description` | `string` | No | Parameter description |
| `required` | `bool` | Yes | Whether parameter is required |
| `defaultValue` | `string` | No | Default value if not provided |

**Operations**:

- `Register(template PromptTemplate) error`: Register prompt template
- `Get(name string) (*PromptTemplate, error)`: Retrieve template by name
- `List() []PromptInfo`: List all prompt metadata
- `Render(name string, arguments map[string]string) (string, error)`: Render template with arguments

**Relationships**:

- **OWNED-BY** MCPServer
- **REFERENCES** Tools (prompts may reference available tools in description)

**Validation Rules**:

- Prompt names must be unique
- Prompt names must match pattern: `^[a-z][a-z0-9_]*$`
- Template must contain valid `{{param}}` placeholders
- Required parameters must be provided during rendering
- Template parameters must match names in template string

**Common Prompt Templates**:

| Name | Description | Parameters |
|------|-------------|------------|
| `start_workflow` | Start a workflow with inputs | `workflow_id`, `input_data` |
| `resume_from_checkpoint` | Resume workflow from checkpoint | `run_id`, `checkpoint_label` |
| `analyze_results` | Analyze workflow results | `run_id` |
| `list_available_tools` | List all available tools | (none) |

---

### 5. ConnectionSession

**Description**: Represents a stateful connection between MCP client and server. Maintains protocol version, capabilities, and authentication context.

**Attributes**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `clientInfo` | `ClientInfo` | Yes | Client identification |
| `protocolVersion` | `string` | Yes | MCP protocol version (e.g., "2025-06-18") |
| `capabilities` | `Capabilities` | Yes | Negotiated capabilities |
| `connectionTime` | `time.Time` | Yes | When connection established |

**ClientInfo** (nested entity):

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Client name (e.g., "Claude Desktop") |
| `version` | `string` | Client version |

**Capabilities** (nested entity):

| Field | Type | Description |
|-------|------|-------------|
| `tools` | `bool` | Client supports tools |
| `resources` | `bool` | Client supports resources |
| `prompts` | `bool` | Client supports prompts |

**Lifecycle**:

```
[initialize request] → Negotiate capabilities → Active
Active → [context cancelled or close] → Closed
```

**Validation Rules**:

- Protocol version must match supported versions (currently "2025-06-18")
- At least one capability must be enabled
- Cannot change capabilities after initialization

---

## JSON-RPC Protocol Entities

### 6. MCP Request/Response Messages

These are protocol-level entities (not stored, transient during message processing).

**Request**:

| Field | Type | Description |
|-------|------|-------------|
| `jsonrpc` | `"2.0"` | JSON-RPC version (always "2.0") |
| `id` | `string \| number` | Request ID (for correlation) |
| `method` | `string` | MCP method (e.g., "tools/list") |
| `params` | `object` | Method-specific parameters |

**Response**:

| Field | Type | Description |
|-------|------|-------------|
| `jsonrpc` | `"2.0"` | JSON-RPC version |
| `id` | `string \| number` | Request ID (matches request) |
| `result` | `any` | Method result (if successful) |
| `error` | `Error \| null` | Error object (if failed) |

**Error**:

| Field | Type | Description |
|-------|------|-------------|
| `code` | `int` | JSON-RPC error code |
| `message` | `string` | Error message |
| `data` | `any` | Additional error context |

**Standard Error Codes**:

| Code | Name | When to Use |
|------|------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid request | Malformed JSON-RPC |
| -32601 | Method not found | Unknown MCP method |
| -32602 | Invalid params | Missing/invalid parameters |
| -32603 | Internal error | Tool execution failed |

---

## Data Flow Examples

### Example 1: Tool Invocation

```
1. MCP Client → MCP Server
   {
     "jsonrpc": "2.0",
     "id": 1,
     "method": "tools/call",
     "params": {
       "name": "weather",
       "input": {"location": "San Francisco"}
     }
   }

2. MCP Server → ToolRegistry.Invoke("weather", {...})

3. ToolRegistry → LangGraph Tool.Call(ctx, {"location": "San Francisco"})

4. LangGraph Tool → returns {"temperature": 72, "conditions": "sunny"}

5. MCP Server → MCP Client
   {
     "jsonrpc": "2.0",
     "id": 1,
     "result": {
       "content": [
         {
           "type": "text",
           "text": "{\"temperature\":72,\"conditions\":\"sunny\"}"
         }
       ]
     }
   }
```

### Example 2: Resource Read

```
1. MCP Client → MCP Server
   {
     "jsonrpc": "2.0",
     "id": 2,
     "method": "resources/read",
     "params": {
       "uri": "workflow_state"
     }
   }

2. MCP Server → ResourceProvider.Read(ctx, "workflow_state")

3. ResourceProvider → LangGraph Store.LoadLatest(ctx, runID)

4. LangGraph Store → returns State {Messages: [...], Counter: 5}

5. MCP Server → MCP Client
   {
     "jsonrpc": "2.0",
     "id": 2,
     "result": {
       "contents": [
         {
           "uri": "workflow_state",
           "mimeType": "application/json",
           "text": "{\"Messages\":[...],\"Counter\":5}"
         }
       ]
     }
   }
```

### Example 3: Prompt Rendering

```
1. MCP Client → MCP Server
   {
     "jsonrpc": "2.0",
     "id": 3,
     "method": "prompts/get",
     "params": {
       "name": "start_workflow",
       "arguments": {
         "workflow_id": "data-pipeline",
         "input_data": "customers.csv"
       }
     }
   }

2. MCP Server → PromptRegistry.Render("start_workflow", {workflow_id: ..., input_data: ...})

3. PromptRegistry → Template substitution
   "Start workflow {{workflow_id}} with data: {{input_data}}"
   → "Start workflow data-pipeline with data: customers.csv"

4. MCP Server → MCP Client
   {
     "jsonrpc": "2.0",
     "id": 3,
     "result": {
       "description": "Start a workflow with parameters",
       "messages": [
         {
           "role": "user",
           "content": {
             "type": "text",
             "text": "Start workflow data-pipeline with data: customers.csv"
           }
         }
       ]
     }
   }
```

---

## Persistence

**Note**: MCP server entities are NOT persisted. All data is in-memory during server runtime.

**Rationale**:
- MCP server is stateless (tools/resources/prompts registered at startup)
- Tool execution uses existing LangGraph Store for persistence
- Resources read from existing LangGraph Store (checkpoints, state)
- No need for separate MCP-specific persistence

**Exception**: Server configuration (which tools/resources to expose) may be persisted in config files, but this is deployment-level configuration, not runtime state.

---

## Concurrency Model

### Thread Safety

| Entity | Concurrency Pattern | Rationale |
|--------|---------------------|-----------|
| **ToolRegistry** | `sync.RWMutex` | Multiple readers, rare writes (registration) |
| **ResourceProvider** | `sync.RWMutex` | Multiple readers, rare writes (registration) |
| **PromptRegistry** | `sync.RWMutex` | Multiple readers, rare writes (registration) |
| **ConnectionSession** | Read-only after init | No locking needed |

### Goroutine Usage

- **One goroutine per client connection**: JSON-RPC connection runs in dedicated goroutine
- **Tool execution**: Synchronous in request goroutine (no spawning)
- **Resource generation**: Synchronous in request goroutine
- **Prompt rendering**: Synchronous in request goroutine

**Rationale**: LangGraph tools are already designed for concurrent execution. MCP server doesn't need additional concurrency management.

---

## Entity Relationships Summary

```
MCPServer (1) ─── HAS-ONE ───> ToolRegistry (1)
                                    │
                                    │ WRAPS
                                    ▼
                                LangGraph Tool (many)

MCPServer (1) ─── HAS-ONE ───> ResourceProvider (1)
                                    │
                                    │ READS-FROM
                                    ▼
                                LangGraph Store (1)

MCPServer (1) ─── HAS-ONE ───> PromptRegistry (1)
                                    │
                                    │ CONTAINS
                                    ▼
                                PromptTemplate (many)

MCPServer (1) ─── CONNECTS-TO ──> MCP Client (many)
                                    │
                                    │ VIA
                                    ▼
                                ConnectionSession (many)
```

---

## Validation Matrix

| Entity | Validation Type | When Validated | Error Code |
|--------|----------------|----------------|------------|
| Tool name | Regex match | Registration | N/A (panic) |
| Tool input schema | JSON Schema | Registration | N/A (panic) |
| Tool input params | JSON Schema | Tool call | -32602 (invalid params) |
| Resource URI | Regex match | Registration | N/A (panic) |
| Resource size | <10MB | Read | -32603 (internal error) |
| Prompt name | Regex match | Registration | N/A (panic) |
| Prompt parameters | Required check | Render | -32602 (invalid params) |
| JSON-RPC message | Protocol spec | Message parse | -32700 (parse error) |
| MCP method | Known methods | Method dispatch | -32601 (method not found) |

**Validation Philosophy**:
- **Registration time**: Fail fast with panics (invalid configuration)
- **Request time**: Return JSON-RPC errors (invalid user input)

---

## Size Estimates

| Entity | Instances | Memory per Instance | Total Memory |
|--------|-----------|---------------------|--------------|
| MCPServer | 1 | ~1KB | 1KB |
| ToolRegistry | 1 | ~50KB (50 tools) | 50KB |
| RegisteredTool | 50 | ~1KB | 50KB |
| ResourceProvider | 1 | ~10KB (20 resources) | 10KB |
| Resource | 20 | ~500KB (avg size) | 10MB |
| PromptRegistry | 1 | ~5KB (10 prompts) | 5KB |
| PromptTemplate | 10 | ~500B | 5KB |
| ConnectionSession | 100 | ~500B | 50KB |

**Total Estimated Memory**: ~10MB for typical server with 50 tools, 20 resources, 10 prompts, 100 clients

**Scalability**: Memory usage scales linearly with number of registered items. For 100+ tools or large resources (>1MB), consider pagination or streaming (future enhancement).

---

## Next Phase

**Proceed to Contracts**: Define API contracts for MCP protocol methods (tools/list, tools/call, resources/read, etc.)
