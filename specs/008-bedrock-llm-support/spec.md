# Feature Specification: AWS Bedrock LLM Provider Support

**Feature Branch**: `008-bedrock-llm-support`
**Created**: 2025-11-14
**Status**: Draft
**Input**: User description: "add the ability to connect to AWS Bedrock models to the list of supported LLM providers"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Basic Bedrock Model Integration (Priority: P1)

A developer building a LangGraph-Go workflow wants to use AWS Bedrock foundation models (like Claude, Llama, or Titan) instead of directly calling OpenAI or Anthropic APIs. They need to configure a Bedrock model adapter with AWS credentials and region, then use it as a drop-in replacement for existing ChatModel implementations.

**Why this priority**: This is the core functionality that enables AWS Bedrock usage. Without this, no Bedrock integration is possible. This story delivers immediate value by allowing developers to leverage AWS-managed models with enterprise features like VPC endpoints, CloudWatch logging, and IAM-based access control.

**Independent Test**: Can be fully tested by creating a Bedrock adapter with valid AWS credentials, sending a simple chat request, and verifying a successful response. Delivers the core value of AWS Bedrock access without requiring advanced features.

**Acceptance Scenarios**:

1. **Given** AWS credentials with Bedrock access permissions, **When** developer creates a Bedrock adapter for Claude 3 Sonnet and sends a chat message, **Then** the adapter successfully returns a response from the Bedrock-hosted model
2. **Given** a configured Bedrock adapter, **When** integrated into a LangGraph-Go workflow as a node, **Then** the workflow executes successfully and state updates with model responses
3. **Given** AWS credentials with insufficient permissions, **When** attempting to invoke a Bedrock model, **Then** the adapter returns a clear authentication error indicating the IAM permission issue

---

### User Story 2 - Multi-Region Support (Priority: P2)

A developer deploying workflows across multiple AWS regions needs to configure Bedrock adapters to use region-specific endpoints. This ensures low latency by routing requests to the nearest Bedrock service endpoint and supports disaster recovery scenarios where primary regions may be unavailable.

**Why this priority**: Multi-region support is critical for production deployments requiring high availability and low latency. While basic single-region access (P1) works for development and simple deployments, production workloads need regional flexibility.

**Independent Test**: Can be tested by creating Bedrock adapters configured for different regions (us-east-1, us-west-2, eu-west-1), invoking models in each region, and verifying requests route to correct endpoints via AWS CloudTrail logs.

**Acceptance Scenarios**:

1. **Given** a Bedrock adapter configured for us-west-2, **When** invoking a model, **Then** the request is sent to the us-west-2 Bedrock endpoint
2. **Given** Bedrock adapters for multiple regions, **When** a workflow uses different adapters for different nodes, **Then** each node's requests route to the configured region
3. **Given** a region where a specific model is unavailable, **When** attempting to invoke that model, **Then** the adapter returns a clear error indicating model availability constraints

---

### User Story 3 - Cross-Region Fallback (Priority: P3)

A developer building resilient workflows needs automatic fallback to secondary AWS regions when the primary Bedrock region is experiencing service issues or throttling. The adapter should transparently retry failed requests in backup regions without manual intervention.

**Why this priority**: This enhances reliability for mission-critical workflows but is not essential for basic functionality. Most users can implement application-level retry logic initially, making this a nice-to-have optimization rather than a core requirement.

**Independent Test**: Can be tested by configuring a Bedrock adapter with primary and fallback regions, simulating a primary region failure (via network rules or invalid endpoint), and verifying automatic failover to the backup region with successful response delivery.

**Acceptance Scenarios**:

1. **Given** a Bedrock adapter with primary region us-east-1 and fallback us-west-2, **When** us-east-1 Bedrock service returns throttling errors, **Then** requests automatically retry in us-west-2
2. **Given** a multi-region configuration, **When** all configured regions are unavailable, **Then** the adapter exhausts all fallback options and returns a clear error indicating regional unavailability
3. **Given** successful primary region requests, **When** no errors occur, **Then** no fallback attempts are made and all requests use the primary region

---

### User Story 4 - Streaming Response Support (Priority: P2)

A developer building interactive applications needs to stream model responses token-by-token from Bedrock models to provide real-time feedback to users. The adapter should support Bedrock's streaming API and integrate with LangGraph-Go's node execution model.

**Why this priority**: Streaming significantly improves user experience for conversational and generative workflows by reducing perceived latency. While non-streaming requests work for batch processing, interactive applications need progressive response rendering.

**Independent Test**: Can be tested by invoking a Bedrock model with streaming enabled, collecting tokens as they arrive, and verifying complete response assembly matches non-streaming output. Delivers value for real-time applications without requiring full workflow integration.

**Acceptance Scenarios**:

1. **Given** a Bedrock adapter configured for streaming, **When** invoking a model with a chat message, **Then** response tokens are delivered progressively via a callback or channel
2. **Given** a streaming request, **When** the connection is interrupted mid-stream, **Then** the adapter returns an error with partial response data received so far
3. **Given** a Bedrock model that doesn't support streaming, **When** streaming is requested, **Then** the adapter falls back to non-streaming mode or returns a clear capability error

---

### User Story 5 - Tool/Function Calling Support (Priority: P2)

A developer building agentic workflows needs Bedrock models to invoke tools during generation (for models that support function calling like Claude). The adapter should translate LangGraph-Go tool specifications to Bedrock's tool format and handle tool invocation responses.

**Why this priority**: Tool calling is essential for building autonomous agents that can take actions beyond text generation. While basic chat works without this, agentic workflows (a core LangGraph use case) require tool integration.

