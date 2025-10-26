# MySQL Store Migrations

Database migration scripts for LangGraph-Go MySQL Store.

## Overview

This directory contains SQL migration scripts to set up, upgrade, and manage the MySQL Store schema. The migrations follow a versioned approach compatible with popular migration tools.

## Migration Files

### Versioned Migrations (golang-migrate compatible)

- `000001_initial_schema.up.sql` - Create initial tables (workflow_steps, workflow_checkpoints)
- `000001_initial_schema.down.sql` - Rollback initial schema

### Standalone Scripts

- `setup.sql` - Complete setup script (can be run standalone)
- `teardown.sql` - Complete teardown script (⚠️ deletes all data)

## Quick Start

### Option 1: Automatic Setup (Recommended)

The MySQL Store automatically creates tables on first connection. Just create your store:

```go
import "github.com/dshills/langgraph-go/graph/store"

dsn := os.Getenv("MYSQL_DSN")
st, err := store.NewMySQLStore[State](dsn)
if err != nil {
    log.Fatal(err)
}
defer st.Close()
```

The tables will be created automatically if they don't exist.

### Option 2: Manual Setup (SQL Scripts)

If you prefer to set up the schema manually before running your application:

```bash
# From command line
mysql -u username -p database_name < setup.sql

# Or from MySQL shell
mysql> USE your_database;
mysql> SOURCE /path/to/migrations/setup.sql;
```

### Option 3: Migration Tool (golang-migrate)

For production environments with version control:

#### Install golang-migrate

```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# Go install
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

#### Run Migrations

```bash
# Set your database URL
export DATABASE_URL="mysql://user:pass@tcp(localhost:3306)/workflows"

# Run migrations up
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" up

# Check migration version
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" version

# Rollback one migration
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" down 1

# Force to specific version (use with caution)
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" force 1
```

## Migration Versioning

Migrations are numbered sequentially:

- `000001` - Initial schema (workflow_steps, workflow_checkpoints)
- `000002` - (Future) Add indexes for performance
- `000003` - (Future) Add metadata columns
- etc.

Each migration has an "up" (apply) and "down" (rollback) script.

## Schema Details

### V1 - Initial Schema (000001)

Creates two tables:

**workflow_steps**:
- Stores step-by-step execution history
- Primary key: auto-incrementing `id`
- Indexed by: `run_id`, `(run_id, step)`
- Unique constraint on `(run_id, step)`

**workflow_checkpoints**:
- Stores named checkpoints for resumption
- Primary key: auto-incrementing `id`
- Unique constraint on `checkpoint_id`
- Tracks creation and update times

See [../README.md](../README.md) for complete schema documentation.

## Creating New Migrations

When you need to modify the schema:

### 1. Create Migration Files

```bash
# Next version number: 000002
touch 000002_your_migration_name.up.sql
touch 000002_your_migration_name.down.sql
```

### 2. Write Up Migration

`000002_your_migration_name.up.sql`:
```sql
-- Add new column
ALTER TABLE workflow_steps ADD COLUMN metadata JSON DEFAULT NULL;

-- Add index
CREATE INDEX idx_metadata ON workflow_steps((CAST(metadata->>'$.key' AS CHAR(255))));
```

### 3. Write Down Migration

`000002_your_migration_name.down.sql`:
```sql
-- Reverse changes in opposite order
DROP INDEX idx_metadata ON workflow_steps;
ALTER TABLE workflow_steps DROP COLUMN metadata;
```

### 4. Test Migrations

```bash
# Test up
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" up

# Verify schema
mysql -u user -p -e "DESCRIBE workflow_steps"

# Test down
migrate -path graph/store/mysql/migrations -database "$DATABASE_URL" down 1

# Verify rollback
mysql -u user -p -e "DESCRIBE workflow_steps"
```

## Best Practices

### 1. Always Test Migrations

- Test on a copy of production data
- Verify both up and down migrations
- Check for data loss or corruption

### 2. Backup Before Migration

```bash
# Backup database
mysqldump -u user -p database_name > backup_$(date +%Y%m%d_%H%M%S).sql

# Backup workflow tables only
mysqldump -u user -p database_name workflow_steps workflow_checkpoints > workflow_backup.sql
```

### 3. Use Transactions (When Possible)

Wrap DDL statements in transactions where supported:

```sql
START TRANSACTION;

ALTER TABLE workflow_steps ADD COLUMN new_col VARCHAR(255);
-- More statements...

COMMIT;
```

⚠️ Note: MySQL DDL statements auto-commit, so true transaction rollback isn't possible for schema changes. Always backup first!

### 4. Document Migration Intent

Include comments explaining:
- What the migration does
- Why it's needed
- Any special considerations

### 5. Never Modify Applied Migrations

Once a migration is applied in production, create a new migration instead of modifying the existing one.

## Troubleshooting

### Migration Failed

```bash
# Check current version
migrate -path migrations -database "$DATABASE_URL" version

# Check for dirty state
mysql> SELECT * FROM schema_migrations;

# Force to specific version (last known good)
migrate -path migrations -database "$DATABASE_URL" force 1

# Manually fix and retry
migrate -path migrations -database "$DATABASE_URL" up
```

### Tables Already Exist

If tables were created automatically by the application:

```bash
# Mark migrations as applied without running them
migrate -path migrations -database "$DATABASE_URL" force 1
```

### Permission Denied

Ensure your MySQL user has these privileges:

```sql
GRANT CREATE, ALTER, DROP, INDEX ON database_name.* TO 'user'@'host';
FLUSH PRIVILEGES;
```

### Migration Locks

If a migration hangs, check for locks:

```sql
-- Show running processes
SHOW PROCESSLIST;

-- Kill stuck process
KILL <process_id>;

-- Check table locks
SHOW OPEN TABLES WHERE In_use > 0;
```

## Production Checklist

Before running migrations in production:

- [ ] Backup database
- [ ] Test migrations on staging/copy
- [ ] Review migration scripts
- [ ] Check for breaking changes
- [ ] Verify down migrations work
- [ ] Schedule during maintenance window
- [ ] Have rollback plan ready
- [ ] Monitor application during migration
- [ ] Verify data integrity after migration

## Integration with Go Code

### Manual Migration Before App Start

```go
package main

import (
    "database/sql"
    "log"

    _ "github.com/go-sql-driver/mysql"
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/mysql"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(dsn string) error {
    // Open database
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return err
    }
    defer db.Close()

    // Create migrate instance
    driver, err := mysql.WithInstance(db, &mysql.Config{})
    if err != nil {
        return err
    }

    m, err := migrate.NewWithDatabaseInstance(
        "file://graph/store/mysql/migrations",
        "mysql",
        driver,
    )
    if err != nil {
        return err
    }

    // Run migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return err
    }

    log.Println("Migrations applied successfully")
    return nil
}

func main() {
    dsn := os.Getenv("MYSQL_DSN")

    // Run migrations before creating store
    if err := runMigrations(dsn); err != nil {
        log.Fatal(err)
    }

    // Now create store
    st, err := store.NewMySQLStore[State](dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer st.Close()

    // ... rest of application
}
```

## References

- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [MySQL ALTER TABLE Reference](https://dev.mysql.com/doc/refman/8.0/en/alter-table.html)
- [MySQL Migration Best Practices](https://dev.mysql.com/doc/workbench/en/wb-migration.html)
- [LangGraph-Go MySQL Store](../README.md)

## Support

For migration issues:
- Check [Troubleshooting](#troubleshooting) section
- Review [MySQL Store README](../README.md)
- Open [GitHub Issue](https://github.com/dshills/langgraph-go/issues)
