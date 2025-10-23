# TASK.md

This document tracks all tasks for the LangGraph-Go project, organized by category. Tasks are derived from the feature specification (specs/001-langgraph-core/spec.md) and project constitution.

**Last Updated**: 2025-10-23

---

## Setup & Infrastructure

- [ ] Initialize Go module structure with proper package organization
- [ ] Set up .gitignore for Go projects (vendor/, *.test, coverage files)
- [ ] Configure golangci-lint with project-specific rules
- [ ] Set up GitHub Actions / CI pipeline for automated testing
- [ ] Configure code coverage reporting
- [ ] Create example workflows directory with sample implementations
- [ ] Set up documentation generation (godoc)
- [ ] Create CONTRIBUTING.md with development guidelines
- [ ] Set up pre-commit hooks for formatting and linting

## Core Framework - Interfaces & Types (P1)

Following TDD principle: write tests first, then implement

- [ ] Define Node[S] interface with Run method signature
- [ ] Write tests for NodeFunc[S] functional node wrapper
- [ ] Implement NodeFunc[S] to satisfy Node[S] interface
- [ ] Define NodeResult[S] struct with Delta, Route, Events, Err fields
- [ ] Write tests for NodeResult validation and behavior
- [ ] Define Next struct for routing (To, Many, Terminal fields)
- [ ] Write tests for routing helpers: Stop(), Goto(id), fan-out
- [ ] Implement routing helper functions
- [ ] Define Reducer[S] function type for state merging
- [ ] Write tests for example reducer implementations
- [ ] Define Edge[S] struct with From, To, When predicate
- [ ] Write tests for conditional edge evaluation
- [ ] Define Predicate[S] function type
- [ ] Write tests for common predicates

## Core Framework - Engine (P1)

- [ ] Define Engine[S] struct with reducer, nodes, edges, store, emitter, opts
- [ ] Write tests for Engine construction with New[S]()
- [ ] Implement Engine[S] constructor
- [ ] Write tests for Engine.Add(nodeID, node) registration
- [ ] Implement node registration with duplicate detection
- [ ] Write tests for Engine.StartAt(nodeID) configuration
- [ ] Implement start node validation
- [ ] Write tests for Engine.Connect(from, to, predicate) edge wiring
- [ ] Implement edge connection with cycle detection warnings
- [ ] Write tests for Engine.Run(ctx, runID, initialState) execution
- [ ] Implement core execution loop with step tracking
- [ ] Write tests for MaxSteps limit enforcement
- [ ] Implement step limit with clear error messaging
- [ ] Write tests for context cancellation propagation
- [ ] Implement graceful shutdown on context cancel
- [ ] Write tests for execution history retrieval
- [ ] Implement execution trace querying

## State Management & Persistence (P1)

- [ ] Define Store[S] interface with SaveStep, LoadLatest, SaveCheckpoint, LoadCheckpoint
- [ ] Write tests for in-memory Store implementation
- [ ] Implement MemStore[S] with thread-safe maps
- [ ] Write tests for checkpoint save/load cycle
- [ ] Implement checkpoint management in MemStore
- [ ] Write tests for state serialization to JSON
- [ ] Implement JSON marshaling with error handling
- [ ] Write tests for state deserialization from JSON
- [ ] Implement JSON unmarshaling with validation
- [ ] Write tests for missing checkpoint error handling
- [ ] Write tests for concurrent access to Store
- [ ] Document Store interface contract and expectations
- [ ] Create example SQL Store implementation (mysql package)
- [ ] Write integration tests for SQL Store with test database

## Routing & Control Flow (P2)

- [ ] Write tests for linear workflow (A → B → C → Stop)
- [ ] Write tests for conditional routing based on state
- [ ] Write tests for loop detection and prevention
- [ ] Implement routing logic in Engine.Run
- [ ] Write tests for multiple matching predicates (priority order)
- [ ] Implement predicate evaluation order
- [ ] Write tests for no matching route error case
- [ ] Implement clear error messaging for routing failures
- [ ] Write tests for explicit Stop() termination
- [ ] Write tests for implicit termination (no next node)
- [ ] Document routing semantics and edge cases

## Parallel Execution & Fan-Out (P3)

- [ ] Write tests for fan-out to multiple nodes with Next.Many
- [ ] Write tests for state isolation in parallel branches
- [ ] Implement state deep-copy for branch isolation
- [ ] Write tests for concurrent node execution timing
- [ ] Implement goroutine-based parallel execution
- [ ] Write tests for reducer-based state merging at join
- [ ] Implement deterministic merge order
- [ ] Write tests for error handling in one parallel branch
- [ ] Implement error aggregation from parallel branches
- [ ] Write tests for fan-out with no join (fire-and-forget)
- [ ] Document concurrency model and goroutine safety

## Error Handling & Retry (P1-P2)

- [ ] Define Options struct with MaxSteps, Retries fields
- [ ] Write tests for node-level error capture in NodeResult.Err
- [ ] Write tests for error propagation to state (LastError field)
- [ ] Implement error state updates
- [ ] Write tests for retry logic with configurable attempts
- [ ] Implement retry mechanism with backoff (optional)
- [ ] Write tests for max retries exceeded error
- [ ] Write tests for context cancellation during retry
- [ ] Write tests for error routing to error-handling nodes
- [ ] Document error handling patterns and best practices

## Observability & Events (P5)

