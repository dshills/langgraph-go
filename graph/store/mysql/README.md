# MySQL Store Documentation

Production-ready MySQL/MariaDB persistence layer for LangGraph-Go workflows.

## Overview

`MySQLStore` provides persistent state storage for LangGraph-Go workflows using MySQL or MariaDB. It enables:

- **Workflow Resumption**: Resume workflows after process restarts
- **Distributed Execution**: Multiple workers sharing the same workflow state
- **Audit Trails**: Complete execution history in a relational database
- **Production Reliability**: ACID guarantees for state transitions

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/store"
)

func main() {
    // Read DSN from environment (NEVER hardcode credentials!)
    dsn := os.Getenv("MYSQL_DSN")
    if dsn == "" {
        log.Fatal("MYSQL_DSN environment variable not set")
    }

    // Create MySQL store
    st, err := store.NewMySQLStore[MyState](dsn)
    if err != nil {
        log.Fatalf("Failed to create MySQL store: %v", err)
    }
    defer st.Close()

    // Use with Engine
    engine := graph.New(reducer, st, emitter, opts)
    // ... configure and run workflow
}
```

## Database Schema

The MySQL Store automatically creates two tables on first connection:

### `workflow_steps` - Step-by-Step Execution History

```sql
CREATE TABLE workflow_steps (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    run_id VARCHAR(255) NOT NULL,
    step INT NOT NULL,
    node_id VARCHAR(255) NOT NULL,
    state JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_run_id (run_id),
    INDEX idx_run_step (run_id, step),
    UNIQUE KEY unique_run_step (run_id, step)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Columns**:
- `id`: Auto-incrementing primary key
- `run_id`: Unique identifier for a workflow execution
- `step`: Step number in the workflow (0-based)
- `node_id`: The node that executed this step
- `state`: JSON-serialized workflow state
- `created_at`: Timestamp of step creation

**Indexes**:
- `idx_run_id`: Fast lookup by run ID
- `idx_run_step`: Fast lookup by run ID + step
- `unique_run_step`: Prevents duplicate steps

### `workflow_checkpoints` - Named Checkpoints

```sql
CREATE TABLE workflow_checkpoints (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    checkpoint_id VARCHAR(255) NOT NULL UNIQUE,
    state JSON NOT NULL,
    step INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**Columns**:
- `id`: Auto-incrementing primary key
- `checkpoint_id`: Unique checkpoint identifier
- `state`: JSON-serialized state at checkpoint
- `step`: Step number when checkpoint was created
- `created_at`: Timestamp of checkpoint creation
- `updated_at`: Timestamp of last update (for checkpoint overwrites)

**Indexes**:
- `UNIQUE(checkpoint_id)`: Fast lookup and uniqueness constraint

## Installation

### 1. Install MySQL Driver

```bash
go get github.com/go-sql-driver/mysql
```

### 2. Create Database

```sql
CREATE DATABASE workflows CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. Create User (Optional but Recommended)

```sql
CREATE USER 'workflow_user'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT, INSERT, UPDATE, CREATE ON workflows.* TO 'workflow_user'@'%';
FLUSH PRIVILEGES;
```

### 4. Set Environment Variable

```bash
export MYSQL_DSN="workflow_user:secure_password@tcp(localhost:3306)/workflows?parseTime=true"
```

## Configuration

### DSN Format

```
[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
```

### DSN Examples

**Local Development**:
```
user:pass@tcp(localhost:3306)/workflows
```

**Remote Server**:
```
user:pass@tcp(mysql.example.com:3306)/workflows?parseTime=true
```

**AWS RDS**:
```
admin:password@tcp(mydb.us-east-1.rds.amazonaws.com:3306)/workflows?tls=skip-verify
```

**Google Cloud SQL**:
```
user:pass@unix(/cloudsql/project:region:instance)/workflows
```

**Docker Container**:
```
root:password@tcp(mysql-container:3306)/workflows
```

### Recommended DSN Parameters

```
?parseTime=true&loc=UTC&timeout=5s&readTimeout=10s&writeTimeout=10s
```

- `parseTime=true`: Parse DATE/DATETIME into time.Time
- `loc=UTC`: Use UTC timezone
- `timeout=5s`: Connection timeout
- `readTimeout=10s`: Read timeout
- `writeTimeout=10s`: Write timeout

### Connection Pool Configuration

The MySQL Store is pre-configured with sensible defaults:

```go
db.SetMaxOpenConns(25)                  // Maximum open connections
db.SetMaxIdleConns(5)                   // Idle connections to keep
db.SetConnMaxLifetime(5 * time.Minute)  // Max connection lifetime
db.SetConnMaxIdleTime(10 * time.Minute) // Max idle time
```

**For high-concurrency workloads**, increase pool size:

```go
store, _ := store.NewMySQLStore[State](dsn)
// Access underlying *sql.DB (not exposed directly, future API improvement)
```

## Usage Examples

### Basic Workflow Execution

```go
func runWorkflow() error {
    dsn := os.Getenv("MYSQL_DSN")
    st, err := store.NewMySQLStore[WorkflowState](dsn)
    if err != nil {
        return err
    }
    defer st.Close()

    engine := graph.New(reducer, st, emitter, graph.Options{MaxSteps: 100})
    engine.Add("start", startNode)
    engine.Add("process", processNode)
    engine.StartAt("start")

    ctx := context.Background()
    final, err := engine.Run(ctx, "run-001", WorkflowState{})
    return err
}
```

### Resuming a Failed Workflow

```go
func resumeWorkflow(runID string) error {
    dsn := os.Getenv("MYSQL_DSN")
    st, err := store.NewMySQLStore[WorkflowState](dsn)
    if err != nil {
        return err
    }
    defer st.Close()

    // Load latest state
    ctx := context.Background()
    state, step, err := st.LoadLatest(ctx, runID)
    if err == store.ErrNotFound {
        return fmt.Errorf("workflow %s not found", runID)
    }
    if err != nil {
        return err
    }

    log.Printf("Resuming workflow %s from step %d", runID, step)

    engine := graph.New(reducer, st, emitter, graph.Options{MaxSteps: 100})
    // ... configure engine same as original run

    // Run will continue from last saved step
    final, err := engine.Run(ctx, runID, state)
    return err
}
```

### Using Named Checkpoints

```go
func workflowWithCheckpoints() error {
    dsn := os.Getenv("MYSQL_DSN")
    st, err := store.NewMySQLStore[WorkflowState](dsn)
    if err != nil {
        return err
    }
    defer st.Close()

    ctx := context.Background()

    // Save checkpoint after validation phase
    validatedState := WorkflowState{Phase: "validated"}
    err = st.SaveCheckpoint(ctx, "after-validation", validatedState, 5)
    if err != nil {
        return err
    }

    // Later: restore from checkpoint
    state, step, err := st.LoadCheckpoint(ctx, "after-validation")
    if err != nil {
        return err
    }

    log.Printf("Restored from checkpoint at step %d", step)
    return nil
}
```

### Health Check

```go
func checkDatabaseHealth(st *store.MySQLStore[State]) error {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    if err := st.Ping(ctx); err != nil {
        return fmt.Errorf("database unhealthy: %w", err)
    }

    // Check pool stats
    stats := st.Stats()
    log.Printf("Open connections: %d/%d", stats.OpenConnections, stats.MaxOpenConnections)
    log.Printf("Idle connections: %d", stats.Idle)

    return nil
}
```

## State Requirements

Your state type must be **JSON-serializable**:

```go
// ✅ Good: All fields are JSON-serializable
type GoodState struct {
    Counter int               `json:"counter"`
    Message string            `json:"message"`
    Data    map[string]string `json:"data"`
    Items   []string          `json:"items"`
}

// ❌ Bad: Channels, functions, and unexported fields don't serialize
type BadState struct {
    ch      chan int          // Unexported field - won't serialize
    Handler func() error      // Function - can't serialize
    Ch      chan int          // Channel - can't serialize
}
```

### Custom JSON Marshaling

```go
type CustomState struct {
    Time time.Time `json:"time"`
}

// Implement custom JSON marshaling if needed
func (s CustomState) MarshalJSON() ([]byte, error) {
    type Alias CustomState
    return json.Marshal(&struct {
        Time string `json:"time"`
        *Alias
    }{
        Time:  s.Time.Format(time.RFC3339),
        Alias: (*Alias)(&s),
    })
}
```

## Migration from MemStore

Switching from `MemStore` to `MySQLStore` is straightforward:

```go
// Before: In-memory storage
st := store.NewMemStore[State]()

// After: MySQL storage
dsn := os.Getenv("MYSQL_DSN")
st, err := store.NewMySQLStore[State](dsn)
if err != nil {
    log.Fatal(err)
}
defer st.Close()

// Rest of the code remains the same!
engine := graph.New(reducer, st, emitter, opts)
```

## Performance Considerations

### Indexing

The default schema includes indexes for common queries:
- `idx_run_id`: Speeds up queries by run ID
- `idx_run_step`: Composite index for run ID + step lookups
- `unique_run_step`: Enforces uniqueness and provides index

### Query Patterns

**Fast Queries** (use indexes):
```go
st.LoadLatest(ctx, "run-123")        // Uses idx_run_step
st.LoadCheckpoint(ctx, "checkpoint") // Uses UNIQUE index
```

**Slow Queries** (full table scans):
- Searching by state content (no index on JSON)
- Searching by node_id without run_id
- Aggregations across many runs

### Optimization Tips

1. **Keep run IDs short**: UUID v4 (36 chars) is fine, but shorter is better
2. **Checkpoint selectively**: Don't create checkpoints on every step
3. **Clean up old data**: Implement retention policies (see below)
4. **Monitor pool usage**: Check `Stats()` for connection pool health

### Retention Policy Example

```sql
-- Delete workflow steps older than 30 days
DELETE FROM workflow_steps
WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);

-- Delete checkpoints older than 90 days
DELETE FROM workflow_checkpoints
WHERE created_at < DATE_SUB(NOW(), INTERVAL 90 DAY);
```

## Troubleshooting

### Connection Refused

```
Error: failed to ping MySQL: dial tcp 127.0.0.1:3306: connect: connection refused
```

**Solutions**:
1. Verify MySQL is running: `systemctl status mysql`
2. Check MySQL port: `netstat -tuln | grep 3306`
3. Verify DSN host and port are correct

### Access Denied

```
Error: failed to ping MySQL: Error 1045: Access denied for user 'user'@'host'
```

**Solutions**:
1. Verify username and password in DSN
2. Check user permissions: `SHOW GRANTS FOR 'user'@'host';`
3. Ensure user has `SELECT, INSERT, UPDATE, CREATE` privileges

### Table Creation Failed

```
Error: failed to create workflow_steps table: Error 1050: Table already exists
```

This shouldn't happen (uses `CREATE TABLE IF NOT EXISTS`), but if it does:

**Solutions**:
1. Check table schema matches expected schema
2. Manually drop and recreate tables (⚠️ loses data!)
3. Verify MySQL version supports `IF NOT EXISTS`

### JSON Serialization Error

```
Error: failed to marshal state: json: unsupported type: chan int
```

**Solutions**:
1. Remove channels, functions, or unexported fields from state
2. Implement custom `MarshalJSON()` method
3. Use JSON-serializable types only

### Connection Pool Exhausted

```
Error: sql: connection reset
```

**Solutions**:
1. Increase `MaxOpenConns` for high concurrency
2. Check for connection leaks (missing `defer Close()`)
3. Reduce workflow concurrency or use connection pool monitoring

### Stale Connections

```
Error: invalid connection
```

The MySQL Store now prevents this with proper timeouts (fixed in T187):
- `ConnMaxLifetime: 5 minutes`
- `ConnMaxIdleTime: 10 minutes`

## Security Best Practices

### 1. Never Hardcode Credentials

❌ **Bad**:
```go
dsn := "root:password123@tcp(localhost:3306)/workflows"
```

✅ **Good**:
```go
dsn := os.Getenv("MYSQL_DSN")
if dsn == "" {
    log.Fatal("MYSQL_DSN not set")
}
```

### 2. Use Least Privilege

Create a dedicated user with minimal permissions:

```sql
CREATE USER 'workflow_app'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT, INSERT, UPDATE ON workflows.* TO 'workflow_app'@'%';
-- Don't grant DELETE, DROP, or admin privileges
```

### 3. Enable TLS for Remote Connections

```
user:pass@tcp(remote-host:3306)/workflows?tls=true
```

### 4. Use IAM Authentication (AWS RDS)

```go
// Use AWS IAM authentication instead of password
import "github.com/aws/aws-sdk-go/service/rds/rdsutils"

authToken, err := rdsutils.BuildAuthToken(endpoint, region, dbUser, awsCreds)
dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, authToken, endpoint, dbName)
```

### 5. Rotate Credentials Regularly

- Use secret managers (AWS Secrets Manager, HashiCorp Vault)
- Implement credential rotation policies
- Don't commit DSNs to version control

## Compatibility

### MySQL Versions

- ✅ MySQL 5.7+ (JSON support required)
- ✅ MySQL 8.0+
- ✅ MariaDB 10.2+ (JSON support)

### Go Versions

- ✅ Go 1.21+
- ✅ Go 1.22+
- ✅ Go 1.23+

### Drivers

- Primary: `github.com/go-sql-driver/mysql` (official)
- Compatible with any `database/sql` driver

## FAQ

**Q: Can I use PostgreSQL instead of MySQL?**
A: Not yet. PostgreSQL support is planned but not implemented. The `Store[S]` interface is designed to support multiple backends.

**Q: What's the maximum state size?**
A: Limited by MySQL's `max_allowed_packet` (default 64MB). Keep state under 1MB for best performance.

**Q: Can multiple workers share the same database?**
A: Yes! This is the primary use case for distributed workflows. Use unique `run_id` values.

**Q: Does it support transactions?**
A: Yes, via `SaveStepBatch()` and `WithTransaction()` for atomic multi-step operations.

**Q: How do I backup workflow data?**
A: Use standard MySQL backup tools (`mysqldump`, Percona XtraBackup, or cloud-native backups).

**Q: Can I query workflow history with SQL?**
A: Yes! Direct SQL queries are supported:
```sql
SELECT run_id, step, node_id, created_at
FROM workflow_steps
WHERE run_id = 'run-123'
ORDER BY step;
```

## References

- [MySQL JSON Documentation](https://dev.mysql.com/doc/refman/8.0/en/json.html)
- [Go MySQL Driver](https://github.com/go-sql-driver/mysql)
- [LangGraph-Go Store Interface](../store.go)

## Support

For issues or questions:
- [GitHub Issues](https://github.com/dshills/langgraph-go/issues)
- [Documentation](../../docs/)
