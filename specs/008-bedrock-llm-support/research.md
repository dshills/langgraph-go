# AWS Bedrock Go Adapter Research

**Date**: 2025-11-14
**Purpose**: Research AWS Bedrock API for implementing a Go adapter in langgraph-go

---

## Executive Summary

This document provides comprehensive research findings for implementing an AWS Bedrock adapter in Go. AWS Bedrock provides a unified API for invoking multiple foundation model families (Anthropic Claude, Meta Llama, Amazon Titan, Mistral AI) with built-in support for streaming responses, tool calling, cross-region inference, and automatic retry/throttling handling.

**Key Findings**:
- Use AWS SDK Go v2 (`github.com/aws/aws-sdk-go-v2/service/bedrockruntime`)
- Different model families require different request/response JSON schemas
- Streaming is supported via `InvokeModelWithResponseStream` with Go channels
- Tool calling is available for Claude models via Messages API
- Built-in retry with exponential backoff for throttling exceptions
- Cross-region inference provides automatic failover at no additional cost

---

## 1. Bedrock InvokeModel API Schemas

### 1.1 Anthropic Claude (Claude 3+, Claude 4)

**Decision**: Use Messages API format for all Claude 3+ models
**Rationale**: Messages API is the current standard, supports system prompts, tool use, and multimodal inputs
**Alternatives Considered**: Text Completions API (legacy, deprecated)

#### Request Schema

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Hello world"
        }
      ]
    }
  ],
  "system": "You are a helpful assistant",
  "temperature": 0.7,
  "top_p": 0.9,
  "stop_sequences": ["</response>"],
  "anthropic_beta": ["tool-use-2024-05-16"]
}
```

**Key Fields**:
- `anthropic_version` (required): Always `"bedrock-2023-05-31"`
- `max_tokens` (required): Maximum tokens in response (1-4096)
- `messages` (required): Array of message objects with `role` ("user"/"assistant") and `content`
- `system` (optional): System prompt for context/instructions (Claude 2.1+)
- `anthropic_beta` (optional): Opt-in to beta features like tool use

#### Response Schema

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "model": "claude-3-5-sonnet-20240620",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I help you today?"
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 12,
    "output_tokens": 25
  }
}
```

**Key Fields**:
- `content[0].text`: Primary response text
- `stop_reason`: "end_turn" (natural stop) or "max_tokens" (limit reached) or "tool_use"
- `usage`: Token consumption for billing/monitoring

**Model IDs** (as of 2025):
- `us.anthropic.claude-3-5-sonnet-20240620-v1:0` (Claude 3.5 Sonnet)
- Claude 4 models (via cross-region inference profiles)
- Claude 3 Haiku, Sonnet, Opus variants

**API Endpoint Format**:
```
POST /model/{modelId}/invoke
POST /model/us.anthropic.claude-3-5-sonnet-20240620-v1:0/invoke
```

**Timeout Considerations**:
- Claude 3.7 Sonnet and Claude 4 models: 60-minute timeout
- Recommendation: Increase SDK client timeout from default 1 minute

**Payload Limits**:
- Maximum request payload: 20MB

---

### 1.2 Meta Llama (Llama 2, Llama 3, Llama 3.1, Llama 3.2, Llama 4)

**Decision**: Use prompt-based format with generation parameters
**Rationale**: Llama models use a simpler prompt string rather than structured messages
**Alternatives Considered**: Chat format (not officially supported in Bedrock)

#### Request Schema

```json
{
  "prompt": "You are a helpful assistant.\n\nUser: Hello\nAssistant:",
  "temperature": 0.7,
  "top_p": 0.9,
  "max_gen_len": 256
}
```

**Llama 3.2+ with Images**:
```json
{
  "prompt": "Describe this image",
  "images": [
    "base64_encoded_image_string"
  ],
  "temperature": 0.7,
  "top_p": 0.9,
  "max_gen_len": 256
}
```

**Key Fields**:
- `prompt` (required): Text prompt with conversation history
- `max_gen_len` (optional): Maximum tokens to generate (default: 256)
- `temperature` (optional): 0.0-1.0, controls randomness (default: 0.7)
- `top_p` (optional): Nucleus sampling threshold (default: 0.9)
- `images` (optional): Array of base64-encoded images (Llama 3.2+)

#### Response Schema

```json
{
  "generation": "Hello! How can I assist you today?",
  "prompt_token_count": 45,
  "generation_token_count": 12,
  "stop_reason": "stop"
}
```

**Key Fields**:
- `generation`: Generated text
- `prompt_token_count`: Input tokens consumed
- `generation_token_count`: Output tokens generated
- `stop_reason`: "stop" (natural) or "length" (max_gen_len reached)

**Model Families**:
- `meta.llama2-*` (Llama 2)
- `meta.llama3-*` (Llama 3, 3.1, 3.2)
- `meta.llama4-*` (Llama 4)

**Streaming Support**:
- **Llama 4 Instruct**: No streaming support (cannot use `InvokeModelWithResponseStream`)
- **Llama 3.x**: Streaming supported

---

### 1.3 Amazon Titan (Titan Text, Titan Embeddings)

**Decision**: Use Titan-specific request format with structured parameters
**Rationale**: Titan models have their own schema distinct from other providers
**Alternatives Considered**: N/A (proprietary Amazon format)

#### Request Schema (Titan Text)

```json
{
  "inputText": "Describe the purpose of a database index",
  "textGenerationConfig": {
    "maxTokenCount": 256,
    "temperature": 0.7,
    "topP": 0.9,
    "stopSequences": ["END"]
  }
}
```

**Key Fields**:
- `inputText` (required): Input prompt
- `textGenerationConfig` (required): Generation parameters object
  - `maxTokenCount`: Maximum tokens (default: 256)
  - `temperature`: 0.0-1.0 (default: 0.7)
  - `topP`: Nucleus sampling (default: 0.9)
  - `stopSequences`: Array of strings to stop generation

#### Response Schema

```json
{
  "inputTextTokenCount": 8,
  "results": [
    {
      "tokenCount": 120,
      "outputText": "A database index is...",
      "completionReason": "FINISH"
    }
  ]
}
```

**Key Fields**:
- `results[0].outputText`: Generated text
- `results[0].completionReason`: "FINISH" (natural) or "LENGTH" (limit)
- `inputTextTokenCount`: Input tokens consumed

**Model IDs**:
- `amazon.titan-text-express-v1`
- `amazon.titan-text-lite-v1`
- `amazon.titan-embed-text-v1` (embeddings)

---

### 1.4 Mistral AI Models

**Decision**: Use Mistral prompt format with standard parameters
**Rationale**: Mistral follows a similar pattern to Llama with prompt strings
**Alternatives Considered**: N/A

#### Request Schema

```json
{
  "prompt": "<s>[INST] Hello [/INST]",
  "max_tokens": 256,
  "temperature": 0.7,
  "top_p": 0.9,
  "top_k": 50
}
```

**Key Fields**:
- `prompt` (required): Input text with Mistral instruction format
- `max_tokens` (optional): Maximum tokens to generate (default: 256)
- `temperature` (optional): 0.0-1.0 (default: 0.7)
- `top_p` (optional): Nucleus sampling (default: 0.9)
- `top_k` (optional): Top-k sampling (default: 50)

#### Response Schema

```json
{
  "outputs": [
    {
      "text": "Hello! How can I help?",
      "stop_reason": "stop"
    }
  ]
}
```

