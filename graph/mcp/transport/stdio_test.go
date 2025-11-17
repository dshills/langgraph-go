package transport

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// mockReadCloser is a test helper that wraps an io.Reader with a Close method
// that can be configured to return an error.
type mockReadCloser struct {
	io.Reader
	closeErr error
	closed   bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return m.closeErr
}

// mockWriteCloser is a test helper that wraps an io.Writer with a Close method
// that can be configured to return an error.
type mockWriteCloser struct {
	io.Writer
	closeErr error
	closed   bool
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return m.closeErr
}

// TestStdioReadWriteCloser_Read verifies the Read method correctly delegates
// to the underlying reader.
func TestStdioReadWriteCloser_Read(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		bufSize  int
		wantN    int
		wantData string
		wantErr  error
	}{
		{
			name:     "read full buffer",
			input:    "hello world",
			bufSize:  11,
			wantN:    11,
			wantData: "hello world",
			wantErr:  nil,
		},
		{
			name:     "read partial buffer",
			input:    "hello world",
			bufSize:  5,
			wantN:    5,
			wantData: "hello",
			wantErr:  nil,
		},
		{
			name:     "read empty input",
			input:    "",
			bufSize:  10,
			wantN:    0,
			wantData: "",
			wantErr:  io.EOF,
		},
		{
			name:     "read with small buffer",
			input:    "test",
			bufSize:  2,
			wantN:    2,
			wantData: "te",
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockReadCloser{
				Reader: bytes.NewBufferString(tt.input),
			}

			s := &StdioReadWriteCloser{
				reader: reader,
				writer: &mockWriteCloser{Writer: io.Discard},
			}

			buf := make([]byte, tt.bufSize)
			n, err := s.Read(buf)

			if n != tt.wantN {
				t.Errorf("Read() n = %v, want %v", n, tt.wantN)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Read() err = %v, want %v", err, tt.wantErr)
			}

			gotData := string(buf[:n])
			if gotData != tt.wantData {
				t.Errorf("Read() data = %q, want %q", gotData, tt.wantData)
			}
		})
	}
}

// TestStdioReadWriteCloser_Write verifies the Write method correctly delegates
// to the underlying writer.
func TestStdioReadWriteCloser_Write(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantN    int
		wantData string
	}{
		{
			name:     "write string data",
			input:    []byte("hello world"),
			wantN:    11,
			wantData: "hello world",
		},
		{
			name:     "write empty data",
			input:    []byte{},
			wantN:    0,
			wantData: "",
		},
		{
			name:     "write binary data",
			input:    []byte{0x01, 0x02, 0x03, 0x04},
			wantN:    4,
			wantData: "\x01\x02\x03\x04",
		},
		{
			name:     "write JSON-RPC message",
			input:    []byte(`{"jsonrpc":"2.0","method":"initialize","id":1}`),
			wantN:    46,
			wantData: `{"jsonrpc":"2.0","method":"initialize","id":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := &mockWriteCloser{Writer: &buf}

			s := &StdioReadWriteCloser{
				reader: &mockReadCloser{Reader: bytes.NewReader(nil)},
				writer: writer,
			}

			n, err := s.Write(tt.input)

			if err != nil {
				t.Errorf("Write() unexpected error: %v", err)
			}

			if n != tt.wantN {
				t.Errorf("Write() n = %v, want %v", n, tt.wantN)
			}

			gotData := buf.String()
			if gotData != tt.wantData {
				t.Errorf("Write() data = %q, want %q", gotData, tt.wantData)
			}
		})
	}
}

// TestStdioReadWriteCloser_Close verifies the Close method properly closes
// both reader and writer and handles errors correctly.
func TestStdioReadWriteCloser_Close(t *testing.T) {
	tests := []struct {
		name        string
		readerErr   error
		writerErr   error
		wantErr     bool
		errContains []string
	}{
		{
			name:      "close both successfully",
			readerErr: nil,
			writerErr: nil,
			wantErr:   false,
		},
		{
			name:        "reader close fails",
			readerErr:   errors.New("reader close error"),
			writerErr:   nil,
			wantErr:     true,
			errContains: []string{"reader close error"},
		},
		{
			name:        "writer close fails",
			readerErr:   nil,
			writerErr:   errors.New("writer close error"),
			wantErr:     true,
			errContains: []string{"writer close error"},
		},
		{
			name:        "both close fail",
			readerErr:   errors.New("reader close error"),
			writerErr:   errors.New("writer close error"),
			wantErr:     true,
			errContains: []string{"reader close error", "writer close error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockReadCloser{
				Reader:   bytes.NewReader(nil),
				closeErr: tt.readerErr,
			}
			writer := &mockWriteCloser{
				Writer:   io.Discard,
				closeErr: tt.writerErr,
			}

			s := &StdioReadWriteCloser{
				reader: reader,
				writer: writer,
			}

			err := s.Close()

			// Verify both Close methods were called
			if !reader.closed {
				t.Error("Close() did not close reader")
			}
			if !writer.closed {
				t.Error("Close() did not close writer")
			}

			// Verify error behavior
			if tt.wantErr && err == nil {
				t.Error("Close() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Close() unexpected error: %v", err)
			}

			// Verify all expected errors are present in joined error
			if err != nil {
				errStr := err.Error()
				for _, want := range tt.errContains {
					if !bytes.Contains([]byte(errStr), []byte(want)) {
						t.Errorf("Close() error = %q, want to contain %q", errStr, want)
					}
				}
			}
		})
	}
}

// TestStdioReadWriteCloser_ReadWrite verifies bidirectional communication
// through the transport works correctly.
func TestStdioReadWriteCloser_ReadWrite(t *testing.T) {
	// Create buffers for simulating stdin/stdout
	input := bytes.NewBufferString("input data")
	var output bytes.Buffer

	reader := &mockReadCloser{Reader: input}
	writer := &mockWriteCloser{Writer: &output}

	s := &StdioReadWriteCloser{
		reader: reader,
		writer: writer,
	}

	// Write some data
	writeData := []byte("output data")
	n, err := s.Write(writeData)
	if err != nil {
		t.Fatalf("Write() unexpected error: %v", err)
	}
	if n != len(writeData) {
		t.Errorf("Write() n = %v, want %v", n, len(writeData))
	}

	// Read some data
	readBuf := make([]byte, 5)
	n, err = s.Read(readBuf)
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("Read() n = %v, want 5", n)
	}

	// Verify written data
	if output.String() != "output data" {
		t.Errorf("Write() data = %q, want %q", output.String(), "output data")
	}

	// Verify read data
	if string(readBuf) != "input" {
		t.Errorf("Read() data = %q, want %q", string(readBuf), "input")
	}

	// Close and verify
	err = s.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
	if !reader.closed || !writer.closed {
		t.Error("Close() did not close both reader and writer")
	}
}

// TestNewStdioReadWriteCloser verifies the constructor creates a properly
// initialized transport with os.Stdin and os.Stdout.
func TestNewStdioReadWriteCloser(t *testing.T) {
	s := NewStdioReadWriteCloser()

	if s == nil {
		t.Fatal("NewStdioReadWriteCloser() returned nil")
	}

	if s.reader == nil {
		t.Error("NewStdioReadWriteCloser() reader is nil")
	}

	if s.writer == nil {
		t.Error("NewStdioReadWriteCloser() writer is nil")
	}

	// Note: We can't directly test os.Stdin/os.Stdout equality in a portable way,
	// but we can verify the transport is usable. The actual os.Stdin/os.Stdout
	// integration would be tested in integration tests.
}
