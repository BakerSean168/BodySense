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
)

type nativeDiagnosisReviewPromptApplication interface {
	Dismiss(context.Context, uuid.UUID, service.DiagnosisReviewPromptDismissCommand) (*model.DiagnosisReviewPromptDismissal, error)
}

type nativeDiagnosisAnalysisRequestApplication interface {
	Bind(context.Context, uuid.UUID, uuid.UUID) (*model.StatementEvidence, bool, error)
}

// WithNativeDiagnosisReview wires the two user-only helpers around a formal
// analysis: binding the user's own chat message as the request statement, and
// the §3.6 "not now" on a derived review prompt. Neither adopts or edits a
// Condition, and no Agent tool reaches them.
func (s *PublicServer) WithNativeDiagnosisReview(requests nativeDiagnosisAnalysisRequestApplication, prompts nativeDiagnosisReviewPromptApplication) *PublicServer {
	s.nativeDiagnosisRequests = requests
	s.nativeDiagnosisReviewPrompts = prompts
	return s
}

type nativeDiagnosisAnalysisRequestWire struct {
	RequestEvidenceID uuid.UUID `json:"requestEvidenceId"`
	MessageID         uuid.UUID `json:"messageId"`
	ConversationID    uuid.UUID `json:"conversationId"`
	SourceVersion     int64     `json:"sourceVersion"`
	Excerpt           string    `json:"excerpt"`
	RecordedAt        time.Time `json:"recordedAt"`
}

func (s *PublicServer) CreateNativeDiagnosisAnalysisRequest(ctx context.Context, req openapiv1.CreateNativeDiagnosisAnalysisRequestRequestObject) (openapiv1.CreateNativeDiagnosisAnalysisRequestResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.Body.MessageId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "explicit request message required"}, nil
	}
	if s.nativeDiagnosisRequests == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal analysis request unavailable"}, nil
	}
	messageID := uuid.UUID(req.Body.MessageId)
	evidence, replayed, err := s.nativeDiagnosisRequests.Bind(applicationRequestContext(ctx), owner, messageID)
	switch {
	case errors.Is(err, model.ErrDiagnosisRequestSourceNotFound):
		return diagnosisHTTPErrorResponse{status: http.StatusNotFound, code: "NOT_FOUND", message: "request message not found"}, nil
	case errors.Is(err, model.ErrDiagnosisRequestSourceInvalid):
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "message cannot be a formal analysis request"}, nil
	case err != nil:
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "formal analysis request unavailable"}, nil
	}
	if evidence == nil || evidence.UserID != owner || evidence.ID == uuid.Nil || evidence.SourceKind != "user_message" ||
		evidence.Availability != "available" || evidence.SourceMessageID == nil || *evidence.SourceMessageID != messageID ||
		evidence.SourceConversationID == nil || evidence.SourceVersion < 1 || evidence.Excerpt == "" || evidence.RecordedAt.IsZero() {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "request statement inconsistent"}, nil
	}
	wire := nativeDiagnosisAnalysisRequestWire{
		RequestEvidenceID: evidence.ID, MessageID: messageID, ConversationID: *evidence.SourceConversationID,
		SourceVersion: evidence.SourceVersion, Excerpt: evidence.Excerpt, RecordedAt: evidence.RecordedAt,
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisAnalysisRequest]("NativeDiagnosisAnalysisRequest", wire)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "request statement violates contract"}, nil
	}
	if replayed {
		return openapiv1.CreateNativeDiagnosisAnalysisRequest200JSONResponse(out), nil
	}
	return openapiv1.CreateNativeDiagnosisAnalysisRequest201JSONResponse(out), nil
}

type nativeDiagnosisReviewPromptWire struct {
	DismissalID                       uuid.UUID `json:"dismissalId"`
	ConditionID                       uuid.UUID `json:"conditionId"`
	BasedOnBodyStateRevision          int64     `json:"basedOnBodyStateRevision"`
	DismissedThroughBodyStateRevision int64     `json:"dismissedThroughBodyStateRevision"`
	DiagnosisRevision                 int64     `json:"diagnosisRevision"`
	ReasonCodes                       []string  `json:"reasonCodes"`
	IdempotencyKey                    string    `json:"idempotencyKey"`
	DismissedAt                       time.Time `json:"dismissedAt"`
}

