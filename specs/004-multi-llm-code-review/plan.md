# Implementation Plan: Multi-LLM Code Review Workflow

**Branch**: `004-multi-llm-code-review` | **Date**: 2025-10-29 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/004-multi-llm-code-review/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Build a graph-based workflow that reviews codebases using multiple AI language model providers (OpenAI, Anthropic, Google) in parallel. The workflow processes files in configurable batches to handle codebases of any size, persists checkpoints for resumability, and consolidates feedback from all providers into a single prioritized markdown report with deduplication and severity ranking. The implementation will serve as a command-line example application in the `examples/` directory demonstrating LangGraph-Go's capabilities for orchestrating complex multi-step workflows with state management, error handling, and concurrent execution.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support)
**Primary Dependencies**:
- `github.com/openai/openai-go` - OpenAI GPT API client
- `github.com/anthropics/anthropic-sdk-go` - Anthropic Claude API client
- `github.com/google/generative-ai-go` - Google Gemini API client
- LangGraph-Go core framework (`/graph` package)
- Standard library: `os`, `path/filepath`, `encoding/json`, `strings`, `regexp`

**Storage**:
- LangGraph-Go Store interface for checkpoint persistence (in-memory for MVP)
- File system for reading codebase and writing reports
- JSON files for configuration and report output

**Testing**:
- Go standard testing (`go test`)
- Table-driven tests for node logic
- Mock LLM providers for integration tests
- Example codebase fixtures for end-to-end tests

**Target Platform**:
- Command-line application (macOS, Linux, Windows)
- Runs in `examples/multi-llm-review/` directory

**Project Type**: Single executable example application

**Performance Goals**:
- Process 150 files/minute across all providers
- Handle codebases up to 10,000 files
- Consolidate reports within 5 minutes
- Resume from checkpoint within 10 seconds

**Constraints**:
- Must respect AI provider API rate limits with exponential backoff
- Memory usage must scale with batch size, not total codebase size
- Checkpoint data must be serializable to JSON
- No external process dependencies (pure Go)

**Scale/Scope**:
- Support 3 AI providers concurrently (OpenAI, Anthropic, Google)
- Batch size: 20 files (configurable)
- Review focus areas: security, performance, style, best practices
- Output: Single markdown report with ~100-1000 issues for typical codebases

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism ✅

**Status**: COMPLIANT

- Workflow state will use Go generics (`State[S]` where S contains batch tracking, file lists, reviews)
- All state mutations through reducer functions (merge batch results, accumulate reviews)
- Checkpoint persistence after each batch enables deterministic replay
- Node results return explicit deltas (batch completion, review results, consolidated issues)

**Implementation Notes**:
- State struct will include: `CurrentBatch`, `TotalBatches`, `CompletedBatches`, `Reviews map[string][]ReviewIssue`, `ConsolidatedReport`
- Reducer merges reviews from multiple providers, updates batch counters
- All state serializable to JSON for checkpoint persistence

### II. Interface-First Design ✅

**Status**: COMPLIANT

- Uses existing LangGraph-Go interfaces: `Node[S]`, `Store[S]`, `ChatModel`
- Will implement custom `CodeReviewer` interface wrapping `ChatModel` for review-specific logic
- Uses existing `graph.NewMemStore[S]()` for checkpoint storage
- AI provider adapters already exist: `model/openai.go`, `model/ollama.go` (can adapt for Anthropic/Google)

**Implementation Notes**:
- No new core interfaces needed
- Example application uses existing framework abstractions
- Mock `ChatModel` implementations for testing

### III. Test-Driven Development (NON-NEGOTIABLE) ✅

**Status**: COMPLIANT - TDD workflow will be followed

**Test Strategy**:
1. **Unit Tests** (write first):
   - File discovery and filtering logic
   - Batch creation from file lists
   - Review deduplication algorithm
   - Issue severity ranking and sorting
   - Reducer function for state merging

2. **Integration Tests** (write first):
   - Mock LLM provider responses
   - Checkpoint save/restore across batches
   - Error handling and retry logic
   - Concurrent provider execution

3. **End-to-End Tests** (write first):
   - Full workflow with fixture codebase
   - Multi-provider review aggregation
   - Report generation and formatting

**Commitment**: All tests written and approved before implementation. Red-Green-Refactor cycle enforced.

