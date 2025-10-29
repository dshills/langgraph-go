# Research: Multi-LLM Code Review Workflow

**Feature**: 004-multi-llm-code-review
**Date**: 2025-10-29
**Phase**: Phase 0 - Research & Design Decisions

## Research Questions

From Technical Context analysis, the following areas required research:

1. **LLM SDK Integration Patterns** - How to structure adapters for OpenAI, Anthropic, and Google APIs
2. **Issue Deduplication Algorithm** - Best approach for fuzzy matching code review issues across providers
3. **Batch Size Optimization** - How to determine optimal batch sizes for different AI providers
4. **Checkpoint Strategy** - How to structure checkpoint data for fast resumability
5. **Review Prompt Engineering** - Effective prompts for code review across different AI providers

---

## 1. LLM SDK Integration Patterns

### Decision: Adapter Pattern with Unified Interface

**Chosen Approach**:
- Define `CodeReviewer` interface with single method: `ReviewBatch(ctx context.Context, files []CodeFile, focus []string) ([]ReviewIssue, error)`
- Each provider adapter wraps the official SDK client
- Adapters handle provider-specific token limits, rate limiting, and response parsing
- Use existing LangGraph-Go `ChatModel` interface as inspiration but specialize for code review

**Rationale**:
- Isolates provider-specific logic in separate adapters
- Easy to mock for testing
- Allows concurrent execution across providers without tight coupling
- Each adapter can implement provider-specific optimizations (e.g., Claude's longer context window)

**Alternatives Considered**:
- **Use ChatModel directly**: Rejected because code review needs structured output (severity, location, description) that generic chat doesn't provide
- **Single unified adapter**: Rejected because provider APIs differ significantly in authentication, request format, and response structure

**Implementation Notes**:
```go
type CodeReviewer interface {
    ReviewBatch(ctx context.Context, req ReviewRequest) (ReviewResponse, error)
}

type ReviewRequest struct {
    Files []CodeFile
    FocusAreas []string // security, performance, style, best-practices
    Language string
}

type ReviewResponse struct {
    Issues []ReviewIssue
    TokensUsed int
    Duration time.Duration
}
```

---

## 2. Issue Deduplication Algorithm

### Decision: Multi-Stage Fuzzy Matching with Levenshtein Distance

**Chosen Approach**:
1. **Exact Match** (fast path): Same file + same line number (±0) + identical description → 100% duplicate
2. **Location Match**: Same file + line proximity (±5 lines) + description similarity (Levenshtein distance < 30%) → likely duplicate
3. **Semantic Match**: Same file + keyword overlap (60%+) in description → possible duplicate (require manual review flag)

**Rationale**:
- Different providers report same issues with slightly different descriptions
- Line numbers may vary by ±5 due to context differences
- Levenshtein distance works well for text similarity without NLP dependencies
- Multi-stage approach balances accuracy and performance (fast path for obvious duplicates)

**Alternatives Considered**:
- **Exact text match only**: Rejected because providers phrase findings differently ("missing nil check" vs "potential nil pointer dereference")
- **ML-based semantic similarity**: Rejected due to dependency minimalism principle and complexity
- **Hash-based deduplication**: Rejected because requires exact match, misses similar-but-not-identical issues

**Implementation Notes**:
- Use `github.com/agnivade/levenshtein` package for Levenshtein distance (small, pure Go)
- Consensus score: count how many providers flagged the same issue
- High consensus (3/3 providers) elevates severity

**Performance**: O(n²) worst case for n issues, but typically n < 1000, so < 1 second for deduplication phase

---

## 3. Batch Size Optimization

### Decision: Dynamic Batch Sizing Based on Token Limits

**Chosen Approach**:
- **Default**: 20 files per batch
- **Dynamic Adjustment**: Estimate tokens per file (lines * 4 tokens/line) and adjust batch size to stay under provider limits:
  - OpenAI GPT-4: 128K token limit → ~6000 lines → ~60 files @ 100 lines/file
  - Anthropic Claude: 200K token limit → ~9000 lines → ~90 files @ 100 lines/file
  - Google Gemini: 32K token limit → ~4800 lines → ~48 files @ 100 lines/file
- Use smallest limit across all configured providers to ensure all can process the batch

**Rationale**:
- Prevents API errors due to exceeding token limits
- Maximizes throughput by batching as many files as possible
- Adapts to different codebases (small vs large files)
- Conservative estimate (4 tokens/line) includes code + review prompt overhead

**Alternatives Considered**:
- **Fixed batch size**: Rejected because large files would exceed limits
- **One file per batch**: Rejected because too slow (API latency dominates)
- **Provider-specific batches**: Rejected because complicates state management (different batches per provider)

**Implementation Notes**:
```go
func CalculateBatchSize(files []CodeFile, providers []CodeReviewer) int {
    minLimit := minTokenLimit(providers) // e.g., 32K for Gemini
    avgFileSize := averageLines(files)
    tokensPerFile := avgFileSize * 4
    return min(minLimit / tokensPerFile, 50) // cap at 50 files for API latency
}
```

---

## 4. Checkpoint Strategy

### Decision: Batch-Level Checkpointing with Incremental State

**Chosen Approach**:
- Save checkpoint after each batch completes across ALL providers
- Checkpoint contains:
  - `CompletedBatches []int` - which batches are done
  - `Reviews map[string][]ReviewIssue` - accumulated reviews per provider
  - `CurrentBatch int` - next batch to process
  - `TotalBatches int` - for progress calculation
- Use LangGraph-Go's `Store.SaveStep()` to persist state as JSON
- On resume: load latest checkpoint, skip completed batches, continue from `CurrentBatch`

**Rationale**:
- Batch-level granularity balances checkpoint overhead vs. resumability
- All-providers-complete ensures consistent state (don't checkpoint half-reviewed batch)
- JSON serialization enables human inspection of checkpoint data
- Fits LangGraph-Go's step-based checkpoint model

**Alternatives Considered**:
- **File-level checkpointing**: Rejected due to excessive checkpoint overhead (write after every file)
- **Provider-level checkpointing**: Rejected because creates inconsistent state (one provider ahead of others)
- **No checkpointing**: Rejected per spec requirement FR-008 (resumability)

**Implementation Notes**:
- Checkpoint file: `.langgraph-checkpoint-{runID}.json`
- Resume logic: check for checkpoint file, load state, fast-forward to `CurrentBatch`
- Checkpoint size: ~10KB per 100 files (file paths + issue summaries)

---

## 5. Review Prompt Engineering

### Decision: Structured Prompt with Role, Context, and Output Format

**Chosen Approach**:
- **System Prompt** (role): "You are an expert code reviewer analyzing {language} code for {focus_areas}."
- **User Prompt** (task): Include file path, code snippet, and specific instructions
- **Output Format** (structure): Request JSON response with schema: `{issues: [{file, line, severity, category, description, remediation}]}`

**Example Prompt Template**:
```text
System: You are an expert code reviewer specializing in Go. Focus on: security, performance, best practices.

User: Review the following Go files and identify code quality issues. For each issue, provide:
- file: file path
- line: line number (approximate)
- severity: critical | high | medium | low | info
- category: security | performance | style | best-practice
- description: what the issue is
- remediation: how to fix it

Files to review:
---
File: main.go
```go
[code here]
```
---
File: handler.go
```go
[code here]
```
---

Return JSON array of issues.
```

**Rationale**:
- Structured output enables parsing and deduplication
- Focus areas in system prompt guide LLM attention
- JSON format is machine-readable and consistent across providers
- Severity + category enable prioritization and filtering

**Alternatives Considered**:
- **Freeform text output**: Rejected because requires complex NLP parsing to extract structured data
- **One file per request**: Rejected because batching reduces API calls and provides context across files
- **Markdown table output**: Rejected because harder to parse than JSON

**Implementation Notes**:
- Fallback: if provider returns non-JSON, use regex to extract file/line/description and assign default severity
- Prompt length: ~500 tokens (system + instructions) + file content
- Test prompts with mock responses to ensure parsability

---

## Cross-Cutting Decisions

### Error Handling & Retry Strategy
- **Exponential Backoff**: Start with 1s delay, double on each retry, max 32s, max 5 retries
- **Retryable Errors**: Rate limit (429), timeout, network errors
- **Non-Retryable Errors**: Invalid API key (401), quota exceeded (402), invalid request (400)
- **Partial Success**: If one provider fails, continue with others and note failure in report

### Progress Tracking
- **Metrics**: Current batch, total batches, files processed, estimated completion time
- **Estimation**: Based on average batch processing time (rolling window of last 5 batches)
- **Updates**: Emit progress event every 30 seconds via `Emitter` interface

### Configuration Format (YAML)
```yaml
providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    enabled: true
  - name: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-3-opus
    enabled: true
  - name: google
    api_key: ${GOOGLE_API_KEY}
    model: gemini-pro
    enabled: true

review:
  batch_size: 20
  focus_areas: [security, performance, best-practices]
  include_patterns: ["*.go", "*.py", "*.js"]
  exclude_patterns: ["*_test.go", "vendor/", "node_modules/"]

output:
  directory: "./review-results"
  format: markdown
```

---

## Research Outcomes Summary

| Question | Decision | Key Benefit |
|----------|----------|-------------|
| LLM SDK Integration | Adapter pattern with `CodeReviewer` interface | Testable, decoupled, provider-specific optimizations |
| Issue Deduplication | Multi-stage fuzzy matching (Levenshtein distance) | Balances accuracy and performance without heavy dependencies |
| Batch Size | Dynamic based on token limits | Maximizes throughput, prevents API errors |
| Checkpoint Strategy | Batch-level with incremental state | Balances overhead vs. resumability |
| Prompt Engineering | Structured JSON output with role/context | Machine-readable, consistent across providers |

**All NEEDS CLARIFICATION items resolved. Ready for Phase 1: Design & Contracts.**
