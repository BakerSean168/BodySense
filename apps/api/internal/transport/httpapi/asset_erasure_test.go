package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeAssetErasure struct {
	receipt *model.AssetErasureReceipt
	err     error
	key     string
}

func (f *fakeAssetErasure) Request(_ context.Context, _, _ uuid.UUID, key string) (*model.AssetErasureReceipt, error) {
	f.key = key
	return f.receipt, f.err
}

func (f *fakeAssetErasure) Receipt(context.Context, uuid.UUID, uuid.UUID) (*model.AssetErasureReceipt, error) {
	return f.receipt, f.err
}

func assetErasureRouter(t *testing.T, erasure assetErasureApplication) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithAssetErasure(erasure)), RouteSecurity{
		Auth:      func(c *gin.Context) { c.Set("user_id", uuid.NewString()); c.Next() },
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

func syntheticErasureReceipt(status model.AssetErasureStatus, sourceState model.AssetErasureTargetState) *model.AssetErasureReceipt {
	now := time.Now().UTC()
	receipt := &model.AssetErasureReceipt{Request: model.AssetErasureRequest{ID: uuid.New(), AssetID: uuid.New(), Status: status,
		InventoryRevision: model.AssetErasureInventoryRevision, RequestedAt: now}}
	for _, name := range model.AssetErasureTargets {
		target := model.AssetErasureTarget{Target: name, State: model.AssetErasureTargetVerified, VerifiedAt: &now}
		if name == model.AssetErasureSourceObjects && sourceState != model.AssetErasureTargetVerified {
			code := model.AssetErasureCodeObjectDeleteFailed
			target = model.AssetErasureTarget{Target: name, State: sourceState, OutstandingCount: 1, FailureCode: &code}
		}
		receipt.Targets = append(receipt.Targets, target)
	}
	if status == model.AssetErasureErased {
		receipt.Request.ErasedAt = &now
	}
	return receipt
}

func doAssetErasure(router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/assets/"+uuid.NewString()+"/erasure", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAssetErasureHTTPReceipts(t *testing.T) {
	outstanding := &fakeAssetErasure{receipt: syntheticErasureReceipt(model.AssetErasureOutstanding, model.AssetErasureTargetFailed)}
	rec := doAssetErasure(assetErasureRouter(t, outstanding), http.MethodPost, `{"idempotency_key":"erase-1"}`)
	if rec.Code != http.StatusAccepted || outstanding.key != "erase-1" {
		t.Fatalf("outstanding: %d %s", rec.Code, rec.Body)
	}
	var view openapiv1.AssetErasureReceipt
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if view.Status != "outstanding" || view.ErasedAt != nil || len(view.OutstandingTargets) != 1 || view.OutstandingTargets[0] != "source_objects" ||
		view.Targets[0].FailureCode == nil || *view.Targets[0].FailureCode != "object_delete_failed" {
		t.Fatalf("outstanding view=%+v", view)
	}
	erased := &fakeAssetErasure{receipt: syntheticErasureReceipt(model.AssetErasureErased, model.AssetErasureTargetVerified)}
	if rec := doAssetErasure(assetErasureRouter(t, erased), http.MethodPost, `{"idempotency_key":"erase-1"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"outstanding_targets":[]`) {
		t.Fatalf("erased: %d %s", rec.Code, rec.Body)
	}
	if rec := doAssetErasure(assetErasureRouter(t, erased), http.MethodGet, ""); rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
}

func TestAssetErasureHTTPErrors(t *testing.T) {
	cases := []struct {
		name, method, body string
		app                assetErasureApplication
		status             int
		code               string
	}{
		{"missing key", http.MethodPost, `{}`, &fakeAssetErasure{}, http.StatusBadRequest, ""},
		{"blank key", http.MethodPost, `{"idempotency_key":" padded "}`, &fakeAssetErasure{}, http.StatusBadRequest, ""},
		{"foreign asset", http.MethodPost, `{"idempotency_key":"k"}`, &fakeAssetErasure{err: model.ErrAssetErasureNotFound}, http.StatusNotFound, "NOT_FOUND"},
		{"key reuse", http.MethodPost, `{"idempotency_key":"k"}`, &fakeAssetErasure{err: model.ErrAssetErasureIdempotencyUsed}, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED"},
		{"no receipt", http.MethodGet, "", &fakeAssetErasure{err: model.ErrAssetErasureNotFound}, http.StatusNotFound, "NOT_FOUND"},
		{"unconfigured", http.MethodGet, "", nil, http.StatusServiceUnavailable, "ASSET_ERASURE_UNAVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAssetErasure(assetErasureRouter(t, tc.app), tc.method, tc.body)
			if rec.Code != tc.status || (tc.code != "" && !strings.Contains(rec.Body.String(), tc.code)) {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
		})
	}
}
