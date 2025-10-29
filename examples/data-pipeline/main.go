package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/dshills/langgraph-go/graph"
	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// PipelineState represents data processing pipeline state
type PipelineState struct {
	BatchID          string
	Records          []string
	ProcessedRecords []string
	FailedRecords    []string
	RetryCount       int
	MaxRetries       int
	ValidationErrors []string
	TransformErrors  []string
	LoadErrors       []string
	Success          bool
}

func reducer(prev, delta PipelineState) PipelineState {
	if len(delta.ProcessedRecords) > 0 {
		prev.ProcessedRecords = append(prev.ProcessedRecords, delta.ProcessedRecords...)
	}
	if len(delta.FailedRecords) > 0 {
		prev.FailedRecords = append(prev.FailedRecords, delta.FailedRecords...)
	}
	if delta.RetryCount > 0 {
		prev.RetryCount = delta.RetryCount
	}
	if len(delta.ValidationErrors) > 0 {
		prev.ValidationErrors = append(prev.ValidationErrors, delta.ValidationErrors...)
	}
	if len(delta.TransformErrors) > 0 {
		prev.TransformErrors = append(prev.TransformErrors, delta.TransformErrors...)
	}
	if len(delta.LoadErrors) > 0 {
		prev.LoadErrors = append(prev.LoadErrors, delta.LoadErrors...)
	}
	if delta.Success {
		prev.Success = true
	}
	return prev
}

