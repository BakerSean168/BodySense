package httpapi

import (
	"context"
	"encoding/json"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type healthWorkspaceService interface {
	Get(ctx context.Context, userID uuid.UUID) (*service.HealthWorkspace, error)
}

func (s *PublicServer) WithHealthWorkspace(service healthWorkspaceService) *PublicServer {
	s.healthWorkspace = service
	return s
}

func (s *PublicServer) GetHealthWorkspace(
	ctx context.Context,
	_ openapiv1.GetHealthWorkspaceRequestObject,
) (openapiv1.GetHealthWorkspaceResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getWorkspaceError401("UNAUTHORIZED", "authentication required"), nil
	}
	if s.healthWorkspace == nil {
		return getWorkspaceError500("INTERNAL_ERROR", "health workspace service is unavailable"), nil
	}

	workspace, err := s.healthWorkspace.Get(ctx, userID)
	if err != nil {
		return getWorkspaceError500("INTERNAL_ERROR", "failed to load health workspace"), nil
	}
	if workspace == nil {
		return getWorkspaceError500("INTERNAL_ERROR", "health workspace projection is unavailable"), nil
	}
	response, err := healthWorkspaceToOpenAPI(workspace)
	if err != nil {
		return getWorkspaceError500("INTERNAL_ERROR", "health workspace violates the public contract"), nil
	}
	return openapiv1.GetHealthWorkspace200JSONResponse(response), nil
}

// HealthWorkspaceService currently owns an application read projection whose
// JSON representation is already the public wire. Until the later read-model
// cleanup moves that projection out of dto/, use one strict presenter bridge:
// encode the application projection, then decode it into the generated transport
// model with unknown-field rejection. The generated type never enters service or
// domain packages.
func healthWorkspaceToOpenAPI(workspace *service.HealthWorkspace) (openapiv1.HealthWorkspace, error) {
	encoded, err := json.Marshal(workspace)
	if err != nil {
		return openapiv1.HealthWorkspace{}, err
	}
	var projected map[string]any
	if err := json.Unmarshal(encoded, &projected); err != nil {
		return openapiv1.HealthWorkspace{}, err
	}
	if diagnosis, ok := projected["diagnosis"].(map[string]any); ok && len(diagnosis) > 0 {
		projected["diagnosis"] = projectDiagnosisPayload(diagnosis)
	}
	if treatment, ok := projected["treatment"].(map[string]any); ok && len(treatment) > 0 {
		sanitizeTreatmentMap(treatment)
	}
	if plan, ok := projected["training_plan"].(map[string]any); ok && len(plan) > 0 {
		sanitizeTrainingPlanMap(plan)
	}
	if revisions, ok := projected["treatment_revisions"].([]any); ok {
		for _, raw := range revisions {
			if revision, ok := raw.(map[string]any); ok {
				sanitizeTreatmentRevisionMap(revision)
			}
		}
	}
	if outcomes, ok := projected["recent_outcomes"].([]any); ok {
		for _, raw := range outcomes {
			if outcome, ok := raw.(map[string]any); ok {
				sanitizeOutcomeMap(outcome)
			}
		}
	}
	return strictOpenAPIConvert[openapiv1.HealthWorkspace]("HealthWorkspace", projected)
}

func getWorkspaceError401(code, message string) openapiv1.GetHealthWorkspace401JSONResponse {
	return openapiv1.GetHealthWorkspace401JSONResponse{
		UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope(code, message)),
	}
}

func getWorkspaceError500(code, message string) openapiv1.GetHealthWorkspace500JSONResponse {
	return openapiv1.GetHealthWorkspace500JSONResponse{
		InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope(code, message)),
	}
}
