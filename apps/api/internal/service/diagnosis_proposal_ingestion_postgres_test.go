package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func diagnosisAgentProposalFor(f diagnosisDecisionFixture, name string) map[string]any {
	return map[string]any{
		"changeKind":      "create",
		"medicalName":     name,
		"assessment":      map[string]any{"level": "suspected", "rationale": "Synthetic AI judgment, not a clinical validation", "informationGaps": []string{}, "differentialNotes": []string{}},
		"causeAnalysis":   map[string]any{"summary": "Causality not established", "rationale": "Synthetic reasoning only"},
		"informationGaps": []string{"Synthetic gap"},
		"handlingPlan":    map[string]any{"summary": "Seek human clinical review", "actions": []string{}, "cautionNotes": []string{}},
		"evidence":        []map[string]any{{"evidenceType": "body_fact", "evidenceId": f.fact.ID, "role": "supporting", "rationale": "Synthetic grounding"}},
	}
}
func diagnosisAgentOutputFor(f diagnosisDecisionFixture, names ...string) map[string]any {
	proposals := []map[string]any{}
	for _, n := range names {
		proposals = append(proposals, diagnosisAgentProposalFor(f, n))
	}
	return map[string]any{"schemaRevision": DiagnosisAgentProposalsSchemaV1, "outcome": "proposals", "summary": "Synthetic", "proposals": proposals}
}
func diagnosisIngestionCommandFor(t *testing.T, f diagnosisDecisionFixture, output any) DiagnosisProposalIngestion {
	t.Helper()
	raw, ok := output.(json.RawMessage)
	if !ok {
		var err error
		if raw, err = json.Marshal(output); err != nil {
			t.Fatal(err)
		}
	}
	return DiagnosisProposalIngestion{
		RequestEvidenceID: f.request.ID, Trigger: model.ConditionNewAnalysis, ScopeDescription: "Synthetic formal request",
		ExpectedRevision: 0, IdempotencyKey: uuid.NewString(), BodyStateRevision: 1,
		InputEvidenceRefs:   []model.DiagnosisEvidenceRef{{EvidenceType: model.DiagnosisBodyFact, EvidenceID: f.fact.ID, Version: "1"}},
		ReplayInput:         json.RawMessage(`{"bodyStateRevision":1,"frozen":"synthetic"}`),
		ExecutionProvenance: map[string]any{"provider": "synthetic"}, AgentOutput: raw,
	}
}
func diagnosisRunsForKey(t *testing.T, db *gorm.DB, owner uuid.UUID, key string) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.ConditionAnalysisRun{}).Where("user_id=? AND idempotency_key=?", owner, key).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
func assertNoAgentAuthority(t *testing.T, db *gorm.DB, owner uuid.UUID, facts int64) {
	t.Helper()
	assertDiagnosisNotAdopted(t, db, owner)
	diagnosisDecisionCount(t, db, owner, "diagnosis_decisions", 0)
	diagnosisDecisionCount(t, db, owner, "condition_entry_evidence", 0)
	diagnosisDecisionCount(t, db, owner, "body_facts", facts)
}

