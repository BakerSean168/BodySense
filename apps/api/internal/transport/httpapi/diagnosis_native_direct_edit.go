package httpapi

import (
	"context"
	"net/http"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
)

type nativeDiagnosisDirectEditApplication interface {
	Apply(context.Context, uuid.UUID, service.DiagnosisDirectEditCommand) (*model.ConditionDirectEdit, error)
}

// WithNativeDiagnosisDirectEdits wires the user's confirmed direct edit of one
// committed Condition. No Agent tool reaches this boundary.
func (s *PublicServer) WithNativeDiagnosisDirectEdits(edits nativeDiagnosisDirectEditApplication) *PublicServer {
	s.nativeDiagnosisDirectEdits = edits
	return s
}

type nativeDiagnosisDirectEditWire struct {
	EditID              uuid.UUID                         `json:"editId"`
	ConditionID         uuid.UUID                         `json:"conditionId"`
	EditKind            model.ConditionDirectEditKind     `json:"editKind"`
	Changes             []model.ConditionDirectEditChange `json:"changes"`
	Reason              string                            `json:"reason"`
	RequestEvidenceID   uuid.UUID                         `json:"requestEvidenceId"`
	RequestMessageID    uuid.UUID                         `json:"requestMessageId"`
	ExpectedRevision    int64                             `json:"expectedRevision"`
	Revision            int64                             `json:"revision"`
	DiagnosisRevisionID uuid.UUID                         `json:"diagnosisRevisionId"`
	BodyStateRevision   int64                             `json:"bodyStateRevision"`
	ToolCallID          string                            `json:"toolCallId"`
	IdempotencyKey      string                            `json:"idempotencyKey"`
	ReceiptID           uuid.UUID                         `json:"receiptId"`
	ConfirmedAt         time.Time                         `json:"confirmedAt"`
}

func (s *PublicServer) DirectEditNativeDiagnosisCondition(ctx context.Context, req openapiv1.DirectEditNativeDiagnosisConditionRequestObject) (openapiv1.DirectEditNativeDiagnosisConditionResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.ConditionId == uuid.Nil || req.Body.RequestEvidenceId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "explicit condition and request statement required"}, nil
	}
	if s.nativeDiagnosisDirectEdits == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "diagnosis direct edit unavailable"}, nil
	}
	cmd := service.DiagnosisDirectEditCommand{
		ConditionID: uuid.UUID(req.ConditionId), EditKind: model.ConditionDirectEditKind(req.Body.EditKind),
		RequestEvidenceID: uuid.UUID(req.Body.RequestEvidenceId), ExpectedRevision: req.Body.ExpectedRevision,
		Reason: req.Body.Reason, ToolCallID: req.Body.ToolCallId, IdempotencyKey: req.Body.IdempotencyKey,
	}
	if req.Body.Metadata != nil {
		notes := req.Body.Metadata.Notes
		cmd.Notes = &notes
	}
	receipt, err := s.nativeDiagnosisDirectEdits.Apply(applicationRequestContext(ctx), owner, cmd)
	if err != nil {
		return nativeDiagnosisCommandError(err), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.ReceiptID == uuid.Nil || receipt.DiagnosisRevisionID == uuid.Nil ||
		receipt.ConditionID != cmd.ConditionID || receipt.EditKind != cmd.EditKind || receipt.RequestEvidenceID != cmd.RequestEvidenceID ||
		receipt.BasedOnDiagnosisRevision != cmd.ExpectedRevision || receipt.ResultingDiagnosisRevision != cmd.ExpectedRevision+1 ||
		receipt.Reason != cmd.Reason || receipt.ConfirmationToolCallID != cmd.ToolCallID || receipt.IdempotencyKey != cmd.IdempotencyKey ||
		len(receipt.Changes) == 0 || receipt.ConfirmedAt.IsZero() {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "direct edit receipt inconsistent"}, nil
	}
	wire := nativeDiagnosisDirectEditWire{
		EditID: receipt.ID, ConditionID: receipt.ConditionID, EditKind: receipt.EditKind, Changes: receipt.Changes,
		Reason: receipt.Reason, RequestEvidenceID: receipt.RequestEvidenceID, RequestMessageID: receipt.RequestMessageID,
		ExpectedRevision: receipt.BasedOnDiagnosisRevision, Revision: receipt.ResultingDiagnosisRevision,
		DiagnosisRevisionID: receipt.DiagnosisRevisionID, BodyStateRevision: receipt.BodyStateRevision,
		ToolCallID: receipt.ConfirmationToolCallID, IdempotencyKey: receipt.IdempotencyKey,
		ReceiptID: receipt.ReceiptID, ConfirmedAt: receipt.ConfirmedAt,
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisDirectEditReceipt]("NativeDiagnosisDirectEditReceipt", wire)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "direct edit receipt violates contract"}, nil
	}
	return openapiv1.DirectEditNativeDiagnosisCondition200JSONResponse(out), nil
}

func (r diagnosisHTTPErrorResponse) VisitDirectEditNativeDiagnosisConditionResponse(w http.ResponseWriter) error {
	return r.write(w)
}
