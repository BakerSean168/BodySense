package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bodysense/api/internal/domain/safety"
	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
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
	Recommended bool   `json:"recommended"`
	Reason      string `json:"reason,omitempty"`
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
		CurrentContent: d.Content, Review: planReviewWire{Recommended: v.Review.Recommended, Reason: v.Review.Reason},
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

func (s *PublicServer) ListTrainingPlanRecords(ctx context.Context, _ openapiv1.ListTrainingPlanRecordsRequestObject) (openapiv1.ListTrainingPlanRecordsResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	views, err := s.trainingPlans.ListViews(applicationRequestContext(ctx), owner)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	out := openapiv1.ListTrainingPlanRecords200JSONResponse{Plans: []openapiv1.TrainingPlanRecord{}}
	for _, v := range views {
		record, err := planRecord(owner, v)
		if err != nil {
			return planInconsistent, nil
		}
		out.Plans = append(out.Plans, record)
	}
	return out, nil
}

func (s *PublicServer) GetTrainingPlanRecord(ctx context.Context, req openapiv1.GetTrainingPlanRecordRequestObject) (openapiv1.GetTrainingPlanRecordResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	view, err := s.trainingPlans.View(applicationRequestContext(ctx), owner, uuid.UUID(req.PlanId))
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if view == nil || view.Plan.ID != uuid.UUID(req.PlanId) {
		return planInconsistent, nil
	}
	record, err := planRecord(owner, *view)
	if err != nil {
		return planInconsistent, nil
	}
	return openapiv1.GetTrainingPlanRecord200JSONResponse(record), nil
}

func (s *PublicServer) GetTrainingPlanRevisionRecord(ctx context.Context, req openapiv1.GetTrainingPlanRevisionRecordRequestObject) (openapiv1.GetTrainingPlanRevisionRecordResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	plan, revisionID := uuid.UUID(req.PlanId), uuid.UUID(req.RevisionId)
	rev, err := s.trainingPlans.Revision(applicationRequestContext(ctx), owner, plan, revisionID)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if rev == nil || rev.UserID != owner || rev.PlanID != plan || rev.ID != revisionID {
		return planInconsistent, nil
	}
	var draft model.TrainingPlanDraft
	proposal, perr := uuid.Parse(rev.SourceID)
	if err := json.Unmarshal(rev.Snapshot, &draft); err != nil || perr != nil {
		return planInconsistent, nil
	}
	wire := planRevisionWire{RevisionID: rev.ID, PlanID: rev.PlanID, ParentRevisionID: rev.ParentRevisionID, Snapshot: normalizePlanDraft(draft),
		SourceKind: rev.SourceKind, SourceProposalID: proposal, AuthorizationReceiptID: rev.AuthorizationID, CreatedAt: rev.CreatedAt}
	out, err := strictOpenAPIConvert[openapiv1.TrainingPlanRevisionRecord]("TrainingPlanRevisionRecord", wire)
	if err != nil {
		return planInconsistent, nil
	}
	return openapiv1.GetTrainingPlanRevisionRecord200JSONResponse(out), nil
}

