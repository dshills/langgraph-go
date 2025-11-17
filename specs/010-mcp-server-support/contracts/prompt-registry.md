# Prompt Registry Contract

**Feature**: MCP Server Support
**Version**: MCP Specification 2025-06-18
**Date**: 2025-11-17

## Overview

This contract defines the MCP Prompt operations that expose reusable prompt templates to external clients. Prompts are templated message patterns with parameter substitution, designed to guide LLM interactions with workflows using standardized formats.

**Key Principles**:
- Prompts are templates with named parameters
- Parameters support required/optional with defaults
- Templates use `{{parameter}}` placeholder syntax
- Rendered prompts ready for LLM consumption

---

## Prompt Discovery

### `prompts/list` Request

Retrieve the list of all prompt templates available on the MCP server.

**Method**: `prompts/list`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "prompts/list",
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
  "method": "prompts/list"
}
```

---

### `prompts/list` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "prompts": [
      {
        "name": string,
        "description": string,
        "arguments": [
          {
            "name": string,
            "description": string,
            "required": boolean
          }
        ]
      }
    ]
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompts` | `array` | Yes | List of available prompt templates. Empty array `[]` if no prompts registered. |
| `prompts[].name` | `string` | Yes | Unique prompt identifier. MUST match pattern `^[a-z][a-z0-9_]*$` (lowercase, underscores). |
| `prompts[].description` | `string` | Yes | Human-readable description of prompt purpose. |
| `prompts[].arguments` | `array` | No | List of template parameters. Empty array or omitted if no parameters. |
| `arguments[].name` | `string` | Yes | Parameter name (used in template placeholders). |
| `arguments[].description` | `string` | No | Parameter description (optional). |
| `arguments[].required` | `boolean` | Yes | Whether parameter is required (`true`) or optional (`false`). |

**Prompt Name Pattern**: `^[a-z][a-z0-9_]*$`
- MUST start with lowercase letter
- MAY contain lowercase letters, digits, underscores
- Examples: `start_workflow`, `resume_checkpoint`, `analyze_results`

**Validation Rules**:
- `prompts` array MAY be empty (valid if no prompts registered)
- Prompt names MUST be unique within the array
- `description` MUST be non-empty string
- `arguments` array MAY be empty (prompts without parameters)
- Parameter names MUST be unique within a prompt's arguments
- Parameter names SHOULD match pattern `^[a-z][a-z0-9_]*$` (recommended)

**Example Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "prompts": [
      {
        "name": "start_workflow",
        "description": "Start a workflow with specified parameters",
        "arguments": [
          {
            "name": "workflow_id",
            "description": "Unique identifier for the workflow to start",
            "required": true
          },
          {
            "name": "input_data",
            "description": "Input data for the workflow",
            "required": false
          }
        ]
      },
      {
        "name": "resume_checkpoint",
        "description": "Resume workflow execution from a saved checkpoint",
        "arguments": [
          {
            "name": "run_id",
            "description": "Workflow run identifier",
            "required": true
          },
          {
            "name": "checkpoint_label",
            "description": "Label of the checkpoint to resume from",
            "required": true
          }
        ]
      },
      {
        "name": "list_available_tools",
        "description": "List all tools available in this workflow",
        "arguments": []
      }
    ]
  }
}
```

---

### `prompts/list` Error Cases

**Error: Prompts Capability Not Enabled**

**JSON-RPC Code**: `-32601` (Method not found)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found: 'prompts/list'",
    "data": {
      "reason": "Prompts capability not negotiated during initialization",
      "availableCapabilities": ["tools", "resources"]
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
    "message": "Invalid params: prompts/list does not accept parameters",
    "data": {
      "received": {"filter": "workflow"}
    }
  }
}
```

---

## Prompt Rendering

### `prompts/get` Request

Render a prompt template with provided argument values.

**Method**: `prompts/get`

**Request Schema**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "method": "prompts/get",
  "params": {
    "name": string,
    "arguments": object
  }
}
```

**Request Parameters**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Prompt name (MUST match a prompt from `prompts/list`) |
| `arguments` | `object` | No | Parameter values for template substitution. Keys are parameter names, values are strings. If omitted, defaults to `{}`. |

**Validation Rules**:
- `name` MUST match a registered prompt name exactly
- `arguments` MUST be an object (not array or primitive)
- All required parameters MUST be present in `arguments`
- Parameter values MUST be strings
- Extra arguments (not defined in prompt) MAY be ignored

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/get",
  "params": {
    "name": "start_workflow",
    "arguments": {
      "workflow_id": "data-pipeline",
      "input_data": "customers.csv"
    }
  }
}
```

