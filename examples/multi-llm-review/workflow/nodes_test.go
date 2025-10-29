package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestReportNode_GeneratesMarkdownFile tests that ReportNode creates a markdown report file.
func TestReportNode_GeneratesMarkdownFile(t *testing.T) {
	// Create temporary directory for test output
	tempDir := t.TempDir()

	// Change to temp directory for test
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	// Create test state
	state := ReviewState{
		CodebaseRoot:       "/test/codebase",
		TotalFilesReviewed: 10,
		ConsolidatedIssues: []ConsolidatedIssue{
			{
				File:        "main.go",
				Line:        42,
				Severity:    "high",
				Category:    "security",
				Description: "SQL injection vulnerability",
				Remediation: "Use parameterized queries",
				Providers:   []string{"openai", "anthropic"},
			},
		},
		StartTime: "2025-10-29T10:00:00Z",
	}

	// Execute node
	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	// Verify no error
	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v, want nil", result.Err)
	}

	// Verify report path was set
	if result.Delta.ReportPath == "" {
		t.Error("ReportNode.Run() did not set ReportPath in delta")
	}

	// Verify file exists
	if _, err := os.Stat(result.Delta.ReportPath); os.IsNotExist(err) {
		t.Errorf("Report file does not exist at path: %s", result.Delta.ReportPath)
	}

	// Verify file is in expected directory
	if !strings.HasPrefix(result.Delta.ReportPath, "review-results/") {
		t.Errorf("Report path = %s, want prefix 'review-results/'", result.Delta.ReportPath)
	}

	// Verify file has .md extension
	if !strings.HasSuffix(result.Delta.ReportPath, ".md") {
		t.Errorf("Report path = %s, want suffix '.md'", result.Delta.ReportPath)
	}
}

