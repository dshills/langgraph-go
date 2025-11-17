# Research: MCP Server Support

**Feature**: MCP Server Support
**Date**: 2025-11-17
**Status**: Complete

## Research Questions

1. **JSON-RPC 2.0 Library Selection**: Should we use github.com/sourcegraph/jsonrpc2, implement custom JSON-RPC 2.0, or use another library?
2. **MCP Protocol Requirements**: What are the current MCP specification requirements for JSON-RPC 2.0?
3. **Stdio Transport Best Practices**: How should we implement stdio transport for MCP protocol in Go?

## Findings

### 1. JSON-RPC 2.0 Library Selection

**Decision**: Use **github.com/sourcegraph/jsonrpc2**

**Rationale**:

1. **MCP Batching Requirement Removed**: MCP specification version 2025-06-18 removed batching support (previously added in March 2025, removed in June 2025 for simplification). The primary concern about sourcegraph/jsonrpc2 lacking batching is no longer relevant.

2. **Production Proven**: Used by Sourcegraph in LSP (Language Server Protocol) implementations, demonstrating production reliability.

3. **Clean Architecture**: Provides Stream abstraction that matches LangGraph's interface-first design philosophy.

4. **Stdio Support**: Well-documented patterns for stdin/stdout transport using BufferedStream and VSCodeObjectCodec.

5. **Time Savings**: ~100-150 LOC integration vs ~1,000+ LOC custom implementation.

6. **Active Maintenance**: Latest release v0.2.1 in May 2025, 229 GitHub stars, MIT license.

7. **Constitution Compliance**:
   - Single external dependency (minimal)
   - Isolated in `graph/mcp/transport/` subpackage
   - Swappable via transport abstraction layer

**Alternatives Considered**:

| Library | Pros | Cons | Verdict |
|---------|------|------|---------|
| go.lsp.dev/jsonrpc2 | LSP-focused, clean API | Stale (last updated March 2022) | ❌ Reject |
| viant/jsonrpc | Batching support | No community adoption (1 star, new) | ❌ Reject |
| trpc-group/trpc-mcp-go | MCP-native framework | Too heavyweight, vendor lock-in | ❌ Reject |
| golang.org/x/tools/internal/jsonrpc2 | Official, battle-tested | Internal package (no stability) | ❌ Reject |
| Custom implementation | Full control, no dependency | 3-5 days dev time, maintenance burden | ⚠️ Fallback |

**Implementation Estimate**:

- With library: ~100-150 LOC for transport layer
- Custom: ~800-1,000 LOC core + ~300-500 LOC tests = ~1,100-1,500 LOC
- Time savings: 3-5 development days

---

### 2. MCP Protocol Requirements

**Current Specification**: MCP 2025-06-18 (June 18, 2025)

**Key Requirements**:

| Requirement | Status | Details |
|------------|--------|---------|
| JSON-RPC 2.0 Base | ✅ Required | All messages follow JSON-RPC 2.0 format |
| Request/Response | ✅ Required | Standard pattern with ID correlation |
| Notifications | ✅ Required | Fire-and-forget messages without ID |
| **Batching** | ❌ **REMOVED** | Batching support removed in 2025-06-18 |
| Standard Error Codes | ✅ Required | -32700 (parse), -32600 (invalid), -32601 (not found), -32602 (params), -32603 (internal) |
| Stdio Transport | ✅ Required | Primary transport for local servers |

**Batching History**:

