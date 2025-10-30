# Data Model: Multi-LLM Code Review Issues Resolution

**Date**: 2025-10-30
**Feature**: Multi-LLM Code Review Issues Resolution
**Purpose**: Define the structure of issues, fixes, and validation tracking

## Overview

This data model represents the entities involved in tracking, categorizing, and resolving code review issues. Unlike traditional features with database entities, this model captures the logical structure of issues from the review report and the fix tracking mechanism.

## Core Entities

### CodeIssue

Represents a specific problem identified in the code review report.

**Attributes**:
- `IssueID`: Unique identifier (e.g., "001", "002")
- `Title`: Brief description of the issue
- `FilePath`: Absolute path to the file containing the issue
- `LineNumber`: Specific line number where issue exists (may be approximate)
- `Severity`: Enumeration (Critical, High, Medium, Low, Informational)
- `Category`: Enumeration (Security, Performance, BestPractices, Style)
- `ProviderConsensus`: Number of providers that flagged this issue (1/3, 2/3, 3/3)
- `Providers`: List of providers that identified the issue (anthropic, google, openai)
- `Description`: Detailed explanation of the problem
- `Remediation`: Recommended fix approach
- `IsTestFixture`: Boolean indicating if issue is in test fixture code

**Relationships**:
- One CodeIssue has one IssueCategory
- One CodeIssue has one SeverityLevel
- One CodeIssue may have one IssueFix (once fixed)

**State Transitions**:
```
Identified (from review report)
    ↓
Triaged (assigned to fix batch, or marked as intentional)
    ↓
InProgress (developer actively fixing)
    ↓
Fixed (code changed, tests pass locally)
    ↓
Validated (peer review approved, CI passed)
    ↓
Closed (merged to main branch)
```

**Validation Rules**:
- FilePath must exist in the repository
- LineNumber must be positive integer
- Severity must be valid enum value
- ProviderConsensus must be 1, 2, or 3
- IsTestFixture=true requires manual triage decision

**Example Instance**:
```
IssueID: "001"
Title: "Division by zero in CalculateValue1"
FilePath: "/Users/dshills/Development/projects/langgraph-go/examples/multi-llm-review/testdata/fixtures/medium/pkg3/util_1.go"
LineNumber: 14
Severity: Critical
Category: Security
ProviderConsensus: 1 (Google only)
Providers: ["google"]
Description: "The CalculateValue1 function performs division without checking if the divisor 'y' is zero..."
Remediation: "Add a check to ensure 'y' is not zero before performing the division..."
IsTestFixture: true
CurrentState: Triaged
TriageDecision: "DO_NOT_FIX - Intentional test case"
```

### IssueFix

Represents the resolution of a CodeIssue.

**Attributes**:
- `FixID`: Unique identifier
- `IssueID`: Reference to the CodeIssue being fixed
- `FixType`: Enumeration (AddValidation, AddNilCheck, AddErrorHandling, AddDocumentation, FormatCode, Optimize, Refactor)
- `CodeChange`: Description of what was changed
- `FilesModified`: List of file paths that were changed
- `TestsAdded`: List of new test cases added (if any)
- `TestsModified`: List of existing test cases modified
- `BreakingChange`: Boolean indicating if this is a breaking API change
- `BackwardCompatible`: Boolean indicating if change maintains compatibility
- `CommitSHA`: Git commit hash once committed
- `ReviewStatus`: Enumeration (Pending, Approved, ChangesRequested)
- `BenchmarkResults`: Performance data if applicable

**Relationships**:
- One IssueFix resolves one CodeIssue
- One IssueFix may affect multiple files

**Validation Rules**:
- Must reference a valid CodeIssue
- BreakingChange=true requires justification and documentation
- TestsAdded must include at least one test if issue was a security or critical bug
- CommitSHA must be valid Git hash once set

**Example Instance**:
```
FixID: "FIX-045"
IssueID: "045"
FixType: AddValidation
CodeChange: "Added zero divisor check with error return in engine.go calculateConcurrencyFactor()"
FilesModified: ["/graph/engine.go"]
TestsAdded: ["TestCalculateConcurrencyFactor_ZeroDivisor"]
TestsModified: []
BreakingChange: false
BackwardCompatible: true
CommitSHA: "" (not yet committed)
ReviewStatus: Pending
```

### FixBatch

Represents a group of related fixes processed together.

**Attributes**:
- `BatchID`: Unique identifier (e.g., "BATCH-P1-CRITICAL")
- `Priority`: Priority level (P1, P2, P3, P4)
- `Description`: What this batch addresses
- `IssueIDs`: List of CodeIssue IDs in this batch
- `StartDate`: When batch work began
- `CompletionDate`: When all fixes in batch were completed
- `Status`: Enumeration (Planned, InProgress, Testing, Complete)
- `TestResults`: Summary of test run results
- `CoverageChange`: Percentage change in code coverage

**Relationships**:
- One FixBatch contains multiple CodeIssues
- One FixBatch produces multiple IssueFixes

**Validation Rules**:
- All issues in batch must have same Priority
- CompletionDate must be after StartDate
- Status=Complete requires TestResults.AllPassed=true

