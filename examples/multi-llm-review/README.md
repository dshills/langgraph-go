# Multi-LLM Code Review Workflow

A fully functional LangGraph-Go example application that performs automated code reviews using multiple AI language model providers (OpenAI, Anthropic, Google) in parallel, consolidating their feedback into a single prioritized report.

This example demonstrates real LLM integration with production-ready provider adapters - no mock data required!

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

# Build the binary
go build -o multi-llm-review .

# Set your API keys (will be read from environment variables)
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GOOGLE_API_KEY="AI..."  # Optional, disabled by default

# On first run, a default config will be created at ~/.multi-llm-review/config.yaml
```

### Usage

```bash
# Review a codebase using the built binary
./multi-llm-review /path/to/codebase

# Or run directly with go run
go run . /path/to/codebase

# Use custom config file
./multi-llm-review --config custom-config.yaml /path/to/codebase

# Generate JSON report instead of markdown
./multi-llm-review --format json /path/to/codebase

# Resume from a previous checkpoint
./multi-llm-review --resume /path/to/codebase
```

**Important**: Make sure your API keys are set as environment variables before running:
```bash
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Then run the review
./multi-llm-review /path/to/your/code
```

## Configuration

The configuration file is located at `~/.multi-llm-review/config.yaml` by default. On first run, a default config will be automatically created.

Edit the config to customize:

- **Providers**: Enable/disable AI providers, set models
- **Batch Size**: Adjust for your file sizes (default: 20)
- **Focus Areas**: security, performance, style, best-practices
- **File Patterns**: Include/exclude file patterns
- **Output**: Report format (markdown/json) and directory

**Config location:**
- Default: `~/.multi-llm-review/config.yaml` (created automatically on first run)
- Custom: Use `--config /path/to/config.yaml` to override

Example config:

```yaml
providers:
  - name: openai
    api_key: ${OPENAI_API_KEY}  # Reads from environment
    model: gpt-4
    enabled: true

  - name: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    model: claude-3-5-sonnet-20241022
    enabled: true

review:
  batch_size: 20
  focus_areas: [security, performance, best-practices]
  include_patterns: ["*.go", "*.py"]
  exclude_patterns: ["*_test.go", "vendor/**"]

output:
  directory: ./review-results
  format: markdown
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
