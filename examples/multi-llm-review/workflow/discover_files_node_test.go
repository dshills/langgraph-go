package workflow

import (
	"context"
	"testing"
)

// mockFileScanner is a test double for FileScanner interface.
type mockFileScanner struct {
	files []DiscoveredFile
	err   error
}

func (m *mockFileScanner) Discover(rootPath string) ([]DiscoveredFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.files, nil
}

func TestDiscoverFilesNode_BasicFunctionality(t *testing.T) {
	// Create mock scanner with test files
	mockFiles := []DiscoveredFile{
		{Path: "/test/file1.go", Content: "package main\n\nfunc main() {}\n", Size: 30, Checksum: "abc123"},
		{Path: "/test/file2.go", Content: "package test\n\nfunc Test() {}\n", Size: 32, Checksum: "def456"},
		{Path: "/test/file3.go", Content: "package util\n\nfunc Util() {}\n", Size: 31, Checksum: "ghi789"},
	}

	scanner := &mockFileScanner{files: mockFiles}

	// Create node with batch size of 2
	node := &DiscoverFilesNode{
		Scanner:   scanner,
		BatchSize: 2,
	}

	// Create initial state
	state := ReviewState{
		CodebaseRoot: "/test",
	}

	// Execute node
	result := node.Run(context.Background(), state)

	// Verify no error
	if result.Err != nil {
		t.Fatalf("DiscoverFilesNode.Run() error = %v, want nil", result.Err)
	}

	// Verify discovered files
	if len(result.Delta.DiscoveredFiles) != 3 {
		t.Errorf("len(DiscoveredFiles) = %d, want 3", len(result.Delta.DiscoveredFiles))
	}

	// Verify batches were created (3 files with batch size 2 = 2 batches)
	if len(result.Delta.Batches) != 2 {
		t.Errorf("len(Batches) = %d, want 2", len(result.Delta.Batches))
	}

	// Verify total batches
	if result.Delta.TotalBatches != 2 {
		t.Errorf("TotalBatches = %d, want 2", result.Delta.TotalBatches)
	}

	// Verify current batch is set to 1
	if result.Delta.CurrentBatch != 1 {
		t.Errorf("CurrentBatch = %d, want 1", result.Delta.CurrentBatch)
	}

	// Verify route
	if result.Route.To != "review-batch" {
		t.Errorf("Route.To = %q, want 'review-batch'", result.Route.To)
	}

	// Verify first batch has 2 files
	if len(result.Delta.Batches[0].Files) != 2 {
		t.Errorf("Batch[0] files = %d, want 2", len(result.Delta.Batches[0].Files))
	}

	// Verify second batch has 1 file
	if len(result.Delta.Batches[1].Files) != 1 {
		t.Errorf("Batch[1] files = %d, want 1", len(result.Delta.Batches[1].Files))
	}

	// Verify batch numbers are 1-indexed
	if result.Delta.Batches[0].BatchNumber != 1 {
		t.Errorf("Batch[0].BatchNumber = %d, want 1", result.Delta.Batches[0].BatchNumber)
	}
	if result.Delta.Batches[1].BatchNumber != 2 {
		t.Errorf("Batch[1].BatchNumber = %d, want 2", result.Delta.Batches[1].BatchNumber)
	}
}

func TestDiscoverFilesNode_ValidatesScanner(t *testing.T) {
	node := &DiscoverFilesNode{
		Scanner:   nil, // Invalid
		BatchSize: 10,
	}

	state := ReviewState{CodebaseRoot: "/test"}

	result := node.Run(context.Background(), state)

	if result.Err == nil {
		t.Error("expected error when Scanner is nil, got nil")
	}
}

func TestDiscoverFilesNode_ValidatesCodebaseRoot(t *testing.T) {
	scanner := &mockFileScanner{files: []DiscoveredFile{}}
	node := &DiscoverFilesNode{
		Scanner:   scanner,
		BatchSize: 10,
	}

	state := ReviewState{CodebaseRoot: ""} // Invalid

	result := node.Run(context.Background(), state)

	if result.Err == nil {
		t.Error("expected error when CodebaseRoot is empty, got nil")
	}
}

func TestDiscoverFilesNode_ValidatesBatchSize(t *testing.T) {
	scanner := &mockFileScanner{files: []DiscoveredFile{}}
	node := &DiscoverFilesNode{
		Scanner:   scanner,
		BatchSize: 0, // Invalid
	}

	state := ReviewState{CodebaseRoot: "/test"}

	result := node.Run(context.Background(), state)

	if result.Err == nil {
		t.Error("expected error when BatchSize is 0, got nil")
	}
}

func TestDiscoverFilesNode_LanguageDetection(t *testing.T) {
	tests := []struct {
		fileName string
		wantLang string
	}{
		{"/test/main.go", "go"},
		{"/test/script.py", "python"},
		{"/test/app.js", "javascript"},
		{"/test/types.ts", "typescript"},
		{"/test/Main.java", "java"},
		{"/test/unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			mockFiles := []DiscoveredFile{
				{Path: tt.fileName, Content: "test content\n", Size: 13, Checksum: "abc"},
			}

			scanner := &mockFileScanner{files: mockFiles}
			node := &DiscoverFilesNode{Scanner: scanner, BatchSize: 10}
			state := ReviewState{CodebaseRoot: "/test"}

			result := node.Run(context.Background(), state)

			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}

			if len(result.Delta.DiscoveredFiles) == 0 {
				t.Fatal("no files discovered")
			}

			got := result.Delta.DiscoveredFiles[0].Language
			if got != tt.wantLang {
				t.Errorf("Language = %q, want %q", got, tt.wantLang)
			}
		})
	}
}

func TestDiscoverFilesNode_LineCountCalculation(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantLines int
	}{
		{"empty file", "", 0},
		{"single line no newline", "package main", 1},
		{"single line with newline", "package main\n", 2},
		{"three lines", "line1\nline2\nline3\n", 4},
		{"multiple newlines", "a\n\nb\n\nc\n", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFiles := []DiscoveredFile{
				{Path: "/test/file.go", Content: tt.content, Size: int64(len(tt.content)), Checksum: "abc"},
			}

			scanner := &mockFileScanner{files: mockFiles}
			node := &DiscoverFilesNode{Scanner: scanner, BatchSize: 10}
			state := ReviewState{CodebaseRoot: "/test"}

			result := node.Run(context.Background(), state)

			if result.Err != nil {
				t.Fatalf("unexpected error: %v", result.Err)
			}

			if len(result.Delta.DiscoveredFiles) == 0 {
				t.Fatal("no files discovered")
			}

			got := result.Delta.DiscoveredFiles[0].LineCount
			if got != tt.wantLines {
				t.Errorf("LineCount = %d, want %d", got, tt.wantLines)
			}
		})
	}
}