**Example Request (Required Only)**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "prompts/get",
  "params": {
    "name": "resume_checkpoint",
    "arguments": {
      "run_id": "run-abc123",
      "checkpoint_label": "iteration-5"
    }
  }
}
```

**Example Request (No Arguments)**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "prompts/get",
  "params": {
    "name": "list_available_tools"
  }
}
```

---

### `prompts/get` Response

**Response Schema (Success)**:
```json
{
  "jsonrpc": "2.0",
  "id": number | string,
  "result": {
    "description": string,
    "messages": [
      {
        "role": "user" | "assistant",
        "content": {
          "type": "text" | "image" | "resource",
          "text": string,
          "data": string,
          "mimeType": string,
          "resource": string
        }
      }
    ]
  }
}
```

**Response Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | `string` | No | Prompt description (typically same as from `prompts/list`) |
| `messages` | `array` | Yes | Array of rendered message objects ready for LLM consumption. MUST have at least one message. |
| `messages[].role` | `string` | Yes | Message role: `"user"` or `"assistant"` |
| `messages[].content` | `object` | Yes | Message content (text, image, or resource) |

**Content Types**:

**Text Content** (most common):
```json
{
  "type": "text",
  "text": string
}
```

**Image Content**:
```json
{
  "type": "image",
  "data": string,
  "mimeType": string
}
```

**Resource Content**:
```json
{
  "type": "resource",
  "resource": string
}
```

**Validation Rules**:
- `messages` array MUST NOT be empty
- `role` MUST be `"user"` or `"assistant"`
- Content type MUST be `"text"`, `"image"`, or `"resource"`
- `type: "text"` REQUIRES `text` field
- `type: "image"` REQUIRES `data` and `mimeType` fields
- `type: "resource"` REQUIRES `resource` field (URI)

**Example Response (Simple)**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "description": "Start a workflow with specified parameters",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Start workflow 'data-pipeline' with input data: customers.csv"
        }
      }
    ]
  }
}
```

**Example Response (Multi-Message)**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "description": "Resume workflow with context",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Resume workflow run-abc123 from checkpoint iteration-5"
        }
      },
      {
        "role": "assistant",
        "content": {
          "type": "text",
          "text": "I'll help you resume the workflow. Let me check the checkpoint state first."
        }
      },
      {
        "role": "user",
        "content": {
          "type": "resource",
          "resource": "checkpoints/iteration-5"
        }
      }
    ]
  }
}
```

---

### `prompts/get` Error Cases

**Error: Prompt Not Found**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "error": {
    "code": -32602,
    "message": "Invalid params: prompt 'unknown_prompt' not found",
    "data": {
      "promptName": "unknown_prompt",
      "availablePrompts": ["start_workflow", "resume_checkpoint", "list_available_tools"]
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
    "message": "Invalid params: missing required parameter 'workflow_id'",
    "data": {
      "promptName": "start_workflow",
      "parameter": "workflow_id",
      "required": true
    }
  }
}
```

**Error: Invalid Parameter Type**

**JSON-RPC Code**: `-32602` (Invalid params)

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32602,
    "message": "Invalid params: parameter 'workflow_id' must be string",
    "data": {
      "parameter": "workflow_id",
      "expected": "string",
      "received": "number",
      "value": 123
    }
  }
}
```

**Error: Template Rendering Failure**

**JSON-RPC Code**: `-32603` (Internal error)

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "error": {
    "code": -32603,
    "message": "Internal error: failed to render prompt template",
    "data": {
      "promptName": "complex_workflow",
      "error": "invalid template syntax at position 42"
    }
  }
}
```

---

## Prompt Registration (Server-Side)

This section documents the Go API for registering prompts with the MCP server (not part of JSON-RPC protocol).

**Go Interface** (`graph/mcp/prompt_registry.go`):

```go
type PromptTemplate struct {
    Name        string             // Prompt identifier (validated against pattern)
    Description string             // Human-readable description
    Parameters  []PromptParameter  // Template parameters
    Template    string             // Template string with {{param}} placeholders
}

type PromptParameter struct {
    Name         string // Parameter name
    Description  string // Parameter description (optional)
    Required     bool   // Whether parameter is required
    DefaultValue string // Default value if not provided (optional)
}

type PromptRegistry interface {
    // Register a prompt template
    Register(template PromptTemplate) error

    // Get template by name
    Get(name string) (*PromptTemplate, error)

    // List all prompt metadata
    List() []PromptInfo

    // Render template with arguments
    Render(name string, arguments map[string]string) (*RenderedPrompt, error)
}

type PromptInfo struct {
    Name        string
    Description string
    Arguments   []PromptParameter
}

type RenderedPrompt struct {
    Description string
    Messages    []Message
}

