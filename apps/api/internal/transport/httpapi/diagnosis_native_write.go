package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type nativeDiagnosisDecisionApplication interface {
	Record(context.Context, uuid.UUID, service.DiagnosisProposalDecisionCommand) (*model.DiagnosisDecision, error)
}
type nativeDiagnosisSettlementApplication interface {
	Settle(context.Context, uuid.UUID, service.DiagnosisSettlementCommand) (*model.DiagnosisSettlement, error)
}

// WithNativeDiagnosisCommands wires the two Go-owned mutation boundaries.
// HTTP never accepts a caller-owned user ID, precomputed Safety verdict, or
// inferred diagnosis; the originating user must explicitly choose and settle.
func (s *PublicServer) WithNativeDiagnosisCommands(decisions nativeDiagnosisDecisionApplication, settlements nativeDiagnosisSettlementApplication) *PublicServer {
	s.nativeDiagnosisDecisions = decisions
	s.nativeDiagnosisSettlements = settlements
	return s
}

// Public decision acknowledgement is NOT a committed Diagnosis adoption.
type nativeDiagnosisDecisionWire struct {
	DecisionID          uuid.UUID                    `json:"decisionId"`
	ProposalID          uuid.UUID                    `json:"proposalId"`
	RunID               uuid.UUID                    `json:"runId"`
	ContentFingerprint  string                       `json:"contentFingerprint"`
	DecidedEvidenceRefs []model.DiagnosisEvidenceRef `json:"decidedEvidenceRefs"`
	Decision            model.DiagnosisDecisionKind  `json:"decision"`
	ExpectedRevision    int64                        `json:"expectedRevision"`
	ToolCallID          string                       `json:"toolCallId"`
	IdempotencyKey      string                       `json:"idempotencyKey"`
	ReceiptID           uuid.UUID                    `json:"receiptId"`
	DecidedAt           time.Time                    `json:"decidedAt"`
}
type nativeDiagnosisSettlementWire struct {
	SettlementID        uuid.UUID   `json:"settlementId"`
	RunID               uuid.UUID   `json:"runId"`
	ExpectedRevision    int64       `json:"expectedRevision"`
	Revision            int64       `json:"revision"`
	Outcome             string      `json:"outcome"`
	DiagnosisRevisionID *uuid.UUID  `json:"diagnosisRevisionId,omitempty"`
	ConditionIDs        []uuid.UUID `json:"conditionIds"`
	DecisionIDs         []uuid.UUID `json:"decisionIds"`
	BodyStateRevision   int64       `json:"bodyStateRevision"`
	IdempotencyKey      string      `json:"idempotencyKey"`
	ToolCallID          string      `json:"toolCallId"`
	ReceiptID           uuid.UUID   `json:"receiptId"`
	SettledAt           time.Time   `json:"settledAt"`
}

func (s *PublicServer) DecideNativeDiagnosisProposal(ctx context.Context, req openapiv1.DecideNativeDiagnosisProposalRequestObject) (openapiv1.DecideNativeDiagnosisProposalResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: 401, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.RunId == uuid.Nil || req.Body.ProposalId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: 400, code: "INVALID_REQUEST", message: "explicit choice and run required"}, nil
	}
	if s.nativeDiagnosisDecisions == nil {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis decision unavailable"}, nil
	}
	cmd := service.DiagnosisProposalDecisionCommand{
		RunID: uuid.UUID(req.RunId), ProposalID: uuid.UUID(req.Body.ProposalId),
		PresentedFingerprint: req.Body.PresentedFingerprint, ExpectedRevision: req.Body.ExpectedRevision,
		Decision: model.DiagnosisDecisionKind(req.Body.Decision), ToolCallID: req.Body.ToolCallId,
		IdempotencyKey: req.Body.IdempotencyKey,
	}
	receipt, err := s.nativeDiagnosisDecisions.Record(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return nativeDiagnosisCommandError(err), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.ReceiptID == uuid.Nil ||
		receipt.ProposalID != cmd.ProposalID || receipt.RunID != cmd.RunID || receipt.ContentFingerprint != cmd.PresentedFingerprint ||
		receipt.Decision != cmd.Decision || receipt.ExpectedRevision != cmd.ExpectedRevision || receipt.ToolCallID != cmd.ToolCallID ||
		receipt.IdempotencyKey != cmd.IdempotencyKey || receipt.DecidedAt.IsZero() {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "decision receipt inconsistent"}, nil
	}
	refs := receipt.DecidedEvidenceRefs
	if refs == nil {
		refs = []model.DiagnosisEvidenceRef{}
	}
	wire := nativeDiagnosisDecisionWire{
		DecisionID: receipt.ID, ProposalID: receipt.ProposalID, RunID: receipt.RunID,
		ContentFingerprint: receipt.ContentFingerprint, DecidedEvidenceRefs: refs,
		Decision: receipt.Decision, ExpectedRevision: receipt.ExpectedRevision,
		ToolCallID: receipt.ToolCallID, IdempotencyKey: receipt.IdempotencyKey,
		ReceiptID: receipt.ReceiptID, DecidedAt: receipt.DecidedAt,
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisDecisionReceipt]("NativeDiagnosisDecisionReceipt", wire)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "decision receipt violates contract"}, nil
	}
	return openapiv1.DecideNativeDiagnosisProposal200JSONResponse(out), nil
}

