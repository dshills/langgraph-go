# Tasks: AWS Bedrock LLM Provider Support

**Input**: Design documents from `/specs/008-bedrock-llm-support/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Per TDD constitution requirement, all tests are written BEFORE implementation (red-green-refactor cycle).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

## Path Conventions

Single library project structure:
- Core adapter: `graph/model/bedrock/`
- Examples: `examples/bedrock_quickstart/`
- Tests collocated with implementation (`_test.go` files)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and dependency setup

- [X] T001 Create bedrock package directory structure at graph/model/bedrock/
- [X] T002 Add AWS SDK v2 dependencies to go.mod (bedrockruntime, config packages)
- [X] T003 [P] Run go mod tidy to resolve dependencies
- [X] T004 [P] Index codebase with gocontext for adapter pattern research

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Research existing ChatModel adapters using gocontext search for "ChatModel implementation patterns"
- [X] T006 [P] Document BedrockConfig struct fields in graph/model/bedrock/config.go (stub only, no implementation)
- [X] T007 [P] Document BedrockAdapter struct fields in graph/model/bedrock/bedrock.go (stub only, no implementation)
- [X] T008 [P] Define SchemaTranslator interface in graph/model/bedrock/schema.go
- [X] T009 [P] Define BedrockError type in graph/model/bedrock/errors.go
- [X] T010 Define ModelFamily enum constants in graph/model/bedrock/schema.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Basic Bedrock Model Integration (Priority: P1) 🎯 MVP

**Goal**: Enable developers to configure a Bedrock adapter for Claude models and send basic chat requests with AWS credentials

**Independent Test**: Create a Bedrock adapter with valid AWS credentials, send a simple chat request to Claude, verify successful response

### Tests for User Story 1 (TDD - Write FIRST) ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T011 [P] [US1] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (region validation)
- [X] T012 [P] [US1] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (modelID format validation)
- [X] T013 [P] [US1] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (temperature range validation)
- [X] T014 [P] [US1] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (maxTokens validation)
- [X] T015 [P] [US1] Write failing test for NewAdapter() credential initialization in graph/model/bedrock/bedrock_test.go
- [X] T016 [P] [US1] Write failing test for detectModelFamily() for Claude model IDs in graph/model/bedrock/schema_test.go
- [X] T017 [US1] Write failing table-driven test for ClaudeSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema_test.go (basic message translation)
- [X] T018 [US1] Write failing table-driven test for ClaudeSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema_test.go (system message extraction)
- [X] T019 [US1] Write failing table-driven test for ClaudeSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema_test.go (text content parsing)
- [X] T020 [US1] Write failing table-driven test for ClaudeSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema_test.go (metadata extraction)
- [X] T021 [US1] Write failing test for BedrockAdapter.Chat() with Claude in graph/model/bedrock/bedrock_test.go (mock AWS client)
- [X] T022 [US1] Write failing test for error wrapping (BedrockError) in graph/model/bedrock/errors_test.go

### Implementation for User Story 1

- [X] T023 [US1] Implement BedrockConfig.Validate() method in graph/model/bedrock/config.go (region, modelID, temperature, maxTokens validation)
- [X] T024 [US1] Implement NewAdapter() function in graph/model/bedrock/bedrock.go (AWS client initialization, credential loading)
- [X] T025 [US1] Implement detectModelFamily() function in graph/model/bedrock/schema.go (ModelID prefix matching for Claude/Llama/Titan/Mistral)
- [X] T026 [US1] Implement ClaudeSchemaTranslator struct in graph/model/bedrock/schema.go
- [X] T027 [US1] Implement ClaudeSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema.go (Messages API format, system message extraction, config parameters)
- [X] T028 [US1] Implement ClaudeSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema.go (content[].text parsing, metadata population)
- [X] T029 [US1] Implement ClaudeSchemaTranslator.SupportsStreaming() and SupportsTools() in graph/model/bedrock/schema.go
- [X] T030 [US1] Implement BedrockAdapter.Chat() method in graph/model/bedrock/bedrock.go (InvokeModel API call, schema translation, error handling)
- [X] T031 [US1] Implement wrapAWSError() function in graph/model/bedrock/errors.go (map AWS SDK errors to BedrockError with retry classification)
- [X] T032 [US1] Implement retry logic with exponential backoff in graph/model/bedrock/bedrock.go (MaxRetries, retryable error detection)
- [X] T033 [US1] Implement context cancellation handling in graph/model/bedrock/bedrock.go
- [X] T034 [US1] Verify all T011-T022 tests now PASS, refactor if needed

**Checkpoint**: At this point, User Story 1 should be fully functional - can create Bedrock adapter, send Claude requests, get responses

---

## Phase 4: User Story 2 - Multi-Region Support (Priority: P2)

**Goal**: Enable region-specific endpoint configuration for low latency and disaster recovery

**Independent Test**: Create Bedrock adapters for different regions (us-east-1, us-west-2, eu-west-1), invoke models, verify requests route to correct endpoints

### Tests for User Story 2 (TDD - Write FIRST) ⚠️

- [ ] T035 [P] [US2] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (region string validation)
- [ ] T036 [P] [US2] Write failing test for NewAdapter() with different regions in graph/model/bedrock/bedrock_test.go (endpoint configuration)
- [ ] T037 [US2] Write failing test for BedrockAdapter.Chat() verifying region in metadata in graph/model/bedrock/bedrock_test.go

### Implementation for User Story 2

- [ ] T038 [US2] Enhance BedrockConfig.Validate() in graph/model/bedrock/config.go (add AWS region validation against known regions list)
- [ ] T039 [US2] Update NewAdapter() in graph/model/bedrock/bedrock.go to configure AWS SDK client with specified region
- [ ] T040 [US2] Update BedrockAdapter.Chat() in graph/model/bedrock/bedrock.go to include region in ChatOut.Meta
- [ ] T041 [US2] Add custom endpoint URL support (EndpointURL field) in graph/model/bedrock/config.go and bedrock.go
- [ ] T042 [US2] Verify all T035-T037 tests now PASS

**Checkpoint**: User Story 1 AND 2 both work - single region (US1) and multi-region (US2) configurations functional

---

## Phase 5: User Story 3 - Cross-Region Fallback (Priority: P3)

**Goal**: Automatic fallback to secondary regions when primary region fails or throttles

**Independent Test**: Configure adapter with primary and fallback regions, simulate primary failure, verify automatic failover to backup region

### Tests for User Story 3 (TDD - Write FIRST) ⚠️

- [ ] T043 [P] [US3] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (FallbackRegions validation - no duplicates)
- [ ] T044 [P] [US3] Write failing test for BedrockConfig.Validate() in graph/model/bedrock/config_test.go (FallbackRegions don't include primary Region)
- [ ] T045 [US3] Write failing test for retry with fallback regions in graph/model/bedrock/bedrock_test.go (mock throttling error → retry in fallback region)
- [ ] T046 [US3] Write failing test for all regions exhausted in graph/model/bedrock/bedrock_test.go (return error after trying all fallback regions)

### Implementation for User Story 3

- [ ] T047 [US3] Enhance BedrockConfig.Validate() in graph/model/bedrock/config.go (add FallbackRegions validation - duplicates, primary region exclusion)
- [ ] T048 [US3] Implement regional retry logic in graph/model/bedrock/bedrock.go (retry in FallbackRegions[0], then [1], etc. on retryable errors)
- [ ] T049 [US3] Update wrapAWSError() in graph/model/bedrock/errors.go to include regional context
- [ ] T050 [US3] Add telemetry/logging for region fallback events in graph/model/bedrock/bedrock.go
- [ ] T051 [US3] Verify all T043-T046 tests now PASS

**Checkpoint**: All 3 stories work - basic (US1), multi-region (US2), and cross-region fallback (US3)

---

## Phase 6: User Story 4 - Streaming Response Support (Priority: P2)

**Goal**: Stream model responses token-by-token for real-time user feedback

**Independent Test**: Invoke Bedrock model with streaming enabled, collect tokens as they arrive, verify complete response assembly matches non-streaming output

### Tests for User Story 4 (TDD - Write FIRST) ⚠️

- [ ] T052 [P] [US4] Write failing test for ClaudeSchemaTranslator.TranslateStreamEvent() in graph/model/bedrock/streaming_test.go (message_start event)
- [ ] T053 [P] [US4] Write failing test for ClaudeSchemaTranslator.TranslateStreamEvent() in graph/model/bedrock/streaming_test.go (content_block_delta event)
- [ ] T054 [P] [US4] Write failing test for ClaudeSchemaTranslator.TranslateStreamEvent() in graph/model/bedrock/streaming_test.go (message_delta event with stop_reason)
- [ ] T055 [US4] Write failing test for BedrockAdapter.ChatStream() in graph/model/bedrock/streaming_test.go (full streaming lifecycle with callback)
- [ ] T056 [US4] Write failing test for streaming error handling in graph/model/bedrock/streaming_test.go (connection interruption mid-stream)

### Implementation for User Story 4

- [ ] T057 [US4] Create StreamChunk struct in graph/model/bedrock/streaming.go (Delta, ToolCallDelta, FinishReason, Metadata fields)
- [ ] T058 [US4] Implement ClaudeSchemaTranslator.TranslateStreamEvent() in graph/model/bedrock/streaming.go (handle 7 event types per streaming-event.json schema)
- [ ] T059 [US4] Implement BedrockAdapter.ChatStream() in graph/model/bedrock/streaming.go (InvokeModelWithResponseStream API, event channel processing)
- [ ] T060 [US4] Add streaming callback mechanism in graph/model/bedrock/streaming.go (token-by-token delivery via callback function)
- [ ] T061 [US4] Implement stream error handling in graph/model/bedrock/streaming.go (error events, partial response capture)
- [ ] T062 [US4] Add SupportsStreaming() check in BedrockAdapter.ChatStream() for model capabilities
- [ ] T063 [US4] Verify all T052-T056 tests now PASS

**Checkpoint**: Streaming support functional - US1 (basic chat), US2 (multi-region), US3 (fallback), US4 (streaming) all work

---

## Phase 7: User Story 5 - Tool/Function Calling Support (Priority: P2)

**Goal**: Enable Bedrock models (Claude) to invoke tools during generation for agentic workflows

**Independent Test**: Define a tool (e.g., get_weather), provide to Bedrock adapter, send prompt triggering tool usage, verify tool call request returned, feed back tool result, get final answer

### Tests for User Story 5 (TDD - Write FIRST) ⚠️

- [ ] T064 [P] [US5] Write failing test for ToolSpec translation in graph/model/bedrock/tools_test.go (LangGraph ToolSpec → Claude tool schema)
- [ ] T065 [P] [US5] Write failing test for tool_use content block parsing in graph/model/bedrock/tools_test.go (Claude response → ToolCall[])
- [ ] T066 [US5] Write failing test for tool result round-trip in graph/model/bedrock/tools_test.go (ToolCall → execute → feed back → final answer)
- [ ] T067 [US5] Write failing test for non-tool-capable models in graph/model/bedrock/tools_test.go (Llama/Titan with tools should handle gracefully)

### Implementation for User Story 5

- [ ] T068 [US5] Implement translateToolSpecs() function in graph/model/bedrock/tools.go (ToolSpec[] → Claude tools[] schema)
- [ ] T069 [US5] Update ClaudeSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema.go to include tools field
- [ ] T070 [US5] Implement parseToolUseCalls() function in graph/model/bedrock/tools.go (content[].tool_use → ToolCall[])
- [ ] T071 [US5] Update ClaudeSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema.go to populate ChatOut.ToolCalls
- [ ] T072 [US5] Implement tool result formatting in graph/model/bedrock/tools.go (ToolCall result → tool_result content block)
- [ ] T073 [US5] Add SupportsTools() check in BedrockAdapter.Chat() to validate tool usage by model family
- [ ] T074 [US5] Verify all T064-T067 tests now PASS

**Checkpoint**: All P1 and P2 stories complete - basic (US1), multi-region (US2), streaming (US4), and tools (US5) functional

---

## Phase 8: Additional Model Families (Lower Priority)

**Goal**: Extend support beyond Claude to Llama, Titan, and Mistral models

**Note**: These are optional enhancements, not blocking for MVP

### Tests (TDD - Write FIRST) ⚠️

- [ ] T075 [P] Write failing test for detectModelFamily() for Llama model IDs in graph/model/bedrock/schema_test.go
- [ ] T076 [P] Write failing test for detectModelFamily() for Titan model IDs in graph/model/bedrock/schema_test.go
- [ ] T077 [P] Write failing test for LlamaSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema_test.go (instruction template formatting)
- [ ] T078 [P] Write failing test for LlamaSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema_test.go (generation field parsing)
- [ ] T079 [P] Write failing test for TitanSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema_test.go (inputText + textGenerationConfig)
- [ ] T080 [P] Write failing test for TitanSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema_test.go (results[0].outputText parsing)

### Implementation

- [ ] T081 [P] Implement LlamaSchemaTranslator struct in graph/model/bedrock/schema.go
- [ ] T082 [P] Implement LlamaSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema.go (Llama instruction template tags)
- [ ] T083 [P] Implement LlamaSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema.go (generation field → ChatOut.Text)
- [ ] T084 [P] Implement TitanSchemaTranslator struct in graph/model/bedrock/schema.go
- [ ] T085 [P] Implement TitanSchemaTranslator.TranslateRequest() in graph/model/bedrock/schema.go (inputText + textGenerationConfig)
- [ ] T086 [P] Implement TitanSchemaTranslator.TranslateResponse() in graph/model/bedrock/schema.go (results[0].outputText → ChatOut.Text)
- [ ] T087 Update detectModelFamily() in graph/model/bedrock/schema.go to support Llama and Titan model ID patterns
- [ ] T088 Verify all T075-T080 tests now PASS

**Checkpoint**: Claude (US1), Llama, and Titan model families all supported

---

## Phase 9: Integration Tests & Examples

**Goal**: Validate end-to-end workflows with real AWS Bedrock API (requires AWS credentials)

### Integration Tests (build tag: integration)

- [ ] T089 [P] Write integration test for Claude basic chat in graph/model/bedrock/integration_test.go (requires AWS_ACCESS_KEY_ID env)
- [ ] T090 [P] Write integration test for multi-region configuration in graph/model/bedrock/integration_test.go (verify endpoint routing)
- [ ] T091 [P] Write integration test for streaming responses in graph/model/bedrock/integration_test.go (collect all chunks, verify complete response)
- [ ] T092 [P] Write integration test for tool calling round-trip in graph/model/bedrock/integration_test.go (get_weather example)
- [ ] T093 [P] Write integration test for error scenarios in graph/model/bedrock/integration_test.go (invalid credentials, model not found, throttling)

### Quickstart Examples

- [ ] T094 [P] Create examples/bedrock_quickstart/main.go (Example 1: Simple chat with Claude per quickstart.md)
- [ ] T095 [P] Create examples/bedrock_quickstart/workflow.go (Example 2: LangGraph workflow integration per quickstart.md)
- [ ] T096 [P] Create examples/bedrock_quickstart/streaming.go (Example 3: Streaming responses per quickstart.md)
- [ ] T097 [P] Create examples/bedrock_quickstart/tools.go (Example 4: Tool calling per quickstart.md)
- [ ] T098 [P] Create examples/bedrock_quickstart/multiregion.go (Example 5: Multi-region fallback per quickstart.md)
- [ ] T099 [P] Create examples/bedrock_quickstart/README.md (Setup instructions, AWS credentials, IAM permissions)

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Final improvements, documentation, and validation

- [ ] T100 [P] Add package-level godoc comments to graph/model/bedrock/bedrock.go
- [ ] T101 [P] Add godoc comments to all exported types and functions in graph/model/bedrock/ (Config, Adapter, SchemaTranslator, errors)
- [ ] T102 [P] Run gofmt on all files in graph/model/bedrock/
- [ ] T103 [P] Run golangci-lint on graph/model/bedrock/ and fix issues
- [ ] T104 [P] Run gosec security scanner on graph/model/bedrock/ (check for credential leaks, error message information disclosure)
- [ ] T105 Run all unit tests: go test ./graph/model/bedrock/
- [ ] T106 Run all integration tests with AWS credentials: go test -tags=integration ./graph/model/bedrock/
- [ ] T107 Validate quickstart examples: go run examples/bedrock_quickstart/main.go (requires AWS setup)
- [ ] T108 Run mcp-pr review_unstaged for pre-commit code review
- [ ] T109 Re-index codebase with gocontext after implementation
- [ ] T110 Update CLAUDE.md with Bedrock adapter usage examples if needed
- [ ] T111 Verify SC-001 through SC-007 success criteria from spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - US1 (P1) can proceed first - no dependencies on other stories
  - US2 (P2) can start in parallel after Foundational - minimal overlap with US1
  - US3 (P3) depends on US2 (extends multi-region with fallback logic)
  - US4 (P2) can start in parallel after Foundational - independent of US1-US3
  - US5 (P2) can start in parallel after Foundational - independent of US1-US4
- **Additional Models (Phase 8)**: Can start after US1 completes (reuses schema translator pattern)
- **Integration Tests (Phase 9)**: Depends on desired user stories being complete
- **Polish (Phase 10)**: Depends on all implementation phases complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Extends US1 config, independently testable
- **User Story 3 (P3)**: Depends on US2 (builds on multi-region support) - Not independently testable without US2
- **User Story 4 (P2)**: Can start after Foundational (Phase 2) - Independent of US1-US3
- **User Story 5 (P2)**: Can start after Foundational (Phase 2) - Independent of US1-US4

### Within Each User Story (TDD Cycle)

1. **Tests FIRST** (all test tasks marked with ⚠️)
2. **Verify tests FAIL** (red phase)
3. **Implement** (green phase - make tests pass)
4. **Refactor** (clean up with passing tests as safety net)
5. **Story complete** before moving to next priority

### Parallel Opportunities

- All Setup tasks (T001-T004) marked [P] can run in parallel
- All Foundational tasks (T005-T010) marked [P] can run in parallel
- Once Foundational completes:
  - Tests for US1 (T011-T022) can run in parallel (write all failing tests together)
  - Tests for US2 (T035-T037) can run in parallel
  - Tests for US4 (T052-T056) can run in parallel
  - Tests for US5 (T064-T067) can run in parallel
- After US1 implementation completes:
  - US2, US4, US5 can proceed in parallel (different concerns)
  - Phase 8 (additional models) can start
- Integration tests (T089-T093) can run in parallel
- Examples (T094-T099) can be created in parallel
- Polish tasks (T100-T104) marked [P] can run in parallel

---

## Parallel Example: User Story 1 Tests (TDD Red Phase)

```bash
# Launch all failing tests for US1 together (red phase):
Task: "Write failing test for BedrockConfig.Validate() region validation" (T011)
Task: "Write failing test for BedrockConfig.Validate() modelID validation" (T012)
Task: "Write failing test for BedrockConfig.Validate() temperature validation" (T013)
Task: "Write failing test for BedrockConfig.Validate() maxTokens validation" (T014)
# ... all T011-T022 can be written in parallel

