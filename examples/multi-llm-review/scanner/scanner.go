package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodeFile represents a single source code file with metadata and content.
type CodeFile struct {
	Path     string // Absolute path to the file
	Content  string // File content as text
	Size     int64  // File size in bytes
	Checksum string // SHA-256 checksum of content
}

// Scanner discovers code files in a directory based on include/exclude patterns.
type Scanner struct {
	IncludePatterns []string // Glob patterns for files to include (e.g., "*.go", "*.py")
	ExcludePatterns []string // Glob patterns for files to exclude (e.g., "*_test.go", "vendor/*")
}

// Discover traverses the directory tree and returns all matching code files.
// It uses filepath.Walk to traverse directories, matches files against patterns,
// reads content, and calculates checksums.
func (s *Scanner) Discover(rootPath string) ([]CodeFile, error) {
	// Validate root path exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access root path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", rootPath)
	}

	var files []CodeFile

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log and skip paths we can't access
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip non-regular files (symlinks, devices, etc.)
		if !info.Mode().IsRegular() {
			return nil
		}

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil // Skip files we can't get absolute path for
		}

		// Check if file matches include patterns
		if !s.matchesInclude(absPath) {
			return nil
		}

		// Check if file matches exclude patterns
		if s.matchesExclude(absPath) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Calculate SHA-256 checksum
		hash := sha256.Sum256(content)
		checksum := hex.EncodeToString(hash[:])

		files = append(files, CodeFile{
			Path:     absPath,
			Content:  string(content),
			Size:     info.Size(),
			Checksum: checksum,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return files, nil
}

// matchesInclude checks if the file path matches any include pattern.
// If no include patterns are specified, all files match.
func (s *Scanner) matchesInclude(path string) bool {
	// If no include patterns, include all files
	if len(s.IncludePatterns) == 0 {
		return true
	}

	filename := filepath.Base(path)

	for _, pattern := range s.IncludePatterns {
		// Support both simple glob patterns and path patterns
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}

		// Also try matching against the full path for directory patterns
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// matchesExclude checks if the file path matches any exclude pattern.
// If no exclude patterns are specified, no files are excluded.
func (s *Scanner) matchesExclude(path string) bool {
	if len(s.ExcludePatterns) == 0 {
		return false
	}

	filename := filepath.Base(path)

	for _, pattern := range s.ExcludePatterns {
		// Handle directory patterns (e.g., "vendor/*", "node_modules/*")
		if strings.Contains(pattern, "/") {
			// Check if path contains the directory pattern
			cleanPattern := strings.TrimSuffix(pattern, "/*")
			cleanPattern = strings.TrimSuffix(cleanPattern, "/")
			if strings.Contains(path, cleanPattern+string(filepath.Separator)) {
				return true
			}
		}

		// Match against filename
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}

		// Also try matching against full path
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}
