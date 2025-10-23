# Tasks: LangGraph-Go Core Framework

**Input**: Design documents from `/specs/001-langgraph-core/`
**Prerequisites**: spec.md (user stories with priorities)
**Project**: Go framework for stateful, graph-based LLM workflows

**TDD Approach**: Per constitution.md, ALL tasks follow Test-Driven Development:
- Tests written BEFORE implementation
- Red-Green-Refactor cycle enforced
- Each implementation task has corresponding test task(s)

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Format: `- [ ] [TaskID] [P?] [Story] Description with file path`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3, US4, US5)
- File paths use `/graph` as root package

## Implementation Strategy

**MVP Scope**: User Story 1 (US1) only - delivers core checkpoint/resume functionality
**Incremental Delivery**: Each user story is independently testable and deliverable
**Priority Order**: US1 (P1) → US2 (P2) → US3 (P3) → US4 (P4) → US5 (P5)

---

## Phase 1: Setup & Project Initialization

**Purpose**: Initialize Go project structure, tooling, and development environment

- [X] T001 Initialize go.mod for github.com/dshills/langgraph-go
- [X] T002 Create /graph package directory structure
- [X] T003 [P] Create /graph/store sub-package directory
- [X] T004 [P] Create /graph/emit sub-package directory
- [X] T005 [P] Create /graph/model sub-package directory
- [X] T006 [P] Create /graph/tool sub-package directory
- [X] T007 [P] Create /examples directory for sample workflows
- [X] T008 Create .gitignore for Go projects (vendor/, *.test, coverage.out)
- [X] T009 Configure golangci-lint with .golangci.yml
- [X] T010 Create README.md with quick start guide
- [X] T011 Create CONTRIBUTING.md with TDD workflow

---

## Phase 2: Foundational Interfaces & Types

**Purpose**: Define core abstractions needed by ALL user stories (blocking prerequisites)

**Story Goal**: Establish type-safe interfaces that all workflow components depend on

**Independent Test**: Interfaces compile, have godoc, and can be mocked in tests

### Core Type Definitions (Foundational)

- [X] T012 Write tests for Node[S] interface contract in graph/node_test.go
- [X] T013 Define Node[S] interface with Run(ctx, state) method in graph/node.go
- [X] T014 [P] Write tests for NodeResult[S] struct validation in graph/node_test.go
- [X] T015 [P] Define NodeResult[S] struct (Delta, Route, Events, Err) in graph/node.go
- [X] T016 [P] Write tests for Next struct routing scenarios in graph/node_test.go
- [X] T017 [P] Define Next struct (To, Many, Terminal) in graph/node.go
- [X] T018 Write tests for Stop() and Goto() helpers in graph/node_test.go
- [X] T019 Implement Stop() and Goto(string) routing helpers in graph/node.go
- [X] T020 [P] Write tests for NodeFunc[S] wrapper in graph/node_test.go
- [X] T021 [P] Implement NodeFunc[S] functional node wrapper in graph/node.go

### State Management Types (Foundational)

- [X] T022 Write tests for Reducer[S] function examples in graph/state_test.go
- [X] T023 Define Reducer[S] function type in graph/state.go
- [X] T024 [P] Write tests for Edge[S] struct and predicates in graph/edge_test.go
- [X] T025 [P] Define Edge[S] struct (From, To, When) in graph/edge.go
- [X] T026 [P] Define Predicate[S] function type in graph/edge.go

### Storage Interface (Foundational)

- [X] T027 Write tests for Store[S] interface contract in graph/store/store_test.go
- [X] T028 Define Store[S] interface (SaveStep, LoadLatest, SaveCheckpoint, LoadCheckpoint) in graph/store/store.go

### Event System (Foundational)

- [X] T029 Write tests for Event struct in graph/emit/event_test.go
- [X] T030 Define Event struct (RunID, Step, NodeID, Msg, Meta) in graph/emit/event.go
- [X] T031 [P] Write tests for Emitter interface in graph/emit/emitter_test.go
- [X] T032 [P] Define Emitter interface with Emit(Event) in graph/emit/emitter.go