**Example Instance**:
```
BatchID: "BATCH-P1-CRITICAL"
Priority: P1
Description: "Critical security issues in production code (excluding test fixtures)"
IssueIDs: ["045", "076", ...] (35 production code issues)
StartDate: 2025-10-30
CompletionDate: null (in progress)
Status: InProgress
TestResults: null
CoverageChange: null
```

## Enumeration Types

### Severity
- **Critical**: Causes crashes, panics, or security vulnerabilities
- **High**: Causes bugs, data corruption, or significant performance issues
- **Medium**: Violates best practices, impacts maintainability
- **Low**: Minor style issues, documentation gaps
- **Informational**: Suggestions for improvement, not actual problems

### Category
- **Security**: Issues that could cause crashes, data loss, or vulnerabilities
- **Performance**: Inefficiencies, unnecessary allocations, scalability concerns
- **BestPractices**: Deviations from Go idioms, missing error handling
- **Style**: Formatting, naming conventions, code organization

### FixType
- **AddValidation**: Input validation, bounds checking, nil checks
- **AddNilCheck**: Specific nil pointer validation
- **AddErrorHandling**: Error checking, wrapping, propagation
- **AddDocumentation**: Godoc comments, inline documentation
- **FormatCode**: Formatting, imports organization
- **Optimize**: Performance improvements
- **Refactor**: Code restructuring for clarity

### State (for CodeIssue)
- **Identified**: Found in review report
- **Triaged**: Reviewed and assigned to batch or marked intentional
- **InProgress**: Being fixed by developer
- **Fixed**: Code changed locally
- **Validated**: Tests passed, peer reviewed
- **Closed**: Merged to main

### ReviewStatus (for IssueFix)
- **Pending**: Awaiting review
- **Approved**: Reviewed and accepted
- **ChangesRequested**: Needs modifications

## Special Cases

### Test Fixture Issues

Test fixture code in `testdata/fixtures/` directories requires special handling:

**Decision Model**:
```
IF IsTestFixture == true THEN
    IF ProviderConsensus < 2 THEN
        → Mark as "INTENTIONAL" (do not fix)
    ELSE
        → MANUAL_REVIEW_REQUIRED
        IF issue is security (division-by-zero, nil deref) THEN
            → INTENTIONAL (test fixtures designed to have these)
        ELSE IF issue is formatting/style THEN
            → Consider fixing if it doesn't affect test purpose
        ELSE
            → Case-by-case evaluation
        END IF
    END IF
END IF
```

**Tracking Attributes**:
- `TriageDecision`: Enumeration (FIX, DO_NOT_FIX, MANUAL_REVIEW)
- `TriageRationale`: Text explanation of decision
- `TriageDate`: When decision was made
- `TriagedBy`: Who made the decision

### Breaking Changes

Any fix that modifies a public API requires special tracking:

**Attributes** (added to IssueFix):
- `BreakingChange`: true/false
- `DeprecationStrategy`: Text description of migration path
- `DocumentedIn`: Reference to changelog/migration guide

**Requirements**:
- Must be justified against value of fix
- Must provide migration guide for users
- Should offer deprecated wrapper functions if possible

## Validation Contracts

### Pre-Fix Validation

Before applying any fix:
1. **File Exists**: Verify FilePath still exists at specified location
2. **Line Number Valid**: Confirm LineNumber is within file bounds
3. **Tests Pass**: Run full test suite to establish baseline
4. **No Conflicts**: Ensure no other fixes have modified same code

### Post-Fix Validation

After applying a fix:
1. **Compiles**: `go build ./...` succeeds
2. **Tests Pass**: `go test ./...` passes with no failures
3. **Formatting**: `gofmt -d .` shows no changes needed
4. **Linting**: Relevant linters pass for fixed issue type
5. **Coverage**: Code coverage maintained or improved
6. **Benchmarks**: Performance benchmarks show no regression (for performance fixes: ≥20% improvement)

### Batch Validation

After completing a fix batch:
1. **All Issues Addressed**: Every issue in batch has associated fix or triage decision
2. **All Tests Pass**: Complete test suite passes
3. **Linter Clean**: Zero warnings for fixed categories
4. **Documentation Updated**: Any new error patterns documented
5. **Review Complete**: All fixes have been peer reviewed

## Persistence

While this is not a traditional data storage feature, issue tracking will be managed through:

1. **Review Report**: Source of truth for identified issues (read-only)
2. **Fix Tracking Spreadsheet/Doc**: Track status of each issue (IssueID → Status → FixID)
3. **Git Commits**: Record fixes with structured commit messages
4. **Pull Request**: Aggregate fixes in batches with summary

**Commit Message Format**:
```
fix(category): brief description of fix

Resolves: IssueID-XXX
Severity: Critical/High/Medium/Low
Category: Security/Performance/BestPractices/Style

- Detailed description of what was changed
- Why the change was necessary
- Any edge cases or special considerations

Testing: Description of test validation
```

## Summary

This data model provides a structured approach to tracking and resolving the 800 code review issues. The key entities (CodeIssue, IssueFix, FixBatch) enable systematic processing with clear validation contracts at each stage. Special handling for test fixtures and breaking changes ensures safe, appropriate remediation.