func (s *PublicServer) DismissNativeDiagnosisReviewPrompt(ctx context.Context, req openapiv1.DismissNativeDiagnosisReviewPromptRequestObject) (openapiv1.DismissNativeDiagnosisReviewPromptResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "authentication required"}, nil
	}
	if req.Body == nil || req.ConditionId == uuid.Nil {
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "explicit condition required"}, nil
	}
	if s.nativeDiagnosisReviewPrompts == nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_UNAVAILABLE", message: "review prompt unavailable"}, nil
	}
	cmd := service.DiagnosisReviewPromptDismissCommand{
		ConditionID: uuid.UUID(req.ConditionId), ExpectedRevision: req.Body.ExpectedRevision,
		EvaluatedBodyStateRevision: req.Body.EvaluatedBodyStateRevision, IdempotencyKey: req.Body.IdempotencyKey,
	}
	receipt, err := s.nativeDiagnosisReviewPrompts.Dismiss(applicationRequestContext(ctx), owner, cmd)
	switch {
	case errors.Is(err, service.ErrDiagnosisReviewPromptInvalid):
		return diagnosisHTTPErrorResponse{status: http.StatusBadRequest, code: "INVALID_REQUEST", message: "invalid review prompt dismissal"}, nil
	case errors.Is(err, service.ErrDiagnosisReviewPromptNotShown):
		return diagnosisHTTPErrorResponse{status: http.StatusConflict, code: "REVIEW_PROMPT_NOT_SHOWN", message: "this condition has no review prompt to dismiss"}, nil
	case errors.Is(err, service.ErrDiagnosisReviewPromptKeyReused):
		return diagnosisHTTPErrorResponse{status: http.StatusConflict, code: "IDEMPOTENCY_KEY_REUSED", message: "idempotency key already used for another dismissal"}, nil
	case err != nil:
		return nativeDiagnosisCommandError(err), nil
	}
	if receipt == nil || receipt.UserID != owner || receipt.ID == uuid.Nil || receipt.ConditionID != cmd.ConditionID ||
		receipt.DiagnosisRevision != cmd.ExpectedRevision || receipt.DismissedThroughBodyStateRevision != cmd.EvaluatedBodyStateRevision ||
		receipt.IdempotencyKey != cmd.IdempotencyKey || len(receipt.ReasonCodes) == 0 || receipt.DismissedAt.IsZero() {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "dismissal receipt inconsistent"}, nil
	}
	wire := nativeDiagnosisReviewPromptWire{
		DismissalID: receipt.ID, ConditionID: receipt.ConditionID, BasedOnBodyStateRevision: receipt.BasedOnBodyStateRevision,
		DismissedThroughBodyStateRevision: receipt.DismissedThroughBodyStateRevision, DiagnosisRevision: receipt.DiagnosisRevision,
		ReasonCodes: receipt.ReasonCodes, IdempotencyKey: receipt.IdempotencyKey, DismissedAt: receipt.DismissedAt,
	}
	out, err := strictOpenAPIConvert[openapiv1.NativeDiagnosisReviewPromptDismissal]("NativeDiagnosisReviewPromptDismissal", wire)
	if err != nil {
		return diagnosisHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "DIAGNOSIS_RECEIPT_UNAVAILABLE", message: "dismissal receipt violates contract"}, nil
	}
	return openapiv1.DismissNativeDiagnosisReviewPrompt200JSONResponse(out), nil
}

func (r diagnosisHTTPErrorResponse) VisitCreateNativeDiagnosisAnalysisRequestResponse(w http.ResponseWriter) error {
	return r.write(w)
}
func (r diagnosisHTTPErrorResponse) VisitDismissNativeDiagnosisReviewPromptResponse(w http.ResponseWriter) error {
	return r.write(w)
}