type Message struct {
    Role    string  // "user" or "assistant"
    Content Content
}

type Content struct {
    Type     string // "text", "image", "resource"
    Text     string
    Data     string // Base64 for images
    MimeType string
    Resource string // Resource URI
}
```

**Registration Example**:

```go
registry := mcp.NewPromptRegistry()

err := registry.Register(mcp.PromptTemplate{
    Name:        "start_workflow",
    Description: "Start a workflow with specified parameters",
    Parameters: []mcp.PromptParameter{
        {
            Name:        "workflow_id",
            Description: "Unique identifier for the workflow",
            Required:    true,
        },
        {
            Name:         "input_data",
            Description:  "Input data for the workflow",
            Required:     false,
            DefaultValue: "default.csv",
        },
    },
    Template: "Start workflow '{{workflow_id}}' with input data: {{input_data}}",
})
```

**Validation at Registration Time**:

The server MUST validate at registration:
- Prompt name matches pattern `^[a-z][a-z0-9_]*$`
- Prompt name is unique (no duplicates)
- Description is non-empty
- Template is non-empty
- All placeholders in template have corresponding parameters
- All parameters referenced in template are defined
- Parameter names are unique within the template

Invalid registrations MUST panic (fail-fast during server initialization).

---

## Template Syntax

### Placeholder Format

Prompts use `{{parameter_name}}` syntax for parameter substitution.

**Example Template**:
```
Start workflow '{{workflow_id}}' with input data: {{input_data}}
```

**With Arguments**:
```json
{
  "workflow_id": "data-pipeline",
  "input_data": "customers.csv"
}
```

**Rendered Output**:
```
Start workflow 'data-pipeline' with input data: customers.csv
```

---

### Default Values

Optional parameters can have default values used when not provided.

**Template Registration**:
```go
PromptParameter{
    Name:         "units",
    Description:  "Temperature units",
    Required:     false,
    DefaultValue: "celsius",
}
```

**Template**:
```
Get weather in {{units}} for {{location}}
```

**Arguments (no units provided)**:
```json
{
  "location": "San Francisco"
}
```

**Rendered Output**:
```
Get weather in celsius for San Francisco
```

---

### Escaping

To include literal `{{` or `}}` in output, escape with backslash:

**Template**:
```
Use \{{placeholder\}} syntax in templates
```

**Rendered Output**:
```
Use {{placeholder}} syntax in templates
```

---

## Common Prompt Templates

### Start Workflow

**Name**: `start_workflow`
**Description**: Start a workflow with parameters

**Template**:
```
Start workflow '{{workflow_id}}' with the following input data:

{{input_data}}

Please confirm you're ready to begin execution.
```

**Parameters**:
- `workflow_id` (required): Workflow identifier
- `input_data` (optional): Input data description

---

### Resume from Checkpoint

**Name**: `resume_checkpoint`
**Description**: Resume workflow from saved checkpoint

**Template**:
```
Resume workflow run {{run_id}} from checkpoint '{{checkpoint_label}}'.

The checkpoint was saved at step {{step}} in node '{{node_id}}'.

Do you want to continue from this point?
```

**Parameters**:
- `run_id` (required): Workflow run identifier
- `checkpoint_label` (required): Checkpoint label
- `step` (optional): Step number
- `node_id` (optional): Node identifier

---

### Analyze Results

**Name**: `analyze_results`
**Description**: Analyze workflow execution results

**Template**:
```
Analyze the results of workflow run {{run_id}}.

Key metrics:
- Total execution time: {{duration}}
- Nodes executed: {{node_count}}
- Final status: {{status}}

Please provide insights on performance and any issues encountered.
```

**Parameters**:
- `run_id` (required): Workflow run identifier
- `duration` (optional): Execution duration
- `node_count` (optional): Number of nodes executed
- `status` (optional): Final workflow status

---

### List Available Tools

**Name**: `list_available_tools`
**Description**: List all tools available in the workflow

**Template**:
```
List all tools available in this LangGraph workflow.

For each tool, provide:
1. Name and description
2. Input parameters
3. Example usage