### IV. Observability & Debugging ✅

**Status**: COMPLIANT

- All nodes will emit events via `Emitter` interface:
  - File discovery start/complete
  - Batch processing start/complete
  - Per-provider review start/complete/error
  - Consolidation start/complete
  - Report generation complete
- Errors captured in `State.LastError` and persisted
- Progress updates emitted every 30 seconds during batch processing
- All events include runID, step number, nodeID, metadata (batch number, file counts, provider status)

**Implementation Notes**:
- Use existing `emit/log.go` stdout emitter for MVP
- Events structured for easy parsing (JSON format)
- Support future OpenTelemetry integration

### V. Dependency Minimalism ⚠️ JUSTIFIED DEVIATION

**Status**: REQUIRES JUSTIFICATION

**Violation**: Adding 3 external LLM SDK dependencies (`openai-go`, `anthropic-sdk-go`, `generative-ai-go`)

**Justification**:
- This is an **example application**, not core framework code
- Core framework (`/graph`) remains pure Go - dependencies are in `examples/multi-llm-review/` only
- Using official SDKs prevents implementing custom HTTP clients for 3 different APIs
- Demonstrates framework's ability to integrate with external services
- Dependencies are isolated to this example, not required for framework users

**Alternative Considered**: Implement custom HTTP clients for each provider
**Why Rejected**: Would add 500+ lines of API client code, error-prone, requires maintaining 3 API integrations

**Mitigation**:
- Dependencies declared in `examples/multi-llm-review/go.mod`, not root `go.mod`
- Example remains optional - framework can be used without this example
- Dependencies audited for security (official SDKs from OpenAI, Anthropic, Google)

### Pre-Design Gate Evaluation

**GATE STATUS**: ✅ PASS WITH JUSTIFIED DEVIATION

- 4/5 principles fully compliant
- 1 justified deviation (Dependency Minimalism) - acceptable for example application
- TDD commitment confirmed
- All state management, interfaces, and observability requirements met

**Next**: Proceed to Phase 0 (Research)

---

## Post-Design Constitution Re-Evaluation

*Re-evaluated after Phase 1 design completion (data-model.md, contracts/, quickstart.md)*

### I. Type Safety & Determinism ✅

**Status**: COMPLIANT (Confirmed)

**Design Validation**:
- `ReviewState` struct defined with all fields JSON-serializable (data-model.md)
- Reducer function `ReduceWorkflowState` implemented as pure function
- All state transitions documented with clear before/after states
- `NodeResult` pattern used for all node implementations
- Checkpoint persistence via `Store.SaveStep()` after each batch

**No Changes Needed**

### II. Interface-First Design ✅

**Status**: COMPLIANT (Confirmed)

**Design Validation**:
- `CodeReviewer` interface formally defined in `contracts/code-reviewer.go`
- All provider adapters (OpenAI, Anthropic, Google) implement this interface
- Mock provider for testing defined
- Uses existing LangGraph-Go interfaces (`Node[S]`, `Store[S]`)
- No external dependencies leak into core workflow logic

**No Changes Needed**

### III. Test-Driven Development (NON-NEGOTIABLE) ✅

**Status**: COMPLIANT (Confirmed)

**Design Validation**:
- Quickstart.md explicitly mandates TDD workflow (section "Development Workflow")
- Test fixtures defined in `testdata/fixtures/` (small & medium codebases)
- Unit test examples provided for all components
- Integration test strategy documented
- "Write tests first" emphasized throughout quickstart

**Test Coverage Plan**:
- State reducer: `workflow/state_test.go`
- File scanner: `scanner/scanner_test.go`
- Deduplicator: `consolidator/deduplicator_test.go`
- Nodes: `workflow/nodes_test.go`
- End-to-end: `workflow/graph_test.go`

**No Changes Needed**

### IV. Observability & Debugging ✅

**Status**: COMPLIANT (Confirmed)

**Design Validation**:
- Events documented for all workflow phases:
  - File discovery start/complete
  - Batch processing start/complete/error
  - Per-provider review start/complete/error
  - Consolidation start/complete
  - Report generation complete
- Progress tracking via `internal/progress.go` with 30-second intervals
- Errors captured in `WorkflowState.LastError` and `FailedProviders`
- All events include runID, step, nodeID per constitution requirements

**No Changes Needed**