---

## Phase 3: User Story 1 - Build Stateful Workflow with Checkpointing (Priority: P1)

**Story Goal**: Enable workflows to be interrupted and resumed from any point using persisted state

**Independent Test**: Create 3-node workflow, save after each step, kill process, resume from checkpoint to completion

**Why First**: Core value proposition - without this, it's just a basic graph executor

### US1: In-Memory Store Implementation

- [X] T033 [US1] Write tests for MemStore[S] construction in graph/store/memory_test.go
- [X] T034 [US1] Implement MemStore[S] struct with maps in graph/store/memory.go
- [X] T035 [US1] Write tests for MemStore SaveStep with concurrent access in graph/store/memory_test.go
- [X] T036 [US1] Implement MemStore SaveStep method in graph/store/memory.go
- [X] T037 [P] [US1] Write tests for MemStore LoadLatest in graph/store/memory_test.go
- [X] T038 [P] [US1] Implement MemStore LoadLatest method in graph/store/memory.go
- [X] T039 [P] [US1] Write tests for MemStore SaveCheckpoint with labels in graph/store/memory_test.go
- [X] T040 [P] [US1] Implement MemStore SaveCheckpoint method in graph/store/memory.go
- [X] T041 [P] [US1] Write tests for MemStore LoadCheckpoint error cases in graph/store/memory_test.go
- [X] T042 [P] [US1] Implement MemStore LoadCheckpoint method in graph/store/memory.go

### US1: Engine Core Construction

- [X] T043 [US1] Write tests for Engine[S] construction in graph/engine_test.go
- [X] T044 [US1] Define Engine[S] struct (reducer, nodes, edges, store, emitter, opts) in graph/engine.go
- [X] T045 [US1] Write tests for Options struct (MaxSteps, Retries) in graph/engine_test.go
- [X] T046 [US1] Define Options struct in graph/engine.go
- [X] T047 [US1] Write tests for New[S]() constructor validation in graph/engine_test.go
- [X] T048 [US1] Implement New[S](reducer, store, emitter, opts) constructor in graph/engine.go

### US1: Engine Workflow Building

- [X] T049 [US1] Write tests for Engine.Add(nodeID, node) in graph/engine_test.go
- [X] T050 [US1] Implement Engine.Add with duplicate detection in graph/engine.go
- [X] T051 [P] [US1] Write tests for Engine.StartAt(nodeID) validation in graph/engine_test.go
- [X] T052 [P] [US1] Implement Engine.StartAt with node existence check in graph/engine.go
- [X] T053 [P] [US1] Write tests for Engine.Connect(from, to, predicate) in graph/engine_test.go
- [X] T054 [P] [US1] Implement Engine.Connect for edge wiring in graph/engine.go

### US1: Engine Linear Execution

- [X] T055 [US1] Write tests for Engine.Run(ctx, runID, initialState) basic execution in graph/engine_test.go
- [X] T056 [US1] Implement Engine.Run core execution loop in graph/engine.go
- [X] T057 [US1] Write tests for state persistence after each node in graph/engine_test.go
- [X] T058 [US1] Implement Store.SaveStep calls in execution loop in graph/engine.go
- [X] T059 [US1] Write tests for MaxSteps limit enforcement in graph/engine_test.go
- [X] T060 [US1] Implement MaxSteps checking and error in graph/engine.go

### US1: Checkpoint & Resume

- [X] T061 [US1] Write tests for checkpoint save at specific steps in graph/engine_test.go
- [X] T062 [US1] Implement Engine.SaveCheckpoint(label) method in graph/engine.go
- [X] T063 [US1] Write tests for resume from checkpoint in graph/engine_test.go
- [X] T064 [US1] Implement Engine.ResumeFromCheckpoint(cpID, newRunID, startNode) in graph/engine.go
- [X] T065 [US1] Write tests for missing checkpoint error handling in graph/engine_test.go (covered in T063)
- [X] T066 [US1] Implement clear error for nonexistent checkpoint in graph/engine.go (covered in T064)

