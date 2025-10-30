# Fix Validation Contract

**Version**: 1.0
**Date**: 2025-10-30
**Purpose**: Define validation criteria that must be met for each fix to be considered complete

## Overview

This contract specifies the validation requirements for code fixes at three levels:
1. **Individual Fix**: Single issue resolution
2. **Batch Completion**: Group of related fixes
3. **Feature Completion**: All fixes merged

## Individual Fix Validation

Every fix must pass ALL of these validations before being committed:

### 1. Code Quality Checks

#### 1.1 Compilation
**Requirement**: Code must compile without errors
**Command**: `go build ./...`
**Pass Criteria**: Exit code 0, no compilation errors
**Failure Action**: Fix compilation errors before proceeding

#### 1.2 Formatting
**Requirement**: Code must follow standard Go formatting
**Command**: `gofmt -l .` (list files needing formatting)
**Pass Criteria**: No files listed (empty output)
**Failure Action**: Run `gofmt -w .` to auto-format

#### 1.3 Imports
**Requirement**: Imports must be organized correctly
**Command**: `goimports -l .`
**Pass Criteria**: No files listed
**Failure Action**: Run `goimports -w .` to fix imports

### 2. Testing Requirements

#### 2.1 Existing Tests
**Requirement**: All existing tests must continue to pass
**Command**: `go test ./...`
**Pass Criteria**: Exit code 0, all tests PASS, zero failures
**Failure Action**: Investigate and fix failing tests; may indicate regression

#### 2.2 Edge Case Tests (Critical/High Severity Only)
**Requirement**: New tests must cover edge cases that were vulnerable
**Examples**:
- Division by zero: Test with zero divisor
- Nil pointer: Test with nil receiver/parameter
- Invalid input: Test with boundary values
**Pass Criteria**: New tests exist and pass
**Failure Action**: Add missing edge case tests

#### 2.3 Test Coverage
**Requirement**: Code coverage must not decrease for modified files
**Command**: `go test -cover ./... | grep -A 1 "modified/package"`
**Pass Criteria**: Coverage % ≥ baseline (measured before fix)
**Failure Action**: Add tests to cover new code paths

### 3. Static Analysis

#### 3.1 Go Vet
**Requirement**: Standard Go static analysis must pass
**Command**: `go vet ./...`
**Pass Criteria**: Exit code 0, no issues reported
**Failure Action**: Address issues identified by go vet

#### 3.2 Security Linting (Security Fixes Only)
**Requirement**: Security-focused linting must pass
**Command**: `gosec ./...`
**Pass Criteria**: No new security issues introduced
**Failure Action**: Address security concerns

#### 3.3 Comprehensive Linting (Best Practices/Style Fixes)
**Requirement**: Linter must pass for relevant categories
**Command**: `golangci-lint run --enable-all --disable=<irrelevant linters>`
**Pass Criteria**: Zero warnings for:
- errcheck (error handling)
- staticcheck (bugs and inefficiencies)
- stylecheck (naming conventions)
- golint (coding style)
**Failure Action**: Fix linter warnings

### 4. Functional Requirements

#### 4.1 Issue Resolution
**Requirement**: The specific issue from the review report must be resolved
**Validation**: Manual code review of the changed location
**Pass Criteria**: Issue no longer present (e.g., division-by-zero check added, nil check present)
**Failure Action**: Revise fix to properly address issue

#### 4.2 No Side Effects
**Requirement**: Fix must not introduce new issues
**Validation**: Review of surrounding code and test results
**Pass Criteria**: No new bugs, panics, or issues introduced
**Failure Action**: Refactor fix to avoid side effects

#### 4.3 API Compatibility (Public APIs Only)
**Requirement**: Public API signatures must remain compatible
**Validation**: Review of function signatures, exported types
**Pass Criteria**:
- No changes to exported function signatures, OR
- Breaking change is documented and justified
**Failure Action**: Revise to maintain compatibility or provide migration path

### 5. Documentation Requirements

