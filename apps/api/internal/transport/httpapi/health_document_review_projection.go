package httpapi

import (
	"encoding/json"
	"errors"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
)

func strictReviewRecord(record *service.DocumentIndicatorReviewRecord) (openapiv1.DocumentIndicatorReviewRecord, error) {
	if record == nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, errors.New("review record is nil")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.DocumentIndicatorReviewRecord{}, err
	}
	delete(projected, "reviewer_user_id")
	return strictOpenAPIConvert[openapiv1.DocumentIndicatorReviewRecord]("DocumentIndicatorReviewRecord", projected)
}

func strictReviewProjectionList(items []service.DocumentIndicatorReviewProjection) ([]openapiv1.DocumentIndicatorReviewProjection, error) {
	projected := make([]openapiv1.DocumentIndicatorReviewProjection, 0, len(items))
	for i := range items {
		item, err := strictReviewProjection(&items[i])
		if err != nil {
			return nil, err
		}
		projected = append(projected, item)
	}
	return projected, nil
}

func strictReviewProjection(value *service.DocumentIndicatorReviewProjection) (openapiv1.DocumentIndicatorReviewProjection, error) {
	if value == nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, errors.New("review projection is nil")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.DocumentIndicatorReviewProjection{}, err
	}
	stripReviewRecordIdentity(projected["effective_review"])
	if history, ok := projected["history"].([]any); ok {
		for _, item := range history {
			stripReviewRecordIdentity(item)
		}
	}
	return strictOpenAPIConvert[openapiv1.DocumentIndicatorReviewProjection]("DocumentIndicatorReviewProjection", projected)
}

func strictHealthDocumentReviewContext(value *service.HealthDocumentReviewContext) (openapiv1.HealthDocumentReviewContext, error) {
	if value == nil {
		return openapiv1.HealthDocumentReviewContext{}, errors.New("review context is nil")
	}
	projected, err := strictReviewProjectionList(value.ReviewCandidates)
	if err != nil {
		return openapiv1.HealthDocumentReviewContext{}, err
	}
	return openapiv1.HealthDocumentReviewContext{
		ExtractionRunId:    value.ExtractionRunID,
		UploadId:           value.AssetID,
		SourceAvailability: openapiv1.HealthDocumentReviewContextSourceAvailability(value.SourceAvailability),
		ReviewCandidates:   projected,
	}, nil
}

func stripReviewRecordIdentity(value any) {
	if record, ok := value.(map[string]any); ok {
		delete(record, "reviewer_user_id")
	}
}
