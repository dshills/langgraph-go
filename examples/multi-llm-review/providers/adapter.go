package providers

import (
	"context"

	"github.com/dshills/langgraph-go/examples/multi-llm-review/workflow"
)

// ProviderAdapter adapts a providers.CodeReviewer to the workflow.CodeReviewer interface.
// This enables the workflow package to use providers without creating a circular dependency.
type ProviderAdapter struct {
	Provider CodeReviewer
}

// ReviewBatch implements workflow.CodeReviewer by converting types and delegating to the underlying provider.
func (a *ProviderAdapter) ReviewBatch(ctx context.Context, req workflow.ReviewRequest) (workflow.ReviewResponse, error) {
	// Convert workflow.ReviewRequest to providers.ReviewRequest
	providerReq := ReviewRequest{
		Files:       make([]CodeFile, len(req.Files)),
		FocusAreas:  req.FocusAreas,
		Language:    req.Language,
		BatchNumber: req.BatchNumber,
	}

	for i, file := range req.Files {
		providerReq.Files[i] = CodeFile{
			FilePath:  file.FilePath,
			Content:   file.Content,
			Language:  file.Language,
			LineCount: file.LineCount,
			SizeBytes: file.SizeBytes,
			Checksum:  file.Checksum,
		}
	}

	// Call underlying provider
	resp, err := a.Provider.ReviewBatch(ctx, providerReq)
	if err != nil {
		return workflow.ReviewResponse{}, err
	}

	// Convert providers.ReviewResponse to workflow.ReviewResponse
	workflowResp := workflow.ReviewResponse{
		Issues:       make([]workflow.ReviewIssueFromProvider, len(resp.Issues)),
		TokensUsed:   resp.TokensUsed,
		Duration:     resp.Duration,
		ProviderName: resp.ProviderName,
	}

	for i, issue := range resp.Issues {
		workflowResp.Issues[i] = workflow.ReviewIssueFromProvider{
			File:         issue.File,
			Line:         issue.Line,
			Severity:     issue.Severity,
			Category:     issue.Category,
			Description:  issue.Description,
			Remediation:  issue.Remediation,
			ProviderName: issue.ProviderName,
			Confidence:   issue.Confidence,
		}
	}

	return workflowResp, nil
}

// Name implements workflow.CodeReviewer by delegating to the underlying provider.
func (a *ProviderAdapter) Name() string {
	return a.Provider.Name()
}
