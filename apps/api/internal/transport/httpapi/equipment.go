package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// equipmentApplication is the owner-scoped EquipmentAsset boundary (PC-D2).
// Only an authenticated direct user edit writes it; owner and revision are
// server-owned. Equipment is not BodyState and has no AI write path.
type equipmentApplication interface {
	Create(context.Context, uuid.UUID, model.CreateEquipmentAssetCommand) (*model.EquipmentAsset, error)
	Update(context.Context, uuid.UUID, uuid.UUID, model.UpdateEquipmentAssetCommand) (*model.EquipmentAsset, error)
	List(context.Context, uuid.UUID) ([]model.EquipmentAsset, error)
}

func (s *PublicServer) WithEquipment(app equipmentApplication) *PublicServer {
	s.equipment = app
	return s
}

func (s *PublicServer) ListEquipmentAssets(ctx context.Context, _ openapiv1.ListEquipmentAssetsRequestObject) (openapiv1.ListEquipmentAssetsResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return openapiv1.ListEquipmentAssets401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(errorEnvelope("UNAUTHORIZED", "authentication required"))}, nil
	}
	internal := openapiv1.ListEquipmentAssets500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(errorEnvelope("INTERNAL_ERROR", "failed to load equipment"))}
	if s.equipment == nil {
		return internal, nil
	}
	items, err := s.equipment.List(applicationRequestContext(ctx), owner)
	if err != nil {
		return internal, nil
	}
	for index := range items {
		if items[index].UserID != owner {
			return internal, nil
		}
	}
	converted, err := strictOpenAPIConvert[openapiv1.EquipmentAssetList]("EquipmentAssetList", map[string]any{"items": items})
	if err != nil {
		return internal, nil
	}
	return openapiv1.ListEquipmentAssets200JSONResponse(converted), nil
}

func (s *PublicServer) CreateEquipmentAsset(ctx context.Context, req openapiv1.CreateEquipmentAssetRequestObject) (openapiv1.CreateEquipmentAssetResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return createEquipmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || s.equipment == nil {
		return createEquipmentError(http.StatusBadRequest, "INVALID_EQUIPMENT", "equipment request required"), nil
	}
	command, err := strictJSONConvert[model.CreateEquipmentAssetCommand](req.Body)
	if err != nil {
		return createEquipmentError(http.StatusBadRequest, "INVALID_EQUIPMENT", "invalid equipment request"), nil
	}
	asset, err := s.equipment.Create(applicationRequestContext(ctx), owner, command)
	if err != nil {
		status, code, message := classifyEquipmentError(err)
		return createEquipmentError(status, code, message), nil
	}
	converted, ok := equipmentAssetResponse(asset, owner, nil)
	if !ok {
		return createEquipmentError(http.StatusInternalServerError, "INVALID_EQUIPMENT_COMMIT", "equipment was not confirmed"), nil
	}
	return openapiv1.CreateEquipmentAsset200JSONResponse(converted), nil
}

func (s *PublicServer) UpdateEquipmentAsset(ctx context.Context, req openapiv1.UpdateEquipmentAssetRequestObject) (openapiv1.UpdateEquipmentAssetResponseObject, error) {
	owner, err := authenticatedUserID(ctx)
	if err != nil || owner == uuid.Nil {
		return updateEquipmentError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if req.Body == nil || s.equipment == nil {
		return updateEquipmentError(http.StatusBadRequest, "INVALID_EQUIPMENT", "equipment request required"), nil
	}
	command, err := strictJSONConvert[model.UpdateEquipmentAssetCommand](req.Body)
	if err != nil {
		return updateEquipmentError(http.StatusBadRequest, "INVALID_EQUIPMENT", "invalid equipment request"), nil
	}
	id := uuid.UUID(req.Id)
	asset, err := s.equipment.Update(applicationRequestContext(ctx), owner, id, command)
	if err != nil {
		status, code, message := classifyEquipmentError(err)
		return updateEquipmentError(status, code, message), nil
	}
	converted, ok := equipmentAssetResponse(asset, owner, &id)
	if !ok {
		return updateEquipmentError(http.StatusInternalServerError, "INVALID_EQUIPMENT_COMMIT", "equipment update was not confirmed"), nil
	}
	return openapiv1.UpdateEquipmentAsset200JSONResponse(converted), nil
}

func equipmentAssetResponse(asset *model.EquipmentAsset, owner uuid.UUID, id *uuid.UUID) (openapiv1.EquipmentAsset, bool) {
	if asset == nil || asset.ID == uuid.Nil || asset.UserID != owner || asset.Revision < 1 || (id != nil && asset.ID != *id) {
		return openapiv1.EquipmentAsset{}, false
	}
	converted, err := strictOpenAPIConvert[openapiv1.EquipmentAsset]("EquipmentAsset", asset)
	if err != nil {
		return openapiv1.EquipmentAsset{}, false
	}
	return converted, true
}

func classifyEquipmentError(err error) (int, string, string) {
	switch {
	case errors.Is(err, model.ErrInvalidEquipmentAsset):
		return http.StatusBadRequest, "INVALID_EQUIPMENT", err.Error()
	case errors.Is(err, model.ErrEquipmentAssetNotFound):
		return http.StatusNotFound, "EQUIPMENT_NOT_FOUND", "equipment is unavailable to this owner"
	case errors.Is(err, model.ErrEquipmentAssetRevisionConflict):
		return http.StatusConflict, "EQUIPMENT_REVISION_CONFLICT", "equipment changed since it was read"
	case errors.Is(err, model.ErrEquipmentAssetRetired):
		return http.StatusConflict, "EQUIPMENT_RETIRED", "retired equipment cannot be changed"
	case errors.Is(err, model.ErrEquipmentAssetDuplicateName):
		return http.StatusConflict, "EQUIPMENT_DUPLICATE_NAME", "equipment with this name is already listed"
	case errors.Is(err, model.ErrEquipmentAssetIdempotencyConflict):
		return http.StatusConflict, "EQUIPMENT_IDEMPOTENCY_CONFLICT", "idempotency key already authorized different input"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save equipment"
	}
}

func createEquipmentError(status int, code, message string) openapiv1.CreateEquipmentAssetResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.CreateEquipmentAsset400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.CreateEquipmentAsset401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.CreateEquipmentAsset409JSONResponse{ConflictJSONResponse: openapiv1.ConflictJSONResponse(e)}
	default:
		return openapiv1.CreateEquipmentAsset500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}

func updateEquipmentError(status int, code, message string) openapiv1.UpdateEquipmentAssetResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateEquipmentAsset400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateEquipmentAsset401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	case http.StatusNotFound:
		return openapiv1.UpdateEquipmentAsset404JSONResponse{NotFoundJSONResponse: openapiv1.NotFoundJSONResponse(e)}
	case http.StatusConflict:
		return openapiv1.UpdateEquipmentAsset409JSONResponse{ConflictJSONResponse: openapiv1.ConflictJSONResponse(e)}
	default:
		return openapiv1.UpdateEquipmentAsset500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