**Model IDs**:
- `mistral.mistral-7b-instruct-v0:2`
- `mistral.mixtral-8x7b-instruct-v0:1`
- `mistral.mistral-large-*`

**Regional Availability**:
- US East (N. Virginia)
- US West (Oregon)
- EU-West-3 (Paris)

---

## 2. Streaming API (InvokeModelWithResponseStream)

### 2.1 Streaming Support by Model

**Decision**: Check streaming support per-model using `GetFoundationModel` API
**Rationale**: Not all models support streaming; checking prevents runtime errors
**Alternatives Considered**: Assume all models support streaming (risky, causes failures)

| Model Family | Streaming Support |
|-------------|-------------------|
| Claude 3.x  | ✅ Yes |
| Claude 4    | ✅ Yes |
| Llama 3.x   | ✅ Yes |
| Llama 4 Instruct | ❌ No |
| Titan Text  | ✅ Yes |
| Mistral     | ✅ Yes |

**Checking Streaming Support**:
```go
resp, err := client.GetFoundationModel(ctx, &bedrock.GetFoundationModelInput{
    ModelIdentifier: aws.String("us.anthropic.claude-3-5-sonnet-20240620-v1:0"),
})
if resp.ModelDetails.ResponseStreamingSupported {
    // Use InvokeModelWithResponseStream
}
```

---

### 2.2 Go SDK Streaming Implementation

**Decision**: Use Go channels with type switch for event handling
**Rationale**: Go SDK v2 provides idiomatic channel-based streaming API
**Alternatives Considered**: Polling-based approach (less efficient, not idiomatic)

#### Code Example (Claude Streaming)

```go
import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type ClaudeStreamResponse struct {
    Type         string `json:"type"`
    Index        int    `json:"index,omitempty"`
    Delta        struct {
        Type string `json:"type"`
        Text string `json:"text"`
    } `json:"delta,omitempty"`
    ContentBlock struct {
        Type string `json:"type"`
        Text string `json:"text"`
    } `json:"content_block,omitempty"`
}

func processStreamingOutput(
    ctx context.Context,
    output *bedrockruntime.InvokeModelWithResponseStreamOutput,
) (string, error) {
    var combinedResult string

    // Get the event stream channel
    eventStream := output.GetStream()
    defer eventStream.Close()

    // Iterate over streaming events
    for event := range eventStream.Events() {
        switch v := event.(type) {
        case *types.ResponseStreamMemberChunk:
            // Decode JSON chunk
            var resp ClaudeStreamResponse
            err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&resp)
            if err != nil {
                return "", fmt.Errorf("failed to decode chunk: %w", err)
            }

            // Extract text based on event type
            if resp.Type == "content_block_delta" {
                combinedResult += resp.Delta.Text
                // Stream text to client in real-time here
                fmt.Print(resp.Delta.Text)
            }

        case *types.UnknownUnionMember:
            fmt.Printf("Unknown event type: %s\n", v.Tag)

        default:
            fmt.Printf("Unexpected event type: %T\n", v)
        }
    }

    // Check for streaming errors
    if err := eventStream.Err(); err != nil {
        return "", fmt.Errorf("stream error: %w", err)
    }

    return combinedResult, nil
}
```

---

### 2.3 Event Stream Format

**Decision**: Parse events based on model-specific response schemas
**Rationale**: Each model family has different streaming event formats
**Alternatives Considered**: Generic parsing (loses type safety, error-prone)

#### Claude Streaming Events

```json
{"type":"message_start","message":{"id":"msg_01...","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20240620","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":12,"output_tokens":1}}}
{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}
{"type":"content_block_stop","index":0}
{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":25}}
{"type":"message_stop"}
```

**Key Event Types**:
- `message_start`: Initial metadata
- `content_block_start`: Begin content block
- `content_block_delta`: Incremental text chunk
- `content_block_stop`: End content block
- `message_stop`: Complete response

#### Llama/Mistral Streaming Events

```json
{"generation":"Hello","prompt_token_count":10,"generation_token_count":1,"stop_reason":null}
{"generation":" there","prompt_token_count":10,"generation_token_count":2,"stop_reason":null}
{"generation":"!","prompt_token_count":10,"generation_token_count":3,"stop_reason":"stop"}
```

---

### 2.4 Error Handling During Streaming

**Decision**: Monitor both event-level and stream-level errors
**Rationale**: Errors can occur mid-stream; must check `stream.Err()` after iteration
**Alternatives Considered**: Only check final error (misses partial failures)

```go
for event := range eventStream.Events() {
    switch v := event.(type) {
    case *types.ResponseStreamMemberChunk:
        // Process chunk
        if err := processChunk(v); err != nil {
            return fmt.Errorf("chunk processing error: %w", err)
        }
    }
}

// Always check stream-level error after iteration
if err := eventStream.Err(); err != nil {
    return fmt.Errorf("streaming error: %w", err)
}
```

**Common Streaming Errors**:
- `ThrottlingException`: Rate limit exceeded during stream
- `ServiceUnavailableException`: Temporary service outage
- `ModelTimeoutException`: Model exceeded execution timeout
- `context.DeadlineExceeded`: Client-side timeout

---

### 2.5 AWS CLI Limitation

**Important Note**: AWS CLI does not support streaming operations (`InvokeModelWithResponseStream`). Use SDK for streaming.

---

## 3. Tool Calling Support

### 3.1 Model Support for Tool Calling

**Decision**: Implement tool calling only for Claude models
**Rationale**: As of 2025, only Claude models officially support tool use in Bedrock
**Alternatives Considered**: Wait for other model families to add support

| Model Family | Tool Calling Support |
|-------------|---------------------|
| Claude 3.x  | ✅ Yes (Messages API) |
| Claude 4    | ✅ Yes (Enhanced) |
| Llama       | ❌ No |
| Titan       | ❌ No |
| Mistral     | ❌ No (as of 2025) |

**Supported Claude Models**:
- Claude 3 Sonnet
- Claude 3 Opus
- Claude 3 Haiku
- Claude 3.5 Sonnet
- Claude 4 Sonnet, Opus, Haiku (via inference profiles)

---

### 3.2 Tool Use Request Format

**Decision**: Use Anthropic Messages API tool schema format
**Rationale**: Standard Claude tool format, well-documented and stable
**Alternatives Considered**: Converse API (higher-level but less flexible)

