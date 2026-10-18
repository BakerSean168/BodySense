package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestAssetRuntimeScopeOutcome(t *testing.T) {
	asset := uuid.New()
	a, b := uuid.New(), uuid.New()
	scope := AssetRuntimeScope{ThreadIDs: []uuid.UUID{a, b}, AttachedThreadIDs: []uuid.UUID{a}, ContentSHA256: "x"}
	busy := "thread_busy"
	conflict := "checkpoint_conflict"
	thread := func(id uuid.UUID, status string, code *string) RuntimeThreadReceipt {
		return RuntimeThreadReceipt{ThreadID: id.String(), Status: status, RedactedRefs: 2, DeletedCheckpoints: 3, FailureCode: code}
	}
	cases := []struct {
		name    string
		receipt *RuntimeErasureReceipt
		want    AssetErasureOutcome
	}{
		{"verified", &RuntimeErasureReceipt{Status: "erased", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetVerified, Erased: 5}},
		{"busy", &RuntimeErasureReceipt{Status: "outstanding", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "outstanding", &busy), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 1, FailureCode: AssetErasureCodeRuntimeThreadBusy}},
		{"conflict", &RuntimeErasureReceipt{Status: "outstanding", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "outstanding", &busy), thread(b, "outstanding", &conflict)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"missing thread", &RuntimeErasureReceipt{Status: "erased", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "erased", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 1, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"other asset", &RuntimeErasureReceipt{Status: "erased", SubjectID: uuid.NewString(),
			Threads: []RuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"contradictory status", &RuntimeErasureReceipt{Status: "outstanding", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"unknown thread status", &RuntimeErasureReceipt{Status: "erased", SubjectID: asset.String(),
			Threads: []RuntimeThreadReceipt{thread(a, "cleaned", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 1, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"no receipt", nil,
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
	}
	for _, tc := range cases {
		if got := scope.Outcome(asset, tc.receipt); got != tc.want {
			t.Errorf("%s: got %+v want %+v", tc.name, got, tc.want)
		}
	}
}
