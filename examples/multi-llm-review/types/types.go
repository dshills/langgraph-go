package types

// CodeFile represents a single source code file in the codebase to be reviewed.
type CodeFile struct {
	// FilePath is the absolute or relative path from codebase root
	FilePath string `json:"file_path"`
	// Content is the full file content
	Content string `json:"content"`
	// Language is the detected language (go, python, javascript, etc.)
	Language string `json:"language"`
	// LineCount is the number of lines for batch sizing
	LineCount int `json:"line_count"`
	// SizeBytes is the file size in bytes
	SizeBytes int64 `json:"size_bytes"`
	// Checksum is the SHA-256 hash for change detection
	Checksum string `json:"checksum"`
}

// Batch represents a group of CodeFiles processed together by all AI providers.
type Batch struct {
	// BatchNumber is the 1-indexed batch identifier
	BatchNumber int `json:"batch_number"`
	// Files are the CodeFiles in this batch
	Files []CodeFile `json:"files"`
	// TotalLines is the sum of LineCount for batch sizing
	TotalLines int `json:"total_lines"`
	// Status is the batch status: pending | in_progress | completed | failed
	Status string `json:"status"`
}

// ReviewIssue represents a single code quality issue identified by an AI provider.
type ReviewIssue struct {
	// File is the file path where issue was found
	File string `json:"file"`
	// Line is the line number (0 if file-level issue)
	Line int `json:"line"`
	// Severity is the issue severity: critical | high | medium | low | info
	Severity string `json:"severity"`
	// Category is the issue category: security | performance | style | best-practice
	Category string `json:"category"`
	// Description is what the issue is
	Description string `json:"description"`
	// Remediation is how to fix the issue
	Remediation string `json:"remediation"`
	// ProviderName is which AI provider identified it (openai | anthropic | google)
	ProviderName string `json:"provider"`
	// Confidence is the confidence score from 0.0 to 1.0 (optional)
	Confidence float64 `json:"confidence"`
}

// Review represents feedback from a single AI provider on a batch of files.
type Review struct {
	// ProviderName is the provider name: openai | anthropic | google
	ProviderName string `json:"provider_name"`
	// BatchNumber is which batch was reviewed
	BatchNumber int `json:"batch_number"`
	// Issues are the issues found in this batch
	Issues []ReviewIssue `json:"issues"`
	// TokensUsed is the API tokens consumed
	TokensUsed int `json:"tokens_used"`
	// Duration is the processing time in milliseconds
	Duration int64 `json:"duration_ms"`
	// Timestamp is the ISO 8601 timestamp
	Timestamp string `json:"timestamp"`
	// Error is the error message if review failed (empty on success)
	Error string `json:"error"`
}

// ConsolidatedIssue represents a merged issue identified by one or more AI providers
// (result of deduplication).
type ConsolidatedIssue struct {
	// File is the file path where issue was found
	File string `json:"file"`
	// Line is the line number (0 if file-level issue)
	Line int `json:"line"`
	// Severity is the highest severity from all providers
	Severity string `json:"severity"`
	// Category is the most common category
	Category string `json:"category"`
	// Description is the merged/best description
	Description string `json:"description"`
	// Remediation is the merged/best remediation
	Remediation string `json:"remediation"`
	// Providers is the list of providers that flagged this (e.g., ["openai", "anthropic"])
	Providers []string `json:"providers"`
	// ConsensusScore is the fraction of providers that flagged it (e.g., 0.67 = 2/3)
	ConsensusScore float64 `json:"consensus_score"`
	// IssueID is the unique ID for this consolidated issue (8-char hex)
	IssueID string `json:"issue_id"`
}

// ReviewState represents the current state of the review workflow (managed by LangGraph-Go).
type ReviewState struct {
	// File Discovery
	// CodebaseRoot is the root directory being reviewed
	CodebaseRoot string `json:"codebase_root"`
	// DiscoveredFiles are all files found after filtering
	DiscoveredFiles []CodeFile `json:"discovered_files"`

	// Batch Processing
	// Batches are all batches to process
	Batches []Batch `json:"batches"`
	// CurrentBatch is the next batch to process (1-indexed)
	CurrentBatch int `json:"current_batch"`
	// TotalBatches is the total number of batches
	TotalBatches int `json:"total_batches"`
	// CompletedBatches is the list of completed batch numbers
	CompletedBatches []int `json:"completed_batches"`

	// Reviews maps provider name to list of reviews
	Reviews map[string][]Review `json:"reviews"`

	// Consolidation
	// ConsolidatedIssues contains deduplicated issues
	ConsolidatedIssues []ConsolidatedIssue `json:"consolidated_issues"`
	// ReportPath is the path to the generated markdown report
	ReportPath string `json:"report_path"`

	// Progress & Metadata
	// StartTime is the ISO 8601 start timestamp
	StartTime string `json:"start_time"`
	// EndTime is the ISO 8601 end timestamp (empty until complete)
	EndTime string `json:"end_time"`
	// TotalFilesReviewed is the count of files processed
	TotalFilesReviewed int `json:"total_files_reviewed"`
	// TotalIssuesFound is the count of all issues (before deduplication)
	TotalIssuesFound int `json:"total_issues_found"`

	// Error Handling
	// LastError is the most recent error message (empty if no errors)
	LastError string `json:"last_error"`
	// FailedProviders is the list of providers that failed
	FailedProviders []string `json:"failed_providers"`
}
