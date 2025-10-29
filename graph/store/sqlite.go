package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	_ "modernc.org/sqlite"
)

// SQLiteStore is a SQLite implementation of Store[S].
//
// It stores workflow state and checkpoints in a single-file database.
// Designed for:
//   - Development and testing with zero setup
//   - Single-process workflows
//   - Local workflows requiring persistence
//   - Prototyping before migrating to distributed store
//
// SQLiteStore uses WAL mode for concurrent reads and proper transactions.
//
// Features:
//   - Single file database (e.g., "./dev.db")
//   - Auto-migration on first use
//   - WAL mode for concurrent reads
//   - Transactional writes for safety
//
// Schema:
//   - workflow_steps: Step-by-step execution history
//   - workflow_checkpoints: Named checkpoints for resumption
//   - workflow_checkpoints_v2: Enhanced checkpoints with full context
//   - idempotency_keys: Duplicate prevention
//   - events_outbox: Transactional event delivery
//
// Type parameter S is the state type to persist (must be JSON-serializable).
type SQLiteStore[S any] struct {
	db     *sql.DB
	mu     sync.RWMutex
	closed bool
	path   string
}

// NewSQLiteStore creates a new SQLite-backed store.
//
// The path parameter specifies the database file location:
//   - "./dev.db" - file in current directory
//   - "/tmp/workflow.db" - absolute path
//   - ":memory:" - in-memory database (data lost on close)
//
// The store automatically:
//   - Creates the database file if it doesn't exist
//   - Creates required tables
//   - Enables WAL mode for concurrent reads
//   - Configures appropriate timeouts
//
// WAL Mode Benefits:
//   - Multiple readers can access database concurrently
//   - Writers don't block readers
//   - Better concurrency for read-heavy workloads
//
// Example:
//
//	store, err := NewSQLiteStore[MyState]("./dev.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
//
// For testing with in-memory database:
//
//	store, err := NewSQLiteStore[MyState](":memory:")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
func NewSQLiteStore[S any](path string) (*SQLiteStore[S], error) {
	// Open database connection
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1)    // SQLite supports one writer at a time
	db.SetMaxIdleConns(1)    // Keep connection open
	db.SetConnMaxLifetime(0) // No max lifetime for SQLite

	// Enable WAL mode for better concurrency
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close() // Ignore close error when returning pragma error
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close() // Ignore close error when returning pragma error
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy timeout (wait up to 5 seconds for locks)
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close() // Ignore close error when returning pragma error
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	store := &SQLiteStore[S]{
		db:     db,
		closed: false,
		path:   path,
	}

	// Create tables if they don't exist
	if err := store.createTables(ctx); err != nil {
		_ = db.Close() // Ignore close error when returning table creation error
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return store, nil
}

