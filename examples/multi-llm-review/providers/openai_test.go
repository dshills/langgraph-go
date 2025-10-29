package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestOpenAIProvider_Name verifies Name() returns "openai".
func TestOpenAIProvider_Name(t *testing.T) {
	provider := &OpenAIProvider{}

	got := provider.Name()
	want := "openai"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

// TestOpenAIProvider_TokenLimit verifies TokenLimit() returns GPT-4 limit.
func TestOpenAIProvider_TokenLimit(t *testing.T) {
	provider := &OpenAIProvider{}

	got := provider.TokenLimit()
	want := 128000 // GPT-4 token limit

	if got != want {
		t.Errorf("TokenLimit() = %d, want %d", got, want)
	}
}

// TestOpenAIProvider_ReviewBatch_ValidInput verifies ReviewBatch handles valid input.
// This test will use actual API calls when API key is available in environment.
func TestOpenAIProvider_ReviewBatch_ValidInput(t *testing.T) {
	// Skip if no API key available
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath:  "test.go",
				Content:   "package main\n\nfunc unsafeSQL(input string) {\n\tdb.Exec(\"SELECT * FROM users WHERE id = \" + input)\n}",
				Language:  "go",
				LineCount: 4,
				SizeBytes: 85,
			},
		},
		FocusAreas:  []string{"security"},
		Language:    "go",
		BatchNumber: 1,
	}

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Verify response structure
	if resp.ProviderName != "openai" {
		t.Errorf("ProviderName = %q, want %q", resp.ProviderName, "openai")
	}

	if resp.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", resp.Duration)
	}

	if resp.TokensUsed <= 0 {
		t.Errorf("TokensUsed = %d, want > 0", resp.TokensUsed)
	}

	// Should find at least one security issue in unsafe SQL code
	if len(resp.Issues) == 0 {
		t.Errorf("Expected at least one issue for unsafe SQL code, got 0")
	}

	// Verify issues are well-formed
	for i, issue := range resp.Issues {
		if err := issue.Validate(); err != nil {
			t.Errorf("Issue[%d] validation failed: %v", i, err)
		}
		if issue.ProviderName != "openai" {
			t.Errorf("Issue[%d].ProviderName = %q, want %q", i, issue.ProviderName, "openai")
		}
	}
}

// TestOpenAIProvider_ReviewBatch_MultipleFiles verifies batch review with multiple files.
func TestOpenAIProvider_ReviewBatch_MultipleFiles(t *testing.T) {
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	req := ReviewRequest{
		Files: []CodeFile{
			{
				FilePath:  "security.go",
				Content:   "package main\n\nfunc eval(cmd string) { exec.Command(\"sh\", \"-c\", cmd).Run() }",
				Language:  "go",
				LineCount: 3,
			},
			{
				FilePath:  "perf.go",
				Content:   "package main\n\nfunc slowLoop() {\n\tfor i := 0; i < 1000000; i++ {\n\t\tdata := make([]byte, 1024)\n\t}\n}",
				Language:  "go",
				LineCount: 6,
			},
		},
		FocusAreas:  []string{"security", "performance"},
		Language:    "go",
		BatchNumber: 1,
	}

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Should find issues in multiple files
	if len(resp.Issues) == 0 {
		t.Error("Expected issues in multiple files")
	}

	// Verify issues reference correct files
	filesFound := make(map[string]bool)
	for _, issue := range resp.Issues {
		filesFound[issue.File] = true
	}

	if len(filesFound) == 0 {
		t.Error("Expected issues to reference file paths")
	}
}

// TestOpenAIProvider_ReviewBatch_ContextCancellation verifies cancellation handling.
func TestOpenAIProvider_ReviewBatch_ContextCancellation(t *testing.T) {
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := ReviewRequest{
		Files: []CodeFile{
			{FilePath: "test.go", Content: "package main", Language: "go"},
		},
		Language:    "go",
		BatchNumber: 1,
	}

	_, err = provider.ReviewBatch(ctx, req)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

// TestOpenAIProvider_ReviewBatch_InvalidAPIKey verifies handling of invalid API key.
func TestOpenAIProvider_ReviewBatch_InvalidAPIKey(t *testing.T) {
	provider, err := NewOpenAIProvider("invalid-key-12345", "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := ReviewRequest{
		Files: []CodeFile{
			{FilePath: "test.go", Content: "package main", Language: "go"},
		},
		Language:    "go",
		BatchNumber: 1,
	}

	_, err = provider.ReviewBatch(ctx, req)
	if err == nil {
		t.Error("Expected error with invalid API key")
		return
	}

	// Should be a non-retryable error
	var reviewErr *ReviewError
	if errors.As(err, &reviewErr) {
		if reviewErr.IsRetryable() {
			t.Error("Invalid API key error should not be retryable")
		}
	}
}

// TestOpenAIProvider_ReviewBatch_EmptyFiles verifies handling of empty file list.
func TestOpenAIProvider_ReviewBatch_EmptyFiles(t *testing.T) {
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	ctx := context.Background()
	req := ReviewRequest{
		Files:       []CodeFile{},
		Language:    "go",
		BatchNumber: 1,
	}

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() with empty files error = %v", err)
	}

	// Should return empty issues
	if len(resp.Issues) != 0 {
		t.Errorf("Expected 0 issues for empty files, got %d", len(resp.Issues))
	}
}

