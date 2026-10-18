package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	openapiv1 "github.com/bodysense/api/internal/generated/openapi/v1"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func assetErasureIntegrationRouter(t *testing.T, db *gorm.DB, actingUser uuid.UUID, registry *uploadstorage.Registry) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	spec, err := openapiv1.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	assets := repository.NewUserAssetRepository(db)
	uploads := service.NewUploadService(assets, repository.NewDocumentExtractionRunRepository(db), repository.NewPostureAnalysisRunRepository(db), nil, nil, registry)
	erasure := service.NewAssetErasureService(repository.NewAssetErasureRepository(db), uploads)
	router := gin.New()
	RegisterRoutes(router, StrictHandler(NewPublicServer().WithUploads(uploads, nil).WithAssetErasure(erasure)), RouteSecurity{
		Auth:      func(c *gin.Context) { c.Set("user_id", actingUser.String()); c.Next() },
		Operator:  func(c *gin.Context) { c.Next() },
		Validator: RequestValidator(spec),
	})
	return router
}

// The public erasure route reports the asset erased only with a fully
// verified receipt, masks foreign assets, and leaves source removal
// reporting the erased lifecycle afterwards.
func TestUserAssetErasureHTTPPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL 18")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	owner, stranger := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, stranger} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id, "as002-http-"+id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		uid := id
		t.Cleanup(func() { db.Exec("DELETE FROM users WHERE id=?", uid) })
	}
	store, err := uploadstorage.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := uploadstorage.NewRegistryFromStores("local", store)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("synthetic-private-report")
	asset := &model.UserAsset{ID: uuid.New(), UserID: owner, Kind: model.UserAssetKindReport, OriginalName: "private.pdf", MimeType: "application/pdf",
		ByteSize: int64(len(payload)), ContentSHA256: strings.Repeat("c", 64), StorageBackend: "local", Availability: model.UserAssetAvailable}
	asset.StorageKey = owner.String() + "/" + asset.ID.String() + "/original.pdf"
	if err := repository.NewUserAssetRepository(db).Create(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), asset.StorageKey, bytes.NewReader(payload), int64(len(payload)), asset.MimeType); err != nil {
		t.Fatal(err)
	}
	do := func(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	path := "/api/v1/assets/" + asset.ID.String() + "/erasure"
	strangerRouter := assetErasureIntegrationRouter(t, db, stranger, registry)
	if rec := do(strangerRouter, http.MethodPost, path, `{"idempotency_key":"steal"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign erasure: %d %s", rec.Code, rec.Body)
	}
	ownerRouter := assetErasureIntegrationRouter(t, db, owner, registry)
	if rec := do(ownerRouter, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("receipt before request: %d %s", rec.Code, rec.Body)
	}
	rec := do(ownerRouter, http.MethodPost, path, `{"idempotency_key":"erase"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("erasure: %d %s", rec.Code, rec.Body)
	}
	var receipt openapiv1.AssetErasureReceipt
	if err := json.Unmarshal(rec.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "erased" || receipt.ErasedAt == nil || len(receipt.OutstandingTargets) != 0 || len(receipt.Targets) != len(model.AssetErasureTargets) {
		t.Fatalf("receipt=%+v", receipt)
	}
	if exists, err := store.Exists(context.Background(), asset.StorageKey); err != nil || exists {
		t.Fatalf("object exists=%v err=%v", exists, err)
	}
	if rec := do(strangerRouter, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign receipt read: %d %s", rec.Code, rec.Body)
	}
	if rec := do(ownerRouter, http.MethodGet, path, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"erased"`) {
		t.Fatalf("receipt read: %d %s", rec.Code, rec.Body)
	}
	if rec := do(ownerRouter, http.MethodDelete, "/api/v1/assets/"+asset.ID.String(), ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"availability":"erased"`) {
		t.Fatalf("source removal after erasure: %d %s", rec.Code, rec.Body)
	}
}
