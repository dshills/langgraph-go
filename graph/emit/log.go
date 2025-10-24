package emit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LogEmitter implements Emitter by writing structured log output to a writer (T161).
//
// Supports two output modes:
//   - Text mode (default): Human-readable format with key=value pairs
//   - JSON mode: Machine-readable JSON format, one event per line
//
// Example text output:
//
//	[node_start] runID=run-001 step=0 nodeID=nodeA
//
// Example JSON output:
//
//	{"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_start","meta":null}
//
// Usage:
//
//	// Text output to stdout
//	emitter := emit.NewLogEmitter(os.Stdout, false)
//
//	// JSON output to file
//	f, _ := os.Create("events.jsonl")
//	defer f.Close()
//	emitter := emit.NewLogEmitter(f, true)
type LogEmitter struct {
	writer   io.Writer
	jsonMode bool
}

// NewLogEmitter creates a new LogEmitter (T161, T163).
//
// Parameters:
//   - writer: Where to write the log output (e.g., os.Stdout, file)
//   - jsonMode: If true, emit JSON format; if false, emit text format
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
//   - JSON mode: Writes event as single-line JSON object
//   - Text mode: Writes human-readable format with [msg] prefix
//
// Example text output:
//
//	[node_start] runID=run-001 step=0 nodeID=nodeA
//	[node_end] runID=run-001 step=0 nodeID=nodeA meta={"delta":{"counter":5}}
//
// Example JSON output:
//
//	{"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_start","meta":null}
//	{"runID":"run-001","step":0,"nodeID":"nodeA","msg":"node_end","meta":{"delta":{"counter":5}}}
func (l *LogEmitter) Emit(event Event) {
	if l.jsonMode {
		l.emitJSON(event)
	} else {
		l.emitText(event)
	}
}

// emitJSON writes event as JSON to the writer (T163).
func (l *LogEmitter) emitJSON(event Event) {
	// Marshal event to JSON
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
		// Fallback to error message if marshal fails
		fmt.Fprintf(l.writer, "{\"error\":\"failed to marshal event: %v\"}\n", err)
		return
	}

	// Write JSON followed by newline (JSONL format)
	fmt.Fprintf(l.writer, "%s\n", data)
}

// emitText writes event as human-readable text to the writer (T161).
func (l *LogEmitter) emitText(event Event) {
	// Format: [msg] runID=xxx step=N nodeID=yyy [meta=...]
	fmt.Fprintf(l.writer, "[%s] runID=%s step=%d nodeID=%s",
		event.Msg, event.RunID, event.Step, event.NodeID)

	// Add meta if present
	if event.Meta != nil && len(event.Meta) > 0 {
		// Try to marshal meta as JSON for readability
		metaJSON, err := json.Marshal(event.Meta)
		if err == nil {
			fmt.Fprintf(l.writer, " meta=%s", metaJSON)
		} else {
			fmt.Fprintf(l.writer, " meta=%v", event.Meta)
		}
	}

	fmt.Fprint(l.writer, "\n")
}
