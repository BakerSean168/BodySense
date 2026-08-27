package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestBodyStateFactFromInputCarriesCanonicalRegionID(t *testing.T) {
	regionID := "shoulder.right"
	fact := bodyStateFactFromInput(dto.BodyStateFactInput{
		Kind:         "discomfort",
		BodyRegion:   "右肩",
		BodyRegionID: &regionID,
		Value:        "抬手疼痛",
	})
	if fact.BodyRegionID == nil || *fact.BodyRegionID != regionID || fact.BodyRegion != "右肩" {
		t.Fatalf("request mapping lost additive region identity: %#v", fact)
	}
}

func TestBodyStateUnknownRegionMapsToBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	if !bodyStateHandleMutationError(ctx, errors.Join(service.ErrUnknownBodyRegionID, errors.New("shoulder.middle"))) {
		t.Fatal("expected mutation error to be handled")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown region must be a client error, got %d", recorder.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "INVALID_BODY_REGION_ID" {
		t.Fatalf("unexpected error code: %q body=%s", body.Error.Code, recorder.Body.String())
	}
}
