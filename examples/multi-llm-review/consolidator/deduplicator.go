package consolidator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/agnivade/levenshtein"
	"github.com/dshills/langgraph-go/examples/multi-llm-review/types"
)

// DeduplicateIssues consolidates duplicate issues from multiple providers using
// multi-stage fuzzy matching.
//
// Stage 1: Exact match (same file + same line + identical description)
// Stage 2: Location match (same file + line proximity ±5 + Levenshtein distance < 30%)
// Stage 3: Semantic match (same file + keyword overlap 60%+) - Future enhancement
//
// The function returns a slice of ConsolidatedIssue where each issue contains:
// - Highest severity from all duplicates
// - Most common category
// - Longest/most detailed description
// - All unique providers that flagged it
// - Consensus score (fraction of total providers)
// - Unique issue ID (8-char hex hash)
func DeduplicateIssues(issues []types.ReviewIssue, totalProviders int) []types.ConsolidatedIssue {
	if len(issues) == 0 {
		return []types.ConsolidatedIssue{}
	}

	// Track which issues have been merged
	merged := make([]bool, len(issues))
	var consolidated []types.ConsolidatedIssue

	// Stage 1: Exact match
	for i := range issues {
		if merged[i] {
			continue
		}

		// Start a new consolidated issue
		group := []types.ReviewIssue{issues[i]}
		merged[i] = true

		// Find all exact matches
		for j := i + 1; j < len(issues); j++ {
			if merged[j] {
				continue
			}

			if isExactMatch(issues[i], issues[j]) {
				group = append(group, issues[j])
				merged[j] = true
			}
		}

		// Create consolidated issue from group
		consolidated = append(consolidated, consolidateGroup(group, totalProviders))
	}

	// Stage 2: Location-based fuzzy match
	// Re-merge consolidated issues that are close in location and similar in description
	fuzzyMerged := make([]bool, len(consolidated))
	var finalConsolidated []types.ConsolidatedIssue

	for i := range consolidated {
		if fuzzyMerged[i] {
			continue
		}

		// Start a new fuzzy group with the base issue
		base := consolidated[i]
		fuzzyGroup := []types.ConsolidatedIssue{base}
		fuzzyMerged[i] = true

		// Find all fuzzy matches
		for j := i + 1; j < len(consolidated); j++ {
			if fuzzyMerged[j] {
				continue
			}

			if isFuzzyMatch(base, consolidated[j]) {
				fuzzyGroup = append(fuzzyGroup, consolidated[j])
				fuzzyMerged[j] = true
			}
		}

		// Merge fuzzy group into a single consolidated issue
		finalConsolidated = append(finalConsolidated, mergeFuzzyGroup(fuzzyGroup, totalProviders))
	}

	// Sort by consensus score (descending), then severity, then file+line
	sortConsolidatedIssues(finalConsolidated)

	return finalConsolidated
}

// isExactMatch returns true if two issues are exact matches:
// - Same file path
// - Same line number (±0)
// - Identical description (case-sensitive)
func isExactMatch(a, b types.ReviewIssue) bool {
	return a.File == b.File &&
		a.Line == b.Line &&
		a.Description == b.Description
}

// isFuzzyMatch returns true if two consolidated issues are fuzzy matches:
// - Same file path
// - Line proximity within ±5 lines
// - Description similarity (Levenshtein distance < 30% of longer string)
func isFuzzyMatch(a, b types.ConsolidatedIssue) bool {
	// Must be same file
	if a.File != b.File {
		return false
	}

	// Check line proximity (within ±5 lines)
	lineDiff := a.Line - b.Line
	if lineDiff < 0 {
		lineDiff = -lineDiff
	}
	if lineDiff > 5 {
		return false
	}

	// Check description similarity using Levenshtein distance
	distance := levenshtein.ComputeDistance(a.Description, b.Description)
	maxLen := len(a.Description)
	if len(b.Description) > maxLen {
		maxLen = len(b.Description)
	}

	// Avoid division by zero for empty descriptions
	if maxLen == 0 {
		return true // Both descriptions are empty
	}

	// Calculate similarity as percentage
	similarity := 1.0 - (float64(distance) / float64(maxLen))

	// Match if similarity is >= 70% (i.e., distance < 30%)
	return similarity >= 0.70
}