// TestReportNode_IncludesSummaryStatistics tests that the report includes summary statistics.
func TestReportNode_IncludesSummaryStatistics(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 150,
		ConsolidatedIssues: []ConsolidatedIssue{
			{File: "file1.go", Severity: "high", Description: "Issue 1", Providers: []string{"openai"}},
			{File: "file2.go", Severity: "medium", Description: "Issue 2", Providers: []string{"anthropic"}},
			{File: "file3.go", Severity: "low", Description: "Issue 3", Providers: []string{"google"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	// Read generated report
	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Verify summary section exists
	if !strings.Contains(reportText, "## Summary Statistics") {
		t.Error("Report does not contain '## Summary Statistics' section")
	}

	// Verify total files count (appears in header and table)
	if !strings.Contains(reportText, "150") {
		t.Error("Report does not contain correct files reviewed count")
	}

	// Verify total issues count (appears in header)
	if !strings.Contains(reportText, "**Issues Found**: 3") {
		t.Error("Report does not contain correct total issues count")
	}
}

// TestReportNode_GroupsIssuesBySeverity tests that issues are grouped by severity.
func TestReportNode_GroupsIssuesBySeverity(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 10,
		ConsolidatedIssues: []ConsolidatedIssue{
			{File: "f1.go", Line: 10, Severity: "critical", Category: "security", Description: "Critical issue", Providers: []string{"openai"}},
			{File: "f2.go", Line: 20, Severity: "critical", Category: "security", Description: "Another critical", Providers: []string{"anthropic"}},
			{File: "f3.go", Line: 30, Severity: "high", Category: "performance", Description: "High severity", Providers: []string{"google"}},
			{File: "f4.go", Line: 40, Severity: "medium", Category: "style", Description: "Medium severity", Providers: []string{"openai"}},
			{File: "f5.go", Line: 50, Severity: "low", Category: "best-practice", Description: "Low severity", Providers: []string{"anthropic"}},
			{File: "f6.go", Line: 60, Severity: "info", Category: "style", Description: "Info severity", Providers: []string{"google"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Verify severity sections exist
	expectedSections := []string{
		"## Critical Issues (2)",
		"## High Priority Issues (1)",
		"## Medium Priority Issues (1)",
		"## Low Priority Issues (1)",
		"## Informational Issues (1)",
	}

	for _, section := range expectedSections {
		if !strings.Contains(reportText, section) {
			t.Errorf("Report does not contain expected section: %s", section)
		}
	}

	// Verify severity sections appear in correct order
	criticalPos := strings.Index(reportText, "## Critical Issues")
	highPos := strings.Index(reportText, "## High Priority Issues")
	mediumPos := strings.Index(reportText, "## Medium Priority Issues")
	lowPos := strings.Index(reportText, "## Low Priority Issues")
	infoPos := strings.Index(reportText, "## Informational Issues")

	if criticalPos > highPos || highPos > mediumPos || mediumPos > lowPos || lowPos > infoPos {
		t.Error("Severity sections are not in the correct order (critical, high, medium, low, info)")
	}
}

// TestReportNode_IncludesProviderAttribution tests that each issue includes provider attribution.
func TestReportNode_IncludesProviderAttribution(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 5,
		ConsolidatedIssues: []ConsolidatedIssue{
			{
				File:           "main.go",
				Line:           42,
				Severity:       "high",
				Category:       "security",
				Description:    "SQL injection vulnerability",
				Remediation:    "Use parameterized queries",
				Providers:      []string{"anthropic", "google", "openai"}, // alphabetically sorted
				ConsensusScore: 1.0,
			},
			{
				File:           "handler.go",
				Line:           15,
				Severity:       "medium",
				Category:       "performance",
				Description:    "Inefficient loop",
				Remediation:    "Use map lookup",
				Providers:      []string{"openai"},
				ConsensusScore: 0.33,
			},
		},
		// Need Reviews map for consensus calculation
		Reviews: map[string][]Review{
			"anthropic": {{ProviderName: "anthropic"}},
			"openai":    {{ProviderName: "openai"}},
			"google":    {{ProviderName: "google"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Verify first issue provider attribution
	if !strings.Contains(reportText, "**Providers**: anthropic, google, openai") {
		t.Error("Report does not contain correct provider attribution for first issue")
	}

	// Verify second issue provider attribution
	if !strings.Contains(reportText, "**Providers**: openai") {
		t.Error("Report does not contain correct provider attribution for second issue")
	}

	// Verify consensus scores with new format (X/Y providers (Z%))
	if !strings.Contains(reportText, "**Consensus**: 3/3 providers (100%)") {
		t.Error("Report does not contain correct consensus score for first issue")
	}

	if !strings.Contains(reportText, "**Consensus**: 1/3 providers (33%)") {
		t.Error("Report does not contain correct consensus score for second issue")
	}
}

// TestReportNode_SetsReportPathInState tests that the node sets ReportPath in the state delta.
func TestReportNode_SetsReportPathInState(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 1,
		ConsolidatedIssues: []ConsolidatedIssue{
			{File: "test.go", Severity: "low", Description: "Test issue", Providers: []string{"openai"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	// Verify ReportPath is set
	if result.Delta.ReportPath == "" {
		t.Error("ReportNode did not set ReportPath in delta")
	}

	// Verify EndTime is set
	if result.Delta.EndTime == "" {
		t.Error("ReportNode did not set EndTime in delta")
	}

	// Verify EndTime is valid RFC3339
	_, err := time.Parse(time.RFC3339, result.Delta.EndTime)
	if err != nil {
		t.Errorf("EndTime is not valid RFC3339 format: %v", err)
	}
}

// TestReportNode_RoutesToEnd tests that the node routes to "end" (Stop).
func TestReportNode_RoutesToEnd(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 1,
		ConsolidatedIssues: []ConsolidatedIssue{},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	// Verify route is terminal (Stop)
	if !result.Route.Terminal {
		t.Error("ReportNode route is not terminal, expected Stop()")
	}

	// Verify no other routing
	if result.Route.To != "" {
		t.Errorf("ReportNode route.To = %s, want empty (terminal route)", result.Route.To)
	}

	if len(result.Route.Many) > 0 {
		t.Errorf("ReportNode route.Many = %v, want empty (terminal route)", result.Route.Many)
	}
}

// TestReportNode_EmptyState tests report generation with minimal/empty state.
func TestReportNode_EmptyState(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	// Should succeed even with empty state
	if result.Err != nil {
		t.Fatalf("ReportNode.Run() with empty state error = %v, want nil", result.Err)
	}

	// Verify report was still created
	if result.Delta.ReportPath == "" {
		t.Error("ReportNode did not set ReportPath for empty state")
	}

	// Verify file exists
	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Should have basic structure
	if !strings.Contains(reportText, "# Code Review Report") {
		t.Error("Report does not contain header")
	}

	if !strings.Contains(reportText, "## Summary Statistics") {
		t.Error("Report does not contain summary section")
	}

	// Should show 0 for empty state
	if !strings.Contains(reportText, "**Files Reviewed**: 0") {
		t.Error("Report does not show 0 files reviewed")
	}

	if !strings.Contains(reportText, "**Issues Found**: 0") {
		t.Error("Report does not show 0 total issues")
	}
}

// TestReportNode_OutputDirectory tests that reports are written to the correct directory.
func TestReportNode_OutputDirectory(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 5,
		ConsolidatedIssues: []ConsolidatedIssue{
			{File: "test.go", Severity: "low", Description: "Test", Providers: []string{"openai"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	// Verify output directory was created
	outputDir := "./review-results/"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("Output directory was not created: %s", outputDir)
	}

	// Verify report is in the output directory
	dir := filepath.Dir(result.Delta.ReportPath)
	if dir != "review-results" {
		t.Errorf("Report directory = %s, want 'review-results'", dir)
	}
}

// TestReportNode_IncludesAllIssueDetails tests that all issue fields are included in the report.
func TestReportNode_IncludesAllIssueDetails(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 1,
		ConsolidatedIssues: []ConsolidatedIssue{
			{
				File:           "database.go",
				Line:           123,
				Severity:       "critical",
				Category:       "security",
				Description:    "Potential SQL injection",
				Remediation:    "Use prepared statements with parameterized queries",
				Providers:      []string{"anthropic", "openai"}, // alphabetically sorted
				ConsensusScore: 0.67,
				IssueID:        "abc123",
			},
		},
		// Need to populate Reviews map for consensus calculation
		Reviews: map[string][]Review{
			"anthropic": {{ProviderName: "anthropic"}},
			"openai":    {{ProviderName: "openai"}},
			"google":    {{ProviderName: "google"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Verify all issue details are present
	expectedDetails := []string{
		"Potential SQL injection",
		"`database.go:123`",
		"**Category**: Security",
		"Use prepared statements with parameterized queries",
		"**Providers**: anthropic, openai",
		"**Consensus**: 2/3 providers (67%)",
	}

	for _, detail := range expectedDetails {
		if !strings.Contains(reportText, detail) {
			t.Errorf("Report does not contain expected detail: %s", detail)
		}
	}
}

// TestReportNode_IncludesReviewsInSummary tests that provider information from reviews is included.
func TestReportNode_IncludesReviewsInSummary(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	state := ReviewState{
		TotalFilesReviewed: 10,
		Reviews: map[string][]Review{
			"openai": {
				{ProviderName: "openai", BatchNumber: 1},
			},
			"anthropic": {
				{ProviderName: "anthropic", BatchNumber: 1},
			},
			"google": {
				{ProviderName: "google", BatchNumber: 1},
			},
		},
		ConsolidatedIssues: []ConsolidatedIssue{
			{File: "test.go", Severity: "low", Description: "Test", Providers: []string{"openai"}},
		},
	}

	node := &ReportNode{}
	result := node.Run(context.Background(), state)

	if result.Err != nil {
		t.Fatalf("ReportNode.Run() error = %v", result.Err)
	}

	content, err := os.ReadFile(result.Delta.ReportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	reportText := string(content)

	// Verify providers are listed (sorted alphabetically)
	if !strings.Contains(reportText, "**Providers**: anthropic, google, openai") {
		t.Error("Report does not contain correct provider list in summary")
	}
}

// ============================================================================
// ReviewBatchNode Concurrent Provider Tests
// ============================================================================

// TestReviewBatchNode_MultipleProvidersConcurrent tests that ReviewBatchNode
// calls all providers concurrently when multiple providers are configured.
func TestReviewBatchNode_MultipleProvidersConcurrent(t *testing.T) {
	ctx := context.Background()

	// Create mock providers with artificial delays to verify concurrency
	provider1 := &mockCodeReviewerWithDelay{
		name:     "provider1",
		delay:    100 * time.Millisecond,
		issueMsg: "Issue from provider1",
	}
	provider2 := &mockCodeReviewerWithDelay{
		name:     "provider2",
		delay:    100 * time.Millisecond,
		issueMsg: "Issue from provider2",
	}
	provider3 := &mockCodeReviewerWithDelay{
		name:     "provider3",
		delay:    100 * time.Millisecond,
		issueMsg: "Issue from provider3",
	}

	// Create node with all three providers
	providers := []CodeReviewer{provider1, provider2, provider3}
	node := NewReviewBatchNode(providers)

	// Create state with one batch containing one file
	state := ReviewState{
		Batches: []Batch{
			{
				BatchNumber: 1,
				Files: []CodeFile{
					{
						FilePath:  "test.go",
						Content:   "package main",
						Language:  "go",
						LineCount: 1,
						SizeBytes: 12,
						Checksum:  "abc123",
					},
				},
			},
		},
		CurrentBatch: 1,
		TotalBatches: 1,
		Reviews:      make(map[string][]Review),
	}

	// Execute with timing
	start := time.Now()
	result := node.Run(ctx, state)
	elapsed := time.Since(start)

	// Verify no error
	if result.Err != nil {
		t.Fatalf("ReviewBatchNode.Run() returned error: %v", result.Err)
	}

	// Verify concurrent execution: should take ~100ms, not 300ms
	// Allow 200ms tolerance for overhead
	if elapsed > 300*time.Millisecond {
		t.Errorf("Expected concurrent execution ~100ms, took %v (likely sequential)", elapsed)
	}

	// Verify all three providers were called
	if len(result.Delta.Reviews) != 3 {
		t.Fatalf("Expected reviews from 3 providers, got %d", len(result.Delta.Reviews))
	}

	// Verify each provider's results
	for _, providerName := range []string{"provider1", "provider2", "provider3"} {
		reviews, exists := result.Delta.Reviews[providerName]
		if !exists {
			t.Errorf("Expected review from %s", providerName)
			continue
		}
		if len(reviews) != 1 {
			t.Errorf("Expected 1 review from %s, got %d", providerName, len(reviews))
			continue
		}
		if len(reviews[0].Issues) == 0 {
			t.Errorf("Expected issues from %s, got none", providerName)
		}
	}
}

// TestReviewBatchNode_PartialProviderFailure tests that when one provider fails,
// the workflow continues with results from successful providers.
func TestReviewBatchNode_PartialProviderFailure(t *testing.T) {
	ctx := context.Background()

	// Create providers where one will fail
	provider1 := &mockCodeReviewerWithFailure{
		name:       "provider1-success",
		shouldFail: false,
	}
	provider2 := &mockCodeReviewerWithFailure{
		name:       "provider2-fail",
		shouldFail: true,
		errorMsg:   "provider2 API error",
	}
	provider3 := &mockCodeReviewerWithFailure{
		name:       "provider3-success",
		shouldFail: false,
	}

	providers := []CodeReviewer{provider1, provider2, provider3}
	node := NewReviewBatchNode(providers)

	state := ReviewState{
		Batches: []Batch{
			{
				BatchNumber: 1,
				Files: []CodeFile{
					{
						FilePath:  "test.go",
						Content:   "package main",
						Language:  "go",
						LineCount: 1,
						SizeBytes: 12,
						Checksum:  "abc123",
					},
				},
			},
		},
		CurrentBatch: 1,
		TotalBatches: 1,
		Reviews:      make(map[string][]Review),
	}

	result := node.Run(ctx, state)

	// Verify no error (partial failure should not block workflow)
	if result.Err != nil {
		t.Fatalf("ReviewBatchNode.Run() returned error: %v (expected graceful handling)", result.Err)
	}

	// Verify successful providers returned results
	successProviders := []string{"provider1-success", "provider3-success"}
	for _, providerName := range successProviders {
		reviews, exists := result.Delta.Reviews[providerName]
		if !exists {
			t.Errorf("Expected review from successful provider %s", providerName)
			continue
		}
		if len(reviews) != 1 {
			t.Errorf("Expected 1 review from %s, got %d", providerName, len(reviews))
		}
	}

	// Verify failed provider is tracked
	if len(result.Delta.FailedProviders) != 1 {
		t.Fatalf("Expected 1 failed provider, got %d", len(result.Delta.FailedProviders))
	}
	if result.Delta.FailedProviders[0] != "provider2-fail" {
		t.Errorf("Expected failed provider 'provider2-fail', got '%s'", result.Delta.FailedProviders[0])
	}

	// Verify failed provider has error in review
	failedReviews, exists := result.Delta.Reviews["provider2-fail"]
	if !exists {
		t.Error("Expected review entry for failed provider")
	} else if len(failedReviews) != 1 {
		t.Errorf("Expected 1 review entry for failed provider, got %d", len(failedReviews))
	} else if failedReviews[0].Error == "" {
		t.Error("Expected error message in failed provider review")
	}

	// Verify workflow continues (not terminal route)
	if result.Route.To == "" && result.Route.Terminal {
		t.Error("Expected workflow to continue after partial failure")
	}
}

// TestReviewBatchNode_AllProvidersAggregate tests that issues from all providers
// are aggregated correctly in the state.
func TestReviewBatchNode_AllProvidersAggregate(t *testing.T) {
	ctx := context.Background()

	// Create three providers that each return different numbers of issues
	provider1 := &mockCodeReviewerWithIssueCount{name: "provider1", issueCount: 2}
	provider2 := &mockCodeReviewerWithIssueCount{name: "provider2", issueCount: 3}
	provider3 := &mockCodeReviewerWithIssueCount{name: "provider3", issueCount: 1}

	providers := []CodeReviewer{provider1, provider2, provider3}
	node := NewReviewBatchNode(providers)

	state := ReviewState{
		Batches: []Batch{
			{
				BatchNumber: 1,
				Files: []CodeFile{
					{
						FilePath:  "test.go",
						Content:   "package main",
						Language:  "go",
						LineCount: 1,
						SizeBytes: 12,
						Checksum:  "abc123",
					},
				},
			},
		},
		CurrentBatch: 1,
		TotalBatches: 1,
		Reviews:      make(map[string][]Review),
	}

	result := node.Run(ctx, state)

	if result.Err != nil {
		t.Fatalf("ReviewBatchNode.Run() returned error: %v", result.Err)
	}

	// Verify total issues found is the sum of all provider issues
	expectedTotal := 2 + 3 + 1
	if result.Delta.TotalIssuesFound != expectedTotal {
		t.Errorf("Expected TotalIssuesFound=%d, got %d", expectedTotal, result.Delta.TotalIssuesFound)
	}

	// Verify each provider's issue count
	testCases := []struct {
		provider string
		expected int
	}{
		{"provider1", 2},
		{"provider2", 3},
		{"provider3", 1},
	}

	for _, tc := range testCases {
		reviews, exists := result.Delta.Reviews[tc.provider]
		if !exists {
			t.Errorf("Expected review from %s", tc.provider)
			continue
		}
		if len(reviews) != 1 {
			t.Errorf("Expected 1 review from %s, got %d", tc.provider, len(reviews))
			continue
		}
		if len(reviews[0].Issues) != tc.expected {
			t.Errorf("Expected %d issues from %s, got %d", tc.expected, tc.provider, len(reviews[0].Issues))
		}
	}
}

// Mock implementations for concurrent testing

type mockCodeReviewerWithDelay struct {
	name     string
	delay    time.Duration
	issueMsg string
}

func (m *mockCodeReviewerWithDelay) Name() string {
	return m.name
}

func (m *mockCodeReviewerWithDelay) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	// Simulate API call delay
	time.Sleep(m.delay)

	return ReviewResponse{
		Issues: []ReviewIssueFromProvider{
			{
				File:         req.Files[0].FilePath,
				Line:         1,
				Severity:     "medium",
				Category:     "best-practice",
				Description:  m.issueMsg,
				Remediation:  "Fix it",
				ProviderName: m.name,
				Confidence:   0.8,
			},
		},
		TokensUsed:   100,
		Duration:     m.delay,
		ProviderName: m.name,
	}, nil
}

type mockCodeReviewerWithFailure struct {
	name       string
	shouldFail bool
	errorMsg   string
}

func (m *mockCodeReviewerWithFailure) Name() string {
	return m.name
}

func (m *mockCodeReviewerWithFailure) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	if m.shouldFail {
		return ReviewResponse{}, fmt.Errorf("%s", m.errorMsg)
	}

	return ReviewResponse{
		Issues: []ReviewIssueFromProvider{
			{
				File:         req.Files[0].FilePath,
				Line:         1,
				Severity:     "medium",
				Category:     "best-practice",
				Description:  "Issue from " + m.name,
				Remediation:  "Fix it",
				ProviderName: m.name,
				Confidence:   0.8,
			},
		},
		TokensUsed:   100,
		Duration:     0,
		ProviderName: m.name,
	}, nil
}

type mockCodeReviewerWithIssueCount struct {
	name       string
	issueCount int
}

func (m *mockCodeReviewerWithIssueCount) Name() string {
	return m.name
}

func (m *mockCodeReviewerWithIssueCount) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	issues := make([]ReviewIssueFromProvider, m.issueCount)
	for i := 0; i < m.issueCount; i++ {
		issues[i] = ReviewIssueFromProvider{
			File:         req.Files[0].FilePath,
			Line:         i + 1,
			Severity:     "medium",
			Category:     "best-practice",
			Description:  fmt.Sprintf("Issue %d from %s", i+1, m.name),
			Remediation:  "Fix it",
			ProviderName: m.name,
			Confidence:   0.8,
		}
	}

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   100,
		Duration:     0,
		ProviderName: m.name,
	}, nil
}

// ============================================================================
// ConsolidateNode Tests
// ============================================================================

// TestConsolidateNode_MergesDuplicateIssues verifies that ConsolidateNode merges
// issues with identical file, line, and description into a single ConsolidatedIssue.
func TestConsolidateNode_MergesDuplicateIssues(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with duplicate issues from multiple providers
	state := ReviewState{
		Reviews: map[string][]Review{
			"openai": {
				{
					ProviderName: "openai",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "main.go",
							Line:         42,
							Severity:     "high",
							Category:     "security",
							Description:  "SQL injection vulnerability",
							Remediation:  "Use parameterized queries",
							ProviderName: "openai",
							Confidence:   0.95,
						},
					},
				},
			},
			"anthropic": {
				{
					ProviderName: "anthropic",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "main.go",
							Line:         42,
							Severity:     "critical",
							Category:     "security",
							Description:  "SQL injection vulnerability",
							Remediation:  "Use prepared statements",
							ProviderName: "anthropic",
							Confidence:   0.98,
						},
					},
				},
			},
			"google": {
				{
					ProviderName: "google",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "main.go",
							Line:         42,
							Severity:     "high",
							Category:     "security",
							Description:  "SQL injection vulnerability",
							Remediation:  "Sanitize user input",
							ProviderName: "google",
							Confidence:   0.92,
						},
					},
				},
			},
		},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: Exactly one consolidated issue (3 duplicates merged)
	if len(result.Delta.ConsolidatedIssues) != 1 {
		t.Fatalf("Expected 1 consolidated issue, got %d", len(result.Delta.ConsolidatedIssues))
	}

	issue := result.Delta.ConsolidatedIssues[0]

	// Verify: Issue fields match the original
	if issue.File != "main.go" {
		t.Errorf("Expected File='main.go', got '%s'", issue.File)
	}
	if issue.Line != 42 {
		t.Errorf("Expected Line=42, got %d", issue.Line)
	}
	if issue.Description != "SQL injection vulnerability" {
		t.Errorf("Expected Description='SQL injection vulnerability', got '%s'", issue.Description)
	}

	// Verify: All three providers are listed
	if len(issue.Providers) != 3 {
		t.Fatalf("Expected 3 providers, got %d", len(issue.Providers))
	}
	providerMap := make(map[string]bool)
	for _, p := range issue.Providers {
		providerMap[p] = true
	}
	expectedProviders := []string{"openai", "anthropic", "google"}
	for _, expected := range expectedProviders {
		if !providerMap[expected] {
			t.Errorf("Expected provider '%s' not found in providers list: %v", expected, issue.Providers)
		}
	}

	// Verify: Consensus score is 1.0 (all 3 providers flagged it)
	expectedScore := 1.0
	if issue.ConsensusScore != expectedScore {
		t.Errorf("Expected ConsensusScore=%.2f, got %.2f", expectedScore, issue.ConsensusScore)
	}

	// Verify: Severity is the highest (critical)
	if issue.Severity != "critical" {
		t.Errorf("Expected Severity='critical' (highest among duplicates), got '%s'", issue.Severity)
	}

	// Verify: IssueID is populated (8-char hex)
	if len(issue.IssueID) != 8 {
		t.Errorf("Expected IssueID length=8, got %d (value: '%s')", len(issue.IssueID), issue.IssueID)
	}
}

// TestConsolidateNode_KeepsUniqueIssuesSeparate verifies that ConsolidateNode
// does not merge issues with different file, line, or description.
func TestConsolidateNode_KeepsUniqueIssuesSeparate(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with unique issues (different file, line, or description)
	state := ReviewState{
		Reviews: map[string][]Review{
			"openai": {
				{
					ProviderName: "openai",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "main.go",
							Line:         42,
							Severity:     "high",
							Category:     "security",
							Description:  "SQL injection vulnerability",
							Remediation:  "Use parameterized queries",
							ProviderName: "openai",
						},
						{
							File:         "handler.go",
							Line:         10,
							Severity:     "medium",
							Category:     "performance",
							Description:  "Inefficient loop",
							Remediation:  "Use map lookup instead",
							ProviderName: "openai",
						},
					},
				},
			},
			"anthropic": {
				{
					ProviderName: "anthropic",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "main.go",
							Line:         100,
							Severity:     "low",
							Category:     "style",
							Description:  "Variable name not idiomatic",
							Remediation:  "Use camelCase",
							ProviderName: "anthropic",
						},
					},
				},
			},
		},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: Exactly three consolidated issues (all unique)
	if len(result.Delta.ConsolidatedIssues) != 3 {
		t.Fatalf("Expected 3 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
	}

	// Verify: Each issue has exactly one provider
	for i, issue := range result.Delta.ConsolidatedIssues {
		if len(issue.Providers) != 1 {
			t.Errorf("Issue %d: Expected 1 provider, got %d (providers: %v)", i, len(issue.Providers), issue.Providers)
		}
	}

	// Verify: Each issue has IssueID populated
	for i, issue := range result.Delta.ConsolidatedIssues {
		if len(issue.IssueID) != 8 {
			t.Errorf("Issue %d: Expected IssueID length=8, got %d (value: '%s')", i, len(issue.IssueID), issue.IssueID)
		}
	}
}

// TestConsolidateNode_CalculatesConsensusScore verifies that ConsolidateNode
// correctly calculates the consensus score as the fraction of providers that flagged the issue.
func TestConsolidateNode_CalculatesConsensusScore(t *testing.T) {
	tests := []struct {
		name          string
		state         ReviewState
		expectedScore float64
		expectedFile  string
		expectedLine  int
	}{
		{
			name: "2 out of 3 providers flag issue",
			state: ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "config.go",
									Line:         15,
									Severity:     "medium",
									Category:     "best-practice",
									Description:  "Magic number should be constant",
									Remediation:  "Extract to named constant",
									ProviderName: "openai",
								},
							},
						},
					},
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "config.go",
									Line:         15,
									Severity:     "low",
									Category:     "best-practice",
									Description:  "Magic number should be constant",
									Remediation:  "Define as constant",
									ProviderName: "anthropic",
								},
							},
						},
					},
					"google": {
						{
							ProviderName: "google",
							BatchNumber:  1,
							Issues:       []ReviewIssue{}, // Google did not flag this issue
						},
					},
				},
			},
			expectedScore: 0.67, // 2/3 = 0.666... rounded to 0.67
			expectedFile:  "config.go",
			expectedLine:  15,
		},
		{
			name: "1 out of 2 providers flag issue",
			state: ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "util.go",
									Line:         50,
									Severity:     "info",
									Category:     "style",
									Description:  "Comment formatting",
									Remediation:  "Add period at end",
									ProviderName: "openai",
								},
							},
						},
					},
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Issues:       []ReviewIssue{}, // Anthropic did not flag this
						},
					},
				},
			},
			expectedScore: 0.5, // 1/2 = 0.5
			expectedFile:  "util.go",
			expectedLine:  50,
		},
		{
			name: "3 out of 3 providers flag issue",
			state: ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "auth.go",
									Line:         99,
									Severity:     "critical",
									Category:     "security",
									Description:  "Hardcoded credentials",
									Remediation:  "Use environment variables",
									ProviderName: "openai",
								},
							},
						},
					},
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "auth.go",
									Line:         99,
									Severity:     "critical",
									Category:     "security",
									Description:  "Hardcoded credentials",
									Remediation:  "Move to config file",
									ProviderName: "anthropic",
								},
							},
						},
					},
					"google": {
						{
							ProviderName: "google",
							BatchNumber:  1,
							Issues: []ReviewIssue{
								{
									File:         "auth.go",
									Line:         99,
									Severity:     "critical",
									Category:     "security",
									Description:  "Hardcoded credentials",
									Remediation:  "Use secret manager",
									ProviderName: "google",
								},
							},
						},
					},
				},
			},
			expectedScore: 1.0, // 3/3 = 1.0
			expectedFile:  "auth.go",
			expectedLine:  99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Execute: Run ConsolidateNode
			node := ConsolidateNode{}
			result := node.Run(ctx, tt.state)

			// Verify: No error
			if result.Err != nil {
				t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
			}

			// Find the issue we're testing
			var found bool
			var issue ConsolidatedIssue
			for _, iss := range result.Delta.ConsolidatedIssues {
				if iss.File == tt.expectedFile && iss.Line == tt.expectedLine {
					found = true
					issue = iss
					break
				}
			}

			if !found {
				t.Fatalf("Expected issue at %s:%d not found in consolidated issues", tt.expectedFile, tt.expectedLine)
			}

			// Verify: Consensus score matches expected (with small tolerance for floating point)
			tolerance := 0.01
			if issue.ConsensusScore < tt.expectedScore-tolerance || issue.ConsensusScore > tt.expectedScore+tolerance {
				t.Errorf("Expected ConsensusScore=%.2f (±%.2f), got %.2f", tt.expectedScore, tolerance, issue.ConsensusScore)
			}
		})
	}
}

