# Lint Fix Summary

## Progress Made

**Starting point**: 764 lint issues
**Current status**: 662 lint issues
**Fixed**: 102 issues (13.4% reduction)

## Successfully Fixed Issues

### 1. Package Comments (revive)
- ✅ Added package comments to all packages
- **Locations fixed**:
  - `graph/` - Core package
  - `graph/emit/` - Event emission package
  - `graph/store/` - Persistence package
  - `graph/model/` - LLM adapters package
  - `graph/tool/` - Tool interfaces package
  - `examples/*/main.go` - All example packages

### 2. Godot Issues (comment periods)
- ✅ Added periods to many comments throughout codebase
- **Impact**: Reduced from 75 to 32 godot issues (43 fixed)

### 3. Error Handling (errcheck)
- ✅ Fixed `defer Close()` calls: Changed to `defer func() { _ = x.Close() }()`
- ✅ Fixed `defer Shutdown()` calls in tests
- **Locations fixed**:
  - `graph/emit/otel_test.go` - All 10 occurrences
  - `examples/sqlite_quickstart/main.go` - Store close
  - Multiple test files

### 4. Error String Capitalization (staticcheck)
- ✅ Fixed capitalized error strings using `fmt.Errorf`
- Changed: `"Google API error"` → `"google API error"`

### 5. Unnecessary Type Conversions (unconvert)
- ✅ Fixed in `graph/model/openai/openai.go`
- Removed: `openaisdk.ChatModel(c.modelName)` → `c.modelName`

### 6. Whitespace Issues
- ✅ Fixed excessive newlines before functions
- ✅ Fixed one leading newline issue

### 7. Unused Code
- ✅ Commented out unused `terminal` field in `graph/engine.go`

## Remaining Issues Breakdown (662 total)

### Critical - Requires Manual Fix

#### 1. errcheck Issues (171 remaining)
**Pattern**: Unchecked `engine.Add()` and `engine.StartAt()` calls in examples

**Fix needed**:
```go
// Before:
engine.Add("node1", NodeFunc[State](func(...) {...}))

// After:
if err := engine.Add("node1", NodeFunc[State](func(...) {...})); err != nil {
    log.Fatalf("failed to add node: %v", err)
}
```

**Files affected**: All example files (`examples/*/main.go`)

**Note**: Automated fixing broke compilation because it incorrectly renamed `ctx` parameters that were actually used. Need manual, careful fixes.

#### 2. revive - Unused Parameters (332 issues)
**Pattern**: Parameters like `ctx`, `event`, `topic` marked as unused

**Fix needed**: Only rename to `_` if truly unused in function body
```go
// Before (if ctx is NOT used):
func processNode(_ context.Context, state State) Result {
    // ctx never referenced
}

// Before (if ctx IS used):
func processNode(ctx context.Context, state State) Result {
    if ctx.Err() != nil { // ctx is used!
        return error
    }
}
```

**WARNING**: Automated tools broke compilation by blindly renaming used parameters!

### Medium Priority

#### 3. gosec - Security Issues (43)
**Common patterns**:
- G104: Duplicate of errcheck (already counted)
- G307: Deferred file close without error check
- Potential integer overflow
- Weak random number generation

**Files affected**: Various throughout codebase

#### 4. goconst - Repeated Strings (19)
**Pattern**: Strings like `"node_start"`, `"run-001"` repeated 3+ times

**Fix needed**: Extract to constants
```go
const (
    EventNodeStart = "node_start"
    EventNodeEnd   = "node_end"
)
```

#### 5. errorlint - Error Handling Patterns (11)
**Pattern**: Non-wrapped error comparisons

**Fix needed**: Use `errors.Is()` instead of `==`
```go
// Before:
if err == ErrNotFound {

// After:
if errors.Is(err, ErrNotFound) {
```

#### 6. godot - Missing Periods (32 remaining)
**Pattern**: Comments without ending punctuation

**Fix needed**: Add periods to comments

### Low Priority

#### 7. gocritic - Code Quality (10)
- Various code quality suggestions
- Can be addressed individually

#### 8. gocyclo - Cyclomatic Complexity (4)
- Functions with complexity > 15
- Consider refactoring into smaller functions

#### 9. godox - TODO/FIXME Comments (9)
- Document or resolve TODO items
- Or add `//nolint:godox` if intentional

#### 10. staticcheck - Static Analysis (9)
- Remaining static analysis issues
- Review individually

#### 11. ineffassign - Ineffective Assignments (4)
- Variables assigned but never used
- Remove or use the assignments

#### 12. prealloc - Preallocate Slices (4)
- Performance: preallocate slices when size known
```go
// Before:
var items []Item
for ... {
    items = append(items, item)
}

// After:
items := make([]Item, 0, knownSize)
for ... {
    items = append(items, item)
}
```

#### 13. unparam - Unused Parameters (1)
- Function parameter never used
- Consider removing or documenting why it's needed

#### 14. unused - Unused Code (11)
**Identified unused items**:
- `graph/observability_test.go`: mockTracer, mockSpan, helper functions
- `graph/replay.go`: recordIO, lookupRecordedIO, verifyReplayHash
- `graph/store/mysql.go`: stepData type
- `graph/replay_test.go`: ReplayTestState

**Options**:
1. Delete if truly unused
2. Add `//nolint:unused` if needed for future
3. Use in tests if intended for testing

## Modified Files

Currently 58 files have been modified with fixes applied.

## Recommended Fix Strategy

### Phase 1: Critical Fixes (Do This First)
1. **Manually fix errcheck issues in examples** (171 issues)
   - Add proper error handling for `engine.Add()` and `engine.StartAt()`
   - Add `log` import where needed
   - Test each example compiles

2. **Review and fix unused parameter warnings** (332 issues)
   - Check each function to see if parameter is actually used
   - Only rename to `_` if genuinely unused
   - Be careful with `ctx` - often used for cancellation checks

### Phase 2: Security & Best Practices
3. **Fix gosec issues** (43 issues)
   - Address security concerns
   - Proper error handling for file operations

4. **Fix errorlint issues** (11 issues)
   - Use proper error wrapping/comparison

### Phase 3: Code Quality
5. **Extract constants** (goconst - 19 issues)
6. **Fix remaining godot** (32 issues)
7. **Address other quality issues** (gocritic, gocyclo, etc.)

### Phase 4: Cleanup
8. **Remove or document unused code** (11 issues)
9. **Fix minor issues** (prealloc, ineffassign, etc.)

## Tools & Commands

```bash
# Run linter
make lint

# Run specific linter
golangci-lint run --disable-all --enable=errcheck

# Format code
go fmt ./...

# Test compilation
go build ./...

# Run tests
go test ./...
```

## Notes & Warnings

1. **DO NOT use automated ctx renaming** - It breaks compilation when ctx is actually used in the function body

2. **Test after each fix** - Especially for errcheck fixes, ensure examples still compile and run

3. **Preserve functionality** - All fixes should maintain existing behavior

4. **Add log imports** - When adding `log.Fatalf()` calls, remember to import `"log"`

5. **Review security issues carefully** - gosec warnings may indicate real security problems

## Next Steps

The most impactful next action is to manually fix the errcheck issues in examples (171 issues). This requires:
1. Reading each example's setup code
2. Adding proper error checks with `log.Fatalf()`
3. Ensuring `log` package is imported
4. Testing that example still compiles

This is tedious but straightforward work that will eliminate 25% of remaining issues.