func main() {
	fmt.Println("=== Data Processing Pipeline with Retry Logic ===")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("• ETL pipeline with extract, transform, and load stages")
	fmt.Println("• Retry logic with exponential backoff")
	fmt.Println("• Error handling and failure tracking")
	fmt.Println("• Batch processing with partial failure recovery")
	fmt.Println("• State checkpointing for resumable processing")
	fmt.Println()

	rand.Seed(time.Now().UnixNano())

	st := store.NewMemStore[PipelineState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)
	opts := graph.Options{MaxSteps: 25}
	engine := graph.New(reducer, st, emitter, opts)

	// Node 1: Extract data
	if err := engine.Add("extract", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Printf("📥 Extracting batch: %s (%d records)\n", state.BatchID, len(state.Records))

		// Simulate extraction with occasional failure
		//nolint:gosec // G404: demo code, simulating random failures
		if rand.Float32() < 0.1 && state.RetryCount == 0 {
			fmt.Println("❌ Extraction failed (simulated)")
			return graph.NodeResult[PipelineState]{
				Delta: PipelineState{RetryCount: state.RetryCount + 1},
				Route: graph.Goto("handle_error"),
			}
		}

		fmt.Println("✅ Extraction successful")
		return graph.NodeResult[PipelineState]{
			Route: graph.Goto("validate"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add extract node: %v\n", err)
		os.Exit(1)
	}

	// Node 2: Validate data
	if err := engine.Add("validate", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Println("🔍 Validating records...")

		var validationErrors []string
		validCount := 0

		for i, record := range state.Records {
			// Simulate validation (10% failure rate)
			//nolint:gosec // G404: demo code, simulating random validation failures
			if rand.Float32() < 0.15 {
				err := fmt.Sprintf("Record %d: %s - validation failed", i, record)
				validationErrors = append(validationErrors, err)
			} else {
				validCount++
			}
		}

		if len(validationErrors) > 0 {
			fmt.Printf("⚠️  %d records failed validation\n", len(validationErrors))
		}

		fmt.Printf("✅ %d/%d records validated\n", validCount, len(state.Records))

		return graph.NodeResult[PipelineState]{
			Delta: PipelineState{
				ValidationErrors: validationErrors,
			},
			Route: graph.Goto("transform"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add validate node: %v\n", err)
		os.Exit(1)
	}

	// Node 3: Transform data
	if err := engine.Add("transform", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Println("🔄 Transforming records...")

		// Simulate transformation with occasional transient failure
		//nolint:gosec // G404: demo code, simulating random transformation failures
		if rand.Float32() < 0.05 {
			fmt.Println("❌ Transformation failed (transient error)")
			return graph.NodeResult[PipelineState]{
				Delta: PipelineState{
					RetryCount:      state.RetryCount + 1,
					TransformErrors: []string{"Transient transformation error"},
				},
				Route: graph.Goto("handle_error"),
			}
		}

		processed := make([]string, 0, len(state.Records))
		for _, record := range state.Records {
			// Transform all records (skip already filtered by validation)
			processed = append(processed, fmt.Sprintf("TRANSFORMED_%s", record))
		}

		fmt.Printf("✅ %d records transformed\n", len(processed))

		return graph.NodeResult[PipelineState]{
			Delta: PipelineState{
				ProcessedRecords: processed,
			},
			Route: graph.Goto("load"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add transform node: %v\n", err)
		os.Exit(1)
	}

	// Node 4: Load data
	if err := engine.Add("load", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Println("💾 Loading records to destination...")

		// Simulate load with occasional failure
		//nolint:gosec // G404: demo code, simulating random load failures
		if rand.Float32() < 0.08 {
			fmt.Println("❌ Load failed (database connection error)")
			return graph.NodeResult[PipelineState]{
				Delta: PipelineState{
					RetryCount: state.RetryCount + 1,
					LoadErrors: []string{"Database connection timeout"},
				},
				Route: graph.Goto("handle_error"),
			}
		}

		fmt.Printf("✅ %d records loaded successfully\n", len(state.ProcessedRecords))

		return graph.NodeResult[PipelineState]{
			Delta: PipelineState{Success: true},
			Route: graph.Goto("complete"),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add load node: %v\n", err)
		os.Exit(1)
	}

	// Node 5: Error handling with retry logic
	if err := engine.Add("handle_error", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Printf("⚠️  Error occurred. Retry attempt %d/%d\n", state.RetryCount, state.MaxRetries)

		if state.RetryCount >= state.MaxRetries {
			fmt.Println("❌ Max retries exceeded. Marking batch as failed.")
			return graph.NodeResult[PipelineState]{
				Route: graph.Goto("complete"),
			}
		}

		// Exponential backoff
		backoffMs := (1 << state.RetryCount) * 100
		fmt.Printf("⏳ Waiting %dms before retry...\n", backoffMs)
		time.Sleep(time.Duration(backoffMs) * time.Millisecond)

		// Determine where to retry from based on the error
		var retryNode string
		if len(state.LoadErrors) > 0 {
			retryNode = "load"
		} else if len(state.TransformErrors) > 0 {
			retryNode = "transform"
		} else {
			retryNode = "extract"
		}

		fmt.Printf("🔄 Retrying from: %s\n", retryNode)

		return graph.NodeResult[PipelineState]{
			Route: graph.Goto(retryNode),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add handle_error node: %v\n", err)
		os.Exit(1)
	}

	// Node 6: Complete pipeline
	if err := engine.Add("complete", graph.NodeFunc[PipelineState](func(ctx context.Context, state PipelineState) graph.NodeResult[PipelineState] {
		fmt.Println()
		fmt.Println("📊 Pipeline Execution Summary:")
		fmt.Printf("   Total Records: %d\n", len(state.Records))
		fmt.Printf("   Processed: %d\n", len(state.ProcessedRecords))
		fmt.Printf("   Failed: %d\n", len(state.FailedRecords))
		fmt.Printf("   Validation Errors: %d\n", len(state.ValidationErrors))
		fmt.Printf("   Retry Attempts: %d\n", state.RetryCount)
		fmt.Printf("   Success: %v\n", state.Success)

		return graph.NodeResult[PipelineState]{
			Route: graph.Stop(),
		}
	})); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add complete node: %v\n", err)
		os.Exit(1)
	}

	if err := engine.StartAt("extract"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set start node to extract: %v\n", err)
		os.Exit(1)
	}

	// Process multiple batches
	batches := []struct {
		id      string
		records []string
	}{
		{"batch-001", []string{"record1", "record2", "record3", "record4", "record5"}},
		{"batch-002", []string{"recordA", "recordB", "recordC"}},
		{"batch-003", []string{"data1", "data2", "data3", "data4"}},
	}

	fmt.Println("Processing data batches...")
	fmt.Println()

	successCount := 0
	for _, batch := range batches {
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("Processing Batch: %s\n", batch.id)
		fmt.Println("═══════════════════════════════════════════════════════════")

		ctx := context.Background()
		initialState := PipelineState{
			BatchID:    batch.id,
			Records:    batch.records,
			MaxRetries: 3,
		}

		final, err := engine.Run(ctx, batch.id, initialState)
		if err != nil {
			fmt.Printf("❌ Batch processing failed: %v\n", err)
			continue
		}

		if final.Success {
			successCount++
		}

		fmt.Println()
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("✅ Processed %d/%d batches successfully\n", successCount, len(batches))
	fmt.Println("═══════════════════════════════════════════════════════════")
}
