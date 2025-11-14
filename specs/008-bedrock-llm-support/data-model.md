# Data Model: AWS Bedrock LLM Provider Support

**Feature**: AWS Bedrock LLM Provider Support
**Date**: 2025-11-14
**Spec**: [spec.md](./spec.md)

## Overview

This document defines the data entities for the Bedrock adapter implementation. All entities are configuration and schema translation types - the adapter is stateless and does not persist data. State management is handled by the existing LangGraph-Go Engine via the reducer pattern.

## Core Entities

### BedrockConfig

**Purpose**: Configuration object for initializing a Bedrock adapter instance.

**Fields**:

| Field | Type | Required | Description | Validation Rules |
|-------|------|----------|-------------|------------------|
| `Region` | `string` | Yes | AWS region for Bedrock endpoint (e.g., "us-east-1") | Must be valid AWS region identifier |
| `ModelID` | `string` | Yes | Bedrock model identifier (e.g., "anthropic.claude-3-5-sonnet-20241022-v2:0") | Must be non-empty, format: `provider.model-name` |
| `CredentialsProvider` | `aws.CredentialsProvider` | No | Explicit credentials (overrides default chain) | If nil, uses AWS SDK default credential chain |
| `EndpointURL` | `string` | No | Custom endpoint URL (for VPC endpoints or testing) | If set, must be valid URL |
| `FallbackRegions` | `[]string` | No | Ordered list of fallback regions for retry | Each must be valid AWS region identifier |
| `MaxRetries` | `int` | No | Maximum retry attempts for throttling | Default: 3, Range: 0-10 |
| `Temperature` | `float64` | No | Model temperature parameter | Range: 0.0-1.0, Default: provider-specific |
| `MaxTokens` | `int` | No | Maximum tokens to generate | Range: 1-model_max, Default: provider-specific |
| `TopP` | `float64` | No | Nucleus sampling parameter | Range: 0.0-1.0, Default: provider-specific |
| `StopSequences` | `[]string` | No | Sequences that stop generation | Max length: 4 sequences (Bedrock limit) |
| `StreamingEnabled` | `bool` | No | Enable streaming responses | Default: false, requires model support |

**Relationships**:
- Referenced by `BedrockAdapter` (composition)
- Validated at adapter initialization via `Validate()` method

**Validation Rules**:
- `Region` must be in set: `[us-east-1, us-west-2, eu-west-1, eu-central-1, ap-southeast-1, ap-northeast-1, ...]`
- `ModelID` format must match: `^[a-z0-9\-]+\.[a-z0-9\-\.]+(:[\d]+)?$`
- `Temperature` must be in range [0.0, 1.0] if set
- `MaxTokens` must be positive and <= model-specific max (4096 for Claude, 8192 for Llama 3.1, etc.)
- `TopP` must be in range [0.0, 1.0] if set
- `FallbackRegions` must not contain duplicates or primary `Region`
- `MaxRetries` must be in range [0, 10]

**State Transitions**: N/A (configuration is immutable after initialization)

---

### BedrockAdapter

**Purpose**: Main adapter implementing the `ChatModel` interface for AWS Bedrock.

**Fields**:

| Field | Type | Required | Description | Validation Rules |
|-------|------|----------|-------------|------------------|
| `config` | `*BedrockConfig` | Yes | Configuration (private field) | Must be valid (Validate() passed) |
| `client` | `*bedrockruntime.Client` | Yes | AWS SDK Bedrock Runtime client (private) | Initialized from config |
| `modelFamily` | `ModelFamily` | Yes | Detected model family (private) | Derived from config.ModelID |
| `schemaTranslator` | `SchemaTranslator` | Yes | Schema translation strategy (private) | Based on modelFamily |

**Methods**:
- `Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)` - Implements ChatModel interface
- `Validate() error` - Validates configuration at initialization

**Relationships**:
- Implements `ChatModel` interface from `graph/model` package
- Composes `BedrockConfig`
- Uses `SchemaTranslator` for request/response conversion
- Uses AWS SDK `bedrockruntime.Client` for API calls

**State Transitions**: N/A (adapter is stateless)

---

### ModelFamily

**Purpose**: Enumeration of supported Bedrock model families for schema selection.

**Values**:

| Value | ModelID Pattern | Schema Translator |
|-------|-----------------|-------------------|
| `Claude` | `anthropic.claude-*` | `ClaudeSchemaTranslator` |
| `Llama` | `meta.llama*` | `LlamaSchemaTranslator` |
| `Titan` | `amazon.titan-*` | `TitanSchemaTranslator` |
| `Mistral` | `mistral.*` | `MistralSchemaTranslator` |

