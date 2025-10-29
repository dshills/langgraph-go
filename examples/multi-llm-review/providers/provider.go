package providers

import (
	"context"
	"time"
)

// CodeReviewer is the interface that all AI provider adapters must implement
// to provide code review functionality. This interface abstracts away the
// provider-specific details of OpenAI, Anthropic, and Google APIs.
//
// Implementations should be safe for concurrent use and should respect
// context cancellation and timeouts. Each provider implementation should
// handle rate limiting, retries, and quota management according to the
// provider's specific requirements.
//
// Example usage:
//
//	// Create a provider adapter
//	reviewer := NewOpenAIReviewer(apiKey, model)
//
//	// Review code files
//	response, err := reviewer.ReviewBatch(ctx, ReviewRequest{
//	    Files: []CodeFile{{
//	        FilePath: "main.go",
//	        Content:  codeContent,
//	        Language: "go",
//	    }},
//	    FocusAreas: []string{"security", "performance"},
//	    Language:   "go",
//	    BatchNumber: 1,
//	})
//	if err != nil {
//	    // Handle error - check IsRetryable() for transient failures
//	    return err
//	}
//
//	// Process review results
//	for _, issue := range response.Issues {
//	    log.Printf("[%s] %s at line %d: %s",
//	        issue.Severity, issue.File, issue.Line, issue.Description)
//	}
type CodeReviewer interface {
	// ReviewBatch performs a code review on a batch of files and returns
	// a list of issues found. This method may be called concurrently for
	// different batches and different providers.
	//
	// The context parameter is used for cancellation and timeout control.
	// Implementations should respect context cancellation and return
	// immediately when ctx.Done() is signaled.
	//
	// The ReviewRequest contains the files to review and configuration,
	// including focus areas (security, performance, style, best-practices)
	// and the programming language for language-specific analysis.
	//
	// Returns ReviewResponse with issues found, or error if review fails.
	// Errors should be distinguishable as retryable (rate limit, timeout)
	// or permanent (invalid API key, quota exceeded). Use the IsRetryable()
	// method on error types that implement it to determine if a retry
	// should be attempted with exponential backoff.
	//
	// Common error conditions:
	// - ErrRateLimited: API rate limit exceeded (retryable)
	// - ErrTimeout: Request timed out (retryable)
	// - ErrInvalidAPIKey: API key is invalid or expired (permanent)
	// - ErrQuotaExceeded: API quota exceeded (permanent)
	ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error)

	// Name returns the provider name for logging and reporting.
	// Must return one of: "openai", "anthropic", "google".
	//
	// This value is used:
	// - In log messages for provider identification
	// - In ReviewResponse.ProviderName and ReviewIssue.ProviderName
	// - For aggregating results from multiple providers
	// - For provider-specific error handling logic
	//
	// Example:
	//
	//	provider.Name() // returns "openai"
	Name() string

	// TokenLimit returns the maximum number of tokens this provider can process
	// in a single request. Used for dynamic batch sizing to avoid exceeding
	// provider limits.
	//
	// Implementations should return the maximum tokens the model can accept
	// for a single request, accounting for both input and output tokens.
	// The actual batch size used may be smaller to ensure reviews complete
	// within the token limit while leaving room for response generation.
	//
	// Example:
	//
	//	openaiReviewer.TokenLimit()     // returns 4096
	//	anthropicReviewer.TokenLimit()  // returns 100000
	//	googleReviewer.TokenLimit()     // returns 30000
	TokenLimit() int
}

// ReviewRequest contains the input for a code review operation.
type ReviewRequest struct {
	// Files to review in this batch. Each file should contain the full content
	// and metadata (language, size, checksum) for accurate analysis.
	Files []CodeFile

	// FocusAreas specifies what aspects to review.
	// Valid values: "security", "performance", "style", "best-practices"
	// Multiple focus areas can be specified for comprehensive reviews.
	// An empty slice indicates a general-purpose review.
	FocusAreas []string

	// Language of the code being reviewed (e.g., "go", "python", "javascript").
	// Used to tailor review prompts to language-specific idioms and conventions.
	Language string

	// BatchNumber identifies which batch this is (for logging/checkpointing).
	// Used to correlate batch results with input state in multi-batch workflows.
	BatchNumber int
}

// ReviewResponse contains the output from a code review operation.
type ReviewResponse struct {
	// Issues found during the review. Each issue represents a specific
	// code quality concern with location, severity, and remediation guidance.
	Issues []ReviewIssue

	// TokensUsed indicates how many API tokens were consumed.
	// Used for cost tracking and rate limiting decisions.
	TokensUsed int

	// Duration of the review operation. Useful for performance analysis
	// and request timing statistics across providers.
	Duration time.Duration

	// ProviderName identifies which AI provider performed the review.
	// Should match the value returned by the provider's Name() method.
	ProviderName string
}

// CodeFile represents a source code file to be reviewed.
type CodeFile struct {
	// FilePath is the relative path from codebase root.
	// Used in ReviewIssue.File to reference locations.
	FilePath string

	// Content is the full file content to be reviewed.
	// Should include all lines for accurate line number reporting.
	Content string

	// Language is the detected or specified language (e.g., "go", "py", "js").
	// Should match the Language field in ReviewRequest for consistency.
	Language string

	// LineCount is the number of lines in the file.
	// Used for validation and batch sizing calculations.
	LineCount int

	// SizeBytes is the file size in bytes.
	// Used for quota and token estimation.
	SizeBytes int64

	// Checksum is SHA-256 hash of the file content.
	// Used for change detection and deduplication across review runs.
	Checksum string
}

