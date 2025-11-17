package main

import (
	"context"
	"testing"
	"time"
)

func TestWeatherTool_Name(t *testing.T) {
	tool := &WeatherTool{}
	expected := "get_weather"

	if tool.Name() != expected {
		t.Errorf("Name() = %q, want %q", tool.Name(), expected)
	}
}

func TestWeatherTool_Call_Success(t *testing.T) {
	tool := &WeatherTool{}
	ctx := context.Background()

	tests := []struct {
		name     string
		location string
	}{
		{"city name", "San Francisco"},
		{"zip code", "94102"},
		{"international", "London"},
		{"with spaces", "New York"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]interface{}{
				"location": tt.location,
			}

			output, err := tool.Call(ctx, input)
			if err != nil {
				t.Fatalf("Call() error = %v, want nil", err)
			}

			// Verify output structure
			if output == nil {
				t.Fatal("Call() returned nil output")
			}

			// Verify required fields are present
			if location, ok := output["location"].(string); !ok {
				t.Error("output missing 'location' field or wrong type")
			} else if location != tt.location {
				t.Errorf("output location = %q, want %q", location, tt.location)
			}

			if _, ok := output["temperature"].(int); !ok {
				t.Error("output missing 'temperature' field or wrong type")
			}

			if _, ok := output["conditions"].(string); !ok {
				t.Error("output missing 'conditions' field or wrong type")
			}

			if _, ok := output["humidity"].(int); !ok {
				t.Error("output missing 'humidity' field or wrong type")
			}
		})
	}
}

func TestWeatherTool_Call_MissingLocation(t *testing.T) {
	tool := &WeatherTool{}
	ctx := context.Background()

	tests := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name:  "empty input",
			input: map[string]interface{}{},
		},
		{
			name:  "wrong type",
			input: map[string]interface{}{"location": 123},
		},
		{
			name:  "empty string",
			input: map[string]interface{}{"location": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tool.Call(ctx, tt.input)
			if err == nil {
				t.Error("Call() error = nil, want error for missing/invalid location")
			}
			if output != nil {
				t.Errorf("Call() output = %v, want nil on error", output)
			}
		})
	}
}

func TestWeatherTool_Call_ContextCancellation(t *testing.T) {
	tool := &WeatherTool{}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := map[string]interface{}{
		"location": "San Francisco",
	}

	output, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want error for cancelled context")
	}
	if output != nil {
		t.Errorf("Call() output = %v, want nil on context error", output)
	}

	// Verify it's a context error
	if err != context.Canceled {
		t.Errorf("Call() error = %v, want context.Canceled", err)
	}
}

func TestWeatherTool_Call_ContextTimeout(t *testing.T) {
	tool := &WeatherTool{}

	// Create a context with a timeout that expires immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for timeout to trigger
	time.Sleep(1 * time.Millisecond)

	input := map[string]interface{}{
		"location": "San Francisco",
	}

	output, err := tool.Call(ctx, input)
	if err == nil {
		t.Error("Call() error = nil, want error for timeout context")
	}
	if output != nil {
		t.Errorf("Call() output = %v, want nil on context error", output)
	}

	// Verify it's a deadline exceeded error
	if err != context.DeadlineExceeded {
		t.Errorf("Call() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestReducer(t *testing.T) {
	tests := []struct {
		name  string
		prev  State
		delta State
		want  State
	}{
		{
			name:  "empty to full",
			prev:  State{},
			delta: State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
			want:  State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
		},
		{
			name:  "partial update",
			prev:  State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
			delta: State{Temperature: 75, LastQuery: "q2"},
			want:  State{Location: "SF", Temperature: 75, Conditions: "sunny", Humidity: 65, LastQuery: "q2"},
		},
		{
			name:  "location change",
			prev:  State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
			delta: State{Location: "NYC", Temperature: 65, Conditions: "cloudy", Humidity: 70, LastQuery: "q2"},
			want:  State{Location: "NYC", Temperature: 65, Conditions: "cloudy", Humidity: 70, LastQuery: "q2"},
		},
		{
			name:  "zero values don't update",
			prev:  State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
			delta: State{},
			want:  State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65, LastQuery: "q1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reducer(tt.prev, tt.delta)

			if got.Location != tt.want.Location {
				t.Errorf("reducer() Location = %q, want %q", got.Location, tt.want.Location)
			}
			if got.Temperature != tt.want.Temperature {
				t.Errorf("reducer() Temperature = %d, want %d", got.Temperature, tt.want.Temperature)
			}
			if got.Conditions != tt.want.Conditions {
				t.Errorf("reducer() Conditions = %q, want %q", got.Conditions, tt.want.Conditions)
			}
			if got.Humidity != tt.want.Humidity {
				t.Errorf("reducer() Humidity = %d, want %d", got.Humidity, tt.want.Humidity)
			}
			if got.LastQuery != tt.want.LastQuery {
				t.Errorf("reducer() LastQuery = %q, want %q", got.LastQuery, tt.want.LastQuery)
			}
		})
	}
}

func TestReducer_Idempotency(t *testing.T) {
	// Verify that applying the same delta multiple times produces the same result
	prev := State{Location: "SF", Temperature: 72}
	delta := State{Temperature: 75, Conditions: "sunny"}

	result1 := reducer(prev, delta)
	result2 := reducer(result1, delta)

	if result1.Temperature != result2.Temperature {
		t.Error("reducer() is not idempotent")
	}
}

func BenchmarkWeatherTool_Call(b *testing.B) {
	tool := &WeatherTool{}
	ctx := context.Background()
	input := map[string]interface{}{
		"location": "San Francisco",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Call(ctx, input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReducer(b *testing.B) {
	prev := State{Location: "SF", Temperature: 72, Conditions: "sunny", Humidity: 65}
	delta := State{Temperature: 75, LastQuery: "q1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reducer(prev, delta)
	}
}
