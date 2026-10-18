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
	ReplaceProfile(context.Context, uuid.UUID, model.ReplaceProfileCommand) (*model.UserProfile, error)
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
	publicProfile, err := strictOpenAPIConvert[openapiv1.UserProfile]("UserProfile", profile)
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

	body := request.Body
	command := model.ReplaceProfileCommand{
		ExpectedRevision: body.ExpectedRevision,
		ProfileFields: model.ProfileFields{
			DisplayName:   body.DisplayName,
			AvatarAssetID: body.AvatarAssetId,
			Locale:        body.Preferences.Locale,
			TimeZone:      body.Preferences.TimeZone,
		},
	}
	if body.Gender != nil {
		gender := string(*body.Gender)
		command.Gender = &gender
	}
	if body.Preferences.UnitPreference != nil {
		unit := string(*body.Preferences.UnitPreference)
		command.UnitPreference = &unit
	}
	if body.BirthDate != nil {
		birthDate, parseErr := model.ParseDateOnly(body.BirthDate.String())
		if parseErr != nil {
			return updateUserProfileError(http.StatusBadRequest, "INVALID_PROFILE", "birth_date must use YYYY-MM-DD"), nil
		}
		command.BirthDate = &birthDate
	}
	stored, err := s.profile.ReplaceProfile(applicationRequestContext(ctx), userID, command)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrProfileRevisionConflict):
			return updateUserProfileError(http.StatusConflict, "PROFILE_REVISION_CONFLICT", "profile changed since it was read"), nil
		case errors.Is(err, model.ErrProfileAvatarUnavailable):
			return updateUserProfileError(http.StatusBadRequest, "PROFILE_AVATAR_UNAVAILABLE", "avatar asset is not an available photo"), nil
		case errors.Is(err, service.ErrInvalidProfile):
			return updateUserProfileError(http.StatusBadRequest, "INVALID_PROFILE", err.Error()), nil
		}
		return updateUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update profile"), nil
	}
	if stored == nil {
		return updateUserProfileError(http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load updated profile"), nil
	}
	publicProfile, err := strictOpenAPIConvert[openapiv1.UserProfile]("UserProfile", stored)
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
	case http.StatusConflict:
		return openapiv1.UpdateUserProfile409JSONResponse{ConflictJSONResponse: openapiv1.ConflictJSONResponse(e)}
	default:
		return openapiv1.UpdateUserProfile500JSONResponse{InternalErrorJSONResponse: openapiv1.InternalErrorJSONResponse(e)}
	}
}