// TestConsolidateNode_PopulatesConsolidatedIssuesField verifies that ConsolidateNode
// correctly populates the ConsolidatedIssues field in the state delta.
func TestConsolidateNode_PopulatesConsolidatedIssuesField(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with multiple issues
	state := ReviewState{
		Reviews: map[string][]Review{
			"openai": {
				{
					ProviderName: "openai",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "server.go",
							Line:         20,
							Severity:     "high",
							Category:     "security",
							Description:  "Missing authentication check",
							Remediation:  "Add auth middleware",
							ProviderName: "openai",
						},
						{
							File:         "server.go",
							Line:         35,
							Severity:     "medium",
							Category:     "performance",
							Description:  "Blocking I/O in handler",
							Remediation:  "Use goroutine",
							ProviderName: "openai",
						},
					},
				},
			},
		},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: ConsolidatedIssues field is populated in delta
	if result.Delta.ConsolidatedIssues == nil {
		t.Fatal("Expected ConsolidatedIssues to be populated, got nil")
	}

	// Verify: Number of consolidated issues matches input (no duplicates in this case)
	if len(result.Delta.ConsolidatedIssues) != 2 {
		t.Fatalf("Expected 2 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
	}

	// Verify: Each consolidated issue has all required fields
	for i, issue := range result.Delta.ConsolidatedIssues {
		if issue.File == "" {
			t.Errorf("Issue %d: File is empty", i)
		}
		if issue.Line == 0 {
			t.Errorf("Issue %d: Line is zero", i)
		}
		if issue.Severity == "" {
			t.Errorf("Issue %d: Severity is empty", i)
		}
		if issue.Category == "" {
			t.Errorf("Issue %d: Category is empty", i)
		}
		if issue.Description == "" {
			t.Errorf("Issue %d: Description is empty", i)
		}
		if issue.Remediation == "" {
			t.Errorf("Issue %d: Remediation is empty", i)
		}
		if len(issue.Providers) == 0 {
			t.Errorf("Issue %d: Providers list is empty", i)
		}
		if issue.ConsensusScore == 0.0 {
			t.Errorf("Issue %d: ConsensusScore is zero", i)
		}
		if issue.IssueID == "" {
			t.Errorf("Issue %d: IssueID is empty", i)
		}
	}
}