### US1: Context Cancellation

- [X] T067 [US1] Write tests for context cancellation during execution in graph/engine_test.go
- [X] T068 [US1] Implement context.Context checking in execution loop in graph/engine.go
- [X] T069 [US1] Write tests for graceful shutdown with state save in graph/engine_test.go
- [X] T070 [US1] Implement state persistence before cancellation exit in graph/engine.go

### US1: State Serialization

- [X] T071 [P] [US1] Write tests for JSON serialization of state in graph/store/memory_test.go
- [X] T072 [P] [US1] Implement JSON marshaling in MemStore in graph/store/memory.go
- [X] T073 [P] [US1] Write tests for JSON deserialization in graph/store/memory_test.go
- [X] T074 [P] [US1] Implement JSON unmarshaling in MemStore in graph/store/memory.go

### US1: Example & Integration Test

- [X] T075 [US1] Create example 3-node workflow in examples/checkpoint/main.go
- [X] T076 [US1] Write integration test for checkpoint/resume cycle in graph/integration_test.go
- [X] T077 [US1] Write integration test for 5-node workflow crash scenario in graph/integration_test.go

---

## Phase 4: User Story 2 - Define Dynamic Routing with Conditional Logic (Priority: P2)

**Story Goal**: Enable workflows where next step depends on current state (loops, branches, adaptive behavior)

**Independent Test**: Create workflow with judge node that routes based on state.confidence threshold

**Dependencies**: Requires US1 (Engine execution) to be complete

### US2: Conditional Edge Evaluation

- [X] T078 [US2] Write tests for predicate evaluation in routing in graph/engine_test.go
- [X] T079 [US2] Implement predicate checking in Engine.Run routing logic in graph/engine.go
- [X] T080 [P] [US2] Write tests for multiple matching predicates (priority order) in graph/engine_test.go
- [X] T081 [P] [US2] Implement predicate priority (first match wins) in graph/engine.go

### US2: Routing Decision Logic

- [X] T082 [US2] Write tests for node-provided Next routing in graph/engine_test.go
- [X] T083 [US2] Implement NodeResult.Route handling in Engine.Run in graph/engine.go
- [X] T084 [US2] Write tests for edge-based routing fallback in graph/engine_test.go
- [X] T085 [US2] Implement edge predicate fallback when Next.To is empty in graph/engine.go

### US2: Error Cases

- [X] T086 [US2] Write tests for no matching route error in graph/engine_test.go
- [X] T087 [US2] Implement clear error when no route found in graph/engine.go
- [X] T088 [P] [US2] Write tests for routing to nonexistent node in graph/engine_test.go
- [X] T089 [P] [US2] Implement validation for target node existence in graph/engine.go

### US2: Loop Support

- [X] T090 [US2] Write tests for workflow loops (A → B → A) in graph/engine_test.go
- [X] T091 [US2] Ensure MaxSteps prevents infinite loops in graph/engine.go
- [X] T092 [US2] Write tests for conditional loop exit in graph/engine_test.go
- [X] T093 [US2] Document loop patterns and MaxSteps usage in graph/engine.go

### US2: Termination

- [ ] T094 [US2] Write tests for explicit Stop() termination in graph/engine_test.go
- [ ] T095 [US2] Implement Next.Terminal checking in Engine.Run in graph/engine.go
- [ ] T096 [P] [US2] Write tests for implicit termination (no edges) in graph/engine_test.go
- [ ] T097 [P] [US2] Handle implicit termination in routing logic in graph/engine.go

### US2: Example & Integration Test

- [ ] T098 [US2] Create example workflow with conditional routing in examples/routing/main.go
- [ ] T099 [US2] Write integration test for confidence-based routing in graph/integration_test.go
- [ ] T100 [US2] Write integration test for loop with exit condition in graph/integration_test.go

