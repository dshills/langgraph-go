# SQLite Quickstart Example

This example demonstrates **zero-configuration persistence** using the SQLite store in LangGraph-Go.

## Why SQLite?

SQLite is perfect for:
- **Local Development**: No database server setup required
- **Prototyping**: Start with SQLite, migrate to MySQL/PostgreSQL later
- **Testing**: In-memory databases (`:memory:`) for fast tests
- **Single-Process Workflows**: Edge computing, CLI tools, local automation
- **CI/CD Pipelines**: Reproducible workflow execution

## What This Example Shows

1. **Zero Setup**: Create a SQLite store with just a file path
2. **Automatic Schema**: Tables are created automatically on first use
3. **State Persistence**: Workflow state survives across runs
4. **ACID Transactions**: Full atomicity guarantees
5. **WAL Mode**: Concurrent reads during workflow execution

## Running the Example

```bash
# Run the example
cd examples/sqlite_quickstart
go run main.go

# Run it again to see that state persists
go run main.go

# Start fresh by deleting the database
rm quickstart.db
go run main.go
```

## Expected Output

```
SQLite Quickstart: Zero-Setup Persistence
==========================================

✓ Created SQLite database at: ./quickstart.db

Starting workflow execution...
─────────────────────────────
→ Node 'start': Initializing workflow
→ Node 'process': Processing (count=1, message="Workflow started")
→ Node 'finish': Completing (count=2, message="Workflow started -> Processed")
─────────────────────────────

✓ Workflow completed!
  Final message: "Workflow started -> Processed -> Complete!"
  Final count: 3
  Done: true

Demonstrating persistence...
─────────────────────────────
✓ Loaded state from database:
  Step: 3
  Message: "Workflow started -> Processed -> Complete!"
  Count: 3
  Done: true

Database file: ./quickstart.db (32768 bytes)

ℹ️  The database persists across runs. Delete it to start fresh:
   rm ./quickstart.db

✅ SQLite Quickstart Complete!
```

## Code Walkthrough

### 1. Create SQLite Store (Zero Configuration)

```go
dbPath := "./quickstart.db"
sqliteStore, err := store.NewSQLiteStore[State](dbPath)
if err != nil {
    log.Fatalf("Failed to create SQLite store: %v", err)
}
defer sqliteStore.Close()
```

That's it! No database server, no schema files, no migrations. Everything is automatic.

### 2. Define Your State Type

```go
type State struct {
    Message string
    Count   int
    Done    bool
}
```

Any JSON-serializable Go struct works. SQLite handles the rest.

### 3. Create Engine with SQLite Store

```go
engine := graph.New(reducer, sqliteStore, emitter, graph.Options{
    MaxSteps: 10,
})
```

The engine uses SQLite for all state persistence automatically.

### 4. Run Your Workflow

```go
finalState, err := engine.Run(ctx, runID, "start", State{})
```

Every step is automatically persisted to SQLite with ACID guarantees.

## Features Enabled by SQLite Store

### Crash Recovery

If your process crashes mid-workflow, simply restart and resume:

```go
// After crash, resume from last checkpoint
finalState, err := engine.Run(ctx, runID, "start", State{})
// Engine automatically continues from last persisted step
```

### State Inspection

Query workflow state at any time:

```go
state, step, err := sqliteStore.LoadLatest(ctx, runID)
fmt.Printf("Workflow is at step %d with state: %+v\n", step, state)
```

### Multiple Runs

Execute multiple workflows concurrently (concurrent reads are supported):

```go
go engine.Run(ctx, "run-001", "start", State{})
go engine.Run(ctx, "run-002", "start", State{})
go engine.Run(ctx, "run-003", "start", State{})
```

Each run has isolated state in the database.

## SQLite Store Configuration

### File-Based Database (Persistent)

```go
store, _ := store.NewSQLiteStore[State]("./my-workflow.db")
// Data persists across runs
```

### In-Memory Database (Testing)

```go
store, _ := store.NewSQLiteStore[State](":memory:")
// Data lost when store closes - perfect for tests
```

### Custom Path

```go
store, _ := store.NewSQLiteStore[State]("/var/lib/myapp/workflows.db")
// Store anywhere on filesystem
```

## Performance Characteristics

- **Write Throughput**: ~1,000 writes/second (single writer)
- **Read Throughput**: Unlimited concurrent reads (WAL mode)
- **Latency**: Sub-millisecond for local disk
- **Database Size**: Supports multi-GB databases efficiently

## When to Use SQLite vs MySQL

### Use SQLite For:
- ✅ Local development and testing
- ✅ Single-process applications
- ✅ Embedded systems and edge computing
- ✅ Prototyping and MVPs
- ✅ < 100K workflow steps per day

### Use MySQL/PostgreSQL For:
- ✅ Distributed systems (multiple workers)
- ✅ High-concurrency workloads (> 100 concurrent writes)
- ✅ Network-attached storage
- ✅ > 1M workflow steps per day
- ✅ Multi-datacenter deployments

## Migration Path

Start with SQLite for development, then migrate to MySQL/PostgreSQL for production:

```go
// Development: SQLite
store, _ := store.NewSQLiteStore[State]("./dev.db")

// Production: MySQL
store, _ := store.NewMySQLStore[State](os.Getenv("MYSQL_DSN"))
```

Both stores implement the same `Store[S]` interface, so no code changes needed!

## Troubleshooting

### Database is Locked

If you see "database is locked" errors:
- SQLite only supports one writer at a time
- Ensure you're not opening multiple connections to the same file
- Consider using MySQL for multi-writer scenarios

### File Permissions

Ensure the directory is writable:
```bash
ls -la ./quickstart.db
# Should show read/write permissions for your user
```

### WAL Files

SQLite creates additional files in WAL mode:
- `quickstart.db` - main database
- `quickstart.db-wal` - write-ahead log
- `quickstart.db-shm` - shared memory

These are normal and managed automatically by SQLite.

## Further Reading

- [Store Guarantees Documentation](../../docs/store-guarantees.md) - Exactly-once semantics
- [SQLite Documentation](https://www.sqlite.org/docs.html) - Official SQLite docs
- [Checkpoint Example](../checkpoint/) - Advanced checkpointing features
- [MySQL Example](../data-pipeline/) - Production-ready store example

## Summary

SQLite provides **zero-configuration persistence** for LangGraph-Go workflows:
- No database server required
- Automatic schema creation
- Full ACID transaction guarantees
- Perfect for development and single-process production workloads

Start with SQLite, migrate to MySQL/PostgreSQL when you need distributed execution!