#### Request with Tools

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 512,
  "messages": [
    {
      "role": "user",
      "content": [{"type": "text", "text": "What's the weather in San Francisco?"}]
    }
  ],
  "tools": [
    {
      "name": "get_weather",
      "description": "Get the current weather in a location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {
            "type": "string",
            "description": "The city and state, e.g. San Francisco, CA"
          },
          "unit": {
            "type": "string",
            "enum": ["celsius", "fahrenheit"],
            "description": "The temperature unit"
          }
        },
        "required": ["location"]
      }
    }
  ],
  "tool_choice": {
    "type": "auto"
  }
}
```

**Key Fields**:
- `tools` (array): Tool definitions with name, description, input_schema
- `input_schema`: JSON Schema defining tool parameters
- `tool_choice` (optional): "auto" (default) | "any" (force tool use) | {"type": "tool", "name": "tool_name"}

#### Response with Tool Use

```json
{
  "id": "msg_01...",
  "model": "claude-3-5-sonnet-20240620",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "I'll check the weather for you."
    },
    {
      "type": "tool_use",
      "id": "toolu_01...",
      "name": "get_weather",
      "input": {
        "location": "San Francisco, CA",
        "unit": "fahrenheit"
      }
    }
  ],
  "stop_reason": "tool_use",
  "usage": {
    "input_tokens": 450,
    "output_tokens": 78
  }
}
```

**Key Fields**:
- `content`: Array may contain both `text` and `tool_use` blocks
- `tool_use.id`: Unique identifier for this tool call
- `tool_use.name`: Tool name to invoke
- `tool_use.input`: Structured parameters for tool
- `stop_reason`: "tool_use" indicates tool invocation requested

---

### 3.3 Tool Result Submission

**Decision**: Submit tool results in follow-up message with `tool_result` content blocks
**Rationale**: Claude requires explicit tool result messages to continue conversation
**Alternatives Considered**: Auto-inject results (not supported by API)

#### Submitting Tool Results

```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "max_tokens": 512,
  "messages": [
    {
      "role": "user",
      "content": [{"type": "text", "text": "What's the weather in San Francisco?"}]
    },
    {
      "role": "assistant",
      "content": [
        {
          "type": "text",
          "text": "I'll check the weather for you."
        },
        {
          "type": "tool_use",
          "id": "toolu_01...",
          "name": "get_weather",
          "input": {"location": "San Francisco, CA", "unit": "fahrenheit"}
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "tool_result",
          "tool_use_id": "toolu_01...",
          "content": "{\"temperature\": 68, \"condition\": \"sunny\"}"
        }
      ]
    }
  ],
  "tools": [
    {
      "name": "get_weather",
      "description": "Get the current weather in a location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"},
          "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
        },
        "required": ["location"]
      }
    }
  ]
}
```

**Key Points**:
- Must include original tool definition in `tools` array
- Tool result goes in a `user` role message
- `tool_use_id` must match the original `tool_use.id`
- Content can be string (JSON) or structured data

---

### 3.4 Advanced Tool Features (2025)

#### Fine-Grained Tool Streaming

**Decision**: Use fine-grained streaming for low-latency tool parameter delivery
**Rationale**: Reduces latency for large tool parameters (e.g., code generation, file content)
**Alternatives Considered**: Buffer entire tool call (higher latency)

**Supported Models**:
- Claude Sonnet 4.5
- Claude Haiku 4.5
- Claude Sonnet 4
- Claude Opus 4

**Activation**:
```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "anthropic_beta": ["fine-grained-tool-streaming-2025-05-14"],
  "messages": [...],
  "tools": [...]
}
```

**Benefits**:
- Stream tool parameters without buffering
- No JSON validation required during streaming
- Lower latency for large parameter sets

#### Automatic Tool Call Clearing (Beta)

**Decision**: Enable for long-running multi-turn tool conversations
**Rationale**: Prevents context window overflow in extended tool use sessions
**Alternatives Considered**: Manual context management (error-prone)

**Supported Models**:
- Claude Sonnet 4.5

**Activation**:
```json
{
  "anthropic_version": "bedrock-2023-05-31",
  "anthropic_beta": ["context-management-2025-06-27"],
  "messages": [...],
  "tools": [...]
}
```

**Behavior**:
- Automatically removes old `tool_result` blocks when approaching token limit
- Preserves most recent and semantically important tool calls
- Enables unlimited tool use turns without manual pruning

#### Memory Tool (Claude Sonnet 4.5)

**Note**: Memory tool provides persistent context across conversations but requires external storage integration. Not directly applicable to stateless `ChatModel` interface.

---

### 3.5 Converse API Alternative

**Decision**: Do not use Converse API for initial implementation
**Rationale**: Messages API provides more control; Converse API abstracts too much
**Alternatives Considered**: Use Converse API (future enhancement)

**Converse API Benefits**:
- Unified API across all Bedrock models that support tools
- Simplified request format
- Automatic tool result handling

**Converse API Drawbacks**:
- Less granular control over request format
- Newer API (less mature)
- Not all models support Converse API

**Recommendation**: Implement Messages API first, consider Converse API as future enhancement

---

## 4. AWS SDK for Go v2 Best Practices

### 4.1 Credential Loading Patterns

**Decision**: Use `config.LoadDefaultConfig()` for automatic credential chain resolution
**Rationale**: Follows AWS best practices, supports multiple credential sources seamlessly
**Alternatives Considered**: Manual credential provider (more complex, less secure)

#### Recommended Credential Precedence

1. **IAM Roles for ECS Tasks** (production ECS workloads)
   - Automatically provided by ECS task execution role
   - No manual credential management required

2. **IAM Roles for EC2 Instances** (production EC2 workloads)
   - Automatically provided by instance profile
   - SDK detects and uses instance credentials

3. **Environment Variables** (development, CI/CD)
   ```bash
   export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
   export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
   export AWS_SESSION_TOKEN=FwoGZXIvYXdzEL...  # Optional for STS temporary credentials
   export AWS_REGION=us-east-1
   ```

4. **Shared Credentials File** (local development)
   - File: `~/.aws/credentials`
   ```ini
   [default]
   aws_access_key_id = AKIAIOSFODNN7EXAMPLE
   aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

   [bedrock-prod]
   aws_access_key_id = AKIAIOSFODNN7ANOTHER
   aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxAnotherKey
   ```

5. **Shared Config File** (local development with profiles)
   - File: `~/.aws/config`
   ```ini
   [default]
   region = us-east-1

   [profile bedrock-prod]
   region = us-west-2
   role_arn = arn:aws:iam::123456789012:role/BedrockAccessRole
   source_profile = default
   ```

#### Code Example: Default Config Loading

```go
import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

func NewBedrockClient(ctx context.Context, region string) (*bedrockruntime.Client, error) {
    // Load default config (automatic credential chain)
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(region),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    // Create Bedrock Runtime client
    client := bedrockruntime.NewFromConfig(cfg)
    return client, nil
}
```

---

### 4.2 Static Credentials (When Necessary)

**Decision**: Avoid static credentials; use only when no alternative exists
**Rationale**: Security best practice, prevents credential leakage in code
**Alternatives Considered**: Environment variables (preferred over hardcoded)

#### Code Example: Static Credentials

```go
import (
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
)

// AVOID THIS IN PRODUCTION - Use IAM roles or environment variables instead
func NewClientWithStaticCreds(ctx context.Context, region, accessKey, secretKey string) (*bedrockruntime.Client, error) {
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(region),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            accessKey,
            secretKey,
            "",  // Session token (empty for long-term credentials)
        )),
    )
    if err != nil {
        return nil, err
    }

    return bedrockruntime.NewFromConfig(cfg), nil
}
```

**When to Use Static Credentials**:
- Local development/testing only
- CI/CD pipelines without IAM role support
- Third-party integrations requiring explicit credentials

**Security Warning**: Never hardcode credentials in source code. Use environment variables or secret management services (AWS Secrets Manager, HashiCorp Vault).

---

### 4.3 Region Configuration

**Decision**: Require explicit region specification in adapter constructor
**Rationale**: Bedrock has no default region; explicit region prevents runtime errors
**Alternatives Considered**: Fallback to us-east-1 (implicit behavior, error-prone)

#### Region Configuration Methods

1. **Explicit Region in Code** (recommended)
   ```go
   cfg, err := config.LoadDefaultConfig(ctx,
       config.WithRegion("us-west-2"),
   )
   ```

2. **Environment Variable**
   ```bash
   export AWS_REGION=us-west-2
   # Or
   export AWS_DEFAULT_REGION=us-west-2
   ```

3. **Shared Config File**
   ```ini
   [default]
   region = us-west-2
   ```

#### Region Precedence (highest to lowest)

1. Explicit `config.WithRegion()` in code
2. `AWS_REGION` environment variable
3. `AWS_DEFAULT_REGION` environment variable
4. Shared config file (`~/.aws/config`)

**Recommendation**: Pass region as parameter to adapter constructor, allow override via environment variables.

---

### 4.4 Error Handling Patterns

**Decision**: Use `errors.As()` for typed error handling with modeled AWS errors
**Rationale**: Type-safe error handling, enables specific recovery strategies
**Alternatives Considered**: String matching (brittle, unreliable)

#### Code Example: Typed Error Handling

```go
import (
    "errors"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/aws/smithy-go"
)

