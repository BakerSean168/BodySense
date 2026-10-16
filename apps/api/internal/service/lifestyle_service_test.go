package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestLifestyleNativeUserEditsPreserveOmittedSectionsAndClearExplicitly(t *testing.T) {
	store := &memoryBodyContextStore{}
	svc := NewLifestyleService(store)
	owner := uuid.New()
	first, err := svc.Update(context.Background(), owner, UpdateLifestyleRequest{Sleep: &LifestyleSectionInput{Summary: "轮班，睡 6-7 小时"}, Activity: &LifestyleSectionInput{Summary: "每天步行"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Sleep.Summary != "轮班，睡 6-7 小时" || first.Activity.FactID == nil {
		t.Fatalf("user statement not stored: %#v", first)
	}
	second, err := svc.Update(context.Background(), owner, UpdateLifestyleRequest{ExpectedRevision: &first.CurrentRevision, Activity: &LifestyleSectionInput{Summary: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Sleep.Summary != first.Sleep.Summary || second.Activity.FactID != nil || second.Activity.Summary != "" {
		t.Fatalf("patch semantics lost: %#v", second)
	}
}