# Verify all tests FAIL (expected - nothing implemented yet)

# Then implement T023-T034 sequentially (or with careful parallelization)

# Verify all tests PASS (green phase)
```

---

## Parallel Example: Multiple User Stories After Foundational

```bash
# After Phase 2 (Foundational) completes, these can proceed in parallel:

# Developer A: User Story 1 (P1 - MVP)
Task: T011-T034 (Claude basic integration)

# Developer B: User Story 4 (P2 - Streaming, independent)
Task: T052-T063 (Streaming support)

# Developer C: User Story 5 (P2 - Tools, independent)
Task: T064-T074 (Tool calling)

# Each story completes and integrates independently
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T010) - **CRITICAL BLOCKER**
3. Complete Phase 3: User Story 1 (T011-T034)
4. **STOP and VALIDATE**:
   - Run all US1 tests: `go test ./graph/model/bedrock/ -run "TestBedrockConfig|TestNewAdapter|TestClaudeSchema|TestBedrockAdapter"`
   - Test independently: Create adapter, send Claude request, verify response
   - Verify SC-001: Developers can configure and execute chat in under 5 minutes
5. **Deploy/demo if ready** (US1 delivers core value)

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → **Deploy/Demo (MVP!)**
   - Deliverable: Basic Claude integration via Bedrock
