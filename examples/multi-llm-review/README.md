# Multi-LLM Code Review Workflow

A LangGraph-Go example application that performs automated code reviews using multiple AI language model providers (OpenAI, Anthropic, Google) in parallel, consolidating their feedback into a single prioritized report.

## Features

- **Batch Processing**: Handles codebases of any size by processing files in configurable batches
- **Multi-Provider**: Concurrent reviews from OpenAI GPT-4, Anthropic Claude, and Google Gemini
- **Smart Deduplication**: Fuzzy matching to merge duplicate issues across providers
- **Consensus Scoring**: Tracks how many providers flagged each issue
- **Prioritized Reports**: Issues grouped by severity (critical → info) with provider attribution
- **Resumable**: Checkpoint-based resumption for long-running reviews
- **Configurable**: YAML-based configuration for providers, focus areas, and file patterns

## Quick Start

### Prerequisites

- Go 1.21 or higher
- API keys for at least one provider:
  - OpenAI: https://platform.openai.com/api-keys
  - Anthropic: https://console.anthropic.com/
  - Google: https://ai.google.dev/

### Installation

```bash
# Clone the repository
git clone https://github.com/dshills/langgraph-go.git
cd langgraph-go/examples/multi-llm-review

# Install dependencies
go mod download

# Copy example config
cp config.example.yaml config.yaml

# Set your API keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GOOGLE_API_KEY="AI..."
```

### Usage

```bash
# Review a codebase
go run . --config config.yaml /path/to/codebase

# Review with specific focus
go run . --config config.yaml --focus security,performance /path/to/codebase

# Resume from checkpoint
go run . --config config.yaml --resume /path/to/codebase

# Generate JSON report
go run . --config config.yaml --format json /path/to/codebase
```

## Configuration

Edit `config.yaml` to customize:

- **Providers**: Enable/disable AI providers, set models
- **Batch Size**: Adjust for your file sizes (default: 20)
- **Focus Areas**: security, performance, style, best-practices
- **File Patterns**: Include/exclude file patterns
- **Output**: Report format (markdown/json) and directory

Example minimal config:

```yaml
providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    enabled: true

review:
  batch_size: 20
  focus_areas: [security, performance]
  include_patterns: ["*.go"]

output:
  directory: ./review-results
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./workflow
go test ./consolidator
```

### Project Structure

```
.
├── main.go                 # CLI entry point
├── workflow/               # LangGraph orchestration
│   ├── state.go           # State management
│   ├── nodes.go           # Workflow nodes
│   └── graph.go           # Graph wiring
├── providers/              # AI provider adapters
│   ├── provider.go        # Interface definition
│   ├── openai.go          # OpenAI adapter
│   ├── anthropic.go       # Anthropic adapter
│   ├── google.go          # Google adapter
│   └── mock.go            # Mock for testing
├── scanner/                # File discovery & batching
├── consolidator/           # Deduplication & reporting
└── internal/              # Configuration, retry, progress

```

## Architecture

This example demonstrates LangGraph-Go capabilities:

1. **State Management**: Typed state with reducer functions
2. **Graph Orchestration**: Multi-node workflow with branching
3. **Checkpointing**: Resumable execution via Store interface
4. **Concurrent Execution**: Fan-out to multiple providers
5. **Error Handling**: Retry logic with exponential backoff
6. **Observability**: Event emission for progress tracking

## Example Output

```markdown
# Code Review Report

**Files Reviewed**: 150
**Total Issues**: 42
**Providers**: OpenAI GPT-4, Anthropic Claude

## Critical Issues

### 1. SQL Injection Vulnerability
**File**: `database/query.go:45`
**Consensus**: 2/2 providers (OpenAI, Anthropic)
**Description**: User input concatenated directly into SQL query
**Remediation**: Use parameterized queries or prepared statements
```

## Performance

- **Throughput**: 150 files/minute (with 3 providers)
- **Scale**: Handles codebases up to 10,000 files
- **Memory**: Scales with batch size, not total files
- **Resume**: 10 seconds to restore from checkpoint

## Troubleshooting

### Rate Limiting

If you hit API rate limits, reduce batch size:

```yaml
review:
  batch_size: 10
```

### Out of Memory

Process fewer files per batch or exclude large files:

```yaml
review:
  batch_size: 5
  exclude_patterns: ["*.pb.go", "vendor/**"]
```

## License

See main repository LICENSE file.

## Related Examples

- [LangGraph Core Examples](../)
- [Checkpoint Example](../checkpoint/)
- [Concurrent Workflow](../concurrent_workflow/)

## Documentation

- [Feature Specification](../../specs/004-multi-llm-code-review/spec.md)
- [Implementation Plan](../../specs/004-multi-llm-code-review/plan.md)
- [Task Breakdown](../../specs/004-multi-llm-code-review/tasks.md)
