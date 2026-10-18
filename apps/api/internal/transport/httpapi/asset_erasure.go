package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// assetErasureApplication is the AS-002 per-asset erasure boundary.
type assetErasureApplication interface {
	Request(ctx context.Context, userID, assetID uuid.UUID, idempotencyKey string) (*model.AssetErasureReceipt, error)
	Receipt(ctx context.Context, userID, assetID uuid.UUID) (*model.AssetErasureReceipt, error)
}

func (s *PublicServer) WithAssetErasure(erasure assetErasureApplication) *PublicServer {
	s.assetErasure = erasure
	return s
}

type assetErasureHTTPError struct {
	status  int
	code    string
	message string
}

func (r assetErasureHTTPError) write(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	return json.NewEncoder(w).Encode(errorEnvelope(r.code, r.message))
}

func (r assetErasureHTTPError) VisitRequestAssetErasureResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r assetErasureHTTPError) VisitGetAssetErasureResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func assetErasureError(err error) assetErasureHTTPError {
	switch {
	case errors.Is(err, model.ErrAssetErasureNotFound):
		return assetErasureHTTPError{http.StatusNotFound, "NOT_FOUND", "asset erasure not found"}
	case errors.Is(err, model.ErrAssetErasureIdempotencyUsed):
		return assetErasureHTTPError{http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotency key was used for another asset"}
	default:
		return assetErasureHTTPError{http.StatusInternalServerError, "INTERNAL_ERROR", "asset erasure failed"}
	}
}

var assetErasureUnavailable = assetErasureHTTPError{http.StatusServiceUnavailable, "ASSET_ERASURE_UNAVAILABLE", "asset erasure is not configured"}

// RequestAssetErasure admits (or repeats) the erasure of one owned asset.
// 200 only when every target verified; otherwise 202 with what is outstanding.
func (s *PublicServer) RequestAssetErasure(ctx context.Context, request openapiv1.RequestAssetErasureRequestObject) (openapiv1.RequestAssetErasureResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return assetErasureHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}, nil
	}
	if s.assetErasure == nil {
		return assetErasureUnavailable, nil
	}
	if request.Body == nil {
		return assetErasureHTTPError{http.StatusBadRequest, "INVALID_REQUEST", "request body is required"}, nil
	}
	receipt, err := s.assetErasure.Request(applicationRequestContext(ctx), uid, request.Id, request.Body.IdempotencyKey)
	if err != nil {
		return assetErasureError(err), nil
	}
	view := assetErasureReceiptView(receipt)
	if receipt.Request.Status == model.AssetErasureErased {
		return openapiv1.RequestAssetErasure200JSONResponse(view), nil
	}
	return openapiv1.RequestAssetErasure202JSONResponse(view), nil
}

// GetAssetErasure reads the owner's receipt; foreign, missing and
// never-requested erasures are the same owner-masked 404.
func (s *PublicServer) GetAssetErasure(ctx context.Context, request openapiv1.GetAssetErasureRequestObject) (openapiv1.GetAssetErasureResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return assetErasureHTTPError{http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"}, nil
	}
	if s.assetErasure == nil {
		return assetErasureUnavailable, nil
	}
	receipt, err := s.assetErasure.Receipt(applicationRequestContext(ctx), uid, request.Id)
	if err != nil {
		return assetErasureError(err), nil
	}
	return openapiv1.GetAssetErasure200JSONResponse(assetErasureReceiptView(receipt)), nil
}

func assetErasureReceiptView(receipt *model.AssetErasureReceipt) openapiv1.AssetErasureReceipt {
	view := openapiv1.AssetErasureReceipt{
		RequestId:          receipt.Request.ID,
		AssetId:            receipt.Request.AssetID,
		Status:             openapiv1.AssetErasureReceiptStatus(receipt.Request.Status),
		InventoryRevision:  receipt.Request.InventoryRevision,
		RequestedAt:        receipt.Request.RequestedAt,
		ErasedAt:           receipt.Request.ErasedAt,
		Targets:            make([]openapiv1.AssetErasureTargetReceipt, 0, len(receipt.Targets)),
		OutstandingTargets: []openapiv1.AssetErasureTargetName{},
		Reviews:            erasureReviewViews(receipt.Reviews),
	}
	for _, target := range receipt.Targets {
		item := openapiv1.AssetErasureTargetReceipt{
			Target:           openapiv1.AssetErasureTargetName(target.Target),
			State:            openapiv1.AssetErasureTargetReceiptState(target.State),
			ErasedCount:      target.ErasedCount,
			OutstandingCount: target.OutstandingCount,
			VerifiedAt:       target.VerifiedAt,
		}
		if target.FailureCode != nil {
			code := openapiv1.AssetErasureTargetReceiptFailureCode(*target.FailureCode)
			item.FailureCode = &code
		}
		view.Targets = append(view.Targets, item)
		if target.State != model.AssetErasureTargetVerified {
			view.OutstandingTargets = append(view.OutstandingTargets, item.Target)
		}
	}
	return view
}

func erasureReviewViews(reviews []model.ErasureReview) []openapiv1.ErasureReview {
	out := make([]openapiv1.ErasureReview, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, openapiv1.ErasureReview{
			Subject:    openapiv1.ErasureReviewSubject(review.Subject),
			SubjectId:  review.SubjectID,
			RevisionId: review.RevisionID,
		})
	}
	return out
}