3. Add User Story 2 → Test independently → Deploy/Demo
   - Deliverable: Multi-region support
4. Add User Story 4 → Test independently → Deploy/Demo
   - Deliverable: Streaming responses
5. Add User Story 5 → Test independently → Deploy/Demo
   - Deliverable: Tool calling for agentic workflows
6. Add User Story 3 → Test independently → Deploy/Demo
   - Deliverable: Cross-region fallback
7. Add Phase 8 (Llama/Titan) → Test independently → Deploy/Demo
   - Deliverable: Additional model family support
8. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001-T010)
2. Once Foundational is done (T010 complete):
   - **Developer A**: User Story 1 (T011-T034) - MVP priority
   - **Developer B**: User Story 4 (T052-T063) - Streaming (parallel)
   - **Developer C**: User Story 5 (T064-T074) - Tools (parallel)
3. After US1 completes:
   - **Developer A**: User Story 2 (T035-T042) - Multi-region
4. After US2 completes:
   - **Developer A**: User Story 3 (T043-T051) - Fallback (depends on US2)
5. After US1-US5 complete:
   - **Team**: Phase 8 (T075-T088) - Additional models (parallel by family)
6. Stories complete and integrate independently

---

## Notes

- **[P] tasks** = different files, no dependencies - can run in parallel
- **[Story] label** = maps task to specific user story (US1-US5) for traceability
- **TDD is NON-NEGOTIABLE** per constitution - all test tasks marked with ⚠️ MUST be written and FAIL before implementation
- **Test naming convention**: `TestFunctionName` for specific function tests, `TestFeatureName` for integration tests
- **Integration tests**: Require AWS credentials via environment variables, marked with build tag `//go:build integration`
- Each user story should be independently completable and testable (except US3 depends on US2)
- **Verify tests fail** before implementing (red phase)
- **Make tests pass** with minimal code (green phase)
- **Refactor** with passing tests as safety net (refactor phase)
- **Commit after each phase** or logical group of tasks
- **Stop at any checkpoint** to validate story independently
- **Avoid**: vague tasks, same file conflicts, cross-story dependencies that break independence
- **Code review**: Run `mcp-pr review_unstaged` before commits per constitution
- **Codebase indexing**: Use gocontext before and after implementation for adapter pattern research

