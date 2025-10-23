# Contributing to LangGraph-Go

Thank you for your interest in contributing to LangGraph-Go! This document outlines our development workflow and guidelines.

## Development Philosophy

LangGraph-Go follows the principles defined in our [Constitution](./.specify/memory/constitution.md):

1. **Type Safety & Determinism** - All state management uses Go generics
2. **Interface-First Design** - Core abstractions defined as interfaces
3. **Test-Driven Development** - Tests written before implementation (NON-NEGOTIABLE)
4. **Observability & Debugging** - Comprehensive event emission
5. **Dependency Minimalism** - Pure Go core, optional adapters

## Test-Driven Development (TDD) Workflow

**CRITICAL**: All code contributions MUST follow TDD. This is non-negotiable per our constitution.

### TDD Cycle

1. **Red** - Write a failing test
   ```bash
   go test ./graph/... -run TestNodeInterface
   # Should FAIL
   ```

2. **Green** - Write minimal code to pass the test
   ```bash
   # Implement the feature
   go test ./graph/... -run TestNodeInterface
   # Should PASS
   ```

3. **Refactor** - Improve code while keeping tests green
   ```bash
   # Refactor implementation
   go test ./...
   # All tests should still PASS
   ```

## Getting Started

### Prerequisites

- Go 1.25.3 or later
- golangci-lint for linting
- git for version control

### Setup

```bash
# Clone the repository
git clone https://github.com/dshills/langgraph-go.git
cd langgraph-go

# Install dependencies
go mod download

# Verify setup
go test ./...
```

## Making Changes

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Follow the Task Plan

Refer to `specs/001-langgraph-core/tasks.md` for the complete task breakdown. Tasks are organized by user story and include:
- Task ID (e.g., T001)
- Parallel markers [P] for independent tasks
- Story labels [US1], [US2], etc.
- File paths for implementation

### 3. Write Tests First

```go
// graph/node_test.go
package graph

import (
    "context"
    "testing"
)

func TestNodeInterface(t *testing.T) {
    // Test should fail initially
    var node Node[TestState]
    _ = node // Use node in tests
}
```

### 4. Implement the Feature

```go
// graph/node.go
package graph

import "context"

type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

### 5. Verify Tests Pass

```bash
go test ./... -v
```

### 6. Run Linter

```bash
golangci-lint run
```

### 7. Format Code

```bash
go fmt ./...
```

### 8. Pre-Commit Code Review (MANDATORY)

**REQUIRED**: All code changes MUST be reviewed using `mcp-pr` before committing, as mandated by our [Constitution v1.1.0](./.specify/memory/constitution.md).

#### What is mcp-pr?

`mcp-pr` is a code review tool provided by the [Model Context Protocol (MCP)](https://github.com/modelcontextprotocol/servers) servers project. It runs **locally on your machine** with no external data transmission, providing automated analysis for:

- **Security vulnerabilities** (injection attacks, unsafe patterns, auth issues)
- **Bugs** (null pointer risks, race conditions, logic errors)
- **Performance** (inefficient algorithms, memory leaks, bottlenecks)
- **Style** (code consistency, naming conventions, formatting)
- **Best practices** (error handling, type safety, TDD compliance)

#### Installation

1. **Install Claude Desktop** (if not already installed)
   - Download from [claude.ai/download](https://claude.ai/download)
   - The MCP server runs within Claude Desktop's sandboxed environment

2. **Configure MCP server** in Claude Desktop settings:
   ```json
   {
     "mcpServers": {
       "mcp-pr": {
         "command": "npx",
         "args": ["-y", "@modelcontextprotocol/server-pr"]
       }
     }
   }
   ```

3. **Verify installation** by checking available tools in Claude Desktop

**Provenance & Trust**:
- Source: [github.com/modelcontextprotocol/servers](https://github.com/modelcontextprotocol/servers)
- Open source, auditable code
- Runs in Claude Desktop's MCP sandbox (no arbitrary code execution)
- No external network calls or data transmission

#### How to Use

**For unstaged changes** (recommended workflow):
```bash
# After making changes but before staging
# Use Claude Desktop with the command:
/review-precommit