func (s *PublicServer) ChangeTrainingPlanStatus(ctx context.Context, req openapiv1.ChangeTrainingPlanStatusRequestObject) (openapiv1.ChangeTrainingPlanStatusResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil {
		return planBadRequest, nil
	}
	cmd := service.PlanStatusCommand{PlanID: uuid.UUID(req.PlanId), Status: trainingplan.PlanStatus(req.Body.Status),
		ExpectedStatusRevision: req.Body.ExpectedStatusRevision, IdempotencyKey: string(req.Body.IdempotencyKey)}
	event, err := s.trainingPlans.ChangeStatus(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if event == nil || event.UserID != owner || event.PlanID != cmd.PlanID || event.ToStatus != cmd.Status ||
		event.StatusRevision != cmd.ExpectedStatusRevision+1 || event.IdempotencyKey != cmd.IdempotencyKey {
		return planInconsistent, nil
	}
	return openapiv1.ChangeTrainingPlanStatus200JSONResponse{
		EventId: event.ID, PlanId: event.PlanID, FromStatus: openapiv1.TrainingPlanStatus(event.FromStatus),
		ToStatus: openapiv1.TrainingPlanStatus(event.ToStatus), StatusRevision: event.StatusRevision,
		IdempotencyKey: event.IdempotencyKey, ChangedAt: event.CreatedAt,
	}, nil
}

func (s *PublicServer) ListPlanProposalRecords(ctx context.Context, req openapiv1.ListPlanProposalRecordsRequestObject) (openapiv1.ListPlanProposalRecordsResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	state := trainingplan.ProposalState("")
	if req.Params.State != nil {
		state = trainingplan.ProposalState(*req.Params.State)
	}
	rows, err := s.trainingPlans.ListProposals(applicationRequestContext(ctx), owner, state)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	out := openapiv1.ListPlanProposalRecords200JSONResponse{Proposals: []openapiv1.PlanProposalRecord{}}
	for _, row := range rows {
		record, err := planProposalRecord(owner, row)
		if err != nil || (state != "" && row.State != state) {
			return planInconsistent, nil
		}
		out.Proposals = append(out.Proposals, record)
	}
	return out, nil
}

func (s *PublicServer) GetPlanProposalRecord(ctx context.Context, req openapiv1.GetPlanProposalRecordRequestObject) (openapiv1.GetPlanProposalRecordResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	row, err := s.trainingPlans.GetProposal(applicationRequestContext(ctx), owner, uuid.UUID(req.ProposalId))
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if row == nil || row.ID != uuid.UUID(req.ProposalId) {
		return planInconsistent, nil
	}
	record, err := planProposalRecord(owner, *row)
	if err != nil {
		return planInconsistent, nil
	}
	return openapiv1.GetPlanProposalRecord200JSONResponse(record), nil
}

// CreatePlanProposal saves the user's own draft as a pending proposal. The
// initiator is fixed to user server-side; acceptance is a separate command.
func (s *PublicServer) CreatePlanProposal(ctx context.Context, req openapiv1.CreatePlanProposalRequestObject) (openapiv1.CreatePlanProposalResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	if req.Body == nil || (req.Body.PlanId == nil) != (req.Body.ExpectedRevisionId == nil) {
		return planBadRequest, nil
	}
	encoded, err := json.Marshal(req.Body.Draft)
	if err != nil {
		return planBadRequest, nil
	}
	draft, err := strictJSONDecode[model.TrainingPlanDraft](encoded)
	if err != nil {
		return planBadRequest, nil
	}
	input := service.PlanProposalInput{Draft: draft, InitiatedBy: trainingplan.InitiatorUser,
		SourceID: "user_api:" + string(req.Body.IdempotencyKey), IdempotencyKey: string(req.Body.IdempotencyKey)}
	if req.Body.PlanId != nil {
		input.PlanID, input.ExpectedRevisionID = uuid.UUID(*req.Body.PlanId), uuid.UUID(*req.Body.ExpectedRevisionId)
		if input.PlanID == uuid.Nil || input.ExpectedRevisionID == uuid.Nil {
			return planBadRequest, nil
		}
	}
	proposal, err := s.trainingPlans.Propose(applicationRequestContext(ctx), owner, input)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if proposal == nil || proposal.IdempotencyKey != input.IdempotencyKey {
		return planInconsistent, nil
	}
	record, err := planProposalRecord(owner, *proposal)
	if err != nil {
		return planInconsistent, nil
	}
	return openapiv1.CreatePlanProposal200JSONResponse(record), nil
}

func planDecisionFromRequest(proposal uuid.UUID, body *openapiv1.PlanDecisionRequest) (service.PlanDecision, bool) {
	if body == nil || proposal == uuid.Nil || body.DecisionId == uuid.Nil || (body.TargetPlanId == nil) != (body.ExpectedRevisionId == nil) {
		return service.PlanDecision{}, false
	}
	d := service.PlanDecision{ProposalID: proposal, DecisionID: uuid.UUID(body.DecisionId), IdempotencyKey: string(body.IdempotencyKey),
		PresentedFingerprint: string(body.PresentedFingerprint)}
	if body.TargetPlanId != nil {
		d.TargetPlanID, d.ExpectedRevisionID = uuid.UUID(*body.TargetPlanId), uuid.UUID(*body.ExpectedRevisionId)
		if d.TargetPlanID == uuid.Nil || d.ExpectedRevisionID == uuid.Nil {
			return service.PlanDecision{}, false
		}
	}
	return d, true
}

func (s *PublicServer) AcceptPlanProposal(ctx context.Context, req openapiv1.AcceptPlanProposalRequestObject) (openapiv1.AcceptPlanProposalResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	decision, ok := planDecisionFromRequest(uuid.UUID(req.ProposalId), req.Body)
	if !ok {
		return planBadRequest, nil
	}
	result, err := s.trainingPlans.Accept(applicationRequestContext(ctx), owner, decision)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if result == nil {
		return planInconsistent, nil
	}
	r := result.Receipt
	if r.UserID != owner || r.ProposalID != decision.ProposalID || r.DecisionID != decision.DecisionID ||
		r.IdempotencyKey != decision.IdempotencyKey || r.Operation != "accept" || r.PlanID == nil || r.RevisionID == nil ||
		*r.PlanID != result.PlanID || *r.RevisionID != result.RevisionID {
		return planInconsistent, nil
	}
	wire := struct {
		ReceiptID      uuid.UUID    `json:"receiptId"`
		ProposalID     uuid.UUID    `json:"proposalId"`
		DecisionID     uuid.UUID    `json:"decisionId"`
		PlanID         uuid.UUID    `json:"planId"`
		RevisionID     uuid.UUID    `json:"revisionId"`
		IdempotencyKey string       `json:"idempotencyKey"`
		DecidedAt      time.Time    `json:"decidedAt"`
		ExecutionGate  planGateWire `json:"executionGate"`
	}{r.ID, r.ProposalID, r.DecisionID, result.PlanID, result.RevisionID, r.IdempotencyKey, r.CreatedAt, planGate(result.Gate)}
	out, err := strictOpenAPIConvert[openapiv1.PlanAcceptanceReceipt]("PlanAcceptanceReceipt", wire)
	if err != nil {
		return planInconsistent, nil
	}
	return openapiv1.AcceptPlanProposal200JSONResponse(out), nil
}

func (s *PublicServer) RejectPlanProposal(ctx context.Context, req openapiv1.RejectPlanProposalRequestObject) (openapiv1.RejectPlanProposalResponseObject, error) {
	owner, fail := s.trainingPlanOwner(ctx)
	if fail != nil {
		return *fail, nil
	}
	decision, ok := planDecisionFromRequest(uuid.UUID(req.ProposalId), req.Body)
	if !ok {
		return planBadRequest, nil
	}
	r, err := s.trainingPlans.Reject(applicationRequestContext(ctx), owner, decision)
	if err != nil {
		return trainingPlanCommandError(err), nil
	}
	if r == nil || r.UserID != owner || r.ProposalID != decision.ProposalID || r.DecisionID != decision.DecisionID ||
		r.IdempotencyKey != decision.IdempotencyKey || r.Operation != "reject" || r.PlanID != nil || r.RevisionID != nil {
		return planInconsistent, nil
	}
	return openapiv1.RejectPlanProposal200JSONResponse{ReceiptId: r.ID, ProposalId: r.ProposalID, DecisionId: r.DecisionID,
		IdempotencyKey: r.IdempotencyKey, DecidedAt: r.CreatedAt}, nil
}
