package graph

import (
	"testing"
)

// TestDeepCopy_JSONLimitations tests and documents the limitations of JSON-based deep copy.
// These tests demonstrate the trade-offs and edge cases users should be aware of.
func TestDeepCopy_JSONLimitations(t *testing.T) {
	t.Run("unexported_fields_are_silently_dropped", func(t *testing.T) {
		type StateWithUnexported struct {
			Public  string
			private string // This will NOT be copied
		}

		original := StateWithUnexported{
			Public:  "visible",
			private: "invisible",
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Public field is copied
		if copied.Public != original.Public {
			t.Errorf("Public field not copied: got %q, want %q", copied.Public, original.Public)
		}

		// Unexported field is NOT copied (zero value)
		if copied.private != "" {
			t.Errorf("Unexported field should be empty, got %q", copied.private)
		}

		// This demonstrates the limitation: unexported fields are lost
		if copied.private == original.private {
			t.Errorf("Unexported field was copied (unexpected)")
		}
	})

	t.Run("channels_cannot_be_copied", func(t *testing.T) {
		type StateWithChannel struct {
			Name string
			Ch   chan int
		}

		original := StateWithChannel{
			Name: "test",
			Ch:   make(chan int, 1),
		}

		// JSON cannot marshal channels
		_, err := deepCopy(original)
		if err == nil {
			t.Error("Expected error when copying state with channel, got nil")
		}
	})

	t.Run("functions_cannot_be_copied", func(t *testing.T) {
		type StateWithFunc struct {
			Name string
			Fn   func() string
		}

		original := StateWithFunc{
			Name: "test",
			Fn:   func() string { return "hello" },
		}

		// JSON cannot marshal functions
		_, err := deepCopy(original)
		if err == nil {
			t.Error("Expected error when copying state with function, got nil")
		}
	})

	t.Run("interface_values_lose_concrete_type", func(t *testing.T) {
		type StateWithInterface struct {
			Value interface{}
		}

		// When the concrete type is not known to JSON, it may lose type info
		original := StateWithInterface{
			Value: map[string]interface{}{
				"number": float64(42), // JSON numbers become float64
			},
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Type information is preserved for JSON-compatible types
		if copied.Value == nil {
			t.Error("Value was not copied")
		}
	})

	t.Run("slices_and_maps_are_properly_copied", func(t *testing.T) {
		type StateWithCollections struct {
			Slice []int
			Map   map[string]string
		}

		original := StateWithCollections{
			Slice: []int{1, 2, 3},
			Map:   map[string]string{"key": "value"},
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify deep copy (not shared)
		copied.Slice[0] = 999
		if original.Slice[0] == 999 {
			t.Error("Slice was not deep copied (shared reference)")
		}

		copied.Map["key"] = "modified"
		if original.Map["key"] == "modified" {
			t.Error("Map was not deep copied (shared reference)")
		}
	})

	t.Run("nested_structs_are_properly_copied", func(t *testing.T) {
		type Inner struct {
			Value int
		}
		type StateWithNested struct {
			Name  string
			Inner Inner
		}

		original := StateWithNested{
			Name:  "test",
			Inner: Inner{Value: 42},
		}

		copied, err := deepCopy(original)
		if err != nil {
			t.Fatalf("deepCopy failed: %v", err)
		}

		// Verify independence
		copied.Inner.Value = 999
		if original.Inner.Value == 999 {
			t.Error("Nested struct was not deep copied")
		}
	})
}

// TestStateCopier_Interface tests the StateCopier interface for custom deep copy.
func TestStateCopier_Interface(t *testing.T) {
	t.Run("custom_copier_is_used_when_implemented", func(t *testing.T) {
		// Use benchStateSmall which already implements StateCopier
		original := benchStateSmall{
			ID:      "test-id",
			Counter: 42,
		}

		copied, err := deepCopyState(original)
		if err != nil {
			t.Fatalf("deepCopyState failed: %v", err)
		}

		if copied.ID != original.ID || copied.Counter != original.Counter {
			t.Error("Custom copier did not copy correctly")
		}

		// Verify independence
		copied.Counter = 999
		if original.Counter == 999 {
			t.Error("State was not deep copied")
		}
	})

	t.Run("json_fallback_when_no_custom_copier", func(t *testing.T) {
		type SimpleState struct {
			Value int
		}

		original := SimpleState{Value: 42}

		copied, err := deepCopyState(original)
		if err != nil {
			t.Fatalf("deepCopyState failed: %v", err)
		}

		if copied.Value != original.Value {
			t.Errorf("Value not copied: got %d, want %d", copied.Value, original.Value)
		}

		// Verify independence
		copied.Value = 999
		if original.Value == 999 {
			t.Error("State was not deep copied")
		}
	})
}

// Example states for benchmarking
type benchStateSmall struct {
	ID      string
	Counter int
}

type benchStateMedium struct {
	ID       string
	Counter  int
	Tags     []string
	Metadata map[string]string
}

type benchStateLarge struct {
	ID       string
	Counter  int
	Tags     []string
	Metadata map[string]string
	Data     []byte
	Nested   []benchStateMedium
}

// Custom copier implementations for benchmarking
func (s benchStateSmall) DeepCopy() (benchStateSmall, error) {
	return benchStateSmall{
		ID:      s.ID,
		Counter: s.Counter,
	}, nil
}

func (s benchStateMedium) DeepCopy() (benchStateMedium, error) {
	return benchStateMedium{
		ID:       s.ID,
		Counter:  s.Counter,
		Tags:     append([]string(nil), s.Tags...),
		Metadata: copyMap(s.Metadata),
	}, nil
}

func (s benchStateLarge) DeepCopy() (benchStateLarge, error) {
	nested := make([]benchStateMedium, len(s.Nested))
	for i, n := range s.Nested {
		copied, err := n.DeepCopy()
		if err != nil {
			return benchStateLarge{}, err
		}
		nested[i] = copied
	}

	return benchStateLarge{
		ID:       s.ID,
		Counter:  s.Counter,
		Tags:     append([]string(nil), s.Tags...),
		Metadata: copyMap(s.Metadata),
		Data:     append([]byte(nil), s.Data...),
		Nested:   nested,
	}, nil
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
