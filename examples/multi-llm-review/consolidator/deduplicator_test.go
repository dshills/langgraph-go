package consolidator

import (
	"testing"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/types"
)

// TestDeduplicateIssues_ExactMatch tests exact match deduplication
// Stage 1: Exact match (same file + same line + identical description)
func TestDeduplicateIssues_ExactMatch(t *testing.T) {
	tests := []struct {
		name           string
		issues         []types.ReviewIssue
		totalProviders int
		want           []types.ConsolidatedIssue
	}{
		{
			name: "exact duplicates - same file, line, and description",
			issues: []types.ReviewIssue{
				{
					File:         "main.go",
					Line:         42,
					Severity:     "high",
					Category:     "security",
					Description:  "Potential nil pointer dereference",
					Remediation:  "Add nil check before accessing pointer",
					ProviderName: "openai",
				},
				{
					File:         "main.go",
					Line:         42,
					Severity:     "high",
					Category:     "security",
					Description:  "Potential nil pointer dereference",
					Remediation:  "Add nil check before accessing pointer",
					ProviderName: "anthropic",
				},
				{
					File:         "main.go",
					Line:         42,
					Severity:     "medium",
					Category:     "security",
					Description:  "Potential nil pointer dereference",
					Remediation:  "Add nil check before accessing pointer",
					ProviderName: "google",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "main.go",
					Line:           42,
					Severity:       "high", // Highest severity
					Category:       "security",
					Description:    "Potential nil pointer dereference",
					Remediation:    "Add nil check before accessing pointer",
					Providers:      []string{"openai", "anthropic", "google"},
					ConsensusScore: 1.0, // All 3 providers
				},
			},
		},
		{
			name: "no duplicates - different files",
			issues: []types.ReviewIssue{
				{
					File:         "main.go",
					Line:         10,
					Severity:     "high",
					Category:     "security",
					Description:  "Missing input validation",
					Remediation:  "Add validation",
					ProviderName: "openai",
				},
				{
					File:         "handler.go",
					Line:         10,
					Severity:     "high",
					Category:     "security",
					Description:  "Missing input validation",
					Remediation:  "Add validation",
					ProviderName: "anthropic",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "handler.go", // Alphabetically sorted
					Line:           10,
					Severity:       "high",
					Category:       "security",
					Description:    "Missing input validation",
					Remediation:    "Add validation",
					Providers:      []string{"anthropic"},
					ConsensusScore: 1.0 / 3.0,
				},
				{
					File:           "main.go", // Alphabetically sorted
					Line:           10,
					Severity:       "high",
					Category:       "security",
					Description:    "Missing input validation",
					Remediation:    "Add validation",
					Providers:      []string{"openai"},
					ConsensusScore: 1.0 / 3.0,
				},
			},
		},
		{
			name: "no duplicates - different lines",
			issues: []types.ReviewIssue{
				{
					File:         "main.go",
					Line:         10,
					Severity:     "high",
					Category:     "security",
					Description:  "Missing nil check",
					Remediation:  "Add check",
					ProviderName: "openai",
				},
				{
					File:         "main.go",
					Line:         20,
					Severity:     "high",
					Category:     "security",
					Description:  "Missing nil check",
					Remediation:  "Add check",
					ProviderName: "anthropic",
				},
			},
			totalProviders: 2,
			want: []types.ConsolidatedIssue{
				{
					File:           "main.go",
					Line:           10,
					Severity:       "high",
					Category:       "security",
					Description:    "Missing nil check",
					Remediation:    "Add check",
					Providers:      []string{"openai"},
					ConsensusScore: 0.5,
				},
				{
					File:           "main.go",
					Line:           20,
					Severity:       "high",
					Category:       "security",
					Description:    "Missing nil check",
					Remediation:    "Add check",
					Providers:      []string{"anthropic"},
					ConsensusScore: 0.5,
				},
			},
		},
		{
			name: "partial duplicates - 2 out of 3 providers",
			issues: []types.ReviewIssue{
				{
					File:         "db.go",
					Line:         100,
					Severity:     "critical",
					Category:     "security",
					Description:  "SQL injection vulnerability",
					Remediation:  "Use parameterized queries",
					ProviderName: "openai",
				},
				{
					File:         "db.go",
					Line:         100,
					Severity:     "high",
					Category:     "security",
					Description:  "SQL injection vulnerability",
					Remediation:  "Use parameterized queries",
					ProviderName: "google",
				},
				{
					File:         "auth.go",
					Line:         50,
					Severity:     "medium",
					Category:     "security",
					Description:  "Weak password hashing",
					Remediation:  "Use bcrypt",
					ProviderName: "anthropic",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "db.go",
					Line:           100,
					Severity:       "critical", // Highest severity
					Category:       "security",
					Description:    "SQL injection vulnerability",
					Remediation:    "Use parameterized queries",
					Providers:      []string{"openai", "google"},
					ConsensusScore: 2.0 / 3.0,
				},
				{
					File:           "auth.go",
					Line:           50,
					Severity:       "medium",
					Category:       "security",
					Description:    "Weak password hashing",
					Remediation:    "Use bcrypt",
					Providers:      []string{"anthropic"},
					ConsensusScore: 1.0 / 3.0,
				},
			},
		},
		{
			name:           "empty input",
			issues:         []types.ReviewIssue{},
			totalProviders: 3,
			want:           []types.ConsolidatedIssue{},
		},
		{
			name: "single issue",
			issues: []types.ReviewIssue{
				{
					File:         "single.go",
					Line:         1,
					Severity:     "low",
					Category:     "style",
					Description:  "Missing comment",
					Remediation:  "Add comment",
					ProviderName: "openai",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "single.go",
					Line:           1,
					Severity:       "low",
					Category:       "style",
					Description:    "Missing comment",
					Remediation:    "Add comment",
					Providers:      []string{"openai"},
					ConsensusScore: 1.0 / 3.0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateIssues(tt.issues, tt.totalProviders)

			// Check length
			if len(got) != len(tt.want) {
				t.Errorf("DeduplicateIssues() returned %d issues, want %d", len(got), len(tt.want))
				return
			}

			// Compare each consolidated issue
			for i := range got {
				// Check basic fields
				if got[i].File != tt.want[i].File {
					t.Errorf("Issue[%d].File = %q, want %q", i, got[i].File, tt.want[i].File)
				}
				if got[i].Line != tt.want[i].Line {
					t.Errorf("Issue[%d].Line = %d, want %d", i, got[i].Line, tt.want[i].Line)
				}
				if got[i].Severity != tt.want[i].Severity {
					t.Errorf("Issue[%d].Severity = %q, want %q", i, got[i].Severity, tt.want[i].Severity)
				}
				if got[i].Category != tt.want[i].Category {
					t.Errorf("Issue[%d].Category = %q, want %q", i, got[i].Category, tt.want[i].Category)
				}
				if got[i].Description != tt.want[i].Description {
					t.Errorf("Issue[%d].Description = %q, want %q", i, got[i].Description, tt.want[i].Description)
				}
				if got[i].Remediation != tt.want[i].Remediation {
					t.Errorf("Issue[%d].Remediation = %q, want %q", i, got[i].Remediation, tt.want[i].Remediation)
				}

				// Check consensus score (with tolerance for floating point comparison)
				const epsilon = 0.001
				if abs(got[i].ConsensusScore-tt.want[i].ConsensusScore) > epsilon {
					t.Errorf("Issue[%d].ConsensusScore = %f, want %f", i, got[i].ConsensusScore, tt.want[i].ConsensusScore)
				}

				// Check providers (order-independent)
				if !equalStringSlices(got[i].Providers, tt.want[i].Providers) {
					t.Errorf("Issue[%d].Providers = %v, want %v", i, got[i].Providers, tt.want[i].Providers)
				}

				// Check that IssueID is not empty
				if got[i].IssueID == "" {
					t.Errorf("Issue[%d].IssueID is empty", i)
				}
			}
		})
	}
}

// Helper function to compare float absolute difference
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Helper function to compare string slices (order-independent)
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int)
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