func TestDiagnosisProposalIngestionPersistsProposalsOnlyPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalIngestionService(db)
	cmd := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome", "Synthetic B syndrome"))
	receipt, err := svc.Ingest(ctx, f.owner, cmd)
	if err != nil || receipt == nil || receipt.Outcome != "proposals" || len(receipt.ProposalIDs) != 2 || receipt.Replayed {
		t.Fatalf("ingest %+v %v", receipt, err)
	}
	// An AI proposal is never a fact, decision, ConditionEntry or revision.
	assertNoAgentAuthority(t, db, f.owner, 1)
	run, err := repository.NewDiagnosisRepository(db).GetAnalysisRun(ctx, f.owner, receipt.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.RequestEvidenceID != f.request.ID || run.RequestMessageID != *f.request.SourceMessageID || run.RequestSourceVersion != 1 ||
		run.BasedOnBodyStateRevision != 1 || run.BasedOnDiagnosisRevision != 0 || len(run.AgentHypotheses) != 2 || len(run.InputEvidenceRefs) != 1 {
		t.Fatalf("run base %+v", run)
	}
	if !diagnosisJSONEqual(json.RawMessage(run.ReplayInput), cmd.ReplayInput) {
		t.Fatalf("replay input not frozen: %s", run.ReplayInput)
	}
	var provenance map[string]any
	if err := json.Unmarshal(run.ExecutionProvenance, &provenance); err != nil || provenance["replayInputDigest"] != DiagnosisReplayInputDigest(cmd.ReplayInput) ||
		provenance["agentOutputSchemaRevision"] != DiagnosisAgentProposalsSchemaV1 || provenance["provider"] != "synthetic" {
		t.Fatalf("provenance %v %v", provenance, err)
	}
	for _, id := range receipt.ProposalIDs {
		p, err := repository.NewDiagnosisRepository(db).GetProposal(ctx, f.owner, id)
		if err != nil {
			t.Fatal(err)
		}
		fp, err := model.DiagnosisProposalContentFingerprint(p)
		if err != nil || fp != p.ContentFingerprint || p.BaseRevision != 0 || p.ChangeKind != model.DiagnosisCreate || p.RunID != run.ID || len(p.EvidenceRefs) != 1 || p.EvidenceRefs[0].Version != "1" {
			t.Fatalf("proposal %+v fp=%s %v", p, fp, err)
		}
	}
	projection, err := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).Read(ctx, f.owner, run.ID)
	if err != nil || projection.State != "open" || len(projection.Proposals) != 2 || len(projection.Events) != 3 ||
		projection.Events[0].Kind != "run_started" || projection.Proposals[0].State != "pending" {
		t.Fatalf("projection %+v %v", projection, err)
	}
	// Adoption still requires the user's explicit choices and the settlement path.
	decisions := NewDiagnosisProposalDecisionService(db)
	for _, p := range projection.Proposals {
		if _, err := decisions.Record(ctx, f.owner, DiagnosisProposalDecisionCommand{RunID: run.ID, ProposalID: p.ProposalID, PresentedFingerprint: p.ContentFingerprint, ExpectedRevision: 0, Decision: model.DiagnosisAccept, ToolCallID: "explicit-user-confirmation", IdempotencyKey: uuid.NewString()}); err != nil {
			t.Fatal(err)
		}
	}
	assertDiagnosisNotAdopted(t, db, f.owner)
	settled, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, DiagnosisSettlementCommand{RunID: run.ID, ExpectedRevision: 0, ToolCallID: "user-close", IdempotencyKey: uuid.NewString()})
	if err != nil || settled.Outcome != "committed" || len(settled.ConditionIDs) != 2 {
		t.Fatalf("settle ingested proposals %+v %v", settled, err)
	}
	diagnosisDecisionCount(t, db, f.owner, "condition_entries", 2)
}

func TestDiagnosisProposalIngestionIdempotentReplayPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalIngestionService(db)
	cmd := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
	first, err := svc.Ingest(ctx, f.owner, cmd)
	if err != nil {
		t.Fatal(err)
	}
	// A retried command with a re-generated (different) Agent output replays
	// the original immutable run; it never writes a second run or proposal set.
	retry := cmd
	retry.AgentOutput, _ = json.Marshal(diagnosisAgentOutputFor(f, "Different syndrome", "Another syndrome"))
	again, err := svc.Ingest(ctx, f.owner, retry)
	if err != nil || !again.Replayed || again.RunID != first.RunID || len(again.ProposalIDs) != 1 || again.ProposalIDs[0] != first.ProposalIDs[0] {
		t.Fatalf("replay %+v %v", again, err)
	}
	pre, err := svc.Replay(ctx, f.owner, cmd)
	if err != nil || pre == nil || pre.RunID != first.RunID {
		t.Fatalf("pre-agent replay %+v %v", pre, err)
	}
	if n := diagnosisRunsForKey(t, db, f.owner, cmd.IdempotencyKey); n != 1 {
		t.Fatalf("runs for key %d", n)
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_proposals", 3) // 2 fixture + 1 ingested
	changed := cmd
	changed.ScopeDescription = "A different request body"
	if _, err := svc.Ingest(ctx, f.owner, changed); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("same key, different body: %v", err)
	}
	if _, err := svc.Replay(ctx, f.owner, changed); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("replay conflict: %v", err)
	}
	// Replay precedes staleness: after the head moves the original receipt stands.
	recordSecondFact(t, db, f.owner)
	if late, err := svc.Ingest(ctx, f.owner, cmd); err != nil || late.RunID != first.RunID {
		t.Fatalf("late replay %+v %v", late, err)
	}
}

