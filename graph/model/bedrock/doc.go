// Package bedrock provides AWS Bedrock LLM integration for LangGraph-Go.
//
// # Overview
//
// This package implements the ChatModel interface for AWS Bedrock, Amazon's
// managed service for foundation models. It supports multiple model families
// including Claude (Anthropic), Llama (Meta), Titan (Amazon), and Mistral.
//
// Key features:
//   - Multi-model support with automatic schema translation
//   - Regional failover for high availability
//   - Automatic retry with exponential backoff
//   - Streaming response support (model-dependent)
//   - Tool/function calling (Claude models)
//   - Comprehensive error handling with retry classification
//
// # Quick Start
//
// Basic usage with Claude model:
//
//	import (
//	    "context"
//	    "github.com/dshills/langgraph-go/graph/model/bedrock"
//	)
//
//	config := bedrock.Config{
//	    Region:  "us-east-1",
//	    ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
//	    MaxTokens: 4096,
//	}
//
//	adapter, err := bedrock.NewAdapter(context.Background(), config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "What is the capital of France?"},
//	}
//
//	response, err := adapter.Chat(context.Background(), messages, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(response.Text) // "The capital of France is Paris."
//
// # Regional Failover
//
// Configure automatic failover to backup regions for high availability:
//
//	config := bedrock.Config{
//	    Region: "us-east-1",
//	    FallbackRegions: []string{"us-west-2", "eu-west-1"},
//	    ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
//	}
//
// On throttling or service errors in us-east-1, requests automatically
// retry in us-west-2, then eu-west-1. The actual region used is tracked
// in ChatOut.Meta["region"] for observability.
//
// # Tool Calling
//
// Claude models support tool/function calling:
//
//	tools := []model.ToolSpec{
//	    {
//	        Name: "get_weather",
//	        Description: "Get current weather for a location",
//	        Schema: map[string]interface{}{
//	            "type": "object",
//	            "properties": map[string]interface{}{
//	                "location": map[string]interface{}{
//	                    "type": "string",
//	                    "description": "City name",
//	                },
//	            },
//	            "required": []string{"location"},
//	        },
//	    },
//	}
//
//	response, err := adapter.Chat(ctx, messages, tools)
//	for _, call := range response.ToolCalls {
//	    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
//	}
//
// # Error Handling
//
// Errors are wrapped as BedrockError with retry classification:
//
//	response, err := adapter.Chat(ctx, messages, nil)
//	if err != nil {
//	    var bedrockErr *bedrock.BedrockError
//	    if errors.As(err, &bedrockErr) {
//	        if bedrockErr.Retryable {
//	            // Transient error - already retried automatically
//	            fmt.Printf("Failed after retries: %s\n", bedrockErr.Code)
//	        } else {
//	            // Permanent error - needs user action
//	            fmt.Printf("Non-retryable error: %s\n", bedrockErr.Code)
//	        }
//	    }
//	}
//
// Retryable errors: ThrottlingException, ModelTimeoutException, InternalServerException
// Non-retryable errors: ValidationException, AccessDeniedException, ResourceNotFoundException
//
// # Supported Models
//
// Claude (Anthropic):
//   - anthropic.claude-3-5-sonnet-20241022-v2:0 (recommended)
//   - anthropic.claude-3-sonnet-20240229-v1:0
//   - anthropic.claude-3-haiku-20240307-v1:0
//   - Features: Tools, streaming, system messages, long context
//
// Llama (Meta) - Phase 5:
//   - meta.llama3-2-90b-instruct-v1:0
//   - meta.llama3-1-70b-instruct-v1:0
//   - Features: Text generation only
//
// Titan (Amazon) - Phase 5:
//   - amazon.titan-text-premier-v1:0
//   - amazon.titan-text-express-v1:0
//   - Features: Text generation only
//
// Mistral - Phase 5:
//   - mistral.mistral-large-2402-v1:0
//   - mistral.mistral-7b-instruct-v0:2
//   - Features: Text generation
//
// # AWS Credentials
//
// The adapter uses the AWS SDK default credential chain:
//  1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
//  2. Shared credentials file (~/.aws/credentials)
//  3. IAM role (for EC2/ECS/Lambda)
//
// Or provide credentials explicitly:
//
//	config := bedrock.Config{
//	    Region: "us-east-1",
//	    ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
//	    CredentialsProvider: aws.NewCredentialsCache(
//	        credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
//	    ),
//	}
//
// # Architecture
//
// The package uses a translator pattern to support multiple model families:
//
//	Adapter (stateless)
//	  ├─ Config (immutable)
//	  ├─ AWS SDK Client (bedrockruntime)
//	  └─ SchemaTranslator (model family specific)
//	     ├─ ClaudeSchemaTranslator
//	     ├─ LlamaSchemaTranslator (Phase 5)
//	     ├─ TitanSchemaTranslator (Phase 5)
//	     └─ MistralSchemaTranslator (Phase 5)
//
// Each translator converts between LangGraph message format and the
// model-specific request/response format required by Bedrock.
//
// # Implementation Status
//
// Completed:
//   - Claude model support (full features)
//   - Regional failover
//   - Retry logic with exponential backoff
//   - Error classification
//   - Metadata tracking
//
// In Progress (Phase 5-10):
//   - Streaming support
//   - Llama, Titan, Mistral models
//   - Integration tests
//   - Examples
//
// See specs/008-bedrock-llm-support/tasks.md for detailed roadmap.
package bedrock
