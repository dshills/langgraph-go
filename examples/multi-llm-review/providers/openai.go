package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAIProvider implements the CodeReviewer interface using OpenAI's GPT models.
// It wraps the official OpenAI Go SDK and provides code review functionality
// with structured JSON output parsing.
//
// The provider is safe for concurrent use as the underlying OpenAI client
// handles thread-safety internally.
//
// Example usage:
//
//	provider, err := NewOpenAIProvider("sk-...", "gpt-4")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	resp, err := provider.ReviewBatch(ctx, ReviewRequest{
//	    Files: []CodeFile{{FilePath: "main.go", Content: code, Language: "go"}},
//	    FocusAreas: []string{"security"},
//	    Language: "go",
//	})
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI code review provider.
//
// Parameters:
//   - apiKey: OpenAI API key (must start with "sk-")
//   - model: Model to use (e.g., "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo")
//
// Returns error if apiKey or model is empty.
func NewOpenAIProvider(apiKey, model string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, errors.New("API key cannot be empty")
	}
	if model == "" {
		return nil, errors.New("model cannot be empty")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &OpenAIProvider{
		client: &client,
		model:  model,
	}, nil
}

// Name returns "openai" as the provider identifier.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// TokenLimit returns the maximum token limit for GPT-4 models.
// This is used for dynamic batch sizing to avoid exceeding API limits.
func (p *OpenAIProvider) TokenLimit() int {
	return 128000 // GPT-4 token limit
}

// ReviewBatch performs a code review on the provided files using OpenAI's API.
//
// It sends the code with a structured prompt requesting JSON-formatted issues,
// parses the response, and returns a ReviewResponse with identified issues.
//
// The method respects context cancellation and timeouts. It handles various
// API errors and maps them to appropriate ReviewError types (retryable vs permanent).
func (p *OpenAIProvider) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	startTime := time.Now()

	// Handle empty file list
	if len(req.Files) == 0 {
		return ReviewResponse{
			Issues:       []ReviewIssue{},
			TokensUsed:   0,
			Duration:     time.Since(startTime),
			ProviderName: p.Name(),
		}, nil
	}

	// Build the review prompt
	prompt := p.buildPrompt(req)

	// Call OpenAI API with JSON mode
	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return ReviewResponse{}, err
	}

	// Parse the JSON response into issues
	issues, err := p.parseResponse(response.Content, req.Files)
	if err != nil {
		return ReviewResponse{}, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	// Set provider name on all issues
	for i := range issues {
		issues[i].ProviderName = p.Name()
	}

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   response.TokensUsed,
		Duration:     time.Since(startTime),
		ProviderName: p.Name(),
	}, nil
}

// buildPrompt constructs the system and user prompts for code review.
func (p *OpenAIProvider) buildPrompt(req ReviewRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer. Analyze the following ")
	sb.WriteString(req.Language)
	sb.WriteString(" code and identify issues.\n\n")

	if len(req.FocusAreas) > 0 {
		sb.WriteString("Focus on these areas: ")
		sb.WriteString(strings.Join(req.FocusAreas, ", "))
		sb.WriteString("\n\n")
	}

	sb.WriteString("For each issue found, return a JSON object with this structure:\n")
	sb.WriteString(`{
  "issues": [
    {
      "file": "filename.go",
      "line": 10,
      "severity": "critical|high|medium|low|info",
      "category": "security|performance|style|best-practice",
      "description": "Brief description of the issue",
      "remediation": "How to fix it",
      "confidence": 0.95
    }
  ]
}

`)

	sb.WriteString("Code files to review:\n\n")
	for _, file := range req.Files {
		sb.WriteString("--- File: ")
		sb.WriteString(file.FilePath)
		sb.WriteString(" ---\n")
		sb.WriteString(file.Content)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Respond ONLY with valid JSON. No markdown, no explanation, just the JSON object.")

	return sb.String()
}

// apiResponse holds the parsed response from OpenAI.
type apiResponse struct {
	Content    string
	TokensUsed int
}

// callAPI makes the actual API call to OpenAI.
func (p *OpenAIProvider) callAPI(ctx context.Context, prompt string) (*apiResponse, error) {
	// Check context before making expensive API call
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Create chat completion request
	completion, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: shared.ChatModel(p.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(prompt),
					},
				},
			},
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: openai.Ptr(shared.NewResponseFormatJSONObjectParam()),
		},
		Temperature: openai.Float(1.0), // Default temperature (some models only support 1.0)
	})

	if err != nil {
		return nil, p.mapError(err)
	}

	// Extract content and token usage
	if len(completion.Choices) == 0 {
		return nil, errors.New("no response from OpenAI API")
	}

	content := completion.Choices[0].Message.Content
	tokensUsed := int(completion.Usage.TotalTokens)

	return &apiResponse{
		Content:    content,
		TokensUsed: tokensUsed,
	}, nil
}

