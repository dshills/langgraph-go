package consolidator

import (
	"testing"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/types"
)

// TestSortBySeverity_Ordering verifies that issues are sorted by severity
// from critical to info.
func TestSortBySeverity_Ordering(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "low", File: "a.go", Line: 1},
		{IssueID: "2", Severity: "critical", File: "b.go", Line: 2},
		{IssueID: "3", Severity: "info", File: "c.go", Line: 3},
		{IssueID: "4", Severity: "high", File: "d.go", Line: 4},
		{IssueID: "5", Severity: "medium", File: "e.go", Line: 5},
	}

	SortBySeverity(issues)

	// Expected order: critical > high > medium > low > info
	expected := []string{"critical", "high", "medium", "low", "info"}
	for i, exp := range expected {
		if issues[i].Severity != exp {
			t.Errorf("issues[%d].Severity = %q, want %q", i, issues[i].Severity, exp)
		}
	}
}

// TestSortBySeverity_AllCritical verifies that all critical issues stay in order.
func TestSortBySeverity_AllCritical(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "critical", File: "a.go", Line: 1},
		{IssueID: "2", Severity: "critical", File: "b.go", Line: 2},
		{IssueID: "3", Severity: "critical", File: "c.go", Line: 3},
	}

	SortBySeverity(issues)

	// All should remain critical
	for i, issue := range issues {
		if issue.Severity != "critical" {
			t.Errorf("issues[%d].Severity = %q, want %q", i, issue.Severity, "critical")
		}
	}
}

// TestSortBySeverity_StableSort verifies that issues with the same severity
// maintain their original relative order.
func TestSortBySeverity_StableSort(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "high", File: "a.go", Line: 1},
		{IssueID: "2", Severity: "high", File: "b.go", Line: 2},
		{IssueID: "3", Severity: "high", File: "c.go", Line: 3},
	}

	SortBySeverity(issues)

	// Order should be preserved since all have same severity
	expectedIDs := []string{"1", "2", "3"}
	for i, expID := range expectedIDs {
		if issues[i].IssueID != expID {
			t.Errorf("issues[%d].IssueID = %q, want %q", i, issues[i].IssueID, expID)
		}
	}
}

// TestSortBySeverity_EmptyList verifies that sorting an empty list doesn't panic.
func TestSortBySeverity_EmptyList(t *testing.T) {
	var issues []types.ConsolidatedIssue
	SortBySeverity(issues) // Should not panic
}

// TestSortBySeverity_SingleIssue verifies that a single issue list works correctly.
func TestSortBySeverity_SingleIssue(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "medium", File: "a.go", Line: 1},
	}

	SortBySeverity(issues)

	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].Severity != "medium" {
		t.Errorf("issues[0].Severity = %q, want %q", issues[0].Severity, "medium")
	}
}

// TestSortBySeverity_MixedWithDuplicates verifies that duplicate severities
// are handled correctly.
func TestSortBySeverity_MixedWithDuplicates(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "info", File: "a.go", Line: 1},
		{IssueID: "2", Severity: "critical", File: "b.go", Line: 2},
		{IssueID: "3", Severity: "info", File: "c.go", Line: 3},
		{IssueID: "4", Severity: "critical", File: "d.go", Line: 4},
		{IssueID: "5", Severity: "low", File: "e.go", Line: 5},
	}

	SortBySeverity(issues)

	// First two should be critical
	if issues[0].Severity != "critical" || issues[1].Severity != "critical" {
		t.Errorf("First two issues should be critical, got %q and %q",
			issues[0].Severity, issues[1].Severity)
	}
	// Last two should be info
	if issues[3].Severity != "info" || issues[4].Severity != "info" {
		t.Errorf("Last two issues should be info, got %q and %q",
			issues[3].Severity, issues[4].Severity)
	}
}

// TestSortByConsensus_Ordering verifies that issues are sorted by consensus score
// in descending order (1.0 > 0.67 > 0.33).
func TestSortByConsensus_Ordering(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", ConsensusScore: 0.33, File: "a.go", Line: 1},
		{IssueID: "2", ConsensusScore: 1.0, File: "b.go", Line: 2},
		{IssueID: "3", ConsensusScore: 0.67, File: "c.go", Line: 3},
	}

	SortByConsensus(issues)

	// Expected order: 1.0 > 0.67 > 0.33
	expected := []float64{1.0, 0.67, 0.33}
	for i, exp := range expected {
		if issues[i].ConsensusScore != exp {
			t.Errorf("issues[%d].ConsensusScore = %f, want %f",
				i, issues[i].ConsensusScore, exp)
		}
	}
}

// TestSortByConsensus_HigherConsensusFirst verifies that higher consensus
// appears first within the same severity level.
func TestSortByConsensus_HigherConsensusFirst(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "high", ConsensusScore: 0.33, File: "a.go"},
		{IssueID: "2", Severity: "high", ConsensusScore: 1.0, File: "b.go"},
		{IssueID: "3", Severity: "high", ConsensusScore: 0.67, File: "c.go"},
	}

	SortByConsensus(issues)

	// All same severity, so consensus determines order
	if issues[0].ConsensusScore != 1.0 {
		t.Errorf("First issue should have consensus 1.0, got %f", issues[0].ConsensusScore)
	}
	if issues[1].ConsensusScore != 0.67 {
		t.Errorf("Second issue should have consensus 0.67, got %f", issues[1].ConsensusScore)
	}
	if issues[2].ConsensusScore != 0.33 {
		t.Errorf("Third issue should have consensus 0.33, got %f", issues[2].ConsensusScore)
	}
}

