package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"net/http"
	"time"
)

// trainingPlanApplication is the Go-owned independent TrainingPlan boundary
// (TP-002). HTTP supplies only the authenticated owner and the user's explicit
// command; it never accepts an Agent decision, a caller-owned user id or a
// precomputed Safety verdict.
type trainingPlanApplication interface {
	ListViews(context.Context, uuid.UUID) ([]service.TrainingPlanView, error)
	View(context.Context, uuid.UUID, uuid.UUID) (*service.TrainingPlanView, error)
	Revision(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*model.TrainingPlanRevision, error)
	ChangeStatus(context.Context, uuid.UUID, service.PlanStatusCommand) (*model.TrainingPlanStatusEvent, error)
	ListProposals(context.Context, uuid.UUID, trainingplan.ProposalState) ([]model.PlanProposal, error)
	GetProposal(context.Context, uuid.UUID, uuid.UUID) (*model.PlanProposal, error)
	Propose(context.Context, uuid.UUID, service.PlanProposalInput) (*model.PlanProposal, error)
	Accept(context.Context, uuid.UUID, service.PlanDecision) (*service.PlanAcceptance, error)
	Reject(context.Context, uuid.UUID, service.PlanDecision) (*model.PlanDecisionReceipt, error)
}

func (s *PublicServer) WithTrainingPlans(plans trainingPlanApplication) *PublicServer {
	s.trainingPlans = plans
	return s
}

type trainingPlanHTTPError struct {
	status  int
	code    string
	message string
}

func (r trainingPlanHTTPError) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}
func (r trainingPlanHTTPError) VisitListTrainingPlanRecordsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitGetTrainingPlanRecordResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitGetTrainingPlanRevisionRecordResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitChangeTrainingPlanStatusResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitListPlanProposalRecordsResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitGetPlanProposalRecordResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitCreatePlanProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitAcceptPlanProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r trainingPlanHTTPError) VisitRejectPlanProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}

var (
	planUnauthorized = trainingPlanHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}
	planUnavailable  = trainingPlanHTTPError{http.StatusServiceUnavailable, "TRAINING_PLAN_UNAVAILABLE", "training plan service unavailable"}
	planInconsistent = trainingPlanHTTPError{http.StatusServiceUnavailable, "TRAINING_PLAN_UNAVAILABLE", "training plan projection violates contract"}
	planBadRequest   = trainingPlanHTTPError{http.StatusBadRequest, "INVALID_REQUEST", "invalid training plan command"}
)

func (s *PublicServer) trainingPlanOwner(ctx context.Context) (uuid.UUID, *trainingPlanHTTPError) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return uuid.Nil, &planUnauthorized
	}
	if s.trainingPlans == nil {
		return uuid.Nil, &planUnavailable
	}
	return owner, nil
}

// trainingPlanCommandError maps service errors. Foreign and missing ids are
// the same owner-masked 404; unreadable context is 503 (fail closed).
func trainingPlanCommandError(err error) trainingPlanHTTPError {
	if verdict, ok := service.IsPlanAdmission(err); ok {
		switch verdict {
		case trainingplan.ProposalUnavailable:
			return trainingPlanHTTPError{http.StatusNotFound, "NOT_FOUND", "training plan or proposal not found"}
		case trainingplan.ProposalTimedOut:
			return trainingPlanHTTPError{http.StatusConflict, "PROPOSAL_EXPIRED", "proposal expired after 7 days; request a new one"}
		case trainingplan.ContextChanged:
			return trainingPlanHTTPError{http.StatusConflict, "CONTEXT_CHANGED", "body, diagnosis or safety context changed; proposal expired, review a new one"}
		case trainingplan.PlanRevisionChanged:
			return trainingPlanHTTPError{http.StatusConflict, "REVISION_CONFLICT", "plan changed since this proposal; proposal expired"}
		case trainingplan.ContentChanged:
			return trainingPlanHTTPError{http.StatusConflict, "CONTENT_CHANGED", "presented content does not match the proposal"}
		case trainingplan.ProposalNotPending:
			return trainingPlanHTTPError{http.StatusConflict, "PROPOSAL_NOT_PENDING", "proposal already decided or expired"}
		case trainingplan.ContextUnavailable:
			return trainingPlanHTTPError{http.StatusServiceUnavailable, "PLAN_CONTEXT_UNAVAILABLE", "authoritative plan context unavailable"}
		default:
			return planBadRequest
		}
	}
	switch {
	case errors.Is(err, service.ErrPlanNotFound):
		return trainingPlanHTTPError{http.StatusNotFound, "NOT_FOUND", "training plan or proposal not found"}
	case errors.Is(err, service.ErrPlanInvalid):
		return planBadRequest
	case errors.Is(err, service.ErrPlanConflict):
		return trainingPlanHTTPError{http.StatusConflict, "REVISION_CONFLICT", "plan, proposal or idempotency key conflict; refresh and retry"}
	case errors.Is(err, service.ErrPlanContextUnavailable):
		return trainingPlanHTTPError{http.StatusServiceUnavailable, "PLAN_CONTEXT_UNAVAILABLE", "authoritative plan context unavailable"}
	default:
		return planUnavailable
	}
}

