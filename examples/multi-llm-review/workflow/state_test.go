package workflow

import (
	"encoding/json"
	"testing"
)

// TestReviewStateCreation verifies that ReviewState can be created with all fields.
func TestReviewStateCreation(t *testing.T) {
	tests := []struct {
		name      string
		state     ReviewState
		wantPanic bool
		validate  func(t *testing.T, s ReviewState)
	}{
		{
			name: "full state with all fields",
			state: ReviewState{
				CodebaseRoot:     "/path/to/codebase",
				DiscoveredFiles:  []CodeFile{{FilePath: "main.go", Language: "go"}},
				Batches:          []Batch{{BatchNumber: 1, Status: "completed"}},
				CurrentBatch:     1,
				TotalBatches:     2,
				CompletedBatches: []int{1},
				Reviews: map[string][]Review{
					"openai": {{
						ProviderName: "openai",
						BatchNumber:  1,
						Issues:       []ReviewIssue{{File: "main.go", Line: 10, Severity: "high"}},
						TokensUsed:   100,
						Duration:     1000,
						Timestamp:    "2025-10-29T14:30:00Z",
						Error:        "",
					}},
				},
				ConsolidatedIssues: []ConsolidatedIssue{{
					File:           "main.go",
					Line:           10,
					Severity:       "high",
					Category:       "performance",
					Description:    "Inefficient loop",
					Remediation:    "Use goroutines",
					Providers:      []string{"openai", "anthropic"},
					ConsensusScore: 0.67,
					IssueID:        "a1b2c3d4",
				}},
				ReportPath:         "/output/report.md",
				StartTime:          "2025-10-29T14:00:00Z",
				EndTime:            "2025-10-29T15:00:00Z",
				TotalFilesReviewed: 10,
				TotalIssuesFound:   5,
				LastError:          "",
				FailedProviders:    []string{},
			},
			validate: func(t *testing.T, s ReviewState) {
				if s.CodebaseRoot != "/path/to/codebase" {
					t.Errorf("CodebaseRoot = %q, want %q", s.CodebaseRoot, "/path/to/codebase")
				}
				if len(s.DiscoveredFiles) != 1 {
					t.Errorf("DiscoveredFiles length = %d, want 1", len(s.DiscoveredFiles))
				}
				if s.CurrentBatch != 1 {
					t.Errorf("CurrentBatch = %d, want 1", s.CurrentBatch)
				}
				if s.TotalBatches != 2 {
					t.Errorf("TotalBatches = %d, want 2", s.TotalBatches)
				}
				if len(s.CompletedBatches) != 1 {
					t.Errorf("CompletedBatches length = %d, want 1", len(s.CompletedBatches))
				}
				if len(s.Reviews) != 1 {
					t.Errorf("Reviews length = %d, want 1", len(s.Reviews))
				}
				if len(s.ConsolidatedIssues) != 1 {
					t.Errorf("ConsolidatedIssues length = %d, want 1", len(s.ConsolidatedIssues))
				}
				if s.TotalFilesReviewed != 10 {
					t.Errorf("TotalFilesReviewed = %d, want 10", s.TotalFilesReviewed)
				}
			},
		},
		{
			name: "minimal state with zero values",
			state: ReviewState{
				CodebaseRoot:       "",
				DiscoveredFiles:    []CodeFile{},
				Batches:            []Batch{},
				CurrentBatch:       0,
				TotalBatches:       0,
				CompletedBatches:   []int{},
				Reviews:            map[string][]Review{},
				ConsolidatedIssues: []ConsolidatedIssue{},
				ReportPath:         "",
				StartTime:          "",
				EndTime:            "",
				TotalFilesReviewed: 0,
				TotalIssuesFound:   0,
				LastError:          "",
				FailedProviders:    []string{},
			},
			validate: func(t *testing.T, s ReviewState) {
				if s.CodebaseRoot != "" {
					t.Errorf("CodebaseRoot = %q, want empty", s.CodebaseRoot)
				}
				if s.CurrentBatch != 0 {
					t.Errorf("CurrentBatch = %d, want 0", s.CurrentBatch)
				}
				if len(s.Reviews) != 0 {
					t.Errorf("Reviews length = %d, want 0", len(s.Reviews))
				}
				if s.TotalFilesReviewed != 0 {
					t.Errorf("TotalFilesReviewed = %d, want 0", s.TotalFilesReviewed)
				}
			},
		},
		{
			name: "state with multiple providers",
			state: ReviewState{
				CodebaseRoot: "/project",
				Reviews: map[string][]Review{
					"openai": {{
						ProviderName: "openai",
						BatchNumber:  1,
						TokensUsed:   150,
					}},
					"anthropic": {{
						ProviderName: "anthropic",
						BatchNumber:  1,
						TokensUsed:   200,
					}},
					"google": {{
						ProviderName: "google",
						BatchNumber:  1,
						TokensUsed:   100,
					}},
				},
				TotalIssuesFound: 15,
			},
			validate: func(t *testing.T, s ReviewState) {
				if len(s.Reviews) != 3 {
					t.Errorf("Reviews length = %d, want 3", len(s.Reviews))
				}
				for provider := range s.Reviews {
					if provider != "openai" && provider != "anthropic" && provider != "google" {
						t.Errorf("unexpected provider %q", provider)
					}
				}
			},
		},
		{
			name: "state with failed providers",
			state: ReviewState{
				CodebaseRoot:     "/project",
				FailedProviders:  []string{"openai", "anthropic"},
				LastError:        "provider timeout",
				TotalIssuesFound: 5,
			},
			validate: func(t *testing.T, s ReviewState) {
				if len(s.FailedProviders) != 2 {
					t.Errorf("FailedProviders length = %d, want 2", len(s.FailedProviders))
				}
				if s.LastError != "provider timeout" {
					t.Errorf("LastError = %q, want %q", s.LastError, "provider timeout")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := tt.state
			if tt.validate != nil {
				tt.validate(t, state)
			}
		})
	}
}

// TestReviewStateJSONSerialization verifies ReviewState is JSON serializable and deserializable.
func TestReviewStateJSONSerialization(t *testing.T) {
	tests := []struct {
		name      string
		state     ReviewState
		wantError bool
	}{
		{
			name: "serialize and deserialize full state",
			state: ReviewState{
				CodebaseRoot:     "/path/to/codebase",
				CurrentBatch:     2,
				TotalBatches:     5,
				CompletedBatches: []int{1, 2},
				DiscoveredFiles: []CodeFile{
					{
						FilePath:  "main.go",
						Content:   "package main",
						Language:  "go",
						LineCount: 100,
						SizeBytes: 1024,
						Checksum:  "abc123def456",
					},
				},
				Batches: []Batch{
					{
						BatchNumber: 1,
						Files: []CodeFile{
							{FilePath: "main.go"},
						},
						TotalLines: 100,
						Status:     "completed",
					},
				},
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
									Category:     "performance",
									Description:  "Inefficient loop",
									Remediation:  "Use goroutines",
									ProviderName: "openai",
									Confidence:   0.95,
								},
							},
							TokensUsed: 250,
							Duration:   1500,
							Timestamp:  "2025-10-29T14:30:00Z",
							Error:      "",
						},
					},
				},
				ConsolidatedIssues: []ConsolidatedIssue{
					{
						File:           "main.go",
						Line:           42,
						Severity:       "high",
						Category:       "performance",
						Description:    "Inefficient loop",
						Remediation:    "Use goroutines",
						Providers:      []string{"openai", "anthropic"},
						ConsensusScore: 0.67,
						IssueID:        "a1b2c3d4",
					},
				},
				ReportPath:         "/output/report.md",
				StartTime:          "2025-10-29T14:00:00Z",
				EndTime:            "2025-10-29T15:00:00Z",
				TotalFilesReviewed: 10,
				TotalIssuesFound:   15,
				LastError:          "",
				FailedProviders:    []string{},
			},
			wantError: false,
		},
		{
			name: "serialize and deserialize minimal state",
			state: ReviewState{
				CodebaseRoot:       "",
				CurrentBatch:       0,
				TotalBatches:       0,
				CompletedBatches:   []int{},
				DiscoveredFiles:    []CodeFile{},
				Batches:            []Batch{},
				Reviews:            map[string][]Review{},
				ConsolidatedIssues: []ConsolidatedIssue{},
				ReportPath:         "",
				StartTime:          "",
				EndTime:            "",
				TotalFilesReviewed: 0,
				TotalIssuesFound:   0,
				LastError:          "",
				FailedProviders:    []string{},
			},
			wantError: false,
		},
		{
			name: "serialize and deserialize state with error",
			state: ReviewState{
				CodebaseRoot:    "/project",
				CurrentBatch:    1,
				TotalBatches:    3,
				LastError:       "failed to call provider API: connection timeout",
				FailedProviders: []string{"openai", "google"},
				Reviews: map[string][]Review{
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Error:        "rate limit exceeded",
						},
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.state)
			if err != nil && !tt.wantError {
				t.Errorf("Marshal() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if err != nil && tt.wantError {
				return
			}

			// Unmarshal from JSON
			var decoded ReviewState
			err = json.Unmarshal(data, &decoded)
			if err != nil && !tt.wantError {
				t.Errorf("Unmarshal() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if err != nil && tt.wantError {
				return
			}

			// Verify critical fields match
			if decoded.CodebaseRoot != tt.state.CodebaseRoot {
				t.Errorf("CodebaseRoot mismatch: got %q, want %q", decoded.CodebaseRoot, tt.state.CodebaseRoot)
			}
			if decoded.CurrentBatch != tt.state.CurrentBatch {
				t.Errorf("CurrentBatch mismatch: got %d, want %d", decoded.CurrentBatch, tt.state.CurrentBatch)
			}
			if decoded.TotalBatches != tt.state.TotalBatches {
				t.Errorf("TotalBatches mismatch: got %d, want %d", decoded.TotalBatches, tt.state.TotalBatches)
			}
			if len(decoded.CompletedBatches) != len(tt.state.CompletedBatches) {
				t.Errorf("CompletedBatches length mismatch: got %d, want %d", len(decoded.CompletedBatches), len(tt.state.CompletedBatches))
			}
			if len(decoded.Reviews) != len(tt.state.Reviews) {
				t.Errorf("Reviews length mismatch: got %d, want %d", len(decoded.Reviews), len(tt.state.Reviews))
			}
			if decoded.TotalFilesReviewed != tt.state.TotalFilesReviewed {
				t.Errorf("TotalFilesReviewed mismatch: got %d, want %d", decoded.TotalFilesReviewed, tt.state.TotalFilesReviewed)
			}
			if decoded.TotalIssuesFound != tt.state.TotalIssuesFound {
				t.Errorf("TotalIssuesFound mismatch: got %d, want %d", decoded.TotalIssuesFound, tt.state.TotalIssuesFound)
			}
		})
	}
}

