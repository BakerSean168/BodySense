package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
)

// removeSourceResponse projects the surviving asset state after the
// source-removal command. Repeating the command returns the same shape.
func removeSourceResponse(asset *model.UserAsset) (openapiv1.AssetSourceRemovalResponse, error) {
	if asset == nil {
		return openapiv1.AssetSourceRemovalResponse{}, errors.New("asset is nil")
	}
	response := openapiv1.AssetSourceRemovalResponse{
		Availability: openapiv1.AssetSourceRemovalResponseAvailabilityErased,
		Message:      "asset source removed; adopted evidence history retained",
	}
	if asset.Availability != model.UserAssetErased {
		response.Availability = openapiv1.AssetSourceRemovalResponseAvailabilitySourceRemoved
		response.Message = "asset source removed; adopted evidence history retained"
	}
	response.SourceRemovedAt = asset.SourceRemovedAt
	return response, nil
}

// DeleteUpload is the legacy upload-surface alias of the source-removal
// command (PC-D7). It removes the private original and marks the backing
// asset source_removed; it is idempotent and never deletes evidence.
func (s *PublicServer) DeleteUpload(
	ctx context.Context,
	request openapiv1.DeleteUploadRequestObject,
) (openapiv1.DeleteUploadResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	asset, err := s.uploads.RemoveSource(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to remove upload source"), nil
	}
	response, err := removeSourceResponse(asset)
	if err != nil {
		return nil, err
	}
	return openapiv1.DeleteUpload200JSONResponse(response), nil
}

// DeleteAsset executes the source-removal command on the sole asset
// identity: DELETE /api/v1/assets/{id}. Owner-masked 404 for missing and
// foreign assets; idempotent for already-removed sources.
func (s *PublicServer) DeleteAsset(
	ctx context.Context,
	request openapiv1.DeleteAssetRequestObject,
) (openapiv1.DeleteAssetResponseObject, error) {
	uid, err := authenticatedUserID(ctx)
	if err != nil {
		return uploadUnauthorized(), nil
	}
	if s.uploads == nil {
		return uploadHTTPErrorResponse{status: http.StatusServiceUnavailable, code: "UPLOAD_DOMAIN_UNAVAILABLE", message: "upload service is not configured"}, nil
	}
	asset, err := s.uploads.RemoveSource(ctx, uid, request.Id)
	if err != nil {
		return ownedUploadError(err, "failed to remove asset source"), nil
	}
	response, err := removeSourceResponse(asset)
	if err != nil {
		return nil, err
	}
	return openapiv1.DeleteAsset200JSONResponse(response), nil
}
