package httpapi

import (
	"net/http"

	"github.com/bodysense/api/internal/dto"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-gonic/gin"
	ginmiddleware "github.com/oapi-codegen/gin-middleware"
)

// RequestValidator validates a generated public OpenAPI route after the existing
// BodySense authentication middleware has established the user identity.
// Authentication itself remains owned by AuthMiddleware; the OpenAPI filter is
// deliberately configured with a no-op authentication callback so it validates
// the bearer requirement's wire shape without duplicating token/session logic.
func RequestValidator(spec *openapi3.T) gin.HandlerFunc {
	// Public deployments sit behind different proxy hosts. Host/server matching
	// is not a request-trust invariant, so remove Servers from the validator copy.
	// Path, method, headers, body and schema validation remain enabled.
	spec.Servers = nil
	return ginmiddleware.OapiRequestValidatorWithOptions(spec, &ginmiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(c *gin.Context, message string, statusCode int) {
			code := "INVALID_REQUEST"
			if statusCode == http.StatusNotFound {
				code = "NOT_FOUND"
			}
			c.JSON(statusCode, dto.NewErrorResponse(code, message))
			c.Abort()
		},
		SilenceServersWarning: true,
	})
}
