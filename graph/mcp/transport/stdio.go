package transport

import (
	"errors"
	"io"
	"os"
)

// StdioReadWriteCloser wraps stdin and stdout to provide an io.ReadWriteCloser
// interface for JSON-RPC communication over standard input/output streams.
//
// This transport is commonly used for MCP server implementations that communicate
// with clients via stdin/stdout, following the JSON-RPC 2.0 protocol over standard
// streams as specified in the MCP specification.
//
// Example usage:
//
//	transport := NewStdioReadWriteCloser()
//	defer transport.Close()
//
//	// Use transport for JSON-RPC communication
//	encoder := json.NewEncoder(transport)
//	decoder := json.NewDecoder(transport)
type StdioReadWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

// NewStdioReadWriteCloser creates a new StdioReadWriteCloser that wraps
// os.Stdin for reading and os.Stdout for writing.
//
// The returned ReadWriteCloser can be used for bidirectional JSON-RPC
// communication over standard streams. Both the reader and writer must
// be closed when done to properly clean up resources.
//
// Returns a StdioReadWriteCloser configured with os.Stdin and os.Stdout.
func NewStdioReadWriteCloser() *StdioReadWriteCloser {
	return &StdioReadWriteCloser{
		reader: os.Stdin,
		writer: os.Stdout,
	}
}

// Read reads up to len(b) bytes from the underlying reader (stdin).
// It returns the number of bytes read and any error encountered.
//
// Read implements the io.Reader interface, allowing the StdioReadWriteCloser
// to be used anywhere an io.Reader is expected.
//
// Parameters:
//   - b: byte slice to read data into
//
// Returns:
//   - n: number of bytes read
//   - err: error if read operation failed, or io.EOF when stream ends
func (s *StdioReadWriteCloser) Read(b []byte) (int, error) {
	return s.reader.Read(b)
}

// Write writes len(b) bytes to the underlying writer (stdout).
// It returns the number of bytes written and any error encountered.
//
// Write implements the io.Writer interface, allowing the StdioReadWriteCloser
// to be used anywhere an io.Writer is expected.
//
// Parameters:
//   - b: byte slice containing data to write
//
// Returns:
//   - n: number of bytes written
//   - err: error if write operation failed
func (s *StdioReadWriteCloser) Write(b []byte) (int, error) {
	return s.writer.Write(b)
}

// Close closes both the reader and writer streams.
// If either close operation fails, all errors are combined and returned
// using errors.Join to preserve all error information.
//
// Close implements the io.Closer interface, allowing the StdioReadWriteCloser
// to be used with defer statements and ensuring proper resource cleanup.
//
// Returns:
//   - err: combined error if any close operation failed, or nil if both succeeded
func (s *StdioReadWriteCloser) Close() error {
	readerErr := s.reader.Close()
	writerErr := s.writer.Close()
	return errors.Join(readerErr, writerErr)
}
