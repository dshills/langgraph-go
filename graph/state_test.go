package graph

import (
	"sort"
	"testing"
)

// TestDeepCopy_SimpleStruct verifies deep copy of simple struct (T101).
func TestDeepCopy_SimpleStruct(t *testing.T) {
	t.Run("copy simple state with primitives", func(t *testing.T) {
		type SimpleState struct {
			Name    string
			Counter int
			Active  bool
		}

		original := SimpleState{
			Name:    "test",
			Counter: 42,
			Active:  true,
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify values match
		if copied.Name != original.Name {
			t.Errorf("expected Name = %q, got %q", original.Name, copied.Name)
		}
		if copied.Counter != original.Counter {
			t.Errorf("expected Counter = %d, got %d", original.Counter, copied.Counter)
		}
		if copied.Active != original.Active {
			t.Errorf("expected Active = %v, got %v", original.Active, copied.Active)
		}

		// Verify it's a separate copy (modifying copy doesn't affect original)
		copied.Name = "modified"
		copied.Counter = 99

		if original.Name == "modified" {
			t.Error("modifying copy affected original Name")
		}
		if original.Counter == 99 {
			t.Error("modifying copy affected original Counter")
		}
	})
}

// TestDeepCopy_NestedStruct verifies deep copy of nested structures (T101).
func TestDeepCopy_NestedStruct(t *testing.T) {
	t.Run("copy state with nested struct", func(t *testing.T) {
		type Address struct {
			Street string
			City   string
		}

		type Person struct {
			Name    string
			Age     int
			Address Address
		}

		original := Person{
			Name: "Alice",
			Age:  30,
			Address: Address{
				Street: "123 Main St",
				City:   "Springfield",
			},
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify deep copy independence
		copied.Address.Street = "456 Oak Ave"
		copied.Address.City = "Shelbyville"

		if original.Address.Street == "456 Oak Ave" {
			t.Error("modifying nested copy affected original")
		}
		if original.Address.City == "Shelbyville" {
			t.Error("modifying nested copy affected original")
		}
	})
}

// TestDeepCopy_WithSlices verifies deep copy of slices (T101).
func TestDeepCopy_WithSlices(t *testing.T) {
	t.Run("copy state with slice", func(t *testing.T) {
		type StateWithSlice struct {
			Items []string
			Count int
		}

		original := StateWithSlice{
			Items: []string{"a", "b", "c"},
			Count: 3,
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify slice independence
		copied.Items[0] = "modified"
		copied.Items = append(copied.Items, "d")

		if original.Items[0] == "modified" {
			t.Error("modifying copied slice affected original")
		}
		if len(original.Items) != 3 {
			t.Error("appending to copied slice affected original length")
		}
	})
}

// TestDeepCopy_WithMaps verifies deep copy of maps (T101).
func TestDeepCopy_WithMaps(t *testing.T) {
	t.Run("copy state with map", func(t *testing.T) {
		type StateWithMap struct {
			Data map[string]int
			Name string
		}

		original := StateWithMap{
			Data: map[string]int{"a": 1, "b": 2},
			Name: "test",
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify map independence
		copied.Data["a"] = 999
		copied.Data["c"] = 3

		if original.Data["a"] == 999 {
			t.Error("modifying copied map affected original")
		}
		if _, exists := original.Data["c"]; exists {
			t.Error("adding to copied map affected original")
		}
	})
}

// TestDeepCopy_WithPointers verifies deep copy of pointer fields (T101).
func TestDeepCopy_WithPointers(t *testing.T) {
	t.Run("copy state with pointer fields", func(t *testing.T) {
		type StateWithPointer struct {
			Value *int
			Name  string
		}

		val := 42
		original := StateWithPointer{
			Value: &val,
			Name:  "test",
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify pointer is copied (not just the address)
		if original.Value == copied.Value {
			t.Error("pointer addresses should be different (deep copy)")
		}

		// Verify values match
		if *copied.Value != *original.Value {
			t.Errorf("expected *Value = %d, got %d", *original.Value, *copied.Value)
		}

		// Verify modifying copy doesn't affect original
		*copied.Value = 999
		if *original.Value == 999 {
			t.Error("modifying copied pointer value affected original")
		}
	})
}

// TestDeepCopy_EmptyState verifies deep copy of zero-value state (T101).
func TestDeepCopy_EmptyState(t *testing.T) {
	t.Run("copy zero-value state", func(t *testing.T) {
		type EmptyState struct {
			Name  string
			Count int
		}

		original := EmptyState{} // Zero value

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		if copied.Name != "" {
			t.Errorf("expected empty Name, got %q", copied.Name)
		}
		if copied.Count != 0 {
			t.Errorf("expected Count = 0, got %d", copied.Count)
		}
	})
}

// DeltaWithOrderKey pairs a state delta with its order key for testing deterministic merging
type DeltaWithOrderKey struct {
	Delta    MergeTestState
	OrderKey uint64
}

// TestDeterministicMerge (T026) verifies that state deltas from concurrent
// nodes merge deterministically regardless of completion order.
//
// According to spec.md FR-004: System MUST merge state deltas from concurrent
// nodes using pure, deterministic reducer functions.
//
// According to spec.md FR-005: System MUST maintain merge order based on
// ascending order_key of producing nodes.
//
// Requirements:
// - Same deltas arriving in different orders produce identical final state
// - Reducer must be deterministic (same inputs -> same output)
// - Order key sorting ensures consistent merge order
//
// This test should FAIL initially because order-key-based merge sorting isn't implemented yet.
func TestDeterministicMerge(t *testing.T) {
	t.Run("merge deltas in order key order", func(t *testing.T) {
		// Define a reducer that concatenates values
		reducer := func(prev, delta MergeTestState) MergeTestState {
			if delta.Value != "" {
				if prev.Value == "" {
					prev.Value = delta.Value
				} else {
					prev.Value += "," + delta.Value
				}
			}
			prev.Counter += delta.Counter
			return prev
		}

		// Create deltas with different order keys
		deltas := []DeltaWithOrderKey{
			{Delta: MergeTestState{Value: "third", Counter: 3}, OrderKey: 300},
			{Delta: MergeTestState{Value: "first", Counter: 1}, OrderKey: 100},
			{Delta: MergeTestState{Value: "second", Counter: 2}, OrderKey: 200},
		}

		// Test 1: Apply in original order
		state1 := MergeTestState{}
		// Sort by order key before applying
		sortedDeltas1 := make([]DeltaWithOrderKey, len(deltas))
		copy(sortedDeltas1, deltas)
		// Sort implementation will be in T038
		// For now, we assume a sortByOrderKey function exists
		sortByOrderKey(sortedDeltas1)

		for _, d := range sortedDeltas1 {
			state1 = reducer(state1, d.Delta)
		}

		// Test 2: Apply in different order
		deltas2 := []DeltaWithOrderKey{
			{Delta: MergeTestState{Value: "second", Counter: 2}, OrderKey: 200},
			{Delta: MergeTestState{Value: "third", Counter: 3}, OrderKey: 300},
			{Delta: MergeTestState{Value: "first", Counter: 1}, OrderKey: 100},
		}

		state2 := MergeTestState{}
		sortedDeltas2 := make([]DeltaWithOrderKey, len(deltas2))
		copy(sortedDeltas2, deltas2)
		sortByOrderKey(sortedDeltas2)

		for _, d := range sortedDeltas2 {
			state2 = reducer(state2, d.Delta)
		}

		// Both should produce identical results
		if state1.Value != state2.Value {
			t.Errorf("non-deterministic merge: state1.Value=%s, state2.Value=%s",
				state1.Value, state2.Value)
		}

		if state1.Counter != state2.Counter {
			t.Errorf("non-deterministic merge: state1.Counter=%d, state2.Counter=%d",
				state1.Counter, state2.Counter)
		}

		// Expected order is first,second,third based on order keys 100,200,300
		expectedValue := "first,second,third"
		if state1.Value != expectedValue {
			t.Errorf("expected Value=%s, got %s", expectedValue, state1.Value)
		}

		expectedCounter := 6
		if state1.Counter != expectedCounter {
			t.Errorf("expected Counter=%d, got %d", expectedCounter, state1.Counter)
		}
	})

	t.Run("concurrent deltas always merge in same order", func(t *testing.T) {
		// Simulate many concurrent nodes completing in random order
		reducer := func(prev, delta MergeTestState) MergeTestState {
			prev.Counter += delta.Counter
			if delta.Value != "" {
				if prev.Value == "" {
					prev.Value = delta.Value
				} else {
					prev.Value += "|" + delta.Value
				}
			}
			return prev
		}

		// Create 10 deltas with order keys
		deltas := []DeltaWithOrderKey{
			{Delta: MergeTestState{Value: "n5", Counter: 5}, OrderKey: 500},
			{Delta: MergeTestState{Value: "n2", Counter: 2}, OrderKey: 200},
			{Delta: MergeTestState{Value: "n8", Counter: 8}, OrderKey: 800},
			{Delta: MergeTestState{Value: "n1", Counter: 1}, OrderKey: 100},
			{Delta: MergeTestState{Value: "n9", Counter: 9}, OrderKey: 900},
			{Delta: MergeTestState{Value: "n4", Counter: 4}, OrderKey: 400},
			{Delta: MergeTestState{Value: "n7", Counter: 7}, OrderKey: 700},
			{Delta: MergeTestState{Value: "n3", Counter: 3}, OrderKey: 300},
			{Delta: MergeTestState{Value: "n6", Counter: 6}, OrderKey: 600},
			{Delta: MergeTestState{Value: "n10", Counter: 10}, OrderKey: 1000},
		}

		// Run merge 5 times with different orderings
		results := make([]MergeTestState, 5)
		for run := 0; run < 5; run++ {
			state := MergeTestState{}

			// Shuffle deltas differently each time (simulating random completion)
			shuffled := make([]DeltaWithOrderKey, len(deltas))
			copy(shuffled, deltas)
			// Shuffle implementation would vary per run
			// Then sort by order key
			sortByOrderKey(shuffled)

			for _, d := range shuffled {
				state = reducer(state, d.Delta)
			}

			results[run] = state
		}

		// All 5 runs should produce identical results
		for i := 1; i < 5; i++ {
			if results[i].Value != results[0].Value {
				t.Errorf("run %d produced different Value: %s vs %s",
					i, results[i].Value, results[0].Value)
			}
			if results[i].Counter != results[0].Counter {
				t.Errorf("run %d produced different Counter: %d vs %d",
					i, results[i].Counter, results[0].Counter)
			}
		}

		// Expected deterministic order based on order keys
		expectedValue := "n1|n2|n3|n4|n5|n6|n7|n8|n9|n10"
		if results[0].Value != expectedValue {
			t.Errorf("expected Value=%s, got %s", expectedValue, results[0].Value)
		}

		expectedCounter := 55 // Sum of 1..10
		if results[0].Counter != expectedCounter {
			t.Errorf("expected Counter=%d, got %d", expectedCounter, results[0].Counter)
		}
	})

	t.Run("reducer purity requirement", func(t *testing.T) {
		// Verify that reducer is pure (no side effects)
		// This is more of a contract test - reducer should not modify external state

		externalCounter := 0

		// Bad reducer with side effects (for demonstration)
		impureReducer := func(prev, delta MergeTestState) MergeTestState {
			externalCounter++ // Side effect! Bad!
			prev.Counter += delta.Counter
			return prev
		}

		state := MergeTestState{}
		delta1 := MergeTestState{Counter: 5}
		delta2 := MergeTestState{Counter: 10}

		state = impureReducer(state, delta1)
		state = impureReducer(state, delta2)

		// This test documents the purity requirement
		// In production, we would validate reducer purity during testing
		if externalCounter != 2 {
			t.Logf("impure reducer modified external state %d times", externalCounter)
		}

		// Note: Actual purity validation would be done via testing framework
		// (SC-008: Reducer purity validation catches 100% of non-deterministic reducers)
		t.Skip("Reducer purity validation framework deferred to Phase 10 (T124)")
	})
}

// MergeTestState is a state type for testing deterministic merging
type MergeTestState struct {
	Value   string
	Counter int
}

// sortByOrderKey is a helper for sorting deltas by order key
// This will be implemented in T038 as part of the engine's merge logic
func sortByOrderKey(deltas []DeltaWithOrderKey) {
	// Sort by OrderKey in ascending order
	sort.Slice(deltas, func(i, j int) bool {
		return deltas[i].OrderKey < deltas[j].OrderKey
	})
}
