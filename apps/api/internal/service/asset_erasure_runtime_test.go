package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func TestAIClientEraseAssetRuntimeCopies(t *testing.T) {
	owner, asset, thread := uuid.New(), uuid.New(), uuid.New()
	var got assetRuntimeErasureRequest
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"status":"erased","asset_id":"` + asset.String() + `","threads":[{"thread_id":"` +
			thread.String() + `","status":"erased","checkpoint_id":"c1","redacted_refs":2,"deleted_checkpoints":4,"failure_code":null}]}`))
	}))
	defer srv.Close()
	client := &AIClient{baseURL: srv.URL, httpClient: srv.Client()}
	scope := model.AssetRuntimeScope{ThreadIDs: []uuid.UUID{thread}, AttachedThreadIDs: []uuid.UUID{thread}, ContentSHA256: "ab"}
	receipt, err := client.EraseAssetRuntimeCopies(context.Background(), owner, asset, scope)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/runtime/assets/"+asset.String()+"/erasure" || got.UserID != owner.String() || got.ContentSHA256 != "ab" ||
		len(got.ThreadIDs) != 1 || got.ThreadIDs[0] != thread.String() || len(got.AttachedThreadIDs) != 1 {
		t.Fatalf("path=%s request=%+v", path, got)
	}
	if outcome := scope.Outcome(asset, receipt); outcome.State != model.AssetErasureTargetVerified || outcome.Erased != 6 {
		t.Fatalf("outcome=%+v", outcome)
	}

	empty := model.AssetRuntimeScope{ContentSHA256: "ab"}
	if _, err := client.EraseAssetRuntimeCopies(context.Background(), owner, asset, empty); err != nil {
		t.Fatal(err)
	}
	if got.ThreadIDs == nil || got.AttachedThreadIDs == nil {
		t.Fatalf("empty lists must serialize as []: %+v", got)
	}
}
