# Quickstart: Multi-LLM Code Review Issues Resolution

**Date**: 2025-10-30
**Feature**: 006-fix-review-issues
**Audience**: Developers implementing the fixes

## Overview

This quickstart guide walks you through the process of fixing code review issues identified in the multi-LLM review report. It covers setup, workflow, and common fix patterns.

## Prerequisites

Before starting, ensure you have:

1. **Go toolchain** (1.21+):
   ```bash
   go version  # Should show 1.21 or higher
   ```

2. **Development tools**:
   ```bash
   # Install if not present
   go install golang.org/x/tools/cmd/goimports@latest
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   ```

3. **Repository access**:
   ```bash
   cd /Users/dshills/Development/projects/langgraph-go
   git checkout 006-fix-review-issues
   ```

4. **Review report**:
   - Location: `./review-results/review-report-20251030-081132.md`
   - 800 issues across 184 files

## Initial Setup

### Step 1: Establish Baseline

Run the full test suite to establish a baseline:

```bash
# Run all tests
go test ./... > baseline-tests.txt 2>&1

# Check coverage
go test -cover ./... > baseline-coverage.txt 2>&1

# Run benchmarks (save for performance fixes)
go test -bench=. -benchmem ./... > baseline-bench.txt 2>&1

# Capture any existing linter warnings
golangci-lint run > baseline-lint.txt 2>&1
```

### Step 2: Create Issue Tracking

Create a tracking spreadsheet/document with columns:
- Issue ID (from review report)
- Severity (Critical, High, Medium, Low)
- Category (Security, Performance, BestPractices, Style)
- File Path
- Status (Identified, Triaged, InProgress, Fixed, Validated)
- Fix ID
- Commit SHA
- Notes

## Workflow

### Step 1: Select an Issue

Choose issues by priority (use batching strategy from research.md):

**Batch 1: Critical Security (P1)** - Start here
- Focus on production code in `/graph` and `/examples` main logic
- Skip test fixtures in `testdata/fixtures/` (mark as "Intentional")

**Batch 2: High Priority (P2)** - After P1 complete
- Input validation, error handling, resource management

**Batch 3: Best Practices (P3)** - After P2 complete
- Error handling, documentation, formatting

**Batch 4: Performance (P4)** - After P3 complete
- Benchmark-driven optimizations

### Step 2: Triage the Issue

Determine if the issue should be fixed:

```bash
# View the issue in context
code /path/to/file.go:line_number

# Check if it's a test fixture
echo "/path/to/file.go" | grep -q "testdata/fixtures" && echo "TEST FIXTURE" || echo "PRODUCTION"
```

**Triage Decision Tree**:
```
Is file in testdata/fixtures/?
├─ YES → Is consensus ≥ 2/3?
│   ├─ NO → Mark as "INTENTIONAL", skip fix
│   └─ YES → Manual review required
│       ├─ Division-by-zero or nil deref? → "INTENTIONAL"
│       └─ Formatting/style? → Consider fixing
└─ NO → FIX (production code)
```

### Step 3: Write Edge Case Test (Critical/High Only)

Before fixing, add a test that would fail with the current code:

```go
// Example: Division by zero test
func TestCalculateConcurrencyFactor_ZeroDivisor(t *testing.T) {
    // This test would panic with current code
    _, err := calculateConcurrencyFactor(10, 0)
    if err == nil {
        t.Error("Expected error for zero divisor, got nil")
    }
}
```

Run the test to confirm it fails:
```bash
go test -v -run TestCalculateConcurrencyFactor_ZeroDivisor
# Should fail/panic
```

### Step 4: Implement the Fix

Apply the fix following patterns from research.md:

#### Pattern 1: Add Division-by-Zero Check

```go
// BEFORE
func calculateConcurrencyFactor(max, divisor int) int {
    return max / divisor
}

// AFTER
func calculateConcurrencyFactor(max, divisor int) (int, error) {
    if divisor == 0 {
        return 0, fmt.Errorf("invalid divisor: cannot divide by zero")
    }
    return max / divisor, nil
}
```