**Independent Test**: Can be tested by defining a simple tool (e.g., "get_weather"), providing it to a Bedrock adapter, sending a prompt that triggers tool usage, and verifying the model returns a tool call request that can be executed and fed back for final response generation.

**Acceptance Scenarios**:

1. **Given** a Bedrock adapter with tool specifications, **When** a model decides to use a tool, **Then** the adapter returns a tool call request with function name and arguments
2. **Given** a tool call response, **When** fed back to the model, **Then** the model generates a final answer incorporating the tool result
3. **Given** a Bedrock model that doesn't support tool calling, **When** tools are provided, **Then** the adapter ignores tool specs or returns a clear capability error

---

### Edge Cases

- What happens when AWS credentials expire during a long-running workflow?
- How does the adapter handle Bedrock service throttling (rate limits)?
- What occurs when a requested model ID doesn't exist in the configured region?
- How are connection timeouts to Bedrock endpoints handled?
- What happens when response sizes exceed Bedrock's payload limits?
- How does the adapter handle malformed responses from Bedrock (invalid JSON)?
- What occurs when AWS SDK is not properly configured (missing credentials, invalid region)?
- How are network connectivity issues to AWS endpoints reported?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a Bedrock adapter that implements the existing ChatModel interface
- **FR-002**: Adapter MUST support authentication via AWS credentials (environment variables, IAM roles, credential files, or explicit configuration)
- **FR-003**: Adapter MUST allow configuration of AWS region for Bedrock endpoint selection
- **FR-004**: Adapter MUST support all Bedrock foundation models accessible via the InvokeModel API (Claude, Llama, Titan, Mistral, etc.)
- **FR-005**: Adapter MUST translate LangGraph-Go message formats to Bedrock-specific request schemas for each model family
- **FR-006**: Adapter MUST translate Bedrock response schemas back to LangGraph-Go message formats
- **FR-007**: System MUST handle Bedrock-specific error responses (throttling, invalid model, authentication failures) with clear error messages
- **FR-008**: Adapter MUST support model-specific parameters (temperature, top_p, max_tokens, stop sequences) via configuration
- **FR-009**: Adapter MUST support streaming responses for models that offer streaming capability
- **FR-010**: Adapter MUST support tool/function calling for models with tool support (e.g., Claude via Bedrock)
- **FR-011**: Adapter MUST allow optional configuration of multiple fallback regions for automatic retry
- **FR-012**: System MUST log Bedrock API request/response metadata for observability (request IDs, latency, token counts)
- **FR-013**: Adapter MUST respect context cancellation for aborting in-flight Bedrock requests
- **FR-014**: Adapter MUST expose Bedrock-specific metadata (model ID, input/output token counts, request ID) in response objects
- **FR-015**: System MUST validate AWS credentials and region configuration at adapter initialization time
- **FR-016**: Adapter MUST support custom endpoint configuration for VPC endpoints or testing against local Bedrock emulators

### Key Entities

- **BedrockAdapter**: Adapter implementing ChatModel interface, configured with AWS credentials, region, and model ID. Manages request translation and response parsing.
- **BedrockConfig**: Configuration object containing AWS region, credentials source, model ID, inference parameters (temperature, max_tokens), optional fallback regions, and endpoint overrides.
- **ModelSchema**: Internal representation mapping model families (Claude, Llama, Titan) to their specific request/response JSON schemas required by Bedrock InvokeModel API.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Developers can configure a Bedrock adapter and execute a successful chat request in under 5 minutes with standard AWS credentials
- **SC-002**: Adapter successfully invokes models in all AWS regions where Bedrock is available (us-east-1, us-west-2, eu-west-1, ap-southeast-1, etc.)
- **SC-003**: Streaming responses deliver initial tokens within 2 seconds of request initiation for interactive workloads
- **SC-004**: Tool calling workflows complete end-to-end (tool request, execution, final response) with zero manual translation steps
- **SC-005**: Error messages for common failures (invalid credentials, missing permissions, throttling) are actionable and include remediation guidance
- **SC-006**: Adapter handles Bedrock API throttling gracefully with automatic retry up to 3 attempts
- **SC-007**: 95% of Bedrock adapter usage matches existing ChatModel interface patterns with no breaking changes

## Assumptions

- AWS SDK for Go is available and will be used for Bedrock API communication
- Developers have existing AWS accounts with Bedrock access enabled
- AWS IAM permissions for Bedrock model invocation are already configured by infrastructure teams
- Bedrock service is available in at least one AWS region accessible to the user
- Model-specific schema translations are documented in AWS Bedrock documentation and stable across API versions
- Default authentication follows standard AWS SDK credential chain (environment vars → IAM role → credential file)
- Network connectivity to AWS Bedrock endpoints is available (direct internet or VPC endpoints)
- Existing LangGraph-Go workflows using other ChatModel implementations can switch to Bedrock adapter with minimal code changes (configuration only)

## Dependencies

- AWS SDK for Go (github.com/aws/aws-sdk-go-v2) for Bedrock API access
- Existing ChatModel interface and message types from graph/model package
- AWS Bedrock service availability in target deployment regions
- IAM permissions configured for bedrock:InvokeModel and bedrock:InvokeModelWithResponseStream actions

## Out of Scope

- Custom fine-tuned Bedrock models (will support foundation models only initially)
- Bedrock Agents or Knowledge Bases integration (focused on foundation model inference)
- Automatic model selection or routing between providers (developer explicitly chooses Bedrock)
- Cost tracking or billing integration with AWS Cost Explorer
- Support for Bedrock Provisioned Throughput (will use on-demand inference initially)
- Integration with AWS SageMaker endpoints (Bedrock-hosted models only)
- Advanced retry strategies beyond exponential backoff with regional fallback (covered in P3 story)
- Bedrock Guardrails integration (content filtering/safety features)
