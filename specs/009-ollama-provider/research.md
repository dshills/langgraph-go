# Research: Ollama Model Provider

**Date**: 2025-11-14
**Branch**: `009-ollama-provider`

## Purpose

Research technical decisions and patterns for implementing the Ollama ChatModel adapter for LangGraph-Go.

## 1. Ollama Go SDK Availability

### Decision: Use Official Ollama Go API Package

**Package**: `github.com/ollama/ollama/api`

**Rationale**:
- Official SDK maintained by Ollama team
- Published November 5, 2025 (actively maintained)
- MIT licensed
- Used internally by Ollama CLI (proven compatibility)
- Imported by 339+ packages (mature ecosystem)
- Provides fully typed, comprehensive client for all Ollama REST API features
- Supports chat, generation, model management, embeddings, streaming

**Alternatives Considered**:
1. **Standard library HTTP client (`net/http`)**:
   - Rejected: Would require manual JSON schema definitions, request/response parsing, error handling
   - Increases maintenance burden and bug surface area
   - Duplicates work already done by official SDK

2. **Third-party SDKs** (`github.com/rozoomcool/go-ollama-sdk`, `github.com/JexSrs/go-ollama`):
   - Rejected: Not officially maintained by Ollama
   - Smaller adoption, less proven in production
   - Risk of API drift from official Ollama changes

**Trade-offs**:
- Adds external dependency (violates pure Go core principle)
- **Justification**: Official SDK provides type safety, reduces implementation complexity, ensures API compatibility
- SDK is stable, well-maintained, and widely adopted
- Follows same pattern as OpenAI, Anthropic, Google adapters which use official SDKs

**Dependency Impact**:
- Adds `github.com/ollama/ollama/api` to `go.mod`
- SDK has minimal transitive dependencies (standard library focused)
- Aligns with constitution section V (Dependency Minimalism) exception for adapter packages

## 2. Ollama API Patterns

### Decision: Use Official API Client with ChatRequest/ChatResponse Types

**Key API Characteristics**:
- **Endpoint**: `/api/chat` (POST)
- **Message Format**: Array of `{role: string, content: string}` objects
- **Roles**: `system`, `user`, `assistant` (maps directly to LangGraph Message)
- **Streaming**: Enabled by default, can be disabled with `"stream": false`
- **Tool Calling**: Supported via `tools` parameter and `tool_calls` in response
- **Parameters**: `temperature`, `top_p`, `seed`, `num_predict` (via `options` object)

**Client Construction Pattern**:
```go
// Environment-based (OLLAMA_HOST)
client, err := api.ClientFromEnvironment()

// Explicit URL
client := api.NewClient(&url.URL{Scheme: "http", Host: "localhost:11434"}, http.DefaultClient)
```

**Chat Method Signature**:
```go
func (c *Client) Chat(ctx context.Context, req *ChatRequest, fn ChatResponseFunc) error
```

**Streaming Pattern**:
- Callback function invoked for each response chunk
- Non-streaming: callback receives single complete response with `done: true`
- Streaming: callback receives multiple partial responses, final response has `done: true`

**Rationale**:
- Official types provide compile-time safety
- Callback pattern enables flexible streaming/non-streaming handling
- Context support built-in (cancellation, timeouts)
- Error handling via `StatusError` type with HTTP status codes

## 3. ChatModel Interface Mapping

### Decision: Direct Mapping with Translation Layer

**Mapping Strategy**:

| LangGraph Type | Ollama Type | Translation |
|----------------|-------------|-------------|
| `[]model.Message` | `[]api.Message` | Direct field mapping (Role, Content) |
| `[]model.ToolSpec` | `[]api.Tool` | Schema conversion (Name, Description, Parameters) |
| `model.ChatOut` | `api.ChatResponse` | Extract Text from Message.Content, ToolCalls from Message.ToolCalls |
| `context.Context` | `context.Context` | Pass-through |

**Error Translation**:
- `api.StatusError` → custom error types with user-friendly messages
- Connection errors → "connection refused" with endpoint details
- Model not found → "model not available, pull with: ollama pull <model>"
- Timeout errors → preserve context.DeadlineExceeded

**Metadata Mapping** (`ChatOut.Meta`):
- `model`: Model name used
- `total_duration_ms`: Total generation time
- `prompt_eval_count`: Input tokens processed
- `eval_count`: Output tokens generated
- `created_at`: Response timestamp

**Rationale**:
- Preserves type safety end-to-end
- Clear separation of concerns (adapter translates, SDK handles protocol)
- Metadata enables observability and debugging
- Error translation provides actionable user guidance

## 4. Configuration Design

### Decision: Immutable Config Struct with Builder Pattern

**Config Structure**:
```go
type Config struct {
    Endpoint     string                 // Default: "http://localhost:11434"
    Model        string                 // Required: e.g., "llama3.2", "mistral"
    Temperature  float64                // Default: 0.8
    TopP         float64                // Default: 0.9
    Seed         *int                   // Optional: nil = non-deterministic
    NumPredict   int                    // Default: -1 (unlimited)
    HTTPClient   *http.Client           // Optional: custom client with timeouts
}
```

**Constructor Pattern**:
```go
func NewChatModel(config Config) (*ChatModel, error) {
    // Validate required fields
    // Apply defaults
    // Create Ollama API client
    // Return adapter
}
```

**Validation Rules**:
- `Model` must be non-empty
- `Endpoint` defaults to `http://localhost:11434` if empty
- `Temperature` must be in range [0.0, 2.0]
- `TopP` must be in range [0.0, 1.0]