---

## Phase 5: User Story 3 - Execute Parallel Node Branches (Priority: P3)

**Story Goal**: Fan out execution to concurrent nodes, merge results deterministically

**Independent Test**: Workflow with 3 parallel nodes (1s each) completes in ~1s total, state contains all results

**Dependencies**: Requires US1 (Engine, State) and US2 (Routing) to be complete

### US3: State Isolation & Deep Copy

- [ ] T101 [US3] Write tests for state deep copy utility in graph/state_test.go
- [ ] T102 [US3] Implement deepCopy[S](state) helper using JSON round-trip in graph/state.go
- [ ] T103 [US3] Write tests for isolated state per branch in graph/engine_test.go
- [ ] T104 [US3] Apply deep copy before each parallel branch execution in graph/engine.go

### US3: Parallel Execution

- [ ] T105 [US3] Write tests for Next.Many fan-out in graph/engine_test.go
- [ ] T106 [US3] Implement goroutine spawning for Next.Many in Engine.Run in graph/engine.go
- [ ] T107 [US3] Write tests for concurrent timing (4 branches in parallel time) in graph/engine_test.go
- [ ] T108 [US3] Use sync.WaitGroup for parallel branch coordination in graph/engine.go

### US3: Result Merging

- [ ] T109 [US3] Write tests for reducer-based merge at join in graph/engine_test.go
- [ ] T110 [US3] Implement deterministic merge order using reducer in graph/engine.go
- [ ] T111 [US3] Write tests for merge order guarantee in graph/engine_test.go
- [ ] T112 [US3] Document merge order semantics (lexicographic by nodeID) in graph/engine.go

### US3: Error Handling in Parallel

- [ ] T113 [US3] Write tests for error in one parallel branch in graph/engine_test.go
- [ ] T114 [US3] Collect errors from all branches using error channel in graph/engine.go
- [ ] T115 [US3] Write tests for multiple branch failures in graph/engine_test.go
- [ ] T116 [US3] Aggregate errors into state.LastError field in graph/engine.go

### US3: Example & Integration Test

- [ ] T117 [US3] Create example workflow with parallel fan-out in examples/parallel/main.go
- [ ] T118 [US3] Write integration test for 4-branch parallel execution in graph/integration_test.go
- [ ] T119 [US3] Write integration test for parallel error handling in graph/integration_test.go

---

## Phase 6: User Story 4 - Integrate LLM Providers (Priority: P4)

**Story Goal**: Consistent interface for calling OpenAI, Anthropic, Google, local models without coupling

**Independent Test**: Workflow calls 3 different providers sequentially, state accumulates all responses

**Dependencies**: Requires US1 (Engine) to be complete; independent of US2/US3

### US4: LLM Abstractions

- [ ] T120 [US4] Write tests for Message struct in graph/model/chat_test.go
- [ ] T121 [US4] Define Message struct (Role, Content) in graph/model/chat.go
- [ ] T122 [P] [US4] Write tests for ToolSpec struct in graph/model/chat_test.go
- [ ] T123 [P] [US4] Define ToolSpec struct (Name, Description, Schema) in graph/model/chat.go
- [ ] T124 [P] [US4] Write tests for ChatOut struct in graph/model/chat_test.go
- [ ] T125 [P] [US4] Define ChatOut struct (Text, ToolCalls) in graph/model/chat.go

### US4: ChatModel Interface

- [ ] T126 [US4] Write tests for ChatModel interface contract in graph/model/chat_test.go
- [ ] T127 [US4] Define ChatModel interface with Chat(ctx, messages, tools) in graph/model/chat.go
- [ ] T128 [US4] Create MockChatModel for testing in graph/model/mock.go
- [ ] T129 [US4] Write tests for MockChatModel behavior in graph/model/mock_test.go

### US4: Tool Interface

