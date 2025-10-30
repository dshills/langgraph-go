# API Contracts: Critical Concurrency Bug Fixes

**Feature**: 005-critical-bug-fixes  
**Date**: 2025-10-29

## N/A - No API Contract Changes

**Justification**: This feature consists of internal bug fixes with zero changes to public APIs. All existing method signatures, behaviors, and contracts remain unchanged.

### Public API Compatibility

**Maintained Contracts**:
- `Engine.New()` - Constructor signature unchanged
- `Engine.Run()` - Method signature and return values unchanged
- `Engine.Add()` - Node registration unchanged
- `Engine.Connect()` - Edge definition unchanged
- `Frontier.Enqueue/Dequeue()` - Signatures unchanged (internal behavior improved)

**Behavioral Contracts Strengthened**:
- Error delivery guarantee now 100% reliable (was best-effort with silent drops)
- Deterministic replay now thread-safe (was broken by RNG races)
- OrderKey ordering now guaranteed under all concurrency scenarios
- Completion detection now immediate and race-free

### Internal Contract Changes

These internal contracts are improved but not exposed publicly:

1. **Worker-Frontier Contract**: Workers receive work items in strict OrderKey order
2. **Error-Results Contract**: All errors guaranteed delivery or explicit context cancellation
3. **RNG Contract**: Each worker has isolated RNG instance  
4. **Completion Contract**: Exactly one worker triggers completion signal

---

**Conclusion**: No API contract documentation needed. All changes are internal improvements maintaining full backward compatibility.

Users will experience:
- More reliable execution (no deadlocks)
- Consistent deterministic replay
- Same APIs and usage patterns
