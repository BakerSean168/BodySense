package model

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeReportedDiscomfortAreas(t *testing.T) {
	got, err := NormalizeReportedDiscomfortAreas([]string{"leg", "neck", "lower_back"})
	if err != nil {
		t.Fatalf("valid areas: %v", err)
	}
	if want := []string{"neck", "lower_back", "leg"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical order: got %v want %v", got, want)
	}
	if got, err := NormalizeReportedDiscomfortAreas([]string{"none"}); err != nil || !reflect.DeepEqual(got, []string{"none"}) {
		t.Fatalf("explicit none: got %v err %v", got, err)
	}
	for name, codes := range map[string][]string{
		"empty":          {},
		"unknown":        {"knee"},
		"duplicate":      {"neck", "neck"},
		"none with area": {"none", "hip"},
		"diagnosis text": {"herniated disc"},
	} {
		if _, err := NormalizeReportedDiscomfortAreas(codes); !errors.Is(err, ErrInvalidReportedDiscomfort) {
			t.Fatalf("%s: err=%v, want ErrInvalidReportedDiscomfort", name, err)
		}
	}
}
