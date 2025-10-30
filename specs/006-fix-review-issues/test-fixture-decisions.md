# Test Fixture Evaluation Criteria

**Feature**: 006-fix-review-issues
**Purpose**: Guidelines for deciding whether to fix issues in test fixture code
**Created**: 2025-10-30

## Overview

Test fixtures in `examples/multi-llm-review/testdata/fixtures/` are specifically designed to contain problematic code for testing the multi-llm-review system's detection capabilities. This document provides criteria for evaluating whether issues in these files should be fixed or left intentional.

## Decision Framework

### Default Rule: DO NOT FIX

**Rationale**: Test fixtures are intentionally problematic. Fixing them defeats their purpose as test data for the review system.

### Decision Matrix

| Issue Type | Test Fixture | Production Code | Rationale |
|------------|--------------|-----------------|-----------|
| Division-by-zero | ❌ DO NOT FIX | ✅ FIX | Intentional test case |
| Nil pointer dereference | ❌ DO NOT FIX | ✅ FIX | Intentional test case |
| Missing error handling | ❌ DO NOT FIX | ✅ FIX | Intentional test case |
| Unchecked error returns | ❌ DO NOT FIX | ✅ FIX | Intentional test case |
| Godoc missing | ❌ SKIP | ✅ FIX | Not needed for fixtures |
| Formatting (gofmt) | 🔍 EVALUATE | ✅ FIX | May fix if doesn't affect test |
| Performance issues | ❌ SKIP | ✅ FIX | Fixtures don't need optimization |
| Race conditions | ❌ DO NOT FIX | ✅ FIX | Intentional test case |
| Resource leaks | ❌ DO NOT FIX | ✅ FIX | Intentional test case |

## Evaluation Process

### Step 1: Identify Location

```bash
# Check if file is in test fixtures
echo "$FILE_PATH" | grep -q "testdata/fixtures"
if [ $? -eq 0 ]; then
    echo "TEST FIXTURE"
else
    echo "PRODUCTION CODE"
fi
```

### Step 2: Check Provider Consensus

- **Consensus ≥ 2/3 (2-3 providers)**: Requires manual review
- **Consensus = 1/3 (1 provider)**: Automatically mark as INTENTIONAL

### Step 3: Apply Decision Matrix

For test fixtures:
1. Security issues (division-by-zero, nil checks) → **INTENTIONAL**
2. Error handling issues → **INTENTIONAL**
3. Best practices violations → **INTENTIONAL**
4. Style/formatting → **EVALUATE** (may fix if safe)
5. Performance issues → **SKIP** (not relevant for fixtures)

### Step 4: Document Decision

Record in issue-tracker.md:
- Issue ID
- File path
- Decision (INTENTIONAL / DO_NOT_FIX / SKIP)
- Reason
- Date

## Test Fixture Directories

**Identified Locations**:
- `examples/multi-llm-review/testdata/fixtures/medium/pkg1/` (40 files)
- `examples/multi-llm-review/testdata/fixtures/medium/pkg2/` (40 files)  
- `examples/multi-llm-review/testdata/fixtures/medium/pkg3/` (30 files)

**Total**: 110 Go files containing intentional issues

## Examples

### Example 1: Division-by-Zero in Fixture

**File**: `examples/multi-llm-review/testdata/fixtures/medium/pkg3/util_1.go:14`

**Issue**: `The CalculateValue1 function performs division without checking if the divisor 'y' is zero.`

**Decision**: **DO NOT FIX** - INTENTIONAL

**Rationale**: This file is a test fixture specifically designed to have division-by-zero for testing the review system's ability to detect this issue. The function name "CalculateValue1" and location in testdata/fixtures confirms this is intentional test data.

### Example 2: Missing Error Handling in Fixture

**File**: `examples/multi-llm-review/testdata/fixtures/medium/pkg2/handler.go`

**Issue**: `Error return value not checked`

**Decision**: **DO NOT FIX** - INTENTIONAL

**Rationale**: Test fixture designed to have missing error handling for review system testing.

### Example 3: Formatting Issue in Fixture

**File**: `examples/multi-llm-review/testdata/fixtures/medium/pkg1/types.go`

**Issue**: `File not formatted with gofmt`

**Decision**: **EVALUATE** → May fix if:
- Formatting doesn't affect the test's purpose
- The test isn't specifically testing formatting detection
- Fixing improves readability without changing behavior

**Likely Decision**: **FIX** if safe, **SKIP** if uncertain

## Impact on Fix Counts

**Estimated Issues in Test Fixtures**: 100-200 of 800 total

**Production Code Issues**: 600-700 (the actual fix target)

**Success Metric Adjustment**: Target ≥75% of *production code issues* (not total issues)

## Special Cases

### Case 1: Test Fixture Used as Example

If a test fixture file is also used as documentation or example code (not just for testing), consider fixing it.

**Indicator**: File has extensive comments, is referenced in documentation

### Case 2: Critical Security Vulnerability

If a test fixture has a vulnerability that could be exploited if copied by users, consider adding a warning comment rather than fixing.

**Example**:
```go
// WARNING: This code intentionally has a security vulnerability
// for testing purposes. DO NOT use in production!
func VulnerableFunction() { ... }
```

### Case 3: Ambiguous Location

If uncertain whether a file is a test fixture or production code:
1. Check file path for "testdata" or "fixtures"
2. Review file contents for test-specific patterns
3. Check git history/commit messages
4. When in doubt, mark as NEEDS_REVIEW and consult team

## Review Process

For any test fixture issue marked for potential fixing:

1. **Document Reasoning**: Why this fixture issue should be fixed
2. **Get Approval**: Discuss with team before modifying test fixtures
3. **Update Tests**: If fixture is fixed, update any tests that depend on it
4. **Validate**: Ensure review system still works correctly after changes

## Conclusion

**Default Action**: DO NOT FIX test fixture issues

**Exceptions**: Only fix if there's a compelling reason and team approval

**Documentation**: Always document the decision in issue-tracker.md

**When Uncertain**: Mark as INTENTIONAL and skip - better to leave test fixtures as-is than risk breaking the review system's test suite.
