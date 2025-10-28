# Research: Production Hardening

**Feature**: Production Hardening and Documentation Enhancements
**Date**: 2025-10-28
**Phase**: Phase 0 - Research

## Research Summary

This feature builds on the merged v0.2.0 concurrent execution implementation. Most technical decisions were already made in feature 002. This research focuses on new components: SQLite store and Prometheus metrics.

---

## 1. SQLite Library Choice

**Decision**: Use modernc.org/sqlite (pure Go, CGo-free)

**Rationale**:
- Pure Go implementation (no CGo dependencies)
- Cross-platform compilation without C toolchain
- Better for CI/CD and cross-compilation
- Slightly slower than mattn/go-sqlite3 but acceptable for dev/test use
- Maintains "minimal dependencies" principle

**Alternatives**:
- mattn/go-sqlite3: Requires CGo, faster but adds C dependency
- crawshaw/sqlite: Good but less maintained
- zombiezen.com/go/sqlite: Modern but less proven

---

## 2. Prometheus Metrics Library

**Decision**: Use github.com/prometheus/client_golang (official Prometheus client)

**Rationale**:
- Official Prometheus client for Go
- Industry standard, well-documented
- Integrates with existing Prometheus infrastructure
- Supports all metric types (counter, gauge, histogram)
- Active maintenance and community support

**Alternatives**: No viable alternatives for Prometheus integration

---

## 3. Functional Options Pattern

**Decision**: Add functional options alongside existing Options struct (both supported)

**Rationale**:
- Maintains backward compatibility (existing Options struct still works)
- Provides ergonomic alternative for new code
- Standard Go pattern (used in grpc, aws-sdk-go, etc.)
- Reduces boilerplate for common configurations
- No breaking changes

**Implementation**:
```go
type Option func(*Engine[S])

func WithMaxConcurrent(n int) Option {
    return func(e *Engine[S]) { e.opts.MaxConcurrentNodes = n }
}
```

---

## 4. Cost Tracking Pricing Data

**Decision**: Use static pricing map, updatable via godoc comments

**Rationale**:
- Pricing changes infrequently (quarterly at most)
- Static map is simple and fast
- Users can override with custom cost calculator
- Documented in code comments for easy updates

**Pricing Sources** (as of Oct 2025):
- OpenAI GPT-4o: $2.50/1M input, $10/1M output tokens
- Anthropic Claude Sonnet 4.5: $3/1M input, $15/1M output
- Google Gemini 2.5 Flash: $0.075/1M input, $0.30/1M output

---

## 5. Enhanced Test Structure

**Decision**: Create dedicated test files for contracts (determinism_test.go, exactly_once_test.go)

**Rationale**:
- Separates contract tests from unit tests
- Easier to run specific test suites in CI
- Clear naming shows intent
- Can be referenced from documentation

**Test Files**:
- graph/determinism_test.go: Ordering and replay tests
- graph/exactly_once_test.go: Idempotency and atomic commit tests
- graph/observability_test.go: Metrics and tracing validation

---

## Summary of Decisions

All research complete with concrete decisions:

1. **SQLite**: modernc.org/sqlite (pure Go, CGo-free)
2. **Prometheus**: Official client_golang library
3. **Functional Options**: Add alongside existing Options struct
4. **Cost Tracking**: Static pricing map with override support
5. **Test Structure**: Dedicated contract test files

Ready to proceed to Phase 1 (Design & Contracts).
