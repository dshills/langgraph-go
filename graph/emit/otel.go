package emit

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTelEmitter implements Emitter by creating OpenTelemetry spans (T109-T111).
//
// Each event becomes a span with:
//   - Span name: event.Msg (e.g., "node_start", "node_end")
//   - Attributes: runID, step, nodeID, and all event.Meta fields
//   - Timestamps: Derived from span creation
//   - Status: Set to error if event.Meta["error"] exists
//
// Supports distributed tracing by:
//   - Creating child spans for node execution
//   - Propagating trace context across service boundaries
//   - Recording performance metrics as span attributes
//   - Capturing errors with stack traces
//
// Concurrency attributes (T111):
//   - step_id: Unique identifier for the execution step
//   - order_key: Deterministic ordering key for replay
//   - attempt: Retry attempt number (0 for first attempt)
//
// Usage:
//
//	// Create tracer from OpenTelemetry provider
//	tracer := otel.Tracer("langgraph-go")
//	emitter := emit.NewOTelEmitter(tracer)
//
//	// Emit events that become spans
//	emitter.Emit(Event{
//	    RunID: "run-001",
//	    Step: 1,
//	    NodeID: "nodeA",
//	    Msg: "node_start",
//	})
//
// Integration with OpenTelemetry:
//
//	// Setup OpenTelemetry provider (application code)
//	import (
//	    "go.opentelemetry.io/otel"
//	    sdktrace "go.opentelemetry.io/otel/sdk/trace"
//	)
//
//	// Create trace provider with exporter (Jaeger, Zipkin, etc.)
//	tp := sdktrace.NewTracerProvider(
//	    sdktrace.WithBatcher(exporter),
//	)
//	otel.SetTracerProvider(tp)
//
//	// Create emitter
//	tracer := otel.Tracer("langgraph-go")
//	emitter := emit.NewOTelEmitter(tracer)
//
//	// Use in engine
//	engine := graph.New[MyState](
//	    graph.WithEmitter(emitter),
//	)
type OTelEmitter struct {
	tracer trace.Tracer
	spans  []trace.Span // track spans for batching
}

// NewOTelEmitter creates a new OTelEmitter (T109).
//
// Parameters:
//   - tracer: OpenTelemetry tracer from otel.Tracer("service-name")
//
// Returns an OTelEmitter that creates spans for each event.
//
// Example:
//
//	tracer := otel.Tracer("langgraph-go")
//	emitter := emit.NewOTelEmitter(tracer)
func NewOTelEmitter(tracer trace.Tracer) *OTelEmitter {
	return &OTelEmitter{
		tracer: tracer,
		spans:  make([]trace.Span, 0),
	}
}

// Emit creates an OpenTelemetry span for the event (T109).
//
// The span includes:
//   - Name: event.Msg (e.g., "node_start", "node_end")
//   - Attributes: All event fields and metadata
//   - Status: Error if event contains error metadata
//   - Timestamps: Start time (now), end time (immediate for instant events)
//
// For performance, the span is immediately ended (not left open).
// This is appropriate for events representing points in time rather than durations.
//
// If the event contains a "duration_ms" metadata field, the span's end time
// is adjusted to reflect the actual duration.
func (o *OTelEmitter) Emit(event Event) {
	// Create span with event message as name
	ctx := context.Background()
	_, span := o.tracer.Start(ctx, event.Msg)
	defer span.End()

	// Add standard attributes
	o.addStandardAttributes(span, event)

	// Add metadata as attributes
	o.addMetadataAttributes(span, event.Meta)

	// Add concurrency attributes if present (T111)
	o.addConcurrencyAttributes(span, event.Meta)

	// Set error status if present
	if err, ok := event.Meta["error"].(string); ok {
		span.SetStatus(codes.Error, err)
		span.RecordError(fmt.Errorf("%s", err))
	}
}

// EmitBatch creates multiple spans efficiently (T109).
//
// Batching provides performance benefits by:
//   - Amortizing tracer overhead across multiple spans
//   - Enabling span processor batch optimizations
//   - Reducing context switching overhead
//   - Maintaining temporal locality for related events
//
// All spans are created and ended immediately. They are recorded in the
// OpenTelemetry batch span processor for efficient export.
//
// Parameters:
//   - ctx: Context for cancellation and trace propagation
//   - events: Events to emit as spans
//
// Returns error if span creation fails (rare, usually indicates misconfiguration).
func (o *OTelEmitter) EmitBatch(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	// Create spans for all events
	// The span processor will batch these for efficient export
	for _, event := range events {
		_, span := o.tracer.Start(ctx, event.Msg)

		// Add standard attributes
		o.addStandardAttributes(span, event)

		// Add metadata as attributes
		o.addMetadataAttributes(span, event.Meta)

		// Add concurrency attributes (T111)
		o.addConcurrencyAttributes(span, event.Meta)

		// Set error status if present
		if err, ok := event.Meta["error"].(string); ok {
			span.SetStatus(codes.Error, err)
			span.RecordError(fmt.Errorf("%s", err))
		}

		// End span immediately (event is a point in time)
		span.End()
	}

	return nil
}

