package httpapi

import (
	"context"
	"errors"
	"net/http"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type profileApplication interface {
	GetProfile(context.Context, uuid.UUID) (*model.UserProfile, error)
	CreateOrUpdateProfile(context.Context, uuid.UUID, *model.UserProfile) error
}

func (s *PublicServer) WithProfile(profile profileApplication) *PublicServer {
	s.profile = profile
	return s
}

func (s *PublicServer) GetCurrentUser(
	ctx context.Context,
	_ openapiv1.GetCurrentUserRequestObject,
) (openapiv1.GetCurrentUserResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getCurrentUserError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	email, ok := ctx.Value("email").(string)
	if !ok || email == "" {
		return getCurrentUserError(http.StatusInternalServerError, "INTERNAL_ERROR", "authenticated email is unavailable"), nil
	}
	return openapiv1.GetCurrentUser200JSONResponse{
		Id: userID, Email: openapi_types.Email(email),
	}, nil
}

func (s *PublicServer) GetUserProfile(
	ctx context.Context,
	_ openapiv1.GetUserProfileRequestObject,
) (openapiv1.GetUserProfileResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return getUserProfileError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.profile == nil {
		return getUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "profile service is unavailable"), nil
	}
	profile, err := s.profile.GetProfile(ctx, userID)
	if err != nil {
		return getUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load profile"), nil
	}
	if profile == nil {
		return openapiv1.GetUserProfile200JSONResponse{Profile: nil}, nil
	}
	publicProfile, err := strictJSONConvert[openapiv1.UserProfile](profile)
	if err != nil {
		return getUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "profile violates the public contract"), nil
	}
	return openapiv1.GetUserProfile200JSONResponse{Profile: &publicProfile}, nil
}

func (s *PublicServer) UpdateUserProfile(
	ctx context.Context,
	request openapiv1.UpdateUserProfileRequestObject,
) (openapiv1.UpdateUserProfileResponseObject, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return updateUserProfileError(http.StatusUnauthorized, "UNAUTHORIZED", "authentication required"), nil
	}
	if s.profile == nil || request.Body == nil {
		return updateUserProfileError(http.StatusBadRequest, "INVALID_REQUEST", "request body is required"), nil
	}

	profile := &model.UserProfile{}
	if request.Body.Gender != nil {
		gender := string(*request.Body.Gender)
		profile.Gender = &gender
	}
	if request.Body.BirthDate != nil {
		birthDate, parseErr := model.ParseDateOnly(request.Body.BirthDate.String())
		if parseErr != nil {
			return updateUserProfileError(http.StatusBadRequest, "INVALID_PROFILE", "birth_date must use YYYY-MM-DD"), nil
		}
		profile.BirthDate = &birthDate
	}
	if err := s.profile.CreateOrUpdateProfile(ctx, userID, profile); err != nil {
		if errors.Is(err, service.ErrInvalidProfile) {
			return updateUserProfileError(http.StatusBadRequest, "INVALID_PROFILE", err.Error()), nil
		}
		return updateUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update profile"), nil
	}

	stored, err := s.profile.GetProfile(ctx, userID)
	if err != nil || stored == nil {
		return updateUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load updated profile"), nil
	}
	publicProfile, err := strictJSONConvert[openapiv1.UserProfile](stored)
	if err != nil {
		return updateUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "profile violates the public contract"), nil
	}
	return openapiv1.UpdateUserProfile200JSONResponse{Profile: publicProfile}, nil
}

func getCurrentUserError(status int, code, message string) openapiv1.GetCurrentUserResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetCurrentUser401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetCurrentUser500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}

func getUserProfileError(status int, code, message string) openapiv1.GetUserProfileResponseObject {
	e := errorEnvelope(code, message)
	if status == http.StatusUnauthorized {
		return openapiv1.GetUserProfile401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	}
	return openapiv1.GetUserProfile500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
}

func updateUserProfileError(status int, code, message string) openapiv1.UpdateUserProfileResponseObject {
	e := errorEnvelope(code, message)
	switch status {
	case http.StatusBadRequest:
		return openapiv1.UpdateUserProfile400JSONResponse{InvalidRequestJSONResponse: openapiv1.InvalidRequestJSONResponse(e)}
	case http.StatusUnauthorized:
		return openapiv1.UpdateUserProfile401JSONResponse{UnauthorizedJSONResponse: openapiv1.UnauthorizedJSONResponse(e)}
	default:
		return openapiv1.UpdateUserProfile500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