#### Pattern 2: Add Nil Check

```go
// BEFORE
func (e *Engine) Process() {
    e.state.Update()
}

// AFTER
func (e *Engine) Process() error {
    if e == nil {
        return fmt.Errorf("cannot process nil Engine")
    }
    if e.state == nil {
        return fmt.Errorf("cannot process Engine with nil state")
    }
    e.state.Update()
    return nil
}
```

#### Pattern 3: Add Error Handling

```go
// BEFORE
func DoSomething() {
    result := riskyOperation()
    processResult(result)
}

// AFTER
func DoSomething() error {
    result, err := riskyOperation()
    if err != nil {
        return fmt.Errorf("risky operation failed: %w", err)
    }
    if err := processResult(result); err != nil {
        return fmt.Errorf("failed to process result: %w", err)
    }
    return nil
}
```

#### Pattern 4: Add Godoc

```go
// BEFORE
func Calculate(x, y int) int {
    return x + y
}

// AFTER
// Calculate returns the sum of x and y.
// It safely handles integer overflow by capping at math.MaxInt.
func Calculate(x, y int) int {
    return x + y
}
```

### Step 5: Run Validations

Execute the validation checklist (see `contracts/fix-validation.md`):

```bash
# 1. Compilation
go build ./...

# 2. Formatting
gofmt -l . | grep "modified/file.go"
# If listed, run: gofmt -w /path/to/file.go

# 3. Imports
goimports -l . | grep "modified/file.go"
# If listed, run: goimports -w /path/to/file.go

# 4. Tests
go test ./...

# 5. Test for edge case passes
go test -v -run TestCalculateConcurrencyFactor_ZeroDivisor

# 6. Coverage check
go test -cover ./modified/package

# 7. Go vet
go vet ./...

# 8. Linting (for relevant categories)
golangci-lint run --enable=errcheck,staticcheck,stylecheck ./modified/package
```

### Step 6: Commit the Fix

Use structured commit message:

```bash
git add /path/to/fixed/file.go /path/to/test/file_test.go

git commit -m "fix(security): add zero divisor check in calculateConcurrencyFactor

Resolves: Issue-045
Severity: Critical
Category: Security

- Added validation to prevent division by zero panic
- Returns error if divisor is zero
- Added test case TestCalculateConcurrencyFactor_ZeroDivisor

Testing: go test ./graph passed with new edge case test"
```

### Step 7: Update Tracking

Update the issue tracking document:
- Status: Fixed
- Fix ID: FIX-045
- Commit SHA: [from git log]
- Notes: Any special considerations

### Step 8: Repeat

Move to the next issue in the same batch.

## Batch Completion

After completing all issues in a batch:

### Step 1: Full Validation

```bash
# Full test suite
go test -v ./... > batch-tests.txt 2>&1

# Race detection
go test -race ./... > batch-race.txt 2>&1

# Coverage report
go test -cover ./... > batch-coverage.txt 2>&1

# Full linting
golangci-lint run > batch-lint.txt 2>&1
```

### Step 2: Compare to Baseline

```bash
# Test comparison
diff baseline-tests.txt batch-tests.txt

# Coverage comparison
diff baseline-coverage.txt batch-coverage.txt

# Lint comparison
diff baseline-lint.txt batch-lint.txt
```

### Step 3: Pre-commit Review

```bash
# Review staged changes
mcp-pr review_staged

# Or review unstaged if not yet staged
mcp-pr review_unstaged
```

### Step 4: Create Pull Request

```bash
# Push branch
git push origin 006-fix-review-issues

# Create PR (if using gh CLI)
gh pr create --title "Fix Batch P1: Critical Security Issues" \
             --body "Resolves 35 critical security issues in production code

## Summary
- Added division-by-zero checks
- Added nil pointer validation
- Added error handling for edge cases

## Testing
- All existing tests pass
- Added 15 edge case tests
- Coverage maintained at 82%

## Issues Resolved
- Issue-045, Issue-076, ... [list all]

See specs/006-fix-review-issues/plan.md for details."
```

