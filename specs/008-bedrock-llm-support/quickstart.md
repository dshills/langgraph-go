# Quickstart: AWS Bedrock LLM Provider

**Feature**: AWS Bedrock LLM Provider Support
**Date**: 2025-11-14
**Spec**: [spec.md](./spec.md)

## Overview

This guide shows how to use AWS Bedrock foundation models (Claude, Llama, Titan, Mistral) in LangGraph-Go workflows. The Bedrock adapter implements the `ChatModel` interface, making it a drop-in replacement for OpenAI, Anthropic, and Google adapters.

## Prerequisites

### 1. AWS Account Setup

**Required**:
- AWS account with Bedrock access enabled
- IAM permissions for Bedrock model invocation
- AWS credentials configured locally

**IAM Permissions**:

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
        "arn:aws:bedrock:*::foundation-model/*"
      ]
    }
  ]
}
```

**Enable Model Access**:
1. Go to AWS Console → Bedrock → Model access
2. Request access to desired models (Claude, Llama, Titan, Mistral)
3. Wait for approval (usually instant for on-demand models)

### 2. AWS Credentials Configuration

**Option 1: Environment Variables** (recommended for development):

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
```

**Option 2: AWS Credentials File** (`~/.aws/credentials`):

```ini
[default]
aws_access_key_id = your-access-key
aws_secret_access_key = your-secret-key
region = us-east-1
```

**Option 3: IAM Role** (recommended for production on EC2/ECS/Lambda):

No configuration needed - SDK automatically uses instance IAM role.

### 3. Go Module Setup

```bash
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime
go get github.com/aws/aws-sdk-go-v2/config
```

## Basic Usage

### Example 1: Simple Chat with Claude

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yourusername/langgraph-go/graph/model"
    "github.com/yourusername/langgraph-go/graph/model/bedrock"
)

func main() {
    ctx := context.Background()

    // Create Bedrock adapter for Claude 3.5 Sonnet
    config := bedrock.Config{
        Region:      "us-east-1",
        ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
        MaxTokens:   4096,
        Temperature: 0.7,
    }

    adapter, err := bedrock.NewAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to create Bedrock adapter: %v", err)
    }

    // Send a chat message
    messages := []model.Message{
        {Role: model.RoleUser, Content: "What is the capital of France?"},
    }

    response, err := adapter.Chat(ctx, messages, nil)
    if err != nil {
        log.Fatalf("Chat failed: %v", err)
    }

    fmt.Printf("Response: %s\n", response.Text)
    fmt.Printf("Input tokens: %d, Output tokens: %d\n",
        response.Meta["input_tokens"], response.Meta["output_tokens"])
}
```

**Expected Output**:

```
Response: The capital of France is Paris.
Input tokens: 12, Output tokens: 8
```

---

### Example 2: Using Bedrock in a LangGraph Workflow

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yourusername/langgraph-go/graph"
    "github.com/yourusername/langgraph-go/graph/model"
    "github.com/yourusername/langgraph-go/graph/model/bedrock"
    "github.com/yourusername/langgraph-go/graph/store"
    "github.com/yourusername/langgraph-go/graph/emit"
)

// Workflow state
type State struct {
    Messages   []model.Message
    LastError  error
}

// Reducer merges state deltas
func reducer(prev State, delta State) State {
    if delta.LastError != nil {
        return State{Messages: prev.Messages, LastError: delta.LastError}
    }
    return State{Messages: append(prev.Messages, delta.Messages...)}
}

// LLM node using Bedrock
type LLMNode struct {
    model model.ChatModel
}

func (n LLMNode) Run(ctx context.Context, state State) graph.NodeResult[State] {
    out, err := n.model.Chat(ctx, state.Messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Append assistant response to messages
    delta := State{
        Messages: []model.Message{
            {Role: model.RoleAssistant, Content: out.Text},
        },
    }

    return graph.NodeResult[State]{
        Delta: delta,
        Route: graph.Stop(), // End workflow
    }
}

func main() {
    ctx := context.Background()

    // Create Bedrock adapter
    config := bedrock.Config{
        Region:    "us-east-1",
        ModelID:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
        MaxTokens: 2048,
    }

    adapter, err := bedrock.NewAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to create adapter: %v", err)
    }

    // Build workflow graph
    memStore := store.NewMemStore[State]()
    logger := emit.NewLogEmitter()

    engine := graph.New(reducer, memStore, logger)
    engine.AddNode("llm", LLMNode{model: adapter})
    engine.SetStart("llm")

    // Run workflow
    initialState := State{
        Messages: []model.Message{
            {Role: model.RoleUser, Content: "Explain quantum computing in one sentence."},
        },
    }

    result, err := engine.Run(ctx, "run-001", initialState)
    if err != nil {
        log.Fatalf("Workflow failed: %v", err)
    }

    fmt.Printf("Final response: %s\n", result.Messages[len(result.Messages)-1].Content)
}
```

