# Per-Node Timeout Example

This example demonstrates how to configure per-node timeouts in LangGraph-Go using `NodePolicy.Timeout` and `Options.DefaultNodeTimeout`.

## Timeout Configuration

LangGraph-Go provides three levels of timeout control:

1. **Per-Node Timeout** (Highest Priority)
   - Set via `NodePolicy.Timeout` on individual nodes
   - Overrides `DefaultNodeTimeout` and global timeouts
   - Useful for nodes with specific time requirements

2. **DefaultNodeTimeout** (Fallback)
   - Set via `Options.DefaultNodeTimeout` when creating the engine
   - Used when a node's `Policy().Timeout` is zero
   - Prevents runaway nodes without explicit timeouts

3. **No Timeout** (Both Zero)
   - When both `Policy().Timeout` and `DefaultNodeTimeout` are zero
   - Node can run indefinitely
   - Use cautiously to avoid hanging workflows

## Example Workflow

The example demonstrates three nodes with different timeout configurations:

```
fastNode (200ms explicit timeout)
  ↓
slowNode (uses DefaultNodeTimeout of 100ms)
  ↓
workflow error (slow node times out)
```

### Node 1: Fast Node (Per-Node Timeout)
- **Timeout**: 200ms (explicit via `Policy()`)
- **Work Duration**: 50ms
- **Result**: Completes successfully

```go
func (n fastNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 200 * time.Millisecond,
    }
}
```

### Node 2: Slow Node (DefaultNodeTimeout)
- **Timeout**: 100ms (from `Options.DefaultNodeTimeout`)
- **Work Duration**: Attempts 300ms
- **Result**: Times out after ~100ms

```go
func (n slowNode) Policy() graph.NodePolicy {
    return graph.NodePolicy{
        Timeout: 0, // Use DefaultNodeTimeout
    }
}
```

## Running the Example

```bash
cd examples/node_timeouts
go run main.go
```

### Expected Output

```
=== Per-Node Timeout Example ===

Running workflow with per-node timeouts...

Configuration:
- Fast node: 200ms explicit timeout (completes in 50ms)
- Slow node: Uses DefaultNodeTimeout of 100ms (attempts 300ms work)

=== Results ===
Total execution time: ~150ms

Workflow error (expected): NODE_TIMEOUT: node slow exceeded timeout of 100ms

Execution trace:
  1. Fast node completed in 50ms
  2. Slow node timed out after 100ms (used DefaultNodeTimeout)

=== Timeout Behavior ===
✓ Fast node used per-node timeout (200ms) and completed successfully
✓ Slow node used DefaultNodeTimeout (100ms) and timed out as expected

Key Takeaways:
1. NodePolicy.Timeout overrides DefaultNodeTimeout
2. When NodePolicy.Timeout is zero, DefaultNodeTimeout is used
3. Setting both to zero allows unlimited execution time
```

## Timeout Precedence

```
NodePolicy.Timeout (if > 0)
    ↓
Options.DefaultNodeTimeout (if > 0)
    ↓
No timeout (unlimited)
```

## Error Handling

When a node times out:
- Returns `context.DeadlineExceeded` error
- Error wrapped as `EngineError` with code `NODE_TIMEOUT`
- Node's `Run()` method receives cancellation via `ctx.Done()`
- Other nodes are not affected (timeout is per-node)

## Best Practices

1. **Set Reasonable Defaults**
   ```go
   opts := graph.Options{
       DefaultNodeTimeout: 30 * time.Second, // Reasonable default
   }
   ```

2. **Override for Specific Nodes**
   ```go
   func (n slowAPICall) Policy() graph.NodePolicy {
       return graph.NodePolicy{
           Timeout: 2 * time.Minute, // API calls may be slow
       }
   }
   ```

3. **Handle Cancellation Gracefully**
   ```go
   func (n myNode) Run(ctx context.Context, state S) NodeResult[S] {
       select {
       case <-heavyWork():
           return NodeResult[S]{...}
       case <-ctx.Done():
           // Clean up resources
           return NodeResult[S]{Err: ctx.Err()}
       }
   }
   ```

4. **Use Zero Timeout Sparingly**
   - Only for truly unbounded operations
   - Consider adding application-level timeouts
   - Monitor for hanging workflows in production

## Related Documentation

- `graph.NodePolicy` - Per-node configuration
- `graph.Options.DefaultNodeTimeout` - Engine-level default
- `graph/timeout.go` - Timeout enforcement implementation
- `graph/policy_test.go:TestNodeTimeout` - Comprehensive test coverage
