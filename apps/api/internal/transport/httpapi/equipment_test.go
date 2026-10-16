package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

type fakeEquipmentApplication struct {
	calls  int
	owner  uuid.UUID
	create model.CreateEquipmentAssetCommand
	update model.UpdateEquipmentAssetCommand
	result *model.EquipmentAsset
	items  []model.EquipmentAsset
	err    error
}

func (f *fakeEquipmentApplication) Create(_ context.Context, owner uuid.UUID, command model.CreateEquipmentAssetCommand) (*model.EquipmentAsset, error) {
	f.calls++
	f.owner, f.create = owner, command
	return f.result, f.err
}

func (f *fakeEquipmentApplication) Update(_ context.Context, owner, _ uuid.UUID, command model.UpdateEquipmentAssetCommand) (*model.EquipmentAsset, error) {
	f.calls++
	f.owner, f.update = owner, command
	return f.result, f.err
}

func (f *fakeEquipmentApplication) List(_ context.Context, owner uuid.UUID) ([]model.EquipmentAsset, error) {
	f.calls++
	f.owner = owner
	return f.items, f.err
}

func equipmentFixture(owner uuid.UUID) model.EquipmentAsset {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	notes := "medium resistance"
	return model.EquipmentAsset{ID: uuid.New(), UserID: owner, DisplayName: "弹力带", Notes: &notes,
		AccessKind: model.EquipmentAccessOwned, Availability: model.EquipmentAvailable, Revision: 1,
		CreatedAt: now, UpdatedAt: now}
}

func TestCreateEquipmentAssetMapsCommandAndReturnsAsset(t *testing.T) {
	owner := uuid.New()
	asset := equipmentFixture(owner)
	app := &fakeEquipmentApplication{result: &asset}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithEquipment(app), owner.String())
	rec := performJSONAt(router, http.MethodPost, "/api/v1/equipment",
		`{"idempotency_key":"eq-1","display_name":"弹力带","notes":"medium resistance","access_kind":"owned","availability":"available"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if app.owner != owner || app.create.IdempotencyKey != "eq-1" || app.create.DisplayName != "弹力带" ||
		app.create.Notes == nil || *app.create.Notes != "medium resistance" || app.create.AccessKind != "owned" {
		t.Fatalf("command not mapped: %#v", app.create)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != asset.ID.String() || body["revision"] != float64(1) || body["valid_until"] != nil {
		t.Fatalf("response: %s", rec.Body.String())
	}
	if _, leaked := body["create_idempotency_key"]; leaked {
		t.Fatalf("idempotency internals leaked: %s", rec.Body.String())
	}
}

func TestCreateEquipmentAssetRejectsRetiredAndUnknownFieldsBeforeService(t *testing.T) {
	app := &fakeEquipmentApplication{}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithEquipment(app), uuid.NewString())
	for _, body := range []string{
		`{"idempotency_key":"k","display_name":"band","access_kind":"owned","availability":"retired"}`,
		`{"idempotency_key":"k","display_name":"band","access_kind":"owned","availability":"available","user_id":"` + uuid.NewString() + `"}`,
		`{"display_name":"band","access_kind":"owned","availability":"available"}`,
	} {
		rec := performJSONAt(router, http.MethodPost, "/api/v1/equipment", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if app.calls != 0 {
		t.Fatalf("invalid requests reached the service: %d", app.calls)
	}
}

func TestUpdateEquipmentAssetMapsErrorsAndRejectsForeignResult(t *testing.T) {
	owner := uuid.New()
	id := uuid.New()
	body := `{"expected_revision":1,"display_name":"band","access_kind":"owned","availability":"unavailable"}`
	cases := map[error]struct {
		status int
		code   string
	}{
		model.ErrEquipmentAssetNotFound:            {http.StatusNotFound, "EQUIPMENT_NOT_FOUND"},
		model.ErrEquipmentAssetRevisionConflict:    {http.StatusConflict, "EQUIPMENT_REVISION_CONFLICT"},
		model.ErrEquipmentAssetRetired:             {http.StatusConflict, "EQUIPMENT_RETIRED"},
		model.ErrEquipmentAssetDuplicateName:       {http.StatusConflict, "EQUIPMENT_DUPLICATE_NAME"},
		model.ErrEquipmentAssetIdempotencyConflict: {http.StatusConflict, "EQUIPMENT_IDEMPOTENCY_CONFLICT"},
	}
	for err, want := range cases {
		app := &fakeEquipmentApplication{err: err}
		router := newOpenAPITestRouterWith(t, NewPublicServer().WithEquipment(app), owner.String())
		rec := performJSONAt(router, http.MethodPut, "/api/v1/equipment/"+id.String(), body)
		if rec.Code != want.status {
			t.Fatalf("%v: status=%d body=%s", err, rec.Code, rec.Body.String())
		}
		assertErrorCode(t, rec, want.code)
	}
	foreign := equipmentFixture(uuid.New())
	foreign.ID = id
	app := &fakeEquipmentApplication{result: &foreign}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithEquipment(app), owner.String())
	rec := performJSONAt(router, http.MethodPut, "/api/v1/equipment/"+id.String(), body)
	if rec.Code != http.StatusInternalServerError || app.update.ExpectedRevision != 1 || app.update.Availability != "unavailable" {
		t.Fatalf("foreign-owner result must not be returned: status=%d cmd=%#v", rec.Code, app.update)
	}
}

func TestListEquipmentAssetsReturnsOwnerItems(t *testing.T) {
	owner := uuid.New()
	app := &fakeEquipmentApplication{items: []model.EquipmentAsset{equipmentFixture(owner)}}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithEquipment(app), owner.String())
	rec := performJSONAt(router, http.MethodGet, "/api/v1/equipment", "")
	if rec.Code != http.StatusOK || app.owner != owner {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Items) != 1 || body.Items[0]["display_name"] != "弹力带" {
		t.Fatalf("list: %s %v", rec.Body.String(), err)
	}
	app.items = []model.EquipmentAsset{equipmentFixture(uuid.New())}
	if rec := performJSONAt(router, http.MethodGet, "/api/v1/equipment", ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("foreign item must fail closed: %d", rec.Code)
	}
}
