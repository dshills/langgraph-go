package consolidator

import (
	"strings"
	"testing"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/types"
)

func TestGenerateMarkdownReport_EmptyState(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 0,
		ConsolidatedIssues: []types.ConsolidatedIssue{},
		Reviews:            map[string][]types.Review{},
	}

	report := GenerateMarkdownReport(state)

	// Should contain header section
	if !strings.Contains(report, "# Code Review Report") {
		t.Error("Report should contain main header")
	}

	// Should show zero files reviewed
	if !strings.Contains(report, "Total Files Reviewed | 0") {
		t.Error("Report should show 0 files reviewed")
	}

	// Should show no issues messages
	if !strings.Contains(report, "No critical issues found") {
		t.Error("Report should contain 'No critical issues found'")
	}
}

func TestGenerateMarkdownReport_SummaryStatistics(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 50,
		ConsolidatedIssues: []types.ConsolidatedIssue{
			{Severity: "critical", Category: "security"},
			{Severity: "critical", Category: "security"},
			{Severity: "high", Category: "performance"},
			{Severity: "medium", Category: "best-practice"},
			{Severity: "low", Category: "style"},
			{Severity: "info", Category: "best-practice"},
		},
		Reviews: map[string][]types.Review{},
	}

	report := GenerateMarkdownReport(state)

	// Check Summary Statistics section exists
	if !strings.Contains(report, "## Summary Statistics") {
		t.Error("Report should contain Summary Statistics section")
	}

	// Check issues by severity table
	if !strings.Contains(report, "Critical Issues | 2") {
		t.Error("Report should show 2 critical issues")
	}
	if !strings.Contains(report, "High Priority Issues | 1") {
		t.Error("Report should show 1 high priority issue")
	}
	if !strings.Contains(report, "Medium Priority Issues | 1") {
		t.Error("Report should show 1 medium priority issue")
	}
	if !strings.Contains(report, "Low Priority Issues | 1") {
		t.Error("Report should show 1 low priority issue")
	}
	if !strings.Contains(report, "Informational Issues | 1") {
		t.Error("Report should show 1 informational issue")
	}
	if !strings.Contains(report, "**Total Issues** | **6**") {
		t.Error("Report should show 6 total issues")
	}
}

func TestGenerateMarkdownReport_CategoryStatistics(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 30,
		ConsolidatedIssues: []types.ConsolidatedIssue{
			{Category: "security"},
			{Category: "security"},
			{Category: "security"},
			{Category: "performance"},
			{Category: "performance"},
			{Category: "best-practice"},
			{Category: "style"},
		},
		Reviews: map[string][]types.Review{},
	}

	report := GenerateMarkdownReport(state)

	// Check By Category section
	if !strings.Contains(report, "### By Category") {
		t.Error("Report should contain 'By Category' section")
	}

	// Check category counts
	if !strings.Contains(report, "Security | 3") {
		t.Error("Report should show 3 security issues")
	}
	if !strings.Contains(report, "Performance | 2") {
		t.Error("Report should show 2 performance issues")
	}
	if !strings.Contains(report, "Best Practices | 1") {
		t.Error("Report should show 1 best-practice issue")
	}
	if !strings.Contains(report, "Style | 1") {
		t.Error("Report should show 1 style issue")
	}
}

func TestGenerateMarkdownReport_ProviderStatistics(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 25,
		ConsolidatedIssues: []types.ConsolidatedIssue{},
		Reviews: map[string][]types.Review{
			"openai": {
				{
					ProviderName: "openai",
					Issues:       make([]types.ReviewIssue, 5),
					TokensUsed:   12500,
					Duration:     45000,
				},
			},
			"anthropic": {
				{
					ProviderName: "anthropic",
					Issues:       make([]types.ReviewIssue, 8),
					TokensUsed:   15000,
					Duration:     50000,
				},
			},
			"google": {
				{
					ProviderName: "google",
					Issues:       make([]types.ReviewIssue, 3),
					TokensUsed:   8000,
					Duration:     30000,
				},
			},
		},
	}

	report := GenerateMarkdownReport(state)

	// Check Provider Statistics section
	if !strings.Contains(report, "### Provider Statistics") {
		t.Error("Report should contain 'Provider Statistics' section")
	}

	// Check table headers
	if !strings.Contains(report, "| Provider | Issues Found | Tokens Used | Duration |") {
		t.Error("Report should contain provider statistics table headers")
	}

	// Check OpenAI row (providers should be sorted alphabetically)
	if !strings.Contains(report, "openai | 5 | 12,500 | 45.0s") {
		t.Error("Report should contain OpenAI statistics")
	}

	// Check Anthropic row
	if !strings.Contains(report, "anthropic | 8 | 15,000 | 50.0s") {
		t.Error("Report should contain Anthropic statistics")
	}

	// Check Google row
	if !strings.Contains(report, "google | 3 | 8,000 | 30.0s") {
		t.Error("Report should contain Google statistics")
	}
}