// parseResponse parses the JSON response from OpenAI into ReviewIssue structs.
func (p *OpenAIProvider) parseResponse(content string, files []CodeFile) ([]ReviewIssue, error) {
	// Clean up any markdown code blocks that might wrap the JSON
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Parse JSON response
	var result struct {
		Issues []ReviewIssue `json:"issues"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Validate each issue
	validIssues := make([]ReviewIssue, 0, len(result.Issues))
	for _, issue := range result.Issues {
		// Set default confidence if not provided
		if issue.Confidence == 0 {
			issue.Confidence = 0.8
		}

		// Validate the issue
		if err := issue.Validate(); err != nil {
			// Log validation error but continue with other issues
			continue
		}

		validIssues = append(validIssues, issue)
	}

	return validIssues, nil
}

// mapError converts OpenAI API errors to ReviewError types.
// It distinguishes between retryable transient failures and permanent failures.
func (p *OpenAIProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()
	lowerErr := strings.ToLower(errStr)

	// Check for context errors first
	if errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ReviewError{
			Code:      "timeout",
			Message:   "OpenAI API request timed out",
			Retryable: true,
		}
	}

	// Rate limit errors (retryable)
	if strings.Contains(lowerErr, "rate limit") ||
		strings.Contains(lowerErr, "429") ||
		strings.Contains(lowerErr, "too many requests") {
		return &ReviewError{
			Code:      "rate_limited",
			Message:   "OpenAI API rate limit exceeded",
			Retryable: true,
		}
	}

	// Authentication errors (permanent)
	if strings.Contains(lowerErr, "invalid api key") ||
		strings.Contains(lowerErr, "incorrect api key") ||
		strings.Contains(lowerErr, "401") ||
		strings.Contains(lowerErr, "unauthorized") ||
		strings.Contains(lowerErr, "authentication") {
		return &ReviewError{
			Code:      "invalid_api_key",
			Message:   "OpenAI API key is invalid or expired",
			Retryable: false,
		}
	}

	// Quota exceeded errors (permanent)
	if strings.Contains(lowerErr, "quota") ||
		strings.Contains(lowerErr, "insufficient_quota") ||
		strings.Contains(lowerErr, "billing") {
		return &ReviewError{
			Code:      "quota_exceeded",
			Message:   "OpenAI API quota exceeded",
			Retryable: false,
		}
	}

	// Server errors (retryable)
	if strings.Contains(lowerErr, "500") ||
		strings.Contains(lowerErr, "502") ||
		strings.Contains(lowerErr, "503") ||
		strings.Contains(lowerErr, "504") ||
		strings.Contains(lowerErr, "internal server error") ||
		strings.Contains(lowerErr, "bad gateway") ||
		strings.Contains(lowerErr, "service unavailable") ||
		strings.Contains(lowerErr, "gateway timeout") {
		return &ReviewError{
			Code:      "server_error",
			Message:   fmt.Sprintf("OpenAI API server error: %v", err),
			Retryable: true,
		}
	}

	// Network errors (retryable)
	if strings.Contains(lowerErr, "connection") ||
		strings.Contains(lowerErr, "timeout") ||
		strings.Contains(lowerErr, "network") {
		return &ReviewError{
			Code:      "network_error",
			Message:   fmt.Sprintf("Network error calling OpenAI API: %v", err),
			Retryable: true,
		}
	}

	// Default: wrap as generic error (not retryable by default)
	return &ReviewError{
		Code:      "api_error",
		Message:   fmt.Sprintf("OpenAI API error: %v", err),
		Retryable: false,
	}
}
