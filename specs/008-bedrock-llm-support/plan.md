# Implementation Plan: AWS Bedrock LLM Provider Support

**Branch**: `008-bedrock-llm-support` | **Date**: 2025-11-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-bedrock-llm-support/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Add AWS Bedrock foundation model support to LangGraph-Go's ChatModel interface, enabling developers to use AWS-managed LLMs (Claude, Llama, Titan, Mistral) with enterprise features like IAM-based access control, VPC endpoints, and CloudWatch observability. The Bedrock adapter will implement the existing ChatModel interface as a drop-in replacement for OpenAI/Anthropic/Google adapters, supporting streaming responses, tool calling, multi-region configuration, and automatic regional failover.

## Technical Context

**Language/Version**: Go 1.21+ (requires generics support for ChatModel interface compatibility)
**Primary Dependencies**:
- AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2/service/bedrockruntime`)
- AWS SDK for Go v2 config (`github.com/aws/aws-sdk-go-v2/config`) for credential loading
- Existing `graph/model` package for ChatModel interface and message types
**Storage**: N/A (adapter is stateless, uses existing Engine state management)
**Testing**: Go standard testing (`go test`), table-driven tests for model schema translations, integration tests with AWS Bedrock API
**Target Platform**: Cross-platform (Linux, macOS, Windows) - same as existing LangGraph-Go framework
**Project Type**: Single library project (graph orchestration framework with model adapters)
**Performance Goals**:
- Streaming: First token within 2 seconds for interactive workloads
- Non-streaming: Complete response within 30 seconds for typical queries
- Regional failover: Retry in fallback region within 5 seconds of primary region failure
**Constraints**:
- Must implement existing ChatModel interface without breaking changes (SC-007: 95% compatibility)
- Must respect context cancellation for in-flight requests (FR-013)
- Must validate credentials at initialization time, not first request (FR-015)
- Must handle Bedrock API throttling with exponential backoff (SC-006: up to 3 retries)
**Scale/Scope**:
- Support 4+ model families (Claude, Llama, Titan, Mistral) with distinct request/response schemas
- Support 6+ AWS regions where Bedrock is available
- Handle request/response payloads up to Bedrock limits (256KB input, 1MB output typical)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Type Safety & Determinism ✓

**Status**: PASS

**Compliance**:
- Bedrock adapter will implement `ChatModel` interface with existing `Message` and `ChatOut` types (compile-time type safety via generics)
- Adapter is stateless - all conversions between LangGraph message format and Bedrock schemas are pure functions
- No state mutation within adapter - configuration set at initialization, request/response translation is deterministic
- Engine handles state management via existing reducer pattern; adapter only translates messages

**Evidence**: Existing adapters (OpenAI, Anthropic, Google) already follow this pattern in `graph/model/*/`. Bedrock will follow same structure.

### II. Interface-First Design ✓

**Status**: PASS

**Compliance**:
- Bedrock adapter implements existing `ChatModel` interface (no interface changes required)
- AWS SDK dependency isolated to `graph/model/bedrock/` package (not in core framework)
- Test implementation will use mock Bedrock responses for unit tests (no AWS account required for testing)
- Breaking changes to ChatModel interface not needed (SC-007: 95% compatibility requirement)

**Evidence**: FR-001 explicitly requires implementing existing ChatModel interface. Follows same pattern as `graph/model/openai/`, `graph/model/anthropic/`, `graph/model/google/`.

### III. Test-Driven Development (NON-NEGOTIABLE) ✓

**Status**: PASS (with execution plan)

**Compliance**:
- Tests will be written BEFORE implementation following TDD red-green-refactor cycle
- Unit tests for schema translation functions (Message → Bedrock request, Bedrock response → ChatOut)
- Integration tests for actual Bedrock API calls (requires AWS credentials, marked with build tag `integration`)
- Table-driven tests for each model family's schema differences
- Example tests for quickstart documentation verification

**Execution Plan**:
1. Write failing tests for BedrockConfig validation (FR-015)
2. Write failing tests for Claude schema translation (P1 user story)
3. Implement until tests pass
4. Write failing tests for streaming support (US4)
5. Implement until tests pass
6. Write failing tests for tool calling (US5)
7. Implement until tests pass
8. Refactor with passing tests as safety net

**Evidence**: Constitution requires tests before commits. Plan includes test generation in Phase 1 (contracts/) and test execution in Phase 2 (tasks.md).

### IV. Observability & Debugging ✓

**Status**: PASS

**Compliance**:
- FR-012 requires logging Bedrock API request/response metadata (request IDs, latency, token counts)
- FR-014 requires exposing metadata in ChatOut response objects for downstream observability
- Errors captured in ChatOut.Error field per existing ChatModel interface contract
- Adapter will emit structured events compatible with existing Emitter interface (no Emitter changes needed)

**Evidence**: Existing ChatModel implementations already return structured errors. Bedrock adapter will follow same pattern with additional AWS-specific metadata (request ID, region, model ARN).

### V. Dependency Minimalism ✓

**Status**: PASS (with justification)

**New Dependencies**:
- `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` - Required for Bedrock API access (InvokeModel, InvokeModelWithResponseStream)
- `github.com/aws/aws-sdk-go-v2/config` - Required for AWS credential chain (environment vars, IAM roles, credential files)

**Justification**:
- AWS SDK v2 is the official, supported Go SDK for AWS services (maintained by AWS)
- No alternative exists for Bedrock API access (Bedrock is AWS-proprietary service)
- Dependencies isolated to `graph/model/bedrock/` package (core framework remains pure Go)
- Users only pay for what they use (adapter is opt-in, imported only when using Bedrock)

**Transitive Dependencies**: AWS SDK v2 has minimal transitive dependencies (smithy-go for AWS protocol support). Supply chain risk mitigated by AWS's security practices and Go module checksums.

**Evidence**: Spec assumptions state "AWS SDK for Go is available and will be used for Bedrock API communication". Dependencies section lists AWS SDK v2 explicitly.

### Go Idioms & Best Practices ✓

**Status**: PASS

**Compliance**:
- Will use `gofmt` for formatting (enforced in existing CI)
- Generics usage: BedrockAdapter will satisfy `ChatModel` interface (no new type parameters)
- Error handling: All errors returned explicitly via ChatOut.Error (no panics)
- Context usage: FR-013 requires respecting context.Context for cancellation
- Concurrency: Streaming responses will use channels for token delivery (idiomatic Go)

### Development Workflow ✓

**Status**: PASS (with execution plan)

**gocontext Indexing**:
- Will index codebase before Phase 0 research to explore existing adapter patterns
- Will use semantic search to find model schema translation patterns in existing adapters
- Will re-index after implementation to capture new Bedrock adapter code

**Code Review**:
- Will run `mcp-pr review_unstaged` before committing Bedrock adapter implementation
- Will address security issues (credential handling, error message information disclosure)
- Will verify test coverage before PR submission

**Evidence**: Constitution mandates gocontext for codebase navigation and mcp-pr for pre-commit review.

### Summary

**All gates PASS**. No violations requiring justification. Bedrock adapter follows existing adapter pattern (OpenAI, Anthropic, Google) with no architectural changes to core framework. Dependencies justified and isolated. TDD workflow planned for Phase 2 execution.

---

## Post-Design Constitution Re-evaluation

*Re-checked after Phase 1 design (data-model, contracts, quickstart)*

### Design Artifacts Review

**Generated Artifacts**:
- `research.md` - AWS Bedrock API patterns, SDK best practices, multi-region strategies
- `data-model.md` - BedrockConfig, BedrockAdapter, SchemaTranslator entities with validation rules
- `contracts/` - JSON schemas for Claude, Llama, Titan request/response formats + streaming events
- `quickstart.md` - 5 working examples (basic chat, workflow integration, streaming, tools, multi-region)

### Constitution Compliance Verification

**I. Type Safety & Determinism** ✓ CONFIRMED
- Data model defines stateless adapter with immutable configuration
- All schema translations are pure functions (Message → Bedrock request, Bedrock response → ChatOut)
- No state mutations after initialization (config validated once, reused)
- Tool calling round-trip preserves message history in workflow state (not adapter state)

**II. Interface-First Design** ✓ CONFIRMED
- BedrockAdapter implements existing ChatModel interface (no changes to chat.go)
- SchemaTranslator defined as strategy interface with 4 implementations (Claude, Llama, Titan, Mistral)
- AWS SDK dependency isolated to graph/model/bedrock/ package
- Mock implementations possible via SchemaTranslator interface for unit tests

**III. Test-Driven Development** ✓ CONFIRMED
- Contracts define complete request/response schemas for test fixtures
- Data model includes validation rules (testable via table-driven tests)
- Quickstart examples serve as integration test cases
- TDD execution order planned: config validation → Claude schema → streaming → tools → refactor

**IV. Observability & Debugging** ✓ CONFIRMED
- BedrockError type captures AWS request IDs, regions, retry status
- ChatOut.Meta includes input_tokens, output_tokens, stop_reason, model for all responses
- Streaming events include metadata for partial response debugging
- Error handling flow documented with retryable/non-retryable classification

**V. Dependency Minimalism** ✓ CONFIRMED
- Only 2 new dependencies (AWS SDK bedrockruntime + config packages)
- Both dependencies necessary (no alternative for AWS Bedrock API access)
- Core framework remains pure Go (adapters in separate packages)
- Transitive dependencies minimal (smithy-go for AWS protocol)

### Design Quality Gates

**Complexity Check**: ✓ PASS
- No new abstractions beyond existing adapter pattern
- Strategy pattern (SchemaTranslator) simplifies model family handling
- No Repository pattern, Domain Events, or other DDD complexity (adapter is stateless)

**Interface Consistency**: ✓ PASS
- BedrockAdapter.Chat() signature matches OpenAI/Anthropic/Google adapters exactly
- Streaming uses ChatStream() method (optional, not in base ChatModel interface)
- Tool calling uses existing ToolSpec[] and ToolCall[] types (no Bedrock-specific types)

**Error Handling**: ✓ PASS
- BedrockError wraps AWS errors with actionable messages (data-model.md §Error Types)
- Retry logic with exponential backoff defined (MaxRetries config, retryable errors documented)
- Context cancellation respected (FR-013, research.md §4 SDK best practices)

### Final Verdict

**All constitution gates remain PASSED after design**. Implementation can proceed to Phase 2 (task generation) with confidence. Design artifacts provide complete specifications for TDD implementation:

1. Contracts define exact JSON schemas for validation
2. Data model defines entities, validation rules, and error handling
3. Quickstart provides executable test cases (examples become integration tests)
4. Research documents AWS SDK patterns and best practices

**Ready for**: `/speckit.tasks` command to generate implementation task breakdown.

## Project Structure

### Documentation (this feature)

```text
specs/008-bedrock-llm-support/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output: Bedrock API schema research, best practices
├── data-model.md        # Phase 1 output: BedrockConfig, ModelSchema entity definitions
├── quickstart.md        # Phase 1 output: Getting started guide for Bedrock adapter
├── contracts/           # Phase 1 output: JSON schemas for request/response formats
│   ├── bedrock-claude-request.json
│   ├── bedrock-claude-response.json
│   ├── bedrock-llama-request.json
│   ├── bedrock-titan-request.json
│   └── streaming-event.json
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
graph/model/bedrock/
├── bedrock.go           # BedrockAdapter implementation, ChatModel interface
├── bedrock_test.go      # Unit tests for adapter (table-driven schema tests)
├── config.go            # BedrockConfig struct, validation logic
├── config_test.go       # Config validation tests
├── schema.go            # Model family schema translations (Claude, Llama, Titan, Mistral)
├── schema_test.go       # Schema translation unit tests
├── streaming.go         # Streaming response handling
├── streaming_test.go    # Streaming tests
├── tools.go             # Tool/function calling translation
├── tools_test.go        # Tool calling tests
└── integration_test.go  # Integration tests with actual Bedrock API (build tag: integration)

graph/model/
├── bedrock/             # New Bedrock adapter package (above)
├── anthropic/           # Existing Anthropic adapter
├── google/              # Existing Google adapter
├── openai/              # Existing OpenAI adapter
├── chat.go              # ChatModel interface definition (no changes)
├── chat_test.go         # Interface contract tests
├── mock.go              # Mock ChatModel for testing (no changes)
└── mock_test.go         # Mock tests

examples/bedrock_quickstart/
├── main.go              # Quickstart example: basic chat with Bedrock
├── streaming.go         # Example: streaming responses
├── tools.go             # Example: tool calling with Bedrock
└── README.md            # Setup instructions (AWS credentials, permissions)
```

**Structure Decision**: Single library project structure. Bedrock adapter lives in `graph/model/bedrock/` following existing adapter pattern. No changes to core `graph/` package. Examples in `examples/bedrock_quickstart/` for documentation.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

N/A - No constitution violations. All gates passed.