// TestSortByConsensus_StableSort verifies that issues with the same consensus score
// maintain their original relative order.
func TestSortByConsensus_StableSort(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", ConsensusScore: 0.67, File: "a.go", Line: 1},
		{IssueID: "2", ConsensusScore: 0.67, File: "b.go", Line: 2},
		{IssueID: "3", ConsensusScore: 0.67, File: "c.go", Line: 3},
	}

	SortByConsensus(issues)

	// Order should be preserved since all have same consensus
	expectedIDs := []string{"1", "2", "3"}
	for i, expID := range expectedIDs {
		if issues[i].IssueID != expID {
			t.Errorf("issues[%d].IssueID = %q, want %q", i, issues[i].IssueID, expID)
		}
	}
}

// TestSortByConsensus_EmptyList verifies that sorting an empty list doesn't panic.
func TestSortByConsensus_EmptyList(t *testing.T) {
	var issues []types.ConsolidatedIssue
	SortByConsensus(issues) // Should not panic
}

// TestSortByConsensus_SingleIssue verifies that a single issue list works correctly.
func TestSortByConsensus_SingleIssue(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", ConsensusScore: 0.5, File: "a.go", Line: 1},
	}

	SortByConsensus(issues)

	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}
	if issues[0].ConsensusScore != 0.5 {
		t.Errorf("issues[0].ConsensusScore = %f, want 0.5", issues[0].ConsensusScore)
	}
}

// TestSortIssues_SeverityThenConsensus verifies that issues are sorted first
// by severity, then by consensus within each severity level.
func TestSortIssues_SeverityThenConsensus(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "medium", ConsensusScore: 1.0, File: "a.go"},
		{IssueID: "2", Severity: "critical", ConsensusScore: 0.33, File: "b.go"},
		{IssueID: "3", Severity: "critical", ConsensusScore: 1.0, File: "c.go"},
		{IssueID: "4", Severity: "low", ConsensusScore: 1.0, File: "d.go"},
		{IssueID: "5", Severity: "critical", ConsensusScore: 0.67, File: "e.go"},
	}

	SortIssues(issues)

	// Expected order:
	// 1. critical with 1.0 consensus (ID: 3)
	// 2. critical with 0.67 consensus (ID: 5)
	// 3. critical with 0.33 consensus (ID: 2)
	// 4. medium with 1.0 consensus (ID: 1)
	// 5. low with 1.0 consensus (ID: 4)
	expectedIDs := []string{"3", "5", "2", "1", "4"}
	for i, expID := range expectedIDs {
		if issues[i].IssueID != expID {
			t.Errorf("issues[%d].IssueID = %q, want %q", i, issues[i].IssueID, expID)
		}
	}
}

// TestSortIssues_AllSameSeverity verifies that consensus determines order
// when all issues have the same severity.
func TestSortIssues_AllSameSeverity(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "high", ConsensusScore: 0.33, File: "a.go"},
		{IssueID: "2", Severity: "high", ConsensusScore: 1.0, File: "b.go"},
		{IssueID: "3", Severity: "high", ConsensusScore: 0.67, File: "c.go"},
	}

	SortIssues(issues)

	// Should be sorted by consensus: 1.0 > 0.67 > 0.33
	expectedConsensus := []float64{1.0, 0.67, 0.33}
	for i, expCons := range expectedConsensus {
		if issues[i].ConsensusScore != expCons {
			t.Errorf("issues[%d].ConsensusScore = %f, want %f",
				i, issues[i].ConsensusScore, expCons)
		}
	}
}

// TestSortIssues_ComplexMix verifies correct sorting with a complex mix
// of severities and consensus scores.
func TestSortIssues_ComplexMix(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "info", ConsensusScore: 1.0, File: "a.go"},
		{IssueID: "2", Severity: "high", ConsensusScore: 0.5, File: "b.go"},
		{IssueID: "3", Severity: "critical", ConsensusScore: 0.33, File: "c.go"},
		{IssueID: "4", Severity: "medium", ConsensusScore: 0.67, File: "d.go"},
		{IssueID: "5", Severity: "high", ConsensusScore: 1.0, File: "e.go"},
		{IssueID: "6", Severity: "low", ConsensusScore: 0.5, File: "f.go"},
		{IssueID: "7", Severity: "critical", ConsensusScore: 1.0, File: "g.go"},
	}

	SortIssues(issues)

	// Expected order by severity first, then consensus:
	// critical: 7 (1.0), 3 (0.33)
	// high: 5 (1.0), 2 (0.5)
	// medium: 4 (0.67)
	// low: 6 (0.5)
	// info: 1 (1.0)
	expectedIDs := []string{"7", "3", "5", "2", "4", "6", "1"}
	for i, expID := range expectedIDs {
		if issues[i].IssueID != expID {
			t.Errorf("issues[%d].IssueID = %q, want %q (severity=%s, consensus=%.2f)",
				i, issues[i].IssueID, expID, issues[i].Severity, issues[i].ConsensusScore)
		}
	}
}

// TestSortIssues_EmptyList verifies that sorting an empty list doesn't panic.
func TestSortIssues_EmptyList(t *testing.T) {
	var issues []types.ConsolidatedIssue
	SortIssues(issues) // Should not panic
}

// TestSortIssues_SingleIssue verifies that a single issue list works correctly.
func TestSortIssues_SingleIssue(t *testing.T) {
	issues := []types.ConsolidatedIssue{
		{IssueID: "1", Severity: "medium", ConsensusScore: 0.5, File: "a.go"},
	}

	SortIssues(issues)

	if len(issues) != 1 {
		t.Errorf("len(issues) = %d, want 1", len(issues))
	}
}