func handleBedrockError(err error) error {
    if err == nil {
        return nil
    }

    // Check for API errors
    var apiErr smithy.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.ErrorCode() {
        case "ThrottlingException":
            // Rate limit exceeded - implement backoff/retry
            return fmt.Errorf("bedrock rate limit: %w", err)

        case "ServiceUnavailableException":
            // Temporary outage - retry with exponential backoff
            return fmt.Errorf("bedrock unavailable: %w", err)

        case "ValidationException":
            // Invalid input - do not retry
            return fmt.Errorf("invalid request: %w", err)

        case "ModelTimeoutException":
            // Model exceeded execution time - do not retry
            return fmt.Errorf("model timeout: %w", err)

        case "ServiceQuotaExceededException":
            // Account quota exceeded - alert/escalate
            return fmt.Errorf("quota exceeded: %w", err)

        default:
            return fmt.Errorf("bedrock api error (%s): %w", apiErr.ErrorCode(), err)
        }
    }

    // Check for context errors
    if errors.Is(err, context.DeadlineExceeded) {
        return fmt.Errorf("client timeout: %w", err)
    }
    if errors.Is(err, context.Canceled) {
        return fmt.Errorf("request canceled: %w", err)
    }

    return fmt.Errorf("bedrock error: %w", err)
}
```

---

### 4.5 Context Cancellation and Timeout Handling

**Decision**: Always pass context with timeout to SDK operations
**Rationale**: Prevents indefinite hangs, enables graceful cancellation
**Alternatives Considered**: No timeout (risky in production)

#### Code Example: Context Timeout

```go
import (
    "context"
    "time"
)

func invokeWithTimeout(client *bedrockruntime.Client, modelID string, payload []byte) error {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()

    // Invoke model with timeout context
    resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(modelID),
        ContentType: aws.String("application/json"),
        Accept:      aws.String("application/json"),
        Body:        payload,
    })

    if err != nil {
        // Context cancellation handling
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("request timed out after 60s: %w", err)
        }
        if errors.Is(err, context.Canceled) {
            return fmt.Errorf("request canceled: %w", err)
        }
        return handleBedrockError(err)
    }

    // Process response...
    return nil
}
```

**Timeout Recommendations**:
- Claude 3.x models: 30-60 seconds
- Claude 4 models: 60-120 seconds (longer timeout support)
- Llama models: 30-60 seconds
- Streaming requests: 120-300 seconds (longer conversations)

**Important**: SDK will not retry requests if context is canceled. Ensure timeouts account for retry attempts.

---

### 4.6 Retry Strategies for Throttling

**Decision**: Use AWS SDK's built-in adaptive retry mode for automatic throttling handling
**Rationale**: SDK provides production-tested exponential backoff with jitter
**Alternatives Considered**: Custom retry logic (reinventing the wheel)

#### Retry Modes

1. **Standard Retry Mode** (default)
   - Rate-limited retry attempts
   - Exponential backoff with jitter
   - Max 3 attempts by default

2. **Adaptive Retry Mode** (recommended for production)
   - Standard mode features +
   - Client-side rate limiting on throttles
   - Token bucket algorithm shared across all requests
   - Experimental (subject to change)

#### Code Example: Adaptive Retry Mode

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws/retry"
    "github.com/aws/aws-sdk-go-v2/config"
)

func NewClientWithAdaptiveRetry(ctx context.Context, region string) (*bedrockruntime.Client, error) {
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(region),
        config.WithRetryMode(aws.RetryModeAdaptive),
        config.WithRetryMaxAttempts(5),  // Increase max attempts
    )
    if err != nil {
        return nil, err
    }

    return bedrockruntime.NewFromConfig(cfg), nil
}
```

#### Custom Retry Configuration

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/aws/retry"
    "time"
)

func NewClientWithCustomRetry(ctx context.Context, region string) (*bedrockruntime.Client, error) {
    // Create custom retryer
    retryer := retry.NewStandard(func(opts *retry.StandardOptions) {
        opts.MaxAttempts = 5
        opts.MaxBackoff = 30 * time.Second  // Cap backoff at 30s
    })

    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(region),
        config.WithRetryer(func() aws.Retryer {
            return retryer
        }),
    )
    if err != nil {
        return nil, err
    }

    return bedrockruntime.NewFromConfig(cfg), nil
}
```

#### Throttling Error Codes

The SDK automatically retries these error codes:
- `Throttling`
- `ThrottlingException`
- `ThrottledException`
- `RequestThrottledException`
- `TooManyRequestsException`
- `ProvisionedThroughputExceededException`

#### Best Practices for Throttling

1. **Exponential Backoff with Jitter**
   - SDK implements this automatically
   - Prevents thundering herd problem
   - Jitter randomizes retry timing

2. **Token Bucket for Adaptive Mode**
   - Shared across all requests for a client
   - Reduces overall request rate when throttled
   - Automatically restores capacity when throttling stops

3. **Monitor Throttle Metrics**
   - Log throttling events for capacity planning
   - Alert on sustained throttling (indicates quota issues)
   - Request quota increases if consistently throttled

4. **Per-Minute Quota Handling**
   - Bedrock has per-minute quotas
   - Ensure backoff lasts at least 1 full minute when hitting per-minute limits
   - SDK handles this automatically with adaptive mode

---

## 5. Multi-Region Patterns

### 5.1 Cross-Region Inference (Built-in Failover)

**Decision**: Use Bedrock's cross-region inference profiles for automatic multi-region failover
**Rationale**: No additional cost, AWS-managed failover, no custom logic required
**Alternatives Considered**: Custom multi-region retry logic (complex, error-prone)

#### How Cross-Region Inference Works

1. **Request Routing**:
   - Client sends request to source region (e.g., us-east-1)
   - Bedrock checks capacity in source region
   - If insufficient capacity, automatically routes to destination regions

2. **Region Selection**:
   - Real-time capacity evaluation across configured regions
   - Automatic failover to regions with available capacity
   - Transparent to client (single API call)

3. **Cost Model**:
   - Pay source region pricing only
   - No additional cost for cross-region routing
   - No data transfer charges
   - No encryption surcharges

4. **Data Residency**:
   - Input prompts and output results may move across regions during inference
   - Data stored only in source region
   - All data transmission encrypted over AWS private network

#### Inference Profile Types

**1. Geography-Specific Inference Profiles**

```go
// US-specific cross-region inference profile
modelID := "us.anthropic.claude-3-5-sonnet-20240620-v1:0"

