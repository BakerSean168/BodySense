package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

func equipmentCreate(key, name string) model.CreateEquipmentAssetCommand {
	return model.CreateEquipmentAssetCommand{IdempotencyKey: key, EquipmentAssetFields: model.EquipmentAssetFields{
		DisplayName: name, AccessKind: model.EquipmentAccessOwned, Availability: model.EquipmentAvailable}}
}

func TestEquipmentAssetLifecycleAndOwnerIsolationPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	foreign := seedBodyStateContextUser(t, db)
	repo := NewEquipmentAssetRepository(db)
	ctx := context.Background()

	notes := "  medium resistance  "
	create := equipmentCreate("eq-1", "  弹力带 ")
	create.Notes = &notes
	band, err := repo.Create(ctx, owner, create)
	if err != nil || band.Revision != 1 || band.DisplayName != "弹力带" || band.Notes == nil || *band.Notes != "medium resistance" || band.UserID != owner {
		t.Fatalf("create: %#v %v", band, err)
	}
	// Exact retry replays; same key with different fields conflicts.
	replay, err := repo.Create(ctx, owner, create)
	if err != nil || replay.ID != band.ID {
		t.Fatalf("replay: %#v %v", replay, err)
	}
	if _, err := repo.Create(ctx, owner, equipmentCreate("eq-1", "哑铃")); !errors.Is(err, model.ErrEquipmentAssetIdempotencyConflict) {
		t.Fatalf("key reuse with different input: %v", err)
	}
	// One current asset per name (case-insensitive) per owner; other owners unaffected.
	if _, err := repo.Create(ctx, owner, equipmentCreate("eq-dup", "弹力带")); !errors.Is(err, model.ErrEquipmentAssetDuplicateName) {
		t.Fatalf("duplicate name: %v", err)
	}
	foreignBand, err := repo.Create(ctx, foreign, equipmentCreate("eq-1", "弹力带"))
	if err != nil || foreignBand.ID == band.ID {
		t.Fatalf("foreign owner must have its own namespace: %#v %v", foreignBand, err)
	}
	if _, err := repo.Create(ctx, owner, model.CreateEquipmentAssetCommand{IdempotencyKey: "bad",
		EquipmentAssetFields: model.EquipmentAssetFields{DisplayName: "x", AccessKind: "owned", Availability: model.EquipmentRetired}}); !errors.Is(err, model.ErrInvalidEquipmentAsset) {
		t.Fatalf("new asset must not start retired: %v", err)
	}

	// Foreign owner cannot see or change the asset.
	update := model.UpdateEquipmentAssetCommand{ExpectedRevision: 1, EquipmentAssetFields: model.EquipmentAssetFields{
		DisplayName: "弹力带", Notes: band.Notes, AccessKind: model.EquipmentAccessOwned, Availability: model.EquipmentUnavailable}}
	if _, err := repo.Update(ctx, foreign, band.ID, update); !errors.Is(err, model.ErrEquipmentAssetNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	paused, err := repo.Update(ctx, owner, band.ID, update)
	if err != nil || paused.Revision != 2 || paused.Availability != model.EquipmentUnavailable {
		t.Fatalf("mark unavailable: %#v %v", paused, err)
	}
	if again, err := repo.Update(ctx, owner, band.ID, update); err != nil || again.Revision != 2 {
		t.Fatalf("exact update replay: %#v %v", again, err)
	}
	stale := update
	stale.Availability = model.EquipmentAvailable
	if _, err := repo.Update(ctx, owner, band.ID, stale); !errors.Is(err, model.ErrEquipmentAssetRevisionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	usable, err := repo.ListUsable(ctx, owner, time.Now())
	if err != nil || len(usable) != 0 {
		t.Fatalf("unavailable equipment must not be usable: %#v %v", usable, err)
	}

	retire := update
	retire.ExpectedRevision, retire.Availability = 2, model.EquipmentRetired
	if retired, err := repo.Update(ctx, owner, band.ID, retire); err != nil || retired.Availability != model.EquipmentRetired || retired.Revision != 3 {
		t.Fatalf("retire: %#v %v", retired, err)
	}
	revive := retire
	revive.ExpectedRevision, revive.Availability = 3, model.EquipmentAvailable
	if _, err := repo.Update(ctx, owner, band.ID, revive); !errors.Is(err, model.ErrEquipmentAssetRetired) {
		t.Fatalf("retired must be terminal: %v", err)
	}
	// Retiring frees the name for a new asset; the retired row is kept but not listed.
	again, err := repo.Create(ctx, owner, equipmentCreate("eq-2", "弹力带"))
	if err != nil {
		t.Fatalf("re-add after retire: %v", err)
	}
	list, err := repo.List(ctx, owner)
	if err != nil || len(list) != 1 || list[0].ID != again.ID {
		t.Fatalf("list: %#v %v", list, err)
	}
	usable, err = repo.ListUsable(ctx, owner, time.Now())
	if err != nil || len(usable) != 1 || usable[0].ID != again.ID {
		t.Fatalf("usable: %#v %v", usable, err)
	}
	var stored int64
	if err := db.Model(&model.EquipmentAsset{}).Where("user_id = ?", owner).Count(&stored).Error; err != nil || stored != 2 {
		t.Fatalf("retired history must be retained: %d %v", stored, err)
	}
}

func TestEquipmentAssetConcurrentCreateSameKeyPostgres(t *testing.T) {
	db := openBodyStateContextIntegrationDB(t)
	owner := seedBodyStateContextUser(t, db)
	repo := NewEquipmentAssetRepository(db)
	ctx := context.Background()
	command := equipmentCreate("eq-race", "壶铃")
	ids := make([]uuid.UUID, 6)
	errs := make([]error, 6)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			asset, err := repo.Create(ctx, owner, command)
			errs[i] = err
			if asset != nil {
				ids[i] = asset.ID
			}
		}(i)
	}
	wg.Wait()
	for i := range ids {
		if errs[i] != nil || ids[i] != ids[0] {
			t.Fatalf("concurrent exact retries must converge on one asset: ids=%v errs=%v", ids, errs)
		}
	}
}

func TestEquipmentAssetValidityWindow(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	cases := []struct {
		asset model.EquipmentAsset
		want  bool
	}{
		{model.EquipmentAsset{Availability: model.EquipmentAvailable}, true},
		{model.EquipmentAsset{Availability: model.EquipmentAvailable, ValidFrom: &future}, false},
		{model.EquipmentAsset{Availability: model.EquipmentAvailable, ValidUntil: &past}, false},
		{model.EquipmentAsset{Availability: model.EquipmentAvailable, ValidFrom: &past, ValidUntil: &future}, true},
		{model.EquipmentAsset{Availability: model.EquipmentUnavailable}, false},
	}
	for i, c := range cases {
		if got := c.asset.UsableAt(now); got != c.want {
			t.Fatalf("case %d: usable=%v want %v", i, got, c.want)
		}
	}
}
