package scanner

import (
	"testing"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/workflow"
)

func TestCreateBatches(t *testing.T) {
	tests := []struct {
		name          string
		files         []workflow.CodeFile
		batchSize     int
		wantBatches   int
		validateFirst func(t *testing.T, batch workflow.Batch)
		validateLast  func(t *testing.T, batch workflow.Batch)
	}{
		{
			name:        "split 100 files into batches of 20",
			files:       generateTestFiles(100, 10), // 100 files with 10 lines each
			batchSize:   20,
			wantBatches: 5,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 1 {
					t.Errorf("First batch BatchNumber = %d, want 1", batch.BatchNumber)
				}
				if len(batch.Files) != 20 {
					t.Errorf("First batch Files length = %d, want 20", len(batch.Files))
				}
				if batch.TotalLines != 200 { // 20 files * 10 lines
					t.Errorf("First batch TotalLines = %d, want 200", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("First batch Status = %q, want %q", batch.Status, "pending")
				}
			},
			validateLast: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 5 {
					t.Errorf("Last batch BatchNumber = %d, want 5", batch.BatchNumber)
				}
				if len(batch.Files) != 20 {
					t.Errorf("Last batch Files length = %d, want 20", len(batch.Files))
				}
				if batch.TotalLines != 200 { // 20 files * 10 lines
					t.Errorf("Last batch TotalLines = %d, want 200", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("Last batch Status = %q, want %q", batch.Status, "pending")
				}
			},
		},
		{
			name:        "split 25 files into batches of 20",
			files:       generateTestFiles(25, 15), // 25 files with 15 lines each
			batchSize:   20,
			wantBatches: 2,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 1 {
					t.Errorf("First batch BatchNumber = %d, want 1", batch.BatchNumber)
				}
				if len(batch.Files) != 20 {
					t.Errorf("First batch Files length = %d, want 20", len(batch.Files))
				}
				if batch.TotalLines != 300 { // 20 files * 15 lines
					t.Errorf("First batch TotalLines = %d, want 300", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("First batch Status = %q, want %q", batch.Status, "pending")
				}
			},
			validateLast: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 2 {
					t.Errorf("Last batch BatchNumber = %d, want 2", batch.BatchNumber)
				}
				if len(batch.Files) != 5 {
					t.Errorf("Last batch Files length = %d, want 5", len(batch.Files))
				}
				if batch.TotalLines != 75 { // 5 files * 15 lines
					t.Errorf("Last batch TotalLines = %d, want 75", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("Last batch Status = %q, want %q", batch.Status, "pending")
				}
			},
		},
		{
			name:        "batch size larger than file count",
			files:       generateTestFiles(5, 20), // 5 files with 20 lines each
			batchSize:   50,
			wantBatches: 1,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 1 {
					t.Errorf("BatchNumber = %d, want 1", batch.BatchNumber)
				}
				if len(batch.Files) != 5 {
					t.Errorf("Files length = %d, want 5", len(batch.Files))
				}
				if batch.TotalLines != 100 { // 5 files * 20 lines
					t.Errorf("TotalLines = %d, want 100", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("Status = %q, want %q", batch.Status, "pending")
				}
			},
			validateLast: nil, // Same as validateFirst
		},
		{
			name:        "empty file list",
			files:       []workflow.CodeFile{},
			batchSize:   20,
			wantBatches: 0,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				t.Error("Expected no batches but got at least one")
			},
			validateLast: nil,
		},
		{
			name:        "single file",
			files:       generateTestFiles(1, 50),
			batchSize:   20,
			wantBatches: 1,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 1 {
					t.Errorf("BatchNumber = %d, want 1", batch.BatchNumber)
				}
				if len(batch.Files) != 1 {
					t.Errorf("Files length = %d, want 1", len(batch.Files))
				}
				if batch.TotalLines != 50 {
					t.Errorf("TotalLines = %d, want 50", batch.TotalLines)
				}
				if batch.Status != "pending" {
					t.Errorf("Status = %q, want %q", batch.Status, "pending")
				}
			},
			validateLast: nil,
		},
		{
			name:        "files with varying line counts",
			files:       generateTestFilesVaryingLines([]int{10, 20, 30, 5, 15, 25}),
			batchSize:   3,
			wantBatches: 2,
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 1 {
					t.Errorf("First batch BatchNumber = %d, want 1", batch.BatchNumber)
				}
				if len(batch.Files) != 3 {
					t.Errorf("First batch Files length = %d, want 3", len(batch.Files))
				}
				// First 3 files: 10 + 20 + 30 = 60 lines
				if batch.TotalLines != 60 {
					t.Errorf("First batch TotalLines = %d, want 60", batch.TotalLines)
				}
			},
			validateLast: func(t *testing.T, batch workflow.Batch) {
				if batch.BatchNumber != 2 {
					t.Errorf("Last batch BatchNumber = %d, want 2", batch.BatchNumber)
				}
				if len(batch.Files) != 3 {
					t.Errorf("Last batch Files length = %d, want 3", len(batch.Files))
				}
				// Last 3 files: 5 + 15 + 25 = 45 lines
				if batch.TotalLines != 45 {
					t.Errorf("Last batch TotalLines = %d, want 45", batch.TotalLines)
				}
			},
		},
		{
			name:        "zero batch size should cause panic or error",
			files:       generateTestFiles(10, 10),
			batchSize:   0,
			wantBatches: 0, // Should not create batches or should panic
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				t.Error("Expected no batches with zero batchSize but got at least one")
			},
			validateLast: nil,
		},
		{
			name:        "negative batch size should cause panic or error",
			files:       generateTestFiles(10, 10),
			batchSize:   -5,
			wantBatches: 0, // Should not create batches or should panic
			validateFirst: func(t *testing.T, batch workflow.Batch) {
				t.Error("Expected no batches with negative batchSize but got at least one")
			},
			validateLast: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := CreateBatches(tt.files, tt.batchSize)

			if len(batches) != tt.wantBatches {
				t.Errorf("CreateBatches() returned %d batches, want %d", len(batches), tt.wantBatches)
			}

			if len(batches) == 0 {
				return // Empty case, nothing more to validate
			}

			// Validate first batch
			if tt.validateFirst != nil {
				tt.validateFirst(t, batches[0])
			}

			// Validate last batch if different from first
			if tt.validateLast != nil && len(batches) > 1 {
				tt.validateLast(t, batches[len(batches)-1])
			}

			// Validate that all batches have sequential batch numbers starting from 1
			for i, batch := range batches {
				expectedBatchNum := i + 1
				if batch.BatchNumber != expectedBatchNum {
					t.Errorf("Batch %d has BatchNumber = %d, want %d", i, batch.BatchNumber, expectedBatchNum)
				}

				// Validate that all batches have status "pending"
				if batch.Status != "pending" {
					t.Errorf("Batch %d has Status = %q, want %q", i, batch.Status, "pending")
				}

				// Validate that TotalLines matches sum of file line counts
				expectedTotal := 0
				for _, file := range batch.Files {
					expectedTotal += file.LineCount
				}
				if batch.TotalLines != expectedTotal {
					t.Errorf("Batch %d TotalLines = %d, want %d (sum of file line counts)", i, batch.TotalLines, expectedTotal)
				}
			}

			// Validate that all original files are present across batches
			totalFiles := 0
			for _, batch := range batches {
				totalFiles += len(batch.Files)
			}
			if totalFiles != len(tt.files) {
				t.Errorf("Total files across batches = %d, want %d", totalFiles, len(tt.files))
			}
		})
	}
}

// generateTestFiles creates n test files with the specified number of lines each.
func generateTestFiles(n, linesPerFile int) []workflow.CodeFile {
	files := make([]workflow.CodeFile, n)
	for i := 0; i < n; i++ {
		files[i] = workflow.CodeFile{
			FilePath:  "test_" + string(rune('a'+i%26)) + ".go",
			Content:   "// test content",
			Language:  "go",
			LineCount: linesPerFile,
			SizeBytes: int64(linesPerFile * 20), // Approximate
			Checksum:  "abc123def456",
		}
	}
	return files
}

// generateTestFilesVaryingLines creates test files with specified line counts.
func generateTestFilesVaryingLines(lineCounts []int) []workflow.CodeFile {
	files := make([]workflow.CodeFile, len(lineCounts))
	for i, lineCount := range lineCounts {
		files[i] = workflow.CodeFile{
			FilePath:  "test_" + string(rune('a'+i)) + ".go",
			Content:   "// test content",
			Language:  "go",
			LineCount: lineCount,
			SizeBytes: int64(lineCount * 20), // Approximate
			Checksum:  "abc123def456",
		}
	}
	return files
}
