# Data Model: Multi-LLM Code Review Workflow

**Feature**: 004-multi-llm-code-review
**Date**: 2025-10-29
**Phase**: Phase 1 - Design & Contracts

## Overview

This document defines the data structures used in the Multi-LLM Code Review workflow. All types are designed for JSON serializability to support checkpoint persistence and are used within the LangGraph-Go state management system.

---

## Core Entities

### CodeFile

Represents a single source code file in the codebase to be reviewed.

```go
type CodeFile struct {
    FilePath  string `json:"file_path"`  // Absolute or relative path from codebase root
    Content   string `json:"content"`    // Full file content
    Language  string `json:"language"`   // Detected language (go, python, javascript, etc.)
    LineCount int    `json:"line_count"` // Number of lines for batch sizing
    SizeBytes int64  `json:"size_bytes"` // File size in bytes
    Checksum  string `json:"checksum"`   // SHA-256 hash for change detection
}
```

**Validation Rules**:
- `FilePath` must not be empty
- `Content` must be valid UTF-8 text
- `Language` must be one of supported languages or "unknown"
- `LineCount` must be positive
- `Checksum` computed via `sha256.Sum256([]byte(Content))`

**State Transitions**: None (immutable once loaded)

---

### Batch

A group of CodeFiles processed together by all AI providers.

```go
type Batch struct {
    BatchNumber int        `json:"batch_number"` // 1-indexed batch identifier
    Files       []CodeFile `json:"files"`        // Files in this batch
    TotalLines  int        `json:"total_lines"`  // Sum of LineCount for batch sizing
    Status      string     `json:"status"`       // pending | in_progress | completed | failed
}
```

**Validation Rules**:
- `BatchNumber` must be positive
- `Files` must not be empty
- `Status` must be one of: "pending", "in_progress", "completed", "failed"
- `TotalLines` should equal `sum(f.LineCount for f in Files)`

**State Transitions**:
```
pending → in_progress (when batch starts)
in_progress → completed (when all providers finish successfully)
in_progress → failed (when retries exhausted)
```

---

### ReviewIssue

A single code quality issue identified by an AI provider.

```go
type ReviewIssue struct {
    File         string   `json:"file"`         // File path where issue found
    Line         int      `json:"line"`         // Line number (0 if file-level)
    Severity     string   `json:"severity"`     // critical | high | medium | low | info
    Category     string   `json:"category"`     // security | performance | style | best-practice
    Description  string   `json:"description"`  // What the issue is
    Remediation  string   `json:"remediation"`  // How to fix it
    ProviderName string   `json:"provider"`     // Which AI provider identified it (openai | anthropic | google)
    Confidence   float64  `json:"confidence"`   // 0.0-1.0 confidence score (optional)
}
```

**Validation Rules**:
- `File` must not be empty
- `Line` must be non-negative (0 means file-level issue)
- `Severity` must be one of: "critical", "high", "medium", "low", "info"
- `Category` must be one of: "security", "performance", "style", "best-practice"
- `Description` must not be empty
- `ProviderName` must be one of: "openai", "anthropic", "google"
- `Confidence` must be in range [0.0, 1.0]

**State Transitions**: None (immutable once created)

---

### Review

Feedback from a single AI provider on a batch of files.

```go
type Review struct {
    ProviderName string        `json:"provider_name"` // openai | anthropic | google
    BatchNumber  int           `json:"batch_number"`  // Which batch was reviewed
    Issues       []ReviewIssue `json:"issues"`        // Issues found in this batch
    TokensUsed   int           `json:"tokens_used"`   // API tokens consumed
    Duration     int64         `json:"duration_ms"`   // Processing time in milliseconds
    Timestamp    string        `json:"timestamp"`     // ISO 8601 timestamp
    Error        string        `json:"error"`         // Error message if review failed (empty on success)
}
```

