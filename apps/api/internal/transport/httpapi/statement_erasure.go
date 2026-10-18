package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// statementErasureApplication is the AS-002 per-statement erasure boundary.
type statementErasureApplication interface {
	Request(ctx context.Context, userID, statementID uuid.UUID, idempotencyKey string) (*model.StatementErasureReceipt, error)
	Receipt(ctx context.Context, userID, statementID uuid.UUID) (*model.StatementErasureReceipt, error)
}

func (s *PublicServer) WithStatementErasure(erasure statementErasureApplication) *PublicServer {
	s.statementErasure = erasure
	return s
}

func (r assetErasureHTTPError) VisitRequestStatementErasureResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r assetErasureHTTPError) VisitGetStatementErasureResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func statementErasureError(err error) assetErasureHTTPError {
	switch {
	case errors.Is(err, model.ErrStatementErasureNotFound):
		return assetErasureHTTPError{http.StatusNotFound, "NOT_FOUND", "statement erasure not found"}
	case errors.Is(err, model.ErrStatementErasureIdempotencyUsed):
		return assetErasureHTTPError{http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotency key was used for another statement"}
	default:
		return assetErasureHTTPError{http.StatusInternalServerError, "INTERNAL_ERROR", "statement erasure failed"}
	}
}

var statementErasureUnavailable = assetErasureHTTPError{http.StatusServiceUnavailable, "STATEMENT_ERASURE_UNAVAILABLE", "statement erasure is not configured"}

// RequestStatementErasure admits (or repeats) the erasure of one owned
// statement. 200 once terminal (erased or erased_with_retention); otherwise
// 202 with what is outstanding and what needs review.
func (s *PublicServer) RequestStatementErasure(ctx context.Context, request openapiv1.RequestStatementErasureRequestObject) (openapiv1.RequestStatementErasureResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return assetErasureHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}, nil
	}
	if s.statementErasure == nil {
		return statementErasureUnavailable, nil
	}
	if request.Body == nil {
		return assetErasureHTTPError{http.StatusBadRequest, "INVALID_REQUEST", "request body is required"}, nil
	}
	receipt, err := s.statementErasure.Request(applicationRequestContext(ctx), uid, request.Id, request.Body.IdempotencyKey)
	if err != nil {
		return statementErasureError(err), nil
	}
	view := statementErasureReceiptView(receipt)
	if receipt.Request.Status.Terminal() {
		return openapiv1.RequestStatementErasure200JSONResponse(view), nil
	}
	return openapiv1.RequestStatementErasure202JSONResponse(view), nil
}

// GetStatementErasure reads the owner's receipt; foreign, missing and
// never-requested erasures are the same owner-masked 404.
func (s *PublicServer) GetStatementErasure(ctx context.Context, request openapiv1.GetStatementErasureRequestObject) (openapiv1.GetStatementErasureResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return assetErasureHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}, nil
	}
	if s.statementErasure == nil {
		return statementErasureUnavailable, nil
	}
	receipt, err := s.statementErasure.Receipt(applicationRequestContext(ctx), uid, request.Id)
	if err != nil {
		return statementErasureError(err), nil
	}
	return openapiv1.GetStatementErasure200JSONResponse(statementErasureReceiptView(receipt)), nil
}

func statementErasureReceiptView(receipt *model.StatementErasureReceipt) openapiv1.StatementErasureReceipt {
	view := openapiv1.StatementErasureReceipt{
		RequestId:          receipt.Request.ID,
		StatementId:        receipt.Request.StatementID,
		Status:             openapiv1.StatementErasureReceiptStatus(receipt.Request.Status),
		InventoryRevision:  receipt.Request.InventoryRevision,
		RequestedAt:        receipt.Request.RequestedAt,
		ErasedAt:           receipt.Request.ErasedAt,
		Targets:            make([]openapiv1.StatementErasureTargetReceipt, 0, len(receipt.Targets)),
		OutstandingTargets: []openapiv1.StatementErasureTargetName{},
		RetainedTargets:    []openapiv1.StatementErasureTargetName{},
		Reviews:            erasureReviewViews(receipt.Reviews),
	}
	for _, target := range receipt.Targets {
		item := openapiv1.StatementErasureTargetReceipt{
			Target:           openapiv1.StatementErasureTargetName(target.Target),
			State:            openapiv1.StatementErasureTargetReceiptState(target.State),
			ErasedCount:      target.ErasedCount,
			OutstandingCount: target.OutstandingCount,
			VerifiedAt:       target.VerifiedAt,
		}
		if target.FailureCode != nil {
			code := openapiv1.StatementErasureTargetReceiptFailureCode(*target.FailureCode)
			item.FailureCode = &code
		}
		if target.RetentionReason != nil {
			reason := openapiv1.StatementErasureTargetReceiptRetentionReason(*target.RetentionReason)
			item.RetentionReason = &reason
		}
		view.Targets = append(view.Targets, item)
		switch target.State {
		case model.AssetErasureTargetVerified:
		case model.ErasureTargetRetainedByPolicy:
			view.RetainedTargets = append(view.RetainedTargets, item.Target)
		default:
			view.OutstandingTargets = append(view.OutstandingTargets, item.Target)
		}
	}
	return view
}
