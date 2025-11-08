<!--
Sync Impact Report:
Version: 1.1.0 → 1.2.0
Reason: MINOR version bump - Added mandatory gocontext tooling requirement for codebase navigation

Modified Principles:
- No core principles modified

Added Sections:
- Development Workflow > Codebase Navigation: Added mandatory gocontext usage and synchronization

Removed Sections:
- None

Templates Requiring Updates:
✅ plan-template.md - No changes needed (generic template, constitution check references principles)
✅ spec-template.md - No changes needed (generic template)
✅ tasks-template.md - No changes needed (already emphasizes tests before commits)
✅ CLAUDE.md - Needs update to reference gocontext usage
⚠ README.md - May benefit from gocontext mention in Development section (optional)

Follow-up TODOs:
- Update CLAUDE.md to include gocontext usage guidance in "Development Commands" section
- Consider adding gocontext setup to project onboarding documentation

Date: 2025-11-08
-->

# LangGraph-Go Constitution

## Core Principles

### I. Type Safety & Determinism

All state management MUST use Go generics to provide compile-time type safety. Every execution
MUST be deterministic and replayable from checkpoints. State transitions MUST be pure functions
(reducers) that can be re-executed without side effects.

**Rationale**: Deterministic replay enables debugging of production issues, testing of complex
workflows, and reliable resumption after failures. Type safety prevents runtime state corruption
and makes refactoring safe.

**Requirements**:
- All state types MUST be serializable to JSON
- Reducer functions MUST be pure (no external state access, no I/O)
- Node execution MUST return explicit `NodeResult[S]` with delta state
- Every state mutation MUST be captured in persisted steps

### II. Interface-First Design

Core abstractions (Node, Store, Emitter, ChatModel, Tool) MUST be defined as interfaces before
implementation. External dependencies (LLM providers, databases, observability systems) MUST be
accessed through adapters implementing framework interfaces.

**Rationale**: Interface-first design enables testability with mocks, supports multiple
implementations (in-memory vs production stores), and isolates external dependencies. This
keeps the core framework pure Go with optional adapters.

**Requirements**:
- Every integration point MUST have an interface definition in the core framework
- Production implementations MUST live in separate packages (e.g., `store/mysql`, `model/openai`)
- Test implementations MUST be provided (e.g., `NewMemStore[S]()`)
- Breaking changes to interfaces require MAJOR version bump

### III. Test-Driven Development (NON-NEGOTIABLE)

Every feature MUST follow TDD: write failing tests → implement until tests pass → refactor.
Tests MUST verify behavior, not implementation details. All exported functions and types MUST
have test coverage.

**Rationale**: TDD ensures design quality (testable code is well-designed code), provides
living documentation, enables safe refactoring, and catches regressions.

**Requirements**:
- Tests written and approved BEFORE implementation
- Unit tests for all node logic, reducers, and utilities
- Integration tests for Store implementations, Engine execution flows
- Example-based tests for documentation (e.g., `TestSmallLoop` in spec)
- Red-Green-Refactor cycle strictly enforced
- No commits without passing tests

### IV. Observability & Debugging

Every execution step MUST emit events through the Emitter interface. Events MUST include runID,
step number, nodeID, and metadata. Errors MUST be captured in state and persisted. The
framework MUST support inspection and visualization of execution history.

**Rationale**: Graph-based workflows can be complex to debug. Comprehensive observability
enables developers to understand execution flow, diagnose failures, and optimize performance.

**Requirements**:
- Every node execution MUST emit start/end events
- Errors MUST be persisted in `LastError` state field and emitted as events
- Store implementations MUST persist full execution history
- Support export to OpenTelemetry, structured logs, and DOT/PNG visualization
- Events MUST be machine-parseable (structured data, not just log strings)

### V. Dependency Minimalism

The core framework (`/graph`) MUST remain pure Go with no external dependencies. Optional
integrations (LLM SDKs, databases, observability) MUST be isolated in adapter packages.
Dependencies MUST be justified and regularly reviewed.

**Rationale**: Minimal dependencies reduce supply chain risk, improve build times, simplify
maintenance, and keep the framework lightweight. Users should only pay for what they use.

