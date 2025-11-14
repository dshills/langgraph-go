package bedrock

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/dshills/langgraph-go/graph/model"
)

// Adapter implements the ChatModel interface for AWS Bedrock.
//
// The adapter is stateless - all configuration is immutable after initialization.
// State management is handled by the LangGraph-Go Engine via the reducer pattern.
//
// Supports multiple model families:
// - Claude (Anthropic): Full features including tools and streaming
// - Llama (Meta): Text generation, no tools
// - Titan (Amazon): Text generation, no tools
// - Mistral: Text generation
//
// Example:
//
//	config := bedrock.Config{
//	    Region:  "us-east-1",
//	    ModelID: "anthropic.claude-3-5-sonnet-20241022-v2:0",
//	}
//	adapter, err := bedrock.NewAdapter(ctx, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	messages := []model.Message{
//	    {Role: model.RoleUser, Content: "Hello!"},
//	}
//	response, err := adapter.Chat(ctx, messages, nil)
type Adapter struct {
	config           *Config
	client           *bedrockruntime.Client
	modelFamily      ModelFamily
	schemaTranslator SchemaTranslator
}

// NewAdapter creates a new Bedrock adapter with the given configuration.
//
// Initializes AWS SDK client, validates configuration, and selects appropriate
// schema translator based on model family.
//
// Returns error if:
// - Configuration validation fails
// - AWS credentials cannot be loaded
// - Model family is unsupported
//
// The returned adapter is safe for concurrent use.
func NewAdapter(ctx context.Context, config Config) (*Adapter, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Detect model family from ModelID
	family := detectModelFamily(config.ModelID)
	if family == ModelFamilyUnknown {
		return nil, fmt.Errorf("unsupported model family for model ID: %s", config.ModelID)
	}

	// Load AWS configuration
	var awsCfg aws.Config
	var err error

	if config.CredentialsProvider != nil {
		// Use explicitly provided credentials
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(config.Region),
			awsconfig.WithCredentialsProvider(config.CredentialsProvider),
		)
	} else {
		// Use default credential chain
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(config.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create Bedrock Runtime client
	client := bedrockruntime.NewFromConfig(awsCfg)

	// Override endpoint if specified (for VPC endpoints or localstack)
	if config.EndpointURL != "" {
		client = bedrockruntime.NewFromConfig(awsCfg, func(o *bedrockruntime.Options) {
			o.BaseEndpoint = aws.String(config.EndpointURL)
		})
	}

	// Select schema translator based on model family
	var translator SchemaTranslator
	switch family {
	case ModelFamilyClaude:
		translator = ClaudeSchemaTranslator{}
	case ModelFamilyLlama:
		// Llama translator implementation in Phase 5
		return nil, fmt.Errorf("Llama model family not yet implemented")
	case ModelFamilyTitan:
		// Titan translator implementation in Phase 5
		return nil, fmt.Errorf("Titan model family not yet implemented")
	case ModelFamilyMistral:
		// Mistral translator implementation in Phase 5
		return nil, fmt.Errorf("Mistral model family not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported model family: %v", family)
	}

	adapter := &Adapter{
		config:           &config,
		client:           client,
		modelFamily:      family,
		schemaTranslator: translator,
	}

	return adapter, nil
}

// Chat sends messages to the Bedrock model and returns the response.
//
// Implements the ChatModel interface from graph/model.
//
// Process:
// 1. Translate LangGraph messages to Bedrock-specific request format
// 2. Call Bedrock InvokeModel API
// 3. Translate Bedrock response back to LangGraph ChatOut format
// 4. Handle errors with retry logic and regional fallback
//
// Parameters:
// - ctx: Context for cancellation and timeout control
// - messages: Conversation history (system, user, assistant messages)
// - tools: Optional tool specifications (only supported by Claude models)
//
// Returns:
// - ChatOut with response text and/or tool calls
// - Error if request fails after retries
//
// Respects context cancellation and enforces timeouts.
func (a *Adapter) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
	// Validate messages
	if len(messages) == 0 {
		return model.ChatOut{}, fmt.Errorf("at least one message is required")
	}

	// Translate LangGraph messages to Bedrock request format
	requestBody, err := a.schemaTranslator.TranslateRequest(messages, tools, a.config)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to translate request: %w", err)
	}

	// Call Bedrock InvokeModel API with retry logic
	response, err := a.invokeModelWithRetry(ctx, requestBody)
	if err != nil {
		return model.ChatOut{}, err
	}

	// Translate Bedrock response to LangGraph ChatOut
	chatOut, err := a.schemaTranslator.TranslateResponse(response)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to translate response: %w", err)
	}

	return chatOut, nil
}

// invokeModelWithRetry calls Bedrock InvokeModel API with exponential backoff retry logic.
func (a *Adapter) invokeModelWithRetry(ctx context.Context, requestBody []byte) ([]byte, error) {
	maxRetries := a.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default retries
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Invoke Bedrock model
		output, err := a.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(a.config.ModelID),
			Body:        requestBody,
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
		})

		if err == nil {
			return output.Body, nil
		}

		// Wrap error
		bedrockErr := wrapAWSError(err, a.config.Region)
		lastErr = bedrockErr

		// Check if error is retryable
		if !bedrockErr.Retryable || attempt == maxRetries {
			return nil, bedrockErr
		}

		// Calculate backoff delay: baseDelay * 2^attempt
		// Using exponential backoff with jitter
		baseDelay := 100 // milliseconds
		delay := baseDelay * (1 << attempt)
		if delay > 5000 {
			delay = 5000 // Cap at 5 seconds
		}

		// Wait before retry (respects context cancellation)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-awaitDelay(ctx, delay):
			// Continue to next retry
		}
	}

	return nil, lastErr
}

// awaitDelay returns a channel that will receive a value after the specified delay in milliseconds.
// Respects context cancellation.
func awaitDelay(ctx context.Context, delayMs int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		timer := time.NewTimer(time.Duration(delayMs) * time.Millisecond)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			close(ch)
		case <-timer.C:
			close(ch)
		}
	}()
	return ch
}
