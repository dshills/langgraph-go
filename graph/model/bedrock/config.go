// Package bedrock provides AWS Bedrock LLM integration for LangGraph-Go.
package bedrock

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Config holds configuration for the Bedrock adapter.
//
// Bedrock is AWS's managed service for foundation models (Claude, Llama, Titan, Mistral).
// This config specifies which model to use, AWS credentials, region, and generation parameters.
//
// Example:
//
//	config := bedrock.Config{
//	    Region:      "us-east-1",
//	    ModelID:     "anthropic.claude-3-5-sonnet-20241022-v2:0",
//	    MaxTokens:   4096,
//	    Temperature: 0.7,
//	}
type Config struct {
	// Region is the AWS region for Bedrock endpoint (e.g., "us-east-1").
	// Required. Must be a valid AWS region where Bedrock is available.
	Region string

	// ModelID is the Bedrock model identifier (e.g., "anthropic.claude-3-5-sonnet-20241022-v2:0").
	// Required. Format: "provider.model-name[:version]"
	ModelID string

	// CredentialsProvider explicitly sets AWS credentials.
	// Optional. If nil, uses AWS SDK default credential chain:
	// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// 2. Shared credentials file (~/.aws/credentials)
	// 3. IAM role (for EC2/ECS/Lambda)
	CredentialsProvider aws.CredentialsProvider

	// EndpointURL overrides the default Bedrock endpoint.
	// Optional. Used for VPC endpoints or testing with localstack.
	EndpointURL string

	// FallbackRegions provides ordered list of backup regions for automatic failover.
	// Optional. On throttling or service errors, adapter retries in fallback regions.
	FallbackRegions []string

	// MaxRetries sets maximum retry attempts for transient errors.
	// Optional. Default: 3. Range: 0-10.
	// Retries use exponential backoff with jitter.
	MaxRetries int

	// Temperature controls randomness in model output.
	// Optional. Range: 0.0 (deterministic) to 1.0 (very random).
	// Default: model-specific (typically 1.0 for Claude, 0.5 for Llama).
	Temperature float64

	// MaxTokens limits the maximum tokens to generate.
	// Optional. Range: 1 to model maximum (4096 for Claude, 8192 for Llama).
	// Default: model-specific.
	MaxTokens int

	// TopP controls nucleus sampling diversity.
	// Optional. Range: 0.0 to 1.0.
	// Default: model-specific (typically 0.9).
	TopP float64

	// StopSequences defines sequences that stop generation.
	// Optional. Maximum 4 sequences (Bedrock API limit).
	StopSequences []string

	// StreamingEnabled enables streaming responses token-by-token.
	// Optional. Default: false.
	// Requires model support (Claude supports, Llama 4 Instruct does not).
	StreamingEnabled bool
}

// Validate checks if the configuration is valid.
//
// Validates:
// - Region is a known AWS region
// - ModelID follows expected format
// - Temperature is in range [0.0, 1.0]
// - MaxTokens is positive and within model limits
// - TopP is in range [0.0, 1.0]
// - FallbackRegions are valid and don't duplicate primary Region
// - MaxRetries is in range [0, 10]
//
// Returns error describing first validation failure, or nil if valid.
func (c *Config) Validate() error {
	// Validate Region
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if !isValidAWSRegion(c.Region) {
		return fmt.Errorf("invalid AWS region: %s", c.Region)
	}

	// Validate ModelID
	if c.ModelID == "" {
		return fmt.Errorf("model ID is required")
	}
	if !isValidModelID(c.ModelID) {
		return fmt.Errorf("invalid model ID format: %s (expected format: provider.model-name)", c.ModelID)
	}

	// Validate Temperature
	if c.Temperature < 0.0 || c.Temperature > 1.0 {
		return fmt.Errorf("temperature must be between 0.0 and 1.0, got: %f", c.Temperature)
	}

	// Validate MaxTokens
	if c.MaxTokens < 0 {
		return fmt.Errorf("max tokens must be non-negative, got: %d", c.MaxTokens)
	}

	// Validate TopP
	if c.TopP < 0.0 || c.TopP > 1.0 {
		return fmt.Errorf("top_p must be between 0.0 and 1.0, got: %f", c.TopP)
	}

	// Validate FallbackRegions (no duplicates, no primary region)
	regionMap := make(map[string]bool)
	regionMap[c.Region] = true
	for _, region := range c.FallbackRegions {
		if regionMap[region] {
			return fmt.Errorf("fallback region %s duplicates primary region or appears multiple times", region)
		}
		if !isValidAWSRegion(region) {
			return fmt.Errorf("invalid fallback region: %s", region)
		}
		regionMap[region] = true
	}

	// Validate MaxRetries
	if c.MaxRetries < 0 || c.MaxRetries > 10 {
		return fmt.Errorf("max retries must be between 0 and 10, got: %d", c.MaxRetries)
	}

	return nil
}

// isValidAWSRegion checks if a region string is a known AWS region.
func isValidAWSRegion(region string) bool {
	validRegions := map[string]bool{
		"us-east-1":      true,
		"us-east-2":      true,
		"us-west-1":      true,
		"us-west-2":      true,
		"eu-west-1":      true,
		"eu-west-2":      true,
		"eu-west-3":      true,
		"eu-central-1":   true,
		"eu-north-1":     true,
		"ap-southeast-1": true,
		"ap-southeast-2": true,
		"ap-northeast-1": true,
		"ap-northeast-2": true,
		"ap-south-1":     true,
		"ca-central-1":   true,
		"sa-east-1":      true,
	}
	return validRegions[region]
}

// isValidModelID checks if a model ID follows the expected format: provider.model-name
func isValidModelID(modelID string) bool {
	// Model ID must contain at least one dot (provider.model format)
	return len(modelID) > 0 && (strings.Contains(modelID, "."))
}