func (s *PublicServer) SettleNativeDiagnosisRun(ctx context.Context, req openapiv1.SettleNativeDiagnosisRunRequestObject) (openapiv1.SettleNativeDiagnosisRunResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: 401, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.RunId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: 400, code: "INVALID_REQUEST", message: "explicit settlement and run required"}, nil
	}
	if s.nativeDiagnosisSettlements == nil {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis settlement unavailable"}, nil
	}
	cmd := service.DiagnosisSettlementCommand{
		RunID: uuid.UUID(req.RunId), ExpectedRevision: req.Body.ExpectedRevision,
		ToolCallID: req.Body.ToolCallId, IdempotencyKey: req.Body.IdempotencyKey,
		CloseRemaining: req.Body.CloseRemaining,
	}
	receipt, err := s.nativeDiagnosisSettlements.Settle(applicationRequestContext(ctx), owner, cmd)
	// The Go settlement service persists a safety-blocked terminal receipt
	// and returns it *along with* ErrDiagnosisSafetyBlocked. Preserve that
	// terminal result (never mark it committed); do not discard a real receipt.
	if err != nil && !(errors.Is(err, service.ErrDiagnosisSafetyBlocked) && receipt != nil && receipt.Outcome == "safety_blocked") {
		return nativeDiagnosisCommandError(err), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.ReceiptID == uuid.Nil ||
		receipt.RunID != cmd.RunID || receipt.ExpectedRevision != cmd.ExpectedRevision ||
		receipt.ToolCallID != cmd.ToolCallID || receipt.IdempotencyKey != cmd.IdempotencyKey ||
		receipt.SettledAt.IsZero() || receipt.BodyStateRevision < 0 {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "settlement receipt inconsistent"}, nil
	}
	switch receipt.Outcome {
	case "committed":
		if receipt.DiagnosisRevisionID == nil || *receipt.DiagnosisRevisionID == uuid.Nil || receipt.Revision != cmd.ExpectedRevision+1 {
			return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "committed settlement revision inconsistent"}, nil
		}
	case "closed_without_change", "safety_blocked":
		if receipt.DiagnosisRevisionID != nil || receipt.Revision != cmd.ExpectedRevision {
			return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "nonadoption settlement revision inconsistent"}, nil
		}
	default:
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "unknown settlement outcome"}, nil
	}
	conditions, decisions := receipt.ConditionIDs, receipt.DecisionIDs
	if conditions == nil {
		conditions = []uuid.UUID{}
	}
	if decisions == nil {
		decisions = []uuid.UUID{}
	}
	wire := nativeDiagnosisSettlementWire{
		SettlementID: receipt.ID, RunID: receipt.RunID, ExpectedRevision: receipt.ExpectedRevision,
		Revision: receipt.Revision, Outcome: receipt.Outcome, DiagnosisRevisionID: receipt.DiagnosisRevisionID,
		ConditionIDs: conditions, DecisionIDs: decisions, BodyStateRevision: receipt.BodyStateRevision,
		IdempotencyKey: receipt.IdempotencyKey, ToolCallID: receipt.ToolCallID,
		ReceiptID: receipt.ReceiptID, SettledAt: receipt.SettledAt,
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisSettlementReceipt]("NativeDiagnosisSettlementReceipt", wire)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "settlement receipt violates contract"}, nil
	}
	return openapiv1.SettleNativeDiagnosisRun200JSONResponse(out), nil
}

func nativeDiagnosisCommandError(err error) diagnosisHTTPErrorResponse {
	switch {
	case errors.Is(err, model.ErrDiagnosisNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return diagnosisHTTPErrorResponse{status: 404, code: "NOT_FOUND", message: "diagnosis run or proposal not found"}
	case errors.Is(err, service.ErrDiagnosisDecisionInvalid), errors.Is(err, service.ErrDiagnosisSettlementInvalid),
		errors.Is(err, service.ErrDiagnosisDirectEditInvalid):
		return diagnosisHTTPErrorResponse{status: 400, code: "INVALID_REQUEST", message: "invalid explicit diagnosis command"}
	case errors.Is(err, service.ErrDiagnosisDecisionConflict), errors.Is(err, service.ErrDiagnosisAnalysisClosed),
		errors.Is(err, service.ErrDiagnosisChoicesIncomplete), errors.Is(err, service.ErrDiagnosisRecheckRequired),
		errors.Is(err, service.ErrDiagnosisEvidenceUnavailable), errors.Is(err, service.ErrDiagnosisMedicalIdentityChanged):
		return diagnosisHTTPErrorResponse{status: 409, code: "REVISION_CONFLICT", message: "diagnosis proposal or evidence changed; refresh and reconfirm"}
	case errors.Is(err, service.ErrDiagnosisSafetyBlocked):
		return diagnosisHTTPErrorResponse{status: 409, code: "SAFETY_BLOCKED", message: "formal analysis stopped by urgent safety restriction"}
	case errors.Is(err, service.ErrDiagnosisSafetyUnavailable):
		return diagnosisHTTPErrorResponse{status: 503, code: "SAFETY_UNAVAILABLE", message: "Go-owned safety validation unavailable"}
	default:
		return diagnosisHTTPErrorResponse{status: 503, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis command unavailable"}
	}
}

func (r diagnosisHTTPErrorResponse) VisitDecideNativeDiagnosisProposalResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitSettleNativeDiagnosisRunResponse(w http.ResponseWriter) error {
	return r.write(w)
}
