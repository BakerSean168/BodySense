package httpapi

import (
	"net/http"

	"github.com/bodysense/api/internal/dto"
	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/gin-gonic/gin"
)

// StrictHandler wraps handwritten adapters with generated request/response
// decoding. OpenAPI schema validation itself is provided by RequestValidator.
func StrictHandler(server *PublicServer) openapiv1.ServerInterface {
	return openapiv1.NewStrictHandlerWithOptions(
		server,
		nil,
		openapiv1.StrictGinServerOptions{
			RequestErrorHandlerFunc: func(c *gin.Context, err error) {
				c.JSON(http.StatusBadRequest, dto.NewErrorResponse("INVALID_REQUEST", err.Error()))
			},
			HandlerErrorFunc: func(c *gin.Context, _ error) {
				c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("INTERNAL_ERROR", "request processing failed"))
			},
			ResponseErrorHandlerFunc: func(c *gin.Context, _ error) {
				c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("INTERNAL_ERROR", "response encoding failed"))
			},
		},
	)
}