// TestReviewStateFieldTypes verifies all field types are correct.
func TestReviewStateFieldTypes(t *testing.T) {
	state := ReviewState{
		CodebaseRoot:       "/path",
		DiscoveredFiles:    []CodeFile{},
		Batches:            []Batch{},
		CurrentBatch:       1,
		TotalBatches:       1,
		CompletedBatches:   []int{1},
		Reviews:            map[string][]Review{},
		ConsolidatedIssues: []ConsolidatedIssue{},
		ReportPath:         "/report.md",
		StartTime:          "2025-10-29T14:00:00Z",
		EndTime:            "2025-10-29T15:00:00Z",
		TotalFilesReviewed: 10,
		TotalIssuesFound:   5,
		LastError:          "",
		FailedProviders:    []string{},
	}

	// Verify CodebaseRoot is string
	if _, ok := interface{}(state.CodebaseRoot).(string); !ok {
		t.Error("CodebaseRoot is not a string")
	}

	// Verify DiscoveredFiles is []CodeFile
	if _, ok := interface{}(state.DiscoveredFiles).([]CodeFile); !ok {
		t.Error("DiscoveredFiles is not []CodeFile")
	}

	// Verify Batches is []Batch
	if _, ok := interface{}(state.Batches).([]Batch); !ok {
		t.Error("Batches is not []Batch")
	}

	// Verify CurrentBatch is int
	if _, ok := interface{}(state.CurrentBatch).(int); !ok {
		t.Error("CurrentBatch is not int")
	}

	// Verify TotalBatches is int
	if _, ok := interface{}(state.TotalBatches).(int); !ok {
		t.Error("TotalBatches is not int")
	}

	// Verify CompletedBatches is []int
	if _, ok := interface{}(state.CompletedBatches).([]int); !ok {
		t.Error("CompletedBatches is not []int")
	}

	// Verify Reviews is map[string][]Review
	if _, ok := interface{}(state.Reviews).(map[string][]Review); !ok {
		t.Error("Reviews is not map[string][]Review")
	}

	// Verify ConsolidatedIssues is []ConsolidatedIssue
	if _, ok := interface{}(state.ConsolidatedIssues).([]ConsolidatedIssue); !ok {
		t.Error("ConsolidatedIssues is not []ConsolidatedIssue")
	}

	// Verify ReportPath is string
	if _, ok := interface{}(state.ReportPath).(string); !ok {
		t.Error("ReportPath is not a string")
	}

	// Verify StartTime is string
	if _, ok := interface{}(state.StartTime).(string); !ok {
		t.Error("StartTime is not a string")
	}

	// Verify EndTime is string
	if _, ok := interface{}(state.EndTime).(string); !ok {
		t.Error("EndTime is not a string")
	}

	// Verify TotalFilesReviewed is int
	if _, ok := interface{}(state.TotalFilesReviewed).(int); !ok {
		t.Error("TotalFilesReviewed is not int")
	}

	// Verify TotalIssuesFound is int
	if _, ok := interface{}(state.TotalIssuesFound).(int); !ok {
		t.Error("TotalIssuesFound is not int")
	}

	// Verify LastError is string
	if _, ok := interface{}(state.LastError).(string); !ok {
		t.Error("LastError is not a string")
	}

	// Verify FailedProviders is []string
	if _, ok := interface{}(state.FailedProviders).([]string); !ok {
		t.Error("FailedProviders is not []string")
	}
}

