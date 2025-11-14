# API Contract: Ollama ChatModel Adapter

**Date**: 2025-11-14
**Branch**: `009-ollama-provider`

## Purpose

Define the public API contract for the Ollama ChatModel adapter, including exported types, methods, and behavior guarantees.

## Package

```go
package ollama // import "github.com/dshills/langgraph-go/graph/model/ollama"
```

## Exported Types

### 1. ChatModel

**Type**: `struct`

**Description**: Implements the `model.ChatModel` interface to integrate Ollama with LangGraph workflows.

**Constructor**:
```go
func NewChatModel(config Config) (*ChatModel, error)
```

**Parameters**:
- `config` (Config): Configuration for Ollama connection and model parameters

**Returns**:
- `*ChatModel`: Initialized adapter instance
- `error`: Configuration validation errors or initialization failures

**Errors**:
- `ErrInvalidConfig`: When required configuration is missing or invalid
  - Empty `Model` field
  - Invalid `Endpoint` URL format
  - `Temperature` out of range [0.0, 2.0]
  - `TopP` out of range [0.0, 1.0]
  - `NumPredict` < -1

**Example**:
```go
config := ollama.Config{
    Endpoint:    "http://localhost:11434",
    Model:       "llama3.2",
    Temperature: 0.7,
}
adapter, err := ollama.NewChatModel(config)
if err != nil {
    log.Fatalf("Failed to create adapter: %v", err)
}
```

---

### 2. Chat Method

**Signature**:
```go
func (m *ChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error)
```

**Description**: Sends messages to Ollama and returns the model's response. Implements the `model.ChatModel` interface.

**Parameters**:
- `ctx` (context.Context): Context for cancellation and timeout control
- `messages` ([]model.Message): Conversation history (system, user, assistant messages)
- `tools` ([]model.ToolSpec): Optional tool specifications the model can use (nil if no tools)

**Returns**:
- `model.ChatOut`: LLM response containing text and/or tool calls
- `error`: Connection errors, model errors, or context cancellation

**Behavior**:
1. Validates inputs (non-empty messages, valid roles)
2. Translates LangGraph types to Ollama API types
3. Sends chat request to Ollama via HTTP
4. Waits for complete response (non-streaming)
5. Translates Ollama response to LangGraph types
6. Returns `ChatOut` with text, tool calls, and metadata

**Context Handling**:
- Respects `ctx.Done()` for cancellation
- Returns `context.Canceled` if context is canceled during request
- Returns `context.DeadlineExceeded` if context timeout is reached
- Cancellation is checked before sending request and during response wait

**Error Cases**:
- `OllamaError{Code: "connection"}`: Failed to connect to Ollama endpoint
- `OllamaError{Code: "model_not_found"}`: Requested model not available
- `OllamaError{Code: "invalid_request"}`: Malformed request or unsupported operation
- `context.DeadlineExceeded`: Request exceeded context deadline
- `context.Canceled`: Context was canceled during request

**Thread Safety**: Safe for concurrent use across multiple goroutines.

**Example**:
```go
messages := []model.Message{
    {Role: model.RoleSystem, Content: "You are a helpful assistant."},
    {Role: model.RoleUser, Content: "What is the capital of France?"},
}

out, err := adapter.Chat(ctx, messages, nil)
if err != nil {
    var ollamaErr *ollama.OllamaError
    if errors.As(err, &ollamaErr) {
        log.Printf("Ollama error [%s]: %s", ollamaErr.Code, ollamaErr.Message)
    }
    return err
}

fmt.Println(out.Text) // "The capital of France is Paris."
```

---

### 3. Config

**Type**: `struct`

**Description**: Configuration for connecting to and interacting with Ollama instances.