// Wire shapes mirror the OpenAPI components; strictOpenAPIConvert validates.
type planGateWire struct {
	Result                safety.Result `json:"result"`
	PermitsExecution      bool          `json:"permitsExecution"`
	Reasons               []string      `json:"reasons"`
	MatchedRestrictionIDs []uuid.UUID   `json:"matchedRestrictionIds"`
	RequiresReview        bool          `json:"requiresReview"`
	UrgentCare            bool          `json:"urgentCare"`
	SafetyRevision        int64         `json:"safetyRevision"`
	PolicyRevision        string        `json:"policyRevision,omitempty"`
}
type planReviewWire struct {
	Recommended          bool   `json:"recommended"`
	Reason               string `json:"reason,omitempty"`
	SessionFeedbackCount *int64 `json:"sessionFeedbackCount,omitempty"`
}
type planRecordWire struct {
	PlanID                    uuid.UUID                 `json:"planId"`
	Title                     string                    `json:"title"`
	Goal                      string                    `json:"goal"`
	Status                    string                    `json:"status"`
	StatusRevision            int64                     `json:"statusRevision"`
	CurrentAcceptedRevisionID uuid.UUID                 `json:"currentAcceptedRevisionId"`
	RelatedConditionIDs       []uuid.UUID               `json:"relatedConditionIds"`
	RelatedBodyRefs           []model.PlanBodyRef       `json:"relatedBodyRefs"`
	CurrentContent            model.TrainingPlanContent `json:"currentContent"`
	Review                    planReviewWire            `json:"review"`
	ExecutionGate             planGateWire              `json:"executionGate"`
	CreatedAt                 time.Time                 `json:"createdAt"`
}
type planRevisionWire struct {
	RevisionID             uuid.UUID               `json:"revisionId"`
	PlanID                 uuid.UUID               `json:"planId"`
	ParentRevisionID       *uuid.UUID              `json:"parentRevisionId,omitempty"`
	Snapshot               model.TrainingPlanDraft `json:"snapshot"`
	SourceKind             string                  `json:"sourceKind"`
	SourceProposalID       uuid.UUID               `json:"sourceProposalId"`
	AuthorizationReceiptID uuid.UUID               `json:"authorizationReceiptId"`
	CreatedAt              time.Time               `json:"createdAt"`
}
type planProposalWire struct {
	ProposalID         uuid.UUID               `json:"proposalId"`
	Kind               string                  `json:"kind"`
	State              string                  `json:"state"`
	InitiatedBy        string                  `json:"initiatedBy"`
	PlanID             *uuid.UUID              `json:"planId,omitempty"`
	BasedOnRevisionID  *uuid.UUID              `json:"basedOnRevisionId,omitempty"`
	Draft              model.TrainingPlanDraft `json:"draft"`
	ContentFingerprint string                  `json:"contentFingerprint"`
	CreatedAt          time.Time               `json:"createdAt"`
	ExpiresAt          time.Time               `json:"expiresAt"`
	ExpiredReason      *string                 `json:"expiredReason,omitempty"`
	DecisionID         *uuid.UUID              `json:"decisionId,omitempty"`
}

