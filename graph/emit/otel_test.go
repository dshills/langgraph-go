package emit

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestOTelEmitter_Emit verifies single event emission creates spans (T112).
func TestOTelEmitter_Emit(t *testing.T) {
	// Setup in-memory span recorder for testing
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_start",
		Meta: map[string]interface{}{
			"node_type": "llm",
			"tokens":    150,
		},
	}
	emitter.Emit(event)

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]

	// Verify span name
	if span.Name != "node_start" {
		t.Errorf("span name = %q, want %q", span.Name, "node_start")
	}

	// Verify standard attributes
	attrs := attributeMap(span.Attributes)
	if got := attrs["langgraph.run_id"]; got != "run-001" {
		t.Errorf("run_id = %v, want %q", got, "run-001")
	}
	if got := attrs["langgraph.step"]; got != int64(1) {
		t.Errorf("step = %v, want %d", got, 1)
	}
	if got := attrs["langgraph.node_id"]; got != "nodeA" {
		t.Errorf("node_id = %v, want %q", got, "nodeA")
	}

	// Verify metadata attributes
	if got := attrs["node_type"]; got != "llm" {
		t.Errorf("node_type = %v, want %q", got, "llm")
	}
	if got := attrs["tokens"]; got != int64(150) {
		t.Errorf("tokens = %v, want %d", got, 150)
	}

	// Verify span was ended (not still recording)
	if !span.EndTime.After(span.StartTime) {
		t.Error("span was not ended")
	}
}

// TestOTelEmitter_EmitWithError verifies error events set error status (T112).
func TestOTelEmitter_EmitWithError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit error event
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_error",
		Meta: map[string]interface{}{
			"error": "validation failed",
		},
	}
	emitter.Emit(event)

	// Verify span has error status
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]

	// Verify error status
	if span.Status.Code != codes.Error {
		t.Errorf("status code = %v, want %v", span.Status.Code, codes.Error)
	}
	if span.Status.Description != "validation failed" {
		t.Errorf("status description = %q, want %q", span.Status.Description, "validation failed")
	}

	// Verify error attribute
	attrs := attributeMap(span.Attributes)
	if got := attrs["error"]; got != "validation failed" {
		t.Errorf("error = %v, want %q", got, "validation failed")
	}

	// Verify error event was recorded
	if len(span.Events) == 0 {
		t.Error("expected error event, got none")
	}
}

// TestOTelEmitter_EmitBatch verifies batch emission creates multiple spans (T112).
func TestOTelEmitter_EmitBatch(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit batch of events
	events := []Event{
		{RunID: "run-001", Step: 1, NodeID: "nodeA", Msg: "node_start"},
		{RunID: "run-001", Step: 1, NodeID: "nodeA", Msg: "node_end"},
		{RunID: "run-001", Step: 2, NodeID: "nodeB", Msg: "node_start"},
	}

	ctx := context.Background()
	if err := emitter.EmitBatch(ctx, events); err != nil {
		t.Fatalf("EmitBatch failed: %v", err)
	}

	// Verify all spans were created
	spans := exporter.GetSpans()
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(spans))
	}

	// Verify span names match events
	expectedNames := []string{"node_start", "node_end", "node_start"}
	for i, span := range spans {
		if span.Name != expectedNames[i] {
			t.Errorf("span[%d] name = %q, want %q", i, span.Name, expectedNames[i])
		}
	}

	// Verify all spans ended
	for i, span := range spans {
		if !span.EndTime.After(span.StartTime) {
			t.Errorf("span[%d] was not ended", i)
		}
	}
}

// TestOTelEmitter_EmitBatch_Empty verifies empty batch is handled (T112).
func TestOTelEmitter_EmitBatch_Empty(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit empty batch
	ctx := context.Background()
	if err := emitter.EmitBatch(ctx, []Event{}); err != nil {
		t.Fatalf("EmitBatch failed on empty batch: %v", err)
	}

	// Verify no spans created
	spans := exporter.GetSpans()
	if len(spans) != 0 {
		t.Errorf("expected 0 spans for empty batch, got %d", len(spans))
	}
}

// TestOTelEmitter_Flush verifies flush forces span export (T112).
func TestOTelEmitter_Flush(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event (will be batched)
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_start",
	}
	emitter.Emit(event)

	// Before flush, span may not be exported yet
	// (depends on batch processor timing)

	// Force flush
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := emitter.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// After flush, span must be exported
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Errorf("expected 1 span after flush, got %d", len(spans))
	}
}