---

## Task Summary

**Total Tasks**: 111
**Task Count by Phase**:
- Phase 1 (Setup): 4 tasks
- Phase 2 (Foundational): 6 tasks
- Phase 3 (US1 - MVP): 24 tasks (12 tests + 12 implementation)
- Phase 4 (US2): 8 tasks (3 tests + 5 implementation)
- Phase 5 (US3): 9 tasks (4 tests + 5 implementation)
- Phase 6 (US4): 12 tasks (5 tests + 7 implementation)
- Phase 7 (US5): 11 tasks (4 tests + 7 implementation)
- Phase 8 (Additional Models): 14 tasks (6 tests + 8 implementation)
- Phase 9 (Integration & Examples): 11 tasks
- Phase 10 (Polish): 12 tasks

**Parallel Opportunities Identified**:
- Setup: 3/4 tasks parallelizable
- Foundational: 5/6 tasks parallelizable
- US1 tests: 12/12 parallelizable (write all failing tests together)
- US2 tests: 3/3 parallelizable
- US4 tests: 5/5 parallelizable
- US5 tests: 4/4 parallelizable
- Additional model tests: 6/6 parallelizable
- Integration tests: 5/5 parallelizable
- Examples: 6/6 parallelizable
- Polish: 5/12 parallelizable

**Independent Test Criteria by Story**:
- **US1**: Create adapter → send Claude chat → verify response (5 min setup time)
- **US2**: Configure us-west-2 → verify request routes to us-west-2 endpoint
- **US3**: Configure fallback regions → simulate primary failure → verify failover
- **US4**: Enable streaming → collect tokens → verify complete response assembly
- **US5**: Define tool → trigger tool use → execute → feed back → verify final answer

**Suggested MVP Scope**: Phase 1 + Phase 2 + Phase 3 (User Story 1 only)
- **Delivers**: Basic Bedrock integration with Claude models
- **Task count**: 34 tasks (10 setup/foundational + 24 US1)
- **Success criteria**: Developers can configure adapter and execute Claude chat in under 5 minutes