- [ ] T130 [P] [US4] Write tests for Tool interface contract in graph/tool/tool_test.go
- [ ] T131 [P] [US4] Define Tool interface with Name(), Call(ctx, input) in graph/tool/tool.go
- [ ] T132 [P] [US4] Create MockTool for testing in graph/tool/mock.go
- [ ] T133 [P] [US4] Write tests for MockTool behavior in graph/tool/mock_test.go

### US4: OpenAI Adapter

- [ ] T134 [US4] Create graph/model/openai sub-package
- [ ] T135 [US4] Write tests for OpenAIChatModel in graph/model/openai/openai_test.go
- [ ] T136 [US4] Implement OpenAIChatModel wrapping openai-go SDK in graph/model/openai/openai.go
- [ ] T137 [US4] Write tests for OpenAI error handling (rate limits) in graph/model/openai/openai_test.go
- [ ] T138 [US4] Implement retry logic for transient errors in graph/model/openai/openai.go

### US4: Anthropic Adapter

- [ ] T139 [P] [US4] Create graph/model/anthropic sub-package
- [ ] T140 [P] [US4] Write tests for AnthropicChatModel in graph/model/anthropic/anthropic_test.go
- [ ] T141 [P] [US4] Implement AnthropicChatModel wrapping anthropic-sdk-go in graph/model/anthropic/anthropic.go
- [ ] T142 [P] [US4] Write tests for Anthropic error handling in graph/model/anthropic/anthropic_test.go
- [ ] T143 [P] [US4] Implement error translation to common format in graph/model/anthropic/anthropic.go

### US4: Google Adapter

- [ ] T144 [P] [US4] Create graph/model/google sub-package
- [ ] T145 [P] [US4] Write tests for GoogleChatModel in graph/model/google/google_test.go
- [ ] T146 [P] [US4] Implement GoogleChatModel wrapping generative-ai-go in graph/model/google/google.go
- [ ] T147 [P] [US4] Write tests for Google safety filter handling in graph/model/google/google_test.go
- [ ] T148 [P] [US4] Implement safety filter error handling in graph/model/google/google.go

### US4: Example & Integration Test

- [ ] T149 [US4] Create example workflow with multi-provider LLM calls in examples/llm/main.go
- [ ] T150 [US4] Write integration test with mocked LLM providers in graph/integration_test.go
- [ ] T151 [US4] Document provider switching patterns in graph/model/chat.go

---

## Phase 7: User Story 5 - Debug Execution with Event Tracing (Priority: P5)

**Story Goal**: Observe every execution step with detailed events for debugging and performance analysis

**Independent Test**: 10-node workflow exports trace capturing all node transitions and state changes

**Dependencies**: Requires US1 (Engine) to be complete; enhances all other stories

### US5: Event Emission in Engine

- [ ] T152 [US5] Write tests for node_start event emission in graph/engine_test.go
- [ ] T153 [US5] Emit node_start event at beginning of node execution in graph/engine.go
- [ ] T154 [US5] Write tests for node_end event with delta in graph/engine_test.go
- [ ] T155 [US5] Emit node_end event after state merge in graph/engine.go
- [ ] T156 [P] [US5] Write tests for routing_decision event in graph/engine_test.go
- [ ] T157 [P] [US5] Emit routing_decision event with chosen path in graph/engine.go
- [ ] T158 [P] [US5] Write tests for error event emission in graph/engine_test.go
- [ ] T159 [P] [US5] Emit error event on NodeResult.Err in graph/engine.go

### US5: LogEmitter Implementation

- [ ] T160 [US5] Write tests for LogEmitter in graph/emit/log_test.go
- [ ] T161 [US5] Implement LogEmitter with structured output to stdout in graph/emit/log.go
- [ ] T162 [US5] Write tests for LogEmitter JSON formatting in graph/emit/log_test.go
- [ ] T163 [US5] Add JSON output option to LogEmitter in graph/emit/log.go