Format the output as a structured guide.
```

**Parameters**: (none)

---

## Multi-Message Prompts

Prompts can include multiple messages to establish conversation context.

**Registration Example**:
```go
registry.Register(mcp.PromptTemplate{
    Name:        "guided_execution",
    Description: "Step-by-step workflow execution guide",
    Parameters: []mcp.PromptParameter{
        {Name: "workflow_id", Required: true},
    },
    Template: `[
        {
            "role": "user",
            "content": {"type": "text", "text": "I need help executing workflow {{workflow_id}}"}
        },
        {
            "role": "assistant",
            "content": {"type": "text", "text": "I'll guide you through the execution. First, let me check the workflow configuration."}
        },
        {
            "role": "user",
            "content": {"type": "resource", "resource": "workflow_state"}
        }
    ]`,
})
```

**Note**: Multi-message templates use JSON array format in the template string.

---

## Prompt with Resource References

Prompts can reference MCP resources for context.

**Template**:
```json
[
    {
        "role": "user",
        "content": {
            "type": "text",
            "text": "Analyze the current workflow state"
        }
    },
    {
        "role": "user",
        "content": {
            "type": "resource",
            "resource": "workflow_state"
        }
    }
]
```

**Rendered Response**:
```json
{
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Analyze the current workflow state"
      }
    },
    {
      "role": "user",
      "content": {
        "type": "resource",
        "resource": "workflow_state"
      }
    }
  ]
}
```

The LLM client will automatically fetch the resource content when rendering the prompt.

---

## Parameter Validation

### Required Parameters

Server MUST enforce required parameters.

**Template**:
```go
PromptTemplate{
    Parameters: []PromptParameter{
        {Name: "workflow_id", Required: true},
    },
}
```

**Valid Request**:
```json
{
  "arguments": {
    "workflow_id": "data-pipeline"
  }
}
```

**Invalid Request** (missing required):
```json
{
  "arguments": {}
}
```

**Error**:
```json
{
  "code": -32602,
  "message": "Invalid params: missing required parameter 'workflow_id'"
}
```

---

### Type Validation

All parameter values MUST be strings.

**Valid**:
```json
{
  "arguments": {
    "workflow_id": "data-pipeline",
    "step": "5"
  }
}
```

**Invalid** (number instead of string):
```json
{
  "arguments": {
    "workflow_id": "data-pipeline",
    "step": 5
  }
}
```

**Error**:
```json
{
  "code": -32602,
  "message": "Invalid params: parameter 'step' must be string, got number"
}
```

---

## Examples

### Example 1: List Prompts

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/list"
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "prompts": [
      {
        "name": "start_workflow",
        "description": "Start a workflow with parameters",
        "arguments": [
          {"name": "workflow_id", "description": "Workflow ID", "required": true},
          {"name": "input_data", "description": "Input data", "required": false}
        ]
      },
      {
        "name": "list_available_tools",
        "description": "List all available tools",
        "arguments": []
      }
    ]
  }
}
```

---

### Example 2: Get Prompt (Success)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "prompts/get",
  "params": {
    "name": "start_workflow",
    "arguments": {
      "workflow_id": "data-pipeline",
      "input_data": "customers.csv"
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
    "description": "Start a workflow with parameters",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Start workflow 'data-pipeline' with input data: customers.csv"
        }
      }
    ]
  }
}
```

---

### Example 3: Get Prompt (Missing Required Parameter)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "prompts/get",
  "params": {
    "name": "start_workflow",
    "arguments": {
      "input_data": "customers.csv"
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
    "message": "Invalid params: missing required parameter 'workflow_id'",
    "data": {
      "promptName": "start_workflow",
      "parameter": "workflow_id",
      "required": true
    }
  }
}
```

---

### Example 4: Get Prompt (With Defaults)

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "prompts/get",
  "params": {
    "name": "start_workflow",
    "arguments": {
      "workflow_id": "data-pipeline"
    }
  }
}
```

**Response** (uses default for `input_data`):
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "description": "Start a workflow with parameters",
    "messages": [
      {
        "role": "user",
        "content": {
          "type": "text",
          "text": "Start workflow 'data-pipeline' with input data: default.csv"
        }
      }
    ]
  }
}
```

---

## Security Considerations

### Template Injection

- Server MUST NOT execute code in templates (no eval, no script execution)
- Templates MUST be pre-registered (no dynamic template creation from client input)
- Parameter values MUST be treated as plain strings (no special interpretation)

**Safe**:
```
Template: "Start workflow {{workflow_id}}"
Arguments: {"workflow_id": "data-pipeline"}
Output: "Start workflow data-pipeline"
```

**Unsafe** (DO NOT):
```
Template: {{user_provided_template}}  // NEVER allow client-provided templates
```

### Information Disclosure

- Prompt descriptions SHOULD NOT leak sensitive system details
- Template rendering errors SHOULD NOT expose internal file paths or stack traces

---

## Performance Considerations

- Prompt listing (`prompts/list`) SHOULD complete in <100ms
- Prompt rendering (`prompts/get`) SHOULD complete in <50ms (simple string substitution)
- Server SHOULD limit maximum template size (recommend 10KB per template)
- Server SHOULD limit maximum rendered prompt size (recommend 100KB)

---

## Reference

- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **Template Syntax**: Mustache-like `{{parameter}}` placeholders (simplified)
