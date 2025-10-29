package scanner

import "github.com/dshills/langgraph-go/examples/multi-llm-review/workflow"

// ScannerAdapter adapts scanner.Scanner to the workflow.FileScanner interface.
type ScannerAdapter struct {
	Scanner *Scanner
}

// Discover implements workflow.FileScanner by calling the underlying scanner and converting types.
func (a *ScannerAdapter) Discover(rootPath string) ([]workflow.DiscoveredFile, error) {
	// Call the underlying scanner
	files, err := a.Scanner.Discover(rootPath)
	if err != nil {
		return nil, err
	}

	// Convert scanner.CodeFile to workflow.DiscoveredFile
	result := make([]workflow.DiscoveredFile, len(files))
	for i, file := range files {
		result[i] = workflow.DiscoveredFile{
			Path:     file.Path,
			Content:  file.Content,
			Size:     file.Size,
			Checksum: file.Checksum,
		}
	}

	return result, nil
}