# Or manually trigger:
mcp-pr review_unstaged --repository-path . --review-depth quick
```

**For staged changes** (before final commit):
```bash
# After staging with 'git add'
git add <files>

# Review staged changes:
mcp-pr review_staged --repository-path . --review-depth quick
```

**Review depth options**:
- `quick` - Focus on critical/high severity issues (5-30 seconds)
- `thorough` - Comprehensive analysis including suggestions (30-90 seconds)

#### Addressing Findings

Review findings are categorized by severity:

| Severity | Action Required |
|----------|----------------|
| **critical** | MUST fix before commit |
| **high** | MUST fix or document deviation |
| **medium** | SHOULD fix or document |
| **low** | Consider fixing or ignore with reason |
| **info** | Informational only |

**Fix-commit cycle**:
1. Run pre-commit review
2. Address critical/high findings
3. Re-run review to verify fixes
4. Commit when clean or deviations documented

#### Exception & Appeal Process

**When you can proceed despite findings**:
- False positive (tool misunderstood code)
- Time-sensitive hotfix (security patch, critical bug)
- Technical debt accepted with plan
- Performance optimization that looks "unsafe" but is correct

**How to document deviations**:
1. Add inline comment explaining why the flagged code is correct:
   ```go
   // REVIEW_DEVIATION: Using unsafe pointer for performance in hot path.
   // Benchmarked 10x speedup. Memory safety verified in TestPointerSafety.
   ptr := (*Type)(unsafe.Pointer(&data))
   ```

2. Include deviation rationale in commit message:
   ```
   fix: optimize state serialization (performance critical path)

   REVIEW DEVIATION: mcp-pr flagged unsafe pointer usage as high severity.
   This is intentional - benchmarks show 10x speedup for hot path.
   Safety verified with extensive tests in TestSerializationSafety.
   ```

3. For hotfixes, commit with `[HOTFIX]` prefix and create follow-up task:
   ```bash
   git commit -m "[HOTFIX] patch critical auth bypass (CVE-2024-XXXX)

   mcp-pr review will be done post-commit due to severity.
   Follow-up task: Review and refactor auth module."
   ```

**Escalation**:
- If uncertain, ask in PR review
- Maintainers can approve documented deviations
- Security findings always require explicit acknowledgment

#### Common False Positives

**Generic type constraints**:
```go
// May be flagged as "too complex" but is idiomatic Go 1.25
func Process[S any, constraint interface{ Method() }](s S) {}
```
→ Document: "Generic constraint required by interface design"

**Table-driven tests with many cases**:
```go
// May be flagged as "function too long"
tests := []struct{ /* 50 test cases */ }
```
→ Acceptable: Test comprehensiveness is valued

**Intentional panics in examples**:
```go
// Example code may panic for brevity
func ExampleEngine() {
    engine := New(nil, nil, nil, Options{}) // Will panic - example only
}
```
→ Document: "Example code, not production"

#### Integration with CI

**Local enforcement** (recommended):
- Pre-commit hook runs `mcp-pr review_staged` automatically
- Setup: `cp .git/hooks/pre-commit.sample .git/hooks/pre-commit`
- Customize hook to call mcp-pr

**CI enforcement** (future):
- GitHub Action to verify commits were reviewed
- Audit log of mcp-pr findings and resolutions
- Block merge if critical findings unaddressed

#### Privacy & Security

**Data handling**:
- ✅ Runs entirely locally (no cloud API calls)
- ✅ Code never leaves your machine
- ✅ No telemetry or usage tracking
- ✅ Sandboxed in MCP environment

**What mcp-pr analyzes**:
- Git diffs (staged or unstaged changes)
- File contents in repository
- AST and syntax patterns

**What mcp-pr does NOT access**:
- Network resources
- Files outside repository
- Environment variables or secrets
- System processes or memory

#### Troubleshooting

**"mcp-pr command not found"**:
- Verify Claude Desktop is running
- Check MCP server configuration
- Restart Claude Desktop

**"Review taking too long"**:
- Use `quick` depth instead of `thorough`
- Review specific files instead of all changes
- Large diffs (>1000 lines) may timeout - break into smaller commits

**"Can't connect to MCP server"**:
- Ensure Claude Desktop is open
- Check MCP server logs in Claude Desktop settings
- Reinstall MCP server: `npx -y @modelcontextprotocol/server-pr@latest`

#### Questions?

- **Tool issues**: [MCP Servers Issues](https://github.com/modelcontextprotocol/servers/issues)
- **Policy questions**: Open discussion or ask in PR review
- **False positives**: Document in commit message and proceed

## Code Standards

### Go Idioms

- Use `gofmt` for formatting (enforced in CI)
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use short, descriptive names
- Prefer composition over inheritance
- Return errors explicitly (no panics in library code)

### Generic Types

- State type parameter `[S any]` must be consistent across Engine, Node, Store
- Document generic type requirements in godoc
- Avoid complex type constraints unless necessary

### Error Handling

- All errors MUST be returned explicitly
- Use `fmt.Errorf` with `%w` for error wrapping
- Capture errors in `NodeResult.Err` for node-level failures
- Document error conditions in godoc

### Documentation

- All exported types and functions MUST have godoc comments
- Complex algorithms MUST include inline comments
- Examples MUST be runnable (use example tests: `func ExampleEngine_Run()`)

## Testing Guidelines

### Unit Tests

- Test behavior, not implementation details
- Use table-driven tests when appropriate
- Mock external dependencies
- Aim for high coverage, but prioritize quality over quantity

Example:

```go
func TestReducer(t *testing.T) {
    tests := []struct {
        name     string
        prev     State
        delta    State
        expected State
    }{
        {
            name:     "merge query",
            prev:     State{Query: "old"},
            delta:    State{Query: "new"},
            expected: State{Query: "new"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := reduce(tt.prev, tt.delta)
            if got != tt.expected {
                t.Errorf("got %v, want %v", got, tt.expected)
            }
        })
    }
}
```

### Integration Tests

- Test complete workflows end-to-end
- Use `MemStore` for deterministic testing
- Verify checkpointing and resumption
- Test error scenarios

### Benchmarks

- Add benchmarks for performance-critical code
- Document performance characteristics

## Pull Request Process

### Before Submitting

- [ ] **Pre-commit review completed** (`/review-precommit` or `mcp-pr review_staged`)
- [ ] Critical/high severity findings addressed or documented
- [ ] All tests pass (`go test ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Tests written BEFORE implementation (TDD)
- [ ] Godoc comments added for exported types/functions
- [ ] Examples updated if API changed