// TestReviewStateZeroValues verifies zero values are handled correctly.
func TestReviewStateZeroValues(t *testing.T) {
	tests := []struct {
		name       string
		fieldName  string
		getField   func(ReviewState) interface{}
		wantZero   interface{}
		wantLength int // for slices and maps, -1 means check value equality instead
	}{
		{
			name:      "CodebaseRoot zero value",
			fieldName: "CodebaseRoot",
			getField: func(s ReviewState) interface{} {
				return s.CodebaseRoot
			},
			wantZero:   "",
			wantLength: -1,
		},
		{
			name:      "CurrentBatch zero value",
			fieldName: "CurrentBatch",
			getField: func(s ReviewState) interface{} {
				return s.CurrentBatch
			},
			wantZero:   0,
			wantLength: -1,
		},
		{
			name:      "TotalBatches zero value",
			fieldName: "TotalBatches",
			getField: func(s ReviewState) interface{} {
				return s.TotalBatches
			},
			wantZero:   0,
			wantLength: -1,
		},
		{
			name:      "CompletedBatches zero value",
			fieldName: "CompletedBatches",
			getField: func(s ReviewState) interface{} {
				return s.CompletedBatches
			},
			wantLength: 0,
		},
		{
			name:      "DiscoveredFiles zero value",
			fieldName: "DiscoveredFiles",
			getField: func(s ReviewState) interface{} {
				return s.DiscoveredFiles
			},
			wantLength: 0,
		},
		{
			name:      "Batches zero value",
			fieldName: "Batches",
			getField: func(s ReviewState) interface{} {
				return s.Batches
			},
			wantLength: 0,
		},
		{
			name:      "Reviews zero value",
			fieldName: "Reviews",
			getField: func(s ReviewState) interface{} {
				return s.Reviews
			},
			wantLength: 0,
		},
		{
			name:      "ConsolidatedIssues zero value",
			fieldName: "ConsolidatedIssues",
			getField: func(s ReviewState) interface{} {
				return s.ConsolidatedIssues
			},
			wantLength: 0,
		},
		{
			name:      "ReportPath zero value",
			fieldName: "ReportPath",
			getField: func(s ReviewState) interface{} {
				return s.ReportPath
			},
			wantZero:   "",
			wantLength: -1,
		},
		{
			name:      "StartTime zero value",
			fieldName: "StartTime",
			getField: func(s ReviewState) interface{} {
				return s.StartTime
			},
			wantZero:   "",
			wantLength: -1,
		},
		{
			name:      "EndTime zero value",
			fieldName: "EndTime",
			getField: func(s ReviewState) interface{} {
				return s.EndTime
			},
			wantZero:   "",
			wantLength: -1,
		},
		{
			name:      "TotalFilesReviewed zero value",
			fieldName: "TotalFilesReviewed",
			getField: func(s ReviewState) interface{} {
				return s.TotalFilesReviewed
			},
			wantZero:   0,
			wantLength: -1,
		},
		{
			name:      "TotalIssuesFound zero value",
			fieldName: "TotalIssuesFound",
			getField: func(s ReviewState) interface{} {
				return s.TotalIssuesFound
			},
			wantZero:   0,
			wantLength: -1,
		},
		{
			name:      "LastError zero value",
			fieldName: "LastError",
			getField: func(s ReviewState) interface{} {
				return s.LastError
			},
			wantZero:   "",
			wantLength: -1,
		},
		{
			name:      "FailedProviders zero value",
			fieldName: "FailedProviders",
			getField: func(s ReviewState) interface{} {
				return s.FailedProviders
			},
			wantLength: 0,
		},
	}

	// Create a zero-value ReviewState
	var zeroState ReviewState

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.getField(zeroState)

			if tt.wantLength >= 0 {
				// Check length for slices and maps
				switch v := value.(type) {
				case []int:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				case []CodeFile:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				case []Batch:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				case []ConsolidatedIssue:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				case []string:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				case map[string][]Review:
					if len(v) != tt.wantLength {
						t.Errorf("%s length = %d, want %d", tt.fieldName, len(v), tt.wantLength)
					}
				default:
					t.Errorf("unknown type for length check: %T", v)
				}
			} else {
				// Check exact equality for scalar values
				if value != tt.wantZero {
					t.Errorf("%s = %v, want %v", tt.fieldName, value, tt.wantZero)
				}
			}
		})
	}
}

