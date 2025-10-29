package workflow

import (
	"context"
	"testing"
)

// TestConsolidateNode_BasicFunctionality tests the ConsolidateNode implementation
// This is a standalone test to verify the implementation works despite the import cycle issue
func TestConsolidateNode_BasicFunctionality(t *testing.T) {
	ctx := context.Background()

	// Test Case 1: Merge duplicates
	t.Run("merges duplicate issues", func(t *testing.T) {
		state := ReviewState{
			Reviews: map[string][]Review{
				"openai": {
					{
						ProviderName: "openai",
						BatchNumber:  1,
						Issues: []ReviewIssue{
							{
								File:         "main.go",
								Line:         42,
								Severity:     "high",
								Category:     "security",
								Description:  "SQL injection vulnerability",
								Remediation:  "Use parameterized queries",
								ProviderName: "openai",
							},
						},
					},
				},
				"anthropic": {
					{
						ProviderName: "anthropic",
						BatchNumber:  1,
						Issues: []ReviewIssue{
							{
								File:         "main.go",
								Line:         42,
								Severity:     "critical",
								Category:     "security",
								Description:  "SQL injection vulnerability",
								Remediation:  "Use prepared statements",
								ProviderName: "anthropic",
							},
						},
					},
				},
			},
		}

		node := ConsolidateNode{}
		result := node.Run(ctx, state)

		if result.Err != nil {
			t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
		}

		if len(result.Delta.ConsolidatedIssues) != 1 {
			t.Fatalf("Expected 1 consolidated issue, got %d", len(result.Delta.ConsolidatedIssues))
		}

		issue := result.Delta.ConsolidatedIssues[0]

		if len(issue.Providers) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(issue.Providers))
		}

		if issue.ConsensusScore != 1.0 {
			t.Errorf("Expected ConsensusScore=1.0, got %.2f", issue.ConsensusScore)
		}

		if issue.Severity != "critical" {
			t.Errorf("Expected Severity='critical', got '%s'", issue.Severity)
		}

		if len(issue.IssueID) != 8 {
			t.Errorf("Expected IssueID length=8, got %d", len(issue.IssueID))
		}

		if result.Route.To != "report" {
			t.Errorf("Expected Route.To='report', got '%s'", result.Route.To)
		}
	})

	// Test Case 2: Keep unique issues separate
	t.Run("keeps unique issues separate", func(t *testing.T) {
		state := ReviewState{
			Reviews: map[string][]Review{
				"openai": {
					{
						ProviderName: "openai",
						BatchNumber:  1,
						Issues: []ReviewIssue{
							{
								File:         "main.go",
								Line:         42,
								Severity:     "high",
								Description:  "Issue A",
								ProviderName: "openai",
							},
							{
								File:         "main.go",
								Line:         50,
								Severity:     "medium",
								Description:  "Issue B",
								ProviderName: "openai",
							},
						},
					},
				},
			},
		}

		node := ConsolidateNode{}
		result := node.Run(ctx, state)

		if result.Err != nil {
			t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
		}

		if len(result.Delta.ConsolidatedIssues) != 2 {
			t.Fatalf("Expected 2 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
		}
	})

	// Test Case 3: Empty reviews
	t.Run("handles empty reviews", func(t *testing.T) {
		state := ReviewState{
			Reviews: map[string][]Review{},
		}

		node := ConsolidateNode{}
		result := node.Run(ctx, state)

		if result.Err != nil {
			t.Fatalf("ConsolidateNode.Run() returned error: %v", result.Err)
		}

		if len(result.Delta.ConsolidatedIssues) != 0 {
			t.Errorf("Expected 0 consolidated issues, got %d", len(result.Delta.ConsolidatedIssues))
		}

		if result.Route.To != "report" {
			t.Errorf("Expected Route.To='report', got '%s'", result.Route.To)
		}
	})
}
