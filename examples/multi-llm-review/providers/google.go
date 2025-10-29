package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GoogleProvider implements the CodeReviewer interface using Google's Gemini API.
// It wraps the official generative-ai-go client and adapts it to the CodeReviewer interface.
//
// The provider uses Gemini models (default: gemini-1.5-flash) to perform code reviews
// with structured JSON output for consistent issue parsing.
//
// Example usage:
//
//	provider, err := NewGoogleProvider("", "gemini-1.5-flash")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer provider.Close()
//
//	resp, err := provider.ReviewBatch(ctx, req)
type GoogleProvider struct {
	client *genai.Client
	model  string
}

const (
	// DefaultGoogleModel is the default Gemini model to use for code review.
	DefaultGoogleModel = "gemini-1.5-flash"

	// GoogleTokenLimit is the maximum number of tokens Gemini can process.
	// Gemini 1.5 has a 1M token context window, but we use a conservative limit.
	GoogleTokenLimit = 32000
)

// NewGoogleProvider creates a new Google Gemini code review provider.
//
// Parameters:
//   - apiKey: Google API key. If empty, reads from GOOGLE_API_KEY environment variable.
//   - model: Gemini model to use (e.g., "gemini-1.5-flash", "gemini-pro").
//     If empty, uses DefaultGoogleModel.
//
// Returns an error if the API key is not provided and cannot be found in the environment.
func NewGoogleProvider(apiKey, model string) (*GoogleProvider, error) {
	// Use environment variable if API key not provided
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, &ReviewError{
				Code:      "missing_api_key",
				Message:   "Google API key not provided and GOOGLE_API_KEY environment variable not set",
				Retryable: false,
			}
		}
	}

	// Use default model if not specified
	if model == "" {
		model = DefaultGoogleModel
	}

	// Create Gemini client
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Google client: %w", err)
	}

	return &GoogleProvider{
		client: client,
		model:  model,
	}, nil
}

// Close closes the underlying Gemini client and releases resources.
// Should be called when the provider is no longer needed.
func (g *GoogleProvider) Close() error {
	if g.client != nil {
		return g.client.Close()
	}
	return nil
}

// Name returns "google" as the provider identifier.
func (g *GoogleProvider) Name() string {
	return "google"
}

// TokenLimit returns the maximum number of tokens this provider can process.
func (g *GoogleProvider) TokenLimit() int {
	return GoogleTokenLimit
}

// ReviewBatch performs a code review using Google's Gemini API.
// It sends the code files to Gemini with a structured prompt requesting JSON output,
// then parses the response into ReviewIssue structs.
//
// The method respects context cancellation and returns appropriate errors for
// different failure modes (rate limits, invalid API key, timeouts, etc.).
func (g *GoogleProvider) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	startTime := time.Now()

	// Handle empty file list
	if len(req.Files) == 0 {
		return ReviewResponse{
			Issues:       []ReviewIssue{},
			TokensUsed:   0,
			Duration:     time.Since(startTime),
			ProviderName: g.Name(),
		}, nil
	}

	// Build the review prompt
	prompt := buildGoogleReviewPrompt(req)

	// Get the generative model
	model := g.client.GenerativeModel(g.model)

	// Configure model for structured JSON output
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"issues": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"file":        {Type: genai.TypeString},
						"line":        {Type: genai.TypeInteger},
						"severity":    {Type: genai.TypeString},
						"category":    {Type: genai.TypeString},
						"description": {Type: genai.TypeString},
						"remediation": {Type: genai.TypeString},
						"confidence":  {Type: genai.TypeNumber},
					},
					Required: []string{"file", "line", "severity", "category", "description", "confidence"},
				},
			},
		},
		Required: []string{"issues"},
	}

	// Generate content
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return ReviewResponse{}, handleGoogleError(err)
	}

	// Parse the response
	issues, tokensUsed, err := parseGoogleResponse(resp, g.Name())
	if err != nil {
		return ReviewResponse{}, fmt.Errorf("failed to parse Google response: %w", err)
	}

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   tokensUsed,
		Duration:     time.Since(startTime),
		ProviderName: g.Name(),
	}, nil
}