**Fields**:
```go
type Config struct {
    // Endpoint is the Ollama server URL
    // Default: "http://localhost:11434"
    Endpoint string

    // Model is the model name to use (required)
    // Examples: "llama3.2", "mistral", "codellama"
    Model string

    // Temperature controls randomness in generation (0.0-2.0)
    // Lower values = more deterministic, higher values = more creative
    // Default: 0.8
    Temperature float64

    // TopP controls nucleus sampling (0.0-1.0)
    // Lower values = more focused, higher values = more diverse
    // Default: 0.9
    TopP float64

    // Seed for deterministic generation (optional)
    // Set to non-nil value for reproducible outputs
    // Default: nil (non-deterministic)
    Seed *int

    // NumPredict is the maximum number of tokens to generate
    // -1 = unlimited (model default)
    // Default: -1
    NumPredict int

    // HTTPClient for custom HTTP transport configuration
    // Use to set custom timeouts, proxies, or TLS settings
    // Default: http.DefaultClient with 60s timeout
    HTTPClient *http.Client
}
```

**Defaults**:
```go
// Applied by NewChatModel if not specified:
Endpoint:    "http://localhost:11434"
Temperature: 0.8
TopP:        0.9
Seed:        nil
NumPredict:  -1
HTTPClient:  &http.Client{Timeout: 60 * time.Second}
```

**Validation**:
- `Model`: Must be non-empty string
- `Endpoint`: Must be valid URL or empty (defaults to localhost)
- `Temperature`: Must be in [0.0, 2.0]
- `TopP`: Must be in [0.0, 1.0]
- `NumPredict`: Must be >= -1

**Example**:
```go
// Local Ollama with defaults
config := ollama.Config{
    Model: "llama3.2",
}

// Remote Ollama with custom parameters
config := ollama.Config{
    Endpoint:    "http://ollama-server:11434",
    Model:       "mistral",
    Temperature: 0.5,
    Seed:        intPtr(42), // Deterministic
}

// Custom HTTP client with shorter timeout
config := ollama.Config{
    Model: "codellama",
    HTTPClient: &http.Client{
        Timeout: 30 * time.Second,
    },
}
```

---

### 4. OllamaError

**Type**: `struct`

**Description**: Structured error type with actionable error messages.

**Fields**:
```go
type OllamaError struct {
    // Code identifies the error category
    Code string

    // Message is the user-friendly error message
    Message string

    // Err is the wrapped original error
    Err error
}
```

**Error Codes**:
- `"connection"`: Failed to connect to Ollama endpoint
- `"model_not_found"`: Requested model not available
- `"invalid_request"`: Malformed request or unsupported operation
- `"timeout"`: Request exceeded deadline
- `"unknown"`: Unexpected error

**Methods**:
```go
func (e *OllamaError) Error() string
func (e *OllamaError) Unwrap() error
```

**Usage with errors.As**:
```go
out, err := adapter.Chat(ctx, messages, nil)
if err != nil {
    var ollamaErr *ollama.OllamaError
    if errors.As(err, &ollamaErr) {
        switch ollamaErr.Code {
        case "connection":
            log.Println("Ollama is not running. Start it with: ollama serve")
        case "model_not_found":
            log.Printf("Model not available. Pull it with: ollama pull %s", config.Model)
        }
    }
}
```

**Example Error Messages**:
```
Connection error:
"Failed to connect to Ollama at http://localhost:11434. Ensure Ollama is running with: ollama serve"

Model not found:
"Model 'llama3.2' not available. Pull it with: ollama pull llama3.2"

Invalid request:
"Invalid request to Ollama: temperature must be between 0.0 and 2.0"
```

---

## Behavior Guarantees

### 1. Thread Safety
- `ChatModel` instances are safe for concurrent use
- Multiple goroutines can call `Chat()` simultaneously
- No shared mutable state across calls

### 2. Context Respect
- All operations respect `context.Context` cancellation
- Cancellation is checked before request and during response
- Returns `context.Canceled` or `context.DeadlineExceeded` appropriately