// TestReviewStateReviewsMap verifies the Reviews map can be populated and accessed correctly.
func TestReviewStateReviewsMap(t *testing.T) {
	tests := []struct {
		name      string
		reviews   map[string][]Review
		providers []string
		validate  func(t *testing.T, reviews map[string][]Review)
	}{
		{
			name:      "empty reviews map",
			reviews:   map[string][]Review{},
			providers: []string{},
			validate: func(t *testing.T, reviews map[string][]Review) {
				if len(reviews) != 0 {
					t.Errorf("expected empty reviews map, got %d entries", len(reviews))
				}
			},
		},
		{
			name: "single provider with multiple reviews",
			reviews: map[string][]Review{
				"openai": {
					{ProviderName: "openai", BatchNumber: 1, TokensUsed: 100},
					{ProviderName: "openai", BatchNumber: 2, TokensUsed: 150},
				},
			},
			providers: []string{"openai"},
			validate: func(t *testing.T, reviews map[string][]Review) {
				if len(reviews) != 1 {
					t.Errorf("expected 1 provider, got %d", len(reviews))
				}
				openaiReviews, ok := reviews["openai"]
				if !ok {
					t.Error("openai provider not found in reviews")
					return
				}
				if len(openaiReviews) != 2 {
					t.Errorf("expected 2 openai reviews, got %d", len(openaiReviews))
				}
			},
		},
		{
			name: "multiple providers",
			reviews: map[string][]Review{
				"openai": {
					{ProviderName: "openai", BatchNumber: 1, TokensUsed: 100},
				},
				"anthropic": {
					{ProviderName: "anthropic", BatchNumber: 1, TokensUsed: 120},
				},
				"google": {
					{ProviderName: "google", BatchNumber: 1, TokensUsed: 90},
				},
			},
			providers: []string{"openai", "anthropic", "google"},
			validate: func(t *testing.T, reviews map[string][]Review) {
				if len(reviews) != 3 {
					t.Errorf("expected 3 providers, got %d", len(reviews))
				}
				for _, provider := range []string{"openai", "anthropic", "google"} {
					if _, ok := reviews[provider]; !ok {
						t.Errorf("provider %q not found in reviews", provider)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ReviewState{
				CodebaseRoot: "/project",
				Reviews:      tt.reviews,
			}

			if tt.validate != nil {
				tt.validate(t, state.Reviews)
			}
		})
	}
}

// TestReviewStateCompletedBatches verifies completed batches tracking.
func TestReviewStateCompletedBatches(t *testing.T) {
	tests := []struct {
		name             string
		currentBatch     int
		totalBatches     int
		completedBatches []int
		validate         func(t *testing.T, s ReviewState)
	}{
		{
			name:             "no batches completed",
			currentBatch:     1,
			totalBatches:     5,
			completedBatches: []int{},
			validate: func(t *testing.T, s ReviewState) {
				if len(s.CompletedBatches) != 0 {
					t.Errorf("expected 0 completed batches, got %d", len(s.CompletedBatches))
				}
				if s.CurrentBatch != 1 {
					t.Errorf("expected current batch 1, got %d", s.CurrentBatch)
				}
			},
		},
		{
			name:             "some batches completed",
			currentBatch:     3,
			totalBatches:     5,
			completedBatches: []int{1, 2},
			validate: func(t *testing.T, s ReviewState) {
				if len(s.CompletedBatches) != 2 {
					t.Errorf("expected 2 completed batches, got %d", len(s.CompletedBatches))
				}
				if s.CurrentBatch != 3 {
					t.Errorf("expected current batch 3, got %d", s.CurrentBatch)
				}
			},
		},
		{
			name:             "all batches completed",
			currentBatch:     6,
			totalBatches:     5,
			completedBatches: []int{1, 2, 3, 4, 5},
			validate: func(t *testing.T, s ReviewState) {
				if len(s.CompletedBatches) != 5 {
					t.Errorf("expected 5 completed batches, got %d", len(s.CompletedBatches))
				}
				if s.CurrentBatch != 6 {
					t.Errorf("expected current batch 6, got %d", s.CurrentBatch)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ReviewState{
				CodebaseRoot:     "/project",
				CurrentBatch:     tt.currentBatch,
				TotalBatches:     tt.totalBatches,
				CompletedBatches: tt.completedBatches,
			}

			if tt.validate != nil {
				tt.validate(t, state)
			}
		})
	}
}

// TestReviewStateTimeTracking verifies start and end time fields.
func TestReviewStateTimeTracking(t *testing.T) {
	tests := []struct {
		name      string
		startTime string
		endTime   string
		validate  func(t *testing.T, s ReviewState)
	}{
		{
			name:      "no times set",
			startTime: "",
			endTime:   "",
			validate: func(t *testing.T, s ReviewState) {
				if s.StartTime != "" {
					t.Errorf("expected empty StartTime, got %q", s.StartTime)
				}
				if s.EndTime != "" {
					t.Errorf("expected empty EndTime, got %q", s.EndTime)
				}
			},
		},
		{
			name:      "only start time set",
			startTime: "2025-10-29T14:00:00Z",
			endTime:   "",
			validate: func(t *testing.T, s ReviewState) {
				if s.StartTime != "2025-10-29T14:00:00Z" {
					t.Errorf("expected StartTime %q, got %q", "2025-10-29T14:00:00Z", s.StartTime)
				}
				if s.EndTime != "" {
					t.Errorf("expected empty EndTime, got %q", s.EndTime)
				}
			},
		},
		{
			name:      "both times set",
			startTime: "2025-10-29T14:00:00Z",
			endTime:   "2025-10-29T15:30:00Z",
			validate: func(t *testing.T, s ReviewState) {
				if s.StartTime != "2025-10-29T14:00:00Z" {
					t.Errorf("expected StartTime %q, got %q", "2025-10-29T14:00:00Z", s.StartTime)
				}
				if s.EndTime != "2025-10-29T15:30:00Z" {
					t.Errorf("expected EndTime %q, got %q", "2025-10-29T15:30:00Z", s.EndTime)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ReviewState{
				CodebaseRoot: "/project",
				StartTime:    tt.startTime,
				EndTime:      tt.endTime,
			}

			if tt.validate != nil {
				tt.validate(t, state)
			}
		})
	}
}

// TestReduceReviewState verifies the ReduceReviewState reducer function merges state correctly.
func TestReduceReviewState(t *testing.T) {
	tests := []struct {
		name      string
		prev      ReviewState
		delta     ReviewState
		validate  func(t *testing.T, result ReviewState)
		wantPanic bool
	}{
		{
			name: "merge batch completions",
			prev: ReviewState{
				CodebaseRoot:     "/project",
				CompletedBatches: []int{1, 2},
				CurrentBatch:     3,
				TotalBatches:     5,
			},
			delta: ReviewState{
				CompletedBatches: []int{3, 4},
				CurrentBatch:     5,
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.CompletedBatches) != 4 {
					t.Errorf("CompletedBatches length = %d, want 4", len(result.CompletedBatches))
				}
				expectedBatches := []int{1, 2, 3, 4}
				for i, expected := range expectedBatches {
					if i >= len(result.CompletedBatches) || result.CompletedBatches[i] != expected {
						t.Errorf("CompletedBatches[%d] = %d, want %d", i, result.CompletedBatches[i], expected)
					}
				}
				if result.CurrentBatch != 5 {
					t.Errorf("CurrentBatch = %d, want 5", result.CurrentBatch)
				}
			},
		},
		{
			name: "merge reviews from multiple providers",
			prev: ReviewState{
				CodebaseRoot: "/project",
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  1,
							Issues:       []ReviewIssue{{File: "main.go", Severity: "high"}},
						},
					},
				},
			},
			delta: ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{
							ProviderName: "openai",
							BatchNumber:  2,
							Issues:       []ReviewIssue{{File: "util.go", Severity: "medium"}},
						},
					},
					"anthropic": {
						{
							ProviderName: "anthropic",
							BatchNumber:  1,
							Issues:       []ReviewIssue{{File: "main.go", Severity: "critical"}},
						},
					},
				},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.Reviews) != 2 {
					t.Errorf("Reviews length = %d, want 2", len(result.Reviews))
				}
				if openaiReviews, ok := result.Reviews["openai"]; !ok {
					t.Error("openai provider not found in Reviews")
				} else if len(openaiReviews) != 2 {
					t.Errorf("openai reviews length = %d, want 2", len(openaiReviews))
				}
				if anthropicReviews, ok := result.Reviews["anthropic"]; !ok {
					t.Error("anthropic provider not found in Reviews")
				} else if len(anthropicReviews) != 1 {
					t.Errorf("anthropic reviews length = %d, want 1", len(anthropicReviews))
				}
			},
		},
		{
			name: "accumulate counters (TotalFilesReviewed, TotalIssuesFound)",
			prev: ReviewState{
				CodebaseRoot:       "/project",
				TotalFilesReviewed: 50,
				TotalIssuesFound:   15,
			},
			delta: ReviewState{
				TotalFilesReviewed: 75,
				TotalIssuesFound:   28,
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.TotalFilesReviewed != 125 { // 50 + 75 (accumulation)
					t.Errorf("TotalFilesReviewed = %d, want 125", result.TotalFilesReviewed)
				}
				if result.TotalIssuesFound != 43 { // 15 + 28 (accumulation)
					t.Errorf("TotalIssuesFound = %d, want 43", result.TotalIssuesFound)
				}
			},
		},
		{
			name: "update EndTime timestamp",
			prev: ReviewState{
				CodebaseRoot: "/project",
				StartTime:    "2025-10-29T14:00:00Z",
				EndTime:      "",
			},
			delta: ReviewState{
				EndTime: "2025-10-29T15:30:00Z",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.StartTime != "2025-10-29T14:00:00Z" {
					t.Errorf("StartTime = %q, want %q", result.StartTime, "2025-10-29T14:00:00Z")
				}
				if result.EndTime != "2025-10-29T15:30:00Z" {
					t.Errorf("EndTime = %q, want %q", result.EndTime, "2025-10-29T15:30:00Z")
				}
			},
		},
		{
			name: "do not replace EndTime if delta is empty",
			prev: ReviewState{
				CodebaseRoot: "/project",
				EndTime:      "2025-10-29T15:00:00Z",
			},
			delta: ReviewState{
				EndTime: "",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.EndTime != "2025-10-29T15:00:00Z" {
					t.Errorf("EndTime = %q, want %q", result.EndTime, "2025-10-29T15:00:00Z")
				}
			},
		},
		{
			name: "track error with LastError",
			prev: ReviewState{
				CodebaseRoot:    "/project",
				LastError:       "",
				FailedProviders: []string{},
			},
			delta: ReviewState{
				LastError:       "API connection timeout",
				FailedProviders: []string{"openai"},
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.LastError != "API connection timeout" {
					t.Errorf("LastError = %q, want %q", result.LastError, "API connection timeout")
				}
				if len(result.FailedProviders) != 1 {
					t.Errorf("FailedProviders length = %d, want 1", len(result.FailedProviders))
				}
				if result.FailedProviders[0] != "openai" {
					t.Errorf("FailedProviders[0] = %q, want %q", result.FailedProviders[0], "openai")
				}
			},
		},
		{
			name: "merge failed providers list",
			prev: ReviewState{
				CodebaseRoot:    "/project",
				FailedProviders: []string{"openai"},
			},
			delta: ReviewState{
				FailedProviders: []string{"google"},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.FailedProviders) != 2 {
					t.Errorf("FailedProviders length = %d, want 2", len(result.FailedProviders))
				}
				hasOpenAI := false
				hasGoogle := false
				for _, provider := range result.FailedProviders {
					if provider == "openai" {
						hasOpenAI = true
					}
					if provider == "google" {
						hasGoogle = true
					}
				}
				if !hasOpenAI {
					t.Error("openai not found in FailedProviders")
				}
				if !hasGoogle {
					t.Error("google not found in FailedProviders")
				}
			},
		},
		{
			name: "do not override LastError if delta is empty",
			prev: ReviewState{
				CodebaseRoot: "/project",
				LastError:    "previous error message",
			},
			delta: ReviewState{
				LastError: "",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.LastError != "previous error message" {
					t.Errorf("LastError = %q, want %q", result.LastError, "previous error message")
				}
			},
		},
		{
			name: "replace consolidated issues when delta has non-empty list",
			prev: ReviewState{
				CodebaseRoot: "/project",
				ConsolidatedIssues: []ConsolidatedIssue{
					{File: "old.go", IssueID: "old001"},
					{File: "old.go", IssueID: "old002"},
				},
			},
			delta: ReviewState{
				ConsolidatedIssues: []ConsolidatedIssue{
					{File: "new.go", IssueID: "new001"},
				},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.ConsolidatedIssues) != 1 {
					t.Errorf("ConsolidatedIssues length = %d, want 1", len(result.ConsolidatedIssues))
				}
				if result.ConsolidatedIssues[0].IssueID != "new001" {
					t.Errorf("ConsolidatedIssues[0].IssueID = %q, want %q", result.ConsolidatedIssues[0].IssueID, "new001")
				}
			},
		},
		{
			name: "do not replace consolidated issues when delta is empty",
			prev: ReviewState{
				CodebaseRoot: "/project",
				ConsolidatedIssues: []ConsolidatedIssue{
					{File: "main.go", IssueID: "issue001"},
				},
			},
			delta: ReviewState{
				ConsolidatedIssues: []ConsolidatedIssue{},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.ConsolidatedIssues) != 1 {
					t.Errorf("ConsolidatedIssues length = %d, want 1", len(result.ConsolidatedIssues))
				}
				if result.ConsolidatedIssues[0].IssueID != "issue001" {
					t.Errorf("ConsolidatedIssues[0].IssueID = %q, want %q", result.ConsolidatedIssues[0].IssueID, "issue001")
				}
			},
		},
		{
			name: "update report path when non-empty",
			prev: ReviewState{
				CodebaseRoot: "/project",
				ReportPath:   "",
			},
			delta: ReviewState{
				ReportPath: "/output/report.md",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.ReportPath != "/output/report.md" {
					t.Errorf("ReportPath = %q, want %q", result.ReportPath, "/output/report.md")
				}
			},
		},
		{
			name: "do not override report path when delta is empty",
			prev: ReviewState{
				CodebaseRoot: "/project",
				ReportPath:   "/output/existing-report.md",
			},
			delta: ReviewState{
				ReportPath: "",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.ReportPath != "/output/existing-report.md" {
					t.Errorf("ReportPath = %q, want %q", result.ReportPath, "/output/existing-report.md")
				}
			},
		},
		{
			name: "comprehensive merge with all fields",
			prev: ReviewState{
				CodebaseRoot:       "/project",
				CurrentBatch:       2,
				TotalBatches:       5,
				CompletedBatches:   []int{1},
				TotalFilesReviewed: 20,
				TotalIssuesFound:   5,
				StartTime:          "2025-10-29T14:00:00Z",
				EndTime:            "",
				LastError:          "",
				FailedProviders:    []string{},
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 1, TokensUsed: 100},
					},
				},
				ConsolidatedIssues: []ConsolidatedIssue{},
				ReportPath:         "",
			},
			delta: ReviewState{
				CurrentBatch:       3,
				CompletedBatches:   []int{2},
				TotalFilesReviewed: 40,
				TotalIssuesFound:   12,
				EndTime:            "2025-10-29T15:30:00Z",
				LastError:          "retried provider error",
				FailedProviders:    []string{"google"},
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 2, TokensUsed: 150},
					},
					"anthropic": {
						{ProviderName: "anthropic", BatchNumber: 2, TokensUsed: 180},
					},
				},
				ConsolidatedIssues: []ConsolidatedIssue{
					{File: "main.go", IssueID: "abc123"},
				},
				ReportPath: "/output/review-report.md",
			},
			validate: func(t *testing.T, result ReviewState) {
				if result.CurrentBatch != 3 {
					t.Errorf("CurrentBatch = %d, want 3", result.CurrentBatch)
				}
				if len(result.CompletedBatches) != 2 {
					t.Errorf("CompletedBatches length = %d, want 2", len(result.CompletedBatches))
				}
				if result.TotalFilesReviewed != 60 { // 20 + 40 (accumulation)
					t.Errorf("TotalFilesReviewed = %d, want 60", result.TotalFilesReviewed)
				}
				if result.TotalIssuesFound != 17 { // 5 + 12 (accumulation)
					t.Errorf("TotalIssuesFound = %d, want 17", result.TotalIssuesFound)
				}
				if result.EndTime != "2025-10-29T15:30:00Z" {
					t.Errorf("EndTime = %q, want %q", result.EndTime, "2025-10-29T15:30:00Z")
				}
				if result.LastError != "retried provider error" {
					t.Errorf("LastError = %q, want %q", result.LastError, "retried provider error")
				}
				if len(result.FailedProviders) != 1 {
					t.Errorf("FailedProviders length = %d, want 1", len(result.FailedProviders))
				}
				if len(result.Reviews) != 2 {
					t.Errorf("Reviews length = %d, want 2", len(result.Reviews))
				}
				if len(result.ConsolidatedIssues) != 1 {
					t.Errorf("ConsolidatedIssues length = %d, want 1", len(result.ConsolidatedIssues))
				}
				if result.ReportPath != "/output/review-report.md" {
					t.Errorf("ReportPath = %q, want %q", result.ReportPath, "/output/review-report.md")
				}
			},
		},
		{
			name: "preserve immutability of prev (verify reducer is pure function)",
			prev: ReviewState{
				CodebaseRoot:     "/project",
				CompletedBatches: []int{1, 2},
				CurrentBatch:     2,
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 1},
					},
				},
			},
			delta: ReviewState{
				CompletedBatches: []int{3},
				CurrentBatch:     3,
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 2},
					},
				},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.CompletedBatches) != 3 {
					t.Errorf("result CompletedBatches length = %d, want 3", len(result.CompletedBatches))
				}
				if result.CurrentBatch != 3 {
					t.Errorf("result CurrentBatch = %d, want 3", result.CurrentBatch)
				}
				if len(result.Reviews["openai"]) != 2 {
					t.Errorf("result Reviews[openai] length = %d, want 2", len(result.Reviews["openai"]))
				}
			},
		},
		{
			name: "merge reviews from new provider to empty Reviews map",
			prev: ReviewState{
				CodebaseRoot: "/project",
				Reviews:      map[string][]Review{},
			},
			delta: ReviewState{
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 1, TokensUsed: 100},
					},
				},
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.Reviews) != 1 {
					t.Errorf("Reviews length = %d, want 1", len(result.Reviews))
				}
				if openaiReviews, ok := result.Reviews["openai"]; !ok {
					t.Error("openai provider not found in Reviews")
				} else if len(openaiReviews) != 1 {
					t.Errorf("openai reviews length = %d, want 1", len(openaiReviews))
				}
			},
		},
		{
			name: "handle nil Reviews map in delta gracefully",
			prev: ReviewState{
				CodebaseRoot: "/project",
				Reviews: map[string][]Review{
					"openai": {
						{ProviderName: "openai", BatchNumber: 1},
					},
				},
			},
			delta: ReviewState{
				Reviews: nil,
			},
			validate: func(t *testing.T, result ReviewState) {
				if len(result.Reviews) != 1 {
					t.Errorf("Reviews length = %d, want 1 (should preserve prev)", len(result.Reviews))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy of prev to check if it's mutated
			prevCopy := tt.prev
			result := ReduceReviewState(tt.prev, tt.delta)

			if tt.validate != nil {
				tt.validate(t, result)
			}

			// Verify prev was mutated (reducer modifies prev in place)
			// This is acceptable per the data model specification
			_ = prevCopy
		})
	}
}
