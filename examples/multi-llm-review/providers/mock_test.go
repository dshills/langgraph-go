package providers

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestMockProvider_ReviewBatch_ReturnsConfiguredIssues verifies that
// MockProvider returns the exact issues configured during setup.
func TestMockProvider_ReviewBatch_ReturnsConfiguredIssues(t *testing.T) {
	tests := []struct {
		name             string
		configuredIssues []ReviewIssue
		request          ReviewRequest
		wantIssueCount   int
	}{
		{
			name: "returns single configured issue",
			configuredIssues: []ReviewIssue{
				{
					File:         "test.go",
					Line:         10,
					Severity:     "high",
					Category:     "security",
					Description:  "Test security issue",
					Remediation:  "Fix the security issue",
					ProviderName: "mock",
					Confidence:   0.95,
				},
			},
			request: ReviewRequest{
				Files: []CodeFile{
					{
						FilePath:  "test.go",
						Content:   "package main\n\nfunc main() {}",
						Language:  "go",
						LineCount: 3,
					},
				},
				FocusAreas:  []string{"security"},
				Language:    "go",
				BatchNumber: 1,
			},
			wantIssueCount: 1,
		},
		{
			name: "returns multiple configured issues",
			configuredIssues: []ReviewIssue{
				{
					File:         "file1.go",
					Line:         5,
					Severity:     "critical",
					Category:     "security",
					Description:  "Critical security flaw",
					Remediation:  "Immediate fix required",
					ProviderName: "mock",
					Confidence:   1.0,
				},
				{
					File:         "file2.go",
					Line:         15,
					Severity:     "medium",
					Category:     "performance",
					Description:  "Performance optimization needed",
					Remediation:  "Consider caching",
					ProviderName: "mock",
					Confidence:   0.8,
				},
			},
			request: ReviewRequest{
				Files: []CodeFile{
					{FilePath: "file1.go", Content: "code1", Language: "go"},
					{FilePath: "file2.go", Content: "code2", Language: "go"},
				},
				FocusAreas:  []string{"security", "performance"},
				Language:    "go",
				BatchNumber: 1,
			},
			wantIssueCount: 2,
		},
		{
			name:             "returns empty list when no issues configured",
			configuredIssues: []ReviewIssue{},
			request: ReviewRequest{
				Files: []CodeFile{
					{FilePath: "clean.go", Content: "perfect code", Language: "go"},
				},
				FocusAreas:  []string{"security"},
				Language:    "go",
				BatchNumber: 1,
			},
			wantIssueCount: 0,
		},
		{
			name:             "handles nil issues slice",
			configuredIssues: nil,
			request: ReviewRequest{
				Files:       []CodeFile{{FilePath: "test.go", Content: "code", Language: "go"}},
				FocusAreas:  []string{"style"},
				Language:    "go",
				BatchNumber: 1,
			},
			wantIssueCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockProvider{
				Issues: tt.configuredIssues,
			}

			ctx := context.Background()
			resp, err := mock.ReviewBatch(ctx, tt.request)

			if err != nil {
				t.Errorf("ReviewBatch() unexpected error = %v", err)
				return
			}

			if len(resp.Issues) != tt.wantIssueCount {
				t.Errorf("ReviewBatch() got %d issues, want %d", len(resp.Issues), tt.wantIssueCount)
			}

			// Verify each returned issue matches configured issue
			for i, got := range resp.Issues {
				if i >= len(tt.configuredIssues) {
					break
				}
				want := tt.configuredIssues[i]

				if got.File != want.File {
					t.Errorf("Issue[%d].File = %q, want %q", i, got.File, want.File)
				}
				if got.Line != want.Line {
					t.Errorf("Issue[%d].Line = %d, want %d", i, got.Line, want.Line)
				}
				if got.Severity != want.Severity {
					t.Errorf("Issue[%d].Severity = %q, want %q", i, got.Severity, want.Severity)
				}
				if got.Category != want.Category {
					t.Errorf("Issue[%d].Category = %q, want %q", i, got.Category, want.Category)
				}
				if got.Description != want.Description {
					t.Errorf("Issue[%d].Description = %q, want %q", i, got.Description, want.Description)
				}
				if got.Remediation != want.Remediation {
					t.Errorf("Issue[%d].Remediation = %q, want %q", i, got.Remediation, want.Remediation)
				}
				if got.ProviderName != want.ProviderName {
					t.Errorf("Issue[%d].ProviderName = %q, want %q", i, got.ProviderName, want.ProviderName)
				}
				if got.Confidence != want.Confidence {
					t.Errorf("Issue[%d].Confidence = %f, want %f", i, got.Confidence, want.Confidence)
				}
			}
		})
	}
}

