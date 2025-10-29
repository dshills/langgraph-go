package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements the CodeReviewer interface using Anthropic's Claude API.
// It wraps the official anthropic-sdk-go client and provides code review functionality
// by formatting review requests into prompts and parsing structured JSON responses.
//
// AnthropicProvider is safe for concurrent use after creation. The underlying SDK
// client handles concurrent requests safely.
//
// Example usage:
//
//	provider := NewAnthropicProvider(apiKey, "claude-3-5-sonnet-20241022")
//	resp, err := provider.ReviewBatch(ctx, ReviewRequest{
//	    Files: []CodeFile{{FilePath: "main.go", Content: code, Language: "go"}},
//	    FocusAreas: []string{"security", "performance"},
//	    Language: "go",
//	})
type AnthropicProvider struct {
	client *anthropic.Client
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider with the given API key and model.
// The model parameter should be one of Claude's available models:
//   - claude-3-5-sonnet-20241022 (recommended, most capable)
//   - claude-3-opus-20240229 (highest capability, slower)
//   - claude-3-sonnet-20240229 (balanced)
//   - claude-3-haiku-20240307 (fastest, lower cost)
//
// The API key can be obtained from https://console.anthropic.com/
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{
		client: &client,
		model:  model,
	}
}

// ReviewBatch implements the CodeReviewer interface by calling Claude to review code files.
// It formats the files and focus areas into a prompt, requests structured JSON output,
// and parses the response into ReviewIssue structs.
//
// The method:
// 1. Constructs a detailed code review prompt with all files
// 2. Specifies focus areas (security, performance, style, best-practices)
// 3. Requests JSON-structured output with specific fields
// 4. Parses the JSON response into ReviewIssue structs
// 5. Validates each issue and populates provider metadata
//
// Returns ReviewResponse with issues found, token usage, and duration.
// Returns error if API call fails, with appropriate error types:
//   - ErrInvalidAPIKey for authentication failures (401, 403)
//   - ErrRateLimited for rate limit errors (429)
//   - ErrTimeout for timeout errors
//   - Generic ReviewError for other API failures
func (a *AnthropicProvider) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	start := time.Now()

	// Handle empty file list
	if len(req.Files) == 0 {
		return ReviewResponse{
			Issues:       []ReviewIssue{},
			TokensUsed:   0,
			Duration:     time.Since(start),
			ProviderName: "anthropic",
		}, nil
	}

	// Build the code review prompt
	prompt := a.buildPrompt(req)

	// Call Claude API with structured output request
	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})

	if err != nil {
		return ReviewResponse{}, a.handleAPIError(err)
	}

	// Parse the response
	issues, err := a.parseResponse(message, req.Files)
	if err != nil {
		return ReviewResponse{}, &ReviewError{
			Code:      "parse_error",
			Message:   fmt.Sprintf("failed to parse Claude response: %v", err),
			Retryable: false,
		}
	}

	// Calculate token usage
	tokensUsed := int(message.Usage.InputTokens + message.Usage.OutputTokens)

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   tokensUsed,
		Duration:     time.Since(start),
		ProviderName: "anthropic",
	}, nil
}

// Name returns "anthropic" as the provider identifier.
func (a *AnthropicProvider) Name() string {
	return "anthropic"
}

// TokenLimit returns Claude's maximum context window size of 200,000 tokens.
// This is Claude 3.5 Sonnet's context window, which is larger than most other
// models and allows reviewing larger batches of code in a single request.
func (a *AnthropicProvider) TokenLimit() int {
	return 200000
}

// buildPrompt constructs the code review prompt for Claude.
// It includes:
//   - Instructions for code review
//   - Focus areas (security, performance, style, best-practices)
//   - All code files with line numbers
//   - JSON output format specification
//   - Example output structure
func (a *AnthropicProvider) buildPrompt(req ReviewRequest) string {
	var sb strings.Builder

	// Header and instructions
	sb.WriteString("You are an expert code reviewer. Please review the following ")
	sb.WriteString(req.Language)
	sb.WriteString(" code and identify potential issues.\n\n")

	// Focus areas
	if len(req.FocusAreas) > 0 {
		sb.WriteString("Focus on these areas: ")
		sb.WriteString(strings.Join(req.FocusAreas, ", "))
		sb.WriteString("\n\n")
	}

	// Files section
	sb.WriteString("Code to review:\n\n")
	for _, file := range req.Files {
		sb.WriteString("File: ")
		sb.WriteString(file.FilePath)
		sb.WriteString("\n```")
		sb.WriteString(file.Language)
		sb.WriteString("\n")
		sb.WriteString(file.Content)
		sb.WriteString("\n```\n\n")
	}

	// Output format instructions
	sb.WriteString("\nProvide your review as a JSON array of issues. Each issue must have:\n")
	sb.WriteString("- file: the file path where the issue was found\n")
	sb.WriteString("- line: the line number (integer, use 0 for file-level issues)\n")
	sb.WriteString("- severity: one of [critical, high, medium, low, info]\n")
	sb.WriteString("- category: one of [security, performance, style, best-practice]\n")
	sb.WriteString("- description: brief description of the issue\n")
	sb.WriteString("- remediation: how to fix the issue\n")
	sb.WriteString("- confidence: a number between 0.0 and 1.0 indicating confidence\n\n")

	sb.WriteString("Return ONLY the JSON array, with no additional text. Example format:\n")
	sb.WriteString(`[{"file":"test.go","line":10,"severity":"high","category":"security",`)
	sb.WriteString(`"description":"SQL injection vulnerability","remediation":"Use parameterized queries",`)
	sb.WriteString(`"confidence":0.95}]`)
	sb.WriteString("\n\nIf no issues found, return an empty array: []")

	return sb.String()
}

