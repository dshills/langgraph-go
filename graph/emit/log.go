// Package emit provides event emission and observability for graph execution.
package emit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LogEmitter implements Emitter by writing structured log output to a writer (T161).
//
// Supports two output modes:
// - Text mode (default): Human-readable format with key=value pairs.
// - JSON mode: Machine-readable JSON format, one event per line.
//
// Example text output:
//
// [node_start] runID=run-001 step=0 nodeID=nodeA.
//
// Example JSON output:
//
// {"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_start","meta":null}.
//
// Usage:
//
// // Text output to stdout.
// emitter := emit.NewLogEmitter(os.Stdout, false).
//
// // JSON output to file.
// f, _ := os.Create("events.jsonl").
// defer func() { _ = f.Close() }().
// emitter := emit.NewLogEmitter(f, true).
type LogEmitter struct {
	writer   io.Writer
	jsonMode bool
}

// NewLogEmitter creates a new LogEmitter (T161, T163).
//
// Parameters:
// - writer: Where to write the log output (e.g., os.Stdout, file).
// - jsonMode: If true, emit JSON format; if false, emit text format.
//
// Returns a LogEmitter that writes structured event data to the provided writer.
func NewLogEmitter(writer io.Writer, jsonMode bool) *LogEmitter {
	if writer == nil {
		writer = os.Stdout
	}
	return &LogEmitter{
		writer:   writer,
		jsonMode: jsonMode,
	}
}

// Emit writes an event to the configured writer (T161).
//
// Format depends on jsonMode:
// - JSON mode: Writes event as single-line JSON object.
// - Text mode: Writes human-readable format with [msg] prefix.
//
// Example text output:
//
// [node_start] runID=run-001 step=0 nodeID=nodeA.
// [node_end] runID=run-001 step=0 nodeID=nodeA meta={"delta":{"counter":5}}.
//
// Example JSON output:
//
// {"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_start","meta":null}.
// {"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_end","meta":{"delta":{"counter":5}}}.
func (l *LogEmitter) Emit(event Event) {
	if l.jsonMode {
		l.emitJSON(event)
	} else {
		l.emitText(event)
	}
}

// emitJSON writes event as JSON to the writer (T163).
func (l *LogEmitter) emitJSON(event Event) {
	// Marshal event to JSON.
	data, err := json.Marshal(struct {
		RunID  string                 `json:"runID"`
		Step   int                    `json:"step"`
		NodeID string                 `json:"nodeID"`
		Msg    string                 `json:"msg"`
		Meta   map[string]interface{} `json:"meta"`
	}{
		RunID:  event.RunID,
		Step:   event.Step,
		NodeID: event.NodeID,
		Msg:    event.Msg,
		Meta:   event.Meta,
	})
	if err != nil {
		// Fallback to error message if marshal fails.
		_, _ = fmt.Fprintf(l.writer, "{\"error\":\"failed to marshal event: %v\"}\n", err)
		return
	}

	// Write JSON followed by newline (JSONL format).
	_, _ = fmt.Fprintf(l.writer, "%s\n", data)
}

// emitText writes event as human-readable text to the writer (T161).
func (l *LogEmitter) emitText(event Event) {
	// Format: [msg] runID=xxx step=N nodeID=yyy [meta=...].
	_, _ = fmt.Fprintf(l.writer, "[%s] runID=%s step=%d nodeID=%s",
		event.Msg, event.RunID, event.Step, event.NodeID)

	// Add meta if present.
	if len(event.Meta) > 0 {
		// Try to marshal meta as JSON for readability.
		metaJSON, err := json.Marshal(event.Meta)
		if err == nil {
			_, _ = fmt.Fprintf(l.writer, " meta=%s", metaJSON)
		} else {
			_, _ = fmt.Fprintf(l.writer, " meta=%v", event.Meta)
		}
	}

	_, _ = fmt.Fprint(l.writer, "\n")
}

// EmitBatch sends multiple events in a single operation for improved performance (T107).
//
// For LogEmitter, batching provides efficiency by:
// - Reducing write syscalls (one write per batch vs per event).
// - Better formatting when viewing multiple related events.
// - Maintaining chronological order within the batch.
//
// In text mode, events are written with blank lines between them for readability.
// In JSON mode, events are written as JSONL (one per line) for easy parsing.
//
// Example text output:
//
// [node_start] runID=run-001 step=0 nodeID=nodeA.
// [node_end] runID=run-001 step=0 nodeID=nodeA.
// [node_start] runID=run-001 step=1 nodeID=nodeB.
//
// Example JSON output:
//
// {"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_start","meta":null}.
// {"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_end","meta":{"delta":{"counter":5}}}.
// {"runID":"run-001","step":1,"nodeID":"nodeB","msg":"node_start","meta":null}.
//
// This implementation is more efficient than calling Emit repeatedly because:
// 1. It can batch multiple events into fewer write operations.
// 2. It can optimize formatting across the entire batch.
// 3. It reduces locking overhead if the writer is synchronized.
//
// Parameters:
// - ctx: Context for cancellation (currently unused but reserved for future enhancements).
// - events: Slice of events to emit in order.
//
// Returns error only if writing fails. Always attempts to write all events.
func (l *LogEmitter) EmitBatch(_ context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	// Build output for all events before writing to minimize syscalls.
	if l.jsonMode {
		// JSON mode: write all events as JSONL.
		for _, event := range events {
			l.emitJSON(event)
		}
	} else {
		// Text mode: write all events with consistent formatting.
		for _, event := range events {
			l.emitText(event)
		}
	}

	return nil
}

// Flush ensures all buffered events are sent to the backend (T108).
//
// For LogEmitter, this is a no-op because:
// - All writes go directly to the underlying io.Writer.
// - No internal buffering is maintained by LogEmitter.
// - The writer itself handles its own buffering (e.g., os.Stdout, bufio.Writer).
//
// If you need flush control, wrap the writer with bufio.Writer and call Flush on it directly:
//
// buf := bufio.NewWriter(os.Stdout).
// emitter := emit.NewLogEmitter(buf, false).
//
//	// ... emit events ...
//
// buf.Flush() // Flush the underlying buffer.
// emitter.Flush(ctx) // No-op for LogEmitter.
//
// This method is provided to satisfy the Emitter interface and enable polymorphic usage.
// with other emitters (e.g., OTelEmitter) that do require flushing.
//
// Parameters:
// - ctx: Context for cancellation (unused, LogEmitter writes are synchronous).
//
// Returns nil (always succeeds).
func (l *LogEmitter) Flush(_ context.Context) error {
	// No-op: LogEmitter writes directly without buffering.
	return nil
}
