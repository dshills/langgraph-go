package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/consolidator"
	"github.com/dshills/langgraph-go/graph"
)

// ReportNode generates a markdown report from consolidated issues.
// It writes the report to the output directory and sets the ReportPath in state.
type ReportNode struct {
	// No configuration fields needed for basic implementation
}

// Run generates the markdown report file and updates state.
func (n *ReportNode) Run(ctx context.Context, state ReviewState) graph.NodeResult[ReviewState] {
	// Determine output directory
	outputDir := "./review-results/"
	if state.CodebaseRoot != "" {
		// Could be configured via config in the future
		outputDir = "./review-results/"
	}

	// Create output directory if needed
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("failed to create output directory: %w", err),
		}
	}

	// Generate report filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	reportPath := filepath.Join(outputDir, fmt.Sprintf("review-report-%s.md", timestamp))

	// Generate markdown content using consolidator
	content := consolidator.GenerateMarkdownReport(state)

	// Write report file
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("failed to write report file: %w", err),
		}
	}

	// Update state with report path and end time
	delta := ReviewState{
		ReportPath: reportPath,
		EndTime:    time.Now().Format(time.RFC3339),
	}

	return graph.NodeResult[ReviewState]{
		Delta: delta,
		Route: graph.Stop(),
	}
}

// ReviewBatchNode processes a single batch of code files through multiple
// AI providers concurrently. It retrieves the current batch from state, calls
// all providers' ReviewBatch methods concurrently, collects results, and
// determines the next routing.
type ReviewBatchNode struct {
	providers []CodeReviewer
}

// CodeReviewer is the interface for AI providers that perform code reviews.
// This is re-exported here to avoid importing providers package in tests.
type CodeReviewer interface {
	ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
	Name() string
}

// ReviewRequest contains the input for a code review operation.
type ReviewRequest struct {
	Files       []CodeFileForReview
	FocusAreas  []string
	Language    string
	BatchNumber int
}

// CodeFileForReview represents a code file for review.
type CodeFileForReview struct {
	FilePath  string
	Content   string
	Language  string
	LineCount int
	SizeBytes int64
	Checksum  string
}

// ReviewResponse contains the output from a code review operation.
type ReviewResponse struct {
	Issues       []ReviewIssueFromProvider
	TokensUsed   int
	Duration     time.Duration
	ProviderName string
}

// ReviewIssueFromProvider represents an issue from a provider.
type ReviewIssueFromProvider struct {
	File         string
	Line         int
	Severity     string
	Category     string
	Description  string
	Remediation  string
	ProviderName string
	Confidence   float64
}

// NewReviewBatchNode creates a new ReviewBatchNode with the given providers.
// All providers will be called concurrently for each batch.
func NewReviewBatchNode(providers []CodeReviewer) *ReviewBatchNode {
	return &ReviewBatchNode{
		providers: providers,
	}
}

