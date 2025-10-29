// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import "errors"

// ErrMaxStepsExceeded indicates that the graph execution reached the maximum.
// allowed step count without completing. This prevents infinite loops and.
// runaway executions.
var ErrMaxStepsExceeded = errors.New("execution exceeded maximum steps limit")

// ErrBackpressure indicates that downstream processing cannot keep up with.
// the current execution rate. This typically occurs when output buffers are.
// full or rate limits are exceeded. This is distinct from ErrBackpressureTimeout.
// which is specifically for frontier queue overflow.
var ErrBackpressure = errors.New("downstream backpressure exceeded threshold")

// Note: The following errors are already defined in checkpoint.go:
// - ErrReplayMismatch: replay mismatch detection.
// - ErrNoProgress: deadlock/no runnable nodes detection.
// - ErrIdempotencyViolation: duplicate checkpoint prevention.
// - ErrMaxAttemptsExceeded: retry exhaustion.
// - ErrBackpressureTimeout: frontier queue overflow.