## Common Patterns

### When to Add Error Returns

If a function currently returns a value but could fail:

```go
// BEFORE
func Process(input string) Result {
    // Could panic or return invalid result
}

// AFTER
func Process(input string) (Result, error) {
    if input == "" {
        return Result{}, fmt.Errorf("input cannot be empty")
    }
    // ... process
    return result, nil
}
```

**Impact**: This is a breaking change for callers. Consider:
1. Create new function `ProcessE` with error return
2. Mark old function as deprecated
3. Update callers incrementally

### When to Skip a Fix

Skip fixing if:
1. File is in `testdata/fixtures/` AND consensus < 2/3
2. Issue is "Informational" severity and low value
3. Fix would require major refactoring (defer to separate feature)

Document the skip decision:
```
Issue-123: DEFERRED
Reason: Requires refactoring of core Engine struct; defer to v2.0
Tracking: Created issue #456 for future resolution
```

### When Fix Causes Test Failures

If your fix breaks existing tests:

1. **Understand why**: Is the test wrong or is your fix wrong?
2. **If test is wrong**: Fix the test (it was testing incorrect behavior)
3. **If fix is wrong**: Revise your approach
4. **If breaking change unavoidable**: Document and provide migration path

## Tips & Tricks

### Bulk Formatting

Fix all formatting at once (for P3 batch):
```bash
# Format all Go files
gofmt -w .

# Fix all imports
goimports -w .

# Verify no changes needed
gofmt -l . && echo "All formatted!" || echo "Still issues"
```

### Finding Related Issues

Find all issues in the same file:
```bash
grep "/path/to/file.go" review-results/review-report-20251030-081132.md
```

### Testing Performance Fixes

For performance improvements:
```bash
# Before fix
go test -bench=BenchmarkName -benchmem ./package > before.txt

# After fix
go test -bench=BenchmarkName -benchmem ./package > after.txt

# Compare
benchcmp before.txt after.txt
# Or manually compare ns/op and allocs/op
```

## Troubleshooting

### "Tests Pass Locally But Fail in CI"

- Check for race conditions: `go test -race ./...`
- Check for platform-specific code
- Verify CI uses same Go version

### "Linter Fails But I Think Code Is Correct"

- Review linter documentation for specific rule
- If genuinely incorrect, add `//nolint:rulename` with justification
- Get peer review on the exception

### "Fix Breaks API Compatibility"

1. Can you maintain old signature with new function name?
2. Provide deprecated wrapper:
   ```go
   // Process is deprecated. Use ProcessE instead.
   func Process(input string) Result {
       result, _ := ProcessE(input)  // Panic on error for backward compat
       return result
   }

   // ProcessE processes input and returns an error if it fails.
   func ProcessE(input string) (Result, error) {
       // New implementation
   }
   ```

## Getting Help

- **Questions**: Consult `research.md` for issue patterns and strategies
- **Validation**: See `contracts/fix-validation.md` for complete checklist
- **Data Model**: See `data-model.md` for issue structure and relationships
- **Planning**: See `plan.md` for overall approach and constitution compliance

## Success Indicators

You're on track if:
- ✅ All validations pass for each fix
- ✅ Test suite remains passing
- ✅ Coverage is maintained or improved
- ✅ Commit messages are structured and clear
- ✅ Issue tracking is up to date
- ✅ No breaking changes (or documented exceptions)

## Next Steps After Quickstart

1. Start with Batch 1 (Critical Security issues)
2. Follow the workflow for each issue
3. Complete batch validations before moving to next batch
4. Create pull requests for each batch
5. Proceed through P2, P3, P4 systematically

Happy fixing! 🔧