func TestGenerateMarkdownReport_IssueProviderAttribution(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 10,
		ConsolidatedIssues: []types.ConsolidatedIssue{
			{
				File:           "main.go",
				Line:           42,
				Severity:       "critical",
				Category:       "security",
				Description:    "SQL injection vulnerability",
				Remediation:    "Use parameterized queries",
				Providers:      []string{"anthropic", "openai"}, // Pre-sorted
				ConsensusScore: 0.67,
			},
		},
		Reviews: map[string][]types.Review{
			"openai":    {{ProviderName: "openai"}},
			"anthropic": {{ProviderName: "anthropic"}},
			"google":    {{ProviderName: "google"}},
		},
	}

	report := GenerateMarkdownReport(state)

	// Check provider attribution is present
	if !strings.Contains(report, "**Providers**: anthropic, openai") {
		t.Error("Report should show sorted provider names")
	}

	// Check consensus score formatting
	if !strings.Contains(report, "**Consensus**: 2/3 providers (67%)") {
		t.Error("Report should show consensus as '2/3 providers (67%)'")
	}
}

func TestGenerateMarkdownReport_IssuesBySeverity(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 15,
		ConsolidatedIssues: []types.ConsolidatedIssue{
			{
				File:        "handler.go",
				Line:        100,
				Severity:    "high",
				Category:    "performance",
				Description: "N+1 query problem",
				Remediation: "Use eager loading",
				Providers:   []string{"openai"},
			},
			{
				File:        "auth.go",
				Line:        50,
				Severity:    "critical",
				Category:    "security",
				Description: "Missing authentication check",
				Remediation: "Add middleware",
				Providers:   []string{"anthropic", "google", "openai"},
			},
		},
		Reviews: map[string][]types.Review{},
	}

	report := GenerateMarkdownReport(state)

	// Check that critical section comes before high
	criticalIdx := strings.Index(report, "## Critical Issues")
	highIdx := strings.Index(report, "## High Priority Issues")

	if criticalIdx == -1 {
		t.Error("Report should contain Critical Issues section")
	}
	if highIdx == -1 {
		t.Error("Report should contain High Priority Issues section")
	}
	if criticalIdx >= highIdx {
		t.Error("Critical Issues section should come before High Priority Issues")
	}

	// Check issue count in section header
	if !strings.Contains(report, "## Critical Issues (1)") {
		t.Error("Report should show '(1)' in Critical Issues header")
	}
	if !strings.Contains(report, "## High Priority Issues (1)") {
		t.Error("Report should show '(1)' in High Priority Issues header")
	}

	// Check issue details are present
	if !strings.Contains(report, "Missing authentication check") {
		t.Error("Report should contain critical issue description")
	}
	if !strings.Contains(report, "N+1 query problem") {
		t.Error("Report should contain high priority issue description")
	}
}

func TestGenerateMarkdownReport_ConsensusScoreFormatting(t *testing.T) {
	tests := []struct {
		name           string
		consensusScore float64
		providers      []string
		totalProviders int
		wantFormat     string
	}{
		{
			name:           "All providers agree (3/3)",
			consensusScore: 1.0,
			providers:      []string{"anthropic", "google", "openai"},
			totalProviders: 3,
			wantFormat:     "3/3 providers (100%)",
		},
		{
			name:           "Two thirds agree (2/3)",
			consensusScore: 0.67,
			providers:      []string{"anthropic", "openai"},
			totalProviders: 3,
			wantFormat:     "2/3 providers (67%)",
		},
		{
			name:           "One third agrees (1/3)",
			consensusScore: 0.33,
			providers:      []string{"openai"},
			totalProviders: 3,
			wantFormat:     "1/3 providers (33%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := types.ReviewState{
				CodebaseRoot:       "/test/project",
				TotalFilesReviewed: 10,
				ConsolidatedIssues: []types.ConsolidatedIssue{
					{
						File:           "test.go",
						Line:           1,
						Severity:       "high",
						Category:       "security",
						Description:    "Test issue",
						Providers:      tt.providers,
						ConsensusScore: tt.consensusScore,
					},
				},
				Reviews: map[string][]types.Review{
					"openai":    {{ProviderName: "openai"}},
					"anthropic": {{ProviderName: "anthropic"}},
					"google":    {{ProviderName: "google"}},
				},
			}

			report := GenerateMarkdownReport(state)

			if !strings.Contains(report, tt.wantFormat) {
				t.Errorf("Report should contain consensus format '%s', got report:\n%s",
					tt.wantFormat, report)
			}
		})
	}
}

func TestGenerateMarkdownReport_EmptySeveritySections(t *testing.T) {
	state := types.ReviewState{
		CodebaseRoot:       "/test/project",
		TotalFilesReviewed: 5,
		ConsolidatedIssues: []types.ConsolidatedIssue{
			{Severity: "critical"},
			{Severity: "info"},
		},
		Reviews: map[string][]types.Review{},
	}

	report := GenerateMarkdownReport(state)

	// Should have critical and info sections
	if !strings.Contains(report, "## Critical Issues") {
		t.Error("Report should contain Critical Issues section")
	}
	if !strings.Contains(report, "## Informational Issues") {
		t.Error("Report should contain Informational Issues section")
	}

	// Should show "No X issues found" for missing severities
	if !strings.Contains(report, "No high priority issues found") {
		t.Error("Report should contain 'No high priority issues found'")
	}
	if !strings.Contains(report, "No medium priority issues found") {
		t.Error("Report should contain 'No medium priority issues found'")
	}
	if !strings.Contains(report, "No low priority issues found") {
		t.Error("Report should contain 'No low priority issues found'")
	}
}