// TestConsolidateNode_RoutesToReportNode verifies that ConsolidateNode
// returns a route directing execution to the "report" node.
func TestConsolidateNode_RoutesToReportNode(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with one issue
	state := ReviewState{
		Reviews: map[string][]Review{
			"openai": {
				{
					ProviderName: "openai",
					BatchNumber:  1,
					Issues: []ReviewIssue{
						{
							File:         "test.go",
							Line:         1,
							Severity:     "info",
							Category:     "style",
							Description:  "Test issue",
							Remediation:  "Fix it",
							ProviderName: "openai",
						},
					},
				},
			},
		},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: Route points to "report" node
	expectedNodeID := "report"
	if result.Route.To != expectedNodeID {
		t.Errorf("Expected Route.To='%s', got '%s'", expectedNodeID, result.Route.To)
	}

	// Verify: Not a terminal node
	if result.Route.Terminal {
		t.Error("Expected Route.Terminal=false, got true")
	}

	// Verify: Not a fan-out route
	if len(result.Route.Many) > 0 {
		t.Errorf("Expected Route.Many to be empty, got %v", result.Route.Many)
	}
}

// TestConsolidateNode_EmptyReviews verifies that ConsolidateNode handles
// the case where there are no reviews gracefully.
func TestConsolidateNode_EmptyReviews(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with no reviews
	state := ReviewState{
		Reviews: map[string][]Review{},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: ConsolidatedIssues is empty
	if len(result.Delta.ConsolidatedIssues) != 0 {
		t.Errorf("Expected 0 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
	}

	// Verify: Still routes to report node (to generate empty report)
	if result.Route.To != "report" {
		t.Errorf("Expected Route.To='report', got '%s'", result.Route.To)
	}
}

// TestConsolidateNode_NoIssuesInReviews verifies that ConsolidateNode handles
// the case where reviews exist but contain no issues.
func TestConsolidateNode_NoIssuesInReviews(t *testing.T) {
	ctx := context.Background()

	// Setup: Create state with reviews but no issues
	state := ReviewState{
		Reviews: map[string][]Review{
			"openai": {
				{
					ProviderName: "openai",
					BatchNumber:  1,
					Issues:       []ReviewIssue{}, // Empty issues
					TokensUsed:   1000,
					Duration:     500,
				},
			},
			"anthropic": {
				{
					ProviderName: "anthropic",
					BatchNumber:  1,
					Issues:       []ReviewIssue{}, // Empty issues
					TokensUsed:   1200,
					Duration:     450,
				},
			},
		},
	}

	// Execute: Run ConsolidateNode
	node := ConsolidateNode{}
	result := node.Run(ctx, state)

	// Verify: No error
	if result.Err != nil {
		t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
	}

	// Verify: ConsolidatedIssues is empty
	if len(result.Delta.ConsolidatedIssues) != 0 {
		t.Errorf("Expected 0 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
	}

	// Verify: Still routes to report node
	if result.Route.To != "report" {
		t.Errorf("Expected Route.To='report', got '%s'", result.Route.To)
	}
}

// TestConsolidateNode_ExactMatchRequiresSameFileLineDescription verifies that
// exact match deduplication only merges issues when File, Line, AND Description
// all match exactly (case-sensitive).
func TestConsolidateNode_ExactMatchRequiresSameFileLineDescription(t *testing.T) {
	tests := []struct {
		name                string
		issue1              ReviewIssue
		issue2              ReviewIssue
		shouldMerge         bool
		expectedIssueCount  int
		expectedDescription string
	}{
		{
			name: "Same file, same line, different description - no merge",
			issue1: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "openai",
			},
			issue2: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "Missing error handling",
				Severity:     "medium",
				Category:     "best-practice",
				ProviderName: "anthropic",
			},
			shouldMerge:        false,
			expectedIssueCount: 2,
		},
		{
			name: "Same file, different line, same description - no merge",
			issue1: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "openai",
			},
			issue2: ReviewIssue{
				File:         "main.go",
				Line:         50,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "anthropic",
			},
			shouldMerge:        false,
			expectedIssueCount: 2,
		},
		{
			name: "Different file, same line, same description - no merge",
			issue1: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "openai",
			},
			issue2: ReviewIssue{
				File:         "handler.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "anthropic",
			},
			shouldMerge:        false,
			expectedIssueCount: 2,
		},
		{
			name: "Same file, same line, same description - merge",
			issue1: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "openai",
			},
			issue2: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "critical",
				Category:     "security",
				ProviderName: "anthropic",
			},
			shouldMerge:         true,
			expectedIssueCount:  1,
			expectedDescription: "SQL injection vulnerability",
		},
		{
			name: "Case-sensitive description - fuzzy match merges",
			issue1: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL injection vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "openai",
			},
			issue2: ReviewIssue{
				File:         "main.go",
				Line:         42,
				Description:  "SQL Injection Vulnerability",
				Severity:     "high",
				Category:     "security",
				ProviderName: "anthropic",
			},
			shouldMerge:         true,
			expectedIssueCount:  1,
			expectedDescription: "SQL injection vulnerability", // First one is kept when same length
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Setup: Create state with two issues
			state := ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  1,
							Issues:       []ReviewIssue{tt.issue1},
						},
					},
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Issues:       []ReviewIssue{tt.issue2},
						},
					},
				},
			}

			// Execute: Run ConsolidateNode
			node := ConsolidateNode{}
			result := node.Run(ctx, state)

			// Verify: No error
			if result.Err != nil {
				t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
			}

			// Verify: Issue count matches expectation
			if len(result.Delta.ConsolidatedIssues) != tt.expectedIssueCount {
				t.Fatalf("Expected %d consolidated issues, got %d", tt.expectedIssueCount, len(result.Delta.ConsolidatedIssues))
			}

			// If should merge, verify merged issue has both providers
			if tt.shouldMerge {
				issue := result.Delta.ConsolidatedIssues[0]
				if len(issue.Providers) != 2 {
					t.Errorf("Expected 2 providers in merged issue, got %d", len(issue.Providers))
				}
				if issue.Description != tt.expectedDescription {
					t.Errorf("Expected Description='%s', got '%s'", tt.expectedDescription, issue.Description)
				}
			}
		})
	}
}