func recordSecondFact(t *testing.T, db *gorm.DB, owner uuid.UUID) {
	t.Helper()
	if _, err := repository.NewBodyStateRepository(db).RecordDirectFacts(context.Background(), owner, model.RecordBodyFactsCommand{
		IdempotencyKey: uuid.NewString(), ExpectedRevision: 1, Facts: []model.BodyFactInput{{Kind: "symptom", Statement: "Synthetic user reports ankle discomfort", BodyRegionIDs: []string{}, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{"presence":"present"}`)}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosisProposalIngestionOwnerIsolationPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	other := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalIngestionService(db)

	// Another owner's request statement is indistinguishable from a missing one.
	foreignRequest := diagnosisIngestionCommandFor(t, other, diagnosisAgentOutputFor(other, "Synthetic A syndrome"))
	foreignRequest.RequestEvidenceID = f.request.ID
	if _, err := svc.Ingest(ctx, other.owner, foreignRequest); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign request: %v", err)
	}
	if n := diagnosisRunsForKey(t, db, other.owner, foreignRequest.IdempotencyKey); n != 0 {
		t.Fatal("foreign request wrote a run")
	}
	// The Agent cannot cite evidence outside its frozen owner-built input.
	foreignEvidence := diagnosisAgentOutputFor(f, "Synthetic A syndrome")
	foreignEvidence["proposals"].([]map[string]any)[0]["evidence"] = []map[string]any{{"evidenceType": "body_fact", "evidenceId": other.fact.ID, "role": "supporting", "rationale": "x"}}
	cmd := diagnosisIngestionCommandFor(t, f, foreignEvidence)
	if _, err := svc.Ingest(ctx, f.owner, cmd); !errors.Is(err, ErrDiagnosisAgentOutputInvalid) {
		t.Fatalf("foreign agent evidence: %v", err)
	}
	// Even a corrupted frozen input naming a foreign fact fails source qualification.
	corrupt := diagnosisIngestionCommandFor(t, f, foreignEvidence)
	corrupt.InputEvidenceRefs = append(corrupt.InputEvidenceRefs, model.DiagnosisEvidenceRef{EvidenceType: model.DiagnosisBodyFact, EvidenceID: other.fact.ID, Version: "1"})
	if _, err := svc.Ingest(ctx, f.owner, corrupt); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign frozen evidence: %v", err)
	}
	for _, key := range []string{cmd.IdempotencyKey, corrupt.IdempotencyKey} {
		if n := diagnosisRunsForKey(t, db, f.owner, key); n != 0 {
			t.Fatal("rejected ingestion left a partial run")
		}
	}
	// An owner's idempotency key is invisible to another owner.
	mine := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
	if _, err := svc.Ingest(ctx, f.owner, mine); err != nil {
		t.Fatal(err)
	}
	theirs := diagnosisIngestionCommandFor(t, other, diagnosisAgentOutputFor(other, "Synthetic A syndrome"))
	theirs.IdempotencyKey = mine.IdempotencyKey
	r, err := svc.Ingest(ctx, other.owner, theirs)
	if err != nil || r.Replayed {
		t.Fatalf("cross-owner key reuse %+v %v", r, err)
	}
	if _, err := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).Read(ctx, other.owner, r.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).Read(ctx, f.owner, r.RunID); !errors.Is(err, model.ErrDiagnosisNotFound) {
		t.Fatalf("foreign run readable: %v", err)
	}
}

func TestDiagnosisProposalIngestionStaleRevisionPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalIngestionService(db)
	staleDiagnosis := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
	staleDiagnosis.ExpectedRevision = 1
	if _, err := svc.Ingest(ctx, f.owner, staleDiagnosis); !errors.Is(err, ErrDiagnosisDecisionConflict) {
		t.Fatalf("stale diagnosis revision: %v", err)
	}
	recordSecondFact(t, db, f.owner)
	// The Agent reasoned over BodyState R1; R2 now exists, so its output is stale.
	staleBody := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
	if _, err := svc.Ingest(ctx, f.owner, staleBody); !errors.Is(err, ErrDiagnosisIngestionStale) {
		t.Fatalf("stale body revision: %v", err)
	}
	for _, key := range []string{staleDiagnosis.IdempotencyKey, staleBody.IdempotencyKey} {
		if n := diagnosisRunsForKey(t, db, f.owner, key); n != 0 {
			t.Fatal("stale ingestion wrote a run")
		}
	}
	assertNoAgentAuthority(t, db, f.owner, 2)
}

func TestDiagnosisProposalIngestionMalformedOutputPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	f := seedDiagnosisDecisionFixture(t, db)
	svc := NewDiagnosisProposalIngestionService(db)
	mutate := func(fn func(out map[string]any, p map[string]any)) map[string]any {
		out := diagnosisAgentOutputFor(f, "Synthetic A syndrome")
		fn(out, out["proposals"].([]map[string]any)[0])
		return out
	}
	cases := map[string]any{
		"not json":                 json.RawMessage(`{"schemaRevision":`),
		"trailing data":            json.RawMessage(`{"schemaRevision":"diagnosis-agent-proposals-v1","outcome":"insufficient_evidence","summary":"","proposals":[]} {}`),
		"legacy candidate shape":   map[string]any{"status": "completed", "candidates": []any{map[string]any{"name": "x", "confidence": "高"}}},
		"wrong schema":             mutate(func(o, _ map[string]any) { o["schemaRevision"] = "diagnosis-output-v3" }),
		"unknown outcome":          mutate(func(o, _ map[string]any) { o["outcome"] = "adopted" }),
		"proposals outcome empty":  mutate(func(o, _ map[string]any) { o["proposals"] = []any{} }),
		"insufficient with items":  mutate(func(o, _ map[string]any) { o["outcome"] = "insufficient_evidence" }),
		"agent fact injection":     mutate(func(o, _ map[string]any) { o["bodyFacts"] = []any{map[string]any{"statement": "AI-invented"}} }),
		"agent condition entry":    mutate(func(o, _ map[string]any) { o["conditionEntries"] = []any{map[string]any{"medicalName": "x"}} }),
		"agent disposition":        mutate(func(_, p map[string]any) { p["newDisposition"] = "adopted" }),
		"agent chosen owner":       mutate(func(_, p map[string]any) { p["userId"] = uuid.NewString() }),
		"agent chosen run":         mutate(func(_, p map[string]any) { p["assessment"].(map[string]any)["basedOnRunId"] = uuid.NewString() }),
		"agent evidence version":   mutate(func(_, p map[string]any) { p["evidence"].([]map[string]any)[0]["evidenceVersion"] = "9" }),
		"update without authority": mutate(func(_, p map[string]any) { p["changeKind"] = "update" }),
		"user-only level":          mutate(func(_, p map[string]any) { p["assessment"].(map[string]any)["level"] = "confirmed" }),
		"missing handling plan": mutate(func(_, p map[string]any) {
			p["handlingPlan"] = map[string]any{"summary": " ", "actions": []string{}, "cautionNotes": []string{}}
		}),
		"missing cause":          mutate(func(_, p map[string]any) { delete(p, "causeAnalysis") }),
		"empty medical name":     mutate(func(_, p map[string]any) { p["medicalName"] = "  " }),
		"no supporting evidence": mutate(func(_, p map[string]any) { p["evidence"].([]map[string]any)[0]["role"] = "counter" }),
		"no evidence":            mutate(func(_, p map[string]any) { p["evidence"] = []any{} }),
		"unseen evidence":        mutate(func(_, p map[string]any) { p["evidence"].([]map[string]any)[0]["evidenceId"] = uuid.NewString() }),
		"evidence type swap":     mutate(func(_, p map[string]any) { p["evidence"].([]map[string]any)[0]["evidenceType"] = "body_observation" }),
		"bad evidence role":      mutate(func(_, p map[string]any) { p["evidence"].([]map[string]any)[0]["role"] = "proof" }),
		"duplicate names":        diagnosisAgentOutputFor(f, "Synthetic A syndrome", "synthetic  a SYNDROME"),
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := diagnosisIngestionCommandFor(t, f, output)
			if _, err := svc.Ingest(ctx, f.owner, cmd); !errors.Is(err, ErrDiagnosisAgentOutputInvalid) {
				t.Fatalf("accepted malformed output: %v", err)
			}
			if n := diagnosisRunsForKey(t, db, f.owner, cmd.IdempotencyKey); n != 0 {
				t.Fatal("malformed output left a partial run")
			}
		})
	}
	diagnosisDecisionCount(t, db, f.owner, "diagnosis_proposals", 2) // fixture only
	diagnosisDecisionCount(t, db, f.owner, "condition_analysis_runs", 1)
	assertNoAgentAuthority(t, db, f.owner, 1)
}

func TestDiagnosisProposalIngestionTerminalOutcomesPostgres(t *testing.T) {
	db := openOnboardingContextIntegrationDB(t)
	ctx := context.Background()
	t.Run("insufficient evidence is terminal and not choosable", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		svc := NewDiagnosisProposalIngestionService(db)
		cmd := diagnosisIngestionCommandFor(t, f, map[string]any{"schemaRevision": DiagnosisAgentProposalsSchemaV1, "outcome": "insufficient_evidence", "summary": "Need more information", "proposals": []any{}})
		r, err := svc.Ingest(ctx, f.owner, cmd)
		if err != nil || r.Outcome != "insufficient_evidence" || len(r.ProposalIDs) != 0 {
			t.Fatalf("insufficient %+v %v", r, err)
		}
		projection, err := NewNativeDiagnosisRunReadService(repository.NewDiagnosisRepository(db)).Read(ctx, f.owner, r.RunID)
		if err != nil || projection.State != "closed_without_change" || projection.Outcome == nil || *projection.Outcome != "insufficient_evidence" {
			t.Fatalf("insufficient projection %+v %v", projection, err)
		}
		if _, err := NewDiagnosisSettlementService(db).Settle(ctx, f.owner, DiagnosisSettlementCommand{RunID: r.RunID, ToolCallID: "user", IdempotencyKey: uuid.NewString(), CloseRemaining: true}); !errors.Is(err, ErrDiagnosisAnalysisClosed) {
			t.Fatalf("insufficient run settled: %v", err)
		}
		replayed, err := svc.Ingest(ctx, f.owner, cmd)
		if err != nil || !replayed.Replayed || replayed.Outcome != "insufficient_evidence" {
			t.Fatalf("insufficient replay %+v %v", replayed, err)
		}
	})
	t.Run("go safety block persists zero proposals", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		svc := NewDiagnosisProposalIngestionService(db)
		cmd := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
		cmd.GoSafetyBlocked = true
		r, err := svc.Ingest(ctx, f.owner, cmd)
		if !errors.Is(err, ErrDiagnosisSafetyBlocked) || r == nil || r.Outcome != "safety_blocked" || len(r.ProposalIDs) != 0 {
			t.Fatalf("blocked %+v %v", r, err)
		}
		var proposals int64
		if err := db.Model(&model.DiagnosisProposal{}).Where("user_id=? AND run_id=?", f.owner, r.RunID).Count(&proposals).Error; err != nil || proposals != 0 {
			t.Fatalf("blocked run has proposals %d %v", proposals, err)
		}
		assertNoAgentAuthority(t, db, f.owner, 1)
	})
	t.Run("urgent live safety blocks without Go flag", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		svc := NewDiagnosisProposalIngestionService(db)
		svc.projectSafety = func(*BodyStateReadModel) (SafetyEnvelopeV2, error) {
			return SafetyEnvelopeV2{ActiveBlockers: []SafetyBlockerV1{{RiskLevel: "urgent"}}}, nil
		}
		r, err := svc.Ingest(ctx, f.owner, diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome")))
		if !errors.Is(err, ErrDiagnosisSafetyBlocked) || r.Outcome != "safety_blocked" {
			t.Fatalf("urgent %+v %v", r, err)
		}
	})
	t.Run("unavailable safety writes nothing", func(t *testing.T) {
		f := seedDiagnosisDecisionFixture(t, db)
		svc := NewDiagnosisProposalIngestionService(db)
		svc.projectSafety = func(*BodyStateReadModel) (SafetyEnvelopeV2, error) { return SafetyEnvelopeV2{}, nil }
		cmd := diagnosisIngestionCommandFor(t, f, diagnosisAgentOutputFor(f, "Synthetic A syndrome"))
		if _, err := svc.Ingest(ctx, f.owner, cmd); !errors.Is(err, ErrDiagnosisSafetyUnavailable) {
			t.Fatalf("incomplete coverage: %v", err)
		}
		if n := diagnosisRunsForKey(t, db, f.owner, cmd.IdempotencyKey); n != 0 {
			t.Fatal("unavailable Safety left a run")
		}
	})
}
