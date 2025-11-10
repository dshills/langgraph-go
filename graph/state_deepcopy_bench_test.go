package graph

import (
	"testing"
)

// Benchmark deepCopy performance with different state sizes
// These benchmarks measure the cost of JSON-based deep copy vs custom copiers

func BenchmarkDeepCopy_Small_JSON(b *testing.B) {
	state := benchStateSmall{
		ID:      "test-id-12345",
		Counter: 42,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopy(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepCopy_Small_CustomCopier(b *testing.B) {
	state := benchStateSmall{
		ID:      "test-id-12345",
		Counter: 42,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopyState(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepCopy_Medium_JSON(b *testing.B) {
	state := benchStateMedium{
		ID:      "test-id-12345",
		Counter: 42,
		Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopy(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepCopy_Medium_CustomCopier(b *testing.B) {
	state := benchStateMedium{
		ID:      "test-id-12345",
		Counter: 42,
		Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopyState(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepCopy_Large_JSON(b *testing.B) {
	// Create a realistic large state
	nested := make([]benchStateMedium, 10)
	for i := range nested {
		nested[i] = benchStateMedium{
			ID:      "nested-id",
			Counter: i,
			Tags:    []string{"tag1", "tag2", "tag3"},
			Metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}
	}

	state := benchStateLarge{
		ID:       "test-id-12345",
		Counter:  42,
		Tags:     []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
		Data:     make([]byte, 1024), // 1KB of data
		Nested:   nested,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopy(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeepCopy_Large_CustomCopier(b *testing.B) {
	// Create a realistic large state
	nested := make([]benchStateMedium, 10)
	for i := range nested {
		nested[i] = benchStateMedium{
			ID:      "nested-id",
			Counter: i,
			Tags:    []string{"tag1", "tag2", "tag3"},
			Metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		}
	}

	state := benchStateLarge{
		ID:       "test-id-12345",
		Counter:  42,
		Tags:     []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
		Data:     make([]byte, 1024), // 1KB of data
		Nested:   nested,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := deepCopyState(state)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark fan-out scenario (multiple concurrent copies)
func BenchmarkDeepCopy_FanOut_JSON(b *testing.B) {
	state := benchStateMedium{
		ID:      "test-id-12345",
		Counter: 42,
		Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	fanOutSize := 4 // Typical fan-out size

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < fanOutSize; j++ {
			_, err := deepCopy(state)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDeepCopy_FanOut_CustomCopier(b *testing.B) {
	state := benchStateMedium{
		ID:      "test-id-12345",
		Counter: 42,
		Tags:    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	fanOutSize := 4 // Typical fan-out size

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < fanOutSize; j++ {
			_, err := deepCopyState(state)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
