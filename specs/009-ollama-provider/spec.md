# Feature Specification: Ollama Model Provider

**Feature Branch**: `009-ollama-provider`
**Created**: 2025-11-14
**Status**: Draft
**Input**: User description: "add Ollama as a model provider. This should support either local or remote Ollama instances"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Local Model Execution (Priority: P1)

A developer wants to run LLM-powered workflows using locally installed Ollama models without incurring API costs or requiring internet connectivity. They configure the adapter to connect to their local Ollama instance (default: `http://localhost:11434`) and execute workflows using models they've already pulled (e.g., `llama3.2`, `mistral`, `codellama`).

**Why this priority**: This is the primary use case for Ollama - enabling local, cost-free LLM execution for development, testing, and privacy-sensitive workloads. It delivers immediate value without dependencies on cloud services.

**Independent Test**: Can be fully tested by installing Ollama locally, pulling a model (`ollama pull llama3.2`), configuring the adapter with default settings, and executing a simple chat workflow. Delivers value by enabling offline LLM execution.

**Acceptance Scenarios**:

1. **Given** Ollama is running locally with llama3.2 model, **When** user creates adapter with default endpoint and model name, **Then** adapter successfully sends messages and receives responses
2. **Given** local Ollama instance is unavailable, **When** user attempts to create adapter or send messages, **Then** adapter returns clear connection error
3. **Given** user specifies non-existent model name, **When** adapter attempts chat operation, **Then** adapter returns model not found error

---

### User Story 2 - Remote Instance Support (Priority: P2)

A developer wants to connect to a remote Ollama instance running on a server, container, or different machine in their network. They configure the adapter with a custom endpoint (e.g., `http://ollama-server:11434` or `https://ollama.example.com`) and authenticate if required.

**Why this priority**: Extends Ollama support beyond localhost to enable team shared instances, containerized deployments, and edge computing scenarios. Essential for production-like testing and resource-constrained local machines.

**Independent Test**: Can be tested independently by deploying Ollama to a remote host (Docker container or VM), configuring adapter with custom endpoint, and verifying connectivity. Delivers value by enabling centralized model hosting.

**Acceptance Scenarios**:

1. **Given** Ollama running on remote server with accessible endpoint, **When** user configures adapter with custom URL, **Then** adapter successfully connects and executes chat operations
2. **Given** remote instance requires authentication, **When** user provides credentials via configuration, **Then** adapter authenticates and executes requests
3. **Given** remote instance is behind firewall or unreachable, **When** adapter attempts connection, **Then** adapter returns network error with endpoint details

---

### User Story 3 - Model Selection and Configuration (Priority: P3)

A developer wants to select from available Ollama models based on task requirements (reasoning, speed, specialized domains) and configure model parameters (temperature, context window, top-p, seed) for consistent or creative outputs.

**Why this priority**: Enables optimization of model selection and fine-tuning of generation parameters. Important for advanced use cases but not essential for basic functionality.

**Independent Test**: Can be tested by configuring adapter with different models and parameter sets, then comparing outputs for consistency (with seed) and variability (with temperature). Delivers value through output quality control.

**Acceptance Scenarios**:

1. **Given** multiple models available in Ollama, **When** user specifies model name in configuration, **Then** adapter uses specified model for all requests
2. **Given** user configures generation parameters (temperature, top-p, seed), **When** adapter sends requests, **Then** Ollama applies parameters to generation
3. **Given** user provides invalid parameter values, **When** adapter validates configuration, **Then** adapter returns validation error before making API calls

---

### User Story 4 - Tool Calling Support (Priority: P4)

A developer wants to use Ollama models that support tool/function calling (e.g., newer Llama models) within LangGraph workflows, enabling the LLM to invoke external tools and APIs as part of the reasoning process.

**Why this priority**: Extends Ollama integration to support advanced agentic workflows. Lower priority because tool calling support varies across Ollama models and is not universally available.

**Independent Test**: Can be tested with tool-capable Ollama models by providing tool specifications, executing chat with tools, and verifying tool calls are returned correctly. Delivers value for agentic workflow patterns.

**Acceptance Scenarios**:

1. **Given** Ollama model supports tool calling, **When** user provides tool specifications, **Then** adapter formats tools in Ollama-compatible format and sends with request
2. **Given** LLM decides to call a tool, **When** adapter receives response, **Then** adapter parses tool calls and returns them in standard ToolCall format
3. **Given** Ollama model does not support tools, **When** user provides tool specifications, **Then** adapter either ignores tools gracefully or returns unsupported operation error