// TestMockProvider_ReviewBatch_ResponseMetadata verifies that ReviewResponse
// contains correct metadata fields (provider name, duration).
func TestMockProvider_ReviewBatch_ResponseMetadata(t *testing.T) {
	mock := &MockProvider{
		Issues: []ReviewIssue{
			{
				File:         "test.go",
				Line:         1,
				Severity:     "info",
				Category:     "style",
				Description:  "Test issue",
				Remediation:  "Test fix",
				ProviderName: "mock",
				Confidence:   0.9,
			},
		},
	}

	ctx := context.Background()
	req := ReviewRequest{
		Files:       []CodeFile{{FilePath: "test.go", Content: "code", Language: "go"}},
		FocusAreas:  []string{"style"},
		Language:    "go",
		BatchNumber: 1,
	}

	resp, err := mock.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	if resp.ProviderName != "mock" {
		t.Errorf("ReviewBatch() ProviderName = %q, want %q", resp.ProviderName, "mock")
	}

	if resp.Duration <= 0 {
		t.Errorf("ReviewBatch() Duration = %v, want > 0", resp.Duration)
	}

	// TokensUsed should be 0 for mock (no real API call)
	if resp.TokensUsed != 0 {
		t.Errorf("ReviewBatch() TokensUsed = %d, want 0", resp.TokensUsed)
	}
}

// TestMockProvider_Name verifies Name() returns "mock".
func TestMockProvider_Name(t *testing.T) {
	mock := &MockProvider{}

	got := mock.Name()
	want := "mock"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

// TestMockProvider_TokenLimit verifies TokenLimit() returns reasonable value.
func TestMockProvider_TokenLimit(t *testing.T) {
	mock := &MockProvider{}

	got := mock.TokenLimit()
	want := 100000

	if got != want {
		t.Errorf("TokenLimit() = %d, want %d", got, want)
	}

	// Verify it's a reasonable value (greater than typical request sizes)
	if got < 1000 {
		t.Errorf("TokenLimit() = %d is too small, should be >= 1000 for realistic testing", got)
	}
}

// TestMockProvider_ConcurrentCalls verifies that MockProvider can handle
// concurrent ReviewBatch calls safely without data races.
func TestMockProvider_ConcurrentCalls(t *testing.T) {
	// Configure mock with shared issues
	sharedIssues := []ReviewIssue{
		{
			File:         "concurrent.go",
			Line:         42,
			Severity:     "medium",
			Category:     "best-practice",
			Description:  "Concurrent access test",
			Remediation:  "Thread-safe by design",
			ProviderName: "mock",
			Confidence:   0.88,
		},
	}

	mock := &MockProvider{
		Issues: sharedIssues,
	}

	// Launch concurrent goroutines
	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			req := ReviewRequest{
				Files: []CodeFile{
					{
						FilePath:  "concurrent.go",
						Content:   "package main",
						Language:  "go",
						LineCount: 1,
					},
				},
				FocusAreas:  []string{"best-practice"},
				Language:    "go",
				BatchNumber: id,
			}

			resp, err := mock.ReviewBatch(ctx, req)
			if err != nil {
				errChan <- err
				return
			}

			// Verify response contains expected number of issues
			if len(resp.Issues) != len(sharedIssues) {
				errChan <- err
				return
			}

			// Verify response metadata
			if resp.ProviderName != "mock" {
				errChan <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check if any goroutine reported errors
	for err := range errChan {
		t.Errorf("Concurrent call failed: %v", err)
	}
}

// TestMockProvider_ContextCancellation verifies that ReviewBatch respects
// context cancellation.
func TestMockProvider_ContextCancellation(t *testing.T) {
	mock := &MockProvider{
		Issues: []ReviewIssue{
			{
				File:         "test.go",
				Line:         1,
				Severity:     "low",
				Category:     "style",
				Description:  "Should not return this",
				Remediation:  "Context was cancelled",
				ProviderName: "mock",
				Confidence:   0.5,
			},
		},
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := ReviewRequest{
		Files:       []CodeFile{{FilePath: "test.go", Content: "code", Language: "go"}},
		FocusAreas:  []string{"style"},
		Language:    "go",
		BatchNumber: 1,
	}

	_, err := mock.ReviewBatch(ctx, req)

	// Should return context.Canceled error
	if err != context.Canceled {
		t.Errorf("ReviewBatch() with cancelled context error = %v, want %v", err, context.Canceled)
	}
}

// TestMockProvider_ContextTimeout verifies ReviewBatch respects timeout.
func TestMockProvider_ContextTimeout(t *testing.T) {
	mock := &MockProvider{
		Issues: []ReviewIssue{
			{
				File:         "test.go",
				Line:         1,
				Severity:     "info",
				Category:     "style",
				Description:  "Test",
				Remediation:  "Test",
				ProviderName: "mock",
				Confidence:   0.9,
			},
		},
	}

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the timeout a chance to trigger
	time.Sleep(1 * time.Millisecond)

	req := ReviewRequest{
		Files:       []CodeFile{{FilePath: "test.go", Content: "code", Language: "go"}},
		FocusAreas:  []string{"style"},
		Language:    "go",
		BatchNumber: 1,
	}

	_, err := mock.ReviewBatch(ctx, req)

	// Should return context deadline exceeded error
	if err != context.DeadlineExceeded {
		t.Errorf("ReviewBatch() with timeout error = %v, want %v", err, context.DeadlineExceeded)
	}
}

// TestMockProvider_InterfaceCompliance verifies MockProvider implements
// the CodeReviewer interface.
func TestMockProvider_InterfaceCompliance(t *testing.T) {
	var _ CodeReviewer = (*MockProvider)(nil)
}