**Detection Logic**: Derived from `BedrockConfig.ModelID` prefix matching.

---

### SchemaTranslator (Interface)

**Purpose**: Strategy pattern interface for translating between LangGraph message format and Bedrock model-specific schemas.

**Methods**:
- `TranslateRequest(messages []Message, tools []ToolSpec, config *BedrockConfig) (json.RawMessage, error)`
- `TranslateResponse(response json.RawMessage) (ChatOut, error)`
- `TranslateStreamEvent(event json.RawMessage) (StreamChunk, error)`
- `SupportsStreaming() bool`
- `SupportsTools() bool`

**Implementations**:
- `ClaudeSchemaTranslator` - Anthropic Messages API format
- `LlamaSchemaTranslator` - Meta Llama prompt format
- `TitanSchemaTranslator` - Amazon Titan format
- `MistralSchemaTranslator` - Mistral instruction format

---

## Request/Response Schemas

### ClaudeRequest (Anthropic Messages API)

**Purpose**: Request schema for Claude models via Bedrock.

**Structure**:

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 4096,
  "messages": [
    {
      "role": "user",
      "content": "Hello, Claude"
    }
  ],
  "system": "You are a helpful assistant.",
  "temperature": 0.7,
  "top_p": 0.9,
  "stop_sequences": ["Human:", "Assistant:"],
  "tools": [
    {
      "name": "get_weather",
      "description": "Get weather for location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        },
        "required": ["location"]
      }
    }
  ]
}
```

**Field Mapping from LangGraph**:
- `messages[]` ← `Message[]` (role: user/assistant, content: string)
- `system` ← First message with role="system" (extracted and removed from messages array)
- `max_tokens` ← `BedrockConfig.MaxTokens` (default: 4096)
- `temperature` ← `BedrockConfig.Temperature` (default: 1.0)
- `top_p` ← `BedrockConfig.TopP` (default: not set)
- `stop_sequences` ← `BedrockConfig.StopSequences`
- `tools[]` ← `ToolSpec[]` (translated to Claude tool schema)

---

### ClaudeResponse

**Purpose**: Response schema from Claude models via Bedrock.

**Structure**:

```json
{
  "id": "msg_01XYZ",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 12,
    "output_tokens": 25
  }
}
```

**Field Mapping to LangGraph ChatOut**:
- `ChatOut.Text` ← `content[0].text` (first text content block)
- `ChatOut.ToolCalls` ← `content[].tool_use` (if present, converted to ToolCall[])
- `ChatOut.Meta` ← `{"request_id": id, "model": model, "stop_reason": stop_reason, "input_tokens": usage.input_tokens, "output_tokens": usage.output_tokens}`

**Tool Call Content Block**:

```json
{
  "type": "tool_use",
  "id": "toolu_01ABC",
  "name": "get_weather",
  "input": {
    "location": "San Francisco"
  }
}
```

Maps to `ToolCall{Name: "get_weather", Arguments: {"location": "San Francisco"}}`

---

### LlamaRequest (Meta Llama Prompt Format)

**Purpose**: Request schema for Llama models via Bedrock.

**Structure**:

```json
{
  "prompt": "<|begin_of_text|><|start_header_id|>user<|end_header_id|>\n\nHello, Llama<|eot_id|><|start_header_id|>assistant<|end_header_id|>",
  "max_gen_len": 512,
  "temperature": 0.7,
  "top_p": 0.9
}
```

**Field Mapping from LangGraph**:
- `prompt` ← Formatted string from `Message[]` using Llama instruction template tags
- `max_gen_len` ← `BedrockConfig.MaxTokens` (default: 512)
- `temperature` ← `BedrockConfig.Temperature` (default: 0.5)
- `top_p` ← `BedrockConfig.TopP` (default: 0.9)

**Note**: Llama models do NOT support tool calling or system messages. System instructions must be prepended to first user message.

---

### LlamaResponse

**Purpose**: Response schema from Llama models via Bedrock.

**Structure**:

```json
{
  "generation": "Hello! How can I assist you today?",
  "prompt_token_count": 15,
  "generation_token_count": 12,
  "stop_reason": "stop"
}
```

**Field Mapping to LangGraph ChatOut**:
- `ChatOut.Text` ← `generation`
- `ChatOut.ToolCalls` ← `nil` (Llama does not support tools)
- `ChatOut.Meta` ← `{"stop_reason": stop_reason, "input_tokens": prompt_token_count, "output_tokens": generation_token_count}`

---

### TitanRequest (Amazon Titan Text Format)

**Purpose**: Request schema for Titan Text models via Bedrock.

**Structure**:

```json
{
  "inputText": "Hello, Titan",
  "textGenerationConfig": {
    "maxTokenCount": 512,
    "temperature": 0.7,
    "topP": 0.9,
    "stopSequences": ["User:"]
  }
}
```

**Field Mapping from LangGraph**:
- `inputText` ← Concatenated text from `Message[]` (format: "User: {msg}\nBot: {msg}")
- `textGenerationConfig.maxTokenCount` ← `BedrockConfig.MaxTokens` (default: 512)
- `textGenerationConfig.temperature` ← `BedrockConfig.Temperature` (default: 0.7)
- `textGenerationConfig.topP` ← `BedrockConfig.TopP` (default: 0.9)
- `textGenerationConfig.stopSequences` ← `BedrockConfig.StopSequences`

**Note**: Titan does NOT support tool calling or structured conversation history.

---

### TitanResponse

**Purpose**: Response schema from Titan Text models via Bedrock.

**Structure**:

```json
{
  "results": [
    {
      "tokenCount": 23,
      "outputText": "Hello! How may I help you?",
      "completionReason": "FINISH"
    }
  ],
  "inputTextTokenCount": 3
}
```

**Field Mapping to LangGraph ChatOut**:
- `ChatOut.Text` ← `results[0].outputText`
- `ChatOut.ToolCalls` ← `nil` (Titan does not support tools)
- `ChatOut.Meta` ← `{"completion_reason": results[0].completionReason, "input_tokens": inputTextTokenCount, "output_tokens": results[0].tokenCount}`

---

### StreamChunk

**Purpose**: Intermediate representation of streaming response chunks.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `Delta` | `string` | Incremental text content |
| `ToolCallDelta` | `*ToolCallDelta` | Incremental tool call (if applicable) |
| `FinishReason` | `string` | Reason for completion (if final chunk) |
| `Metadata` | `map[string]any` | Model-specific metadata |

**Usage**: Returned by `SchemaTranslator.TranslateStreamEvent()` for assembling streaming responses.

---

## Error Types

### BedrockError

**Purpose**: Wrapper for AWS Bedrock-specific errors with actionable messages.

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `Code` | `string` | Error code (e.g., "AccessDeniedException", "ThrottlingException") |
| `Message` | `string` | Human-readable error message |
| `RequestID` | `string` | AWS request ID for debugging |
| `Region` | `string` | Region where error occurred |
| `Retryable` | `bool` | Whether error is transient and retryable |

**Common Error Codes**:
- `AccessDeniedException` - IAM permissions issue (not retryable)
- `ThrottlingException` - Rate limit exceeded (retryable)
- `ValidationException` - Invalid request parameters (not retryable)
- `ModelNotReadyException` - Model not available in region (not retryable)
- `ModelTimeoutException` - Model inference timeout (retryable)
- `ServiceQuotaExceededException` - Account quota exceeded (not retryable)

**Error Handling Flow**:
1. AWS SDK returns typed error
2. Adapter wraps in `BedrockError` with context
3. If `Retryable == true` and attempts < MaxRetries, retry with exponential backoff
4. If `Retryable == true` and FallbackRegions configured, try next region
5. If all retries exhausted, return error to caller

---

## Entity Relationships Diagram

```
BedrockConfig (configuration)
    ↓ (validated at init)
BedrockAdapter (stateless)
    ↓ (uses)
SchemaTranslator (strategy interface)
    ↓ (implementations)
├── ClaudeSchemaTranslator
├── LlamaSchemaTranslator
├── TitanSchemaTranslator
└── MistralSchemaTranslator
    ↓ (translates to/from)
Bedrock API Requests/Responses
    ↓ (returns)
ChatOut (LangGraph standard format)
```

---

## Validation Summary

**BedrockConfig Validation** (at initialization):
1. Region is valid AWS region
2. ModelID matches expected format
3. Temperature in range [0.0, 1.0]
4. MaxTokens > 0 and <= model max
5. TopP in range [0.0, 1.0]
6. FallbackRegions are valid and not duplicates
7. MaxRetries in range [0, 10]

**Runtime Validation** (per request):
1. Messages array is not empty
2. Tools array is empty OR model supports tools (Claude only)
3. Streaming is disabled OR model supports streaming
4. Context has deadline/timeout set

**Error Validation**:
1. AWS SDK errors mapped to BedrockError
2. Non-retryable errors return immediately
3. Retryable errors trigger retry with backoff
4. All retries exhausted → return error to caller
