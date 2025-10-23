package graph

import "testing"

// TestReducer_Examples verifies Reducer function type with examples (T022).
func TestReducer_Examples(t *testing.T) {
	t.Run("simple merge reducer", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			if delta.Counter != 0 {
				prev.Counter = delta.Counter
			}
			return prev
		}

		prev := TestState{Value: "old", Counter: 5}
		delta := TestState{Value: "new"}

		result := reducer(prev, delta)

		if result.Value != "new" {
			t.Errorf("expected Value = 'new', got %q", result.Value)
		}
		if result.Counter != 5 {
			t.Errorf("expected Counter = 5 (unchanged), got %d", result.Counter)
		}
	})

	t.Run("accumulating reducer", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			return prev
		}

		prev := TestState{Value: "base", Counter: 10}
		delta := TestState{Counter: 5}

		result := reducer(prev, delta)

		if result.Counter != 15 {
			t.Errorf("expected Counter = 15, got %d", result.Counter)
		}
		if result.Value != "base" {
			t.Errorf("expected Value = 'base', got %q", result.Value)
		}
	})

	t.Run("reducer is deterministic", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			prev.Counter = prev.Counter + delta.Counter
			return prev
		}

		prev := TestState{Value: "initial", Counter: 1}
		delta := TestState{Value: "updated", Counter: 2}

		// Run reducer multiple times - should always produce same result
		result1 := reducer(prev, delta)
		prev = TestState{Value: "initial", Counter: 1} // reset
		result2 := reducer(prev, delta)

		if result1 != result2 {
			t.Errorf("reducer not deterministic: %+v != %+v", result1, result2)
		}
	})

	t.Run("reducer with zero delta", func(t *testing.T) {
		reducer := func(prev, delta TestState) TestState {
			if delta.Value != "" {
				prev.Value = delta.Value
			}
			if delta.Counter != 0 {
				prev.Counter = delta.Counter
			}
			return prev
		}

		prev := TestState{Value: "unchanged", Counter: 42}
		delta := TestState{} // zero value

		result := reducer(prev, delta)

		if result.Value != "unchanged" {
			t.Errorf("expected Value unchanged, got %q", result.Value)
		}
		if result.Counter != 42 {
			t.Errorf("expected Counter unchanged, got %d", result.Counter)
		}
	})
}

// TestReducer_TypeSignature verifies Reducer function type can be declared (T023).
func TestReducer_TypeSignature(t *testing.T) {
	// Test that Reducer[S] type can be assigned
	var r Reducer[TestState]

	r = func(prev, delta TestState) TestState {
		return prev
	}

	// Use the reducer
	result := r(TestState{Value: "test"}, TestState{})

	if result.Value != "test" {
		t.Errorf("expected Value = 'test', got %q", result.Value)
	}
}
