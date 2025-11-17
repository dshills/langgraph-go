# Implementation Plan: Production Hardening and Documentation Enhancements

**Branch**: `003-production-hardening` | **Date**: 2025-10-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-production-hardening/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

This feature hardens the v0.2.0 concurrent execution implementation for production use by adding formal documentation of guarantees (determinism, exactly-once), comprehensive observability (Prometheus metrics + enhanced OpenTelemetry), frictionless development experience (SQLite store), enhanced CI testing, and improved API ergonomics (functional options, typed errors, cost tracking). The primary requirements are: (1) document ordering function and atomic commit contracts, (2) add 6 Prometheus metrics and enhanced OTel attributes, (3) implement SQLite store, (4) create CI tests proving determinism/ordering/exactly-once, (5) add functional options pattern, and (6) complete documentation for conflict policies, HITL, and competitive positioning. This is a polish/documentation-heavy feature building on existing 002 implementation.

## Technical Context

**Language/Version**: Go 1.21+ (same as feature 002)
**Primary Dependencies**: Builds on v0.2.0 implementation, adds: SQLite library (modernc.org/sqlite or mattn/go-sqlite3), Prometheus client (github.com/prometheus/client_golang)
**Storage**: Adds SQLite store to existing MemStore and MySQLStore implementations
**Testing**: Go testing framework, enhanced CI test suite for determinism/ordering/exactly-once guarantees
**Target Platform**: Linux/macOS/Windows servers, same as v0.2.0
**Project Type**: Single library project with enhanced documentation and examples
**Performance Goals**: No performance degradation from v0.2.0, SQLite store comparable to MemStore for <10k checkpoints
**Constraints**: Maintain 100% backward compatibility with v0.2.0, no breaking API changes, SQLite single-process only (document limitation)
**Scale/Scope**: Documentation enhancements (~1,500 lines), SQLite store (~500 lines), Prometheus metrics (~300 lines), enhanced tests (~800 lines), API refactoring (~400 lines)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
# [REMOVE IF UNUSED] Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# [REMOVE IF UNUSED] Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# [REMOVE IF UNUSED] Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure: feature modules, UI flows, platform tests]
```

**Structure Decision**: [Document the selected structure and reference the real
directories captured above]

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |

[Gates determined based on constitution file]

### Principle I-V: Type Safety & Determinism ✅
**Status**: PASS - Feature enhances existing v0.2.0 without changing core contracts

### TDD ✅  
**Status**: PASS - Enhanced test suite with contract tests

### GATE RESULT**: ✅ PASS

## Project Structure

This feature enhances existing v0.2.0 codebase:

```
graph/store/
├── sqlite.go          # NEW: SQLite store implementation
├── sqlite_test.go     # NEW: SQLite store tests

graph/
├── determinism_test.go    # NEW: Contract tests for ordering
├── exactly_once_test.go   # NEW: Contract tests for idempotency  
├── observability_test.go  # NEW: Metrics validation tests
├── options.go             # NEW: Functional options pattern
├── metrics.go             # NEW: Prometheus metrics
├── cost.go                # NEW: Cost tracking

docs/
├── store-guarantees.md    # NEW: Store contract documentation
├── why-go.md              # NEW: Go vs Python comparison
├── observability.md       # Enhanced with Prometheus
└── (existing docs enhanced)

examples/
├── prometheus_monitoring/ # NEW: Metrics example
└── (existing examples enhanced)
```

**Structure Decision**: Enhance existing /graph package with new modules. Add SQLite store alongside MemStore and MySQLStore. Documentation primarily in /docs.
