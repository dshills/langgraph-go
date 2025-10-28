package graph

import (
	"io"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/store"
)

// TestFunctionalOptionsPattern verifies that functional options correctly configure the Engine.
func TestFunctionalOptionsPattern(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	tests := []struct {
		name     string
		options  []interface{}
		validate func(*testing.T, *Engine[TestState])
	}{
		{
			name: "WithMaxConcurrent sets MaxConcurrentNodes",
			options: []interface{}{
				WithMaxConcurrent(16),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.MaxConcurrentNodes != 16 {
					t.Errorf("MaxConcurrentNodes = %d, want 16", e.opts.MaxConcurrentNodes)
				}
			},
		},
		{
			name: "WithQueueDepth sets QueueDepth",
			options: []interface{}{
				WithQueueDepth(2048),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.QueueDepth != 2048 {
					t.Errorf("QueueDepth = %d, want 2048", e.opts.QueueDepth)
				}
			},
		},
		{
			name: "WithBackpressureTimeout sets BackpressureTimeout",
			options: []interface{}{
				WithBackpressureTimeout(60 * time.Second),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.BackpressureTimeout != 60*time.Second {
					t.Errorf("BackpressureTimeout = %v, want 60s", e.opts.BackpressureTimeout)
				}
			},
		},
		{
			name: "WithDefaultNodeTimeout sets DefaultNodeTimeout",
			options: []interface{}{
				WithDefaultNodeTimeout(10 * time.Second),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.DefaultNodeTimeout != 10*time.Second {
					t.Errorf("DefaultNodeTimeout = %v, want 10s", e.opts.DefaultNodeTimeout)
				}
			},
		},
		{
			name: "WithRunWallClockBudget sets RunWallClockBudget",
			options: []interface{}{
				WithRunWallClockBudget(5 * time.Minute),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.RunWallClockBudget != 5*time.Minute {
					t.Errorf("RunWallClockBudget = %v, want 5m", e.opts.RunWallClockBudget)
				}
			},
		},
		{
			name: "WithReplayMode enables ReplayMode",
			options: []interface{}{
				WithReplayMode(true),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if !e.opts.ReplayMode {
					t.Error("ReplayMode = false, want true")
				}
			},
		},
		{
			name: "WithStrictReplay enables StrictReplay",
			options: []interface{}{
				WithStrictReplay(true),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if !e.opts.StrictReplay {
					t.Error("StrictReplay = false, want true")
				}
			},
		},
		{
			name: "Multiple options applied in order",
			options: []interface{}{
				WithMaxConcurrent(8),
				WithQueueDepth(1024),
				WithBackpressureTimeout(30 * time.Second),
				WithDefaultNodeTimeout(15 * time.Second),
			},
			validate: func(t *testing.T, e *Engine[TestState]) {
				if e.opts.MaxConcurrentNodes != 8 {
					t.Errorf("MaxConcurrentNodes = %d, want 8", e.opts.MaxConcurrentNodes)
				}
				if e.opts.QueueDepth != 1024 {
					t.Errorf("QueueDepth = %d, want 1024", e.opts.QueueDepth)
				}
				if e.opts.BackpressureTimeout != 30*time.Second {
					t.Errorf("BackpressureTimeout = %v, want 30s", e.opts.BackpressureTimeout)
				}
				if e.opts.DefaultNodeTimeout != 15*time.Second {
					t.Errorf("DefaultNodeTimeout = %v, want 15s", e.opts.DefaultNodeTimeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := New(reducer, st, emitter, tt.options...)
			tt.validate(t, engine)
		})
	}
}

// TestBackwardCompatibility verifies that the legacy Options struct still works.
func TestBackwardCompatibility(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	t.Run("Options struct works", func(t *testing.T) {
		opts := Options{
			MaxSteps:            100,
			Retries:             3,
			MaxConcurrentNodes:  8,
			QueueDepth:          2048,
			BackpressureTimeout: 45 * time.Second,
			DefaultNodeTimeout:  20 * time.Second,
			RunWallClockBudget:  10 * time.Minute,
			ReplayMode:          true,
			StrictReplay:        true,
		}

		engine := New(reducer, st, emitter, opts)

		if engine.opts.MaxSteps != 100 {
			t.Errorf("MaxSteps = %d, want 100", engine.opts.MaxSteps)
		}
		if engine.opts.Retries != 3 {
			t.Errorf("Retries = %d, want 3", engine.opts.Retries)
		}
		if engine.opts.MaxConcurrentNodes != 8 {
			t.Errorf("MaxConcurrentNodes = %d, want 8", engine.opts.MaxConcurrentNodes)
		}
		if engine.opts.QueueDepth != 2048 {
			t.Errorf("QueueDepth = %d, want 2048", engine.opts.QueueDepth)
		}
		if engine.opts.BackpressureTimeout != 45*time.Second {
			t.Errorf("BackpressureTimeout = %v, want 45s", engine.opts.BackpressureTimeout)
		}
		if engine.opts.DefaultNodeTimeout != 20*time.Second {
			t.Errorf("DefaultNodeTimeout = %v, want 20s", engine.opts.DefaultNodeTimeout)
		}
		if engine.opts.RunWallClockBudget != 10*time.Minute {
			t.Errorf("RunWallClockBudget = %v, want 10m", engine.opts.RunWallClockBudget)
		}
		if !engine.opts.ReplayMode {
			t.Error("ReplayMode = false, want true")
		}
		if !engine.opts.StrictReplay {
			t.Error("StrictReplay = false, want true")
		}
	})

	t.Run("Empty Options struct works (zero values)", func(t *testing.T) {
		engine := New(reducer, st, emitter, Options{})

		if engine.opts.MaxSteps != 0 {
			t.Errorf("MaxSteps = %d, want 0", engine.opts.MaxSteps)
		}
		if engine.opts.Retries != 0 {
			t.Errorf("Retries = %d, want 0", engine.opts.Retries)
		}
	})

	t.Run("No options works (zero values)", func(t *testing.T) {
		engine := New(reducer, st, emitter)

		if engine.opts.MaxSteps != 0 {
			t.Errorf("MaxSteps = %d, want 0", engine.opts.MaxSteps)
		}
	})
}

// TestBothPatternsTogether verifies that Options struct and functional options can be mixed.
func TestBothPatternsTogether(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	t.Run("Functional options override Options struct", func(t *testing.T) {
		baseOpts := Options{
			MaxSteps:           100,
			MaxConcurrentNodes: 4,   // Will be overridden
			QueueDepth:         512, // Will be overridden
		}

		engine := New(
			reducer, st, emitter,
			baseOpts,
			WithMaxConcurrent(16), // Override
			WithQueueDepth(2048),  // Override
		)

		// Check base opts preserved
		if engine.opts.MaxSteps != 100 {
			t.Errorf("MaxSteps = %d, want 100", engine.opts.MaxSteps)
		}

		// Check functional options override
		if engine.opts.MaxConcurrentNodes != 16 {
			t.Errorf("MaxConcurrentNodes = %d, want 16 (should be overridden)", engine.opts.MaxConcurrentNodes)
		}
		if engine.opts.QueueDepth != 2048 {
			t.Errorf("QueueDepth = %d, want 2048 (should be overridden)", engine.opts.QueueDepth)
		}
	})

	t.Run("Multiple Options structs - last wins", func(t *testing.T) {
		opts1 := Options{MaxSteps: 100}
		opts2 := Options{MaxSteps: 200}

		engine := New(reducer, st, emitter, opts1, opts2)

		if engine.opts.MaxSteps != 200 {
			t.Errorf("MaxSteps = %d, want 200 (last Options struct should win)", engine.opts.MaxSteps)
		}
	})

	t.Run("Functional options after Options struct", func(t *testing.T) {
		baseOpts := Options{
			MaxSteps:   100,
			ReplayMode: false,
		}

		engine := New(
			reducer, st, emitter,
			baseOpts,
			WithReplayMode(true),   // Override
			WithStrictReplay(true), // Add new setting
		)

		if engine.opts.MaxSteps != 100 {
			t.Errorf("MaxSteps = %d, want 100", engine.opts.MaxSteps)
		}
		if !engine.opts.ReplayMode {
			t.Error("ReplayMode = false, want true (should be overridden)")
		}
		if !engine.opts.StrictReplay {
			t.Error("StrictReplay = false, want true (should be added)")
		}
	})
}

// TestConflictPolicy verifies conflict policy validation.
func TestConflictPolicy(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	t.Run("ConflictFail policy accepted", func(t *testing.T) {
		// ConflictFail is the only supported policy
		engine := New(reducer, st, emitter, WithConflictPolicy(ConflictFail))

		// Should not panic or error during construction
		if engine == nil {
			t.Fatal("Expected engine to be created")
		}
	})

	t.Run("Unsupported policies handled gracefully", func(t *testing.T) {
		// LastWriterWins not yet implemented - should be handled gracefully
		// The error is returned by the Option function, but New ignores it
		engine := New(reducer, st, emitter, WithConflictPolicy(LastWriterWins))

		// Engine still created (validation deferred to Run time)
		if engine == nil {
			t.Fatal("Expected engine to be created even with unsupported policy")
		}
	})
}

// TestOptionApplicationOrder verifies options are applied in declaration order.
func TestOptionApplicationOrder(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	t.Run("Later options override earlier ones", func(t *testing.T) {
		engine := New(
			reducer, st, emitter,
			WithMaxConcurrent(4),
			WithMaxConcurrent(8),  // Override first value
			WithMaxConcurrent(16), // Override second value
		)

		if engine.opts.MaxConcurrentNodes != 16 {
			t.Errorf("MaxConcurrentNodes = %d, want 16 (last value should win)", engine.opts.MaxConcurrentNodes)
		}
	})

	t.Run("Options struct before functional options", func(t *testing.T) {
		engine := New(
			reducer, st, emitter,
			Options{MaxConcurrentNodes: 4},
			WithMaxConcurrent(16), // Should override
		)

		if engine.opts.MaxConcurrentNodes != 16 {
			t.Errorf("MaxConcurrentNodes = %d, want 16 (functional option should override)", engine.opts.MaxConcurrentNodes)
		}
	})

	t.Run("Options struct after functional options", func(t *testing.T) {
		engine := New(
			reducer, st, emitter,
			WithMaxConcurrent(16),
			Options{MaxConcurrentNodes: 4}, // Should override functional option
		)

		if engine.opts.MaxConcurrentNodes != 4 {
			t.Errorf("MaxConcurrentNodes = %d, want 4 (Options struct should override)", engine.opts.MaxConcurrentNodes)
		}
	})
}

// TestZeroValues verifies that zero values are handled correctly.
func TestZeroValues(t *testing.T) {
	type TestState struct {
		Value int
	}

	reducer := func(prev, delta TestState) TestState {
		return TestState{Value: prev.Value + delta.Value}
	}

	st := store.NewMemStore[TestState]()
	emitter := emit.NewLogEmitter(io.Discard, false)

	t.Run("WithMaxConcurrent(0) sets zero", func(t *testing.T) {
		engine := New(reducer, st, emitter, WithMaxConcurrent(0))

		if engine.opts.MaxConcurrentNodes != 0 {
			t.Errorf("MaxConcurrentNodes = %d, want 0", engine.opts.MaxConcurrentNodes)
		}
	})

	t.Run("WithReplayMode(false) disables replay", func(t *testing.T) {
		engine := New(reducer, st, emitter, WithReplayMode(false))

		if engine.opts.ReplayMode {
			t.Error("ReplayMode = true, want false")
		}
	})
}
