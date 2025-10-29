package workflow

import "github.com/dshills/langgraph-go/examples/multi-llm-review/types"

// Re-export types from types package for backward compatibility
type (
	CodeFile          = types.CodeFile
	Batch             = types.Batch
	ReviewIssue       = types.ReviewIssue
	Review            = types.Review
	ConsolidatedIssue = types.ConsolidatedIssue
	ReviewState       = types.ReviewState
)

// ReduceReviewState merges a delta ReviewState into the previous state using the
// reducer pattern. This function ensures deterministic state merging for checkpoint
// persistence and resumable execution.
func ReduceReviewState(prev, delta ReviewState) ReviewState {
	// Merge file discovery (from DiscoverFilesNode)
	if len(delta.DiscoveredFiles) > 0 {
		prev.DiscoveredFiles = delta.DiscoveredFiles
	}
	if len(delta.Batches) > 0 {
		prev.Batches = delta.Batches
	}
	if delta.TotalBatches > 0 {
		prev.TotalBatches = delta.TotalBatches
	}

	// Merge batch completion
	prev.CompletedBatches = append(prev.CompletedBatches, delta.CompletedBatches...)
	if delta.CurrentBatch > 0 {
		prev.CurrentBatch = delta.CurrentBatch
	}

	// Merge reviews
	for provider, reviews := range delta.Reviews {
		prev.Reviews[provider] = append(prev.Reviews[provider], reviews...)
	}

	// Replace consolidated issues and report (consolidation phase)
	if len(delta.ConsolidatedIssues) > 0 {
		prev.ConsolidatedIssues = delta.ConsolidatedIssues
	}
	if delta.ReportPath != "" {
		prev.ReportPath = delta.ReportPath
	}

	// Update counters and timestamps
	if delta.TotalFilesReviewed > 0 {
		prev.TotalFilesReviewed += delta.TotalFilesReviewed
	}
	if delta.TotalIssuesFound > 0 {
		prev.TotalIssuesFound += delta.TotalIssuesFound
	}
	if delta.EndTime != "" {
		prev.EndTime = delta.EndTime
	}

	// Error tracking
	if delta.LastError != "" {
		prev.LastError = delta.LastError
	}
	prev.FailedProviders = append(prev.FailedProviders, delta.FailedProviders...)

	return prev
}
