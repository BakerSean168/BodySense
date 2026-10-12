package httpapi

import (
	"context"

	"github.com/gin-gonic/gin"
)

// applicationRequestContext passes the immutable standard request context into
// domain/database work. Gin reuses its Context after a response, while database/sql
// may still observe cancellation in another goroutine after Rows.Close returns.
// Auth identity must be resolved before this boundary and passed explicitly.
func applicationRequestContext(ctx context.Context) context.Context {
	if request, ok := ctx.(*gin.Context); ok {
		if request.Request == nil {
			return context.Background()
		}
		return request.Request.Context()
	}
	return ctx
}
