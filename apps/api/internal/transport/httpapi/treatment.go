package httpapi

import (
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
)

// Treatment projections still read by the legacy workspace and /training
// responses. They are removed with those fields.

func strictTreatmentRevision(value *model.TreatmentRevision) (openapiv1.TreatmentRevision, error) {
	projected, err := publicTreatmentRevisionMap(value)
	if err != nil {
		return openapiv1.TreatmentRevision{}, err
	}
	return strictOpenAPIConvert[openapiv1.TreatmentRevision]("TreatmentRevision", projected)
}

func publicTreatmentRevisionMap(value *model.TreatmentRevision) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeTreatmentRevisionMap(projected)
	return projected, nil
}

func sanitizeTreatmentRevisionMap(projected map[string]any) {
	if interventions, ok := projected["interventions"].([]any); ok {
		for _, raw := range interventions {
			if intervention, ok := raw.(map[string]any); ok {
				delete(intervention, "user_id")
			}
		}
	}
}

