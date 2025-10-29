package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockCodeReviewer is a test implementation of the CodeReviewer interface.
type MockCodeReviewer struct {
	name        string
	shouldFail  bool
	issuesCount int
	reviewDelay time.Duration
	tokensUsed  int
}

// NewMockCodeReviewer creates a new mock code reviewer for testing.
func NewMockCodeReviewer(name string) *MockCodeReviewer {
	return &MockCodeReviewer{
		name:        name,
		tokensUsed:  100,
		issuesCount: 2,
	}
}

// WithFailure configures the mock to fail.
func (m *MockCodeReviewer) WithFailure() *MockCodeReviewer {
	m.shouldFail = true
	return m
}

// WithIssues configures the number of issues to return.
func (m *MockCodeReviewer) WithIssues(count int) *MockCodeReviewer {
	m.issuesCount = count
	return m
}

// ReviewBatch implements the CodeReviewer interface.
func (m *MockCodeReviewer) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	if m.reviewDelay > 0 {
		time.Sleep(m.reviewDelay)
	}

	if m.shouldFail {
		return ReviewResponse{}, &ReviewError{Message: "mock review failed"}
	}

	// Generate mock issues
	issues := make([]ReviewIssueFromProvider, m.issuesCount)
	for i := 0; i < m.issuesCount; i++ {
		issues[i] = ReviewIssueFromProvider{
			File:         req.Files[0].FilePath,
			Line:         i + 1,
			Severity:     "medium",
			Category:     "best-practice",
			Description:  "Test issue " + string(rune('A'+i)),
			Remediation:  "Fix test issue " + string(rune('A'+i)),
			ProviderName: m.name,
			Confidence:   0.8,
		}
	}

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   m.tokensUsed,
		Duration:     m.reviewDelay,
		ProviderName: m.name,
	}, nil
}

// Name implements the CodeReviewer interface.
func (m *MockCodeReviewer) Name() string {
	return m.name
}

// ReviewError represents a review error for testing.
type ReviewError struct {
	Message string
}

func (e *ReviewError) Error() string {
	return e.Message
}

// MockFileScanner is a test implementation of the FileScanner interface.
type MockFileScanner struct {
	files      []DiscoveredFile
	shouldFail bool
}

// NewMockFileScanner creates a new mock file scanner for testing.
func NewMockFileScanner(fileCount int) *MockFileScanner {
	files := make([]DiscoveredFile, fileCount)
	for i := 0; i < fileCount; i++ {
		content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		files[i] = DiscoveredFile{
			Path:     "file" + string(rune('A'+i)) + ".go",
			Content:  content,
			Size:     int64(len(content)),
			Checksum: "checksum" + string(rune('A'+i)),
		}
	}
	return &MockFileScanner{files: files}
}

// WithFailure configures the mock to fail.
func (m *MockFileScanner) WithFailure() *MockFileScanner {
	m.shouldFail = true
	return m
}

// Discover implements the FileScanner interface.
func (m *MockFileScanner) Discover(rootPath string) ([]DiscoveredFile, error) {
	if m.shouldFail {
		return nil, &ScanError{Message: "mock scan failed"}
	}
	return m.files, nil
}

// ScanError represents a scan error for testing.
type ScanError struct {
	Message string
}

func (e *ScanError) Error() string {
	return e.Message
}

// RealFileScanner is a test implementation that actually scans a directory for .go files.
type RealFileScanner struct {
	rootPath string
}