// EU-specific cross-region inference profile
modelID := "eu.anthropic.claude-3-5-sonnet-20240620-v1:0"
```

**Characteristics**:
- Requests stay within geographic boundary (e.g., US or EU)
- Multiple destination regions within geography
- Compliance-friendly (data residency requirements)

**2. Global Inference Profiles**

```go
// Global cross-region inference profile (all commercial regions)
modelID := "global.anthropic.claude-sonnet-4-20250514-v1:0"
```

**Characteristics**:
- Routes to any commercial AWS region worldwide
- Optimizes for lowest latency and highest availability
- Currently only supported for Claude Sonnet 4 (as of 2025)

**Supported Source Regions for Global Profiles**:
- US West (Oregon)
- US East (N. Virginia)
- US East (Ohio)
- Europe (Ireland)
- Asia Pacific (Tokyo)

---

### 5.2 Inference Profile Usage

**Decision**: Allow users to specify either model ID or inference profile ARN
**Rationale**: Flexibility for different use cases (single-region vs multi-region)
**Alternatives Considered**: Force inference profiles (too opinionated)

#### Using Inference Profiles

```go
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

func invokeWithInferenceProfile(client *bedrockruntime.Client, inferenceProfileID string, payload []byte) error {
    resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(inferenceProfileID),  // Use inference profile ID
        ContentType: aws.String("application/json"),
        Accept:      aws.String("application/json"),
        Body:        payload,
    })
    // ...
}

// Examples:
// Regional model: "anthropic.claude-3-5-sonnet-20240620-v1:0"
// US cross-region: "us.anthropic.claude-3-5-sonnet-20240620-v1:0"
// EU cross-region: "eu.anthropic.claude-3-5-sonnet-20240620-v1:0"
// Global: "global.anthropic.claude-sonnet-4-20250514-v1:0"
```

#### Inference Profile ARN Support

```go
// Can also use full ARN
inferenceProfileARN := "arn:aws:bedrock:us-east-1:123456789012:inference-profile/us.anthropic.claude-3-5-sonnet-20240620-v1:0"
```

**Converse API**: Supports both ID and ARN
**InvokeModel API**: Supports inference profile ID only (not ARN)

---

### 5.3 Monitoring Cross-Region Routing

**Decision**: Parse CloudTrail logs and Model Invocation Logs for actual inference region
**Rationale**: Essential for debugging, cost attribution, and compliance auditing
**Alternatives Considered**: Assume source region (incorrect for routed requests)

#### CloudTrail Event with Inference Region

```json
{
  "eventName": "InvokeModel",
  "requestParameters": {
    "modelId": "us.anthropic.claude-3-5-sonnet-20240620-v1:0"
  },
  "additionalEventData": {
    "inferenceRegion": "us-west-2"
  },
  "awsRegion": "us-east-1"
}
```

**Key Fields**:
- `awsRegion`: Source region (where request was made)
- `inferenceRegion`: Destination region (where inference ran)
- If `inferenceRegion` differs from `awsRegion`, request was routed

#### Model Invocation Logs

Enable Bedrock Model Invocation Logging to S3 or CloudWatch:
- Contains inference region for each request
- Useful for cost analysis and performance metrics
- Required for compliance auditing

---

### 5.4 Service Control Policies (SCPs) and IAM

**Decision**: Ensure IAM policies allow inference in all destination regions
**Rationale**: Cross-region inference fails if any destination region is blocked
**Alternatives Considered**: Region-specific policies (breaks cross-region routing)

#### IAM Policy for Cross-Region Inference

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:us-east-1::foundation-model/*",
        "arn:aws:bedrock:us-west-2::foundation-model/*",
        "arn:aws:bedrock:eu-west-1::foundation-model/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:GetFoundationModel"
      ],
      "Resource": "*"
    }
  ]
}
```

**Important**: If any destination region in an inference profile is blocked by SCPs or IAM policies, the entire request will fail even if other regions remain accessible.

---

### 5.5 Regional Model Availability

**Decision**: Provide model availability lookup helper function
**Rationale**: Models are not available in all regions; adapter should help users choose regions
**Alternatives Considered**: Ignore availability (causes runtime errors)

#### Model Availability by Region (2025)

| Model Family | US East (N. Virginia) | US West (Oregon) | EU (Ireland) | EU (Paris) | AP (Tokyo) |
|-------------|:---------------------:|:----------------:|:------------:|:----------:|:----------:|
| Claude 3.x  | ✅ | ✅ | ✅ | ❌ | ✅ |
| Claude 4    | ✅ (inference profile) | ✅ (inference profile) | ✅ (inference profile) | ❌ | ✅ (inference profile) |
| Llama 2/3   | ✅ | ✅ | ✅ | ✅ | ✅ |
| Llama 4     | ✅ | ✅ | ✅ | ❌ | ✅ |
| Titan Text  | ✅ | ✅ | ✅ | ❌ | ✅ |
| Mistral     | ✅ | ✅ | ❌ | ✅ | ❌ |

**Note**: Availability changes frequently. Use `bedrock:ListFoundationModels` API to check current availability.

#### Code Example: Check Model Availability

```go
import (
    "github.com/aws/aws-sdk-go-v2/service/bedrock"
    "github.com/aws/aws-sdk-go-v2/aws"
)

func isModelAvailableInRegion(ctx context.Context, client *bedrock.Client, modelID string) (bool, error) {
    resp, err := client.ListFoundationModels(ctx, &bedrock.ListFoundationModelsInput{
        ByOutputModality: []types.ModelModality{types.ModelModalityText},
    })
    if err != nil {
        return false, err
    }

    for _, model := range resp.ModelSummaries {
        if aws.ToString(model.ModelId) == modelID {
            return true, nil
        }
    }

    return false, nil
}
```

---

### 5.6 Custom Multi-Region Failover (Alternative)

**Decision**: Do not implement custom multi-region failover in initial version
**Rationale**: Cross-region inference profiles handle this better with no additional cost
**Alternatives Considered**: Custom failover (complex, unnecessary with inference profiles)

#### When to Consider Custom Failover

- **Compliance Requirements**: Data cannot leave specific regions
- **Cost Optimization**: Prefer specific regions for pricing
- **Latency Optimization**: Route based on client location

#### Custom Failover Pattern (Reference)

```go
type MultiRegionClient struct {
    clients map[string]*bedrockruntime.Client
    regions []string
}

func (m *MultiRegionClient) InvokeWithFailover(ctx context.Context, input *bedrockruntime.InvokeModelInput) (*bedrockruntime.InvokeModelOutput, error) {
    var lastErr error

    for _, region := range m.regions {
        client := m.clients[region]

        resp, err := client.InvokeModel(ctx, input)
        if err == nil {
            return resp, nil
        }

        // Only retry on transient errors
        var apiErr smithy.APIError
        if errors.As(err, &apiErr) {
            switch apiErr.ErrorCode() {
            case "ThrottlingException", "ServiceUnavailableException":
                lastErr = err
                continue  // Try next region
            default:
                return nil, err  // Non-retryable error
            }
        }

        lastErr = err
    }

    return nil, fmt.Errorf("all regions failed: %w", lastErr)
}
```

**Recommendation**: Use cross-region inference profiles instead of custom failover for production.

---

## 6. Implementation Recommendations

### 6.1 Adapter Interface Design

**Decision**: Create model-specific adapter implementations behind unified `ChatModel` interface
**Rationale**: Different models require different request/response handling
**Alternatives Considered**: Single generic adapter (loses type safety, complex)

#### Proposed Package Structure