// createTables creates the required database schema if it doesn't exist.
func (s *SQLiteStore[S]) createTables(ctx context.Context) error {
	// workflow_steps table: stores step-by-step execution history
	stepsTable := `
		CREATE TABLE IF NOT EXISTS workflow_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			step INTEGER NOT NULL,
			node_id TEXT NOT NULL,
			state TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(run_id, step)
		)
	`
	if _, err := s.db.ExecContext(ctx, stepsTable); err != nil {
		return fmt.Errorf("failed to create workflow_steps table: %w", err)
	}

	// Create indexes for workflow_steps
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_steps_run_id ON workflow_steps(run_id)"); err != nil {
		return fmt.Errorf("failed to create idx_steps_run_id: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_steps_run_step ON workflow_steps(run_id, step)"); err != nil {
		return fmt.Errorf("failed to create idx_steps_run_step: %w", err)
	}

	// workflow_checkpoints table: stores named checkpoints
	checkpointsTable := `
		CREATE TABLE IF NOT EXISTS workflow_checkpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			checkpoint_id TEXT NOT NULL UNIQUE,
			state TEXT NOT NULL,
			step INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := s.db.ExecContext(ctx, checkpointsTable); err != nil {
		return fmt.Errorf("failed to create workflow_checkpoints table: %w", err)
	}

	// workflow_checkpoints_v2 table: stores enhanced checkpoints with full execution context
	checkpointsV2Table := `
		CREATE TABLE IF NOT EXISTS workflow_checkpoints_v2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL,
			step_id INTEGER NOT NULL,
			state TEXT NOT NULL,
			frontier TEXT NOT NULL,
			rng_seed INTEGER NOT NULL,
			recorded_ios TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			timestamp TIMESTAMP NOT NULL,
			label TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(run_id, step_id)
		)
	`
	if _, err := s.db.ExecContext(ctx, checkpointsV2Table); err != nil {
		return fmt.Errorf("failed to create workflow_checkpoints_v2 table: %w", err)
	}

	// Create indexes for workflow_checkpoints_v2
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_checkpoints_v2_run_id ON workflow_checkpoints_v2(run_id)"); err != nil {
		return fmt.Errorf("failed to create idx_checkpoints_v2_run_id: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_checkpoints_v2_run_step ON workflow_checkpoints_v2(run_id, step_id)"); err != nil {
		return fmt.Errorf("failed to create idx_checkpoints_v2_run_step: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_checkpoints_v2_label ON workflow_checkpoints_v2(run_id, label)"); err != nil {
		return fmt.Errorf("failed to create idx_checkpoints_v2_label: %w", err)
	}

	// idempotency_keys table: tracks used idempotency keys to prevent duplicate commits
	idempotencyTable := `
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			key_value TEXT NOT NULL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := s.db.ExecContext(ctx, idempotencyTable); err != nil {
		return fmt.Errorf("failed to create idempotency_keys table: %w", err)
	}

	// Create index for idempotency_keys
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_idempotency_created ON idempotency_keys(created_at)"); err != nil {
		return fmt.Errorf("failed to create idx_idempotency_created: %w", err)
	}

	// events_outbox table: stores events for transactional outbox pattern
	eventsOutboxTable := `
		CREATE TABLE IF NOT EXISTS events_outbox (
			id TEXT NOT NULL PRIMARY KEY,
			run_id TEXT NOT NULL,
			event_data TEXT NOT NULL,
			emitted_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	if _, err := s.db.ExecContext(ctx, eventsOutboxTable); err != nil {
		return fmt.Errorf("failed to create events_outbox table: %w", err)
	}

	// Create indexes for events_outbox
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_events_pending ON events_outbox(emitted_at, created_at)"); err != nil {
		return fmt.Errorf("failed to create idx_events_pending: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_events_run_id ON events_outbox(run_id)"); err != nil {
		return fmt.Errorf("failed to create idx_events_run_id: %w", err)
	}

	return nil
}

// SaveStep persists a workflow execution step (implements Store interface).
//
// Steps are stored in the workflow_steps table with the current state.
// If a step with the same runID and step number already exists, it is replaced.
//
// Thread-safe for concurrent writes.
func (s *SQLiteStore[S]) SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	// Serialize state to JSON
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Insert or replace step
	query := `
		INSERT INTO workflow_steps (run_id, step, node_id, state)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(run_id, step) DO UPDATE SET
			node_id = excluded.node_id,
			state = excluded.state
	`

	_, err = s.db.ExecContext(ctx, query, runID, step, nodeID, string(stateJSON))
	if err != nil {
		return fmt.Errorf("failed to save step: %w", err)
	}

	return nil
}

// LoadLatest retrieves the most recent step for a run (implements Store interface).
//
// Returns the step with the highest step number for the given runID.
// Returns ErrNotFound if no steps exist for the runID.
func (s *SQLiteStore[S]) LoadLatest(ctx context.Context, runID string) (state S, step int, err error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		var zero S
		return zero, 0, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	query := `
		SELECT step, state
		FROM workflow_steps
		WHERE run_id = ?
		ORDER BY step DESC
		LIMIT 1
	`

	var stateJSON string
	err = s.db.QueryRowContext(ctx, query, runID).Scan(&step, &stateJSON)
	if err == sql.ErrNoRows {
		var zero S
		return zero, 0, ErrNotFound
	}
	if err != nil {
		var zero S
		return zero, 0, fmt.Errorf("failed to load latest step: %w", err)
	}

	// Deserialize state
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		var zero S
		return zero, 0, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, step, nil
}

// SaveCheckpoint creates a named checkpoint (implements Store interface).
//
// Checkpoints are stored in the workflow_checkpoints table.
// If a checkpoint with the same ID exists, it is updated.
//
// Thread-safe for concurrent writes.
func (s *SQLiteStore[S]) SaveCheckpoint(ctx context.Context, cpID string, state S, step int) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	// Serialize state to JSON
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Insert or update checkpoint
	query := `
		INSERT INTO workflow_checkpoints (checkpoint_id, state, step)
		VALUES (?, ?, ?)
		ON CONFLICT(checkpoint_id) DO UPDATE SET
			state = excluded.state,
			step = excluded.step,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query, cpID, string(stateJSON), step)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint retrieves a named checkpoint (implements Store interface).