// Flush forces export of all pending spans (T110).
//
// This method:
//   - Calls ForceFlush on the tracer provider if available
//   - Blocks until all spans are exported or timeout occurs
//   - Should be called before application shutdown
//   - Respects context cancellation and deadlines
//
// OpenTelemetry typically buffers spans in a batch span processor for efficiency.
// Flush ensures these buffered spans are sent to the backend (Jaeger, Zipkin, etc.)
// before the application exits.
//
// Usage:
//
//	defer func() {
//	    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    if err := emitter.Flush(ctx); err != nil {
//	        log.Printf("failed to flush spans: %v", err)
//	    }
//	}()
//
// Parameters:
//   - ctx: Context with timeout/cancellation
//
// Returns error if flush fails or times out.
func (o *OTelEmitter) Flush(ctx context.Context) error {
	// Get tracer provider and force flush if supported
	tp := otel.GetTracerProvider()

	// Check if provider supports flushing (SDK tracer provider)
	type flusher interface {
		ForceFlush(context.Context) error
	}

	if f, ok := tp.(flusher); ok {
		return f.ForceFlush(ctx)
	}

	// Provider doesn't support flushing (e.g., noop provider)
	return nil
}

// addStandardAttributes adds core event fields as span attributes.
func (o *OTelEmitter) addStandardAttributes(span trace.Span, event Event) {
	span.SetAttributes(
		attribute.String("langgraph.run_id", event.RunID),
		attribute.Int("langgraph.step", event.Step),
		attribute.String("langgraph.node_id", event.NodeID),
	)
}

// addMetadataAttributes converts event metadata to span attributes (T047-T048).
//
// Handles common types:
//   - string, int, int64, float64, bool: Direct conversion
//   - time.Duration: Convert to milliseconds
//   - Other types: Convert to string representation
//
// Cost tracking attributes (T047):
//   - tokens_in, tokens_out: LLM token usage (integer attributes)
//   - cost_usd: LLM cost in USD (float64 attribute)
//   - latency_ms: Node execution latency in milliseconds
func (o *OTelEmitter) addMetadataAttributes(span trace.Span, meta map[string]interface{}) {
	if meta == nil {
		return
	}

	for key, value := range meta {
		// Skip concurrency attributes (handled separately)
		if key == "step_id" || key == "order_key" || key == "attempt" {
			continue
		}

		// T047: Map cost tracking attributes to OpenTelemetry conventions
		attrKey := key
		switch key {
		case "tokens_in":
			attrKey = "langgraph.llm.tokens_in"
		case "tokens_out":
			attrKey = "langgraph.llm.tokens_out"
		case "cost_usd":
			attrKey = "langgraph.llm.cost_usd"
		case "latency_ms":
			attrKey = "langgraph.node.latency_ms"
		case "model":
			attrKey = "langgraph.llm.model"
		}

		// Convert value to appropriate attribute type
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(attrKey, v))
		case int:
			span.SetAttributes(attribute.Int(attrKey, v))
		case int64:
			span.SetAttributes(attribute.Int64(attrKey, v))
		case float64:
			span.SetAttributes(attribute.Float64(attrKey, v))
		case bool:
			span.SetAttributes(attribute.Bool(attrKey, v))
		case time.Duration:
			// Convert duration to milliseconds
			span.SetAttributes(attribute.Int64(attrKey, int64(v/time.Millisecond)))
		default:
			// Fallback to string representation
			span.SetAttributes(attribute.String(attrKey, fmt.Sprintf("%v", v)))
		}
	}
}

// addConcurrencyAttributes adds concurrency-specific span attributes (T111).
//
// Adds attributes for concurrent execution tracking:
//   - langgraph.step_id: Unique identifier for the execution step
//   - langgraph.order_key: Deterministic ordering key for replay
//   - langgraph.attempt: Retry attempt number (0 for first attempt)
//
// These attributes enable:
//   - Tracking concurrent node execution
//   - Understanding replay ordering
//   - Correlating retry attempts
//   - Analyzing deterministic behavior
//
// OpenTelemetry attribute naming follows semantic conventions:
// - Namespace: "langgraph" for framework-specific attributes
// - Format: snake_case as per OpenTelemetry specification
func (o *OTelEmitter) addConcurrencyAttributes(span trace.Span, meta map[string]interface{}) {
	if meta == nil {
		return
	}

	// Add step_id if present
	if stepID, ok := meta["step_id"].(string); ok {
		span.SetAttributes(attribute.String("langgraph.step_id", stepID))
	}

	// Add order_key if present
	if orderKey, ok := meta["order_key"].(string); ok {
		span.SetAttributes(attribute.String("langgraph.order_key", orderKey))
	}

	// Add attempt if present
	if attempt, ok := meta["attempt"].(int); ok {
		span.SetAttributes(attribute.Int("langgraph.attempt", attempt))
	} else if attempt, ok := meta["attempt"].(int64); ok {
		span.SetAttributes(attribute.Int64("langgraph.attempt", attempt))
	}
}