```
/graph
  /model
    chat.go              // ChatModel interface
    bedrock/
      client.go          // Shared Bedrock client wrapper
      claude.go          // Claude-specific adapter
      llama.go           // Llama-specific adapter
      titan.go           // Titan-specific adapter
      mistral.go         // Mistral-specific adapter
      types.go           // Common types (ToolSpec, etc.)
      errors.go          // Error handling utilities
```

#### Adapter Constructor Pattern

```go
package bedrock

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type ClaudeAdapter struct {
    client  *bedrockruntime.Client
    modelID string
    options AdapterOptions
}

type AdapterOptions struct {
    Temperature       float64
    MaxTokens         int
    EnableStreaming   bool
    InferenceProfile  string  // Optional: use cross-region inference
}

func NewClaudeAdapter(ctx context.Context, region, modelID string, opts AdapterOptions) (*ClaudeAdapter, error) {
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(region),
        config.WithRetryMode(aws.RetryModeAdaptive),
        config.WithRetryMaxAttempts(5),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    client := bedrockruntime.NewFromConfig(cfg)

    return &ClaudeAdapter{
        client:  client,
        modelID: modelID,
        options: opts,
    }, nil
}
```

---

### 6.2 Error Handling Strategy

**Decision**: Wrap AWS errors in domain-specific error types for adapter consumers
**Rationale**: Abstracts AWS SDK details, enables unified error handling across providers
**Alternatives Considered**: Expose raw AWS errors (leaks implementation details)

#### Domain Error Types

```go
package bedrock

import "errors"

var (
    ErrRateLimitExceeded    = errors.New("bedrock: rate limit exceeded")
    ErrModelTimeout         = errors.New("bedrock: model execution timeout")
    ErrServiceUnavailable   = errors.New("bedrock: service temporarily unavailable")
    ErrInvalidInput         = errors.New("bedrock: invalid request parameters")
    ErrQuotaExceeded        = errors.New("bedrock: account quota exceeded")
    ErrModelNotFound        = errors.New("bedrock: model not found in region")
)

func mapAWSError(err error) error {
    if err == nil {
        return nil
    }

    var apiErr smithy.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.ErrorCode() {
        case "ThrottlingException":
            return fmt.Errorf("%w: %v", ErrRateLimitExceeded, err)
        case "ModelTimeoutException":
            return fmt.Errorf("%w: %v", ErrModelTimeout, err)
        case "ServiceUnavailableException":
            return fmt.Errorf("%w: %v", ErrServiceUnavailable, err)
        case "ValidationException":
            return fmt.Errorf("%w: %v", ErrInvalidInput, err)
        case "ServiceQuotaExceededException":
            return fmt.Errorf("%w: %v", ErrQuotaExceeded, err)
        case "ResourceNotFoundException":
            return fmt.Errorf("%w: %v", ErrModelNotFound, err)
        }
    }

    return fmt.Errorf("bedrock error: %w", err)
}
```

---

### 6.3 Streaming Support

**Decision**: Implement streaming via callback function in `ChatModel.Chat()` method
**Rationale**: Enables real-time token delivery without changing interface signature
**Alternatives Considered**: Separate streaming method (diverges from other adapters)

#### Streaming Callback Pattern

```go
package model

type StreamCallback func(ctx context.Context, token string) error

type ChatOptions struct {
    Temperature    float64
    MaxTokens      int
    StreamCallback StreamCallback  // Optional: enable streaming
}

type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec, opts ChatOptions) (ChatOut, error)
}
```

#### Claude Streaming Implementation

```go
func (a *ClaudeAdapter) Chat(ctx context.Context, messages []Message, tools []ToolSpec, opts ChatOptions) (ChatOut, error) {
    if opts.StreamCallback != nil {
        return a.chatWithStreaming(ctx, messages, tools, opts)
    }
    return a.chatWithoutStreaming(ctx, messages, tools, opts)
}

func (a *ClaudeAdapter) chatWithStreaming(ctx context.Context, messages []Message, tools []ToolSpec, opts ChatOptions) (ChatOut, error) {
    payload := a.buildClaudeRequest(messages, tools, opts)

    resp, err := a.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
        ModelId:     aws.String(a.modelID),
        ContentType: aws.String("application/json"),
        Accept:      aws.String("application/json"),
        Body:        payload,
    })
    if err != nil {
        return ChatOut{}, mapAWSError(err)
    }

    var fullText string
    eventStream := resp.GetStream()
    defer eventStream.Close()

    for event := range eventStream.Events() {
        switch v := event.(type) {
        case *types.ResponseStreamMemberChunk:
            var chunk ClaudeStreamResponse
            if err := json.Unmarshal(v.Value.Bytes, &chunk); err != nil {
                return ChatOut{}, fmt.Errorf("failed to parse chunk: %w", err)
            }

            if chunk.Type == "content_block_delta" && chunk.Delta.Text != "" {
                fullText += chunk.Delta.Text

                // Call streaming callback
                if err := opts.StreamCallback(ctx, chunk.Delta.Text); err != nil {
                    return ChatOut{}, fmt.Errorf("streaming callback error: %w", err)
                }
            }
        }
    }

    if err := eventStream.Err(); err != nil {
        return ChatOut{}, mapAWSError(err)
    }

    return ChatOut{
        Text:       fullText,
        FinishReason: "end_turn",
    }, nil
}
```

---

### 6.4 Tool Calling Integration

**Decision**: Implement tool calling for Claude adapters only initially
**Rationale**: Only Claude supports tool use in Bedrock as of 2025
**Alternatives Considered**: Wait for other models (delays feature availability)

#### Tool Definition Conversion

```go
func convertToolSpecToClaudeTool(spec ToolSpec) claudeTool {
    return claudeTool{
        Name:        spec.Name,
        Description: spec.Description,
        InputSchema: map[string]interface{}{
            "type":       "object",
            "properties": spec.Parameters,
            "required":   spec.Required,
        },
    }
}
```

#### Tool Use Detection and Handling

```go
func (a *ClaudeAdapter) Chat(ctx context.Context, messages []Message, tools []ToolSpec, opts ChatOptions) (ChatOut, error) {
    // ... invoke model ...

    var resp ClaudeResponse
    if err := json.Unmarshal(respBody, &resp); err != nil {
        return ChatOut{}, err
    }

    // Extract tool calls from response
    var toolCalls []ToolCall
    for _, content := range resp.Content {
        if content.Type == "tool_use" {
            toolCalls = append(toolCalls, ToolCall{
                ID:   content.ID,
                Name: content.Name,
                Args: content.Input,
            })
        }
    }

    return ChatOut{
        Text:         extractTextFromContent(resp.Content),
        FinishReason: resp.StopReason,
        ToolCalls:    toolCalls,
    }, nil
}
```

---

### 6.5 Configuration and Testing

**Decision**: Support multiple configuration methods with sensible defaults
**Rationale**: Flexibility for different deployment environments
**Alternatives Considered**: Single configuration method (inflexible)

#### Configuration Options