#### 5.1 Code Comments (Best Practices/Documentation Fixes)
**Requirement**: Exported APIs must have godoc comments
**Format**:
```go
// FunctionName does X and Y.
// It returns Z or an error if W.
func FunctionName(...) (...) { }
```
**Pass Criteria**: All exported functions, types, methods have godoc
**Failure Action**: Add missing documentation

#### 5.2 Complex Logic Comments
**Requirement**: Non-obvious code must have explanatory comments
**Pass Criteria**: Complex algorithms, edge cases explained
**Failure Action**: Add inline comments

#### 5.3 Commit Message
**Requirement**: Commit must follow structured format
**Format**:
```
fix(category): brief description

Resolves: IssueID-XXX
Severity: Critical/High/Medium/Low
Category: Security/Performance/BestPractices/Style

- What was changed
- Why it was changed
- Edge cases handled

Testing: How fix was validated
```
**Pass Criteria**: Commit message includes all required sections
**Failure Action**: Amend commit with proper message

### 6. Performance Requirements (Performance Fixes Only)

#### 6.1 Benchmark Baseline
**Requirement**: Establish performance baseline before fix
**Command**: `go test -bench=. -benchmem ./package > baseline.txt`
**Pass Criteria**: Benchmark results captured
**Failure Action**: Run benchmark and save results

#### 6.2 Performance Improvement
**Requirement**: Performance must improve or not regress
**Command**: `go test -bench=. -benchmem ./package > after.txt`
**Pass Criteria**:
- Performance fixes: ≥20% improvement in relevant metric
- Non-performance fixes: No regression (within 5% of baseline)
**Failure Action**: Revise optimization or revert if regression

## Batch Completion Validation

A batch is complete when ALL issues in the batch have been addressed AND:

### 1. Batch-Level Testing

#### 1.1 Full Test Suite
**Requirement**: Complete test suite passes with all fixes applied
**Command**: `go test -v ./...`
**Pass Criteria**:
- Exit code 0
- All tests pass
- Zero failures or panics
**Failure Action**: Debug and fix failing tests

#### 1.2 Race Detection
**Requirement**: No race conditions introduced
**Command**: `go test -race ./...`
**Pass Criteria**: No races detected
**Failure Action**: Fix race conditions

#### 1.3 Coverage Report
**Requirement**: Overall coverage maintained or improved
**Command**: `go test -cover ./... | tail -1`
**Pass Criteria**: Coverage ≥ pre-batch baseline
**Failure Action**: Add tests for uncovered code

### 2. Batch-Level Quality

#### 2.1 All Issues Addressed
**Requirement**: Every issue in batch has resolution
**Validation**: Check issue tracking list
**Pass Criteria**:
- Every issue has status: Fixed, Intentional, or Deferred with justification
**Failure Action**: Complete remaining fixes or document deferral

#### 2.2 Linter Clean
**Requirement**: All relevant linters pass for the batch's category
**Command**: `golangci-lint run`
**Pass Criteria**: Zero warnings in modified files for batch category
**Failure Action**: Fix remaining linter warnings

#### 2.3 No Breaking Changes (Unless Justified)
**Requirement**: Public APIs remain compatible
**Validation**: API diff review
**Pass Criteria**:
- Zero breaking changes, OR
- All breaking changes documented with migration guide
**Failure Action**: Provide compatibility layer or documentation

### 3. Documentation Updates

#### 3.1 Changelog Entry
**Requirement**: Changes summarized in changelog
**Location**: `CHANGELOG.md` or batch summary document
**Pass Criteria**:
- Number of issues fixed
- Categories addressed
- Any breaking changes noted
**Failure Action**: Add changelog entry

#### 3.2 Issue Tracking Updated
**Requirement**: Issue status reflects completion
**Validation**: Issue tracking spreadsheet/document
**Pass Criteria**: All issues marked with Fix ID and commit SHA
**Failure Action**: Update tracking

## Feature Completion Validation

The entire feature is complete when ALL batches are done AND:

### 1. Global Quality Gates

#### 1.1 Full Build
**Requirement**: Entire codebase builds cleanly
**Command**: `go build ./...`
**Pass Criteria**: Exit code 0
**Failure Action**: Fix build errors

