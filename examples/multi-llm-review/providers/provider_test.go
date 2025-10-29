package providers

import (
	"context"
	"testing"
	"time"
)

// TestReviewIssueValidation verifies that ReviewIssue.Validate() correctly
// identifies valid and invalid issues.
func TestReviewIssueValidation(t *testing.T) {
	tests := []struct {
		name    string
		issue   ReviewIssue
		wantErr bool
		errCode string
	}{
		{
			name: "valid issue",
			issue: ReviewIssue{
				File:        "main.go",
				Line:        42,
				Severity:    "high",
				Category:    "security",
				Description: "Missing nil check",
				Confidence:  0.95,
			},
			wantErr: false,
		},
		{
			name: "missing file path",
			issue: ReviewIssue{
				File:        "",
				Line:        10,
				Severity:    "medium",
				Category:    "style",
				Description: "Unused variable",
				Confidence:  0.8,
			},
			wantErr: true,
			errCode: "invalid_file",
		},
		{
			name: "negative line number",
			issue: ReviewIssue{
				File:        "util.go",
				Line:        -1,
				Severity:    "low",
				Category:    "best-practice",
				Description: "Comment formatting",
				Confidence:  0.5,
			},
			wantErr: true,
			errCode: "invalid_line",
		},
		{
			name: "invalid severity",
			issue: ReviewIssue{
				File:        "pkg.go",
				Line:        5,
				Severity:    "catastrophic",
				Category:    "performance",
				Description: "Slow algorithm",
				Confidence:  0.7,
			},
			wantErr: true,
			errCode: "invalid_severity",
		},
		{
			name: "invalid category",
			issue: ReviewIssue{
				File:        "pkg.go",
				Line:        5,
				Severity:    "high",
				Category:    "correctness",
				Description: "Logic error",
				Confidence:  0.7,
			},
			wantErr: true,
			errCode: "invalid_category",
		},
		{
			name: "empty description",
			issue: ReviewIssue{
				File:        "pkg.go",
				Line:        5,
				Severity:    "info",
				Category:    "style",
				Description: "",
				Confidence:  0.5,
			},
			wantErr: true,
			errCode: "empty_description",
		},
		{
			name: "confidence too high",
			issue: ReviewIssue{
				File:        "pkg.go",
				Line:        5,
				Severity:    "medium",
				Category:    "best-practice",
				Description: "Consider refactoring",
				Confidence:  1.5,
			},
			wantErr: true,
			errCode: "invalid_confidence",
		},
		{
			name: "confidence negative",
			issue: ReviewIssue{
				File:        "pkg.go",
				Line:        5,
				Severity:    "low",
				Category:    "style",
				Description: "Minor style issue",
				Confidence:  -0.1,
			},
			wantErr: true,
			errCode: "invalid_confidence",
		},
		{
			name: "file-level issue (line 0)",
			issue: ReviewIssue{
				File:        "module.go",
				Line:        0,
				Severity:    "info",
				Category:    "best-practice",
				Description: "Module needs documentation",
				Confidence:  0.6,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				revErr, ok := err.(*ReviewError)
				if !ok {
					t.Errorf("error is not ReviewError: %T", err)
				} else if revErr.Code != tt.errCode {
					t.Errorf("error code = %s, want %s", revErr.Code, tt.errCode)
				}
			}
		})
	}
}

// TestReviewErrorIsRetryable verifies that IsRetryable() correctly identifies
// transient vs permanent errors.
func TestReviewErrorIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       *ReviewError
		wantRetry bool
	}{
		{
			name:      "rate limited is retryable",
			err:       ErrRateLimited,
			wantRetry: true,
		},
		{
			name:      "timeout is retryable",
			err:       ErrTimeout,
			wantRetry: true,
		},
		{
			name:      "invalid API key is not retryable",
			err:       ErrInvalidAPIKey,
			wantRetry: false,
		},
		{
			name:      "quota exceeded is not retryable",
			err:       ErrQuotaExceeded,
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetryable(); got != tt.wantRetry {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

// TestCodeReviewerImplementation shows how to implement the CodeReviewer interface.
// This is a mock implementation for testing purposes.
type mockReviewer struct {
	name       string
	tokenLimit int
}

func (m *mockReviewer) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	// Simulate review processing
	return ReviewResponse{
		Issues:       []ReviewIssue{},
		TokensUsed:   100,
		Duration:     100 * time.Millisecond,
		ProviderName: m.name,
	}, nil
}

func (m *mockReviewer) Name() string {
	return m.name
}

func (m *mockReviewer) TokenLimit() int {
	return m.tokenLimit
}

// TestCodeReviewerInterface verifies that the CodeReviewer interface can be
// implemented and called correctly.
func TestCodeReviewerInterface(t *testing.T) {
	// Verify mock implements CodeReviewer
	var _ CodeReviewer = &mockReviewer{}

	reviewer := &mockReviewer{
		name:       "mock",
		tokenLimit: 4096,
	}

	if got := reviewer.Name(); got != "mock" {
		t.Errorf("Name() = %s, want mock", got)
	}

	if got := reviewer.TokenLimit(); got != 4096 {
		t.Errorf("TokenLimit() = %d, want 4096", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := reviewer.ReviewBatch(ctx, ReviewRequest{
		Files:       []CodeFile{},
		FocusAreas:  []string{"security", "performance"},
		Language:    "go",
		BatchNumber: 1,
	})

	if err != nil {
		t.Errorf("ReviewBatch() error = %v", err)
	}

	if response.ProviderName != "mock" {
		t.Errorf("ReviewResponse.ProviderName = %s, want mock", response.ProviderName)
	}
}
