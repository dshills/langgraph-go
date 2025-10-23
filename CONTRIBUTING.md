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
