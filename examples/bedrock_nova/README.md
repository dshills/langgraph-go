# AWS Bedrock Nova Example

This example demonstrates how to use Amazon Nova models through AWS Bedrock with LangGraph-Go.

## Overview

Amazon Nova is AWS's family of foundation models, available through AWS Bedrock. This example shows:

1. **Basic Chat** - Simple question-answer interaction
2. **Tool Calling** - Using Nova's function calling capabilities
3. **Multi-Turn Conversations** - Maintaining context across turns
4. **Model Variants** - Comparing Nova Micro, Lite, and Pro

## Prerequisites

### 1. AWS Account Setup

You need an AWS account with:
- AWS Bedrock access enabled
- Model access granted for Nova models in your region

### 2. Enable Model Access

1. Go to AWS Console → Amazon Bedrock → Model access
2. Request access for:
   - Amazon Nova Micro
   - Amazon Nova Lite
   - Amazon Nova Pro
3. Wait for approval (usually instant for Nova models)

### 3. AWS Credentials

Configure AWS credentials using one of these methods:

**Environment Variables:**
```bash
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1
```

**AWS Credentials File (~/.aws/credentials):**
```ini
[default]
aws_access_key_id = your_access_key
aws_secret_access_key = your_secret_key
region = us-east-1
```

**IAM Role (for EC2/ECS/Lambda):**
No configuration needed - automatically uses instance role.

## Running the Example

```bash
# From project root
cd examples/bedrock_nova

# Run the example
go run main.go
```

## Expected Output

```
=== Example 1: Basic Chat ===
Question: What is the capital of France?
Nova Lite: The capital of France is Paris.
Tokens: 12 input, 8 output

=== Example 2: Tool Calling ===
Question: What's the weather like in Tokyo?
Nova wants to call tool: get_weather
With arguments: map[location:Tokyo]

=== Example 3: Multi-Turn Conversation ===
User: My favorite color is blue.
Nova: That's great! Blue is a wonderful color...

User: What color did I just mention?
Nova: You mentioned that your favorite color is blue.

=== Example 4: Nova Model Variants ===

--- Nova Micro (Fastest, lowest cost) ---
Response: 4
Tokens: 8 in, 1 out

--- Nova Lite (Balanced speed and capability) ---
Response: The answer is 4.
Tokens: 8 in, 5 out

--- Nova Pro (Most capable, multimodal) ---
Response: 2 + 2 equals 4.
Tokens: 8 in, 7 out
```

## Nova Model Variants

| Model | Use Case | Speed | Cost | Capabilities |
|-------|----------|-------|------|--------------|
| **Nova Micro** | Simple tasks, high throughput | Fastest | Lowest | Text only |
| **Nova Lite** | General purpose, balanced | Fast | Low | Text, reasoning |
| **Nova Pro** | Complex reasoning, multimodal | Medium | Medium | Text, images, advanced reasoning |

## Configuration Options

```go
config := bedrock.Config{
    Region:          "us-east-1",              // AWS region
    ModelID:         "amazon.nova-lite-v1:0",  // Model ID
    MaxTokens:       512,                       // Max response tokens
    Temperature:     0.7,                       // 0.0-1.0 (creativity)
    TopP:            0.9,                       // Nucleus sampling
    StopSequences:   []string{"\n\n"},         // Stop generation
    FallbackRegions: []string{"us-west-2"},    // Regional failover
}
```

## Inference Profiles

Use inference profiles for automatic cross-region routing:

```go
// Inference profile (recommended for production)
ModelID: "us.amazon.nova-lite-v1:0"

// Direct model ID (single region)
ModelID: "amazon.nova-lite-v1:0"
```

Inference profiles provide:
- Automatic regional failover
- Better availability
- Load balancing across regions

## Error Handling

```go
response, err := adapter.Chat(ctx, messages, nil)
if err != nil {
    // Check for specific error types
    var bedrockErr *bedrock.BedrockError
    if errors.As(err, &bedrockErr) {
        switch bedrockErr.Code {
        case "ModelNotReadyException":
            // Model loading
        case "ThrottlingException":
            // Rate limited
        case "ValidationException":
            // Invalid request
        }
    }
}
```

## Troubleshooting

**"Model access denied"**
- Go to AWS Console → Bedrock → Model access
- Request access for Nova models in your region

**"Region not supported"**
- Nova models are available in: us-east-1, us-west-2, eu-west-1, ap-southeast-1
- Check AWS documentation for latest regional availability

**"AWS credentials not found"**
- Verify AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are set
- Check ~/.aws/credentials file exists and is readable
- Ensure IAM role has bedrock:InvokeModel permission

**"Throttling errors"**
- Implement exponential backoff (built into adapter)
- Request quota increase in AWS Service Quotas console
- Use inference profiles for better rate limits

## Next Steps

- Explore streaming responses with `ChatStream()`
- Implement tool execution loops
- Add conversation memory with checkpointing
- Use multi-region failover for production resilience

## References

- [AWS Bedrock Documentation](https://docs.aws.amazon.com/bedrock/)
- [Amazon Nova Models](https://aws.amazon.com/bedrock/nova/)
- [LangGraph-Go Bedrock Adapter](../../graph/model/bedrock/)
