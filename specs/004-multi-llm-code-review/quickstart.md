# Quickstart: Multi-LLM Code Review Workflow

**Feature**: 004-multi-llm-code-review
**Date**: 2025-10-29
**Phase**: Phase 1 - Implementation Guidance

## Overview

This quickstart guide helps developers implement and use the Multi-LLM Code Review workflow. It provides:
- Implementation order and priorities
- Quick start commands
- Testing strategy
- Example usage

---

## Implementation Order

### Phase 1: Foundation (P1 - Must Have)

**Goal**: Get basic workflow running with mock providers

1. **Data Structures** (`workflow/state.go`)
   - Define `ReviewState` struct with all fields from data-model.md
   - Implement reducer function for state merging
   - Write tests for reducer (TDD)

2. **File Scanner** (`scanner/scanner.go`)
   - Implement file discovery with include/exclude patterns
   - Add language detection logic
   - Create batches from file lists
   - **Test First**: Write tests with fixture codebases in `testdata/fixtures/`

3. **Mock Provider** (`providers/mock.go`)
   - Implement `CodeReviewer` interface with configurable responses
   - Return sample issues for testing
   - **Purpose**: Enable end-to-end testing without real API keys

4. **Basic Workflow Nodes** (`workflow/nodes.go`)
   - `DiscoverFilesNode`: Scan codebase, create batches
   - `ReviewBatchNode`: Call provider for single batch
   - `ConsolidateNode`: Basic deduplication (exact match only)
   - `ReportNode`: Generate simple markdown report
   - **Test First**: Write node tests with mock provider

5. **Graph Wiring** (`workflow/graph.go`)
   - Wire nodes together using LangGraph-Go engine
   - Configure in-memory store for checkpoints
   - Add basic event emitters
   - **Test First**: End-to-end test with mock provider

6. **CLI Entry Point** (`main.go`)
   - Parse command-line arguments (codebase path, config file)
   - Load configuration from YAML
   - Initialize engine and run workflow
   - Display progress and final report path

**Milestone**: Can review a small codebase with mock provider and generate report

---

### Phase 2: Real Providers (P1 - Must Have)

**Goal**: Integrate with OpenAI, Anthropic, and Google APIs

7. **Provider Interface** (`providers/provider.go`)
   - Define `CodeReviewer` interface formally
   - Add helper functions (token estimation, error handling)

8. **OpenAI Adapter** (`providers/openai.go`)
   - Wrap `github.com/openai/openai-go` client
   - Implement `ReviewBatch` method
   - Parse JSON responses into `ReviewIssue` structs
   - **Test First**: Integration test with mock HTTP responses

9. **Anthropic Adapter** (`providers/anthropic.go`)
   - Wrap `github.com/anthropics/anthropic-sdk-go` client
   - Similar to OpenAI adapter
   - Handle Claude-specific response format

10. **Google Adapter** (`providers/google.go`)
    - Wrap `github.com/google/generative-ai-go` client
    - Handle Gemini-specific response format

11. **Concurrent Execution** (`workflow/nodes.go` update)
    - Modify `ReviewBatchNode` to call all providers concurrently
    - Use `sync.WaitGroup` or channels
    - Collect results from all providers

**Milestone**: Can review codebase with real AI providers (requires API keys)

---

### Phase 3: Deduplication & Reporting (P1 - Must Have)

**Goal**: Intelligent deduplication and formatted reports

12. **Deduplication Logic** (`consolidator/deduplicator.go`)
    - Implement fuzzy matching (Levenshtein distance)
    - Multi-stage matching (exact, location, semantic)
    - Calculate consensus scores
    - **Test First**: Unit tests with duplicate issues

13. **Prioritization** (`consolidator/prioritizer.go`)
    - Sort by severity, then consensus
    - Group by file
    - **Test First**: Unit tests with mixed severity issues

14. **Report Generator** (`consolidator/reporter.go`)
    - Generate markdown report following contracts/report-format.md
    - Include summary statistics
    - Add provider attribution
    - **Test First**: Test with sample consolidated issues

**Milestone**: Reports show deduplicated issues with consensus scores

---

### Phase 4: Resilience & Configuration (P2 - Should Have)

**Goal**: Handle failures gracefully and support customization

15. **Retry Logic** (`internal/retry.go`)
    - Implement exponential backoff
    - Detect retryable vs permanent errors
    - **Test First**: Mock provider that fails then succeeds

16. **Configuration** (`internal/config.go`)
    - Load from YAML file
    - Support environment variable substitution
    - Validate configuration
    - **Test First**: Test with various config files

17. **Progress Tracking** (`internal/progress.go`)
    - Emit progress events every 30 seconds
    - Calculate estimated completion time
    - **Test First**: Test with mock timer

18. **Error Handling** (`workflow/nodes.go` update)
    - Capture errors in state
    - Continue with partial results if one provider fails
    - Log failures in report

**Milestone**: Workflow handles failures gracefully, supports configuration

---

### Phase 5: Advanced Features (P2 - Nice to Have)

**Goal**: Checkpointing, dynamic batching, performance

19. **Checkpoint Persistence** (`workflow/state.go` update)
    - Save state after each batch using `Store.SaveStep()`
    - Implement resume logic: load checkpoint, skip completed batches

20. **Dynamic Batch Sizing** (`scanner/batcher.go` update)
    - Estimate tokens per file
    - Adjust batch size to stay under provider limits

21. **JSON Report Format** (`consolidator/reporter.go` update)
    - Add JSON output option
    - Follow contracts/report-format.md JSON schema

22. **Performance Optimization**
    - Profile with `go test -bench`
    - Optimize deduplication algorithm if needed
    - Consider caching file content hashes

**Milestone**: Production-ready with resume capability and optimizations

