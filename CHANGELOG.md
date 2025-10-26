# Changelog

All notable changes to LangGraph-Go will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Core Framework (v0.1.0)

**Workflow Engine**
- Generic `Engine[S]` for type-safe workflow orchestration
- `Node[S]` interface for processing units
- `NodeResult[S]` for node execution results with delta, routing, events, and errors
- `Reducer[S]` functions for deterministic state merging
- Automatic step-by-step state persistence
- Configurable retry logic with `Options.Retries`
- Maximum step limit protection with `Options.MaxSteps`

**Routing & Control Flow**
- Explicit routing via `Goto(nodeID)` and `Stop()`
- Conditional edges with `Connect(from, to, predicate)`
- Fan-out to multiple parallel nodes with `Next{Many: []string{...}}`
- Edge-based routing with predicate functions
- Loop support via self-referencing edges
- Default edge support (nil predicate matches always)

**State Management**
- Type-safe state using Go generics
- Partial state updates via `NodeResult.Delta`
- Customizable reducer functions for merge strategies
- Support for nested struct states
- JSON serialization for persistence

**Persistence & Checkpoints**
- `Store[S]` interface for state persistence
- In-memory store implementation (`MemStore`)
- MySQL store implementation (`MySQLStore`) - Coming soon
- Automatic step-by-step checkpointing
- Named checkpoint save/restore
- Resume workflows from last successful step
- Query checkpoint history

**Parallel Execution**
- Concurrent node execution via goroutines
- Isolated state copies for each parallel branch
- Deterministic merge via reducer (lexicographic ordering by nodeID)
- Error handling: first-error-wins strategy
- Fan-out/fan-in patterns

**Event Tracing & Observability**
- `Emitter` interface for event emission
- Standard event types: `node_start`, `node_end`, `routing_decision`, `error`
- `LogEmitter` for stdout/stderr logging (text and JSON modes)
- `BufferedEmitter` for in-memory event capture and analysis
- `NullEmitter` for zero-overhead production deployments
- Event metadata with 50+ documented conventions
- Helper methods: `WithDuration()`, `WithError()`, `WithNodeType()`, `WithMeta()`
- History filtering by nodeID, message type, and step range
- Multi-emitter pattern for simultaneous logging and analysis

**LLM Integration**
- `ChatModel` interface for unified LLM provider API
- OpenAI adapter (GPT-4, GPT-3.5, GPT-4o)
- Anthropic adapter (Claude Sonnet 4.5, Claude 3 Opus/Sonnet/Haiku)
- Google Gemini adapter (Gemini 2.5 Flash, Gemini Pro Vision)
- Tool calling support with JSON Schema
- Standard message format (system, user, assistant roles)
- Provider-specific error handling (e.g., Google safety filters)
- Automatic retry for transient errors (OpenAI)

**Error Handling**
- Node-level errors via `NodeResult.Err`
- Configurable retry attempts
- Error routing to handler nodes
- Graceful error capture in parallel branches
- Context cancellation and timeout support

#### Documentation

**User Guides** (8 comprehensive guides)
- Getting Started (454 lines) - Installation, first workflow, core concepts
- Building Workflows (559 lines) - 4 architecture patterns, error handling, testing
- State Management (559 lines) - 7 reducer patterns, design best practices
- Checkpoints & Resume (559 lines) - Persistence, resumption strategies
- Conditional Routing (559 lines) - Edge predicates, 6 routing patterns
- Parallel Execution (559 lines) - Fan-out patterns, state isolation
- LLM Integration (590 lines) - Multi-provider patterns, tool calling
- Event Tracing (560 lines) - Observability, monitoring patterns

**Reference Documentation**
- Complete API Reference with examples
- FAQ with 40+ common questions
- GoDoc comments on all exported types and functions

#### Examples

- `simple/` - Basic 3-node workflow
- `checkpoint/` - Checkpoint save and resume
- `routing/` - Conditional routing with edges
- `parallel/` - Parallel execution with fan-out
- `llm/` - Multi-provider LLM integration
- `tracing/` - Event tracing and analysis

#### Testing

- 100+ unit tests covering core functionality
- Integration tests for all 5 user stories
- Comprehensive test coverage for:
  - State management and reducers
  - Routing (explicit, conditional, fan-out)
  - Parallel execution and deterministic merge
  - Error handling and retry logic
  - Event emission and filtering
  - Checkpoint save/restore
  - LLM provider adapters

### Changed

- N/A (initial release)

### Deprecated

- N/A (initial release)

### Removed

- N/A (initial release)

### Fixed

- N/A (initial release)

### Security

- N/A (initial release)

---

## Release Notes

### v0.1.0 - Core Framework Complete

**Release Date**: TBD

This is the initial release of LangGraph-Go with all core features implemented:

**Highlights**:
- ✅ Complete workflow engine with type-safe state management
- ✅ Conditional routing and parallel execution
- ✅ Multi-provider LLM integration (OpenAI, Anthropic, Google)
- ✅ Comprehensive event tracing and observability
- ✅ Production-ready error handling and retry logic
- ✅ 4,400+ lines of documentation
- ✅ 100+ unit and integration tests

**User Stories Completed**:
1. **US1**: Stateful workflow with checkpointing ✅
2. **US2**: Conditional routing and control flow ✅
3. **US3**: Parallel execution with fan-out ✅
4. **US4**: Multi-provider LLM integration ✅
5. **US5**: Event tracing and observability ✅

**Known Limitations**:
- MySQL store implementation pending (T183-T191)
- OpenTelemetry emitter pending (T192-T195)
- Tool system pending (T176-T182)
- Performance benchmarks pending (T196-T200)

**Breaking Changes**: N/A (initial release)

**Migration Guide**: N/A (initial release)

---

## Versioning Strategy

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

**Pre-1.0 Releases**: During initial development (0.x.x), minor version bumps may include breaking changes. Pin to specific versions in production.

---

## How to Upgrade

### General Upgrade Steps

1. Read the changelog for your target version
2. Check for breaking changes in the release notes
3. Update your `go.mod`:
   ```bash
   go get github.com/dshills/langgraph-go@vX.Y.Z
   ```
4. Run your tests to verify compatibility
5. Address any breaking changes (see migration guide if provided)

### Staying Updated

- **Latest stable**: `go get github.com/dshills/langgraph-go@latest`
- **Specific version**: `go get github.com/dshills/langgraph-go@v0.1.0`
- **Pre-release**: `go get github.com/dshills/langgraph-go@v0.2.0-beta.1`

---

## Support & Feedback

- **Bug Reports**: [GitHub Issues](https://github.com/dshills/langgraph-go/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/dshills/langgraph-go/discussions)
- **Documentation**: [docs/](./docs/)
- **Questions**: [FAQ](./docs/FAQ.md)

---

[Unreleased]: https://github.com/dshills/langgraph-go/compare/v0.1.0...HEAD