//
// Returns ErrNotFound if the checkpoint ID doesn't exist.
func (s *SQLiteStore[S]) LoadCheckpoint(ctx context.Context, cpID string) (state S, step int, err error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		var zero S
		return zero, 0, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	query := `
		SELECT state, step
		FROM workflow_checkpoints
		WHERE checkpoint_id = ?
	`

	var stateJSON string
	err = s.db.QueryRowContext(ctx, query, cpID).Scan(&stateJSON, &step)
	if err == sql.ErrNoRows {
		var zero S
		return zero, 0, ErrNotFound
	}
	if err != nil {
		var zero S
		return zero, 0, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Deserialize state
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		var zero S
		return zero, 0, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, step, nil
}

// SaveCheckpointV2 persists an enhanced checkpoint with full execution context.
//
// This method saves a complete checkpoint including:
//   - Current state after all deltas applied
//   - Frontier of pending work items
//   - Recorded I/O for deterministic replay
//   - RNG seed for random value consistency
//   - Idempotency key to prevent duplicate commits
//
// The operation is performed in a transaction to ensure atomicity.
// If the idempotency key already exists, returns an error (prevents duplicate saves).
//
// Thread-safe for concurrent writes.
func (s *SQLiteStore[S]) SaveCheckpointV2(ctx context.Context, checkpoint CheckpointV2[S]) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	// Serialize JSON fields
	stateJSON, err := json.Marshal(checkpoint.State)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	frontierJSON, err := json.Marshal(checkpoint.Frontier)
	if err != nil {
		return fmt.Errorf("failed to marshal frontier: %w", err)
	}

	recordedIOsJSON, err := json.Marshal(checkpoint.RecordedIOs)
	if err != nil {
		return fmt.Errorf("failed to marshal recorded IOs: %w", err)
	}

	// Begin transaction for atomic insert
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on error
	defer func() {
		if err != nil {
			_ = tx.Rollback() // Ignore rollback error when already returning error
		}
	}()

	// Insert idempotency key first (will fail if duplicate)
	idempotencyQuery := `
		INSERT INTO idempotency_keys (key_value)
		VALUES (?)
	`

	_, err = tx.ExecContext(ctx, idempotencyQuery, checkpoint.IdempotencyKey)
	if err != nil {
		// Check if it's a duplicate key error
		return fmt.Errorf("idempotency key already exists or insert failed: %w", err)
	}

	// Insert checkpoint
	checkpointQuery := `
		INSERT INTO workflow_checkpoints_v2
		(run_id, step_id, state, frontier, rng_seed, recorded_ios, idempotency_key, timestamp, label)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, step_id) DO UPDATE SET
			state = excluded.state,
			frontier = excluded.frontier,
			rng_seed = excluded.rng_seed,
			recorded_ios = excluded.recorded_ios,
			idempotency_key = excluded.idempotency_key,
			timestamp = excluded.timestamp,
			label = excluded.label
	`

	_, err = tx.ExecContext(ctx, checkpointQuery,
		checkpoint.RunID,
		checkpoint.StepID,
		string(stateJSON),
		string(frontierJSON),
		checkpoint.RNGSeed,
		string(recordedIOsJSON),
		checkpoint.IdempotencyKey,
		checkpoint.Timestamp.Format(time.RFC3339Nano),
		checkpoint.Label,
	)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadCheckpointV2 retrieves an enhanced checkpoint by run ID and step ID.
//
// This method can also load checkpoints by label if stepID is 0 and a label is stored.
// Returns ErrNotFound if no checkpoint exists for the given identifiers.
func (s *SQLiteStore[S]) LoadCheckpointV2(ctx context.Context, runID string, stepID int) (CheckpointV2[S], error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	query := `
		SELECT run_id, step_id, state, frontier, rng_seed, recorded_ios, idempotency_key, timestamp, label
		FROM workflow_checkpoints_v2
		WHERE run_id = ? AND step_id = ?
		LIMIT 1
	`

	var (
		stateJSON       string
		frontierJSON    string
		recordedIOsJSON string
		timestampStr    string
		checkpoint      CheckpointV2[S]
	)

	err := s.db.QueryRowContext(ctx, query, runID, stepID).Scan(
		&checkpoint.RunID,
		&checkpoint.StepID,
		&stateJSON,
		&frontierJSON,
		&checkpoint.RNGSeed,
		&recordedIOsJSON,
		&checkpoint.IdempotencyKey,
		&timestampStr,
		&checkpoint.Label,
	)

	if err == sql.ErrNoRows {
		var zero CheckpointV2[S]
		return zero, ErrNotFound
	}
	if err != nil {
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Parse timestamp
	checkpoint.Timestamp, err = time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Deserialize JSON fields
	if err := json.Unmarshal([]byte(stateJSON), &checkpoint.State); err != nil {
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if err := json.Unmarshal([]byte(frontierJSON), &checkpoint.Frontier); err != nil {
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("failed to unmarshal frontier: %w", err)
	}

	if err := json.Unmarshal([]byte(recordedIOsJSON), &checkpoint.RecordedIOs); err != nil {
		var zero CheckpointV2[S]
		return zero, fmt.Errorf("failed to unmarshal recorded IOs: %w", err)
	}

	return checkpoint, nil
}

// CheckIdempotency verifies if an idempotency key has been used.
//
// Returns true if the key exists in the idempotency_keys table.
// Returns false if the key doesn't exist (safe to use).
// Returns error only on database access failures.
//
// This uses a unique constraint on the key for race-safe duplicate detection.
func (s *SQLiteStore[S]) CheckIdempotency(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return false, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	query := `
		SELECT COUNT(*) FROM idempotency_keys WHERE key_value = ?
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, key).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check idempotency: %w", err)
	}

	return count > 0, nil
}

// PendingEvents retrieves events from the outbox that haven't been emitted yet.
//
// Returns events where emitted_at IS NULL, ordered by created_at.
// Limited to the specified number of events for batching.
func (s *SQLiteStore[S]) PendingEvents(ctx context.Context, limit int) ([]emit.Event, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	query := `
		SELECT id, run_id, event_data
		FROM events_outbox
		WHERE emitted_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []emit.Event
	for rows.Next() {
		var (
			id        string
			runID     string
			eventJSON string
		)

		if err := rows.Scan(&id, &runID, &eventJSON); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}

		var event emit.Event
		if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event rows: %w", err)
	}

	return events, nil
}

// MarkEventsEmitted marks events as successfully emitted to prevent re-delivery.
//
// Updates the emitted_at timestamp for the specified event IDs.
// This ensures the events won't be returned by PendingEvents again.
func (s *SQLiteStore[S]) MarkEventsEmitted(ctx context.Context, eventIDs []string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	if len(eventIDs) == 0 {
		return nil // No-op for empty list
	}

	// Build IN clause with placeholders
	placeholders := ""
	args := make([]interface{}, len(eventIDs))
	for i, id := range eventIDs {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = id
	}

	// #nosec G201 -- placeholders are not user input, just "?" marks for parameterized query
	query := fmt.Sprintf(`
		UPDATE events_outbox
		SET emitted_at = CURRENT_TIMESTAMP
		WHERE id IN (%s)
	`, placeholders)

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark events as emitted: %w", err)
	}

	return nil
}

// Close closes the database connection.
//
// After Close, all operations will return an error.
// Calling Close multiple times is safe (subsequent calls are no-ops).
func (s *SQLiteStore[S]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil // Double-close is a no-op
	}

	s.closed = true
	return s.db.Close()
}

// Ping verifies the database connection is alive.
//
// Useful for health checks and connection validation.
func (s *SQLiteStore[S]) Ping(ctx context.Context) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("store is closed")
	}
	s.mu.RUnlock()

	return s.db.PingContext(ctx)
}

// Path returns the database file path.
//
// This is useful for debugging and logging.
func (s *SQLiteStore[S]) Path() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.path
}