**Validation Rules**:
- `ProviderName` must not be empty
- `BatchNumber` must be positive
- `TokensUsed` must be non-negative
- `Duration` must be non-negative
- `Timestamp` must be valid ISO 8601 format (RFC3339)
- If `Error` is non-empty, `Issues` should be empty

**State Transitions**: None (immutable once created)

---

### ConsolidatedIssue

A merged issue identified by one or more AI providers (result of deduplication).

```go
type ConsolidatedIssue struct {
    File           string   `json:"file"`            // File path where issue found
    Line           int      `json:"line"`            // Line number (0 if file-level)
    Severity       string   `json:"severity"`        // Highest severity from all providers
    Category       string   `json:"category"`        // Most common category
    Description    string   `json:"description"`     // Merged/best description
    Remediation    string   `json:"remediation"`     // Merged/best remediation
    Providers      []string `json:"providers"`       // List of providers that flagged this (e.g., ["openai", "anthropic"])
    ConsensusScore float64  `json:"consensus_score"` // Fraction of providers that flagged it (e.g., 0.67 = 2/3)
    IssueID        string   `json:"issue_id"`        // Unique ID for this consolidated issue
}
```

**Validation Rules**:
- All fields from `ReviewIssue` apply
- `Providers` must not be empty
- `ConsensusScore` must be in range (0.0, 1.0] (at least one provider flagged it)
- `IssueID` computed as `sha256(file + line + category)[:8]` (8-char hex)

**State Transitions**: None (created during consolidation phase)

**Deduplication Logic**:
- Issues are deduplicated if:
  1. Same `File` AND same `Line` (±5) AND Levenshtein distance of `Description` < 30%
  2. OR same `File` AND keyword overlap > 60% AND same `Category`
- Merged issue takes:
  - Highest `Severity` from all duplicates
  - Most common `Category`
  - Longest/most detailed `Description`
  - All unique `Providers`

---

### WorkflowState

The current state of the review workflow (managed by LangGraph-Go).

```go
type WorkflowState struct {
    // File Discovery
    CodebaseRoot    string      `json:"codebase_root"`    // Root directory being reviewed
    DiscoveredFiles []CodeFile  `json:"discovered_files"` // All files found after filtering

    // Batch Processing
    Batches         []Batch     `json:"batches"`          // All batches to process
    CurrentBatch    int         `json:"current_batch"`    // Next batch to process (1-indexed)
    TotalBatches    int         `json:"total_batches"`    // Total number of batches
    CompletedBatches []int      `json:"completed_batches"` // List of completed batch numbers

    // Reviews
    Reviews         map[string][]Review `json:"reviews"` // Key: provider name, Value: list of reviews

    // Consolidation
    ConsolidatedIssues []ConsolidatedIssue `json:"consolidated_issues"` // Deduplicated issues
    ReportPath         string              `json:"report_path"`         // Path to generated markdown report

    // Progress & Metadata
    StartTime       string `json:"start_time"`       // ISO 8601 start timestamp
    EndTime         string `json:"end_time"`         // ISO 8601 end timestamp (empty until complete)
    TotalFilesReviewed int `json:"total_files_reviewed"` // Count of files processed
    TotalIssuesFound   int `json:"total_issues_found"`   // Count of all issues (before deduplication)

    // Error Handling
    LastError       string `json:"last_error"`       // Most recent error message (empty if no errors)
    FailedProviders []string `json:"failed_providers"` // List of providers that failed
}
```

**Validation Rules**:
- `CodebaseRoot` must be valid directory path
- `CurrentBatch` must be in range [1, TotalBatches + 1] (TotalBatches + 1 means all done)
- `CompletedBatches` must be subset of [1..TotalBatches]
- `Reviews` keys must match configured provider names
- `TotalFilesReviewed` should equal `len(DiscoveredFiles)` when complete

**State Transitions**:
```
Initial → FileDiscovery (load files, create batches)
FileDiscovery → BatchProcessing (process batch, collect reviews)
BatchProcessing → BatchProcessing (repeat for each batch)
BatchProcessing → Consolidation (after all batches complete)
Consolidation → ReportGeneration (deduplicate, prioritize)
ReportGeneration → Complete (write report, emit final events)
```