// TestDeduplicateIssues_LocationMatch tests location-based fuzzy matching
// Stage 2: Location match (same file + line proximity ±5 + Levenshtein distance < 30%)
func TestDeduplicateIssues_LocationMatch(t *testing.T) {
	tests := []struct {
		name           string
		issues         []types.ReviewIssue
		totalProviders int
		want           []types.ConsolidatedIssue
	}{
		{
			name: "fuzzy match - same file, close lines (±2), similar descriptions",
			issues: []types.ReviewIssue{
				{
					File:         "auth.go",
					Line:         100,
					Severity:     "high",
					Category:     "security",
					Description:  "Missing nil check",
					Remediation:  "Add nil check",
					ProviderName: "openai",
				},
				{
					File:         "auth.go",
					Line:         102, // Within ±5 lines
					Severity:     "medium",
					Category:     "security",
					Description:  "Missing null check", // Similar to "Missing nil check" (<30% Levenshtein)
					Remediation:  "Add null check",
					ProviderName: "anthropic",
				},
			},
			totalProviders: 2,
			want: []types.ConsolidatedIssue{
				{
					File:           "auth.go",
					Line:           100, // Uses first issue's line
					Severity:       "high",
					Category:       "security",
					Description:    "Missing null check", // Longest description (18 chars vs 17 chars)
					Remediation:    "Add null check",     // Longest remediation (15 chars vs 13 chars)
					Providers:      []string{"anthropic", "openai"},
					ConsensusScore: 1.0, // Both providers
				},
			},
		},
		{
			name: "fuzzy match - boundary case at ±5 lines",
			issues: []types.ReviewIssue{
				{
					File:         "db.go",
					Line:         50,
					Severity:     "critical",
					Category:     "security",
					Description:  "SQL injection vulnerability detected",
					Remediation:  "Use parameterized queries",
					ProviderName: "openai",
				},
				{
					File:         "db.go",
					Line:         55, // Exactly +5 lines (should match)
					Severity:     "high",
					Category:     "security",
					Description:  "SQL injection vulnerability found", // Similar (>70%)
					Remediation:  "Use prepared statements",
					ProviderName: "google",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "db.go",
					Line:           50,
					Severity:       "critical",
					Category:       "security",
					Description:    "SQL injection vulnerability detected", // Longest
					Remediation:    "Use parameterized queries",            // Longest
					Providers:      []string{"google", "openai"},
					ConsensusScore: 2.0 / 3.0,
				},
			},
		},
		{
			name: "no fuzzy match - lines too far apart (>5 lines)",
			issues: []types.ReviewIssue{
				{
					File:         "handler.go",
					Line:         100,
					Severity:     "high",
					Category:     "performance",
					Description:  "Inefficient loop",
					Remediation:  "Optimize",
					ProviderName: "openai",
				},
				{
					File:         "handler.go",
					Line:         107, // 7 lines away (too far)
					Severity:     "high",
					Category:     "performance",
					Description:  "Inefficient loop",
					Remediation:  "Optimize",
					ProviderName: "anthropic",
				},
			},
			totalProviders: 2,
			want: []types.ConsolidatedIssue{
				{
					File:           "handler.go",
					Line:           100,
					Severity:       "high",
					Category:       "performance",
					Description:    "Inefficient loop",
					Remediation:    "Optimize",
					Providers:      []string{"openai"},
					ConsensusScore: 0.5,
				},
				{
					File:           "handler.go",
					Line:           107,
					Severity:       "high",
					Category:       "performance",
					Description:    "Inefficient loop",
					Remediation:    "Optimize",
					Providers:      []string{"anthropic"},
					ConsensusScore: 0.5,
				},
			},
		},
		{
			name: "no fuzzy match - descriptions too different (>30% Levenshtein)",
			issues: []types.ReviewIssue{
				{
					File:         "api.go",
					Line:         200,
					Severity:     "medium",
					Category:     "style",
					Description:  "Missing error handling",
					Remediation:  "Add error check",
					ProviderName: "openai",
				},
				{
					File:         "api.go",
					Line:         202,
					Severity:     "medium",
					Category:     "style",
					Description:  "Unused variable declaration", // Very different (>30%)
					Remediation:  "Remove unused variable",
					ProviderName: "google",
				},
			},
			totalProviders: 2,
			want: []types.ConsolidatedIssue{
				{
					File:           "api.go",
					Line:           200,
					Severity:       "medium",
					Category:       "style",
					Description:    "Missing error handling",
					Remediation:    "Add error check",
					Providers:      []string{"openai"},
					ConsensusScore: 0.5,
				},
				{
					File:           "api.go",
					Line:           202,
					Severity:       "medium",
					Category:       "style",
					Description:    "Unused variable declaration",
					Remediation:    "Remove unused variable",
					Providers:      []string{"google"},
					ConsensusScore: 0.5,
				},
			},
		},
		{
			name: "fuzzy match - negative line offset (issue reported earlier)",
			issues: []types.ReviewIssue{
				{
					File:         "service.go",
					Line:         75,
					Severity:     "high",
					Category:     "best-practice",
					Description:  "Context not passed correctly",
					Remediation:  "Pass context as first param",
					ProviderName: "anthropic",
				},
				{
					File:         "service.go",
					Line:         72, // -3 lines (within ±5)
					Severity:     "medium",
					Category:     "best-practice",
					Description:  "Context not passed properly", // Similar
					Remediation:  "Pass context as first parameter",
					ProviderName: "openai",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "service.go",
					Line:           75, // Uses first issue's line
					Severity:       "high",
					Category:       "best-practice",
					Description:    "Context not passed correctly",
					Remediation:    "Pass context as first parameter", // Longest
					Providers:      []string{"anthropic", "openai"},
					ConsensusScore: 2.0 / 3.0,
				},
			},
		},
		{
			name: "exact match takes precedence over fuzzy match",
			issues: []types.ReviewIssue{
				{
					File:         "utils.go",
					Line:         42,
					Severity:     "low",
					Category:     "style",
					Description:  "Variable naming convention",
					Remediation:  "Use camelCase",
					ProviderName: "openai",
				},
				{
					File:         "utils.go",
					Line:         42, // Exact line match
					Severity:     "low",
					Category:     "style",
					Description:  "Variable naming convention", // Exact description match
					Remediation:  "Use camelCase",
					ProviderName: "anthropic",
				},
				{
					File:         "utils.go",
					Line:         45, // Close but not exact
					Severity:     "low",
					Category:     "style",
					Description:  "Variable naming style", // Similar but not exact
					Remediation:  "Use camelCase naming",
					ProviderName: "google",
				},
			},
			totalProviders: 3,
			want: []types.ConsolidatedIssue{
				{
					File:           "utils.go",
					Line:           42,
					Severity:       "low",
					Category:       "style",
					Description:    "Variable naming convention",
					Remediation:    "Use camelCase",
					Providers:      []string{"anthropic", "openai"},
					ConsensusScore: 2.0 / 3.0,
				},
				{
					File:           "utils.go",
					Line:           45,
					Severity:       "low",
					Category:       "style",
					Description:    "Variable naming style",
					Remediation:    "Use camelCase naming",
					Providers:      []string{"google"},
					ConsensusScore: 1.0 / 3.0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateIssues(tt.issues, tt.totalProviders)

			// Check length
			if len(got) != len(tt.want) {
				t.Errorf("DeduplicateIssues() returned %d issues, want %d", len(got), len(tt.want))
				for i, issue := range got {
					t.Logf("  got[%d]: File=%s Line=%d Desc=%q Providers=%v", i, issue.File, issue.Line, issue.Description, issue.Providers)
				}
				for i, issue := range tt.want {
					t.Logf("  want[%d]: File=%s Line=%d Desc=%q Providers=%v", i, issue.File, issue.Line, issue.Description, issue.Providers)
				}
				return
			}

			// Compare each consolidated issue
			for i := range got {
				if got[i].File != tt.want[i].File {
					t.Errorf("Issue[%d].File = %q, want %q", i, got[i].File, tt.want[i].File)
				}
				if got[i].Line != tt.want[i].Line {
					t.Errorf("Issue[%d].Line = %d, want %d", i, got[i].Line, tt.want[i].Line)
				}
				if got[i].Severity != tt.want[i].Severity {
					t.Errorf("Issue[%d].Severity = %q, want %q", i, got[i].Severity, tt.want[i].Severity)
				}
				if got[i].Category != tt.want[i].Category {
					t.Errorf("Issue[%d].Category = %q, want %q", i, got[i].Category, tt.want[i].Category)
				}
				if got[i].Description != tt.want[i].Description {
					t.Errorf("Issue[%d].Description = %q, want %q", i, got[i].Description, tt.want[i].Description)
				}

				const epsilon = 0.001
				if abs(got[i].ConsensusScore-tt.want[i].ConsensusScore) > epsilon {
					t.Errorf("Issue[%d].ConsensusScore = %f, want %f", i, got[i].ConsensusScore, tt.want[i].ConsensusScore)
				}

				if !equalStringSlices(got[i].Providers, tt.want[i].Providers) {
					t.Errorf("Issue[%d].Providers = %v, want %v", i, got[i].Providers, tt.want[i].Providers)
				}

				if got[i].IssueID == "" {
					t.Errorf("Issue[%d].IssueID is empty", i)
				}
			}
		})
	}
}