#### 1.2 Full Test Suite
**Requirement**: All tests pass across entire codebase
**Command**: `go test ./...`
**Pass Criteria**: 100% pass rate
**Failure Action**: Fix failing tests

#### 1.3 All Linters Pass
**Requirement**: Comprehensive linting clean
**Command**: `golangci-lint run --enable-all --disable=<project-excluded>`
**Pass Criteria**: Zero warnings for addressed categories
**Failure Action**: Fix remaining issues

### 2. Metrics Achievement

#### 2.1 Issue Resolution Rate
**Requirement**: Target percentage of issues fixed
**Validation**: Count fixes vs. total issues
**Pass Criteria**: ≥75% of issues fixed (excluding intentional test fixtures)
**Failure Action**: Complete remaining high-value fixes

#### 2.2 Test Coverage
**Requirement**: Coverage maintained or improved
**Validation**: Coverage report comparison
**Pass Criteria**: Coverage ≥80% for core framework, no regression
**Failure Action**: Add tests for uncovered areas

#### 2.3 Performance
**Requirement**: No performance regression
**Validation**: Benchmark comparison
**Pass Criteria**: All benchmarks within 5% of baseline or improved
**Failure Action**: Investigate and fix regressions

### 3. Review and Approval

#### 3.1 Code Review
**Requirement**: All changes reviewed by maintainers
**Process**: Pull request review
**Pass Criteria**: Approved by ≥1 maintainer with no unresolved comments
**Failure Action**: Address reviewer feedback

#### 3.2 Constitution Compliance
**Requirement**: Changes comply with project constitution
**Validation**: Manual review against constitution principles
**Pass Criteria**: All principles satisfied or deviations justified
**Failure Action**: Revise to comply or document justification

#### 3.3 Pre-Commit Review
**Requirement**: Final automated review before merge
**Command**: `mcp-pr review_staged` or `mcp-pr review_unstaged`
**Pass Criteria**: No critical or high-severity issues flagged
**Failure Action**: Address issues identified

## Validation Checklist Template

For each fix, complete this checklist:

```markdown
## Fix Validation: [Issue ID] - [Brief Description]

**Date**: YYYY-MM-DD
**Developer**: Name
**Files Modified**: List of files

### Code Quality
- [ ] ✅ Compiles (`go build ./...`)
- [ ] ✅ Formatted (`gofmt -l .` returns empty)
- [ ] ✅ Imports organized (`goimports -l .` returns empty)

### Testing
- [ ] ✅ Existing tests pass (`go test ./...`)
- [ ] ✅ Edge case tests added (if Critical/High)
- [ ] ✅ Coverage maintained (before: X%, after: Y%)

### Static Analysis
- [ ] ✅ Go vet passes (`go vet ./...`)
- [ ] ✅ Security linting passes (if Security fix)
- [ ] ✅ Comprehensive linting passes (if Best Practices/Style fix)

### Functional
- [ ] ✅ Issue resolved (verified by code review)
- [ ] ✅ No side effects introduced
- [ ] ✅ API compatibility maintained

### Documentation
- [ ] ✅ Godoc added (if exported API)
- [ ] ✅ Complex logic commented
- [ ] ✅ Commit message structured correctly

### Performance (if applicable)
- [ ] ✅ Baseline benchmark captured
- [ ] ✅ Performance improved ≥20% or no regression

**Notes**: Any additional context or special considerations

**Commit SHA**: [hash once committed]
```

## Failure Handling

### When Validation Fails

1. **Stop**: Do not proceed to next fix until current fix passes all validations
2. **Investigate**: Understand root cause of validation failure
3. **Fix**: Address the validation failure
4. **Re-validate**: Run validations again
5. **Iterate**: Repeat until all validations pass

### When to Skip Validations

Never. All validations must pass for every fix. If a validation is not applicable:
- Mark as "N/A" with justification
- Document why the validation doesn't apply
- Get peer confirmation on N/A decision

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-10-30 | Initial validation contract |