---

## Quick Start Commands

### Prerequisites

```bash
# Install Go 1.21+
go version  # Should be 1.21 or higher

# Clone repository
cd /path/to/langgraph-go

# Navigate to example directory
cd examples/multi-llm-review
```

### Setup

```bash
# Create example config file
cp config.example.yaml config.yaml

# Edit config with your API keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GOOGLE_API_KEY="AI..."

# Initialize Go module
go mod init github.com/yourusername/langgraph-go/examples/multi-llm-review
go mod tidy
```

### Run Tests (TDD Workflow)

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./workflow
go test ./providers
go test ./consolidator

# Run with coverage
go test -cover ./...

# Run with verbose output
go test -v ./...

# Run integration tests only
go test -tags=integration ./...
```

### Build and Run

```bash
# Build binary
go build -o multi-llm-review .

# Run on example codebase
./multi-llm-review --config config.yaml ../..

# Run with specific focus areas
./multi-llm-review --config config.yaml --focus security,performance ../..

# Run with custom output directory
./multi-llm-review --config config.yaml --output ./audit-results ../..

# Resume from checkpoint
./multi-llm-review --config config.yaml --resume ../..
```

---

## Example Configuration

**Minimal** (`config.example.yaml`):
```yaml
providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    enabled: true

review:
  batch_size: 20
  focus_areas: [security, performance, best-practices]
  include_patterns: ["*.go"]
  exclude_patterns: ["*_test.go", "vendor/**"]

output:
  directory: ./review-results
  format: markdown
```

---

## Testing Strategy

### Unit Tests (Write First)

```go
// workflow/state_test.go
func TestReduceWorkflowState(t *testing.T) {
    prev := ReviewState{CurrentBatch: 1, CompletedBatches: []int{}}
    delta := ReviewState{CurrentBatch: 2, CompletedBatches: []int{1}}

    result := ReduceWorkflowState(prev, delta)

    assert.Equal(t, 2, result.CurrentBatch)
    assert.Equal(t, []int{1}, result.CompletedBatches)
}

// scanner/scanner_test.go
func TestDiscoverFiles(t *testing.T) {
    scanner := NewScanner([]string{"*.go"}, []string{"*_test.go"})

    files, err := scanner.Discover("testdata/fixtures/small")

    assert.NoError(t, err)
    assert.Len(t, files, 5)  // Expected 5 Go files (excluding tests)
}

// consolidator/deduplicator_test.go
func TestDeduplicateIssues(t *testing.T) {
    issues := []ReviewIssue{
        {File: "main.go", Line: 42, Description: "missing nil check"},
        {File: "main.go", Line: 44, Description: "potential nil pointer"},  // Similar, ±5 lines
    }

    consolidated := DeduplicateIssues(issues)

    assert.Len(t, consolidated, 1)  // Should merge to 1 issue
    assert.Len(t, consolidated[0].Providers, 2)  // Both providers flagged it
}
```

### Integration Tests

```go
// workflow/graph_test.go
func TestEndToEndWorkflow(t *testing.T) {
    // Use mock provider
    provider := NewMockProvider("test", []ReviewIssue{
        {File: "main.go", Line: 10, Severity: "high", Description: "test issue"},
    })

    // Run workflow
    state, err := RunWorkflow("testdata/fixtures/small", provider)

    assert.NoError(t, err)
    assert.Greater(t, len(state.ConsolidatedIssues), 0)
    assert.FileExists(t, state.ReportPath)
}
```

### Test Fixtures

```
testdata/
├── fixtures/
│   ├── small/          # 10 files for quick tests
│   │   ├── main.go
│   │   ├── handler.go
│   │   └── ...
│   └── medium/         # 100 files for integration tests
│       └── ...
└── expected/
    ├── report_sample.md   # Expected report format
    └── issues_sample.json # Expected consolidated issues
```

---

## Development Workflow

1. **Pick a task** from Implementation Order above
2. **Write tests** for the component (TDD)
3. **Run tests** - they should fail (red)
4. **Implement** the component
5. **Run tests** - they should pass (green)
6. **Refactor** if needed
7. **Commit** with descriptive message
8. **Review** using `mcp-pr review_unstaged` before committing
9. **Repeat** for next task

---

## Common Issues & Solutions

### Issue: API Rate Limits

**Solution**: Reduce `batch_size` or add delays between batches

```yaml
review:
  batch_size: 10  # Smaller batches

advanced:
  request_timeout: 180  # Longer timeout
```

### Issue: Out of Memory

**Solution**: Process fewer files per batch, exclude large files

```yaml
review:
  batch_size: 5
  exclude_patterns: ["*.pb.go", "large_file.go"]
```

### Issue: Deduplication Too Aggressive

**Solution**: Adjust Levenshtein threshold in `consolidator/deduplicator.go`

```go
const levenshteinThreshold = 0.4  // Increase to 40% (was 30%)
```

---

## Next Steps

After implementing Phase 1-5:
1. Add more AI providers (if needed)
2. Support additional languages
3. Add IDE integrations
4. Create GitHub Action for CI/CD
5. Add cost tracking and budgeting

---

## Resources

- **LangGraph-Go Docs**: [CLAUDE.md](../../CLAUDE.md)
- **Specification**: [spec.md](./spec.md)
- **Data Model**: [data-model.md](./data-model.md)
- **Contracts**: [contracts/](./contracts/)
- **Research**: [research.md](./research.md)

---

## Support

For questions or issues:
1. Check [CLAUDE.md](../../CLAUDE.md) for Go idioms and framework patterns
2. Review [research.md](./research.md) for design decisions
3. Consult [data-model.md](./data-model.md) for entity relationships
4. Read [contracts/report-format.md](./contracts/report-format.md) for output specs