---

### Edge Cases

- What happens when Ollama is running but the specified model is not pulled locally?
- How does the adapter handle Ollama instance restart or temporary unavailability during execution?
- What happens when request context timeout is shorter than Ollama's generation time?
- How does the adapter handle streaming vs non-streaming response modes?
- What happens when Ollama returns malformed JSON or unexpected response structure?
- How are Unicode and special characters handled in prompts and responses?
- What happens when remote Ollama instance uses self-signed SSL certificates?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow configuration of Ollama endpoint URL (default: `http://localhost:11434`)
- **FR-002**: System MUST support specifying model name (e.g., `llama3.2`, `mistral`, `codellama`)
- **FR-003**: System MUST implement the `ChatModel` interface for LangGraph compatibility
- **FR-004**: System MUST convert standard `Message` format to Ollama API format
- **FR-005**: System MUST parse Ollama responses back to standard `ChatOut` format
- **FR-006**: System MUST respect context cancellation and timeouts
- **FR-007**: System MUST support generation parameters (temperature, top-p, seed, num-predict)
- **FR-008**: System MUST handle connection errors with clear error messages
- **FR-009**: System MUST validate model availability before execution
- **FR-010**: System MUST support tool/function calling for compatible models
- **FR-011**: System MUST handle authentication headers if required by remote instances
- **FR-012**: System MUST translate Ollama-specific errors to common error format
- **FR-013**: System MUST support both streaming and non-streaming response modes
- **FR-014**: System MUST provide metadata in response (model used, generation stats)

### Key Entities

- **OllamaAdapter**: Implements `ChatModel` interface, manages connection to Ollama instance, translates requests/responses
- **OllamaConfig**: Configuration object containing endpoint URL, model name, authentication credentials, generation parameters
- **OllamaRequest**: Internal representation of Ollama API request format
- **OllamaResponse**: Internal representation of Ollama API response format
- **ConnectionManager**: Handles HTTP connection pooling, retry logic, and endpoint validation

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can configure and execute workflows with local Ollama instances in under 5 minutes (from installation to first successful workflow run)
- **SC-002**: Adapter successfully processes chat requests with response latency within 10% of direct Ollama API calls (minimal overhead)
- **SC-003**: Adapter handles 100% of common Ollama error scenarios with user-actionable error messages (connection failures, missing models, invalid parameters)
- **SC-004**: Workflows using Ollama adapter achieve the same functional outcomes as workflows using cloud provider adapters (OpenAI, Anthropic) for equivalent tasks
- **SC-005**: Adapter supports at least 90% of Ollama's generation parameters and API features
- **SC-006**: Zero additional dependencies beyond standard Go libraries and Ollama HTTP API (no vendor-specific SDKs required unless Ollama provides official Go client)

## Assumptions

- Ollama is already installed and running on the target machine or accessible via network
- Users have already pulled the desired models using `ollama pull <model-name>`
- Ollama API format follows the documented specification at https://github.com/ollama/ollama/blob/main/docs/api.md
- Tool calling support is optional and only available for specific Ollama models
- Authentication requirements for remote instances follow standard HTTP authentication patterns (Bearer tokens, Basic Auth)
- Streaming responses use standard HTTP chunked transfer encoding or server-sent events
- Ollama version compatibility targets Ollama 0.1.0+ (current stable releases)

## Dependencies

- Ollama server installation and running instance
- Network connectivity to Ollama endpoint (localhost or remote)
- Existing LangGraph framework and `ChatModel` interface

## Scope

### In Scope

- Ollama adapter implementation conforming to `ChatModel` interface
- Support for local (`http://localhost:11434`) and remote endpoints
- Configuration of model name and generation parameters
- Error handling and translation for common failure modes
- Tool calling support for compatible models
- Connection validation and retry logic
- Documentation and usage examples

### Out of Scope

- Ollama installation or model management (assumed pre-existing)
- Custom Ollama model training or fine-tuning
- Ollama server administration or monitoring
- Embedding generation (separate API endpoint, not chat)
- Multi-modal inputs (images, audio) unless already supported by ChatModel interface
- Ollama-specific advanced features (model caching strategies, GPU allocation)

## Non-Functional Requirements

- **Performance**: Adapter overhead must be minimal (< 10ms per request)
- **Reliability**: Graceful handling of network failures and timeouts
- **Maintainability**: Clean separation between adapter logic and Ollama API client
- **Testability**: Mock-friendly design for unit testing without running Ollama
- **Documentation**: Clear examples for local and remote configuration scenarios
