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
	thread := func(id uuid.UUID, status string, code *string) AssetRuntimeThreadReceipt {
		return AssetRuntimeThreadReceipt{ThreadID: id.String(), Status: status, RedactedRefs: 2, DeletedCheckpoints: 3, FailureCode: code}
	}
	cases := []struct {
		name    string
		receipt *AssetRuntimeErasureReceipt
		want    AssetErasureOutcome
	}{
		{"verified", &AssetRuntimeErasureReceipt{Status: "erased", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetVerified, Erased: 5}},
		{"busy", &AssetRuntimeErasureReceipt{Status: "outstanding", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "outstanding", &busy), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 1, FailureCode: AssetErasureCodeRuntimeThreadBusy}},
		{"conflict", &AssetRuntimeErasureReceipt{Status: "outstanding", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "outstanding", &busy), thread(b, "outstanding", &conflict)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"missing thread", &AssetRuntimeErasureReceipt{Status: "erased", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "erased", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 1, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"other asset", &AssetRuntimeErasureReceipt{Status: "erased", AssetID: uuid.NewString(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"contradictory status", &AssetRuntimeErasureReceipt{Status: "outstanding", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "erased", nil), thread(b, "absent", nil)}},
			AssetErasureOutcome{State: AssetErasureTargetFailed, Outstanding: 2, FailureCode: AssetErasureCodeRuntimeUnverified}},
		{"unknown thread status", &AssetRuntimeErasureReceipt{Status: "erased", AssetID: asset.String(),
			Threads: []AssetRuntimeThreadReceipt{thread(a, "cleaned", nil), thread(b, "absent", nil)}},
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
