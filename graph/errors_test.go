// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"errors"
	"testing"
)

// TestTypedErrorHandling verifies that all exported errors work with errors.Is.
func TestTypedErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		target   error
		shouldBe bool
	}{
		{
			name:     "ErrMaxStepsExceeded identity",
			err:      ErrMaxStepsExceeded,
			target:   ErrMaxStepsExceeded,
			shouldBe: true,
		},
		{
			name:     "ErrBackpressure identity",
			err:      ErrBackpressure,
			target:   ErrBackpressure,
			shouldBe: true,
		},
		{
			name:     "ErrReplayMismatch identity",
			err:      ErrReplayMismatch,
			target:   ErrReplayMismatch,
			shouldBe: true,
		},
		{
			name:     "ErrNoProgress identity",
			err:      ErrNoProgress,
			target:   ErrNoProgress,
			shouldBe: true,
		},
		{
			name:     "ErrIdempotencyViolation identity",
			err:      ErrIdempotencyViolation,
			target:   ErrIdempotencyViolation,
			shouldBe: true,
		},
		{
			name:     "ErrMaxAttemptsExceeded identity",
			err:      ErrMaxAttemptsExceeded,
			target:   ErrMaxAttemptsExceeded,
			shouldBe: true,
		},
		{
			name:     "ErrBackpressureTimeout identity",
			err:      ErrBackpressureTimeout,
			target:   ErrBackpressureTimeout,
			shouldBe: true,
		},
		{
			name:     "Different errors don't match",
			err:      ErrMaxStepsExceeded,
			target:   ErrBackpressure,
			shouldBe: false,
		},
		{
			name:     "Nil error doesn't match",
			err:      nil,
			target:   ErrMaxStepsExceeded,
			shouldBe: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errors.Is(tt.err, tt.target) != tt.shouldBe {
				if tt.shouldBe {
					t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.target)
				} else {
					t.Errorf("errors.Is(%v, %v) = true, want false", tt.err, tt.target)
				}
			}
		})
	}
}

// TestEngineErrorWrapping verifies that EngineError can be detected with errors.As.
func TestEngineErrorWrapping(t *testing.T) {
	t.Run("EngineError matches with errors.As", func(t *testing.T) {
		originalErr := &EngineError{
			Message: "test error",
			Code:    "TEST_ERROR",
		}

		var engineErr *EngineError
		if !errors.As(originalErr, &engineErr) {
			t.Error("errors.As failed to match EngineError")
		}

		if engineErr.Code != "TEST_ERROR" {
			t.Errorf("Code = %s, want TEST_ERROR", engineErr.Code)
		}
		if engineErr.Message != "test error" {
			t.Errorf("Message = %s, want 'test error'", engineErr.Message)
		}
	})

	t.Run("Wrapped EngineError matches with errors.As", func(t *testing.T) {
		originalErr := &EngineError{
			Message: "inner error",
			Code:    "INNER_ERROR",
		}
		wrappedErr := errors.Join(originalErr, errors.New("outer error"))

		var engineErr *EngineError
		if !errors.As(wrappedErr, &engineErr) {
			t.Error("errors.As failed to match wrapped EngineError")
		}

		if engineErr.Code != "INNER_ERROR" {
			t.Errorf("Code = %s, want INNER_ERROR", engineErr.Code)
		}
	})

	t.Run("EngineError.Error() includes code", func(t *testing.T) {
		err := &EngineError{
			Message: "something went wrong",
			Code:    "ERR_CODE",
		}

		expected := "ERR_CODE: something went wrong"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("EngineError.Error() without code", func(t *testing.T) {
		err := &EngineError{
			Message: "something went wrong",
		}

		expected := "something went wrong"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})
}

// TestErrorDocumentation verifies error messages are descriptive.
func TestErrorDocumentation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrMaxStepsExceeded", ErrMaxStepsExceeded},
		{"ErrBackpressure", ErrBackpressure},
		{"ErrReplayMismatch", ErrReplayMismatch},
		{"ErrNoProgress", ErrNoProgress},
		{"ErrIdempotencyViolation", ErrIdempotencyViolation},
		{"ErrMaxAttemptsExceeded", ErrMaxAttemptsExceeded},
		{"ErrBackpressureTimeout", ErrBackpressureTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("Error is nil")
			}

			msg := tt.err.Error()
			if msg == "" {
				t.Error("Error message is empty")
			}

			// Error messages should be descriptive (at least 10 characters).
			if len(msg) < 10 {
				t.Errorf("Error message too short (%d chars): %q", len(msg), msg)
			}
		})
	}
}
