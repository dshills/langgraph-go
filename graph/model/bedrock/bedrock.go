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

	// Call Bedrock InvokeModel API with retry and regional fallback logic
	response, usedRegion, err := a.invokeModelWithRegionalFallback(ctx, requestBody)
	if err != nil {
		return model.ChatOut{}, err
	}

	// Translate Bedrock response to LangGraph ChatOut
	chatOut, err := a.schemaTranslator.TranslateResponse(response)
	if err != nil {
		return model.ChatOut{}, fmt.Errorf("failed to translate response: %w", err)
	}

	// Add region to metadata (use the region that actually served the request)
	if chatOut.Meta == nil {
		chatOut.Meta = make(map[string]interface{})
	}
	chatOut.Meta["region"] = usedRegion

	return chatOut, nil
}

// invokeModelWithRegionalFallback attempts to invoke the model with regional fallback.
//
// Flow:
//  1. Try primary region with retries
//  2. If primary fails with retryable error, try each fallback region
//  3. Return success from first region that works
//  4. Return error if all regions fail
//
// Returns:
//   - Response body
//   - Region that successfully served the request
//   - Error if all regions exhausted
func (a *Adapter) invokeModelWithRegionalFallback(ctx context.Context, requestBody []byte) ([]byte, string, error) {
	// Build list of regions to try: primary + fallbacks
	regions := []string{a.config.Region}
	regions = append(regions, a.config.FallbackRegions...)

	var lastErr error
	var attemptedRegions []string

	for _, region := range regions {
		attemptedRegions = append(attemptedRegions, region)

		// Check context cancellation before trying each region
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		default:
		}

		// Determine which client to use
		var client *bedrockruntime.Client
		if region == a.config.Region {
			// Use primary client for primary region
			client = a.client
		} else {
			// Create temporary client for fallback region
			var err error
			client, err = a.createClientForRegion(ctx, region)
			if err != nil {
				// Log client creation failure and try next region
				lastErr = fmt.Errorf("failed to create client for region %s: %w", region, err)
				continue
			}
		}

		// Try to invoke model in this region
		response, err := a.invokeModelInRegion(ctx, client, region, requestBody)
		if err == nil {
			// Success! Return response and region used
			if region != a.config.Region {
				// Log successful fallback to secondary region (T050)
				// In production, this would emit telemetry event
				// For now, we just track it in error context
			}
			return response, region, nil
		}

		lastErr = err

		// Check if error is retryable
		bedrockErr, ok := err.(*BedrockError)
		if !ok || !bedrockErr.Retryable {
			// Non-retryable error, don't try other regions
			return nil, "", err
		}

		// Retryable error in this region, try next fallback region
		if region != regions[len(regions)-1] {
			// Not the last region, continue to next fallback
			// Log fallback attempt (T050)
			continue
		}
	}

	// All regions exhausted
	return nil, "", fmt.Errorf("all regions exhausted (tried: %v): %w", attemptedRegions, lastErr)
}

// createClientForRegion creates a Bedrock Runtime client for a specific region.
func (a *Adapter) createClientForRegion(ctx context.Context, region string) (*bedrockruntime.Client, error) {
	var awsCfg aws.Config
	var err error

	if a.config.CredentialsProvider != nil {
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(a.config.CredentialsProvider),
		)
	} else {
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}

	client := bedrockruntime.NewFromConfig(awsCfg)

	// Override endpoint if specified
	if a.config.EndpointURL != "" {
		client = bedrockruntime.NewFromConfig(awsCfg, func(o *bedrockruntime.Options) {
			o.BaseEndpoint = aws.String(a.config.EndpointURL)
		})
	}

	return client, nil
}

// invokeModelInRegion invokes the model in a specific region with retries.
func (a *Adapter) invokeModelInRegion(ctx context.Context, client *bedrockruntime.Client, region string, requestBody []byte) ([]byte, error) {
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
		output, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(a.config.ModelID),
			Body:        requestBody,
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
		})

		if err == nil {
			return output.Body, nil
		}

		// Wrap error
		bedrockErr := wrapAWSError(err, region)
		lastErr = bedrockErr

		// Check if error is retryable
		if !bedrockErr.Retryable || attempt == maxRetries {
			return nil, bedrockErr
		}

		// Calculate backoff delay: baseDelay * 2^attempt
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

// invokeModelWithRetry calls Bedrock InvokeModel API with exponential backoff retry logic.
// DEPRECATED: Use invokeModelWithRegionalFallback instead for regional failover support.
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