// TestOpenAIProvider_ReviewBatch_JSONParsing verifies correct JSON parsing of issues.
func TestOpenAIProvider_ReviewBatch_JSONParsing(t *testing.T) {
	// Test that we can parse the expected JSON structure
	jsonResponse := `{
		"issues": [
			{
				"file": "test.go",
				"line": 10,
				"severity": "high",
				"category": "security",
				"description": "SQL injection vulnerability",
				"remediation": "Use parameterized queries",
				"confidence": 0.95
			}
		]
	}`

	var result struct {
		Issues []ReviewIssue `json:"issues"`
	}

	err := json.Unmarshal([]byte(jsonResponse), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.File != "test.go" {
		t.Errorf("File = %q, want %q", issue.File, "test.go")
	}
	if issue.Line != 10 {
		t.Errorf("Line = %d, want 10", issue.Line)
	}
	if issue.Severity != "high" {
		t.Errorf("Severity = %q, want %q", issue.Severity, "high")
	}
	if issue.Category != "security" {
		t.Errorf("Category = %q, want %q", issue.Category, "security")
	}
	if issue.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", issue.Confidence)
	}
}

// TestOpenAIProvider_ConcurrentCalls verifies safe concurrent ReviewBatch calls.
func TestOpenAIProvider_ConcurrentCalls(t *testing.T) {
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	const numCalls = 3
	var wg sync.WaitGroup
	wg.Add(numCalls)

	errChan := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := ReviewRequest{
				Files: []CodeFile{
					{
						FilePath:  "concurrent.go",
						Content:   "package main\n\nfunc test() { println(\"test\") }",
						Language:  "go",
						LineCount: 3,
					},
				},
				Language:    "go",
				BatchNumber: id,
			}

			_, err := provider.ReviewBatch(ctx, req)
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent call failed: %v", err)
	}
}

// TestOpenAIProvider_InterfaceCompliance verifies OpenAIProvider implements CodeReviewer.
func TestOpenAIProvider_InterfaceCompliance(t *testing.T) {
	var _ CodeReviewer = (*OpenAIProvider)(nil)
}

// TestNewOpenAIProvider_InvalidInput verifies constructor validation.
func TestNewOpenAIProvider_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		wantErr bool
	}{
		{
			name:    "empty API key",
			apiKey:  "",
			model:   "gpt-4",
			wantErr: true,
		},
		{
			name:    "empty model",
			apiKey:  "sk-test123",
			model:   "",
			wantErr: true,
		},
		{
			name:    "valid inputs",
			apiKey:  "sk-test123",
			model:   "gpt-4",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOpenAIProvider(tt.apiKey, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestOpenAIProvider_ResponseMetadata verifies response contains proper metadata.
func TestOpenAIProvider_ResponseMetadata(t *testing.T) {
	apiKey := getTestAPIKey()
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-4o")
	if err != nil {
		t.Fatalf("NewOpenAIProvider() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := ReviewRequest{
		Files: []CodeFile{
			{FilePath: "test.go", Content: "package main", Language: "go", LineCount: 1},
		},
		Language:    "go",
		BatchNumber: 5,
	}

	resp, err := provider.ReviewBatch(ctx, req)
	if err != nil {
		t.Fatalf("ReviewBatch() error = %v", err)
	}

	// Verify all metadata fields are populated
	if resp.ProviderName != "openai" {
		t.Errorf("ProviderName = %q, want %q", resp.ProviderName, "openai")
	}

	if resp.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", resp.Duration)
	}

	if resp.TokensUsed < 0 {
		t.Errorf("TokensUsed = %d, want >= 0", resp.TokensUsed)
	}
}

// getTestAPIKey retrieves OpenAI API key from environment for testing.
func getTestAPIKey() string {
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
}