### US5: NullEmitter (Opt-Out)

- [ ] T164 [P] [US5] Write tests for NullEmitter (no-op) in graph/emit/null_test.go
- [ ] T165 [P] [US5] Implement NullEmitter for production opt-out in graph/emit/null.go

### US5: Event Metadata

- [ ] T166 [US5] Write tests for Event.Meta population in graph/emit/event_test.go
- [ ] T167 [US5] Add helper for common metadata (duration, nodeType) in graph/emit/event.go
- [ ] T168 [US5] Document Event.Meta conventions in graph/emit/event.go

### US5: Execution History Query

- [ ] T169 [US5] Write tests for execution history retrieval in graph/engine_test.go
- [ ] T170 [US5] Implement Engine.GetHistory(runID) method in graph/engine.go
- [ ] T171 [US5] Write tests for history filtering (by nodeID, timerange) in graph/engine_test.go
- [ ] T172 [US5] Add filtering options to GetHistory in graph/engine.go

### US5: Example & Integration Test

- [ ] T173 [US5] Create example workflow with event tracing in examples/tracing/main.go
- [ ] T174 [US5] Write integration test for 10-node trace capture in graph/integration_test.go
- [ ] T175 [US5] Document event schema and observability patterns in graph/emit/event.go

---

## Phase 8: Cross-Cutting Concerns & Polish

**Purpose**: Features that enhance multiple user stories, documentation, production readiness

### Error Handling & Retry

- [ ] T176 Write tests for Options.Retries behavior in graph/engine_test.go
- [ ] T177 Implement retry logic with exponential backoff in graph/engine.go
- [ ] T178 Write tests for max retries exceeded error in graph/engine_test.go
- [ ] T179 Document retry patterns and backoff strategy in graph/engine.go

### Validation & Safety

- [ ] T180 Write tests for workflow validation (detect orphan nodes) in graph/engine_test.go
- [ ] T181 Implement Engine.Validate() method in graph/engine.go
- [ ] T182 [P] Write tests for cycle detection warning in graph/engine_test.go
- [ ] T183 [P] Add cycle detection in Engine.Connect in graph/engine.go

### Examples & Documentation

- [ ] T184 [P] Create simple 3-node linear workflow example in examples/simple/main.go
- [ ] T185 [P] Create complex workflow example (all features) in examples/complex/main.go
- [ ] T186 Write comprehensive README.md with architecture overview
- [ ] T187 Add godoc package comment to graph/doc.go
- [ ] T188 Create tutorial: "Building Your First Workflow" in docs/tutorial.md
- [ ] T189 Create architecture diagrams (execution flow) in docs/architecture.md

### Production Features (Optional - SQL Store)

- [ ] T190 Create graph/store/sql sub-package for PostgreSQL
- [ ] T191 Write integration tests for SQL Store in graph/store/sql/sql_test.go
- [ ] T192 Implement PostgreSQL Store adapter in graph/store/sql/postgres.go
- [ ] T193 Document SQL schema requirements in graph/store/sql/README.md

### Production Features (Optional - OpenTelemetry)

- [ ] T194 [P] Create graph/emit/otel sub-package
- [ ] T195 [P] Write tests for OtelEmitter in graph/emit/otel/otel_test.go
- [ ] T196 [P] Implement OtelEmitter with trace spans in graph/emit/otel/otel.go
- [ ] T197 [P] Document OpenTelemetry integration in graph/emit/otel/README.md

### Benchmarks & Performance

- [ ] T198 Write benchmark for large workflow (100+ nodes) in graph/benchmark_test.go
- [ ] T199 Write benchmark for high-frequency small workflows in graph/benchmark_test.go
- [ ] T200 Profile memory usage with pprof in graph/benchmark_test.go
- [ ] T201 Document performance characteristics in docs/performance.md

---

## Dependencies & Execution Order

