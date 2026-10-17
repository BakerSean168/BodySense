package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSafetyRestrictionHTTPRealVersionOwnerAndReceiptPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PG18")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	owner, foreign := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{owner, foreign} {
		if err := db.Exec("INSERT INTO users(id,email,password_hash) VALUES(?,?,?)", id, id.String()+"@example.test", "synthetic").Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Exec("DELETE FROM users WHERE id=?", id).Error })
	}
	repo := repository.NewBodyStateRepository(db)
	facts, err := repo.RecordDirectFacts(context.Background(), owner, model.RecordBodyFactsCommand{IdempotencyKey: "facts", Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "synthetic symptom", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(facts.FactIDs, &ids); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: "0123456789abcdef01234567", BodyFactID: ids[0], SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(), SourceToolCallID: "intake-0123456789abcdef01234567", Numbness: true, BodyStateRevision: 1, RecordedAt: now}
	if err := db.Create(&screen).Error; err != nil {
		t.Fatal(err)
	}
	target := model.SafetyRestriction{ID: uuid.New(), UserID: owner, SafetyScreenID: screen.ID, Concept: "numbness", PolicyRevision: model.SafetyScreenRestrictionPolicyV1, RiskLevel: "restricted", ScopeKind: "all_training", LiftPolicy: "professional_clearance", Reason: "synthetic", CreatedRevision: 1, ValidFrom: now}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	conversation := model.Conversation{ID: uuid.New(), UserID: owner, Status: "active"}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	run := model.Run{ID: uuid.New(), UserID: owner, ConversationID: conversation.ID, TurnID: uuid.New(), RequestID: uuid.NewString(), Model: "synthetic", Status: model.RunStatusRunning}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	parts, _ := json.Marshal([]map[string]string{{"type": "text", "text": model.ProfessionalClearanceStatement}})
	message := model.Message{ID: uuid.New(), ConversationID: conversation.ID, TurnID: run.TurnID, Role: "user", Status: "completed", Seq: 1, Parts: datatypes.JSON(parts)}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	router := newOpenAPITestRouterWith(t, NewPublicServer().WithSafetyRestrictions(repo), owner.String())
	path := "/api/v1/body-state/safety/restrictions/" + target.ID.String() + "/resolve"
	command := model.ResolveSafetyRestrictionCommand{IdempotencyKey: "lift", ExpectedRevision: 1, ExpectedRestrictionVersion: 2, BasisKind: "professional_clearance", SourceVersion: 1, SourceMessageID: message.ID, Statement: model.ProfessionalClearanceStatement}
	post := func(command model.ResolveSafetyRestrictionCommand) (int, string) {
		body, _ := json.Marshal(command)
		res := performJSONAt(router, http.MethodPost, path, string(body))
		return res.Code, res.Body.String()
	}
	// Go commands contain zero UUIDs for inapplicable sources. The transport
	// accepts those as absent and never treats them as actual evidence.
	if code, body := post(command); code != 409 {
		t.Fatalf("stale restriction: %d %s", code, body)
	}
	foreignRouter := newOpenAPITestRouterWith(t, NewPublicServer().WithSafetyRestrictions(repo), foreign.String())
	body, _ := json.Marshal(command)
	if res := performJSONAt(foreignRouter, http.MethodPost, path, string(body)); res.Code != 404 {
		t.Fatalf("foreign: %d %s", res.Code, res.Body.String())
	}
	command.ExpectedRestrictionVersion = 1
	code, first := post(command)
	if code != 200 {
		t.Fatalf("commit: %d %s", code, first)
	}
	var receipt model.SafetyResolution
	if err := json.Unmarshal([]byte(first), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.RestrictionVersion != 1 || receipt.SourceVersion != 1 || receipt.SourceMessageID == nil || *receipt.SourceMessageID != message.ID {
		t.Fatalf("wrong receipt %+v", receipt)
	}
	code, replay := post(command)
	if code != 200 || replay != first {
		t.Fatalf("receipt replay changed %d %s", code, replay)
	}
	command.IdempotencyKey = "other"
	if code, _ := post(command); code != 409 {
		t.Fatalf("repeat target=%d", code)
	}
	if res := performJSONAt(router, http.MethodPost, path, fmt.Sprintf(`{"basis_kind":"expiry","idempotency_key":"fake-time","expected_revision":2,"expected_restriction_version":1,"now":%q}`, time.Now().Add(24*time.Hour).Format(time.RFC3339))); res.Code != 400 {
		t.Fatal("client supplied expiry time accepted")
	}
	if strings.Contains(first, model.ProfessionalClearanceStatement) {
		t.Fatal("receipt leaked source text")
	}
}