// Run implements the graph.Node interface for ReviewBatchNode.
// It calls all configured providers concurrently and collects results.
func (n *ReviewBatchNode) Run(ctx context.Context, state ReviewState) graph.NodeResult[ReviewState] {
	// Validate providers are set
	if len(n.providers) == 0 {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("no providers configured"),
		}
	}

	// Validate state has batches
	if len(state.Batches) == 0 {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("no batches to process"),
		}
	}

	// Validate CurrentBatch is within range (1-indexed)
	if state.CurrentBatch < 1 || state.CurrentBatch > len(state.Batches) {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("invalid current batch: %d (total batches: %d)", state.CurrentBatch, len(state.Batches)),
		}
	}

	// Get the current batch (convert from 1-indexed to 0-indexed)
	batch := state.Batches[state.CurrentBatch-1]

	// Convert workflow.CodeFile to review request format
	reviewFiles := make([]CodeFileForReview, len(batch.Files))
	for i, file := range batch.Files {
		reviewFiles[i] = CodeFileForReview{
			FilePath:  file.FilePath,
			Content:   file.Content,
			Language:  file.Language,
			LineCount: file.LineCount,
			SizeBytes: file.SizeBytes,
			Checksum:  file.Checksum,
		}
	}

	// Build the review request (same for all providers)
	req := ReviewRequest{
		Files:       reviewFiles,
		FocusAreas:  []string{"security", "performance", "style", "best-practice"},
		Language:    "go",
		BatchNumber: batch.BatchNumber,
	}

	// Call all providers concurrently
	type providerResult struct {
		providerName string
		review       Review
		err          error
	}

	results := make(chan providerResult, len(n.providers))
	var wg sync.WaitGroup

	for _, provider := range n.providers {
		wg.Add(1)
		go func(p CodeReviewer) {
			defer wg.Done()

			// Call provider with timing
			start := time.Now()
			resp, err := p.ReviewBatch(ctx, req)
			duration := time.Since(start)

			// Build review result
			review := Review{
				ProviderName: p.Name(),
				BatchNumber:  batch.BatchNumber,
				Issues:       make([]ReviewIssue, 0),
				TokensUsed:   0,
				Duration:     duration.Milliseconds(),
				Timestamp:    time.Now().Format(time.RFC3339),
			}

			// Handle provider error
			if err != nil {
				review.Error = err.Error()
				results <- providerResult{
					providerName: p.Name(),
					review:       review,
					err:          err,
				}
				return
			}

			// Success: populate review with response data
			review.TokensUsed = resp.TokensUsed
			for _, issue := range resp.Issues {
				review.Issues = append(review.Issues, ReviewIssue{
					File:         issue.File,
					Line:         issue.Line,
					Severity:     issue.Severity,
					Category:     issue.Category,
					Description:  issue.Description,
					Remediation:  issue.Remediation,
					ProviderName: issue.ProviderName,
					Confidence:   issue.Confidence,
				})
			}

			results <- providerResult{
				providerName: p.Name(),
				review:       review,
				err:          nil,
			}
		}(provider)
	}

	// Wait for all providers to complete
	wg.Wait()
	close(results)

	// Collect results from all providers
	reviews := make(map[string][]Review)
	failedProviders := []string{}
	totalIssuesFound := 0
	var lastError string

	for result := range results {
		// Add review to results
		reviews[result.providerName] = []Review{result.review}

		// Track failures
		if result.err != nil {
			failedProviders = append(failedProviders, result.providerName)
			lastError = fmt.Sprintf("provider %s failed on batch %d: %v", result.providerName, batch.BatchNumber, result.err)
		} else {
			// Count issues from successful providers
			totalIssuesFound += len(result.review.Issues)
		}
	}

	// Build delta state with all provider results
	delta := ReviewState{
		Reviews:            reviews,
		CurrentBatch:       state.CurrentBatch + 1,
		CompletedBatches:   []int{batch.BatchNumber},
		TotalIssuesFound:   totalIssuesFound,
		TotalFilesReviewed: len(batch.Files),
		FailedProviders:    failedProviders,
		LastError:          lastError,
	}

	// Determine next route
	route := n.determineRoute(state.CurrentBatch+1, state.TotalBatches)

	return graph.NodeResult[ReviewState]{
		Delta: delta,
		Route: route,
	}
}

// determineRoute calculates the next routing based on batch progress.
func (n *ReviewBatchNode) determineRoute(nextBatch, totalBatches int) graph.Next {
	if nextBatch <= totalBatches {
		return graph.Goto("review-batch")
	}
	return graph.Goto("consolidate")
}

// ConsolidateNode merges duplicate issues from multiple AI providers into
// a single consolidated list with consensus scoring and provider attribution.
type ConsolidateNode struct {
	// No configuration fields needed (stateless)
}

// Run implements the graph.Node interface for ConsolidateNode.
// It collects all issues from all reviews, deduplicates them using advanced
// fuzzy matching (exact + location-based), and sorts by severity and consensus.
func (n *ConsolidateNode) Run(ctx context.Context, state ReviewState) graph.NodeResult[ReviewState] {
	// Count total number of providers for consensus calculation
	totalProviders := len(state.Reviews)

	// Collect all issues from all reviews into a flat list
	var allIssues []ReviewIssue
	for _, reviews := range state.Reviews {
		for _, review := range reviews {
			allIssues = append(allIssues, review.Issues...)
		}
	}

	// Deduplicate issues using advanced multi-stage fuzzy matching
	consolidated := consolidator.DeduplicateIssues(allIssues, totalProviders)

	// Sort issues by severity and consensus score
	consolidator.SortIssues(consolidated)

	// Build delta state with consolidated issues
	delta := ReviewState{
		ConsolidatedIssues: consolidated,
	}

	return graph.NodeResult[ReviewState]{
		Delta: delta,
		Route: graph.Goto("report"),
	}
}

