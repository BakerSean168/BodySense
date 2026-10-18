package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// outcomeApplication records and lists the user's post-intervention Outcomes.
type outcomeApplication interface {
	RecordOutcome(context.Context, uuid.UUID, model.Outcome) (*model.Outcome, bool, error)
	ListOutcomes(context.Context, uuid.UUID, int) ([]model.Outcome, error)
}

func (s *PublicServer) WithOutcomes(outcomes outcomeApplication) *PublicServer {
	s.outcomes = outcomes
	return s
}

type outcomeHTTPErrorResponse struct {
	status  int
	code    string
	message string
}

var outcomeUnavailable = outcomeHTTPErrorResponse{
	status: http.StatusServiceUnavailable, code: "OUTCOME_DOMAIN_UNAVAILABLE", message: "outcome service is not configured",
}

func (r outcomeHTTPErrorResponse) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r outcomeHTTPErrorResponse) VisitRecordOutcomeResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r outcomeHTTPErrorResponse) VisitListOutcomesResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (s *PublicServer) RecordOutcome(
	ctx context.Context,
	request openapiv1.RecordOutcomeRequestObject,
) (openapiv1.RecordOutcomeResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return outcomeHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.outcomes == nil || request.Body == nil {
		return outcomeUnavailable, nil
	}
	occurredAt := time.Now().UTC()
	if request.Body.OccurredAt != nil {
		occurredAt = *request.Body.OccurredAt
	}
	value := datatypes.JSON([]byte(`{}`))
	if request.Body.Value != nil {
		value = jsonObjectBytes(*request.Body.Value)
	}
	provenance := datatypes.JSON([]byte(`{}`))
	if request.Body.Provenance != nil {
		provenance = jsonObjectBytes(*request.Body.Provenance)
	}
	outcome := model.Outcome{
		TreatmentID:          uuidPointer(request.Body.TreatmentId),
		TreatmentRevisionID:  uuidPointer(request.Body.TreatmentRevisionId),
		InterventionID:       uuidPointer(request.Body.InterventionId),
		SourceType:           request.Body.SourceType,
		SourceKey:            request.Body.SourceKey,
		Kind:                 request.Body.Kind,
		ConcernKey:           stringOrEmpty(request.Body.ConcernKey),
		BodyRegion:           stringOrEmpty(request.Body.BodyRegion),
		Value:                value,
		Notes:                stringOrEmpty(request.Body.Notes),
		AssociationStatement: stringOrEmpty(request.Body.AssociationStatement),
		CausalityLevel:       optionalEnumString(request.Body.CausalityLevel),
		OccurredAt:           occurredAt,
		Provenance:           provenance,
	}
	stored, created, err := s.outcomes.RecordOutcome(ctx, uid, outcome)
	if err != nil {
		return outcomeHTTPErrorResponse{status: http.StatusBadRequest, code: "OUTCOME_OPERATION_FAILED", message: err.Error()}, nil
	}
	publicOutcome, err := strictOutcome(stored)
	if err != nil || publicOutcome == nil {
		return nil, err
	}
	response := openapiv1.OutcomeMutationResponse{Outcome: *publicOutcome, Created: created}
	if created {
		return openapiv1.RecordOutcome201JSONResponse(response), nil
	}
	return openapiv1.RecordOutcome200JSONResponse(response), nil
}

func (s *PublicServer) ListOutcomes(
	ctx context.Context,
	request openapiv1.ListOutcomesRequestObject,
) (openapiv1.ListOutcomesResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return outcomeHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "missing authenticated user"}, nil
	}
	if s.outcomes == nil {
		return outcomeUnavailable, nil
	}
	limit := 100
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	items, err := s.outcomes.ListOutcomes(ctx, uid, limit)
	if err != nil {
		return outcomeHTTPErrorResponse{status: http.StatusInternalServerError, code: "INTERNAL_ERROR", message: "failed to load outcomes"}, nil
	}
	publicItems := make([]openapiv1.Outcome, 0, len(items))
	for i := range items {
		item, convertErr := strictOutcome(&items[i])
		if convertErr != nil {
			return nil, convertErr
		}
		publicItems = append(publicItems, *item)
	}
	return openapiv1.ListOutcomes200JSONResponse{Outcomes: publicItems}, nil
}

func strictTrainingPlan(value *model.TrainingPlan) (*openapiv1.TrainingPlan, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := publicTrainingPlanMap(value)
	if err != nil {
		return nil, err
	}
	converted, err := strictOpenAPIConvert[openapiv1.TrainingPlan]("TrainingPlan", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func strictOutcome(value *model.Outcome) (*openapiv1.Outcome, error) {
	if value == nil {
		return nil, nil
	}
	projected, err := publicOutcomeMap(value)
	if err != nil {
		return nil, err
	}
	converted, err := strictOpenAPIConvert[openapiv1.Outcome]("Outcome", projected)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func publicTrainingPlanMap(value *model.TrainingPlan) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeTrainingPlanMap(projected)
	return projected, nil
}

func sanitizeTrainingPlanMap(projected map[string]any) {
	delete(projected, "user_id")
}

func publicOutcomeMap(value *model.Outcome) (map[string]any, error) {
	projected, err := marshalPublicMap(value)
	if err != nil {
		return nil, err
	}
	sanitizeOutcomeMap(projected)
	return projected, nil
}

func sanitizeOutcomeMap(projected map[string]any) {
	delete(projected, "user_id")
}

func marshalPublicMap(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return nil, err
	}
	return projected, nil
}

func jsonObjectBytes(value openapiv1.JsonObject) datatypes.JSON {
	encoded, _ := json.Marshal(value)
	return datatypes.JSON(encoded)
}

func uuidPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalEnumString[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
