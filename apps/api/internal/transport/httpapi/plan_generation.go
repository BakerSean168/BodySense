package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

// planGenerationApplication is the Treatment Agent's PlanProposal(create)
// generation entry point, so the generation path keeps one Go gate and one
// Agent wiring.
type planGenerationApplication interface {
	GeneratePlanProposal(context.Context, uuid.UUID, service.TreatmentPlanGenerationInput) (*service.TreatmentPlanGenerationResult, error)
}

func (s *PublicServer) WithPlanGeneration(generator planGenerationApplication) *PublicServer {
	s.planGenerator = generator
	return s
}

var planGenerationUnavailable = trainingPlanHTTPError{http.StatusServiceUnavailable, "PLAN_GENERATION_UNAVAILABLE", "training plan generation is not configured"}

func (r trainingPlanHTTPError) VisitGeneratePlanProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}

// GeneratePlanProposal asks the Treatment Agent for a plan draft and returns
// the pending agent PlanProposal(create). The owner comes only from auth; the
// body carries the user's command and never an Agent result or decision.
func (s *PublicServer) GeneratePlanProposal(ctx context.Context, req openapiv1.GeneratePlanProposalRequestObject) (openapiv1.GeneratePlanProposalResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil {
		return planUnauthorized, nil
	}
	generator := s.planGenerator
	if generator == nil {
		return planGenerationUnavailable, nil
	}
	if req.Body == nil || (req.Body.DiagnosisRevision != nil && *req.Body.DiagnosisRevision < 1) {
		return planBadRequest, nil
	}
	input := service.TreatmentPlanGenerationInput{IdempotencyKey: string(req.Body.IdempotencyKey), UserConstraints: map[string]any{}}
	if req.Body.DiagnosisRevision != nil {
		revision := *req.Body.DiagnosisRevision
		input.ExpectedDiagnosisRevision = &revision
	}
	if req.Body.UserConstraints != nil {
		input.UserConstraints = map[string]any(*req.Body.UserConstraints)
	}
	if req.Body.ChangeReason != nil {
		input.ChangeReason = strings.TrimSpace(*req.Body.ChangeReason)
	}
	result, err := generator.GeneratePlanProposal(applicationRequestContext(ctx), owner, input)
	if err != nil {
		return planGenerationError(err), nil
	}
	if result == nil || result.Proposal == nil || result.Proposal.IdempotencyKey != input.IdempotencyKey ||
		result.Proposal.InitiatedBy != trainingplan.InitiatorAgent || result.Proposal.Kind != trainingplan.ProposalCreate {
		return planInconsistent, nil
	}
	record, err := planProposalRecord(owner, *result.Proposal)
	if err != nil {
		return planInconsistent, nil
	}
	if result.Replayed {
		return openapiv1.GeneratePlanProposal200JSONResponse(record), nil
	}
	return openapiv1.GeneratePlanProposal201JSONResponse(record), nil
}

func planGenerationError(err error) trainingPlanHTTPError {
	switch {
	case errors.Is(err, service.ErrTreatmentPlanGenerationUnavailable):
		return planGenerationUnavailable
	case errors.Is(err, service.ErrTreatmentDiagnosisNotReady):
		return trainingPlanHTTPError{http.StatusConflict, "DIAGNOSIS_NOT_READY", "adopt at least one Diagnosis condition before plan generation"}
	case errors.Is(err, service.ErrTreatmentDiagnosisStale):
		return trainingPlanHTTPError{http.StatusConflict, "DIAGNOSIS_STALE", "the Diagnosis changed; review the current conditions before plan generation"}
	case errors.Is(err, service.ErrTreatmentSafetyBlocked):
		return trainingPlanHTTPError{http.StatusConflict, "SAFETY_BLOCKED", "current safety state blocks ordinary plan generation"}
	case errors.Is(err, service.ErrTreatmentPlanUnavailable),
		errors.Is(err, service.ErrTreatmentPlanOutputInvalid),
		errors.Is(err, service.ErrTreatmentConfigurationMismatch):
		return trainingPlanHTTPError{http.StatusBadGateway, "AGENT_OUTPUT_REJECTED", "the Agent did not return an admissible training plan; nothing was saved"}
	case errors.Is(err, service.ErrPlanNotFound):
		return planUnavailable
	case errors.Is(err, service.ErrPlanInvalid),
		errors.Is(err, service.ErrPlanConflict),
		errors.Is(err, service.ErrPlanContextUnavailable):
		return trainingPlanCommandError(err)
	default:
		return trainingPlanHTTPError{http.StatusInternalServerError, "INTERNAL_ERROR", "training plan generation failed"}
	}
}