### 3. Idempotency
- Each `Chat()` call is independent
- No state carries over between calls
- Same inputs (with same seed) produce same outputs (deterministic)

### 4. Error Transparency
- All errors are returned explicitly (no panics)
- Errors wrap underlying causes with `fmt.Errorf("%w")`
- Actionable messages guide users to solutions

### 5. Metadata Availability
- Every successful response includes metadata in `ChatOut.Meta`
- Metadata fields: `model`, `created_at`, `total_duration_ms`, `prompt_tokens`, `completion_tokens`
- Metadata is optional but always present for Ollama responses

### 6. Tool Calling Parity
- Tool specifications are translated to Ollama format
- Tool calls are parsed from responses
- Models without tool support return clear error

---

## Integration Patterns

### 1. Basic Chat
```go
adapter, _ := ollama.NewChatModel(ollama.Config{Model: "llama3.2"})
messages := []model.Message{
    {Role: model.RoleUser, Content: "Hello!"},
}
out, _ := adapter.Chat(context.Background(), messages, nil)
fmt.Println(out.Text)
```

### 2. With Timeout
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

out, err := adapter.Chat(ctx, messages, nil)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Request timed out")
}
```

### 3. With Tool Calling
```go
tools := []model.ToolSpec{
    {
        Name:        "get_weather",
        Description: "Get current weather for a location",
        Schema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{"type": "string"},
            },
            "required": []string{"location"},
        },
    },
}

out, _ := adapter.Chat(ctx, messages, tools)
for _, call := range out.ToolCalls {
    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
}
```

### 4. Remote Ollama Instance
```go
config := ollama.Config{
    Endpoint: "http://ollama-server.example.com:11434",
    Model:    "mistral",
}
adapter, _ := ollama.NewChatModel(config)
```

### 5. Deterministic Generation
```go
seed := 42
config := ollama.Config{
    Model:       "llama3.2",
    Seed:        &seed,
    Temperature: 0.0, // Minimum randomness
}
adapter, _ := ollama.NewChatModel(config)
// Same inputs will produce identical outputs
```

---

## Compatibility

### LangGraph Framework
- Implements `model.ChatModel` interface (version 1.x)
- Compatible with all LangGraph graph execution features
- Works with Engine state management and checkpointing

### Ollama API
- Targets Ollama API version 0.1.0+ (current stable)
- Uses official SDK: `github.com/ollama/ollama/api`
- Supports all major Ollama models (llama, mistral, codellama, etc.)

### Go Version
- Requires Go 1.21+ (generics support)
- Uses standard library packages only (except Ollama SDK)

---

## Testing Contract

### Unit Tests
- All exported functions have unit tests
- Mock HTTP server for Ollama API responses
- Error scenarios comprehensively tested
- Table-driven tests for input validation

### Integration Tests
- Optional integration tests with real Ollama instance
- Marked with `//go:build integration`
- Require local Ollama installation and `llama3.2` model

### Example Tests
- Runnable examples for documentation
- Cover basic usage, remote config, tool calling

---

## Deprecation Policy

If breaking changes are needed:
1. Deprecated features marked with `// Deprecated:` comment
2. At least one minor version with deprecation warning
3. Major version bump for removal

Example:
```go
// Deprecated: Use NewChatModel with Config instead.
func NewChatModelSimple(model string) *ChatModel
```

---

## Performance Characteristics

- **Latency**: < 10ms adapter overhead (translation + HTTP setup)
- **Memory**: ~1 KB per ChatModel instance
- **Throughput**: Limited by Ollama server, not adapter
- **Concurrency**: Scales linearly with goroutines (no contention)

---

## Summary

The Ollama adapter provides a **simple, safe, and idiomatic** Go API:
- 3 exported types: `ChatModel`, `Config`, `OllamaError`
- 1 primary method: `Chat(ctx, messages, tools)`
- Thread-safe, context-aware, error-transparent
- Full parity with other LangGraph model adapters