---

### Example 3: Streaming Responses

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yourusername/langgraph-go/graph/model"
    "github.com/yourusername/langgraph-go/graph/model/bedrock"
)

func main() {
    ctx := context.Background()

    // Enable streaming in config
    config := bedrock.Config{
        Region:           "us-east-1",
        ModelID:          "anthropic.claude-3-5-sonnet-20241022-v2:0",
        StreamingEnabled: true,
    }

    adapter, err := bedrock.NewAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to create adapter: %v", err)
    }

    messages := []model.Message{
        {Role: model.RoleUser, Content: "Write a haiku about Go programming."},
    }

    // Chat with streaming callback
    response, err := adapter.ChatStream(ctx, messages, nil, func(chunk string) {
        fmt.Print(chunk) // Print each token as it arrives
    })
    if err != nil {
        log.Fatalf("Streaming failed: %v", err)
    }

    fmt.Printf("\n\nFull response: %s\n", response.Text)
}
```

**Expected Output** (tokens appear progressively):

```
Code flows like streams,
Goroutines dance in channels,
Simple, fast, and clean.

Full response: Code flows like streams,
Goroutines dance in channels,
Simple, fast, and clean.
```

---

### Example 4: Tool Calling (Claude Only)

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/yourusername/langgraph-go/graph/model"
    "github.com/yourusername/langgraph-go/graph/model/bedrock"
)

func main() {
    ctx := context.Background()

    config := bedrock.Config{
        Region:  "us-east-1",
        ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
    }

    adapter, err := bedrock.NewAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to create adapter: %v", err)
    }

    // Define tool
    tools := []model.ToolSpec{
        {
            Name:        "get_weather",
            Description: "Get current weather for a location",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "location": map[string]any{
                        "type":        "string",
                        "description": "City name",
                    },
                },
                "required": []string{"location"},
            },
        },
    }

    messages := []model.Message{
        {Role: model.RoleUser, Content: "What's the weather in Paris?"},
    }

    // First request: model decides to use tool
    response, err := adapter.Chat(ctx, messages, tools)
    if err != nil {
        log.Fatalf("Chat failed: %v", err)
    }

    if len(response.ToolCalls) > 0 {
        toolCall := response.ToolCalls[0]
        fmt.Printf("Tool requested: %s with args: %v\n", toolCall.Name, toolCall.Arguments)

        // Execute tool (mocked here)
        toolResult := `{"temperature": 18, "condition": "partly cloudy"}`

        // Send tool result back to model
        messages = append(messages, model.Message{
            Role:    model.RoleAssistant,
            Content: "", // Claude doesn't echo tool calls
            ToolCalls: []model.ToolCall{toolCall},
        })
        messages = append(messages, model.Message{
            Role:       model.RoleUser,
            Content:    toolResult,
            ToolCallID: toolCall.ID,
        })

        // Second request: model generates final answer
        finalResponse, err := adapter.Chat(ctx, messages, tools)
        if err != nil {
            log.Fatalf("Final chat failed: %v", err)
        }

        fmt.Printf("Final answer: %s\n", finalResponse.Text)
    }
}
```

**Expected Output**:

```
Tool requested: get_weather with args: map[location:Paris]
Final answer: The weather in Paris is currently 18°C with partly cloudy skies.
```

---

### Example 5: Multi-Region with Fallback

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yourusername/langgraph-go/graph/model"
    "github.com/yourusername/langgraph-go/graph/model/bedrock"
)