**Requirements**:
- Core framework MUST compile with only standard library
- Adapters MAY depend on external SDKs (OpenAI, Anthropic, Google SDKs)
- New dependencies require explicit justification in PR
- Transitive dependencies MUST be audited (supply chain security)
- Use `go mod tidy` to remove unused dependencies

## Go Idioms & Best Practices

### Idiomatic Go

Code MUST follow official Go guidelines:
- Use `gofmt` for formatting (enforced in CI)
- Follow effective Go naming conventions (short, descriptive names)
- Prefer composition over inheritance
- Use context.Context for cancellation and deadlines
- Return errors explicitly (no panics in library code)

### Generics Usage

- State type parameter `[S any]` MUST be consistent across Engine, Node, Store
- Avoid complex type constraints unless necessary
- Document generic type requirements in godoc

### Error Handling

- All errors MUST be returned explicitly, not logged and swallowed
- Use `fmt.Errorf` with `%w` for error wrapping
- Capture errors in `NodeResult.Err` for node-level failures
- Retry logic MUST respect `Options.Retries` configuration

### Concurrency

- Fan-out execution MUST isolate state per branch (deep copy)
- Use goroutines and channels for parallel node execution
- Join nodes MUST merge state using the reducer function
- Engine MUST enforce `MaxSteps` to prevent infinite loops

## Development Workflow

### Codebase Navigation

**gocontext Indexing (MANDATORY)**:
- ALL codebase exploration MUST use `gocontext` for fast semantic search
- The codebase MUST be indexed using `mcp__gocontext__index_codebase` at project start
- gocontext index MUST be updated after significant file changes (new packages, major refactors)
- Use `mcp__gocontext__search_code` for natural language code queries instead of manual grep/find

**Rationale**: gocontext provides semantic code search with natural language queries, DDD pattern
filtering, and context-aware results. This dramatically improves code discovery, reduces time
spent searching, and enables better understanding of architecture patterns.

**Requirements**:
- Index the codebase before beginning feature work: `mcp__gocontext__index_codebase(path, include_tests=true)`
- Re-index after adding/moving packages or substantial code changes
- Use semantic search for questions like "find all aggregates" or "error handling patterns"
- Prefer gocontext over grep/ripgrep for exploratory code search
- Use `search_mode="hybrid"` for balanced semantic + keyword search
- Filter by `ddd_patterns` when searching for domain patterns (aggregates, entities, repositories, etc.)

### Code Review

**Pre-Commit Review (MANDATORY)**:
- ALL code changes MUST be reviewed using `mcp-pr` before committing
- Run automated review: `mcp-pr review_unstaged` for unstaged changes or `mcp-pr review_staged` for staged changes
- Address all issues flagged by automated review (security, bugs, performance, style)
- Document any intentional deviations with clear rationale

**Pull Request Review**:
- All changes MUST go through PR review after local review
- PRs MUST reference tests that verify the change
- Breaking changes MUST be called out explicitly
- Constitution compliance MUST be verified in review

**Review Focus Areas**:
- Type safety and error handling
- Test coverage and TDD compliance
- Interface usage and dependency injection
- Observability (event emission)
- Performance and resource usage
- Security vulnerabilities

### Testing Gates

- `go test ./...` MUST pass before merge
- Coverage SHOULD be measured but not strictly enforced (quality over quantity)
- Integration tests MUST pass for Store and ChatModel adapters

### Documentation

- All exported types and functions MUST have godoc comments
- Complex algorithms MUST include inline comments explaining logic
- Examples MUST be runnable (use example tests: `func ExampleEngine_Run()`)
- Update CLAUDE.md when adding major features or architectural changes

### Versioning

Semantic versioning (MAJOR.MINOR.PATCH):
- MAJOR: Breaking changes to public API (interface changes, removed features)
- MINOR: New features, new interfaces, backward-compatible additions
- PATCH: Bug fixes, documentation improvements, performance optimizations

## Governance

This constitution supersedes all other development practices. Amendments require:
1. Proposal with rationale
2. Team review and approval
3. Version increment following semantic versioning rules
4. Migration plan for breaking changes

All PRs and reviews MUST verify compliance with these principles. Complexity MUST be justified
against the principle of simplicity. Use CLAUDE.md for runtime development guidance and quick
reference.

**Version**: 1.2.0 | **Ratified**: 2025-10-23 | **Last Amended**: 2025-11-08
