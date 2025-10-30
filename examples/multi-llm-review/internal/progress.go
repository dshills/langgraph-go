package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/dshills/langgraph-go/graph/emit"
)

// ProgressEmitter implements emit.Emitter and shows progress for each provider.
// It only displays progress percentages and errors, suppressing all other output.
type ProgressEmitter struct {
	writer         io.Writer
	mu             sync.Mutex
	totalBatches   int
	currentBatch   int
	providerNames  []string
	batchesStarted bool
	hasError       bool
	debug          bool
}

// NewProgressEmitter creates a new progress emitter that writes to the given writer.
func NewProgressEmitter(writer io.Writer, providerNames []string) *ProgressEmitter {
	if writer == nil {
		writer = os.Stdout
	}

	// Enable debug mode via environment variable
	debug := os.Getenv("DEBUG_PROGRESS") == "1"

	return &ProgressEmitter{
		writer:        writer,
		providerNames: providerNames,
		debug:         debug,
	}
}

// Emit processes workflow events and displays progress.
func (p *ProgressEmitter) Emit(event emit.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Handle different event types
	switch event.Msg {
	case "node_start":
		p.handleNodeStart(event)
	case "node_end":
		p.handleNodeEnd(event)
	case "error":
		p.handleError(event)
	}
}

// handleNodeStart tracks when nodes begin execution.
func (p *ProgressEmitter) handleNodeStart(event emit.Event) {
	// Track when discover node completes to get total batches
	if event.NodeID == "discover" {
		// Will get total batches from node_end
	}
}

// handleNodeEnd processes node completion events to update progress.
func (p *ProgressEmitter) handleNodeEnd(event emit.Event) {
	// Get total batches from discover node
	if event.NodeID == "discover" {
		delta := p.extractDelta(event.Meta)
		if delta != nil {
			if totalBatches := p.getIntField(delta, "total_batches"); totalBatches > 0 {
				p.totalBatches = totalBatches
				fmt.Fprintf(os.Stderr, "Found %d batches to process\n", p.totalBatches)
			}
		}
		return
	}

	// Track batch completion
	if event.NodeID == "review-batch" {
		delta := p.extractDelta(event.Meta)
		if delta != nil {
			// CurrentBatch in the delta is the NEXT batch to process, so subtract 1 for completed
			if currentBatch := p.getIntField(delta, "current_batch"); currentBatch > 0 {
				p.currentBatch = currentBatch - 1
			}
		}

		// Display progress after each batch
		if !p.hasError && p.totalBatches > 0 {
			p.displayProgress()
		}
	}
}

// extractDelta extracts the delta map from event metadata.
func (p *ProgressEmitter) extractDelta(meta map[string]interface{}) map[string]interface{} {
	if meta == nil {
		return nil
	}

	// Check if delta key exists
	deltaRaw, exists := meta["delta"]
	if !exists {
		return nil
	}

	// Try direct map access first
	if delta, ok := deltaRaw.(map[string]interface{}); ok {
		return delta
	}

	// Delta is a struct - marshal to JSON then unmarshal to map
	// This handles the case where emitNodeEnd passes the actual struct
	deltaJSON, err := json.Marshal(deltaRaw)
	if err != nil {
		return nil
	}

	var delta map[string]interface{}
	if err := json.Unmarshal(deltaJSON, &delta); err != nil {
		return nil
	}

	return delta
}

// getKeys returns the keys of a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getIntField extracts an integer field from a map, handling float64 conversion.
func (p *ProgressEmitter) getIntField(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}

	val, ok := m[key]
	if !ok {
		return 0
	}

	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	default:
		return 0
	}
}

// handleError processes error events and displays them.
func (p *ProgressEmitter) handleError(event emit.Event) {
	p.hasError = true

	if errorMsg, ok := event.Meta["error"].(string); ok {
		// Clear any progress display
		p.clearProgress()

		// Write error to stderr
		fmt.Fprintf(os.Stderr, "❌ Error in %s: %s\n", event.NodeID, errorMsg)
	}
}

// displayProgress shows current progress for all providers.
func (p *ProgressEmitter) displayProgress() {
	if p.totalBatches == 0 {
		return
	}

	// Clear previous progress if we've already started
	if p.batchesStarted {
		p.clearProgress()
	}
	p.batchesStarted = true

	// Calculate progress
	percentage := float64(p.currentBatch) / float64(p.totalBatches) * 100

	// Display progress for each provider
	for _, providerName := range p.providerNames {
		fmt.Fprintf(p.writer, "  %s: %.0f%% (%d/%d batches)\n",
			providerName, percentage, p.currentBatch, p.totalBatches)
	}

	// Force flush to ensure output is visible immediately
	if f, ok := p.writer.(*os.File); ok {
		f.Sync()
	}
}

// clearProgress clears the progress display lines.
func (p *ProgressEmitter) clearProgress() {
	if !p.batchesStarted {
		return
	}

	// Move up and clear for each provider line
	for range p.providerNames {
		fmt.Fprint(p.writer, "\033[1A\033[2K")
	}
}

// EmitBatch processes multiple events in a single operation.
func (p *ProgressEmitter) EmitBatch(ctx context.Context, events []emit.Event) error {
	for _, event := range events {
		p.Emit(event)
	}
	return nil
}

// Flush ensures all pending output is written.
func (p *ProgressEmitter) Flush(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Leave the final progress on screen
	if !p.hasError && p.batchesStarted {
		fmt.Fprintln(p.writer)
	}

	return nil
}

// FormatProviderList formats provider names for display.
func FormatProviderList(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	if len(names) == 2 {
		return names[0] + " and " + names[1]
	}
	// Oxford comma for 3+
	return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
}
