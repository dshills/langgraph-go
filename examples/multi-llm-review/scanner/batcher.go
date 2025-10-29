package scanner

import "github.com/dshills/langgraph-go/examples/multi-llm-review/workflow"

// CreateBatches splits a list of CodeFiles into batches of the specified size.
// Each batch is assigned a sequential batch number (1-indexed), has its TotalLines
// calculated from the sum of file line counts, and is initialized with status "pending".
//
// Parameters:
//   - files: The list of CodeFiles to split into batches
//   - batchSize: The maximum number of files per batch
//
// Returns:
//   - A slice of Batch structs, each containing up to batchSize files
//
// Behavior:
//   - If files is empty, returns an empty slice
//   - If batchSize is greater than the number of files, returns a single batch
//   - The last batch may contain fewer than batchSize files
//   - Batch numbers are 1-indexed (first batch is BatchNumber 1)
//   - All batches are initialized with Status "pending"
//   - TotalLines is calculated as the sum of LineCount for all files in the batch
//
// Example:
//
//	files := []workflow.CodeFile{...} // 100 files
//	batches := CreateBatches(files, 20)
//	// Returns 5 batches, each with 20 files
//	// batches[0].BatchNumber == 1
//	// batches[4].BatchNumber == 5
func CreateBatches(files []workflow.CodeFile, batchSize int) []workflow.Batch {
	// Handle empty file list
	if len(files) == 0 {
		return []workflow.Batch{}
	}

	// Calculate number of batches needed
	numBatches := (len(files) + batchSize - 1) / batchSize
	batches := make([]workflow.Batch, 0, numBatches)

	// Split files into batches
	for i := 0; i < len(files); i += batchSize {
		// Calculate end index for this batch
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		// Extract files for this batch
		batchFiles := files[i:end]

		// Calculate total lines for this batch
		totalLines := 0
		for _, file := range batchFiles {
			totalLines += file.LineCount
		}

		// Create batch with 1-indexed batch number
		batch := workflow.Batch{
			BatchNumber: len(batches) + 1,
			Files:       batchFiles,
			TotalLines:  totalLines,
			Status:      "pending",
		}

		batches = append(batches, batch)
	}

	return batches
}