**Rationale**:
- Explicit configuration reduces implicit behavior
- Immutable config prevents race conditions
- Builder pattern provides flexibility (optional parameters)
- Early validation fails fast with clear errors
- Follows pattern from Bedrock adapter (similar remote service)

**Alternatives Considered**:
- **Functional options pattern**: Rejected due to verbosity for simple config
- **Global defaults**: Rejected due to hidden state and testing complexity

## 5. Testing Strategy

### Decision: Multi-Layer Testing with Mock Server

**Test Layers**:

1. **Unit Tests** (no external dependencies):
   - Mock HTTP server using `httptest.NewServer`
   - Test message translation (Message → api.Message)
   - Test tool spec conversion (ToolSpec → api.Tool)
   - Test error translation (StatusError → custom errors)
   - Test configuration validation

2. **Integration Tests** (optional, require Ollama):
   - Marked with build tag `//go:build integration`
   - Test against real Ollama instance with `llama3.2`
   - Verify end-to-end functionality
   - Document setup requirements in test comments

3. **Example Tests**:
   - `ExampleChatModel_Chat` for documentation
   - Demonstrate local and remote configuration
   - Show tool calling usage

**Mock Server Pattern**:
```go
func TestChat(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock /api/chat response
    }))
    defer server.Close()

    // Create adapter with server.URL
    // Run test assertions
}
```

**Rationale**:
- Unit tests run fast without external dependencies (CI-friendly)
- Mock server provides full control over responses (error scenarios)
- Integration tests validate real-world behavior
- Example tests serve as documentation
- Follows TDD constitution requirement

## 6. Streaming Support

### Decision: Support Both Streaming and Non-Streaming via Internal Accumulation

**Implementation Strategy**:
- Always use `stream: false` in Ollama requests for simplicity
- Accumulate complete response before returning
- Future enhancement: optional streaming support with callback parameter

**Rationale**:
- ChatModel interface returns `ChatOut` (complete response, not streaming)
- Non-streaming simplifies implementation and error handling
- Matches behavior of OpenAI, Anthropic adapters
- Streaming support can be added later without breaking changes

**Future Streaming Design** (deferred):
```go
type StreamCallback func(chunk string) error

func (m *ChatModel) ChatStream(ctx context.Context, messages []Message, tools []ToolSpec, callback StreamCallback) (ChatOut, error)
```

## 7. Tool Calling Support

### Decision: Full Tool Calling Support with Schema Translation

**Implementation**:
- Convert `model.ToolSpec` → `api.Tool` with JSON schema in Parameters field
- Parse `api.Message.ToolCalls` → `[]model.ToolCall` with Name and Input extraction
- Return tool calls in `ChatOut.ToolCalls` array

**Schema Conversion**:
```go
// ToolSpec.Schema (map[string]interface{}) → api.Tool.Function.Parameters (map[string]any)
// Direct mapping, Ollama expects JSON Schema format
```

**Tool Call Parsing**:
```go
// api.ToolCall → model.ToolCall
// Extract: Name (string), Arguments (JSON → map[string]interface{})
```

**Rationale**:
- Tool calling is core feature for agentic workflows (P4 user story)
- Ollama supports tools in recent models (llama3.1+, mistral, etc.)
- Schema format is compatible (JSON Schema standard)
- Enables feature parity with cloud providers

**Graceful Degradation**:
- If model doesn't support tools, Ollama returns error
- Adapter translates to user-friendly error: "Model {name} does not support tool calling"

## 8. Error Handling Patterns

### Decision: Rich Error Types with Actionable Messages

**Error Categories**:

1. **Connection Errors**:
   - Original: `connection refused`
   - Translated: `"Failed to connect to Ollama at {endpoint}. Ensure Ollama is running with: ollama serve"`

2. **Model Not Found**:
   - Original: `model not found`
   - Translated: `"Model '{model}' not available. Pull it with: ollama pull {model}"`

3. **Invalid Request**:
   - Original: `invalid request`
   - Translated: `"Invalid request to Ollama: {details}"`

4. **Context Errors**:
   - Pass through: `context.DeadlineExceeded`, `context.Canceled`

**Error Type**:
```go
type OllamaError struct {
    Code    string // "connection", "model_not_found", "invalid_request"
    Message string
    Err     error  // Wrapped original error
}

func (e *OllamaError) Error() string { return e.Message }
func (e *OllamaError) Unwrap() error { return e.Err }
```

**Rationale**:
- Actionable messages guide users to solutions
- Structured errors enable programmatic handling
- Follows Go error wrapping conventions (`errors.Is`, `errors.As`)
- Improves developer experience (especially for newcomers)

## Summary of Decisions

| Area | Decision | Key Rationale |
|------|----------|---------------|
| SDK | Use official `github.com/ollama/ollama/api` | Type safety, compatibility, official support |
| API Pattern | ChatRequest/ChatResponse with callback | Proven pattern, streaming support built-in |
| Interface Mapping | Direct translation layer | Clean separation, type-safe |
| Configuration | Immutable Config struct | Explicit, testable, no hidden state |
| Testing | Mock HTTP server + optional integration | Fast CI, comprehensive coverage |
| Streaming | Non-streaming (defer streaming to v2) | Simplicity, matches interface design |
| Tool Calling | Full support with schema translation | Feature parity, agentic workflows |
| Error Handling | Rich errors with actionable messages | Developer experience, debuggability |

## Open Questions

None. All NEEDS CLARIFICATION items resolved.