### V. Dependency Minimalism ⚠️ JUSTIFIED DEVIATION (Confirmed)

**Status**: DEVIATION JUSTIFIED (No Change)

**Design Validation**:
- Example application lives in `examples/multi-llm-review/` with own `go.mod`
- Core framework (`/graph`) remains pure Go (unchanged)
- LLM SDKs isolated to example: `openai-go`, `anthropic-sdk-go`, `generative-ai-go`
- Quickstart.md documents separate module initialization (`go mod init`)
- Dependencies audited: official SDKs from OpenAI, Anthropic, Google

**Justification Remains Valid**: Example demonstrates framework integration, not core framework requirement

### Post-Design Gate Evaluation

**FINAL GATE STATUS**: ✅ PASS - READY FOR IMPLEMENTATION

- All 5 constitution principles validated against detailed design
- TDD workflow explicitly documented and mandated
- Interfaces, types, and contracts fully defined
- Observability requirements met with event emission strategy
- Justified deviation remains valid (example app with isolated dependencies)

**Recommendation**: Proceed to Phase 2 - Generate tasks.md via `/speckit.tasks`

**Approval**: Design phase complete, constitution compliant

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
examples/multi-llm-review/
├── main.go                    # CLI entry point, config loading, workflow execution
├── go.mod                     # Module file with LLM SDK dependencies
├── go.sum                     # Dependency checksums
├── config.example.yaml        # Example configuration file
├── README.md                  # Usage instructions and quick start
│
├── workflow/
│   ├── state.go              # ReviewState struct and reducer function
│   ├── nodes.go              # Node implementations (discover, batch, review, consolidate, report)
│   └── graph.go              # Graph wiring and engine setup
│
├── providers/
│   ├── provider.go           # CodeReviewer interface definition
│   ├── openai.go             # OpenAI GPT adapter (wraps anthropic-sdk-go)
│   ├── anthropic.go          # Anthropic Claude adapter (wraps anthropic-sdk-go)
│   ├── google.go             # Google Gemini adapter (wraps generative-ai-go)
│   └── mock.go               # Mock provider for testing
│
├── scanner/
│   ├── scanner.go            # File discovery, filtering, language detection
│   └── batcher.go            # Batch creation from file lists
│
├── consolidator/
│   ├── deduplicator.go       # Issue deduplication logic (fuzzy matching)
│   ├── prioritizer.go        # Severity ranking and sorting
│   └── reporter.go           # Markdown report generation
│
├── internal/
│   ├── config.go             # Configuration struct and validation
│   ├── retry.go              # Exponential backoff retry logic
│   └── progress.go           # Progress tracking and estimation
│
└── testdata/
    ├── fixtures/             # Example codebase fixtures for testing
    │   ├── small/           # 10 files for unit tests
    │   └── medium/          # 100 files for integration tests
    └── expected/            # Expected outputs for validation
        └── report_sample.md # Sample consolidated report

# Tests colocated with source
examples/multi-llm-review/
├── workflow/
│   ├── state_test.go
│   ├── nodes_test.go
│   └── graph_test.go
├── providers/
│   └── provider_test.go
├── scanner/
│   ├── scanner_test.go
│   └── batcher_test.go
├── consolidator/
│   ├── deduplicator_test.go
│   ├── prioritizer_test.go
│   └── reporter_test.go
└── internal/
    ├── config_test.go
    └── retry_test.go
```

**Structure Decision**: Single project structure within `examples/` directory

This is an example application demonstrating LangGraph-Go framework usage. The structure follows Go idioms:
- `/workflow` contains graph orchestration logic (state, nodes, engine setup)
- `/providers` contains AI provider adapters implementing `CodeReviewer` interface
- `/scanner` contains file discovery and batching logic
- `/consolidator` contains deduplication, prioritization, and reporting logic
- `/internal` contains utilities not exposed as public API
- `/testdata` contains fixtures and expected outputs for tests
- Tests are colocated with source files (`*_test.go`)

The example is self-contained with its own `go.mod` to isolate LLM SDK dependencies from the core framework.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| External LLM SDK dependencies (openai-go, anthropic-sdk-go, generative-ai-go) | Example application demonstrating multi-provider integration with real-world APIs | Custom HTTP clients would add 500+ lines, error-prone, requires maintaining 3 API integrations. Dependencies isolated to `examples/` directory, not core framework. |