// DiscoveredFile represents a file discovered by a scanner (avoiding circular dependency).
type DiscoveredFile struct {
	Path     string
	Content  string
	Size     int64
	Checksum string
}

// FileScanner is the interface for discovering files in a codebase.
type FileScanner interface {
	Discover(rootPath string) ([]DiscoveredFile, error)
}

// DiscoverFilesNode discovers all code files in the codebase and creates batches for review.
// It uses the Scanner to find files and the batcher to split them into manageable chunks.
type DiscoverFilesNode struct {
	Scanner   FileScanner
	BatchSize int
}

// Run implements the graph.Node interface for DiscoverFilesNode.
func (n *DiscoverFilesNode) Run(ctx context.Context, state ReviewState) graph.NodeResult[ReviewState] {
	// Validate scanner is configured
	if n.Scanner == nil {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("scanner is nil"),
		}
	}

	// Validate codebase root is set
	if state.CodebaseRoot == "" {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("codebase root is empty"),
		}
	}

	// Validate batch size
	if n.BatchSize <= 0 {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("batch size must be positive, got %d", n.BatchSize),
		}
	}

	// Discover files using the scanner
	discoveredFiles, err := n.Scanner.Discover(state.CodebaseRoot)
	if err != nil {
		return graph.NodeResult[ReviewState]{
			Err: fmt.Errorf("failed to discover files: %w", err),
		}
	}

	// Convert scanner.CodeFile to workflow.CodeFile
	workflowFiles := make([]CodeFile, len(discoveredFiles))
	for i, file := range discoveredFiles {
		// Count lines in content
		lineCount := countLines(file.Content)

		// Detect language from file extension
		language := detectLanguage(file.Path)

		workflowFiles[i] = CodeFile{
			FilePath:  file.Path,
			Content:   file.Content,
			Language:  language,
			LineCount: lineCount,
			SizeBytes: file.Size,
			Checksum:  file.Checksum,
		}
	}

	// Create batches by splitting files into chunks
	batches := createBatches(workflowFiles, n.BatchSize)

	// Build delta state with discovered files and batches
	delta := ReviewState{
		DiscoveredFiles: workflowFiles,
		Batches:         batches,
		TotalBatches:    len(batches),
		CurrentBatch:    1, // Start with first batch
	}

	// Route to review-batch node
	return graph.NodeResult[ReviewState]{
		Delta: delta,
		Route: graph.Goto("review-batch"),
	}
}

// countLines counts the number of lines in the content.
func countLines(content string) int {
	if content == "" {
		return 0
	}
	lines := 1 // Start with 1 for the first line
	for _, ch := range content {
		if ch == '\n' {
			lines++
		}
	}
	return lines
}

// detectLanguage detects the programming language from file extension.
func detectLanguage(filePath string) string {
	ext := filepath.Ext(filePath)
	ext = strings.ToLower(ext)

	// Map common extensions to language names
	languageMap := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".jsx":   "javascript",
		".tsx":   "typescript",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".cxx":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".rs":    "rust",
		".rb":    "ruby",
		".php":   "php",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".cs":    "csharp",
		".kt":    "kotlin",
		".swift": "swift",
		".m":     "objective-c",
		".mm":    "objective-c",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	return "unknown"
}

// createBatches splits files into batches of the specified size.
func createBatches(files []CodeFile, batchSize int) []Batch {
	if len(files) == 0 {
		return []Batch{}
	}

	// Calculate number of batches needed
	numBatches := (len(files) + batchSize - 1) / batchSize
	batches := make([]Batch, 0, numBatches)

	// Split files into batches
	for i := 0; i < len(files); i += batchSize {
		// Calculate end index for this batch
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		// Extract files for this batch
		batchFiles := files[i:end]

		// Calculate total lines for this batch
		totalLines := 0
		for _, file := range batchFiles {
			totalLines += file.LineCount
		}

		// Create batch with 1-indexed batch number
		batch := Batch{
			BatchNumber: len(batches) + 1,
			Files:       batchFiles,
			TotalLines:  totalLines,
			Status:      "pending",
		}

		batches = append(batches, batch)
	}

	return batches
}
