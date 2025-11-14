# Data Model: Ollama Model Provider

**Date**: 2025-11-14
**Branch**: `009-ollama-provider`

## Purpose

Define the data structures and their relationships for the Ollama ChatModel adapter. This adapter is **stateless** - it does not manage execution state, which is handled by the LangGraph Engine.

## Core Types

### 1. ChatModel (Adapter)

**Purpose**: Implements the `model.ChatModel` interface to integrate Ollama with LangGraph workflows.

**Attributes**:
- `client`: Ollama API client (`*api.Client`)
- `config`: Immutable configuration (`Config`)

**Responsibilities**:
- Translate LangGraph types to Ollama API types
- Execute chat requests via Ollama API
- Parse responses and translate back to LangGraph types
- Handle errors with actionable messages

**Lifecycle**:
- Created via `NewChatModel(config Config)` constructor
- Immutable after creation (thread-safe)
- Can be reused across multiple workflow executions

**Relationships**:
- Implements `model.ChatModel` interface
- Contains `Config` (composition)
- Uses `api.Client` for Ollama communication

### 2. Config (Configuration)

**Purpose**: Holds configuration for connecting to and interacting with Ollama instances.

**Attributes**:
```go
type Config struct {
    // Endpoint is the Ollama server URL
    // Default: "http://localhost:11434"
    Endpoint string

    // Model is the model name to use (required)
    // Examples: "llama3.2", "mistral", "codellama"
    Model string

    // Temperature controls randomness (0.0-2.0)
    // Default: 0.8
    Temperature float64

    // TopP controls nucleus sampling (0.0-1.0)
    // Default: 0.9
    TopP float64

    // Seed for deterministic generation (optional)
    // nil = non-deterministic
    Seed *int

    // NumPredict is max tokens to generate
    // Default: -1 (unlimited)
    NumPredict int

    // HTTPClient for custom timeouts/transport
    // Default: http.DefaultClient with 60s timeout
    HTTPClient *http.Client
}
```

**Validation Rules**:
- `Model`: Must be non-empty string
- `Endpoint`: Defaults to `http://localhost:11434` if empty, must be valid URL
- `Temperature`: Must be in range [0.0, 2.0]
- `TopP`: Must be in range [0.0, 1.0]
- `NumPredict`: Must be >= -1
- `HTTPClient`: Defaults to `http.DefaultClient` if nil

**Relationships**:
- Owned by `ChatModel`
- Validated in `NewChatModel()` constructor

### 3. OllamaError (Error Type)

**Purpose**: Structured error type with actionable error messages.

**Attributes**:
```go
type OllamaError struct {
    // Code identifies the error category
    Code string // "connection", "model_not_found", "invalid_request", "timeout"

    // Message is the user-friendly error message
    Message string

    // Err is the wrapped original error
    Err error
}
```

**Error Codes**:
- `connection`: Failed to connect to Ollama endpoint
- `model_not_found`: Requested model not available
- `invalid_request`: Malformed request or unsupported operation
- `timeout`: Request exceeded deadline
- `unknown`: Unexpected error

**Methods**:
- `Error() string`: Returns formatted error message
- `Unwrap() error`: Returns wrapped error for `errors.Is`/`errors.As`

**Relationships**:
- Wraps underlying errors from Ollama API
- Returned by `ChatModel.Chat()` on failures

## Translation Types (Internal)

These types are used internally for translating between LangGraph and Ollama API formats. They are **not exported**.

### 4. messageTranslator (Internal)

**Purpose**: Translates between `model.Message` and `api.Message`.

**Translation Logic**:
```go
// model.Message → api.Message
func toOllamaMessage(msg model.Message) api.Message {
    return api.Message{
        Role:    msg.Role,    // Direct mapping
        Content: msg.Content, // Direct mapping
    }
}

// api.Message → string (extract content)
func fromOllamaMessage(msg api.Message) string {
    return msg.Content
}
```

**Validation**:
- Roles must be one of: "system", "user", "assistant"
- Content can be empty for system messages

### 5. toolTranslator (Internal)

**Purpose**: Translates between `model.ToolSpec` and `api.Tool`.

**Translation Logic**:
```go
// model.ToolSpec → api.Tool
func toOllamaTool(spec model.ToolSpec) api.Tool {
    return api.Tool{
        Type: "function",
        Function: api.ToolFunction{
            Name:        spec.Name,
            Description: spec.Description,
            Parameters:  spec.Schema, // Direct mapping (both use JSON Schema)
        },
    }
}

// api.ToolCall → model.ToolCall
func fromOllamaToolCall(call api.ToolCall) (model.ToolCall, error) {
    // Parse Arguments JSON string to map[string]interface{}
    var input map[string]interface{}
    err := json.Unmarshal([]byte(call.Function.Arguments), &input)
    if err != nil {
        return model.ToolCall{}, fmt.Errorf("invalid tool call arguments: %w", err)
    }

    return model.ToolCall{
        Name:  call.Function.Name,
        Input: input,
    }, nil
}
```

**Validation**:
- Tool names must be non-empty
- Schema must be valid JSON Schema (validated by Ollama)
- Tool call arguments must be valid JSON

### 6. responseTranslator (Internal)

**Purpose**: Translates `api.ChatResponse` to `model.ChatOut`.

