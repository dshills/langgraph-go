# Implementation Plan: Ollama Model Provider

**Branch**: `009-ollama-provider` | **Date**: 2025-11-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/009-ollama-provider/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Add Ollama as a model provider for LangGraph-Go, implementing the ChatModel interface to support both local and remote Ollama instances. This enables cost-free, offline LLM execution using locally-hosted models (llama3.2, mistral, codellama, etc.) while maintaining compatibility with the existing graph execution framework.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support for ChatModel interface compatibility)
**Primary Dependencies**: `github.com/ollama/ollama/api` (official Ollama Go SDK), standard library (`context`, `encoding/json`)
**Storage**: N/A (adapter is stateless, uses existing Engine state management)
**Testing**: Go test framework (`go test`), table-driven tests, mock HTTP server for unit tests, optional integration tests with real Ollama
**Target Platform**: Cross-platform (Linux, macOS, Windows) - any platform that can run Go and connect to Ollama via HTTP
**Project Type**: Single project (library adapter package)
**Performance Goals**: < 10% overhead compared to direct Ollama API calls, < 10ms adapter processing time per request
**Constraints**: Use official Ollama SDK (justified by type safety and compatibility), must respect context cancellation
**Scale/Scope**: Single adapter package (`graph/model/ollama/`), ~500-800 LOC including tests, support for 90% of Ollama API features

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism ✓ PASS
- Adapter implements `ChatModel` interface with strongly-typed `Message`, `ToolSpec`, and `ChatOut`
- No state mutations - adapter is stateless, delegates to Engine state management
- Error handling via explicit error returns (no panics)

### II. Interface-First Design ✓ PASS
- Implements existing `ChatModel` interface from `graph/model/chat.go`
- Follows adapter pattern used by OpenAI, Anthropic, Google, Bedrock providers
- Package structure: `graph/model/ollama/` (consistent with other adapters)

### III. Test-Driven Development ✓ COMMITMENT
- Will follow TDD: write tests first for Chat() method, configuration, error handling
- Unit tests with mock HTTP server for Ollama API responses
- Integration tests with actual Ollama instance (optional, documented)
- Example tests for documentation (`ExampleChatModel_Chat`)

### IV. Observability & Debugging ✓ PASS
- Adapter returns errors explicitly for debugging
- Uses standard `ChatOut.Meta` for response metadata (model, tokens, latency)
- No custom observability needed - delegates to Engine's existing event emission

### V. Dependency Minimalism ✓ PASS
- Core adapter uses only standard library (`net/http`, `encoding/json`, `context`)
- No external SDK dependencies unless official Ollama Go SDK exists and provides value
- Follows constitution requirement: core framework remains pure Go

### Go Idioms & Best Practices ✓ COMMITMENT
- Will use `gofmt` for formatting
- Context-aware: respects `context.Context` for cancellation and timeouts
- Explicit error returns with `fmt.Errorf` and `%w` for wrapping
- Table-driven tests for comprehensive coverage

### Development Workflow ✓ COMMITMENT
- Will use `mcp__gocontext__index_codebase` before starting implementation
- Will run `mcp-pr review_staged` before commits
- Will verify all tests pass with `go test ./...`

**Gate Result**: ✅ PASS - No constitution violations. Adapter follows established patterns.

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
graph/model/ollama/
├── ollama.go           # ChatModel implementation, NewChatModel constructor
├── ollama_test.go      # Unit tests with mock HTTP server
├── config.go           # Config struct for endpoint, model, parameters
├── config_test.go      # Config validation tests
├── schema.go           # Request/response JSON schema structs
├── schema_test.go      # Schema parsing tests
├── errors.go           # Error types and translation
├── errors_test.go      # Error handling tests
└── doc.go              # Package documentation