### PR Guidelines

1. **Title**: Use conventional commits format
   - `feat: add conditional routing support`
   - `fix: correct state serialization bug`
   - `docs: update architecture documentation`
   - `test: add integration tests for checkpointing`

2. **Description**: Include
   - What changed and why
   - Link to related issues/tasks
   - Testing approach
   - Breaking changes (if any)

3. **Size**: Keep PRs focused and manageable
   - Prefer multiple small PRs over one large PR
   - Each PR should implement complete, testable functionality

### Review Process

- All PRs require review before merging
- Reviews verify:
  - Constitution compliance (TDD, interfaces, type safety)
  - Test coverage and quality
  - Documentation completeness
  - Code style adherence

## Architecture Decisions

### Adding New Features

1. Check if feature aligns with constitution principles
2. Define interfaces before implementations
3. Consider impact on type safety and determinism
4. Document in CLAUDE.md if it affects architecture

### Breaking Changes

- Require MAJOR version bump
- Must be called out explicitly in PR
- Migration guide required
- Consider backward compatibility where possible

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: File an issue with reproduction steps
- **Features**: Propose in an issue before implementing
- **Documentation**: Check CLAUDE.md and specs/SPEC.md

## Code of Conduct

- Be respectful and professional
- Focus on constructive feedback
- Assume good intentions
- Help create a welcoming environment

## Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes to public API
- **MINOR**: New features, backward-compatible
- **PATCH**: Bug fixes, documentation

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to LangGraph-Go! 🚀
