package graph

import (
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