// ReviewIssue represents a single code quality issue identified during review.
type ReviewIssue struct {
	// File is the path where issue found (should match a CodeFile.FilePath).
	File string

	// Line is the line number (1-indexed). Use 0 for file-level issues
	// that don't correspond to a specific line.
	Line int

	// Severity is the issue priority level.
	// Valid values: "critical", "high", "medium", "low", "info"
	// Used for sorting and filtering by reporters.
	Severity string

	// Category is the type of issue.
	// Valid values: "security", "performance", "style", "best-practice"
	// Should match or be related to request FocusAreas.
	Category string

	// Description explains what the issue is in 1-2 sentences.
	// Should be specific to this occurrence, not a generic explanation.
	Description string

	// Remediation suggests how to fix the issue.
	// Should be actionable and specific to the code context.
	Remediation string

	// ProviderName identifies which provider found this issue.
	// Useful for provider-specific validation or filtering.
	ProviderName string

	// Confidence is a 0.0-1.0 confidence score for the issue.
	// Used for sorting and filtering; 1.0 = definite, 0.5 = uncertain.
	// Providers may return different confidence levels for the same issue.
	Confidence float64
}

// Validate checks if a ReviewIssue is well-formed and returns an error
// if any required fields are missing or invalid.
//
// Validation checks:
// - File path is not empty
// - Line number is non-negative
// - Severity is one of: critical, high, medium, low, info
// - Category is one of: security, performance, style, best-practice
// - Description is not empty
// - Confidence is between 0.0 and 1.0 (inclusive)
func (r ReviewIssue) Validate() error {
	if r.File == "" {
		return ErrInvalidFile
	}
	if r.Line < 0 {
		return ErrInvalidLine
	}
	if !isValidSeverity(r.Severity) {
		return ErrInvalidSeverity
	}
	if !isValidCategory(r.Category) {
		return ErrInvalidCategory
	}
	if r.Description == "" {
		return ErrEmptyDescription
	}
	if r.Confidence < 0.0 || r.Confidence > 1.0 {
		return ErrInvalidConfidence
	}
	return nil
}

func isValidSeverity(s string) bool {
	return s == "critical" || s == "high" || s == "medium" || s == "low" || s == "info"
}

func isValidCategory(c string) bool {
	return c == "security" || c == "performance" || c == "style" || c == "best-practice"
}

// Common error sentinels for code review operations.
var (
	// ErrInvalidFile indicates the file path is empty or invalid.
	ErrInvalidFile = &ReviewError{Code: "invalid_file", Message: "file path cannot be empty"}

	// ErrInvalidLine indicates the line number is negative.
	ErrInvalidLine = &ReviewError{Code: "invalid_line", Message: "line number cannot be negative"}

	// ErrInvalidSeverity indicates an invalid severity value.
	ErrInvalidSeverity = &ReviewError{Code: "invalid_severity", Message: "severity must be one of: critical, high, medium, low, info"}

	// ErrInvalidCategory indicates an invalid category value.
	ErrInvalidCategory = &ReviewError{Code: "invalid_category", Message: "category must be one of: security, performance, style, best-practice"}

	// ErrEmptyDescription indicates the description is empty.
	ErrEmptyDescription = &ReviewError{Code: "empty_description", Message: "description cannot be empty"}

	// ErrInvalidConfidence indicates the confidence score is out of range.
	ErrInvalidConfidence = &ReviewError{Code: "invalid_confidence", Message: "confidence must be between 0.0 and 1.0"}

	// ErrRateLimited indicates the API rate limit was exceeded (retryable).
	ErrRateLimited = &ReviewError{Code: "rate_limited", Message: "API rate limit exceeded", Retryable: true}

	// ErrTimeout indicates the request exceeded the timeout (retryable).
	ErrTimeout = &ReviewError{Code: "timeout", Message: "request timed out", Retryable: true}

	// ErrInvalidAPIKey indicates the API key is invalid or expired (permanent).
	ErrInvalidAPIKey = &ReviewError{Code: "invalid_api_key", Message: "API key is invalid or expired"}

	// ErrQuotaExceeded indicates the API quota has been exceeded (permanent).
	ErrQuotaExceeded = &ReviewError{Code: "quota_exceeded", Message: "API quota exceeded"}
)

// ReviewError represents an error that occurred during code review operations.
// It distinguishes between retryable transient failures and permanent failures.
type ReviewError struct {
	// Code is the machine-readable error code for programmatic handling.
	Code string

	// Message is the human-readable error message for logging and display.
	Message string

	// Retryable indicates whether the operation can be retried with backoff.
	// True for transient failures (rate limits, timeouts).
	// False for permanent failures (invalid credentials, quota exceeded).
	Retryable bool
}

// Error implements the error interface and returns the human-readable message.
func (e *ReviewError) Error() string {
	return e.Message
}

// IsRetryable returns true if the error indicates a transient failure
// that can be retried with exponential backoff. Returns false for
// permanent failures that require user intervention (e.g., API key change).
func (e *ReviewError) IsRetryable() bool {
	return e.Retryable
}