- 2024-11-05: No batching
- 2025-03-26: Batching added (PR #228)
- 2025-06-18: **Batching removed** (PR #416) for simplification

**Impact**: Removal of batching simplifies implementation significantly and removes the primary concern about sourcegraph/jsonrpc2.

**MCP Methods to Implement**:

| Method | Purpose | Priority |
|--------|---------|----------|
| `initialize` | Capability negotiation | P1 (required) |
| `tools/list` | List available tools | P1 (core feature) |
| `tools/call` | Invoke tool | P1 (core feature) |
| `resources/list` | List available resources | P2 |
| `resources/read` | Read resource content | P2 |
| `resources/subscribe` | Subscribe to resource updates | P3 (optional) |
| `prompts/list` | List prompt templates | P3 |
| `prompts/get` | Get prompt with arguments | P3 |

---

### 3. Stdio Transport Best Practices

**Recommended Pattern** (with sourcegraph/jsonrpc2):

```go
package transport

import (
    "context"
    "errors"
    "io"
    "os"

    "github.com/sourcegraph/jsonrpc2"
)

// StdioReadWriteCloser wraps stdin/stdout as io.ReadWriteCloser
type StdioReadWriteCloser struct {
    reader io.ReadCloser
    writer io.WriteCloser
}

func NewStdioReadWriteCloser() *StdioReadWriteCloser {
    return &StdioReadWriteCloser{
        reader: os.Stdin,
        writer: os.Stdout,
    }
}

func (rw *StdioReadWriteCloser) Read(b []byte) (int, error) {
    return rw.reader.Read(b)
}

func (rw *StdioReadWriteCloser) Write(b []byte) (int, error) {
    return rw.writer.Write(b)
}

func (rw *StdioReadWriteCloser) Close() error {
    return errors.Join(rw.reader.Close(), rw.writer.Close())
}

// MCPStdioServer wraps JSON-RPC connection for MCP protocol
type MCPStdioServer struct {
    conn *jsonrpc2.Conn
}

func NewMCPStdioServer(ctx context.Context, handler jsonrpc2.Handler) (*MCPStdioServer, error) {
    stdio := NewStdioReadWriteCloser()

    // Use VSCodeObjectCodec for Content-Length headers (LSP-compatible)
    stream := jsonrpc2.NewBufferedStream(stdio, jsonrpc2.VSCodeObjectCodec{})

    conn := jsonrpc2.NewConn(ctx, stream, handler)

    return &MCPStdioServer{conn: conn}, nil
}

func (s *MCPStdioServer) Start(ctx context.Context) error {
    // Connection handles messages automatically
    <-ctx.Done()
    return s.conn.Close()
}
```

**Key Design Decisions**:

1. **VSCodeObjectCodec**: Uses Content-Length header framing (LSP-compatible), more robust than newline-delimited JSON

2. **BufferedStream**: Provides buffering for efficient I/O

3. **Context-Based Lifecycle**: Server runs until context cancellation

4. **Read/Write Separation**: stdin for reading, stdout for writing (stderr available for logging)

**Testing Strategy**:

```go
// Use io.Pipe for testing without real stdio
type MockReadWriteCloser struct {
    reader *io.PipeReader
    writer *io.PipeWriter
}

func NewMockReadWriteCloser() (*MockReadWriteCloser, *MockReadWriteCloser) {
    r1, w1 := io.Pipe()
    r2, w2 := io.Pipe()

    client := &MockReadWriteCloser{reader: r1, writer: w2}
    server := &MockReadWriteCloser{reader: r2, writer: w1}

    return client, server
}
```

---

## Technology Stack Decisions

### Core Dependencies

| Dependency | Version | Purpose | Justification |
|------------|---------|---------|---------------|
| Go | 1.21+ | Language | Generics support required |
| github.com/sourcegraph/jsonrpc2 | v0.2.1 | JSON-RPC 2.0 | Production-proven, active maintenance |
| Standard library | - | Core functionality | encoding/json, io, context, sync |

### No Additional Dependencies

MCP server implementation uses ONLY:
- sourcegraph/jsonrpc2 (JSON-RPC protocol)
- Standard library (no LLM SDKs, no databases, no observability frameworks)

Existing LangGraph dependencies used:
- graph/tool (Tool interface)
- graph/store (Store interface for resource data)
- graph/emit (Emitter interface for observability)

---

## Best Practices Research

### 1. MCP Server Patterns

**Tool Registration**:
```go
// Simple registration API
server.RegisterTool("weather", weatherTool, ToolMetadata{
    Description: "Get current weather for a location",
    InputSchema: weatherSchema,
})
```

**Resource Providers**:
```go
// Static resources (fixed content)
server.RegisterStaticResource("workflow_state", stateProvider)

// Dynamic resources (computed on-demand)
server.RegisterDynamicResource("metrics", func(ctx context.Context) (interface{}, error) {
    return getCurrentMetrics(), nil
})
```

**Prompt Templates**:
```go
// Template with parameter substitution
server.RegisterPrompt("start_workflow", PromptTemplate{
    Description: "Start a workflow with parameters",
    Parameters: []PromptParameter{
        {Name: "workflow_id", Required: true},
        {Name: "input_data", Required: false},
    },
    Template: "Start workflow {{workflow_id}} with data: {{input_data}}",
})
```

### 2. Error Handling

MCP servers should use standard JSON-RPC error codes:

| Code | Meaning | When to Use |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON received |
| -32600 | Invalid request | JSON-RPC format invalid |
| -32601 | Method not found | Unknown MCP method |
| -32602 | Invalid params | Missing/invalid parameters |
| -32603 | Internal error | Tool execution failed |

### 3. Concurrency Patterns

**Connection Handling**:
- One goroutine per client connection
- Context cancellation propagates to all tool executions
- Shared tool/resource registries protected by sync.RWMutex

**Tool Execution**:
- Tool.Call() receives context from MCP request
- Timeout/cancellation handled by existing LangGraph context management
- No special concurrency needed (LangGraph tools already thread-safe)

---

## Risk Analysis

### Using sourcegraph/jsonrpc2

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| API changes (pre-1.0) | Low | Medium | Pin to v0.2.1, monitor releases |
| Batching needed later | Very Low | Low | MCP spec removed batching |
| Maintenance stops | Low | Low | Mature codebase, can fork |
| License issues | Very Low | Low | MIT is permissive |

**Overall Risk**: **LOW**

### Custom Implementation

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Protocol bugs | Medium | High | Extensive testing required |
| Maintenance burden | High | Medium | Ongoing edge case fixes |
| Security issues | Low | High | Input validation, review |
| Development delay | High | Medium | 3-5 day timeline impact |

**Overall Risk**: **MEDIUM-HIGH**

---

## Implementation Roadmap

### Phase 1: Proof of Concept (1-2 days)

1. Install sourcegraph/jsonrpc2: `go get github.com/sourcegraph/jsonrpc2@v0.2.1`
2. Implement minimal echo server with stdio transport
3. Test with MCP client (Claude Desktop or custom test client)
4. Verify JSON-RPC message format compliance

**Success Criteria**: Echo server responds to `initialize` and custom `echo` method

### Phase 2: Core MCP Server (3-4 days)

1. Implement MCPServer interface with lifecycle methods
2. Add tool registration and `tools/list` handler
3. Implement `tools/call` with LangGraph Tool integration
4. Add comprehensive tests with mock clients

**Success Criteria**: Tools can be registered and invoked from MCP clients

### Phase 3: Resources & Prompts (2-3 days)

1. Implement ResourceProvider interface
2. Add `resources/list` and `resources/read` handlers
3. Implement PromptRegistry with template rendering
4. Add `prompts/list` and `prompts/get` handlers

**Success Criteria**: Resources and prompts accessible from MCP clients

### Phase 4: Examples & Documentation (2 days)

1. Create example MCP server in `examples/mcp_server/`
2. Write quickstart guide with Claude Desktop setup
3. Document tool/resource/prompt registration patterns
4. Add integration tests

**Success Criteria**: Users can run example and connect from Claude Desktop within 10 minutes

**Total Timeline**: 8-11 days

---

## References

- MCP Specification: https://modelcontextprotocol.io/specification/2025-06-18
- sourcegraph/jsonrpc2: https://github.com/sourcegraph/jsonrpc2
- JSON-RPC 2.0 Specification: https://www.jsonrpc.org/specification
- LSP Specification (reference): https://microsoft.github.io/language-server-protocol/

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-11-17 | Use sourcegraph/jsonrpc2 | MCP batching removed, library is production-proven |
| 2025-11-17 | Stdio with VSCodeObjectCodec | LSP-compatible, robust framing |
| 2025-11-17 | Defer HTTP+SSE transport | Stdio sufficient for MVP, add later |
| 2025-11-17 | No additional LLM SDK dependencies | MCP exposes existing tools, no new capabilities |

---

## Open Questions Resolved

✅ **Q1**: Should MCP server run in-process or as separate service?
**A1**: In-process via `graph/mcp/` package. Simpler integration, leverages existing Engine context, matches Tool/Store pattern.

✅ **Q2**: How to handle tool name collisions?
**A2**: Namespace tools by server instance. Each workflow has its own MCPServer with isolated tool registry.

✅ **Q3**: JSON-RPC 2.0 library vs. custom?
**A3**: Use sourcegraph/jsonrpc2. Batching requirement removed from MCP spec, library saves 3-5 days dev time.

✅ **Q4**: Which transport codec?
**A4**: VSCodeObjectCodec (Content-Length framing). More robust than newline-delimited, LSP-compatible.

---

## Next Phase

**Proceed to Phase 1**: Design data model and API contracts

- Define MCP protocol contracts (request/response schemas)
- Design tool adapter interface
- Design resource provider interface
- Design prompt registry interface
- Create quickstart guide template