**Translation Logic**:
```go
func toLangGraphOutput(resp api.ChatResponse) (model.ChatOut, error) {
    out := model.ChatOut{
        Text: resp.Message.Content,
        Meta: map[string]interface{}{
            "model":             resp.Model,
            "created_at":        resp.CreatedAt,
            "total_duration_ms": resp.TotalDuration / 1_000_000, // ns → ms
            "prompt_tokens":     resp.PromptEvalCount,
            "completion_tokens": resp.EvalCount,
        },
    }

    // Translate tool calls if present
    if len(resp.Message.ToolCalls) > 0 {
        toolCalls := make([]model.ToolCall, len(resp.Message.ToolCalls))
        for i, call := range resp.Message.ToolCalls {
            tc, err := fromOllamaToolCall(call)
            if err != nil {
                return model.ChatOut{}, fmt.Errorf("tool call %d: %w", i, err)
            }
            toolCalls[i] = tc
        }
        out.ToolCalls = toolCalls
    }

    return out, nil
}
```

## State Transitions

The adapter is **stateless** and has no internal state transitions. Each `Chat()` call is independent.

**Request Flow**:
1. User creates `ChatModel` with `Config` → adapter initialized
2. User calls `Chat(ctx, messages, tools)` → request initiated
3. Adapter translates inputs → Ollama API types
4. Adapter sends request via `api.Client` → HTTP call
5. Ollama processes request → generates response
6. Adapter receives `api.ChatResponse` → response arrives
7. Adapter translates response → LangGraph types
8. Adapter returns `ChatOut` or error → call complete

**Error Paths**:
- Connection failure → `OllamaError{Code: "connection"}`
- Model not found → `OllamaError{Code: "model_not_found"}`
- Invalid request → `OllamaError{Code: "invalid_request"}`
- Context timeout → `context.DeadlineExceeded` (pass-through)
- Context canceled → `context.Canceled` (pass-through)

## Validation Rules

### Configuration Validation (Constructor)
- `Model` must be non-empty
- `Endpoint` must be valid URL or empty (defaults to localhost)
- `Temperature` must be in [0.0, 2.0]
- `TopP` must be in [0.0, 1.0]
- `NumPredict` must be >= -1

### Input Validation (Chat Method)
- `messages` must not be empty
- Message roles must be valid ("system", "user", "assistant")
- Tool names must be non-empty
- Tool schemas must be valid JSON

### Response Validation
- Response must have non-nil `Message`
- Tool call arguments must be valid JSON
- Metadata fields must have expected types

## Relationships with External Types

### LangGraph Framework Types (graph/model)
- **model.ChatModel** (interface): Implemented by `ChatModel`
- **model.Message**: Translated to `api.Message`
- **model.ToolSpec**: Translated to `api.Tool`
- **model.ToolCall**: Parsed from `api.ToolCall`
- **model.ChatOut**: Constructed from `api.ChatResponse`

### Ollama API Types (github.com/ollama/ollama/api)
- **api.Client**: Used for HTTP communication
- **api.ChatRequest**: Constructed from LangGraph inputs
- **api.ChatResponse**: Parsed into LangGraph outputs
- **api.Message**: Translated from `model.Message`
- **api.Tool**: Translated from `model.ToolSpec`
- **api.ToolCall**: Parsed into `model.ToolCall`

### Go Standard Library Types
- **context.Context**: Passed through for cancellation/timeout
- **http.Client**: Used for HTTP transport configuration
- **error**: Used for error propagation

## Metadata Schema

The `ChatOut.Meta` field contains structured metadata from Ollama responses:

```go
Meta: map[string]interface{}{
    "model":             string,  // Model name used (e.g., "llama3.2")
    "created_at":        string,  // ISO 8601 timestamp
    "total_duration_ms": int64,   // Total generation time in milliseconds
    "prompt_tokens":     int,     // Number of input tokens processed
    "completion_tokens": int,     // Number of output tokens generated
}
```

**Usage**:
- Observability: Track model performance and token usage
- Debugging: Verify correct model selection
- Cost tracking: Monitor token consumption (even though Ollama is free)
- Performance tuning: Identify slow generations

## Thread Safety

- **ChatModel**: Thread-safe (immutable after construction, API client handles concurrency)
- **Config**: Immutable (thread-safe by design)
- **OllamaError**: Immutable (thread-safe)
- **api.Client**: Thread-safe (per Ollama SDK documentation)

Multiple goroutines can safely call `Chat()` on the same `ChatModel` instance.

## Memory Characteristics

- **ChatModel**: ~1 KB (client pointer + config struct)
- **Config**: ~200 bytes (strings, numbers, pointer)
- **ChatOut**: Variable size based on response (text + tool calls + metadata)
- **No state accumulation**: Each call is independent, no memory leaks

## Example Usage

```go
// Create adapter
config := ollama.Config{
    Endpoint:    "http://localhost:11434",
    Model:       "llama3.2",
    Temperature: 0.7,
}
adapter, err := ollama.NewChatModel(config)
if err != nil {
    log.Fatal(err)
}

// Use in LangGraph workflow
messages := []model.Message{
    {Role: model.RoleUser, Content: "What is the capital of France?"},
}

out, err := adapter.Chat(ctx, messages, nil)
if err != nil {
    log.Fatal(err)
}

fmt.Println(out.Text) // "The capital of France is Paris."
fmt.Printf("Model: %s\n", out.Meta["model"])
fmt.Printf("Tokens: %d\n", out.Meta["completion_tokens"])
```

## Summary

The Ollama adapter is a **stateless translation layer** between LangGraph and Ollama API:
- **3 exported types**: `ChatModel`, `Config`, `OllamaError`
- **3 internal translators**: message, tool, response
- **Immutable design**: Thread-safe, no shared state
- **Rich metadata**: Performance metrics, model info
- **Actionable errors**: User-friendly messages with solutions