**Reducer Function**:
```go
func ReduceWorkflowState(prev, delta WorkflowState) WorkflowState {
    // Merge batch completion
    prev.CompletedBatches = append(prev.CompletedBatches, delta.CompletedBatches...)
    prev.CurrentBatch = delta.CurrentBatch

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
    prev.TotalFilesReviewed = delta.TotalFilesReviewed
    prev.TotalIssuesFound = delta.TotalIssuesFound
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
```

---

### Configuration

User-specified settings for the workflow (loaded from YAML file, not part of workflow state).

```go
type Configuration struct {
    Providers []ProviderConfig `json:"providers"`
    Review    ReviewConfig     `json:"review"`
    Output    OutputConfig     `json:"output"`
}

type ProviderConfig struct {
    Name    string `json:"name"`     // openai | anthropic | google
    APIKey  string `json:"api_key"`  // API key (or environment variable reference)
    Model   string `json:"model"`    // Model name (e.g., gpt-4, claude-3-opus, gemini-pro)
    Enabled bool   `json:"enabled"`  // Whether to use this provider
}

type ReviewConfig struct {
    BatchSize       int      `json:"batch_size"`        // Number of files per batch (default: 20)
    FocusAreas      []string `json:"focus_areas"`       // security, performance, style, best-practices
    IncludePatterns []string `json:"include_patterns"`  // File glob patterns to include (e.g., *.go)
    ExcludePatterns []string `json:"exclude_patterns"`  // File glob patterns to exclude (e.g., vendor/)
}

type OutputConfig struct {
    Directory string `json:"directory"` // Output directory for reports and checkpoints
    Format    string `json:"format"`    // markdown | json (default: markdown)
}
```

**Validation Rules**:
- At least one provider must have `Enabled: true`
- `APIKey` must not be empty for enabled providers
- `BatchSize` must be positive (recommended: 10-50)
- `FocusAreas` must be subset of: security, performance, style, best-practices
- `IncludePatterns` and `ExcludePatterns` must be valid glob patterns
- `Directory` must be writable path
- `Format` must be one of: markdown, json

---

## Entity Relationships

```
Configuration
    ↓
WorkflowState (LangGraph-Go managed)
    ├── DiscoveredFiles: []CodeFile
    ├── Batches: []Batch
    │       └── Files: []CodeFile
    ├── Reviews: map[string][]Review
    │       └── Issues: []ReviewIssue
    └── ConsolidatedIssues: []ConsolidatedIssue
            └── (derived from ReviewIssue via deduplication)
```

---

## Serialization Notes

All types are JSON-serializable for:
1. **Checkpoint Persistence**: `WorkflowState` saved to `.langgraph-checkpoint-{runID}.json`
2. **Configuration Loading**: `Configuration` loaded from `config.yaml`
3. **Report Generation**: `ConsolidatedIssues` serialized to JSON or Markdown

**Example Checkpoint JSON**:
```json
{
  "codebase_root": "/Users/dev/myproject",
  "discovered_files": [...],
  "batches": [...],
  "current_batch": 5,
  "total_batches": 10,
  "completed_batches": [1, 2, 3, 4],
  "reviews": {
    "openai": [...],
    "anthropic": [...],
    "google": [...]
  },
  "start_time": "2025-10-29T14:30:00Z",
  "total_files_reviewed": 80
}
```

---

## Summary

- **7 core entities** defined: CodeFile, Batch, ReviewIssue, Review, ConsolidatedIssue, WorkflowState, Configuration
- **All types JSON-serializable** for checkpoint persistence
- **Clear validation rules** for each entity
- **State transitions documented** for Batch and WorkflowState
- **Reducer function defined** for deterministic state merging
- **Entity relationships mapped** for clarity

**Ready for Phase 1: API Contracts**
