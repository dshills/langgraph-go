package providers

import (
	"context"
	"time"
)

// MockProvider is a test implementation of the CodeReviewer interface
// that returns pre-configured issues without making actual API calls.
// It's designed for testing workflows and integration scenarios where
// real API credentials are not available or not desired.
//
// MockProvider is safe for concurrent use as it only reads from the
// Issues field, which should be configured before concurrent access.
//
// Example usage:
//
//	mock := &MockProvider{
//	    Issues: []ReviewIssue{
//	        {
//	            File:         "test.go",
//	            Line:         10,
//	            Severity:     "high",
//	            Category:     "security",
//	            Description:  "SQL injection vulnerability",
//	            Remediation:  "Use parameterized queries",
//	            ProviderName: "mock",
//	            Confidence:   0.95,
//	        },
//	    },
//	}
//
//	resp, err := mock.ReviewBatch(ctx, req)
//	// resp.Issues will contain the pre-configured issues
type MockProvider struct {
	// Issues is the list of pre-configured issues to return from ReviewBatch.
	// This field should be set before calling ReviewBatch, typically during
	// test setup. An empty or nil slice will result in no issues being returned.
	//
	// The Issues slice is read-only during ReviewBatch calls, making MockProvider
	// safe for concurrent use after configuration.
	Issues []ReviewIssue
}

// ReviewBatch implements the CodeReviewer interface by returning the
// pre-configured issues from the Issues field. It respects context
// cancellation and returns immediately if the context is cancelled.
//
// Unlike real provider implementations, MockProvider:
// - Returns instantly (after checking context)
// - Consumes no API tokens (TokensUsed = 0)
// - Ignores the request parameters (files, focus areas)
// - Always succeeds unless context is cancelled
//
// This makes it ideal for testing orchestration logic, concurrent batch
// processing, and error handling without the overhead and cost of real
// API calls.
//
// Returns context.Canceled if ctx.Done() is closed before returning.
// Returns context.DeadlineExceeded if ctx deadline was exceeded.
func (m *MockProvider) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	start := time.Now()

	// Respect context cancellation
	select {
	case <-ctx.Done():
		return ReviewResponse{}, ctx.Err()
	default:
	}

	// Return the pre-configured issues
	issues := make([]ReviewIssue, len(m.Issues))
	copy(issues, m.Issues)

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   0,
		Duration:     time.Since(start),
		ProviderName: "mock",
	}, nil
}

// Name implements the CodeReviewer interface and returns "mock" as the
// provider identifier. This name appears in log messages, ReviewResponse.ProviderName,
// and ReviewIssue.ProviderName fields.
func (m *MockProvider) Name() string {
	return "mock"
}

// TokenLimit implements the CodeReviewer interface and returns 100000
// as the mock token limit. This is a reasonable upper bound that's larger
// than most real provider limits, making it suitable for testing without
// artificial constraints.
//
// The value of 100000 is intentionally high because:
// - It won't artificially limit test scenarios
// - It's higher than Claude's 100k context window
// - It's higher than GPT-4's 8k/32k limits
// - It allows testing large batch sizes without hitting limits
func (m *MockProvider) TokenLimit() int {
	return 100000
}
