package consolidator

import (
	"sort"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/types"
)

// severityRank maps severity levels to numeric priorities for sorting.
// Higher values indicate more severe issues.
var severityRank = map[string]int{
	"critical": 5,
	"high":     4,
	"medium":   3,
	"low":      2,
	"info":     1,
}

// SortBySeverity sorts issues by severity level in descending order
// (critical > high > medium > low > info).
// Uses stable sorting to preserve relative order of issues with the same severity.
func SortBySeverity(issues []types.ConsolidatedIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		rankI := severityRank[issues[i].Severity]
		rankJ := severityRank[issues[j].Severity]
		return rankI > rankJ // Higher rank (more severe) comes first
	})
}

// SortByConsensus sorts issues by consensus score in descending order
// (1.0 > 0.67 > 0.33, etc.).
// Higher consensus score indicates more providers flagged the issue.
// Uses stable sorting to preserve relative order of issues with the same consensus score.
func SortByConsensus(issues []types.ConsolidatedIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		return issues[i].ConsensusScore > issues[j].ConsensusScore
	})
}

// SortIssues sorts issues first by severity (critical to info), then by consensus
// score (1.0 to 0.0) within each severity level.
// This provides a prioritized list where the most critical issues with highest
// consensus appear first.
func SortIssues(issues []types.ConsolidatedIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		rankI := severityRank[issues[i].Severity]
		rankJ := severityRank[issues[j].Severity]

		// First compare by severity
		if rankI != rankJ {
			return rankI > rankJ
		}

		// If severity is the same, compare by consensus score
		return issues[i].ConsensusScore > issues[j].ConsensusScore
	})
}