```go
type BedrockConfig struct {
    // Required
    Region  string
    ModelID string

    // Optional - Credentials (defaults to AWS credential chain)
    AccessKeyID     string
    SecretAccessKey string
    SessionToken    string

    // Optional - Generation parameters
    Temperature float64  // Default: 0.7
    MaxTokens   int      // Default: 1024
    TopP        float64  // Default: 0.9

    // Optional - Performance tuning
    RetryMaxAttempts int           // Default: 5
    RetryMode        string         // "standard" or "adaptive", default: "adaptive"
    Timeout          time.Duration  // Default: 60s

    // Optional - Advanced features
    InferenceProfile string  // Use cross-region inference profile
    EnableStreaming  bool    // Default: false
}

func (c *BedrockConfig) Validate() error {
    if c.Region == "" {
        return errors.New("region is required")
    }
    if c.ModelID == "" {
        return errors.New("modelID is required")
    }
    return nil
}

func (c *BedrockConfig) WithDefaults() *BedrockConfig {
    if c.Temperature == 0 {
        c.Temperature = 0.7
    }
    if c.MaxTokens == 0 {
        c.MaxTokens = 1024
    }
    if c.TopP == 0 {
        c.TopP = 0.9
    }
    if c.RetryMaxAttempts == 0 {
        c.RetryMaxAttempts = 5
    }
    if c.RetryMode == "" {
        c.RetryMode = "adaptive"
    }
    if c.Timeout == 0 {
        c.Timeout = 60 * time.Second
    }
    return c
}
```

---

### 6.6 Testing Strategy

**Decision**: Use AWS SDK mocking for unit tests, real API for integration tests
**Rationale**: Fast unit tests, confidence from integration tests
**Alternatives Considered**: Only integration tests (slow, expensive)

#### Unit Test with Mock

```go
import (
    "testing"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type mockBedrockClient struct {
    InvokeModelFunc func(ctx context.Context, input *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

func (m *mockBedrockClient) InvokeModel(ctx context.Context, input *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
    return m.InvokeModelFunc(ctx, input, optFns...)
}

func TestClaudeAdapter_Chat(t *testing.T) {
    mockClient := &mockBedrockClient{
        InvokeModelFunc: func(ctx context.Context, input *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error) {
            // Return mock response
            respJSON := `{
                "id": "msg_test",
                "type": "message",
                "role": "assistant",
                "content": [{"type": "text", "text": "Hello!"}],
                "stop_reason": "end_turn",
                "usage": {"input_tokens": 10, "output_tokens": 5}
            }`
            return &bedrockruntime.InvokeModelOutput{
                Body: []byte(respJSON),
            }, nil
        },
    }

    adapter := &ClaudeAdapter{client: mockClient}

    out, err := adapter.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}}, nil, ChatOptions{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if out.Text != "Hello!" {
        t.Errorf("expected 'Hello!', got '%s'", out.Text)
    }
}
```

#### Integration Test Configuration

```go
// integration_test.go
// +build integration

func TestClaudeIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    region := os.Getenv("AWS_REGION")
    if region == "" {
        t.Skip("AWS_REGION not set")
    }

    adapter, err := NewClaudeAdapter(context.Background(), region, "anthropic.claude-3-5-sonnet-20240620-v1:0", AdapterOptions{})
    if err != nil {
        t.Fatalf("failed to create adapter: %v", err)
    }

    out, err := adapter.Chat(context.Background(), []Message{{Role: "user", Content: "Say hello"}}, nil, ChatOptions{})
    if err != nil {
        t.Fatalf("chat failed: %v", err)
    }

    if out.Text == "" {
        t.Error("expected non-empty response")
    }
}
```

---

## 7. Security Considerations

### 7.1 Credential Management

**Best Practices**:
1. Never hardcode credentials in source code
2. Use IAM roles whenever possible (ECS, EC2, Lambda)
3. Rotate credentials regularly
4. Use least-privilege IAM policies
5. Enable AWS CloudTrail for audit logging

**IAM Policy Example** (Least Privilege):
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-*",
        "arn:aws:bedrock:us-west-2::foundation-model/anthropic.claude-*"
      ],
      "Condition": {
        "StringEquals": {
          "aws:RequestedRegion": ["us-east-1", "us-west-2"]
        }
      }
    }
  ]
}
```

---

### 7.2 Data Privacy

**Considerations**:
1. **Data in Transit**: All Bedrock API calls use HTTPS/TLS encryption
2. **Data at Rest**: Bedrock does not store customer prompts or responses (zero data retention for on-demand inference)
3. **Cross-Region Inference**: Data may transit across AWS regions but remains encrypted
4. **Logging**: Disable model invocation logging if prompts contain sensitive data
5. **Compliance**: Bedrock is HIPAA eligible, SOC 1/2/3, ISO 27001 certified

**Disabling Model Invocation Logging**:
```go
// Do not enable model invocation logging for sensitive workloads
// Logging is opt-in; no action required to disable
```

---

### 7.3 Rate Limiting and Cost Control

**Best Practices**:
1. Set up CloudWatch alarms for API call volume
2. Implement client-side rate limiting for high-volume workloads
3. Use cross-region inference profiles to distribute load
4. Monitor token usage to predict costs
5. Set up AWS Budgets alerts for cost overruns

**CloudWatch Metric Example**:
```
Namespace: AWS/Bedrock
Metric: Invocations
Dimensions: ModelId
```

---

## 8. Performance Optimization

### 8.1 Connection Pooling

**Decision**: Reuse `bedrockruntime.Client` instances across requests
**Rationale**: SDK client maintains HTTP connection pool; creating new clients is expensive
**Alternatives Considered**: Create client per request (high overhead)

```go
// Good: Reuse client
var globalClient *bedrockruntime.Client

func init() {
    cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
    globalClient = bedrockruntime.NewFromConfig(cfg)
}

// Bad: Create client per request
func invokeModel() {
    cfg, _ := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
    client := bedrockruntime.NewFromConfig(cfg)  // Expensive!
    client.InvokeModel(...)
}
```

---

### 8.2 Request Batching

**Note**: Bedrock does not support batch inference for on-demand models. Each request is independent.

For high-throughput workloads, use:
1. Concurrent requests with semaphore limiting
2. Cross-region inference profiles
3. Provisioned throughput (if available for model)

---

### 8.3 Caching Strategies

**Decision**: Do not cache model responses in adapter
**Rationale**: Caching is application-specific; adapter should remain stateless
**Alternatives Considered**: Built-in caching (violates single responsibility)

**Recommendation**: Implement caching at application layer if needed (e.g., Redis, in-memory cache).

---

## 9. Example Code

### 9.1 Complete Claude Adapter Example

```go
package bedrock

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type ClaudeAdapter struct {
    client  *bedrockruntime.Client
    modelID string
    config  ClaudeConfig
}

type ClaudeConfig struct {
    Region       string
    Temperature  float64
    MaxTokens    int
    TopP         float64
    Timeout      time.Duration
}

type claudeRequest struct {
    AnthropicVersion string         `json:"anthropic_version"`
    MaxTokens        int            `json:"max_tokens"`
    Messages         []claudeMessage `json:"messages"`
    Temperature      float64        `json:"temperature,omitempty"`
    TopP             float64        `json:"top_p,omitempty"`
    System           string         `json:"system,omitempty"`
    Tools            []claudeTool   `json:"tools,omitempty"`
}

type claudeMessage struct {
    Role    string                   `json:"role"`
    Content []claudeContentBlock     `json:"content"`
}

type claudeContentBlock struct {
    Type  string                 `json:"type"`
    Text  string                 `json:"text,omitempty"`
    ID    string                 `json:"id,omitempty"`
    Name  string                 `json:"name,omitempty"`
    Input map[string]interface{} `json:"input,omitempty"`
}

type claudeTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    InputSchema map[string]interface{} `json:"input_schema"`
}