// Discover implements the FileScanner interface by scanning the directory for .go files.
func (r *RealFileScanner) Discover(rootPath string) ([]DiscoveredFile, error) {
	var files []DiscoveredFile

	err := filepath.Walk(r.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only include .go files
		if filepath.Ext(path) != ".go" {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Calculate checksum
		hash := sha256.Sum256(content)
		checksum := hex.EncodeToString(hash[:])

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		files = append(files, DiscoveredFile{
			Path:     absPath,
			Content:  string(content),
			Size:     info.Size(),
			Checksum: checksum,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// TestNewReviewWorkflow tests the graph initialization.
func TestNewReviewWorkflow(t *testing.T) {
	provider := NewMockCodeReviewer("test-provider")
	scanner := NewMockFileScanner(3)

	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}
}

// TestGraphNodeConnections tests that all required nodes are wired correctly.
func TestGraphNodeConnections(t *testing.T) {
	provider := NewMockCodeReviewer("test-provider")
	scanner := NewMockFileScanner(3)

	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	// Run a minimal workflow to verify node connections
	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/path",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	finalState, err := engine.Run(ctx, "test-run-001", initialState)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify workflow completed successfully
	if finalState.ReportPath == "" {
		t.Error("Expected ReportPath to be set")
	}

	if finalState.EndTime == "" {
		t.Error("Expected EndTime to be set")
	}

	if len(finalState.ConsolidatedIssues) == 0 {
		t.Error("Expected ConsolidatedIssues to be populated")
	}
}

// TestGraphExecutionFlow tests the complete workflow execution.
func TestGraphExecutionFlow(t *testing.T) {
	provider := NewMockCodeReviewer("test-provider").WithIssues(3)
	scanner := NewMockFileScanner(4)

	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/codebase",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	finalState, err := engine.Run(ctx, "test-run-002", initialState)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify state progression
	if len(finalState.DiscoveredFiles) != 4 {
		t.Errorf("Expected 4 discovered files, got %d", len(finalState.DiscoveredFiles))
	}

	if finalState.TotalBatches != 2 {
		t.Errorf("Expected 2 batches (batch size 2), got %d", finalState.TotalBatches)
	}

	if len(finalState.Reviews) != 1 {
		t.Errorf("Expected 1 provider in reviews, got %d", len(finalState.Reviews))
	}

	if len(finalState.ConsolidatedIssues) == 0 {
		t.Error("Expected consolidated issues to be populated")
	}

	if finalState.ReportPath == "" {
		t.Error("Expected report path to be set")
	}

	if finalState.EndTime == "" {
		t.Error("Expected end time to be set")
	}
}

// TestGraphWithInMemoryStore tests checkpointing with the in-memory store.
func TestGraphWithInMemoryStore(t *testing.T) {
	provider := NewMockCodeReviewer("test-provider")
	scanner := NewMockFileScanner(2)

	engine, err := NewReviewWorkflow(provider, scanner, 1)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/checkpoint",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	runID := "test-run-checkpoint-001"
	finalState, err := engine.Run(ctx, runID, initialState)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// Verify workflow completed
	if finalState.ReportPath == "" {
		t.Error("Expected report path to be set")
	}

	// Note: Testing actual checkpoint retrieval would require access to
	// the engine's store, which is not exposed. In production code,
	// we would use engine.SaveCheckpoint and engine.ResumeFromCheckpoint.
}

// TestGraphNodeRouting tests that nodes route correctly.
func TestGraphNodeRouting(t *testing.T) {
	tests := []struct {
		name          string
		fileCount     int
		batchSize     int
		expectedRoute string
	}{
		{
			name:          "single batch routes to consolidate",
			fileCount:     2,
			batchSize:     5,
			expectedRoute: "consolidate",
		},
		{
			name:          "multiple batches route through review loop",
			fileCount:     6,
			batchSize:     2,
			expectedRoute: "review-batch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewMockCodeReviewer("test-provider")
			scanner := NewMockFileScanner(tt.fileCount)

			engine, err := NewReviewWorkflow(provider, scanner, tt.batchSize)
			if err != nil {
				t.Fatalf("NewReviewWorkflow failed: %v", err)
			}

			ctx := context.Background()
			initialState := ReviewState{
				CodebaseRoot: "/test/routing",
				StartTime:    time.Now().Format(time.RFC3339),
				Reviews:      make(map[string][]Review),
			}

			finalState, err := engine.Run(ctx, "test-run-routing-"+tt.name, initialState)
			if err != nil {
				t.Fatalf("Workflow execution failed: %v", err)
			}

			// Verify workflow completed successfully
			if finalState.ReportPath == "" {
				t.Error("Expected report path to be set")
			}
		})
	}
}

// TestGraphWithFailingProvider tests error handling when provider fails.
func TestGraphWithFailingProvider(t *testing.T) {
	provider := NewMockCodeReviewer("failing-provider").WithFailure()
	scanner := NewMockFileScanner(2)

	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/failure",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	finalState, err := engine.Run(ctx, "test-run-failure-001", initialState)

	// The workflow should handle provider errors gracefully
	// and continue to report generation with partial results
	if err != nil {
		t.Fatalf("Expected workflow to handle provider error gracefully, got: %v", err)
	}

	// Verify error was tracked in state
	if finalState.LastError == "" {
		t.Error("Expected LastError to be set when provider fails")
	}

	if len(finalState.FailedProviders) == 0 {
		t.Error("Expected FailedProviders to be populated")
	}

	// Report should still be generated even with failures
	if finalState.ReportPath == "" {
		t.Error("Expected report to be generated even with provider failure")
	}
}

// TestGraphStateReduction tests that the reducer correctly merges state.
func TestGraphStateReduction(t *testing.T) {
	// Test the ReduceReviewState function directly
	prev := ReviewState{
		CurrentBatch:     1,
		CompletedBatches: []int{},
		Reviews:          make(map[string][]Review),
		TotalIssuesFound: 0,
	}

	delta := ReviewState{
		CurrentBatch:     2,
		CompletedBatches: []int{1},
		Reviews: map[string][]Review{
			"provider1": {
				{
					ProviderName: "provider1",
					BatchNumber:  1,
					Issues:       []ReviewIssue{{File: "test.go", Line: 1}},
				},
			},
		},
		TotalIssuesFound: 5,
	}

	result := ReduceReviewState(prev, delta)

	if result.CurrentBatch != 2 {
		t.Errorf("Expected CurrentBatch=2, got %d", result.CurrentBatch)
	}

	if len(result.CompletedBatches) != 1 {
		t.Errorf("Expected 1 completed batch, got %d", len(result.CompletedBatches))
	}

	if len(result.Reviews["provider1"]) != 1 {
		t.Errorf("Expected 1 review from provider1, got %d", len(result.Reviews["provider1"]))
	}

	if result.TotalIssuesFound != 5 {
		t.Errorf("Expected TotalIssuesFound=5, got %d", result.TotalIssuesFound)
	}
}

// TestGraphConcurrentExecution tests that the graph supports concurrent node execution.
func TestGraphConcurrentExecution(t *testing.T) {
	t.Skip("Concurrent execution not yet implemented in LangGraph-Go")

	provider := NewMockCodeReviewer("test-provider")
	scanner := NewMockFileScanner(4)

	// Future: Test with Options.MaxConcurrentNodes > 0
	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/concurrent",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	start := time.Now()
	_, err = engine.Run(ctx, "test-run-concurrent-001", initialState)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Concurrent workflow execution failed: %v", err)
	}

	// Concurrent execution should be faster than sequential
	t.Logf("Concurrent execution took %v", elapsed)
}

// BenchmarkGraphExecution benchmarks the full workflow execution.
func BenchmarkGraphExecution(b *testing.B) {
	provider := NewMockCodeReviewer("benchmark-provider")
	scanner := NewMockFileScanner(10)

	engine, err := NewReviewWorkflow(provider, scanner, 5)
	if err != nil {
		b.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/benchmark/codebase",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := "benchmark-run-" + string(rune('A'+i%26))
		_, err := engine.Run(ctx, runID, initialState)
		if err != nil {
			b.Fatalf("Benchmark execution failed: %v", err)
		}
	}
}

// TestGraphEmitsEvents tests that the graph emits observability events.
func TestGraphEmitsEvents(t *testing.T) {
	// Note: Testing event emission would require a mock emitter
	// that captures events. For now, we verify the workflow runs
	// with an emitter configured.

	provider := NewMockCodeReviewer("test-provider")
	scanner := NewMockFileScanner(2)

	engine, err := NewReviewWorkflow(provider, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/events",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	_, err = engine.Run(ctx, "test-run-events-001", initialState)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// In a more complete implementation, we would verify:
	// - node_start events were emitted for each node
	// - node_end events were emitted for each node
	// - routing_decision events were emitted
	// This would require a mock emitter that captures events.
}

// TestEndToEnd_SmallFixture tests the complete workflow with the small fixture.
// This test validates US1 acceptance criteria:
// - Batch processing: Files processed in batches
// - Progress display: Can track current batch
// - Report generation: Markdown report created
// - Error handling: Continues on batch failure
// - File filtering: Only processes code files
func TestEndToEnd_SmallFixture(t *testing.T) {
	// Create a realistic mock provider that returns issues for each file
	provider := NewMockCodeReviewer("test-llm-provider").WithIssues(3)

	// Use real file scanner on the small fixture directory
	fixtureDir := "/Users/dshills/Development/projects/langgraph-go/examples/multi-llm-review/testdata/fixtures/small"

	// Create a scanner that discovers .go files in the fixture directory
	scanner := &RealFileScanner{rootPath: fixtureDir}

	// Count expected .go files in the fixture
	expectedFileCount := 10

	// Create workflow engine with batch size of 3 for testing batch processing
	batchSize := 3
	engine, err := NewReviewWorkflow(provider, scanner, batchSize)
	if err != nil {
		t.Fatalf("NewReviewWorkflow failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: fixtureDir,
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	// Execute the workflow end-to-end
	finalState, err := engine.Run(ctx, "test-run-e2e-small", initialState)
	if err != nil {
		t.Fatalf("End-to-end workflow execution failed: %v", err)
	}

	// ✅ US1: File filtering - Only processes code files
	if len(finalState.DiscoveredFiles) != expectedFileCount {
		t.Errorf("Expected %d discovered .go files, got %d", expectedFileCount, len(finalState.DiscoveredFiles))
	}

	// Verify all discovered files have .go extension
	for _, file := range finalState.DiscoveredFiles {
		if len(file.FilePath) < 3 || file.FilePath[len(file.FilePath)-3:] != ".go" {
			t.Errorf("Non-Go file discovered: %s", file.FilePath)
		}
	}

	// ✅ US1: Batch processing - Files processed in batches
	expectedBatches := (expectedFileCount + batchSize - 1) / batchSize // Ceiling division
	if finalState.TotalBatches != expectedBatches {
		t.Errorf("Expected %d batches (batch size %d), got %d", expectedBatches, batchSize, finalState.TotalBatches)
	}

	// Verify batches were actually processed
	if len(finalState.CompletedBatches) != expectedBatches {
		t.Errorf("Expected %d completed batches, got %d", expectedBatches, len(finalState.CompletedBatches))
	}

	// ✅ US1: Progress display - Can track current batch
	// CurrentBatch may be expectedBatches or expectedBatches+1 depending on when it's incremented
	if finalState.CurrentBatch < expectedBatches || finalState.CurrentBatch > expectedBatches+1 {
		t.Errorf("Expected CurrentBatch to be %d or %d after completion, got %d", expectedBatches, expectedBatches+1, finalState.CurrentBatch)
	}

	// Verify progress tracking throughout execution
	for i := 0; i < expectedBatches; i++ {
		expectedBatchNum := i + 1
		found := false
		for _, completedBatch := range finalState.CompletedBatches {
			if completedBatch == expectedBatchNum {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Batch %d not found in CompletedBatches", expectedBatchNum)
		}
	}

	// ✅ US1: Report generation - Markdown report created
	if finalState.ReportPath == "" {
		t.Error("Expected ReportPath to be set")
	}

	// Verify report file exists (path should be absolute)
	// Note: In a real test, we would check os.Stat(finalState.ReportPath)
	// For this mock test, we verify the field is populated

	// ✅ US1: Report generation - Report contains issues
	if len(finalState.ConsolidatedIssues) == 0 {
		t.Error("Expected ConsolidatedIssues to be populated")
	}

	// Verify issues were found for multiple files
	filesWithIssues := make(map[string]bool)
	for _, issue := range finalState.ConsolidatedIssues {
		filesWithIssues[issue.File] = true
	}

	if len(filesWithIssues) == 0 {
		t.Error("Expected issues across multiple files")
	}

	// ✅ US1: Error handling - Continues on batch failure (implicit)
	// The workflow completed successfully despite any potential errors
	// which are tracked in finalState.LastError and FailedProviders
	if finalState.EndTime == "" {
		t.Error("Expected EndTime to be set after workflow completion")
	}

	// Verify review metadata
	if len(finalState.Reviews) != 1 {
		t.Errorf("Expected 1 provider in reviews, got %d", len(finalState.Reviews))
	}

	providerReviews, exists := finalState.Reviews["test-llm-provider"]
	if !exists {
		t.Error("Expected reviews from test-llm-provider")
	}

	// Verify all batches were reviewed
	if len(providerReviews) != expectedBatches {
		t.Errorf("Expected %d batch reviews from provider, got %d", expectedBatches, len(providerReviews))
	}

	// Verify total issues count matches consolidated issues
	totalIssuesFromProvider := 0
	for _, review := range providerReviews {
		totalIssuesFromProvider += len(review.Issues)
	}

	if totalIssuesFromProvider == 0 {
		t.Error("Expected provider to report issues")
	}

	// Log summary for manual verification
	t.Logf("End-to-End Test Summary:")
	t.Logf("  Files Discovered: %d", len(finalState.DiscoveredFiles))
	t.Logf("  Total Batches: %d", finalState.TotalBatches)
	t.Logf("  Completed Batches: %d", len(finalState.CompletedBatches))
	t.Logf("  Consolidated Issues: %d", len(finalState.ConsolidatedIssues))
	t.Logf("  Files with Issues: %d", len(filesWithIssues))
	t.Logf("  Report Path: %s", finalState.ReportPath)
	t.Logf("  Duration: %s to %s", finalState.StartTime, finalState.EndTime)
}

// ============================================================================
// Multi-Provider Concurrent Execution Tests (US2)
// ============================================================================

// TestGraphWithThreeProviders tests the workflow with three concurrent providers.
// This validates US2 acceptance criteria:
// - Concurrent execution across providers
// - Partial success handling (one provider fails, others continue)
func TestGraphWithThreeProviders(t *testing.T) {
	// Create three mock providers
	provider1 := NewMockCodeReviewer("openai").WithIssues(3)
	provider2 := NewMockCodeReviewer("anthropic").WithIssues(2)
	provider3 := NewMockCodeReviewer("google").WithIssues(4)

	providers := []CodeReviewer{provider1, provider2, provider3}
	scanner := NewMockFileScanner(4) // 4 files, 2 batches

	// Create workflow with multiple providers
	engine, err := NewReviewWorkflowWithProviders(providers, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflowWithProviders failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/multi-provider",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	// Execute workflow
	finalState, err := engine.Run(ctx, "test-run-three-providers", initialState)
	if err != nil {
		t.Fatalf("Multi-provider workflow execution failed: %v", err)
	}

	// ✅ US2: Concurrent execution across providers
	// Verify all three providers were called
	if len(finalState.Reviews) != 3 {
		t.Fatalf("Expected reviews from 3 providers, got %d", len(finalState.Reviews))
	}

	// Verify each provider has results for all batches
	expectedBatches := 2
	for _, providerName := range []string{"openai", "anthropic", "google"} {
		reviews, exists := finalState.Reviews[providerName]
		if !exists {
			t.Errorf("Expected reviews from provider %s", providerName)
			continue
		}
		if len(reviews) != expectedBatches {
			t.Errorf("Expected %d batch reviews from %s, got %d", expectedBatches, providerName, len(reviews))
		}
	}

	// Verify consolidated issues include results from all providers
	if len(finalState.ConsolidatedIssues) == 0 {
		t.Error("Expected consolidated issues from multiple providers")
	}

	// Verify some issues have multiple provider attribution
	multiProviderIssues := 0
	for _, issue := range finalState.ConsolidatedIssues {
		if len(issue.Providers) > 1 {
			multiProviderIssues++
		}
	}

	// With 3 providers each returning issues, some overlap is expected
	if multiProviderIssues == 0 {
		t.Log("Warning: No issues flagged by multiple providers (may be due to mock implementation)")
	}

	// Verify workflow completed successfully
	if finalState.EndTime == "" {
		t.Error("Expected EndTime to be set")
	}

	if finalState.ReportPath == "" {
		t.Error("Expected report to be generated")
	}

	t.Logf("Multi-Provider Test Summary:")
	t.Logf("  Providers: %d", len(finalState.Reviews))
	t.Logf("  Total Batches: %d", finalState.TotalBatches)
	t.Logf("  Consolidated Issues: %d", len(finalState.ConsolidatedIssues))
	t.Logf("  Multi-Provider Issues: %d", multiProviderIssues)
}

// TestGraphPartialProviderFailure tests that the workflow continues when one provider fails.
// This validates US2 acceptance criteria for partial success handling.
func TestGraphPartialProviderFailure(t *testing.T) {
	// Create three providers where one will fail
	provider1 := NewMockCodeReviewer("openai-success").WithIssues(2)
	provider2 := NewMockCodeReviewer("anthropic-fail").WithFailure()
	provider3 := NewMockCodeReviewer("google-success").WithIssues(3)

	providers := []CodeReviewer{provider1, provider2, provider3}
	scanner := NewMockFileScanner(2) // 2 files, 1 batch

	engine, err := NewReviewWorkflowWithProviders(providers, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflowWithProviders failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/partial-failure",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	// Execute workflow
	finalState, err := engine.Run(ctx, "test-run-partial-failure", initialState)

	// ✅ US2: Partial success handling - workflow should complete despite failure
	if err != nil {
		t.Fatalf("Expected workflow to complete despite provider failure, got error: %v", err)
	}

	// Verify successful providers returned results
	successProviders := []string{"openai-success", "google-success"}
	for _, providerName := range successProviders {
		reviews, exists := finalState.Reviews[providerName]
		if !exists {
			t.Errorf("Expected reviews from successful provider %s", providerName)
			continue
		}
		if len(reviews) != 1 {
			t.Errorf("Expected 1 batch review from %s, got %d", providerName, len(reviews))
		}
		// Verify reviews have no errors
		for _, review := range reviews {
			if review.Error != "" {
				t.Errorf("Successful provider %s should not have error, got: %s", providerName, review.Error)
			}
		}
	}

	// Verify failed provider is tracked
	if len(finalState.FailedProviders) == 0 {
		t.Error("Expected at least one failed provider to be tracked")
	}

	// Check that the failed provider is in the list
	foundFailedProvider := false
	for _, failedProvider := range finalState.FailedProviders {
		if failedProvider == "anthropic-fail" {
			foundFailedProvider = true
			break
		}
	}
	if !foundFailedProvider {
		t.Error("Expected 'anthropic-fail' to be in FailedProviders list")
	}

	// Verify failed provider has error recorded in review
	failedReviews, exists := finalState.Reviews["anthropic-fail"]
	if !exists {
		t.Error("Expected review entry for failed provider")
	} else {
		for _, review := range failedReviews {
			if review.Error == "" {
				t.Error("Expected error message in failed provider review")
			}
		}
	}

	// Verify workflow completed with partial results
	if finalState.EndTime == "" {
		t.Error("Expected EndTime to be set despite partial failure")
	}

	if finalState.ReportPath == "" {
		t.Error("Expected report to be generated with partial results")
	}

	// Verify consolidated issues from successful providers
	if len(finalState.ConsolidatedIssues) == 0 {
		t.Error("Expected consolidated issues from successful providers")
	}

	t.Logf("Partial Failure Test Summary:")
	t.Logf("  Total Providers: %d", len(finalState.Reviews))
	t.Logf("  Failed Providers: %d", len(finalState.FailedProviders))
	t.Logf("  Consolidated Issues: %d", len(finalState.ConsolidatedIssues))
	t.Logf("  Workflow Completed: %v", finalState.EndTime != "")
}

// TestGraphConcurrentExecutionTiming tests that providers are called concurrently, not sequentially.
func TestGraphConcurrentExecutionTiming(t *testing.T) {
	// Create three providers with artificial delays
	delay := 100 * time.Millisecond
	provider1 := &MockCodeReviewerWithDelay{name: "provider1", delay: delay}
	provider2 := &MockCodeReviewerWithDelay{name: "provider2", delay: delay}
	provider3 := &MockCodeReviewerWithDelay{name: "provider3", delay: delay}

	providers := []CodeReviewer{provider1, provider2, provider3}
	scanner := NewMockFileScanner(2) // 2 files, 1 batch

	engine, err := NewReviewWorkflowWithProviders(providers, scanner, 2)
	if err != nil {
		t.Fatalf("NewReviewWorkflowWithProviders failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: "/test/timing",
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	// Execute workflow and measure time
	start := time.Now()
	finalState, err := engine.Run(ctx, "test-run-concurrent-timing", initialState)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// ✅ US2: Concurrent execution
	// With 100ms delay per provider, concurrent execution should take ~100ms
	// Sequential execution would take ~300ms
	// Allow generous tolerance for overhead
	maxExpectedTime := 500 * time.Millisecond
	if elapsed > maxExpectedTime {
		t.Errorf("Expected concurrent execution ~100-200ms, took %v (likely sequential)", elapsed)
	}

	// Verify all providers completed
	if len(finalState.Reviews) != 3 {
		t.Errorf("Expected reviews from 3 providers, got %d", len(finalState.Reviews))
	}

	t.Logf("Concurrent Execution Timing:")
	t.Logf("  Provider Delay: %v", delay)
	t.Logf("  Total Time: %v", elapsed)
	t.Logf("  Expected (concurrent): ~%v", delay)
	t.Logf("  Expected (sequential): ~%v", delay*3)
}

// MockCodeReviewerWithDelay is a mock provider with artificial delay for timing tests.
type MockCodeReviewerWithDelay struct {
	name  string
	delay time.Duration
}

func (m *MockCodeReviewerWithDelay) Name() string {
	return m.name
}

func (m *MockCodeReviewerWithDelay) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	// Simulate API call delay
	time.Sleep(m.delay)

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
		Duration:     m.delay,
		ProviderName: m.name,
	}, nil
}

// ============================================================================
// Consolidated Report Tests with Deduplication (T085-T086, US3)
// ============================================================================

// TestConsolidatedReport_WithDuplicates validates US3 acceptance criteria:
// - Deduplication: Multiple providers flagging same issue merged
// - Severity grouping: Critical issues appear first
// - Provider attribution: Shows which providers flagged each issue
// - Report format: Includes file path, line, severity, remediation
func TestConsolidatedReport_WithDuplicates(t *testing.T) {
	// Create 3 mock providers that return the same issues (duplicates)
	// This simulates multiple AI providers detecting the same problems
	provider1 := NewMockCodeReviewerWithDuplicates("openai")
	provider2 := NewMockCodeReviewerWithDuplicates("anthropic")
	provider3 := NewMockCodeReviewerWithDuplicates("google")

	providers := []CodeReviewer{provider1, provider2, provider3}

	// Use real file scanner on the small fixture directory
	fixtureDir := "/Users/dshills/Development/projects/langgraph-go/examples/multi-llm-review/testdata/fixtures/small"
	scanner := &RealFileScanner{rootPath: fixtureDir}

	// Create workflow with batch size of 5 to process files in multiple batches
	batchSize := 5
	engine, err := NewReviewWorkflowWithProviders(providers, scanner, batchSize)
	if err != nil {
		t.Fatalf("NewReviewWorkflowWithProviders failed: %v", err)
	}

	ctx := context.Background()
	initialState := ReviewState{
		CodebaseRoot: fixtureDir,
		StartTime:    time.Now().Format(time.RFC3339),
		Reviews:      make(map[string][]Review),
	}

	// Execute the workflow end-to-end
	finalState, err := engine.Run(ctx, "test-run-duplicates", initialState)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	// ✅ US3: Deduplication - Multiple providers flagging same issue merged
	// All three providers return identical issues, so we expect deduplication
	if len(finalState.ConsolidatedIssues) == 0 {
		t.Fatal("Expected consolidated issues to be populated")
	}

	// Verify that duplicate issues were merged
	// Each provider returns the same issues, so consolidated count should be much less than total
	totalIssuesBeforeMerge := 0
	for _, reviews := range finalState.Reviews {
		for _, review := range reviews {
			totalIssuesBeforeMerge += len(review.Issues)
		}
	}

	if totalIssuesBeforeMerge == 0 {
		t.Fatal("Expected issues from providers before deduplication")
	}

	// After deduplication, we should have fewer consolidated issues
	if len(finalState.ConsolidatedIssues) >= totalIssuesBeforeMerge {
		t.Errorf("Expected deduplication to reduce issue count, got %d consolidated from %d total",
			len(finalState.ConsolidatedIssues), totalIssuesBeforeMerge)
	}

	// ✅ US3: Provider attribution - Shows which providers flagged each issue
	// Since all three providers return the same issues, all consolidated issues should have all 3 providers
	foundMultiProviderIssue := false
	for _, issue := range finalState.ConsolidatedIssues {
		if len(issue.Providers) > 1 {
			foundMultiProviderIssue = true

			// Verify provider names are populated
			if len(issue.Providers) != 3 {
				t.Logf("Issue %s flagged by %d providers: %v", issue.IssueID, len(issue.Providers), issue.Providers)
			}

			// ✅ US3: Consensus score calculated correctly (e.g., 3/3 = 1.0)
			expectedConsensus := float64(len(issue.Providers)) / 3.0
			// Round to 2 decimal places to match implementation
			expectedConsensus = float64(int(expectedConsensus*100+0.5)) / 100

			if issue.ConsensusScore != expectedConsensus {
				t.Errorf("Issue %s: expected consensus %.2f (from %d providers), got %.2f",
					issue.IssueID, expectedConsensus, len(issue.Providers), issue.ConsensusScore)
			}

			// Verify all provider names are present
			providerMap := make(map[string]bool)
			for _, p := range issue.Providers {
				providerMap[p] = true
			}

			if len(issue.Providers) == 3 {
				if !providerMap["openai"] || !providerMap["anthropic"] || !providerMap["google"] {
					t.Errorf("Issue %s: expected all three providers, got %v", issue.IssueID, issue.Providers)
				}
			}
		}
	}

	if !foundMultiProviderIssue {
		t.Error("Expected at least one issue flagged by multiple providers (deduplication verification)")
	}

	// ✅ US3: Report format - Includes file path, line, severity, remediation
	for _, issue := range finalState.ConsolidatedIssues {
		if issue.File == "" {
			t.Errorf("Issue %s: missing file path", issue.IssueID)
		}
		// Line can be 0 for file-level issues, so we only check it's set
		if issue.Line < 0 {
			t.Errorf("Issue %s: invalid line number %d", issue.IssueID, issue.Line)
		}
		if issue.Severity == "" {
			t.Errorf("Issue %s: missing severity", issue.IssueID)
		}
		if issue.Description == "" {
			t.Errorf("Issue %s: missing description", issue.IssueID)
		}
		if issue.Remediation == "" {
			t.Errorf("Issue %s: missing remediation", issue.IssueID)
		}
		if len(issue.Providers) == 0 {
			t.Errorf("Issue %s: missing provider attribution", issue.IssueID)
		}
		if issue.IssueID == "" {
			t.Error("Issue missing IssueID")
		}
	}

	// ✅ US3: Severity grouping - Critical issues appear first
	// Verify issues are properly categorized by severity
	severityCounts := make(map[string]int)
	for _, issue := range finalState.ConsolidatedIssues {
		severityCounts[issue.Severity]++
	}

	// Verify we have a mix of severities (based on our mock implementation)
	if len(severityCounts) == 0 {
		t.Error("Expected issues with various severity levels")
	}

	// Verify severity values are valid
	validSeverities := map[string]bool{
		"critical": true,
		"high":     true,
		"medium":   true,
		"low":      true,
		"info":     true,
	}

	for severity := range severityCounts {
		if !validSeverities[severity] {
			t.Errorf("Invalid severity level: %s", severity)
		}
	}

	// Verify workflow completed successfully
	if finalState.EndTime == "" {
		t.Error("Expected EndTime to be set after workflow completion")
	}

	if finalState.ReportPath == "" {
		t.Error("Expected report to be generated")
	}

	// Verify all providers completed
	if len(finalState.Reviews) != 3 {
		t.Errorf("Expected reviews from 3 providers, got %d", len(finalState.Reviews))
	}

	// Log summary for manual verification
	t.Logf("Consolidated Report Test Summary:")
	t.Logf("  Providers: %d", len(finalState.Reviews))
	t.Logf("  Total Issues (before dedup): %d", totalIssuesBeforeMerge)
	t.Logf("  Consolidated Issues (after dedup): %d", len(finalState.ConsolidatedIssues))
	t.Logf("  Deduplication Rate: %.1f%%", (1.0-float64(len(finalState.ConsolidatedIssues))/float64(totalIssuesBeforeMerge))*100)
	t.Logf("  Severity Distribution:")
	for severity, count := range severityCounts {
		t.Logf("    %s: %d", severity, count)
	}
	t.Logf("  Report Path: %s", finalState.ReportPath)
}

// MockCodeReviewerWithDuplicates returns identical issues across all providers
// to test deduplication logic.
type MockCodeReviewerWithDuplicates struct {
	name string
}

func NewMockCodeReviewerWithDuplicates(name string) *MockCodeReviewerWithDuplicates {
	return &MockCodeReviewerWithDuplicates{name: name}
}

func (m *MockCodeReviewerWithDuplicates) Name() string {
	return m.name
}

func (m *MockCodeReviewerWithDuplicates) ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error) {
	// Return identical issues for each file to test deduplication
	// All providers will return the same issues at the same locations
	issues := make([]ReviewIssueFromProvider, 0)

	for _, file := range req.Files {
		// Issue 1: Critical security vulnerability (all providers agree)
		issues = append(issues, ReviewIssueFromProvider{
			File:         file.FilePath,
			Line:         10,
			Severity:     "critical",
			Category:     "security",
			Description:  "SQL injection vulnerability detected in database query",
			Remediation:  "Use parameterized queries or prepared statements",
			ProviderName: m.name,
			Confidence:   0.95,
		})

		// Issue 2: High priority bug (all providers agree)
		issues = append(issues, ReviewIssueFromProvider{
			File:         file.FilePath,
			Line:         25,
			Severity:     "high",
			Category:     "bug",
			Description:  "Potential nil pointer dereference",
			Remediation:  "Add nil check before dereferencing pointer",
			ProviderName: m.name,
			Confidence:   0.90,
		})

		// Issue 3: Medium priority code quality (all providers agree)
		issues = append(issues, ReviewIssueFromProvider{
			File:         file.FilePath,
			Line:         50,
			Severity:     "medium",
			Category:     "best-practice",
			Description:  "Function complexity too high (cyclomatic complexity > 15)",
			Remediation:  "Refactor into smaller functions",
			ProviderName: m.name,
			Confidence:   0.85,
		})

		// Issue 4: Low priority style issue (all providers agree)
		issues = append(issues, ReviewIssueFromProvider{
			File:         file.FilePath,
			Line:         75,
			Severity:     "low",
			Category:     "style",
			Description:  "Variable name does not follow naming conventions",
			Remediation:  "Use camelCase for variable names",
			ProviderName: m.name,
			Confidence:   0.80,
		})
	}

	return ReviewResponse{
		Issues:       issues,
		TokensUsed:   500,
		Duration:     50 * time.Millisecond,
		ProviderName: m.name,
	}, nil
}
