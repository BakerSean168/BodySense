package handler

import (
	"net/http"

	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

type HealthWorkspaceHandler struct {
	workspace *service.HealthWorkspaceService
}

func NewHealthWorkspaceHandler(workspace *service.HealthWorkspaceService) *HealthWorkspaceHandler {
	return &HealthWorkspaceHandler{workspace: workspace}
}

func (h *HealthWorkspaceHandler) Get(c *gin.Context) {
	uid, ok := getUserUUID(c)
	if !ok {
		return
	}
	workspace, err := h.workspace.Get(c.Request.Context(), uid)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load health workspace")
		return
	}
	c.JSON(http.StatusOK, workspace)
}