examples/ollama/
└── main.go             # Usage example with local/remote Ollama
```

**Structure Decision**: Single project (library adapter) following the established pattern from other model adapters (OpenAI, Anthropic, Google, Bedrock). All code lives in `graph/model/ollama/` package with comprehensive tests co-located. Example code demonstrates local and remote usage patterns.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| External dependency (`github.com/ollama/ollama/api`) | Official SDK provides type safety, API compatibility, and reduces implementation complexity | Standard library HTTP client would require manual JSON schema definitions, request/response parsing, error handling - duplicates work already done by official SDK and increases bug surface area |

**Justification**: Constitution section V (Dependency Minimalism) allows adapter packages to depend on external SDKs. The official Ollama SDK:
- Provides compile-time type safety
- Ensures API compatibility (maintained by Ollama team)
- Used internally by Ollama CLI (proven in production)
- Imported by 339+ packages (mature ecosystem)
- Minimal transitive dependencies

This follows the same pattern as OpenAI, Anthropic, and Google adapters which use official SDKs.

---

## Phase 0: Research Summary

**Status**: ✅ COMPLETE

All research completed and documented in [research.md](./research.md). Key decisions:

1. **SDK Selection**: Use official `github.com/ollama/ollama/api` package
2. **API Patterns**: ChatRequest/ChatResponse with callback-based streaming
3. **Interface Mapping**: Direct translation layer between LangGraph and Ollama types
4. **Configuration**: Immutable Config struct with builder pattern
5. **Testing**: Multi-layer testing with mock HTTP server
6. **Streaming**: Non-streaming mode initially (deferred to v2)
7. **Tool Calling**: Full support with JSON schema translation
8. **Error Handling**: Rich error types with actionable messages

All "NEEDS CLARIFICATION" items resolved. Ready for implementation.

---

## Phase 1: Design Summary

**Status**: ✅ COMPLETE

### Artifacts Generated

1. **[data-model.md](./data-model.md)**: Complete data model with:
   - Core types: `ChatModel`, `Config`, `OllamaError`
   - Internal translators: message, tool, response
   - Validation rules and relationships
   - Thread safety guarantees

2. **[contracts/api-contract.md](./contracts/api-contract.md)**: Public API contract with:
   - Exported types and methods
   - Behavior guarantees (thread safety, context respect, idempotency)
   - Integration patterns
   - Error handling contracts

3. **[quickstart.md](./quickstart.md)**: User guide with:
   - 5-minute quickstart
   - Common use cases (local, remote, tool calling, deterministic)
   - LangGraph integration example
   - Troubleshooting guide
   - Model recommendations

### Constitution Re-Check (Post-Design)

✅ **Type Safety & Determinism**: PASS
- Config validates all parameters at construction time
- Explicit error returns for all failure modes
- No state mutations (immutable design)

✅ **Interface-First Design**: PASS
- Implements `model.ChatModel` interface
- Translation layer cleanly separates concerns
- Testable with mock HTTP server

✅ **Test-Driven Development**: COMMITMENT CONFIRMED
- Test structure defined (unit, integration, example)
- Mock server pattern documented
- Table-driven test approach planned

✅ **Observability & Debugging**: PASS
- Rich error messages with error codes
- Metadata in all responses (model, tokens, latency)
- Actionable error guidance

✅ **Dependency Minimalism**: JUSTIFIED
- External dependency justified and documented in Complexity Tracking
- Follows established adapter pattern
- Minimal transitive dependencies

---

## Next Steps

The planning phase is complete. To proceed with implementation:

1. **Run `/speckit.tasks`** to generate `tasks.md` with actionable implementation tasks
2. **Run `/speckit.implement`** to execute the task-driven implementation workflow
3. **Follow TDD**: Write tests first, then implement until tests pass

### Implementation Order (Preview)

Based on the design, recommended implementation order:

1. **Foundation** (P0):
   - Config struct with validation
   - Error types (OllamaError)
   - Package documentation (doc.go)

2. **Core Adapter** (P1):
   - ChatModel struct and constructor
   - Message translation (LangGraph ↔ Ollama)
   - Chat() method implementation
   - Response parsing and metadata extraction

3. **Tool Support** (P2):
   - ToolSpec translation
   - ToolCall parsing
   - Schema conversion

4. **Testing** (P3):
   - Unit tests with mock HTTP server
   - Configuration validation tests
   - Error handling tests
   - Example tests for documentation

5. **Example** (P4):
   - `examples/ollama/main.go`
   - Demonstrate local and remote usage
   - Show tool calling

6. **Documentation** (P5):
   - Update CLAUDE.md with Ollama provider
   - Add to model provider capability matrix

---

## Summary

**Branch**: `009-ollama-provider`
**Status**: Planning Complete, Ready for Implementation

**Generated Artifacts**:
- ✅ plan.md (this file)
- ✅ research.md (technical decisions)
- ✅ data-model.md (types and relationships)
- ✅ contracts/api-contract.md (public API)
- ✅ quickstart.md (user guide)

**Next Command**: `/speckit.tasks` to generate actionable implementation tasks
