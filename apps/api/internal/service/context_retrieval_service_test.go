package service

import (
	"testing"

	"github.com/bodysense/api/internal/model"
)

func TestBuildHistoricalContextTermsUsesCurrentTurnAndDurableState(t *testing.T) {
	snapshot := &BodyStateSnapshot{
		Facts: []model.BodyStateFact{{
			ConcernKey: "region:neck", BodyRegion: "颈肩", Value: "久坐后酸胀",
		}},
	}
	terms := BuildHistoricalContextTerms("最近抬头时也不舒服，但是睡眠还好", snapshot)
	want := map[string]bool{
		"最近抬头时也不舒服":   true,
		"颈肩":          true,
		"久坐后酸胀":       true,
		"region:neck": true,
	}
	for value := range want {
		found := false
		for _, term := range terms {
			if term == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected term %q in %#v", value, terms)
		}
	}
}
