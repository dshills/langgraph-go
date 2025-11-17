# Implementation Plan: MCP Server Support

**Branch**: `010-mcp-server-support` | **Date**: 2025-11-17 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/010-mcp-server-support/spec.md`

## Summary

Add Model Context Protocol (MCP) server support to LangGraph-Go, enabling workflows to expose tools, resources, and prompts to external LLM applications (Claude Desktop, VS Code extensions, etc.) via JSON-RPC 2.0 over stdio transport. This feature creates a new `graph/mcp` package that wraps existing LangGraph tools, state, and workflow patterns in MCP protocol adapters, allowing seamless integration with the broader MCP ecosystem without modifying core framework interfaces.

**Technical Approach**: Implement MCP server as an interface-based adapter layer that translates between MCP JSON-RPC 2.0 messages and LangGraph's existing Tool/State/Engine interfaces. Use stdio as primary transport (HTTP+SSE deferred). Support three MCP primitives: Tools (expose graph/tool.Tool), Resources (expose workflow state/checkpoints), and Prompts (templated workflow patterns).

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support for type-safe state management)
**Primary Dependencies**:
- github.com/sourcegraph/jsonrpc2 v0.2.1 (JSON-RPC 2.0 protocol - decision rationale in research.md)
- Existing LangGraph core (`graph/engine.go`, `graph/tool/`, `graph/store/`)
- Standard library only for core MCP implementation (io, context, encoding/json, sync)

**Storage**: N/A for MCP server itself (leverages existing LangGraph Store interface for accessing workflow state/checkpoints)
**Testing**: Standard Go testing (go test), existing LangGraph test patterns (unit + integration tests with MemStore)
**Target Platform**: Cross-platform (Linux, macOS, Windows) - stdio transport is platform-agnostic
**Project Type**: Single project (new `graph/mcp/` package within existing monorepo structure)
**Performance Goals**:
- <2s end-to-end latency for tool invocations (excluding actual tool execution)
- <1s server startup with 50 registered tools
- <500ms resource read latency for 10MB datasets
- Support 100+ concurrent client connections

**Constraints**:
- Must maintain compatibility with existing LangGraph Tool interface (no breaking changes)
- MCP server must respect LangGraph context cancellation and timeout policies
- Protocol must conform to MCP spec version 2025-06-18 or later
- Zero external dependencies for core `graph/mcp` package (adapters may depend on jsonrpc2 lib)

**Scale/Scope**:
- Support 50+ registered tools per server without performance degradation
- Handle concurrent connections from multiple MCP clients (100+ simultaneous)
- Resource URIs: support hundreds of resources (state snapshots, checkpoints, metrics)
- Prompt templates: 10-20 common workflow patterns initially

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism ✅ PASS

**Requirement**: State management uses Go generics, deterministic replay from checkpoints, pure reducer functions.

**Compliance**:
- MCP server will NOT introduce new state management - it exposes existing LangGraph state via Resource interface
- Tool invocations execute through existing Engine context, maintaining deterministic execution
- No new state mutations - server acts as read/write adapter to existing Store
- All MCP protocol messages serializable to JSON (JSON-RPC 2.0 requirement)

**Justification**: Feature is purely an interface adapter; determinism guaranteed by existing Engine implementation.

---

### II. Interface-First Design ✅ PASS

**Requirement**: Core abstractions defined as interfaces, external dependencies accessed through adapters.

**Compliance**:
- New interfaces to be defined:
  - `MCPServer` interface for lifecycle (Start, Stop, RegisterTool, RegisterResource, RegisterPrompt)
  - `ToolAdapter` interface for wrapping graph/tool.Tool with MCP metadata
  - `ResourceProvider` interface for supplying resource data on-demand
  - `PromptRegistry` interface for managing prompt templates
- JSON-RPC 2.0 library dependency isolated behind transport abstraction
- MCP server implementation in `graph/mcp/server.go` with interface-first design
- Test implementations: MockMCPClient for testing server without real MCP clients

**Justification**: Follows existing LangGraph pattern (cf. Store, Emitter, ChatModel interfaces).

---

### III. Test-Driven Development (NON-NEGOTIABLE) ✅ PASS

**Requirement**: TDD workflow, tests before implementation, no commits without passing tests.

**Compliance**:
- Phase 0 will include test plan defining test cases for:
  - Tool registration and discovery
  - Tool invocation (success and error cases)
  - Resource read operations (static and dynamic)
  - Prompt template rendering
  - Protocol compliance (JSON-RPC 2.0 message formats)
  - Concurrent client connections
  - Context cancellation and timeout handling
- Tests will be written BEFORE implementation in Red-Green-Refactor cycle
- Integration tests will use MemStore and mock MCP clients
- Example-based tests for documentation (quickstart.md examples)

**Justification**: TDD enforced per constitution; plan includes explicit test specifications before code.

---

### IV. Observability & Debugging ✅ PASS

**Requirement**: All execution steps emit events through Emitter, errors captured and persisted.

**Compliance**:
- MCP server will emit events for:
  - Server lifecycle (start, stop, client connect/disconnect)
  - Tool invocations (request received, execution start/end, result/error)
  - Resource reads (URI requested, data retrieved)
  - Prompt rendering (template requested, arguments applied)
  - Protocol errors (invalid messages, schema validation failures)
- Events will use existing `graph.Emitter` interface (no new observability system)
- All MCP protocol interactions logged at DEBUG level for troubleshooting
- Tool execution errors propagated to MCP client as JSON-RPC error responses

**Justification**: Leverages existing observability infrastructure; no new patterns required.

---

### V. Dependency Minimalism ✅ PASS

**Requirement**: Core framework remains pure Go, external dependencies justified and isolated.

**Compliance**:
- Core `graph/mcp/` package will use ONLY standard library:
  - `io` for stdio transport
  - `encoding/json` for JSON-RPC message parsing
  - `context` for cancellation
  - `sync` for concurrency primitives
- Optional JSON-RPC 2.0 helper library (if chosen) isolated in `graph/mcp/transport/` subpackage
- No new LLM SDK dependencies (MCP exposes existing tools, doesn't add new ones)
- Dependency justification: JSON-RPC 2.0 library reduces boilerplate vs. custom implementation

**Rationale for JSON-RPC 2.0 library**:
- Protocol has subtle requirements (batching, notification vs. request/response, error codes)
- Well-tested libraries available (sourcegraph/jsonrpc2 has 1k+ stars, used in production)
- Custom implementation would be ~500 LOC vs. ~50 LOC with library
- Library can be swapped if needed (transport abstraction)

**Decision**: Use github.com/sourcegraph/jsonrpc2 v0.2.1 (see research.md for detailed evaluation).

---

### Go Idioms & Best Practices ✅ PASS

**Compliance**:
- Use `gofmt` for formatting (existing CI enforcement)
- Follow effective Go naming (e.g., `MCPServer` not `MCP_Server`)
- Context.Context for cancellation in all MCP operations
- Explicit error returns (no panics in library code)
- Generics for state type `[S any]` consistency with existing Engine
- Goroutines + channels for handling concurrent client connections

**Justification**: Standard Go practices, aligned with existing codebase patterns.

---

### Development Workflow ✅ PASS

**gocontext Indexing**:
- Codebase indexed before feature work begins
- Re-index after adding `graph/mcp/` package
- Use semantic search for understanding existing Tool/Store/Emitter patterns

**Code Review**:
- All code reviewed with `mcp-pr` before commits
- PRs will reference TDD tests
- No breaking changes to existing interfaces

**Testing Gates**:
- `go test ./...` must pass
- Integration tests for MCP server with real tool invocations
- Example tests runnable as documentation

**Justification**: Constitution compliance enforced throughout development.

---

### Summary

**Gate Status**: ✅ ALL CHECKS PASS

No constitution violations. Feature is:
- Interface-based adapter (no state management changes)
- Uses existing observability (Emitter)
- TDD-first approach planned
- Minimal dependencies (standard library + optional jsonrpc2 helper)
- Follows Go idioms and existing patterns

**Proceed to Phase 0 Research**: Approved

## Project Structure

### Documentation (this feature)

```text
specs/010-mcp-server-support/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   ├── mcp-protocol.md  # MCP JSON-RPC 2.0 message schemas
│   ├── tool-adapter.md  # Tool registration and invocation contracts
│   ├── resource-provider.md  # Resource read operation contracts
│   └── prompt-registry.md    # Prompt template contracts
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
graph/
├── mcp/                          # NEW: MCP server implementation
│   ├── server.go                 # MCPServer interface + default implementation
│   ├── server_test.go            # Server lifecycle and integration tests
│   ├── tool_adapter.go           # Wraps graph/tool.Tool with MCP metadata
│   ├── tool_adapter_test.go      # Tool registration and invocation tests
│   ├── resource_provider.go     # Resource interface for state/checkpoint exposure
│   ├── resource_provider_test.go # Resource read tests (static + dynamic)
│   ├── prompt_registry.go        # Prompt template management
│   ├── prompt_registry_test.go   # Prompt rendering tests
│   ├── protocol.go               # MCP message types (JSON-RPC 2.0 structs)
│   ├── protocol_test.go          # Protocol compliance tests
│   ├── transport/                # Transport layer abstractions
│   │   ├── stdio.go              # Stdio transport implementation
│   │   ├── stdio_test.go         # Stdio transport tests
│   │   └── jsonrpc.go            # Optional: JSON-RPC 2.0 helper (if library used)
│   └── examples/                 # Example MCP servers for testing
│       ├── weather_server.go     # Simple tool exposure example
│       └── stateful_server.go    # Resource + prompt example
│
├── tool/                         # EXISTING: No changes to tool interface
│   ├── tool.go                   # Tool interface (unchanged)
│   ├── http.go                   # HTTPTool (can be exposed via MCP)
│   └── ...
│
├── store/                        # EXISTING: Used for resource data access
│   ├── store.go                  # Store interface (unchanged)
│   └── ...
│
└── engine.go                     # EXISTING: Tool execution context (unchanged)

examples/
├── mcp_server/                   # NEW: Working example of MCP server
│   ├── main.go                   # Full example with tools + resources + prompts
│   ├── README.md                 # How to run with Claude Desktop
│   └── config.yaml               # Example MCP server configuration

tests/
├── integration/
│   └── mcp_integration_test.go   # End-to-end MCP server tests with mock clients
```

**Structure Decision**: Single project structure. MCP server implemented as new `graph/mcp/` package following existing patterns (cf. `graph/tool/`, `graph/store/`, `graph/emit/`). Package is self-contained with transport abstraction, allowing future HTTP+SSE support without core changes.

**Rationale**:
- Follows existing monorepo structure
- MCP is an optional feature (users can ignore `graph/mcp/` if not needed)
- Clear separation: protocol logic in `mcp/`, transport in `mcp/transport/`
- Examples directory provides working code for documentation

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations detected. This section is empty (as required).