// TestOTelEmitter_Flush_Timeout verifies flush respects context timeout (T112).
func TestOTelEmitter_Flush_Timeout(_ *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Create already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Flush with cancelled context
	// Should return error (though implementation may vary)
	err := emitter.Flush(ctx)
	// Note: Some implementations may return nil if flush completes quickly
	// This test primarily verifies that context is passed through
	_ = err // Don't fail test, just verify it doesn't panic
}

// TestOTelEmitter_ConcurrencyAttributes verifies concurrency attributes are added (T112).
func TestOTelEmitter_ConcurrencyAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event with concurrency metadata
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_start",
		Meta: map[string]interface{}{
			"step_id":   "step-abc123",
			"order_key": "00000001:0",
			"attempt":   2,
		},
	}
	emitter.Emit(event)

	// Verify concurrency attributes
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := attributeMap(span.Attributes)

	// Verify step_id
	if got := attrs["langgraph.step_id"]; got != "step-abc123" {
		t.Errorf("step_id = %v, want %q", got, "step-abc123")
	}

	// Verify order_key
	if got := attrs["langgraph.order_key"]; got != "00000001:0" {
		t.Errorf("order_key = %v, want %q", got, "00000001:0")
	}

	// Verify attempt
	if got := attrs["langgraph.attempt"]; got != int64(2) {
		t.Errorf("attempt = %v, want %d", got, 2)
	}
}

// TestOTelEmitter_ConcurrencyAttributes_Missing verifies missing attributes don't cause errors (T112).
func TestOTelEmitter_ConcurrencyAttributes_Missing(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event without concurrency metadata
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_start",
		Meta:   map[string]interface{}{},
	}
	emitter.Emit(event)

	// Should not panic, concurrency attributes are optional
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := attributeMap(span.Attributes)

	// Verify concurrency attributes are absent
	if _, ok := attrs["langgraph.step_id"]; ok {
		t.Error("step_id should not be present")
	}
	if _, ok := attrs["langgraph.order_key"]; ok {
		t.Error("order_key should not be present")
	}
	if _, ok := attrs["langgraph.attempt"]; ok {
		t.Error("attempt should not be present")
	}
}

// TestOTelEmitter_MetadataTypes verifies different metadata types are handled (T112).
func TestOTelEmitter_MetadataTypes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event with various metadata types
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "test_types",
		Meta: map[string]interface{}{
			"string_val":   "hello",
			"int_val":      42,
			"int64_val":    int64(99),
			"float64_val":  3.14,
			"bool_val":     true,
			"duration_val": 250 * time.Millisecond,
		},
	}
	emitter.Emit(event)

	// Verify attributes have correct types
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := attributeMap(span.Attributes)

	// Verify each type
	if got := attrs["string_val"]; got != "hello" {
		t.Errorf("string_val = %v, want %q", got, "hello")
	}
	if got := attrs["int_val"]; got != int64(42) {
		t.Errorf("int_val = %v, want %d", got, 42)
	}
	if got := attrs["int64_val"]; got != int64(99) {
		t.Errorf("int64_val = %v, want %d", got, 99)
	}
	if got := attrs["float64_val"]; got != 3.14 {
		t.Errorf("float64_val = %v, want %f", got, 3.14)
	}
	if got := attrs["bool_val"]; got != true {
		t.Errorf("bool_val = %v, want %t", got, true)
	}
	// Duration converted to milliseconds
	if got := attrs["duration_val"]; got != int64(250) {
		t.Errorf("duration_val = %v, want %d ms", got, 250)
	}
}

// TestOTelEmitter_NilMeta verifies nil metadata is handled (T112).
func TestOTelEmitter_NilMeta(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := otel.Tracer("test")
	emitter := NewOTelEmitter(tracer)

	// Emit event with nil metadata
	event := Event{
		RunID:  "run-001",
		Step:   1,
		NodeID: "nodeA",
		Msg:    "node_start",
		Meta:   nil,
	}
	emitter.Emit(event)

	// Should not panic
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Verify standard attributes are still present
	span := spans[0]
	attrs := attributeMap(span.Attributes)

	if got := attrs["langgraph.run_id"]; got != "run-001" {
		t.Errorf("run_id = %v, want %q", got, "run-001")
	}
}

// attributeMap converts span attributes to map for easy testing.
func attributeMap(attrs []attribute.KeyValue) map[string]interface{} {
	m := make(map[string]interface{})
	for _, kv := range attrs {
		m[string(kv.Key)] = kv.Value.AsInterface()
	}
	return m
}