func main() {
    ctx := context.Background()

    // Configure primary region with fallbacks
    config := bedrock.Config{
        Region:          "us-east-1",
        FallbackRegions: []string{"us-west-2", "eu-west-1"},
        ModelID:         "anthropic.claude-3-5-sonnet-20241022-v2:0",
        MaxRetries:      3,
    }

    adapter, err := bedrock.NewAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to create adapter: %v", err)
    }

    messages := []model.Message{
        {Role: model.RoleUser, Content: "Hello!"},
    }

    // If us-east-1 fails, automatically retries in us-west-2, then eu-west-1
    response, err := adapter.Chat(ctx, messages, nil)
    if err != nil {
        log.Fatalf("Chat failed in all regions: %v", err)
    }

    fmt.Printf("Response: %s\n", response.Text)
    fmt.Printf("Region used: %s\n", response.Meta["region"])
}
```

---

## Configuration Reference

### BedrockConfig Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `Region` | `string` | Yes | - | AWS region (e.g., "us-east-1") |
| `ModelID` | `string` | Yes | - | Bedrock model ID (e.g., "anthropic.claude-3-5-sonnet-20241022-v2:0") |
| `CredentialsProvider` | `aws.CredentialsProvider` | No | Default chain | AWS credentials |
| `EndpointURL` | `string` | No | - | Custom endpoint (VPC/testing) |
| `FallbackRegions` | `[]string` | No | - | Fallback regions for retry |
| `MaxRetries` | `int` | No | 3 | Max retry attempts |
| `Temperature` | `float64` | No | Model default | Sampling temperature (0.0-1.0) |
| `MaxTokens` | `int` | No | Model default | Max tokens to generate |
| `TopP` | `float64` | No | Model default | Nucleus sampling (0.0-1.0) |
| `StopSequences` | `[]string` | No | - | Stop sequences (max 4) |
| `StreamingEnabled` | `bool` | No | `false` | Enable streaming |

### Supported Model IDs

**Claude (Anthropic)**:
- `anthropic.claude-3-5-sonnet-20241022-v2:0` - Latest Claude 3.5 Sonnet (recommended)
- `anthropic.claude-3-sonnet-20240229-v1:0` - Claude 3 Sonnet
- `anthropic.claude-3-haiku-20240307-v1:0` - Claude 3 Haiku (fast, cheap)

**Llama (Meta)**:
- `meta.llama3-2-90b-instruct-v1:0` - Llama 3.2 90B Instruct
- `meta.llama3-1-70b-instruct-v1:0` - Llama 3.1 70B Instruct
- `meta.llama3-1-8b-instruct-v1:0` - Llama 3.1 8B Instruct

**Titan (Amazon)**:
- `amazon.titan-text-premier-v1:0` - Titan Text Premier
- `amazon.titan-text-express-v1` - Titan Text Express

**Mistral**:
- `mistral.mistral-large-2402-v1:0` - Mistral Large
- `mistral.mistral-7b-instruct-v0:2` - Mistral 7B Instruct

See [AWS Bedrock documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html) for complete model list and regional availability.

---

## Error Handling

### Common Errors

**AccessDeniedException**:
```
Error: AccessDeniedException: User is not authorized to perform bedrock:InvokeModel
```

**Solution**: Add IAM permissions for `bedrock:InvokeModel` action.

---

**ModelNotReadyException**:
```
Error: ModelNotReadyException: Model 'anthropic.claude-3-5-sonnet-20241022-v2:0' is not available in region 'eu-central-1'
```

**Solution**: Request model access in Bedrock console or use different region.

---

**ThrottlingException**:
```
Error: ThrottlingException: Rate exceeded
```

**Solution**: Adapter automatically retries with exponential backoff (up to `MaxRetries`). Increase `MaxRetries` or use `FallbackRegions`.

---

**ValidationException**:
```
Error: ValidationException: max_tokens must be between 1 and 200000
```

**Solution**: Fix `MaxTokens` in config to match model limits.

---

## Performance Tips

1. **Reuse Adapter Instances**: Create one adapter per model and reuse across requests (SDK clients are pooled internally)
2. **Use Streaming for UX**: Enable streaming for interactive applications to reduce perceived latency
3. **Choose Right Model**: Claude for tool calling, Llama for simple chat, Titan for cost optimization
4. **Regional Placement**: Deploy in same region as Bedrock service for lowest latency
5. **Multi-Region Fallback**: Configure fallback regions for production high-availability

---

## Next Steps

- Read [data-model.md](./data-model.md) for detailed entity definitions
- Review [contracts/](./contracts/) for request/response schemas
- Check [research.md](./research.md) for AWS best practices and advanced patterns
- Run `/speckit.tasks` to see implementation task breakdown

---

## Troubleshooting

**Q: "context deadline exceeded" errors**

A: Increase timeout on context: `ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)`

---

**Q: Streaming not working**

A: Check model supports streaming (Llama 4 Instruct does NOT). Verify `StreamingEnabled: true` in config.

---

**Q: Tool calling returns empty ToolCalls**

A: Only Claude models support tool calling. Llama/Titan/Mistral ignore tools and generate text responses.

---

**Q: High latency in responses**

A: Use region closest to your deployment. Consider cross-region inference profiles for automatic routing.
