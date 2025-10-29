package providers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestGoogleProvider_Name verifies that the provider identifies itself correctly.
func TestGoogleProvider_Name(t *testing.T) {
	provider := &GoogleProvider{}

	got := provider.Name()
	want := "google"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

// TestGoogleProvider_TokenLimit verifies that the provider returns the correct token limit.
func TestGoogleProvider_TokenLimit(t *testing.T) {
	provider := &GoogleProvider{}

	got := provider.TokenLimit()
	want := 32000 // Gemini 1.5 token limit

	if got != want {
		t.Errorf("TokenLimit() = %d, want %d", got, want)
	}
}

// TestGoogleProvider_ReviewBatch_ValidInput tests successful code review with valid input.
func TestGoogleProvider_ReviewBatch_ValidInput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires a valid GOOGLE_API_KEY environment variable
	provider, err := NewGoogleProvider("", "gemini-1.5-flash")
	if err != nil {
		t.Skip("Skipping test: ", err)
	}

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath: "test.go",
				Content: `package main

func divide(a, b int) int {
	return a / b // Potential division by zero
}
`,
				Language:  "go",
				LineCount: 5,
				SizeBytes: 100,
				Checksum:  "abc123",
			},
		},
		FocusAreas:  []string{"security"},
		Language:    "go",
		BatchNumber: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Verify response structure
	if resp.ProviderName != "google" {
		t.Errorf("ProviderName = %q, want %q", resp.ProviderName, "google")
	}

	if resp.TokensUsed <= 0 {
		t.Errorf("TokensUsed = %d, want > 0", resp.TokensUsed)
	}

	if resp.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", resp.Duration)
	}

	// We expect at least one issue for the division by zero
	if len(resp.Issues) == 0 {
		t.Log("Warning: No issues found, expected at least one for division by zero")
	}

	// Validate each issue
	for i, issue := range resp.Issues {
		if err := issue.Validate(); err != nil {
			t.Errorf("Issue[%d] validation failed: %v", i, err)
		}
		if issue.ProviderName != "google" {
			t.Errorf("Issue[%d].ProviderName = %q, want %q", i, issue.ProviderName, "google")
		}
		if issue.File != "test.go" {
			t.Errorf("Issue[%d].File = %q, want %q", i, issue.File, "test.go")
		}
	}
}

// TestGoogleProvider_ReviewBatch_EmptyFiles tests handling of empty file list.
func TestGoogleProvider_ReviewBatch_EmptyFiles(t *testing.T) {
	provider := &GoogleProvider{}

	req := ReviewRequest{
		Files:       []CodeFile{},
		FocusAreas:  []string{"security"},
		Language:    "go",
		BatchNumber: 1,
	}

	ctx := context.Background()
	resp, err := provider.ReviewBatch(ctx, req)

	if err != nil {
		t.Errorf("ReviewBatch() with empty files should not error, got: %v", err)
	}

	if len(resp.Issues) != 0 {
		t.Errorf("Expected no issues for empty files, got %d", len(resp.Issues))
	}

	if resp.ProviderName != "google" {
		t.Errorf("ProviderName = %q, want %q", resp.ProviderName, "google")
	}
}

// TestGoogleProvider_ReviewBatch_ContextCancellation tests context cancellation handling.
func TestGoogleProvider_ReviewBatch_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider, err := NewGoogleProvider("", "gemini-1.5-flash")
	if err != nil {
		t.Skip("Skipping test: ", err)
	}

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath:  "test.go",
				Content:   "package main\n\nfunc main() {}\n",
				Language:  "go",
				LineCount: 3,
				SizeBytes: 30,
				Checksum:  "def456",
			},
		},
		FocusAreas:  []string{"security"},
		Language:    "go",
		BatchNumber: 1,
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = provider.ReviewBatch(ctx, req)

	if err == nil {
		t.Error("Expected error for cancelled context, got nil")
	}

	// Check if it's a timeout/cancellation error (retryable)
	var reviewErr *ReviewError
	if errors.As(err, &reviewErr) {
		if !reviewErr.IsRetryable() {
			t.Error("Context cancellation should produce a retryable error")
		}
	}
}

