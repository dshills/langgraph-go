# LangGraph-Go Documentation

Complete documentation index for LangGraph-Go - Find guides, references, and examples organized by topic.

## 🚀 Getting Started

**New to LangGraph-Go? Start here:**

- [Getting Started Guide](./guides/01-getting-started.md) - Build your first workflow in 5 minutes
- [Installation](#) - `go get github.com/dshills/langgraph-go`
- [Quick Start Example](./guides/01-getting-started.md#quick-start) - Minimal working code
- [Why Go?](./why-go.md) - Comparison with Python LangGraph

## 📚 Core Concepts

**Understand the fundamentals:**

- [Building Workflows](./guides/02-building-workflows.md) - Graph construction patterns and best practices
- [State Management](./guides/03-state-management.md) - Reducers, state design, and advanced patterns
- [Conditional Routing](./guides/05-routing.md) - Dynamic control flow and branching logic
- [Architecture Overview](./architecture.md) - System design and component relationships

## 🎯 Essential Guides

**Key features explained in depth:**

### State & Persistence

- [Checkpoints & Resume](./guides/04-checkpoints.md) - Save and resume workflows from any point
- [State Management](./guides/03-state-management.md) - Advanced reducer patterns and conflict resolution
- [Conflict Resolution Policies](./conflict-policies.md) - ConflictFail, LastWriterWins, and CRDT hooks

### Execution Models

- [Parallel Execution](./guides/06-parallel.md) - Fan-out/fan-in patterns for concurrent node execution
- [Concurrency Model](./concurrency.md) - Worker pools, deterministic ordering, backpressure
- [Deterministic Replay](./replay.md) - Record and replay executions exactly for debugging

### Integration

- [LLM Integration](./guides/07-llm-integration.md) - Multi-provider support (OpenAI, Anthropic, Google, Ollama)
- [Event Tracing](./guides/08-event-tracing.md) - Observability, logging, and monitoring
- [Human-in-the-Loop](./human-in-the-loop.md) - Approval workflows and pause/resume patterns

## 🔧 Advanced Topics

**For production deployments and advanced use cases:**

### Production Readiness

- [Performance Benchmarks](./performance.md) - Throughput, latency, and resource usage analysis
- [Store Guarantees](#) - Exactly-once semantics and atomic commits *(Coming Soon)*
- [Determinism Guarantees](#) - Formal ordering proofs and collision resistance *(Coming Soon)*

### Advanced Patterns

- [Streaming](./streaming.md) - Current status, workarounds, and future roadmap
- [Human-in-the-Loop](./human-in-the-loop.md) - External input integration and approval gates
- [Conflict Policies](./conflict-policies.md) - Handling concurrent state updates

### Operations

- [Migration Guide v0.1→v0.2](./migration-v0.2.md) - Upgrade from v0.1.x to v0.2.x
- [Concurrency Model](./concurrency.md) - Worker configuration and backpressure handling
- [Replay Guide](./replay.md) - Debugging production issues with exact reproduction

## 📖 API Reference

**Complete API documentation:**

- [API Overview](./api/README.md) - Core interfaces and types
- [Engine API](./api/README.md#engine) - Workflow execution
- [Node API](./api/README.md#node) - Node interface and implementations
- [Store API](./api/README.md#store) - Persistence layer
- [Emitter API](./api/README.md#emitter) - Event system

## 💡 Examples

**Working code examples by use case:**

See [`../examples/`](../examples/) for complete, runnable examples:

### Basic Examples

- **[Chatbot](../examples/chatbot/)** - Customer support with intent detection
- **[Checkpoint](../examples/checkpoint/)** - Save and resume workflows
- **[Routing](../examples/routing/)** - Conditional branching and dynamic control flow

### Intermediate Examples

- **[Parallel Execution](../examples/parallel/)** - Fan-out/fan-in with concurrent nodes
- **[LLM Integration](../examples/llm/)** - Multi-provider LLM usage (OpenAI, Anthropic, Google)
- **[Tools](../examples/tools/)** - Tool calling and HTTP integration
- **[Tracing](../examples/tracing/)** - Event emission and observability

### Advanced Examples

- **[Data Pipeline](../examples/data-pipeline/)** - ETL workflow with validation and retries
- **[Research Pipeline](../examples/research-pipeline/)** - Multi-stage research with LLM coordination
- **[Interactive Workflow](../examples/interactive-workflow/)** - User interaction and dynamic inputs
- **[Human-in-the-Loop](../examples/human_in_the_loop/)** - Approval workflows with pause/resume
- **[AI Research Assistant](../examples/ai_research_assistant/)** - Complex multi-agent system
- **[Concurrent Workflow](../examples/concurrent_workflow/)** - High-performance parallel execution
- **[Replay Demo](../examples/replay_demo/)** - Deterministic replay and debugging

### Performance

- **[Benchmarks](../examples/benchmarks/)** - Performance measurement and optimization

**Build examples:**

```bash
make examples
./build/<example-name>
```

## 🆘 Troubleshooting

### Common Issues

**Q: Workflow execution hangs indefinitely**

A: Possible causes and solutions:

1. **Infinite Loop**: Check for nodes that route back to themselves without progress
   ```go
   // ❌ Bad: Infinite loop
   if someCondition {
       return NodeResult[S]{Route: Goto("current-node")}
   }

   // ✅ Good: Progress condition
   if state.Attempts < maxAttempts {
       return NodeResult[S]{
           Delta: S{Attempts: state.Attempts + 1},
           Route: Goto("current-node"),
       }
   }
   ```

2. **MaxSteps Hit**: Increase `Options.MaxSteps` or fix loop logic
   ```go
   opts := graph.Options{
       MaxSteps: 1000, // Increase if legitimate deep workflow
   }
   ```

3. **Deadlock**: Check for circular dependencies in concurrent execution

**Q: Non-deterministic execution results**

A: Ensure determinism:

1. **Use Seeded RNG**: Get RNG from context, not global `rand` package
   ```go
   // ❌ Bad: Non-deterministic
   value := rand.Intn(100)

   // ✅ Good: Deterministic
   rng := ctx.Value(RNGKey).(*rand.Rand)
   value := rng.Intn(100)
   ```

2. **Sort Map Iterations**: Maps iterate in random order in Go
   ```go
   // ❌ Bad: Non-deterministic order
   for key := range myMap {
       process(key)
   }

   // ✅ Good: Deterministic order
   keys := make([]string, 0, len(myMap))
   for key := range myMap {
       keys = append(keys, key)
   }
   sort.Strings(keys)
   for _, key := range keys {
       process(key)
   }
   ```

3. **Avoid time.Now()**: Use state-provided timestamps
   ```go
   // ❌ Bad: Current time (different on replay)
   timestamp := time.Now()

   // ✅ Good: Execution start time from state
   timestamp := state.ExecutionStartTime
   ```

**Q: ErrReplayMismatch during replay**

A: Replay detected divergence from original execution:

1. **Check for Code Changes**: Ensure same code version as original execution
   ```bash
   # Checkout original version
   git checkout <original-commit-sha>
   go build ./...
   ```

2. **Verify Recorded I/O**: Ensure checkpoint contains recorded I/O
   ```go
   checkpoint, _ := store.LoadCheckpointV2(ctx, runID, "")
   log.Printf("Recorded I/Os: %d", len(checkpoint.RecordedIOs))
   ```

3. **Use Lenient Replay**: Temporarily disable strict mode for debugging
   ```go
   opts := graph.Options{
       ReplayMode:   true,
       StrictReplay: false, // Warn but continue
   }
   ```

**Q: High memory usage during concurrent execution**

A: Optimize concurrency settings:

1. **Reduce Worker Pool**: Lower `MaxConcurrentNodes`
   ```go
   opts := graph.Options{
       MaxConcurrentNodes: 10, // Instead of 100
   }
   ```

2. **Limit Queue Depth**: Reduce `QueueDepth` to apply backpressure
   ```go
   opts := graph.Options{
       QueueDepth: 100, // Instead of 1000
   }
   ```

3. **Profile Memory**: Use Go profiling tools
   ```bash
   go test -memprofile=mem.prof -bench=.
   go tool pprof mem.prof
   ```

**Q: Checkpoint save fails with idempotency error**

A: Duplicate step execution detected (expected behavior):

```go
err := store.SaveCheckpointV2(ctx, checkpoint)
if errors.Is(err, ErrIdempotencyViolation) {
    // This step already committed - safe to continue
    log.Info("Step already committed (idempotent)")
} else {
    // Real error - handle appropriately
    return err
}
```

**Q: No events emitted during execution**

A: Check emitter configuration:

1. **Emitter Set**: Ensure emitter is provided to engine
   ```go
   emitter := emit.NewLogEmitter(os.Stdout, false)
   engine := graph.New(reducer, store, emitter, opts)
   ```

2. **Buffered Emitter**: Consume events from channel
   ```go
   emitter := emit.NewBufferedEmitter(1000)
   engine := graph.New(reducer, store, emitter, opts)

   go func() {
       for event := range emitter.Events() {
           log.Printf("Event: %+v", event)
       }
   }()
   ```

3. **Check Event Types**: Verify expected event types are emitted
   ```go
   // Available events: node.start, node.complete, node.error, state.updated
   ```

### Getting Help

**Still stuck? Get support:**

- 📖 [FAQ](./FAQ.md) - Frequently asked questions
- 💬 [GitHub Discussions](https://github.com/dshills/langgraph-go/discussions) - Community Q&A
- 🐛 [GitHub Issues](https://github.com/dshills/langgraph-go/issues) - Bug reports and feature requests
- 📧 [Contributing Guide](../CONTRIBUTING.md) - How to contribute

## 🔍 By Topic

### Graph Construction

- [Building Workflows](./guides/02-building-workflows.md)
- [Conditional Routing](./guides/05-routing.md)
- [Parallel Execution](./guides/06-parallel.md)

### State Management

- [State Guide](./guides/03-state-management.md)
- [Conflict Policies](./conflict-policies.md)
- [Checkpoints](./guides/04-checkpoints.md)

### Execution

- [Concurrency](./concurrency.md)
- [Deterministic Replay](./replay.md)
- [Performance](./performance.md)

### Integration

- [LLM Integration](./guides/07-llm-integration.md)
- [Human-in-the-Loop](./human-in-the-loop.md)
- [Event Tracing](./guides/08-event-tracing.md)

### Production

- [Architecture](./architecture.md)
- [Migration Guide](./migration-v0.2.md)
- [Streaming (Future)](./streaming.md)

## 📝 Contributing

Want to improve the docs? We welcome contributions!

- [Contributing Guide](../CONTRIBUTING.md) - Development workflow
- [Code of Conduct](../CODE_OF_CONDUCT.md) - Community guidelines
- [Security Policy](../SECURITY.md) - Reporting vulnerabilities

## 🔗 External Resources

- [Main Repository](https://github.com/dshills/langgraph-go)
- [Go Package Documentation](https://pkg.go.dev/github.com/dshills/langgraph-go)
- [LangGraph Python (Inspiration)](https://github.com/langchain-ai/langgraph)

## 📊 Documentation Stats

- **8 Core Guides** covering fundamentals
- **6 Advanced Topics** for production use
- **15+ Working Examples** demonstrating patterns
- **Complete API Reference** with all interfaces
- **Troubleshooting Guide** for common issues

## 🗺️ Documentation Roadmap

**Coming Soon:**

- [ ] Store Guarantees documentation (exactly-once semantics)
- [ ] Determinism Guarantees documentation (formal proofs)
- [ ] Advanced Testing Guide
- [ ] Production Deployment Guide
- [ ] Error Handling Best Practices
- [ ] Security Considerations

## 📅 Last Updated

This documentation index was last updated for **LangGraph-Go v0.2.0**.

For the latest updates, see the [CHANGELOG](../CHANGELOG.md).

---

**Need help? Start with:**
1. ✅ [Getting Started Guide](./guides/01-getting-started.md)
2. ✅ [Examples Directory](../examples/)
3. ✅ [FAQ](./FAQ.md)
4. ✅ [GitHub Discussions](https://github.com/dshills/langgraph-go/discussions)