// normalizePlanDraft makes optional collections explicit arrays on the wire.
func normalizePlanDraft(d model.TrainingPlanDraft) model.TrainingPlanDraft {
	if d.RelatedConditionIDs == nil {
		d.RelatedConditionIDs = []uuid.UUID{}
	}
	if d.RelatedBodyRefs == nil {
		d.RelatedBodyRefs = []model.PlanBodyRef{}
	}
	if d.Content.Constraints == nil {
		d.Content.Constraints = []string{}
	}
	for i := range d.Content.Sessions {
		for j := range d.Content.Sessions[i].Stages {
			if d.Content.Sessions[i].Stages[j].Exercises == nil {
				d.Content.Sessions[i].Stages[j].Exercises = []model.PlanExercise{}
			}
		}
	}
	return d
}

func planGate(d safety.Decision) planGateWire {
	w := planGateWire{Result: d.Result, PermitsExecution: d.Permits(), Reasons: []string{}, MatchedRestrictionIDs: []uuid.UUID{},
		RequiresReview: d.RequiresReview, UrgentCare: d.UrgentCare, SafetyRevision: d.SafetyRevision, PolicyRevision: d.PolicyRevision}
	if w.SafetyRevision < 0 {
		w.SafetyRevision = 0
	}
	switch d.Result {
	case safety.ResultAllow, safety.ResultWarn, safety.ResultBlock:
	default:
		// Anything Go cannot name is unavailable, never allow.
		w.Result, w.PermitsExecution = safety.ResultUnavailable, false
	}
	for _, r := range d.Reasons {
		w.Reasons = append(w.Reasons, string(r))
	}
	w.MatchedRestrictionIDs = append(w.MatchedRestrictionIDs, d.MatchedRestrictionIDs...)
	return w
}

func planRecord(owner uuid.UUID, v service.TrainingPlanView) (openapiv1.TrainingPlanRecord, error) {
	if v.Plan.UserID != owner || v.Revision.UserID != owner || v.Plan.CurrentAcceptedRevisionID == nil ||
		*v.Plan.CurrentAcceptedRevisionID != v.Revision.ID || v.Revision.PlanID != v.Plan.ID {
		return openapiv1.TrainingPlanRecord{}, errors.New("plan projection owner or revision mismatch")
	}
	d := normalizePlanDraft(v.Draft)
	wire := planRecordWire{
		PlanID: v.Plan.ID, Title: v.Plan.Title, Goal: v.Plan.Goal, Status: v.Plan.Status, StatusRevision: v.Plan.StatusRevision,
		CurrentAcceptedRevisionID: v.Revision.ID, RelatedConditionIDs: d.RelatedConditionIDs, RelatedBodyRefs: d.RelatedBodyRefs,
		CurrentContent: d.Content, Review: planReviewWire{Recommended: v.Review.Recommended, Reason: v.Review.Reason, SessionFeedbackCount: v.Review.SessionFeedbackCount},
		ExecutionGate: planGate(v.Gate), CreatedAt: v.Plan.CreatedAt,
	}
	return strictOpenAPIConvert[openapiv1.TrainingPlanRecord]("TrainingPlanRecord", wire)
}

func planProposalRecord(owner uuid.UUID, p model.PlanProposal) (openapiv1.PlanProposalRecord, error) {
	if p.UserID != owner {
		return openapiv1.PlanProposalRecord{}, errors.New("proposal owner mismatch")
	}
	var draft model.TrainingPlanDraft
	if err := json.Unmarshal(p.Draft, &draft); err != nil {
		return openapiv1.PlanProposalRecord{}, err
	}
	wire := planProposalWire{
		ProposalID: p.ID, Kind: string(p.Kind), State: string(p.State), InitiatedBy: string(p.InitiatedBy),
		PlanID: p.PlanID, BasedOnRevisionID: p.BasedOnRevisionID, Draft: normalizePlanDraft(draft),
		ContentFingerprint: p.ContentFingerprint, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt,
		ExpiredReason: p.ExpiredReason, DecisionID: p.DecisionID,
	}
	return strictOpenAPIConvert[openapiv1.PlanProposalRecord]("PlanProposalRecord", wire)
}
