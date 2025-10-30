# Performance Optimization Plan - US4

## Baseline Benchmarks

### Graph Package (Core Framework)
- **BenchmarkLargeWorkflow**: 46933 ns/op, 121961 B/op, 815 allocs/op
- **BenchmarkSmallWorkflowHighFrequency**: 9435 ns/op, 9397 B/op, 34 allocs/op
- **BenchmarkCheckpointSave**: 327.6 ns/op, 179 B/op, 2 allocs/op
- **BenchmarkCheckpointLoad**: 9.633 ns/op, 0 B/op, 0 allocs/op
- **BenchmarkParallelBranchCoordination**: 19415 ns/op, 10756 B/op, 77 allocs/op
- **BenchmarkStateAllocation**: 8498 ns/op, 7067 B/op, 16 allocs/op

### Examples (multi-llm-review)
- **BenchmarkParseArgs**: 500.3 ns/op, 1073 B/op, 15 allocs/op

## Top 10 Performance Bottlenecks (from Review Report)

### High Priority (Core Framework - graph/)

1. **Issue #546**: deepCopy uses JSON marshal/unmarshal - inefficient and loses type information
   - **File**: graph/engine.go
   - **Impact**: Hot path, used in state management
   - **Target**: 50%+ improvement by using proper deep copy

2. **Issue #580**: Append operations in reducer without pre-allocation
   - **File**: graph/state.go
   - **Impact**: Excessive reallocations for large datasets
   - **Target**: 30%+ improvement with pre-allocation

3. **Issue #526, #273, #741**: ReplayRun sequentially loads checkpoints (O(n) search)
   - **File**: graph/engine.go
   - **Impact**: Very slow for long-running workflows
   - **Target**: 80%+ improvement with better algorithm

4. **Issue #588**: Error mapping uses multiple string.Contains with case conversion
   - **File**: graph/engine.go (error handling)
   - **Impact**: Inefficient error classification
   - **Target**: 40%+ improvement with map lookup

5. **Issue #520**: BufferedEmitter iterates events without batch optimization
   - **File**: graph/emit/ (BufferedEmitter)
   - **Impact**: High event volumes suffer
   - **Target**: 30%+ improvement with single mutex lock for batch

### Medium Priority (Examples - examples/multi-llm-review/)

6. **Issue #734**: ProcessItems builds strings by concatenating single characters (O(n²))
   - **File**: examples/multi-llm-review/scanner/ or consolidator/
   - **Impact**: String building performance
   - **Target**: 50%+ improvement with strings.Builder

7. **Issue #779**: CreateBatches pre-allocates but still uses append
   - **File**: examples/multi-llm-review/scanner/batcher.go
   - **Impact**: Unnecessary allocation
   - **Target**: 20%+ improvement with direct indexing

8. **Issue #739**: detectLanguage allocates languageMap on every call
   - **File**: examples/multi-llm-review/scanner/
   - **Impact**: Repeated allocations
   - **Target**: 90%+ improvement with package-level variable

9. **Issue #510**: Inefficient string concatenation in loop
   - **File**: examples/multi-llm-review/ (string processing)
   - **Impact**: Multiple intermediate string objects
   - **Target**: 40%+ improvement with strings.Builder

10. **Issue #512**: FindDuplicates uses nested loops (O(n²))
    - **File**: examples/multi-llm-review/consolidator/
    - **Impact**: Slow for large inputs
    - **Target**: 80%+ improvement with map-based deduplication

## Implementation Strategy

### Phase 1: Core Framework (graph/) - High Impact
1. Fix deepCopy (JSON → proper copy) - graph/engine.go
2. Fix append pre-allocation - graph/state.go
3. Fix ReplayRun linear search - graph/engine.go
4. Fix error mapping - graph/engine.go
5. Fix BufferedEmitter batching - graph/emit/

### Phase 2: Examples (multi-llm-review/) - Medium Impact
6. Fix string concatenation loops - use strings.Builder
7. Fix CreateBatches allocation - use direct indexing
8. Fix detectLanguage allocation - use package-level map
9. Fix FindDuplicates O(n²) - use map for O(n)

### Phase 3: Store Optimizations (if time permits)
- Optimize MySQL query patterns
- Connection pooling improvements
- Serialization overhead reduction

## Success Criteria

- **Minimum**: 20% improvement on 10 optimizations OR no regression
- **Each optimization must**:
  - Pass all tests
  - Pass race detector
  - Show measurable improvement in benchmarks
  - Be documented with before/after metrics

## Benchmark Comparison Format

For each optimization:
```
Optimization: [Description]
File: [Path]
Before: X ns/op, Y B/op, Z allocs/op
After: A ns/op, B B/op, C allocs/op
Improvement: D% faster, E% less memory, F% fewer allocs
Status: ✅ PASS / ❌ FAIL (regression)
```

## Notes

- All benchmarks run on Apple M4 Pro (darwin/arm64)
- Baseline captured on 2025-10-30
- Test failures exist in replay_test.go (determinism issues) but don't affect performance work
- Focus on hot paths (graph/engine.go and graph/state.go)