// mergeFuzzyGroup merges a group of fuzzy-matched consolidated issues into a single
// ConsolidatedIssue. It combines all providers and recalculates consensus score.
func mergeFuzzyGroup(group []types.ConsolidatedIssue, totalProviders int) types.ConsolidatedIssue {
	if len(group) == 0 {
		return types.ConsolidatedIssue{}
	}

	if len(group) == 1 {
		return group[0]
	}

	// Use first issue as base
	base := group[0]

	// Collect all unique providers from all issues in the group
	providerSet := make(map[string]bool)
	for _, issue := range group {
		for _, provider := range issue.Providers {
			providerSet[provider] = true
		}
	}
	providers := make([]string, 0, len(providerSet))
	for provider := range providerSet {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	// Find highest severity across all issues
	highestSeverity := base.Severity
	highestRank := severityRank[highestSeverity]
	for _, issue := range group[1:] {
		rank := severityRank[issue.Severity]
		if rank > highestRank {
			highestSeverity = issue.Severity
			highestRank = rank
		}
	}

	// Find most common category
	categoryCounts := make(map[string]int)
	for _, issue := range group {
		categoryCounts[issue.Category]++
	}
	mostCommonCategory := base.Category
	maxCount := categoryCounts[mostCommonCategory]
	for category, count := range categoryCounts {
		if count > maxCount {
			mostCommonCategory = category
			maxCount = count
		}
	}

	// Find longest description
	longestDescription := base.Description
	for _, issue := range group[1:] {
		if len(issue.Description) > len(longestDescription) {
			longestDescription = issue.Description
		}
	}

	// Find longest remediation
	longestRemediation := base.Remediation
	for _, issue := range group[1:] {
		if len(issue.Remediation) > len(longestRemediation) {
			longestRemediation = issue.Remediation
		}
	}

	// Recalculate consensus score based on unique providers
	consensusScore := float64(len(providerSet)) / float64(totalProviders)

	// Generate new issue ID based on merged data
	issueID := generateIssueID(base.File, base.Line, mostCommonCategory)

	return types.ConsolidatedIssue{
		File:           base.File,
		Line:           base.Line,
		Severity:       highestSeverity,
		Category:       mostCommonCategory,
		Description:    longestDescription,
		Remediation:    longestRemediation,
		Providers:      providers,
		ConsensusScore: consensusScore,
		IssueID:        issueID,
	}
}

// consolidateGroup merges a group of duplicate issues into a single ConsolidatedIssue.
// It selects:
// - Highest severity
// - Most common category
// - Longest description (assumed to be most detailed)
// - Longest remediation
// - All unique providers
// - Consensus score based on total providers
func consolidateGroup(group []types.ReviewIssue, totalProviders int) types.ConsolidatedIssue {
	if len(group) == 0 {
		return types.ConsolidatedIssue{}
	}

	// Use first issue as base
	base := group[0]

	// Collect all providers (deduplicated)
	providerSet := make(map[string]bool)
	for _, issue := range group {
		providerSet[issue.ProviderName] = true
	}
	providers := make([]string, 0, len(providerSet))
	for provider := range providerSet {
		providers = append(providers, provider)
	}
	sort.Strings(providers) // Consistent ordering

	// Find highest severity
	severity := findHighestSeverity(group)

	// Find most common category
	category := findMostCommonCategory(group)

	// Find longest description (most detailed)
	description := findLongestString(group, func(issue types.ReviewIssue) string {
		return issue.Description
	})

	// Find longest remediation
	remediation := findLongestString(group, func(issue types.ReviewIssue) string {
		return issue.Remediation
	})

	// Calculate consensus score
	consensusScore := float64(len(providerSet)) / float64(totalProviders)

	// Generate issue ID (deterministic hash)
	issueID := generateIssueID(base.File, base.Line, category)

	return types.ConsolidatedIssue{
		File:           base.File,
		Line:           base.Line,
		Severity:       severity,
		Category:       category,
		Description:    description,
		Remediation:    remediation,
		Providers:      providers,
		ConsensusScore: consensusScore,
		IssueID:        issueID,
	}
}

// findHighestSeverity returns the highest severity from a group of issues
func findHighestSeverity(group []types.ReviewIssue) string {
	if len(group) == 0 {
		return "info"
	}

	highest := group[0].Severity
	highestRank := severityRank[highest]

	for _, issue := range group[1:] {
		rank := severityRank[issue.Severity]
		if rank > highestRank {
			highest = issue.Severity
			highestRank = rank
		}
	}

	return highest
}

// findMostCommonCategory returns the most frequently occurring category
func findMostCommonCategory(group []types.ReviewIssue) string {
	if len(group) == 0 {
		return ""
	}

	counts := make(map[string]int)
	for _, issue := range group {
		counts[issue.Category]++
	}

	mostCommon := group[0].Category
	maxCount := counts[mostCommon]

	for category, count := range counts {
		if count > maxCount {
			mostCommon = category
			maxCount = count
		}
	}

	return mostCommon
}

// findLongestString returns the longest string from a group using an extractor function
func findLongestString(group []types.ReviewIssue, extractor func(types.ReviewIssue) string) string {
	if len(group) == 0 {
		return ""
	}

	longest := extractor(group[0])
	for _, issue := range group[1:] {
		candidate := extractor(issue)
		if len(candidate) > len(longest) {
			longest = candidate
		}
	}

	return longest
}

// generateIssueID creates a deterministic 8-character hex ID from file, line, and category
func generateIssueID(file string, line int, category string) string {
	data := fmt.Sprintf("%s:%d:%s", file, line, category)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:8]
}

// sortConsolidatedIssues sorts issues by:
// 1. Consensus score (descending) - issues flagged by more providers first
// 2. Severity (descending) - critical before high before medium, etc.
// 3. File path (ascending) - alphabetical
// 4. Line number (ascending) - top to bottom
func sortConsolidatedIssues(issues []types.ConsolidatedIssue) {
	sort.Slice(issues, func(i, j int) bool {
		// First: Consensus score (higher is better)
		if issues[i].ConsensusScore != issues[j].ConsensusScore {
			return issues[i].ConsensusScore > issues[j].ConsensusScore
		}

		// Second: Severity (higher is more important)
		rankI := severityRank[issues[i].Severity]
		rankJ := severityRank[issues[j].Severity]
		if rankI != rankJ {
			return rankI > rankJ
		}

		// Third: File path (alphabetical)
		if issues[i].File != issues[j].File {
			return issues[i].File < issues[j].File
		}

		// Fourth: Line number (top to bottom)
		return issues[i].Line < issues[j].Line
	})
}
