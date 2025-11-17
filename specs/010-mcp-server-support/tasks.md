# Tasks: MCP Server Support

**Input**: Design documents from `/specs/010-mcp-server-support/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Tests are included per constitution TDD requirement

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- Go project: `graph/mcp/` for implementation, `graph/mcp/transport/` for transport layer
- Examples: `examples/mcp_server/` for working examples
- Tests: `graph/mcp/*_test.go` for unit tests, `tests/integration/mcp_integration_test.go` for integration tests

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure for MCP server package

- [x] T001 Create `graph/mcp/` package directory structure per plan.md
- [x] T002 Create `graph/mcp/transport/` subdirectory for transport implementations
- [x] T003 [P] Install dependency: `go get github.com/sourcegraph/jsonrpc2@v0.2.1`
- [x] T004 [P] Create `examples/mcp_server/` directory for working examples

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core MCP protocol infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Define MCP protocol message types in `graph/mcp/protocol.go` (JSON-RPC 2.0 Request, Response, Error structs)
- [x] T006 [P] Write tests for protocol message serialization in `graph/mcp/protocol_test.go`
- [x] T007 Implement stdio transport wrapper in `graph/mcp/transport/stdio.go` (StdioReadWriteCloser)
- [x] T008 [P] Write tests for stdio transport in `graph/mcp/transport/stdio_test.go`
- [x] T009 Implement JSON-RPC connection adapter in `graph/mcp/transport/jsonrpc.go` (MCPStdioServer wrapper)
- [x] T010 [P] Write tests for JSON-RPC adapter in `graph/mcp/transport/jsonrpc_test.go`
- [x] T011 Define MCPServer interface in `graph/mcp/server.go` (Start, Stop, RegisterTool, RegisterResource, RegisterPrompt methods)
- [x] T012 Implement ServerConfig struct in `graph/mcp/server.go` (Name, Version, Emitter fields)
- [x] T013 Implement MCPServer lifecycle (Uninitialized → Initializing → Running → Stopped states) in `graph/mcp/server.go`
- [x] T014 [P] Write tests for server lifecycle in `graph/mcp/server_test.go`
- [x] T015 Implement `initialize` method handler in `graph/mcp/server.go` (capability negotiation)
- [x] T016 [P] Write tests for initialize handler in `graph/mcp/server_test.go`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Expose Workflow Tools via MCP (Priority: P1) 🎯 MVP

**Goal**: Enable developers to expose LangGraph tools to MCP clients, allowing tool discovery and invocation

**Independent Test**: Create a simple weather tool, start MCP server, connect client, list tools, invoke tool, verify result

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [P] [US1] Contract test for `tools/list` method in `graph/mcp/tool_adapter_test.go` (verify response schema)
- [x] T018 [P] [US1] Contract test for `tools/call` method in `graph/mcp/tool_adapter_test.go` (verify invocation flow)
- [x] T019 [P] [US1] Integration test for tool registration and discovery in `graph/mcp/server_test.go`
- [x] T020 [P] [US1] Integration test for tool invocation with mock Tool in `graph/mcp/server_test.go`
- [x] T021 [P] [US1] Integration test for tool error handling in `graph/mcp/server_test.go`

### Implementation for User Story 1

- [x] T022 [P] [US1] Define ToolMetadata struct in `graph/mcp/tool_adapter.go` (name, description, inputSchema)
- [x] T023 [P] [US1] Define RegisteredTool struct in `graph/mcp/tool_adapter.go` (tool, metadata)
- [x] T024 [P] [US1] Define ToolRegistry struct in `graph/mcp/tool_adapter.go` (tools map, sync.RWMutex)
- [x] T025 [US1] Implement ToolRegistry.Register method in `graph/mcp/tool_adapter.go` (validate name pattern, schema)
- [x] T026 [US1] Implement ToolRegistry.Get method in `graph/mcp/tool_adapter.go` (retrieve tool by name)
- [x] T027 [US1] Implement ToolRegistry.List method in `graph/mcp/tool_adapter.go` (return all tool metadata)
- [x] T028 [US1] Implement ToolRegistry.Invoke method in `graph/mcp/tool_adapter.go` (validate input, call tool, handle errors)
- [x] T029 [US1] Implement `tools/list` handler in `graph/mcp/server.go` (integrate ToolRegistry.List)
- [x] T030 [US1] Implement `tools/call` handler in `graph/mcp/server.go` (integrate ToolRegistry.Invoke)
- [x] T031 [US1] Add MCPServer.RegisterTool method in `graph/mcp/server.go` (public API for tool registration)
- [x] T032 [US1] Add input validation using JSON Schema in `graph/mcp/tool_adapter.go` (validate tool params before invocation)
- [x] T033 [US1] Add error mapping to JSON-RPC error codes in `graph/mcp/tool_adapter.go` (map tool errors to -32603, validation to -32602)
- [x] T034 [US1] Add observability events for tool operations in `graph/mcp/server.go` (emit tool_call_start, tool_call_end via Emitter)
- [x] T035 [P] [US1] Write unit tests for ToolRegistry operations in `graph/mcp/tool_adapter_test.go`
- [x] T036 [P] [US1] Create example weather tool server in `examples/mcp_server/weather_server.go`
- [x] T037 [P] [US1] Add README for weather example in `examples/mcp_server/README.md` (how to run with Claude Desktop)

**Checkpoint**: At this point, User Story 1 should be fully functional - tools can be registered, discovered, and invoked via MCP

---

## Phase 4: User Story 2 - Share Workflow State as Resources (Priority: P2)

**Goal**: Enable developers to expose workflow state, checkpoints, and metrics as read-only MCP resources

**Independent Test**: Register workflow state as resource, connect client, list resources, read resource content, verify no state mutation

### Tests for User Story 2

- [ ] T038 [P] [US2] Contract test for `resources/list` method in `graph/mcp/resource_provider_test.go`
- [ ] T039 [P] [US2] Contract test for `resources/read` method in `graph/mcp/resource_provider_test.go`
- [ ] T040 [P] [US2] Integration test for static resource registration in `graph/mcp/server_test.go`
- [ ] T041 [P] [US2] Integration test for dynamic resource registration in `graph/mcp/server_test.go`
- [ ] T042 [P] [US2] Integration test for resource size limits in `graph/mcp/resource_provider_test.go`

### Implementation for User Story 2

- [ ] T043 [P] [US2] Define Resource interface in `graph/mcp/resource_provider.go` (URI, MIMEType, Read methods)
- [ ] T044 [P] [US2] Define StaticResource struct in `graph/mcp/resource_provider.go` (uri, mimeType, content)
- [ ] T045 [P] [US2] Define DynamicResource struct in `graph/mcp/resource_provider.go` (uri, mimeType, generator func)
- [ ] T046 [P] [US2] Define ResourceProvider struct in `graph/mcp/resource_provider.go` (resources map, sync.RWMutex)
- [ ] T047 [US2] Implement ResourceProvider.RegisterStatic method in `graph/mcp/resource_provider.go` (validate URI pattern, size limit)
- [ ] T048 [US2] Implement ResourceProvider.RegisterDynamic method in `graph/mcp/resource_provider.go` (validate URI, generator not nil)
- [ ] T049 [US2] Implement ResourceProvider.Get method in `graph/mcp/resource_provider.go` (retrieve resource by URI)
- [ ] T050 [US2] Implement ResourceProvider.List method in `graph/mcp/resource_provider.go` (return all resource info)
- [ ] T051 [US2] Implement ResourceProvider.Read method in `graph/mcp/resource_provider.go` (invoke Read on resource, enforce size limit)
- [ ] T052 [US2] Implement `resources/list` handler in `graph/mcp/server.go` (integrate ResourceProvider.List)
- [ ] T053 [US2] Implement `resources/read` handler in `graph/mcp/server.go` (integrate ResourceProvider.Read)
- [ ] T054 [US2] Add MCPServer.RegisterStaticResource method in `graph/mcp/server.go` (public API)
- [ ] T055 [US2] Add MCPServer.RegisterDynamicResource method in `graph/mcp/server.go` (public API)
- [ ] T056 [US2] Add observability events for resource operations in `graph/mcp/server.go` (emit resource_read via Emitter)
- [ ] T057 [P] [US2] Write unit tests for ResourceProvider operations in `graph/mcp/resource_provider_test.go`
- [ ] T058 [P] [US2] Add workflow state resource example in `examples/mcp_server/stateful_server.go` (expose current state via Store.LoadLatest)
- [ ] T059 [P] [US2] Add checkpoint resource example in `examples/mcp_server/stateful_server.go` (expose checkpoints via Store.LoadCheckpoint)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - tools + resources both functional

---

## Phase 5: User Story 4 - Connect Multiple Workflows Across Applications (Priority: P2)

**Goal**: Enable cross-service MCP connections where one workflow can invoke tools/access resources from another via MCP protocol

**Independent Test**: Start two MCP servers, configure one as client of the other, invoke cross-service tool, verify execution

**Note**: This story builds on US1 and US2 but focuses on multi-server orchestration

### Tests for User Story 4

- [ ] T060 [P] [US4] Integration test for cross-service tool invocation in `tests/integration/mcp_integration_test.go`
- [ ] T061 [P] [US4] Integration test for cross-service resource reads in `tests/integration/mcp_integration_test.go`
- [ ] T062 [P] [US4] Integration test for connection timeout handling in `tests/integration/mcp_integration_test.go`
- [ ] T063 [P] [US4] Integration test for graceful disconnection in `tests/integration/mcp_integration_test.go`

### Implementation for User Story 4

- [ ] T064 [US4] Add connection tracking to MCPServer in `graph/mcp/server.go` (track active connections, ConnectionSession)
- [ ] T065 [US4] Define ConnectionSession struct in `graph/mcp/server.go` (clientInfo, protocolVersion, capabilities, connectionTime)
- [ ] T066 [US4] Implement concurrent connection handling in `graph/mcp/server.go` (one goroutine per connection, context cancellation)
- [ ] T067 [US4] Add context cancellation support for tool execution in `graph/mcp/tool_adapter.go` (respect ctx.Done during tool calls)
- [ ] T068 [US4] Add timeout handling for cross-service operations in `graph/mcp/server.go` (propagate timeouts from context)
- [ ] T069 [US4] Add graceful shutdown logic in `graph/mcp/server.go` (close active connections, wait for in-flight requests)
- [ ] T070 [US4] Add observability events for connection lifecycle in `graph/mcp/server.go` (emit client_connect, client_disconnect)
- [ ] T071 [P] [US4] Create multi-server example in `examples/mcp_server/cross_service_example.go` (two servers, one calling the other)
- [ ] T072 [P] [US4] Write unit tests for connection handling in `graph/mcp/server_test.go`

**Checkpoint**: Cross-service orchestration functional - multiple workflows can communicate via MCP

---

## Phase 6: User Story 3 - Provide Workflow Templates as Prompts (Priority: P3)

**Goal**: Enable developers to expose reusable workflow patterns as prompt templates with parameter substitution

**Independent Test**: Register prompt template with parameters, connect client, list prompts, render prompt with args, verify output

### Tests for User Story 3

- [ ] T073 [P] [US3] Contract test for `prompts/list` method in `graph/mcp/prompt_registry_test.go`
- [ ] T074 [P] [US3] Contract test for `prompts/get` method in `graph/mcp/prompt_registry_test.go`
- [ ] T075 [P] [US3] Integration test for prompt registration in `graph/mcp/server_test.go`
- [ ] T076 [P] [US3] Integration test for prompt rendering with arguments in `graph/mcp/server_test.go`
- [ ] T077 [P] [US3] Integration test for required parameter validation in `graph/mcp/prompt_registry_test.go`

### Implementation for User Story 3

- [ ] T078 [P] [US3] Define PromptTemplate struct in `graph/mcp/prompt_registry.go` (name, description, parameters, template)
- [ ] T079 [P] [US3] Define PromptParameter struct in `graph/mcp/prompt_registry.go` (name, description, required, defaultValue)
- [ ] T080 [P] [US3] Define PromptRegistry struct in `graph/mcp/prompt_registry.go` (prompts map, sync.RWMutex)
- [ ] T081 [US3] Implement PromptRegistry.Register method in `graph/mcp/prompt_registry.go` (validate name pattern, template placeholders)
- [ ] T082 [US3] Implement PromptRegistry.Get method in `graph/mcp/prompt_registry.go` (retrieve template by name)
- [ ] T083 [US3] Implement PromptRegistry.List method in `graph/mcp/prompt_registry.go` (return all prompt metadata)
- [ ] T084 [US3] Implement PromptRegistry.Render method in `graph/mcp/prompt_registry.go` (substitute {{param}} placeholders, validate required params)
- [ ] T085 [US3] Implement `prompts/list` handler in `graph/mcp/server.go` (integrate PromptRegistry.List)
- [ ] T086 [US3] Implement `prompts/get` handler in `graph/mcp/server.go` (integrate PromptRegistry.Render)
- [ ] T087 [US3] Add MCPServer.RegisterPrompt method in `graph/mcp/server.go` (public API for prompt registration)
- [ ] T088 [US3] Add parameter validation in `graph/mcp/prompt_registry.go` (check required params provided, apply defaults)
- [ ] T089 [US3] Add observability events for prompt operations in `graph/mcp/server.go` (emit prompt_render via Emitter)
- [ ] T090 [P] [US3] Write unit tests for PromptRegistry operations in `graph/mcp/prompt_registry_test.go`
- [ ] T091 [P] [US3] Add prompt examples in `examples/mcp_server/stateful_server.go` (start_workflow, analyze_results templates)

**Checkpoint**: All user stories should now be independently functional - tools, resources, cross-service, and prompts all working

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories, documentation, and production readiness

- [ ] T092 [P] Update quickstart.md with complete working example (validate against actual implementation)
- [ ] T093 [P] Add error handling guide in `graph/mcp/README.md` (JSON-RPC error codes, best practices)
- [ ] T094 [P] Add security considerations in `graph/mcp/README.md` (input validation, resource size limits, rate limiting recommendations)
- [ ] T095 [P] Add troubleshooting guide in `examples/mcp_server/README.md` (common issues, solutions)
- [ ] T096 [P] Add Claude Desktop configuration example in `examples/mcp_server/config.yaml`
- [ ] T097 [P] Create end-to-end integration test in `tests/integration/mcp_integration_test.go` (full workflow: server start, tool call, resource read, prompt render, shutdown)
- [ ] T098 Add performance benchmarks in `graph/mcp/server_benchmark_test.go` (tool invocation latency, concurrent connections)
- [ ] T099 Run `go test ./graph/mcp/...` and verify all tests pass
- [ ] T100 Run `go fmt ./graph/mcp/...` to format code
- [ ] T101 [P] Run `golangci-lint run ./graph/mcp/...` if available (check linting)
- [ ] T102 [P] Run `gosec ./graph/mcp/...` if available (security checks)
- [ ] T103 Validate quickstart.md example runs successfully (build weather server, test with mock MCP client)
- [ ] T104 Update CLAUDE.md with MCP server usage patterns

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - Tools are MVP foundation
- **User Story 2 (Phase 4)**: Depends on Foundational - Independent of US1 (can parallelize)
- **User Story 4 (Phase 5)**: Depends on US1 and US2 - Needs tools and resources for cross-service orchestration
- **User Story 3 (Phase 6)**: Depends on Foundational - Independent of US1/US2/US4 (prompts don't require tools/resources)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

```
Phase 2 (Foundation)
    │
    ├─────────┬─────────┬─────────┐
    ▼         ▼         ▼         ▼
   US1       US2       US3    (wait)
  (Tools) (Resources)(Prompts)   │
    │         │                   │
    └────┬────┘                   │
         ▼                        │
        US4 ◄───────────────────┘
   (Cross-Service)
```

- **User Story 1 (P1 - Tools)**: Can start after Foundational - No dependencies on other stories ✅ MVP
- **User Story 2 (P2 - Resources)**: Can start after Foundational - Independent of US1 ⚡ Can parallelize with US1
- **User Story 3 (P3 - Prompts)**: Can start after Foundational - Independent of US1/US2 ⚡ Can parallelize with US1/US2
- **User Story 4 (P2 - Cross-Service)**: Requires US1 and US2 complete - Builds on tools and resources

### Within Each User Story

- Tests MUST be written and FAIL before implementation (TDD requirement per constitution)
- Protocol/interface definitions before implementations
- Core implementations before integrations
- Unit tests alongside implementations
- Integration tests after core components complete
- Story complete and independently testable before moving to next priority

### Parallel Opportunities

**Setup (Phase 1)**:
- T003 (install dependency) + T004 (create examples dir) can run in parallel

**Foundational (Phase 2)**:
- T006 (protocol tests) + T008 (stdio tests) + T010 (jsonrpc tests) can run in parallel after T005, T007, T009
- T014 (lifecycle tests) + T016 (initialize tests) can run in parallel after T013, T015

**User Story 1**:
- T017-T021 (all tests) can run in parallel
- T022-T024 (struct definitions) can run in parallel
- T035 (unit tests) + T036 (example) + T037 (README) can run in parallel after implementation

**User Story 2**:
- T038-T042 (all tests) can run in parallel
- T043-T046 (struct definitions) can run in parallel
- T057-T059 (tests + examples) can run in parallel after implementation

**User Story 3**:
- T073-T077 (all tests) can run in parallel
- T078-T080 (struct definitions) can run in parallel
- T090-T091 (tests + examples) can run in parallel after implementation

**User Story 4**:
- T060-T063 (integration tests) can run in parallel
- T071-T072 (examples + tests) can run in parallel after implementation

**Polish (Phase 7)**:
- Most documentation tasks (T092-T096) can run in parallel
- T099-T102 (testing + linting) can run in parallel

**Cross-Phase Parallelization**:
- After Foundation complete: US1, US2, US3 can ALL start in parallel (US4 must wait for US1 + US2)

---

## Parallel Example: User Story 1 (Tools)

```bash
# After Foundation complete, launch all US1 tests together:
Task T017: "Contract test for tools/list"
Task T018: "Contract test for tools/call"
Task T019: "Integration test for tool registration"
Task T020: "Integration test for tool invocation"
Task T021: "Integration test for tool error handling"

# Launch struct definitions in parallel:
Task T022: "Define ToolMetadata struct"
Task T023: "Define RegisteredTool struct"
Task T024: "Define ToolRegistry struct"

# After implementation, launch parallel tasks:
Task T035: "Unit tests for ToolRegistry"
Task T036: "Example weather server"
Task T037: "README for weather example"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (~1 hour)
2. Complete Phase 2: Foundational (~2-3 days - CRITICAL)
3. Complete Phase 3: User Story 1 (~2-3 days)
4. **STOP and VALIDATE**: Test US1 independently with real MCP client (Claude Desktop)
5. Deploy/demo working MCP server with tools

**Total MVP Timeline**: ~5-7 days for core tool exposure capability

### Incremental Delivery (Recommended)

1. **Foundation** (Phase 1 + Phase 2): Setup + Core protocol → ~3-4 days
2. **MVP Release** (Phase 3): Add US1 (Tools) → Test independently → Deploy/Demo (~3 days)
3. **Increment 2** (Phase 4): Add US2 (Resources) → Test independently → Deploy/Demo (~2 days)
4. **Increment 3** (Phase 6): Add US3 (Prompts) → Test independently → Deploy/Demo (~2 days)
5. **Increment 4** (Phase 5): Add US4 (Cross-Service) → Test independently → Deploy/Demo (~2 days)
6. **Production Polish** (Phase 7): Documentation, benchmarks, security hardening (~2 days)

**Total Timeline**: ~14-16 days for complete feature with all user stories

### Parallel Team Strategy

With 3 developers:

1. **Week 1**: Team completes Setup + Foundational together (~3-4 days)
2. **Week 2** (once Foundation done):
   - Developer A: User Story 1 (Tools)
   - Developer B: User Story 2 (Resources)
   - Developer C: User Story 3 (Prompts)
3. **Week 3**:
   - Developer A: User Story 4 (Cross-Service) - needs US1 + US2 complete
   - Developer B: Polish and documentation
   - Developer C: Integration tests and benchmarks

**Parallel Timeline**: ~10-12 days with proper coordination

---

## Notes

- [P] tasks = different files, no dependencies, can run in parallel
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- TDD enforced per constitution: Write tests FIRST, ensure they FAIL, then implement
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Tool/Resource/Prompt registries are immutable after server start (registration during initialization only)
- All MCP operations must respect context cancellation (per LangGraph patterns)
- Observability events emitted via existing Emitter interface (no new logging system)
- Constitution compliance: Interface-first design, type safety, dependency minimalism, TDD workflow
