package httpapi

import (
	"context"
	"encoding/json"
	"github.com/bodysense/api/internal/domain/trainingplan"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"time"
)

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