// TestGoogleProvider_ReviewBatch_InvalidAPIKey tests handling of invalid API key.
func TestGoogleProvider_ReviewBatch_InvalidAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create provider with invalid API key
	provider, err := NewGoogleProvider("invalid-api-key-12345", "gemini-1.5-flash")
	if err != nil {
		t.Fatalf("NewGoogleProvider() error = %v", err)
	}

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath:  "test.go",
				Content:   "package main\n\nfunc main() {}\n",
				Language:  "go",
				LineCount: 3,
				SizeBytes: 30,
				Checksum:  "ghi789",
			},
		},
		FocusAreas:  []string{"security"},
		Language:    "go",
		BatchNumber: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = provider.ReviewBatch(ctx, req)

	if err == nil {
		t.Fatal("Expected error for invalid API key, got nil")
	}

	// Check that it's a permanent error (not retryable)
	var reviewErr *ReviewError
	if errors.As(err, &reviewErr) {
		if reviewErr.IsRetryable() {
			t.Error("Invalid API key error should not be retryable")
		}
	}
}

// TestGoogleProvider_ReviewBatch_MultipleFiles tests reviewing multiple files in a batch.
func TestGoogleProvider_ReviewBatch_MultipleFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider, err := NewGoogleProvider("", "gemini-1.5-flash")
	if err != nil {
		t.Skip("Skipping test: ", err)
	}

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath: "unsafe.go",
				Content: `package main

func processInput(input string) {
	// SQL injection vulnerability
	query := "SELECT * FROM users WHERE name = '" + input + "'"
	executeQuery(query)
}
`,
				Language:  "go",
				LineCount: 7,
				SizeBytes: 150,
				Checksum:  "file1",
			},
			{
				FilePath: "slow.go",
				Content: `package main

func findItem(items []int, target int) bool {
	// Inefficient O(n) search in potentially sorted array
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
`,
				Language:  "go",
				LineCount: 11,
				SizeBytes: 200,
				Checksum:  "file2",
			},
		},
		FocusAreas:  []string{"security", "performance"},
		Language:    "go",
		BatchNumber: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Should find issues in both files
	filesWithIssues := make(map[string]bool)
	for _, issue := range resp.Issues {
		filesWithIssues[issue.File] = true
		if err := issue.Validate(); err != nil {
			t.Errorf("Invalid issue: %v", err)
		}
	}

	if len(resp.Issues) == 0 {
		t.Log("Warning: Expected to find security and performance issues")
	}

	t.Logf("Found %d issues across %d files", len(resp.Issues), len(filesWithIssues))
}

// TestGoogleProvider_ReviewBatch_JSONParsing tests that the response parsing handles various formats.
func TestGoogleProvider_ReviewBatch_JSONParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider, err := NewGoogleProvider("", "gemini-1.5-flash")
	if err != nil {
		t.Skip("Skipping test: ", err)
	}

	// Simple code that should produce structured output
	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath: "example.go",
				Content: `package main

import "fmt"

func greet(name string) {
	fmt.Println("Hello " + name) // String concatenation in print
}
`,
				Language:  "go",
				LineCount: 7,
				SizeBytes: 120,
				Checksum:  "parse123",
			},
		},
		FocusAreas:  []string{"style", "best-practice"},
		Language:    "go",
		BatchNumber: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Verify all issues are properly structured
	for i, issue := range resp.Issues {
		// Check all required fields are present
		if issue.File == "" {
			t.Errorf("Issue[%d].File is empty", i)
		}
		if issue.Severity == "" {
			t.Errorf("Issue[%d].Severity is empty", i)
		}
		if issue.Category == "" {
			t.Errorf("Issue[%d].Category is empty", i)
		}
		if issue.Description == "" {
			t.Errorf("Issue[%d].Description is empty", i)
		}
		if issue.Confidence < 0.0 || issue.Confidence > 1.0 {
			t.Errorf("Issue[%d].Confidence = %f, want 0.0-1.0", i, issue.Confidence)
		}

		// Validate the entire issue
		if err := issue.Validate(); err != nil {
			t.Errorf("Issue[%d] validation failed: %v\nIssue: %+v", i, err, issue)
		}
	}
}

// TestNewGoogleProvider tests the constructor with various inputs.
func TestNewGoogleProvider(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		model     string
		wantError bool
	}{
		{
			name:      "valid model",
			apiKey:    "test-key",
			model:     "gemini-1.5-flash",
			wantError: false,
		},
		{
			name:      "empty model uses default",
			apiKey:    "test-key",
			model:     "",
			wantError: false,
		},
		{
			name:      "custom model",
			apiKey:    "test-key",
			model:     "gemini-pro",
			wantError: false,
		},
		{
			name:      "empty api key tries env",
			apiKey:    "",
			model:     "gemini-1.5-flash",
			wantError: false, // Will succeed or skip based on env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGoogleProvider(tt.apiKey, tt.model)

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantError && err == nil {
				if provider == nil {
					t.Error("Expected non-nil provider")
				}
				if provider.Name() != "google" {
					t.Errorf("Name() = %q, want %q", provider.Name(), "google")
				}
			}
		})
	}
}