// buildGoogleReviewPrompt constructs the prompt for the Gemini API.
// It includes the files to review, focus areas, and instructions for structured output.
func buildGoogleReviewPrompt(req ReviewRequest) string {
	var sb strings.Builder

	sb.WriteString("You are a code review expert. Review the following ")
	sb.WriteString(req.Language)
	sb.WriteString(" code and identify issues.\n\n")

	if len(req.FocusAreas) > 0 {
		sb.WriteString("Focus on these areas: ")
		sb.WriteString(strings.Join(req.FocusAreas, ", "))
		sb.WriteString("\n\n")
	}

	sb.WriteString("Files to review:\n\n")
	for _, file := range req.Files {
		sb.WriteString("File: ")
		sb.WriteString(file.FilePath)
		sb.WriteString("\n```")
		sb.WriteString(file.Language)
		sb.WriteString("\n")
		sb.WriteString(file.Content)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("For each issue found, provide:\n")
	sb.WriteString("- file: exact file path from above\n")
	sb.WriteString("- line: line number (1-indexed, use 0 for file-level issues)\n")
	sb.WriteString("- severity: one of [critical, high, medium, low, info]\n")
	sb.WriteString("- category: one of [security, performance, style, best-practice]\n")
	sb.WriteString("- description: clear explanation of the issue\n")
	sb.WriteString("- remediation: how to fix it (optional, can be empty string)\n")
	sb.WriteString("- confidence: your confidence level (0.0 to 1.0)\n\n")
	sb.WriteString("Return your findings as JSON with an 'issues' array. ")
	sb.WriteString("If no issues are found, return an empty issues array.")

	return sb.String()
}

// parseGoogleResponse extracts ReviewIssue structs from the Gemini API response.
// It handles the JSON parsing and validates each issue.
func parseGoogleResponse(resp *genai.GenerateContentResponse, providerName string) ([]ReviewIssue, int, error) {
	if resp == nil {
		return nil, 0, fmt.Errorf("nil response from Google API")
	}

	// Calculate token usage
	tokensUsed := 0
	if resp.UsageMetadata != nil {
		tokensUsed = int(resp.UsageMetadata.TotalTokenCount)
	}

	// Check if response has content
	if len(resp.Candidates) == 0 {
		return []ReviewIssue{}, tokensUsed, nil
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return []ReviewIssue{}, tokensUsed, nil
	}

	// Extract text from the first part
	var responseText string
	for _, part := range candidate.Content.Parts {
		if text, ok := part.(genai.Text); ok {
			responseText = string(text)
			break
		}
	}

	if responseText == "" {
		return []ReviewIssue{}, tokensUsed, nil
	}

	// Parse JSON response
	var result struct {
		Issues []struct {
			File        string  `json:"file"`
			Line        int     `json:"line"`
			Severity    string  `json:"severity"`
			Category    string  `json:"category"`
			Description string  `json:"description"`
			Remediation string  `json:"remediation"`
			Confidence  float64 `json:"confidence"`
		} `json:"issues"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return nil, tokensUsed, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	// Convert to ReviewIssue structs
	issues := make([]ReviewIssue, 0, len(result.Issues))
	for _, item := range result.Issues {
		issue := ReviewIssue{
			File:         item.File,
			Line:         item.Line,
			Severity:     item.Severity,
			Category:     item.Category,
			Description:  item.Description,
			Remediation:  item.Remediation,
			ProviderName: providerName,
			Confidence:   item.Confidence,
		}

		// Validate the issue
		if err := issue.Validate(); err != nil {
			// Log invalid issue but continue processing others
			continue
		}

		issues = append(issues, issue)
	}

	return issues, tokensUsed, nil
}

// handleGoogleError converts Google API errors to ReviewError with appropriate codes and retryability.
func handleGoogleError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context cancellation/timeout
	if ctx := context.Cause(context.Background()); ctx != nil {
		if ctx == context.Canceled || ctx == context.DeadlineExceeded {
			return &ReviewError{
				Code:      "timeout",
				Message:   fmt.Sprintf("request timed out: %v", err),
				Retryable: true,
			}
		}
	}

	errMsg := err.Error()
	lowerMsg := strings.ToLower(errMsg)

	// Check for API key errors
	if strings.Contains(lowerMsg, "api key") ||
		strings.Contains(lowerMsg, "authentication") ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "invalid_api_key") {
		return &ReviewError{
			Code:      "invalid_api_key",
			Message:   fmt.Sprintf("invalid or missing API key: %v", err),
			Retryable: false,
		}
	}

	// Check for rate limiting
	if strings.Contains(lowerMsg, "rate limit") ||
		strings.Contains(lowerMsg, "quota") ||
		strings.Contains(lowerMsg, "too many requests") ||
		strings.Contains(lowerMsg, "resource_exhausted") {
		return &ReviewError{
			Code:      "rate_limited",
			Message:   fmt.Sprintf("rate limit exceeded: %v", err),
			Retryable: true,
		}
	}

	// Check for quota exceeded (permanent)
	if strings.Contains(lowerMsg, "quota exceeded") ||
		strings.Contains(lowerMsg, "billing") {
		return &ReviewError{
			Code:      "quota_exceeded",
			Message:   fmt.Sprintf("quota exceeded: %v", err),
			Retryable: false,
		}
	}

	// Default to a generic retryable error
	return &ReviewError{
		Code:      "api_error",
		Message:   fmt.Sprintf("Google API error: %v", err),
		Retryable: true,
	}
}