type claudeResponse struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"`
    Role       string                 `json:"role"`
    Content    []claudeContentBlock   `json:"content"`
    StopReason string                 `json:"stop_reason"`
    Usage      struct {
        InputTokens  int `json:"input_tokens"`
        OutputTokens int `json:"output_tokens"`
    } `json:"usage"`
}

func NewClaudeAdapter(ctx context.Context, cfg ClaudeConfig) (*ClaudeAdapter, error) {
    if cfg.Region == "" {
        return nil, errors.New("region is required")
    }
    if cfg.Temperature == 0 {
        cfg.Temperature = 0.7
    }
    if cfg.MaxTokens == 0 {
        cfg.MaxTokens = 1024
    }
    if cfg.TopP == 0 {
        cfg.TopP = 0.9
    }
    if cfg.Timeout == 0 {
        cfg.Timeout = 60 * time.Second
    }

    awsCfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(cfg.Region),
        config.WithRetryMode(aws.RetryModeAdaptive),
        config.WithRetryMaxAttempts(5),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    client := bedrockruntime.NewFromConfig(awsCfg)

    return &ClaudeAdapter{
        client:  client,
        modelID: "anthropic.claude-3-5-sonnet-20240620-v1:0",
        config:  cfg,
    }, nil
}

func (a *ClaudeAdapter) Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error) {
    ctx, cancel := context.WithTimeout(ctx, a.config.Timeout)
    defer cancel()

    // Build request
    reqBody := claudeRequest{
        AnthropicVersion: "bedrock-2023-05-31",
        MaxTokens:        a.config.MaxTokens,
        Messages:         a.convertMessages(messages),
        Temperature:      a.config.Temperature,
        TopP:             a.config.TopP,
    }

    if len(tools) > 0 {
        reqBody.Tools = a.convertTools(tools)
    }

    payload, err := json.Marshal(reqBody)
    if err != nil {
        return ChatOut{}, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Invoke model
    resp, err := a.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(a.modelID),
        ContentType: aws.String("application/json"),
        Accept:      aws.String("application/json"),
        Body:        payload,
    })
    if err != nil {
        return ChatOut{}, mapAWSError(err)
    }

    // Parse response
    var claudeResp claudeResponse
    if err := json.Unmarshal(resp.Body, &claudeResp); err != nil {
        return ChatOut{}, fmt.Errorf("failed to unmarshal response: %w", err)
    }

    // Extract text and tool calls
    var text string
    var toolCalls []ToolCall
    for _, content := range claudeResp.Content {
        if content.Type == "text" {
            text += content.Text
        } else if content.Type == "tool_use" {
            toolCalls = append(toolCalls, ToolCall{
                ID:   content.ID,
                Name: content.Name,
                Args: content.Input,
            })
        }
    }

    return ChatOut{
        Text:         text,
        FinishReason: claudeResp.StopReason,
        ToolCalls:    toolCalls,
    }, nil
}

func (a *ClaudeAdapter) convertMessages(messages []Message) []claudeMessage {
    result := make([]claudeMessage, len(messages))
    for i, msg := range messages {
        result[i] = claudeMessage{
            Role: msg.Role,
            Content: []claudeContentBlock{
                {Type: "text", Text: msg.Content},
            },
        }
    }
    return result
}

func (a *ClaudeAdapter) convertTools(tools []ToolSpec) []claudeTool {
    result := make([]claudeTool, len(tools))
    for i, tool := range tools {
        result[i] = claudeTool{
            Name:        tool.Name,
            Description: tool.Description,
            InputSchema: map[string]interface{}{
                "type":       "object",
                "properties": tool.Parameters,
                "required":   tool.Required,
            },
        }
    }
    return result
}
```

---

## 10. References

### Official AWS Documentation
- [Amazon Bedrock User Guide](https://docs.aws.amazon.com/bedrock/latest/userguide/)
- [Bedrock Runtime API Reference](https://docs.aws.amazon.com/bedrock/latest/APIReference/)
- [AWS SDK for Go v2 Documentation](https://aws.github.io/aws-sdk-go-v2/docs/)
- [Bedrock Runtime Go Package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime)

### Model-Specific Documentation
- [Anthropic Claude on Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters-anthropic-claude-messages.html)
- [Meta Llama on Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters-meta.html)
- [Amazon Titan on Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/titan-text-models.html)
- [Mistral AI on Bedrock](https://docs.mistral.ai/deployment/cloud/aws/)

### Cross-Region Inference
- [Cross-Region Inference Guide](https://docs.aws.amazon.com/bedrock/latest/userguide/cross-region-inference.html)
- [Inference Profiles](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles.html)
- [Getting Started with Cross-Region Inference (Blog)](https://aws.amazon.com/blogs/machine-learning/getting-started-with-cross-region-inference-in-amazon-bedrock/)

### Tool Calling
- [Tool Use with Claude](https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters-anthropic-claude-messages-tool-use.html)
- [Converse API Tool Use](https://docs.aws.amazon.com/bedrock/latest/userguide/tool-use.html)

### SDK Best Practices
- [AWS SDK Go v2 Configuration](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/)
- [Error Handling in Go SDK v2](https://aws.github.io/aws-sdk-go-v2/docs/handling-errors/)
- [Retries and Timeouts](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/retries-timeouts/)

### Community Resources
- [AWS SDK Go v2 GitHub Repository](https://github.com/aws/aws-sdk-go-v2)
- [Bedrock Code Examples](https://docs.aws.amazon.com/code-library/latest/ug/bedrock-runtime_code_examples.html)
- [Working with Bedrock Streaming in Go (Blog)](https://www.micahwalter.com/2023/11/working-with-amazon-bedrocks-streaming-response-api-and-go/)

---

## 11. Decision Summary

| Topic | Decision | Rationale |
|-------|----------|-----------|
| **Model Schema** | Use model-specific request/response formats | Each model family has different schemas; generic approach loses type safety |
| **Streaming** | Use Go channels with type switch | Idiomatic Go SDK v2 pattern, efficient |
| **Tool Calling** | Implement for Claude only initially | Only Claude supports tools in Bedrock as of 2025 |
| **Credentials** | Use `config.LoadDefaultConfig()` | Automatic credential chain, follows AWS best practices |
| **Region** | Require explicit region in constructor | Bedrock has no default region, prevents errors |
| **Retry Strategy** | Use adaptive retry mode | Built-in exponential backoff with token bucket, production-tested |
| **Error Handling** | Wrap AWS errors in domain types | Abstracts AWS details, unified error handling |
| **Multi-Region** | Use cross-region inference profiles | No additional cost, AWS-managed, simpler than custom failover |
| **Adapter Design** | Model-specific adapters behind unified interface | Type safety, clean separation of concerns |
| **Testing** | Mock for unit tests, real API for integration | Fast unit tests, confidence from integration tests |

---

## 12. Next Steps

1. **Implement Claude adapter first** - Most feature-complete (tool calling, streaming)
2. **Add Llama adapter** - Second priority, popular open-source models
3. **Add integration tests** - Validate against real AWS Bedrock API
4. **Add examples** - Demonstrate common use cases (tool calling, streaming, multi-region)
5. **Document adapter usage** - Update CLAUDE.md with Bedrock-specific guidance
6. **Consider Converse API** - Future enhancement for unified interface across models
7. **Add Titan and Mistral adapters** - Lower priority, niche use cases

---

**End of Research Document**