### Story Dependency Graph

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational Interfaces) ← MUST complete before any user story
    ↓
    ├─→ US1 (P1) - Checkpointing     [MVP - Can ship alone]
    │       ↓
    │   ├─→ US2 (P2) - Routing       [Depends on US1]
    │   │       ↓
    │   │   US3 (P3) - Parallel      [Depends on US1 + US2]
    │   │
    │   └─→ US4 (P4) - LLM           [Depends on US1 only]
    │       │
    │   └─→ US5 (P5) - Events        [Depends on US1 only, enhances all]
    │
    └─→ Phase 8 (Polish)             [Depends on completed stories]
```

### Parallel Execution Opportunities

**Within Phase 2 (Foundational)**:
- T012-T021 (Node types) || T022-T026 (State types) || T027-T028 (Store) || T029-T032 (Events)

**Within US1**:
- T033-T042 (MemStore) can run while T043-T054 (Engine construction) runs
- T071-T074 (Serialization) can run in parallel with core engine

**Within US2**:
- T078-T081 (Predicates) || T082-T085 (Routing) || T094-T097 (Termination)

**Within US3**:
- State copy logic is independent of parallel execution logic

**Within US4**:
- T134-T138 (OpenAI) || T139-T143 (Anthropic) || T144-T148 (Google) - all adapters in parallel

**Within US5**:
- T160-T163 (LogEmitter) || T164-T165 (NullEmitter) in parallel

**Phase 8 (Polish)**:
- T184-T189 (Docs) || T190-T193 (SQL) || T194-T197 (Otel) || T198-T201 (Benchmarks)

---

## Independent Test Criteria Per Story

**US1 (Checkpointing)**:
- ✅ Create 3-node workflow
- ✅ Execute through step 1, save checkpoint
- ✅ Kill process (os.Exit or panic simulation)
- ✅ Restart, load checkpoint
- ✅ Resume and complete workflow
- ✅ Verify state continuity across resume

**US2 (Routing)**:
- ✅ Create workflow with conditional judge node
- ✅ Test state.confidence = 0.9 → routes to finalize
- ✅ Test state.confidence = 0.6 → loops to refine
- ✅ Test no matching predicate → error raised
- ✅ Verify MaxSteps prevents infinite loops

**US3 (Parallel)**:
- ✅ Create workflow with 3 parallel branches (1s each)
- ✅ Measure total execution time ~1s (not 3s)
- ✅ Verify state contains results from all branches
- ✅ Test error in one branch → captured in merged state
- ✅ Verify deterministic merge order

**US4 (LLM Integration)**:
- ✅ Create workflow calling OpenAI node
- ✅ Add Anthropic node in sequence
- ✅ Add Google node in sequence
- ✅ Verify state accumulates all responses
- ✅ Test provider swap (change from OpenAI → Anthropic)
- ✅ Verify zero code changes in workflow logic

**US5 (Events)**:
- ✅ Create 10-node workflow with varied transitions
- ✅ Execute with LogEmitter
- ✅ Export execution trace
- ✅ Verify all node_start/node_end events present
- ✅ Verify routing_decision events capture paths
- ✅ Verify state deltas recorded in events

---

## Task Statistics

**Total Tasks**: 201
**Test Tasks**: 101 (TDD: every implementation has tests)
**Implementation Tasks**: 100

**By User Story**:
- Setup (Phase 1): 11 tasks
- Foundational (Phase 2): 21 tasks
- US1 (Checkpointing): 45 tasks
- US2 (Routing): 23 tasks
- US3 (Parallel): 19 tasks
- US4 (LLM Integration): 32 tasks
- US5 (Events): 24 tasks
- Polish (Phase 8): 26 tasks

**Parallel Opportunities**: 47 tasks marked [P] can run concurrently

**MVP Scope** (US1 only): 77 tasks (Setup + Foundational + US1)

---

**Next Step**: Begin with Phase 1 (Setup) tasks T001-T011, then Phase 2 (Foundational) tasks T012-T032 before starting any user story implementation.
