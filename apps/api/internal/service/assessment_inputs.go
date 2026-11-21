package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func (s *AssessmentService) collectReviewedReportEvidence(ctx context.Context, userID uuid.UUID, uploads []model.UserUploadView) (json.RawMessage, error) {
	if s.reviews == nil {
		return json.RawMessage(`[]`), nil
	}
	reportUploads := make([]model.UserUploadView, 0, len(uploads))
	reviewsByUpload := make(map[uuid.UUID][]model.DocumentIndicatorReview, len(uploads))
	for _, upload := range uploads {
		if upload.FileType != "report" {
			continue
		}
		reportUploads = append(reportUploads, upload)
		rows, err := s.reviews.ListByAsset(ctx, upload.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("load document indicator reviews for upload %s: %w", upload.ID, err)
		}
		if len(rows) > 0 {
			reviewsByUpload[upload.ID] = rows
		}
	}
	reviewed := buildAssessmentReviewedIndicators(reportUploads, reviewsByUpload)
	encoded, err := json.Marshal(reviewed)
	if err != nil {
		return nil, fmt.Errorf("encode reviewed report evidence: %w", err)
	}
	return encoded, nil
}

func assessmentInputsFromUploads(uploads []model.UserUploadView) []any {
	reportIndicators := make([]any, 0)
	for _, upload := range uploads {
		switch upload.FileType {
		case "report":
			if upload.OCRStatus != "completed" {
				continue
			}
			var ocrResponse map[string]any
			if json.Unmarshal(upload.OCRResult, &ocrResponse) == nil {
				if result, ok := ocrResponse["result"].(map[string]any); ok {
					if indicators, ok := result["indicators"].([]any); ok {
						for indicatorIndex, indicator := range indicators {
							reportIndicators = append(reportIndicators, map[string]any{
								"upload_id":       upload.ID.String(),
								"indicator_index": indicatorIndex,
								"value":           indicator,
							})
						}
					}
				}
			}
		}
	}
	return reportIndicators
}
