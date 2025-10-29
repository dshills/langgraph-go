// Package contracts defines the interfaces and types for the Multi-LLM Code Review workflow.
// This file serves as the contract specification for the CodeReviewer interface.
package contracts

import (
	"context"
	"time"
)

// CodeReviewer is the interface that all AI provider adapters must implement
// to provide code review functionality. This interface abstracts away the
// provider-specific details of OpenAI, Anthropic, and Google APIs.
type CodeReviewer interface {
	// ReviewBatch performs a code review on a batch of files and returns
	// a list of issues found. This method may be called concurrently for
	// different batches and different providers.
	//
	// Context is used for cancellation and timeout control.
	// Request contains the files to review and configuration.
	//
	// Returns ReviewResponse with issues found, or error if review fails.
	// Errors should be retryable (rate limit, timeout) or permanent (invalid API key).
	ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error)

	// Name returns the provider name for logging and reporting.
	// Must return one of: "openai", "anthropic", "google".
	Name() string

	// TokenLimit returns the maximum number of tokens this provider can process
	// in a single request. Used for dynamic batch sizing.
	TokenLimit() int
}

// ReviewRequest contains the input for a code review operation.
type ReviewRequest struct {
	// Files to review in this batch
	Files []CodeFile `json:"files"`

	// FocusAreas specifies what aspects to review
	// Valid values: "security", "performance", "style", "best-practices"
	FocusAreas []string `json:"focus_areas"`

	// Language of the code being reviewed (e.g., "go", "python", "javascript")
	// Used to tailor review prompts to language-specific idioms
	Language string `json:"language"`

	// BatchNumber identifies which batch this is (for logging/checkpointing)
	BatchNumber int `json:"batch_number"`
}

// ReviewResponse contains the output from a code review operation.
type ReviewResponse struct {
	// Issues found during the review
	Issues []ReviewIssue `json:"issues"`

	// TokensUsed indicates how many API tokens were consumed
	// Used for cost tracking and rate limiting
	TokensUsed int `json:"tokens_used"`

	// Duration of the review operation
	Duration time.Duration `json:"duration"`

	// ProviderName identifies which AI provider performed the review
	ProviderName string `json:"provider_name"`
}

// CodeFile represents a source code file to be reviewed.
type CodeFile struct {
	FilePath  string `json:"file_path"`  // Relative path from codebase root
	Content   string `json:"content"`    // Full file content
	Language  string `json:"language"`   // Detected language
	LineCount int    `json:"line_count"` // Number of lines
	SizeBytes int64  `json:"size_bytes"` // File size in bytes
	Checksum  string `json:"checksum"`   // SHA-256 for change detection
}

// ReviewIssue represents a single code quality issue identified during review.
type ReviewIssue struct {
	File         string  `json:"file"`        // File path where issue found
	Line         int     `json:"line"`        // Line number (0 = file-level)
	Severity     string  `json:"severity"`    // critical|high|medium|low|info
	Category     string  `json:"category"`    // security|performance|style|best-practice
	Description  string  `json:"description"` // What the issue is
	Remediation  string  `json:"remediation"` // How to fix it
	ProviderName string  `json:"provider"`    // Provider that identified it
	Confidence   float64 `json:"confidence"`  // 0.0-1.0 confidence score
}

// Validate checks if a ReviewIssue is well-formed.
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

// Common errors
var (
	ErrInvalidFile       = &ReviewError{Code: "invalid_file", Message: "file path cannot be empty"}
	ErrInvalidLine       = &ReviewError{Code: "invalid_line", Message: "line number cannot be negative"}
	ErrInvalidSeverity   = &ReviewError{Code: "invalid_severity", Message: "severity must be one of: critical, high, medium, low, info"}
	ErrInvalidCategory   = &ReviewError{Code: "invalid_category", Message: "category must be one of: security, performance, style, best-practice"}
	ErrEmptyDescription  = &ReviewError{Code: "empty_description", Message: "description cannot be empty"}
	ErrInvalidConfidence = &ReviewError{Code: "invalid_confidence", Message: "confidence must be between 0.0 and 1.0"}
	ErrRateLimited       = &ReviewError{Code: "rate_limited", Message: "API rate limit exceeded", Retryable: true}
	ErrTimeout           = &ReviewError{Code: "timeout", Message: "request timed out", Retryable: true}
	ErrInvalidAPIKey     = &ReviewError{Code: "invalid_api_key", Message: "API key is invalid or expired"}
	ErrQuotaExceeded     = &ReviewError{Code: "quota_exceeded", Message: "API quota exceeded"}
)

// ReviewError represents an error that occurred during code review.
type ReviewError struct {
	Code      string // Machine-readable error code
	Message   string // Human-readable error message
	Retryable bool   // Whether the operation can be retried
}

func (e *ReviewError) Error() string {
	return e.Message
}

// IsRetryable returns true if the error indicates a transient failure
// that can be retried with exponential backoff.
func (e *ReviewError) IsRetryable() bool {
	return e.Retryable
}