// parseResponse extracts review issues from Claude's response.
// It handles both structured JSON responses and attempts to extract JSON
// from responses that include additional text.
//
// The method:
// 1. Extracts text content from the message
// 2. Attempts to parse as JSON array
// 3. Falls back to searching for JSON array in text
// 4. Validates and enriches each issue with provider metadata
//
// Returns a slice of validated ReviewIssue structs or an error if parsing fails.
func (a *AnthropicProvider) parseResponse(message *anthropic.Message, files []CodeFile) ([]ReviewIssue, error) {
	// Extract text content from the message
	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText += block.Text
		}
	}

	if responseText == "" {
		return []ReviewIssue{}, nil
	}

	// Try to parse the response as JSON
	var rawIssues []struct {
		File        string  `json:"file"`
		Line        int     `json:"line"`
		Severity    string  `json:"severity"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
		Remediation string  `json:"remediation"`
		Confidence  float64 `json:"confidence"`
	}

	// First, try direct JSON parsing
	err := json.Unmarshal([]byte(responseText), &rawIssues)
	if err != nil {
		// If that fails, try to extract JSON array from text
		jsonStart := strings.Index(responseText, "[")
		jsonEnd := strings.LastIndex(responseText, "]")

		if jsonStart == -1 || jsonEnd == -1 || jsonStart >= jsonEnd {
			return nil, fmt.Errorf("no valid JSON array found in response")
		}

		jsonStr := responseText[jsonStart : jsonEnd+1]
		err = json.Unmarshal([]byte(jsonStr), &rawIssues)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	// Convert to ReviewIssue structs and validate
	issues := make([]ReviewIssue, 0, len(rawIssues))
	for _, raw := range rawIssues {
		issue := ReviewIssue{
			File:         raw.File,
			Line:         raw.Line,
			Severity:     raw.Severity,
			Category:     raw.Category,
			Description:  raw.Description,
			Remediation:  raw.Remediation,
			ProviderName: "anthropic",
			Confidence:   raw.Confidence,
		}

		// Validate the issue
		if err := issue.Validate(); err != nil {
			// Log validation error but continue with other issues
			continue
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// handleAPIError converts Anthropic SDK errors to ReviewError types.
// It distinguishes between:
//   - Authentication errors (401, 403) -> ErrInvalidAPIKey (permanent)
//   - Rate limit errors (429) -> ErrRateLimited (retryable)
//   - Timeout errors -> ErrTimeout (retryable)
//   - Context cancellation -> wrapped context error (retryable)
//   - Other API errors -> generic ReviewError (depends on type)
//
// Returns a ReviewError with appropriate code, message, and retryability.
func (a *AnthropicProvider) handleAPIError(err error) error {
	// Check for context errors first
	if ctx, ok := err.(interface{ Unwrap() error }); ok {
		if unwrapped := ctx.Unwrap(); unwrapped == context.Canceled || unwrapped == context.DeadlineExceeded {
			return &ReviewError{
				Code:      "timeout",
				Message:   "request cancelled or timed out",
				Retryable: true,
			}
		}
	}

	// Check for Anthropic-specific errors
	errMsg := err.Error()

	// Authentication errors (401, 403)
	if strings.Contains(errMsg, "401") ||
		strings.Contains(errMsg, "403") ||
		strings.Contains(errMsg, "authentication") ||
		strings.Contains(errMsg, "api_key") {
		return &ReviewError{
			Code:      "invalid_api_key",
			Message:   "API key is invalid or expired",
			Retryable: false,
		}
	}

	// Rate limiting errors (429)
	if strings.Contains(errMsg, "429") ||
		strings.Contains(errMsg, "rate_limit") ||
		strings.Contains(errMsg, "too many requests") {
		return &ReviewError{
			Code:      "rate_limited",
			Message:   "API rate limit exceeded",
			Retryable: true,
		}
	}

	// Quota/billing errors
	if strings.Contains(errMsg, "quota") ||
		strings.Contains(errMsg, "insufficient_quota") ||
		strings.Contains(errMsg, "billing") {
		return &ReviewError{
			Code:      "quota_exceeded",
			Message:   "API quota exceeded",
			Retryable: false,
		}
	}

	// Timeout errors
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline") {
		return &ReviewError{
			Code:      "timeout",
			Message:   "request timed out",
			Retryable: true,
		}
	}

	// Generic API error
	return &ReviewError{
		Code:      "api_error",
		Message:   fmt.Sprintf("Anthropic API error: %v", err),
		Retryable: false,
	}
}
