package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// PostureAnalysisView is one completed single-view analysis, stripped of raw
// file metadata so consultation tools and assessment can consume it safely.
type PostureAnalysisView struct {
	UploadID       string          `json:"upload_id"`
	View           string          `json:"view"`
	FileType       string          `json:"file_type"`
	AnalysisStatus string          `json:"analysis_status"`
	Analysis       json.RawMessage `json:"analysis"`
	CreatedAt      time.Time       `json:"created_at"`
}

// PostureAnalysisSummary is the Agent/assessment-facing aggregate of the
// user's latest completed three-view posture analyses.
type PostureAnalysisSummary struct {
	HasAnalysis bool                  `json:"has_analysis"`
	Views       []PostureAnalysisView `json:"views"`
	// Findings is a flattened list of finding objects across views for quick use.
	Findings []any `json:"findings"`
	// Summaries is per-view summary_markdown text when present.
	Summaries []string `json:"summaries"`
}

// BuildPostureAnalysisSummary collapses completed posture runs into the compact
// shape shared by the read-only HTTP endpoint, consultation business context,
// and assessment reuse. Prefer the newest completed run per view.
func BuildPostureAnalysisSummary(runs []model.PostureAnalysisRun) PostureAnalysisSummary {
	summary := PostureAnalysisSummary{
		HasAnalysis: false,
		Views:       []PostureAnalysisView{},
		Findings:    []any{},
		Summaries:   []string{},
	}
	if len(runs) == 0 {
		return summary
	}

	seenView := map[string]bool{}
	for _, run := range runs {
		if seenView[run.View] {
			continue
		}
		if len(run.ResultPayload) == 0 {
			continue
		}
		seenView[run.View] = true
		summary.HasAnalysis = true
		summary.Views = append(summary.Views, PostureAnalysisView{
			UploadID:       run.AssetID.String(),
			View:           run.View,
			FileType:       run.FileType(),
			AnalysisStatus: string(run.Status),
			Analysis:       json.RawMessage(run.ResultPayload),
			CreatedAt:      run.CreatedAt,
		})

		var parsed map[string]any
		if err := json.Unmarshal(run.ResultPayload, &parsed); err == nil {
			if findings, ok := parsed["findings"].([]any); ok {
				summary.Findings = append(summary.Findings, findings...)
			}
			if text, ok := parsed["summary_markdown"].(string); ok && text != "" {
				summary.Summaries = append(summary.Summaries, text)
			}
		}
	}
	return summary
}

// GetPostureAnalysisSummary loads and collapses the caller's completed posture
// analyses. Empty result (no completed analysis) is not an error.
func (s *UploadService) GetPostureAnalysisSummary(ctx context.Context, userID uuid.UUID) (PostureAnalysisSummary, error) {
	runs, err := s.postureRuns.ListCompletedByUser(ctx, userID)
	if err != nil {
		return PostureAnalysisSummary{}, err
	}
	return BuildPostureAnalysisSummary(runs), nil
}