- [ ] Define Event struct with RunID, Step, NodeID, Msg, Meta fields
- [ ] Define Emitter interface with Emit(Event) method
- [ ] Write tests for LogEmitter to stdout
- [ ] Implement LogEmitter with structured output
- [ ] Write tests for event emission on node start
- [ ] Implement node start event in Engine
- [ ] Write tests for event emission on node end with delta
- [ ] Implement node end event with state capture
- [ ] Write tests for event emission on routing decision
- [ ] Implement routing event with chosen path
- [ ] Write tests for event emission on error
- [ ] Implement error event with stack trace
- [ ] Write tests for NullEmitter (no-op)
- [ ] Implement NullEmitter for production opt-out
- [ ] Create example OpenTelemetry emitter (otel package)
- [ ] Document event schema and metadata conventions

## LLM Integration - Core Abstractions (P4)

- [ ] Define Message struct with Role, Content fields
- [ ] Define ToolSpec struct for tool declarations
- [ ] Define ChatOut struct with Text, ToolCalls fields
- [ ] Define ChatModel interface with Chat method signature
- [ ] Write tests for ChatModel mock implementation
- [ ] Create MockChatModel for unit testing
- [ ] Define Tool interface with Name(), Call() methods
- [ ] Write tests for Tool mock implementation
- [ ] Create MockTool for unit testing
- [ ] Document ChatModel and Tool contracts

## LLM Integration - Provider Adapters (P4)

- [ ] Create openai package for OpenAI adapter
- [ ] Write tests for OpenAI adapter with API mocks
- [ ] Implement OpenAIChatModel wrapping openai-go SDK
- [ ] Write tests for error handling (rate limits, API errors)
- [ ] Create anthropic package for Anthropic adapter
- [ ] Write tests for Anthropic adapter with API mocks
- [ ] Implement AnthropicChatModel wrapping anthropic-sdk-go
- [ ] Create google package for Google Gemini adapter
- [ ] Write tests for Google adapter with API mocks
- [ ] Implement GoogleChatModel wrapping generative-ai-go
- [ ] Create ollama package for local model adapter
- [ ] Write integration tests for Ollama (requires local setup)
- [ ] Implement OllamaChatModel for local models
- [ ] Document adapter usage and provider switching

## Testing & Examples

- [ ] Create example: simple 3-node linear workflow
- [ ] Create example: conditional routing with decision node
- [ ] Create example: parallel execution with fan-out/join
- [ ] Create example: checkpointing and resumption
- [ ] Create example: LLM integration with multiple providers
- [ ] Create example: error handling and retries
- [ ] Create example: custom reducer for complex state merging
- [ ] Write integration test: end-to-end workflow with MemStore
- [ ] Write integration test: checkpoint save/resume cycle
- [ ] Write integration test: parallel execution correctness
- [ ] Write benchmark: large workflow (100+ nodes)
- [ ] Write benchmark: high-frequency small workflows
- [ ] Create testing utilities package for common test helpers

## Documentation

- [ ] Write comprehensive README.md with quick start
- [ ] Document core concepts (Node, State, Reducer, Engine)
- [ ] Document routing and control flow patterns
- [ ] Document parallel execution model
- [ ] Document error handling strategies
- [ ] Document observability and event system
- [ ] Document LLM integration patterns
- [ ] Create architecture diagrams (execution flow, state lifecycle)
- [ ] Write godoc comments for all exported types and functions
- [ ] Create tutorial: building your first workflow
- [ ] Create tutorial: integrating with LLM providers
- [ ] Create migration guide (if applicable to future versions)

## Production Readiness

- [ ] Implement SQL Store adapter (PostgreSQL)
- [ ] Write integration tests for SQL Store
- [ ] Implement SQL Store adapter (MySQL)
- [ ] Create OpenTelemetry emitter with trace spans
- [ ] Write tests for OpenTelemetry integration
- [ ] Implement visualization export (DOT format for Graphviz)
- [ ] Create CLI tool for workflow visualization
- [ ] Implement workflow validation (detect cycles, orphan nodes)
- [ ] Write tests for workflow validation
- [ ] Add metrics collection (execution time, node counts)
- [ ] Create performance profiling guide
- [ ] Document deployment patterns
- [ ] Document scaling considerations

## Future Considerations (Out of Scope for v1)

- [ ] HTTP/GraphQL API for remote workflow execution
- [ ] Visual workflow editor (web-based)
- [ ] Distributed execution with CRDT-based reducers
- [ ] Workflow versioning and migration tools
- [ ] Policy framework for access control
- [ ] Async node scheduling for long-running jobs
- [ ] Real-time workflow monitoring dashboard
- [ ] Workflow marketplace and template library

---

## Completed Work

**Format**: [Date] - Task description

_No completed work yet - this section will be populated as tasks are finished._

---

## Task Management Notes

- **Priority Mapping**: Tasks are tagged with priority (P1-P5) matching user stories from spec
- **TDD Enforcement**: All implementation tasks must have corresponding test tasks completed first
- **Dependencies**: Some tasks have implicit dependencies (e.g., Engine requires Node interface)
- **Constitution Compliance**: All tasks follow principles from constitution.md (type safety, interfaces, TDD, observability, minimal deps)

**How to use this file**:
1. Check off tasks as they are completed with [X]
2. Move completed tasks to "Completed Work" section with date
3. Add new tasks as requirements evolve
4. Reference task numbers in commit messages and PRs
5. Update priorities if user needs change
