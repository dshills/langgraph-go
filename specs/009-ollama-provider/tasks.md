# Tasks: Ollama Model Provider

**Input**: Design documents from `/specs/009-ollama-provider/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md
**Branch**: `009-ollama-provider`

**Tests**: Following TDD constitution requirement - write tests FIRST, ensure they FAIL, then implement until tests pass.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

**User Context**: Use `gpt-oss` model for testing (per user request)

## Format: `- [ ] [ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

All code in `graph/model/ollama/` package, following existing adapter pattern.
Example code in `examples/ollama/` directory.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and package structure

- [X] T001 Create package directory `graph/model/ollama/` following existing adapter structure
- [X] T002 Add `github.com/ollama/ollama/api` dependency to `go.mod`
- [X] T003 [P] Create package documentation file `graph/model/ollama/doc.go` with package overview

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and infrastructure that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Create `Config` struct in `graph/model/ollama/config.go` with all fields (Endpoint, Model, Temperature, TopP, Seed, NumPredict, HTTPClient)
- [X] T005 [P] Implement `validateConfig()` function in `graph/model/ollama/config.go` with validation rules
- [X] T006 [P] Create `OllamaError` struct in `graph/model/ollama/errors.go` with Code, Message, Err fields
- [X] T007 [P] Implement `Error()` and `Unwrap()` methods for `OllamaError` in `graph/model/ollama/errors.go`
- [X] T008 [P] Create error translation helper functions in `graph/model/ollama/errors.go` (connection, model_not_found, invalid_request, timeout, unknown)
- [X] T009 [P] Create internal schema types in `graph/model/ollama/schema.go` for request/response translation

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Local Model Execution (Priority: P1) 🎯 MVP

**Goal**: Enable developers to run LLM workflows using local Ollama instance at default endpoint (`http://localhost:11434`)

**Independent Test**: Install Ollama locally, pull `gpt-oss` model, create adapter with default config, execute simple chat workflow, verify response

### Tests for User Story 1 (TDD - Write First)

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T010 [P] [US1] Create test file `graph/model/ollama/config_test.go` with table-driven tests for Config validation
- [X] T011 [P] [US1] Write tests for default endpoint (`http://localhost:11434`) in `graph/model/ollama/config_test.go`
- [X] T012 [P] [US1] Write tests for required Model field validation in `graph/model/ollama/config_test.go`
- [X] T013 [P] [US1] Create test file `graph/model/ollama/ollama_test.go` with mock HTTP server setup
- [X] T014 [P] [US1] Write test for `NewChatModel()` constructor with valid config in `graph/model/ollama/ollama_test.go`
- [X] T015 [P] [US1] Write test for `NewChatModel()` constructor with invalid config (empty Model) in `graph/model/ollama/ollama_test.go`
- [X] T016 [P] [US1] Write test for successful Chat() call with mock Ollama response in `graph/model/ollama/ollama_test.go`
- [X] T017 [P] [US1] Write test for message translation (model.Message → api.Message) in `graph/model/ollama/ollama_test.go`
- [X] T018 [P] [US1] Write test for response parsing (api.ChatResponse → model.ChatOut) in `graph/model/ollama/ollama_test.go`
- [X] T019 [P] [US1] Write test for metadata extraction in ChatOut.Meta in `graph/model/ollama/ollama_test.go`

### Implementation for User Story 1

- [X] T020 [US1] Implement `NewChatModel()` constructor in `graph/model/ollama/ollama.go` with config validation and defaults
- [X] T021 [US1] Create `ChatModel` struct in `graph/model/ollama/ollama.go` with client and config fields
- [X] T022 [US1] Implement message translation function `toOllamaMessages()` in `graph/model/ollama/ollama.go`
- [X] T023 [US1] Implement response translation function `toLangGraphOutput()` in `graph/model/ollama/ollama.go`
- [X] T024 [US1] Implement `Chat()` method in `graph/model/ollama/ollama.go` - basic request/response flow
- [X] T025 [US1] Add metadata extraction (model, tokens, duration) in `Chat()` method in `graph/model/ollama/ollama.go`
- [X] T026 [US1] Add context cancellation check in `Chat()` method in `graph/model/ollama/ollama.go`
- [X] T027 [US1] Run tests with `go test ./graph/model/ollama/...` and verify all US1 tests pass

**Checkpoint**: At this point, basic local Ollama execution should work. Users can create adapter with default config and send messages to `gpt-oss` model.

---

## Phase 4: User Story 2 - Remote Instance Support (Priority: P2)

**Goal**: Enable connection to remote Ollama instances with custom endpoints

**Independent Test**: Deploy Ollama to Docker container, configure adapter with custom endpoint (`http://localhost:11434` in container), verify connectivity

### Tests for User Story 2 (TDD - Write First)

- [ ] T028 [P] [US2] Write test for custom endpoint URL in `graph/model/ollama/config_test.go`
- [ ] T029 [P] [US2] Write test for endpoint URL validation (valid/invalid URLs) in `graph/model/ollama/config_test.go`
- [ ] T030 [P] [US2] Write test for connection error handling in `graph/model/ollama/ollama_test.go` (mock unreachable endpoint)
- [ ] T031 [P] [US2] Write test for network timeout error handling in `graph/model/ollama/ollama_test.go`
- [ ] T032 [P] [US2] Write test for custom HTTPClient configuration in `graph/model/ollama/config_test.go`

### Implementation for User Story 2

- [ ] T033 [US2] Update `validateConfig()` to validate endpoint URL format in `graph/model/ollama/config.go`
- [ ] T034 [US2] Update `NewChatModel()` to use custom endpoint from config in `graph/model/ollama/ollama.go`
- [ ] T035 [US2] Add connection error translation in `Chat()` method in `graph/model/ollama/ollama.go`
- [ ] T036 [US2] Add actionable error messages (e.g., "Ensure Ollama is running with: ollama serve") in `graph/model/ollama/errors.go`
- [ ] T037 [US2] Run tests with `go test ./graph/model/ollama/...` and verify all US2 tests pass

**Checkpoint**: At this point, both local AND remote Ollama instances should work independently

---

## Phase 5: User Story 3 - Model Selection and Configuration (Priority: P3)

**Goal**: Enable model selection and parameter configuration (temperature, top-p, seed, num-predict)

**Independent Test**: Configure adapter with different models (`gpt-oss`, `llama3.2`) and parameters (seed=42 for deterministic), compare outputs

### Tests for User Story 3 (TDD - Write First)

- [ ] T038 [P] [US3] Write test for Temperature validation (must be in [0.0, 2.0]) in `graph/model/ollama/config_test.go`
- [ ] T039 [P] [US3] Write test for TopP validation (must be in [0.0, 1.0]) in `graph/model/ollama/config_test.go`
- [ ] T040 [P] [US3] Write test for NumPredict validation (must be >= -1) in `graph/model/ollama/config_test.go`
- [ ] T041 [P] [US3] Write test for Seed parameter (deterministic generation) in `graph/model/ollama/ollama_test.go`
- [ ] T042 [P] [US3] Write test for parameter transmission to Ollama API in `graph/model/ollama/ollama_test.go`
- [ ] T043 [P] [US3] Write test for model name in request in `graph/model/ollama/ollama_test.go`

### Implementation for User Story 3

- [ ] T044 [US3] Update `validateConfig()` with parameter range validation in `graph/model/ollama/config.go`
- [ ] T045 [US3] Add parameter mapping (Temperature, TopP, Seed, NumPredict) to Ollama API request in `graph/model/ollama/ollama.go`
- [ ] T046 [US3] Implement default parameter values in `NewChatModel()` (Temperature: 0.8, TopP: 0.9, NumPredict: -1) in `graph/model/ollama/ollama.go`
- [ ] T047 [US3] Add invalid parameter error messages in `graph/model/ollama/errors.go`
- [ ] T048 [US3] Run tests with `go test ./graph/model/ollama/...` and verify all US3 tests pass

**Checkpoint**: All parameter configuration should work, users can fine-tune model behavior

---

## Phase 6: User Story 4 - Tool Calling Support (Priority: P4)

**Goal**: Enable tool/function calling for agentic workflows with compatible models

**Independent Test**: Configure adapter with tool-capable model (`gpt-oss` with tools), provide tool specifications, execute chat, verify tool calls are parsed correctly

### Tests for User Story 4 (TDD - Write First)

- [ ] T049 [P] [US4] Write test for tool spec translation (model.ToolSpec → api.Tool) in `graph/model/ollama/ollama_test.go`
- [ ] T050 [P] [US4] Write test for tool call parsing (api.ToolCall → model.ToolCall) in `graph/model/ollama/ollama_test.go`
- [ ] T051 [P] [US4] Write test for JSON schema pass-through in tool definitions in `graph/model/ollama/ollama_test.go`
- [ ] T052 [P] [US4] Write test for multiple tool calls in single response in `graph/model/ollama/ollama_test.go`
- [ ] T053 [P] [US4] Write test for tool call with invalid JSON arguments (error handling) in `graph/model/ollama/ollama_test.go`
- [ ] T054 [P] [US4] Write test for model without tool support (graceful degradation) in `graph/model/ollama/ollama_test.go`

### Implementation for User Story 4

- [X] T055 [US4] Implement tool spec translation function `toOllamaTools()` in `graph/model/ollama/ollama.go`
- [X] T056 [US4] Implement tool call parsing function `fromOllamaToolCalls()` in `graph/model/ollama/ollama.go`
- [X] T057 [US4] Update `Chat()` method to include tools in request if provided in `graph/model/ollama/ollama.go`
- [X] T058 [US4] Update response parsing to extract tool calls from api.ChatResponse in `graph/model/ollama/ollama.go`
- [X] T059 [US4] Add tool-related error messages (e.g., "Model does not support tools") in `graph/model/ollama/errors.go`
- [X] T060 [US4] Run tests with `go test ./graph/model/ollama/...` and verify all US4 tests pass

**Checkpoint**: All user stories (US1-US4) should now be independently functional

---

## Phase 7: Error Handling & Edge Cases

**Purpose**: Comprehensive error handling for all edge cases from spec.md

- [ ] T061 [P] Write test for model not found error ("model not pulled") in `graph/model/ollama/errors_test.go`
- [ ] T062 [P] Write test for Ollama restart during execution in `graph/model/ollama/ollama_test.go`
- [ ] T063 [P] Write test for context timeout shorter than generation time in `graph/model/ollama/ollama_test.go`
- [ ] T064 [P] Write test for malformed JSON response from Ollama in `graph/model/ollama/ollama_test.go`
- [ ] T065 [P] Write test for Unicode and special characters in prompts/responses in `graph/model/ollama/ollama_test.go`
- [ ] T066 Implement model not found error translation in `graph/model/ollama/errors.go`
- [ ] T067 Implement malformed response error handling in `graph/model/ollama/ollama.go`
- [ ] T068 Add timeout error translation in `graph/model/ollama/errors.go`
- [ ] T069 Run error handling tests with `go test ./graph/model/ollama/...`

---

## Phase 8: Example & Documentation

**Purpose**: User-facing example and documentation

- [X] T070 [P] Create example directory `examples/ollama/`
- [X] T071 [P] Create example main.go in `examples/ollama/main.go` with basic local usage
- [X] T072 [P] Add remote instance example to `examples/ollama/main.go`
- [X] T073 [P] Add tool calling example to `examples/ollama/main.go`
- [X] T074 [P] Add deterministic generation example (with seed) to `examples/ollama/main.go`
- [X] T075 [P] Add comments and usage instructions to `examples/ollama/main.go`
- [ ] T076 [P] Write `ExampleChatModel_Chat` in `graph/model/ollama/ollama_test.go` for godoc
- [ ] T077 [P] Write `ExampleNewChatModel` in `graph/model/ollama/ollama_test.go` for godoc
- [ ] T078 Run example with `go run examples/ollama/main.go` (requires local Ollama with `gpt-oss` model)

---

## Phase 9: Integration & Polish

**Purpose**: Integration tests and final polish

- [ ] T079 [P] Create integration test file `graph/model/ollama/integration_test.go` with build tag `//go:build integration`
- [ ] T080 [P] Write integration test for real Ollama instance with `gpt-oss` model in `graph/model/ollama/integration_test.go`
- [ ] T081 [P] Write integration test for tool calling with real Ollama in `graph/model/ollama/integration_test.go`
- [ ] T082 [P] Document integration test requirements (Ollama running, gpt-oss pulled) in `graph/model/ollama/integration_test.go`
- [ ] T083 Update CLAUDE.md with Ollama provider documentation (add to "LLM Integration" section)
- [ ] T084 Update model provider capability matrix in `graph/model/chat.go` with Ollama entry
- [ ] T085 Run full test suite: `go test ./...` and verify all tests pass
- [ ] T086 Run linter: `golangci-lint run ./graph/model/ollama/` and fix issues
- [ ] T087 Run `gofmt -s -w ./graph/model/ollama/` to format code
- [ ] T088 Create PR description summarizing implementation

---

## Phase 10: Code Review & Commit

**Purpose**: Pre-commit review and final verification

- [ ] T089 Stage changes: `git add graph/model/ollama/ examples/ollama/ CLAUDE.md`
- [ ] T090 Run pre-commit review: `mcp-pr review_staged` and address all issues
- [ ] T091 Verify constitution compliance (TDD, type safety, interfaces, observability, dependencies)
- [ ] T092 Run final test suite: `go test -race ./...` (with race detector)
- [ ] T093 Verify example runs successfully: `go run examples/ollama/main.go`
- [ ] T094 Commit changes with message: "feat: add Ollama model provider adapter (009)"
- [ ] T095 Create PR with generated description from T088

---

## Dependencies & Parallel Execution

### User Story Dependencies

```
Phase 1 (Setup) → Phase 2 (Foundation) → Phase 3 (US1) ⊥ Phase 4 (US2) ⊥ Phase 5 (US3) ⊥ Phase 6 (US4)
                                            ↓              ↓              ↓              ↓
                                            └──────────────┴──────────────┴──────────────┘
                                                                   ↓
                                                    Phase 7 (Error Handling)
                                                                   ↓
                                                    Phase 8 (Examples) ⊥ Phase 9 (Integration)
                                                                   ↓
                                                    Phase 10 (Review & Commit)
```

**Key**: `→` = sequential, `⊥` = can run in parallel

### Parallel Execution Opportunities

**After Phase 2 (Foundation) completes, these can run in parallel**:

**Stream A (US1 - MVP)**:
- T010-T027: User Story 1 (Local execution)

**Stream B (US2 - Remote)**:
- T028-T037: User Story 2 (Remote instances)

**Stream C (US3 - Config)**:
- T038-T048: User Story 3 (Model configuration)

**Stream D (US4 - Tools)**:
- T049-T060: User Story 4 (Tool calling)

**After all user stories complete**:

**Stream E (Examples)**:
- T070-T078: Example code

**Stream F (Integration)**:
- T079-T082: Integration tests

**Stream G (Docs)**:
- T083-T084: Documentation updates

### Recommended MVP Scope

**Minimum Viable Product (User Story 1 only)**:
- Phase 1: Setup (T001-T003)
- Phase 2: Foundation (T004-T009)
- Phase 3: User Story 1 (T010-T027)
- Phase 8: Example (T070-T071 - basic example only)
- Phase 10: Review & Commit (T089-T095)

**Total MVP Tasks**: ~35 tasks
**Estimated Time**: 4-6 hours (with TDD)

### Implementation Strategy

1. **Start with MVP** (US1 only): Delivers core value (local Ollama execution)
2. **Add US2-US4 incrementally**: Each story adds independent value
3. **Test continuously**: Every task has corresponding tests (TDD)
4. **Parallelize**: After foundation, US2-US4 can be developed in parallel
5. **Polish last**: Error handling, examples, docs after core functionality works

---

## Task Summary

**Total Tasks**: 95
- **Phase 1 (Setup)**: 3 tasks
- **Phase 2 (Foundation)**: 6 tasks (blocking)
- **Phase 3 (US1 - MVP)**: 18 tasks (10 tests + 8 implementation)
- **Phase 4 (US2)**: 10 tasks (5 tests + 5 implementation)
- **Phase 5 (US3)**: 11 tasks (6 tests + 5 implementation)
- **Phase 6 (US4)**: 12 tasks (6 tests + 6 implementation)
- **Phase 7 (Error Handling)**: 9 tasks
- **Phase 8 (Examples)**: 9 tasks
- **Phase 9 (Integration)**: 10 tasks
- **Phase 10 (Review)**: 7 tasks

**MVP Tasks** (US1 only): 35 tasks
**Full Feature Tasks**: 95 tasks

**Parallel Opportunities**: After Phase 2, US2-US4 can run in 3 parallel streams (30 tasks total)

**Independent Test Criteria**:
- **US1**: Adapter connects to local Ollama, sends message to `gpt-oss`, receives response
- **US2**: Adapter connects to remote Ollama endpoint, handles connection errors gracefully
- **US3**: Adapter respects all configuration parameters (temperature, seed, etc.)
- **US4**: Adapter translates tool specs, parses tool calls from compatible models

**Format Validation**: ✅ All tasks follow checklist format `- [ ] [ID] [P?] [Story] Description with file path`

---

## Next Steps

To execute tasks:
1. Run `/speckit.implement` to begin TDD implementation workflow
2. Follow task order (Setup → Foundation → User Stories in priority order)
3. Write tests FIRST for each task (marked with test comments)
4. Implement until tests pass (Red-Green-Refactor)
5. Commit after each completed phase
