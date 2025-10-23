package graph

import "testing"

// TestEdge_Struct verifies Edge[S] struct and predicates (T024).
func TestEdge_Struct(t *testing.T) {
	t.Run("unconditional edge", func(t *testing.T) {
		edge := Edge[TestState]{
			From: "node1",
			To:   "node2",
			When: nil, // unconditional
		}

		if edge.From != "node1" {
			t.Errorf("expected From = 'node1', got %q", edge.From)
		}
		if edge.To != "node2" {
			t.Errorf("expected To = 'node2', got %q", edge.To)
		}
		if edge.When != nil {
			t.Error("expected When = nil for unconditional edge")
		}
	})

	t.Run("conditional edge with predicate", func(t *testing.T) {
		predicate := func(s TestState) bool {
			return s.Counter > 10
		}

		edge := Edge[TestState]{
			From: "node1",
			To:   "node2",
			When: predicate,
		}

		// Test predicate evaluation
		if !edge.When(TestState{Counter: 15}) {
			t.Error("predicate should return true for Counter = 15")
		}
		if edge.When(TestState{Counter: 5}) {
			t.Error("predicate should return false for Counter = 5")
		}
	})

	t.Run("predicate with string condition", func(t *testing.T) {
		predicate := func(s TestState) bool {
			return s.Value == "ready"
		}

		edge := Edge[TestState]{
			From: "check",
			To:   "process",
			When: predicate,
		}

		if !edge.When(TestState{Value: "ready"}) {
			t.Error("predicate should return true for Value = 'ready'")
		}
		if edge.When(TestState{Value: "not-ready"}) {
			t.Error("predicate should return false for Value = 'not-ready'")
		}
	})

	t.Run("complex predicate", func(t *testing.T) {
		predicate := func(s TestState) bool {
			return s.Counter > 0 && s.Value != ""
		}

		edge := Edge[TestState]{
			From: "start",
			To:   "end",
			When: predicate,
		}

		// Both conditions met
		if !edge.When(TestState{Counter: 1, Value: "test"}) {
			t.Error("predicate should return true when both conditions met")
		}

		// Only one condition met
		if edge.When(TestState{Counter: 1, Value: ""}) {
			t.Error("predicate should return false when Value empty")
		}
		if edge.When(TestState{Counter: 0, Value: "test"}) {
			t.Error("predicate should return false when Counter zero")
		}
	})
}

// TestPredicate_Type verifies Predicate[S] function type (T026).
func TestPredicate_Type(t *testing.T) {
	t.Run("predicate type can be declared", func(t *testing.T) {
		var pred Predicate[TestState]

		pred = func(s TestState) bool {
			return s.Counter > 0
		}

		if !pred(TestState{Counter: 5}) {
			t.Error("predicate should return true for Counter = 5")
		}
		if pred(TestState{Counter: 0}) {
			t.Error("predicate should return false for Counter = 0")
		}
	})

	t.Run("predicate can be nil", func(t *testing.T) {
		var pred Predicate[TestState]

		if pred != nil {
			t.Error("uninitialized predicate should be nil")
		}

		// Nil predicate represents unconditional edge
		edge := Edge[TestState]{
			From: "a",
			To:   "b",
			When: pred,
		}

		if edge.When != nil {
			t.Error("edge with nil predicate should have When = nil")
		}
	})

	t.Run("predicate composition", func(t *testing.T) {
		// Multiple predicates can be composed
		isPositive := func(s TestState) bool {
			return s.Counter > 0
		}
		hasValue := func(s TestState) bool {
			return s.Value != ""
		}

		// AND composition
		bothTrue := func(s TestState) bool {
			return isPositive(s) && hasValue(s)
		}

		if !bothTrue(TestState{Counter: 1, Value: "test"}) {
			t.Error("composed predicate should return true when both conditions met")
		}
		if bothTrue(TestState{Counter: 0, Value: "test"}) {
			t.Error("composed predicate should return false when first condition fails")
		}
	})
}

// TestEdge_MultipleEdges verifies multiple edges from same node (T024).
func TestEdge_MultipleEdges(t *testing.T) {
	t.Run("fan-out with predicates", func(t *testing.T) {
		// Simulate routing logic with multiple edges from same node
		edges := []Edge[TestState]{
			{
				From: "router",
				To:   "path-a",
				When: func(s TestState) bool { return s.Counter < 10 },
			},
			{
				From: "router",
				To:   "path-b",
				When: func(s TestState) bool { return s.Counter >= 10 },
			},
		}

		// Test routing for Counter = 5 (should go to path-a)
		state1 := TestState{Counter: 5}
		var selected string
		for _, edge := range edges {
			if edge.When == nil || edge.When(state1) {
				selected = edge.To
				break
			}
		}
		if selected != "path-a" {
			t.Errorf("expected route to 'path-a', got %q", selected)
		}

		// Test routing for Counter = 15 (should go to path-b)
		state2 := TestState{Counter: 15}
		selected = ""
		for _, edge := range edges {
			if edge.When == nil || edge.When(state2) {
				selected = edge.To
				break
			}
		}
		if selected != "path-b" {
			t.Errorf("expected route to 'path-b', got %q", selected)
		}
	})
}
