package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/service"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type validator struct {
	db          *gorm.DB
	bodyRepo    *repository.BodyStateRepository
	body        *service.BodyStateService
	diagnosis   *service.DiagnosisAnalysisService
	treatment   *service.TreatmentService
	training    *service.TrainingService
	treatmentDB *repository.TreatmentRepository
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	flag.Parse()
	if *databaseURL == "" {
		log.Fatal("database-url is required")
	}

	db, err := gorm.Open(postgres.Open(*databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("connect domain validator database: %v", err)
	}
	v := newValidator(db)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	userID, cleanup, err := v.createUser(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	fact, revision, err := v.validateBodyStateSemantics(ctx, userID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("BODY_STATE_SEMANTICS=PASS")

	if err := v.validateBodyRegionIdentity(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Println("BODY_REGION_ID_ROUNDTRIP=PASS")

	analysis, err := v.createDiagnosis(ctx, userID, revision, fact.ID)
	if err != nil {
		log.Fatal(err)
	}
	proposal, err := v.createTreatmentProposal(ctx, userID, revision, analysis)
	if err != nil {
		log.Fatal(err)
	}
	trainingPlan, err := v.validateTreatmentActivationAtomicity(ctx, userID, proposal)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("TREATMENT_ACTIVATION_ATOMICITY=PASS")

	if err := v.validateOutcomeFeedbackAtomicity(ctx, userID, trainingPlan, fact.ID); err != nil {
		log.Fatal(err)
	}
	fmt.Println("OUTCOME_FEEDBACK_ATOMICITY=PASS")
	fmt.Println("DOMAIN_SEMANTICS=PASS")
}

func newValidator(db *gorm.DB) *validator {
	bodyRepo := repository.NewBodyStateRepository(db)
	bodyService := service.NewBodyStateService(bodyRepo).WithBodyRegionIDValidator(
		service.NewCanonicalBodyRegionIDValidator(),
	)
	diagnosisRepo := repository.NewDiagnosisAnalysisRepository(db)
	diagnosisService := service.NewDiagnosisAnalysisService(diagnosisRepo)
	freshnessRepo := repository.NewDiagnosisFreshnessRepository(db)
	freshnessService := service.NewDiagnosisFreshnessService(freshnessRepo, bodyService)
	treatmentRepo := repository.NewTreatmentRepository(db)
	unitOfWork := database.NewTransactionManager(db)
	agentDeploymentPolicy, err := service.NewAgentDeploymentPolicy()
	if err != nil {
		panic(fmt.Sprintf("configure Agent deployment policy: %v", err))
	}
	treatmentService := service.NewTreatmentService(
		treatmentRepo,
		diagnosisService,
		bodyService,
		freshnessService,
		nil,
		nil,
		unitOfWork,
		agentDeploymentPolicy,
	)
	trainingService := service.NewTrainingService(
		repository.NewTrainingRepository(db),
		treatmentService,
		unitOfWork,
	)
	return &validator{
		db: db, bodyRepo: bodyRepo, body: bodyService,
		diagnosis: diagnosisService, treatment: treatmentService,
		training: trainingService, treatmentDB: treatmentRepo,
	}
}

func (v *validator) createUser(ctx context.Context) (uuid.UUID, func(), error) {
	user := model.User{
		ID:           uuid.New(),
		Email:        fmt.Sprintf("domain-validator-%s@example.invalid", uuid.NewString()),
		PasswordHash: "domain-validator-only",
	}
	if err := v.db.WithContext(ctx).Create(&user).Error; err != nil {
		return uuid.Nil, nil, fmt.Errorf("create validator user: %w", err)
	}
	cleanup := func() {
		_ = v.db.WithContext(context.Background()).Delete(&model.User{}, "id = ?", user.ID).Error
	}
	return user.ID, cleanup, nil
}

func (v *validator) validateBodyStateSemantics(
	ctx context.Context,
	userID uuid.UUID,
) (*model.BodyStateFact, int64, error) {
	expected := int64(0)
	input := model.BodyStateFact{
		ConcernKey:     "region:neck",
		Kind:           "discomfort",
		BodyRegion:     "neck",
		Value:          "discomfort after prolonged sitting",
		Details:        datatypes.JSON(`{"severity":"mild","trigger":"sitting"}`),
		Origin:         "user_reported",
		ReviewState:    "confirmed",
		LifecycleState: "active",
		Trend:          "stable",
		SourceKey:      "domain-validator:fact:neck",
		Provenance:     datatypes.JSON(`{"source_type":"domain_validator"}`),
	}
	fact, firstRevision, err := v.bodyRepo.UpsertFact(ctx, userID, &expected, input, "domain_validator")
	if err != nil {
		return nil, 0, fmt.Errorf("create initial fact: %w", err)
	}
	if firstRevision == nil || firstRevision.Revision != 1 {
		return nil, 0, fmt.Errorf("expected initial BodyState revision 1, got %#v", firstRevision)
	}

	replayed, duplicateRevision, err := v.bodyRepo.UpsertFact(ctx, userID, nil, input, "domain_validator")
	if err != nil {
		return nil, 0, fmt.Errorf("replay fact: %w", err)
	}
	if replayed.ID != fact.ID || duplicateRevision != nil {
		return nil, 0, fmt.Errorf("source-key replay created a semantic change: fact=%s replay=%s revision=%#v", fact.ID, replayed.ID, duplicateRevision)
	}

	staleExpected := int64(0)
	_, _, err = v.bodyRepo.UpsertFact(ctx, userID, &staleExpected, model.BodyStateFact{
		ConcernKey: "region:sleep", Kind: "lifestyle", Value: "late bedtime",
		Origin: "user_reported", ReviewState: "confirmed", LifecycleState: "active",
		Trend: "unknown", SourceKey: "domain-validator:fact:stale",
	}, "domain_validator")
	if !errors.Is(err, repository.ErrBodyStateRevisionConflict) {
		return nil, 0, fmt.Errorf("stale revision did not conflict: %v", err)
	}

	expected = 1
	temporal, temporalRevision, err := v.bodyRepo.UpdateFactTemporal(
		ctx, userID, &expected, fact.ID, "active", "improving", nil, "domain_validator",
	)
	if err != nil {
		return nil, 0, fmt.Errorf("update fact temporal state: %w", err)
	}
	if temporal.ID != fact.ID || temporalRevision == nil || temporalRevision.Revision != 2 || temporal.Trend != "improving" {
		return nil, 0, fmt.Errorf("temporal change rewrote identity or revision: fact=%#v revision=%#v", temporal, temporalRevision)
	}

	expected = 2
	replacement, correctionRevision, err := v.bodyRepo.CorrectFact(
		ctx,
		userID,
		&expected,
		fact.ID,
		model.BodyStateFact{
			ConcernKey: "region:neck", Kind: "discomfort", BodyRegion: "left neck",
			Value:   "left-sided discomfort after prolonged sitting",
			Details: datatypes.JSON(`{"severity":"mild","trigger":"sitting"}`),
			Origin:  "user_edited", ReviewState: "confirmed", LifecycleState: "active",
			Trend: "stable", SourceKey: "domain-validator:fact:neck:corrected",
			Provenance: datatypes.JSON(`{"source_type":"domain_validator"}`),
		},
		"domain_validator",
	)
	if err != nil {
		return nil, 0, fmt.Errorf("correct fact: %w", err)
	}
	if replacement.ID == fact.ID || replacement.SupersedesFactID == nil || *replacement.SupersedesFactID != fact.ID || correctionRevision == nil || correctionRevision.Revision != 3 {
		return nil, 0, fmt.Errorf("correction did not preserve historical identity: replacement=%#v revision=%#v", replacement, correctionRevision)
	}
	var previous model.BodyStateFact
	if err := v.db.WithContext(ctx).Where("id = ?", fact.ID).First(&previous).Error; err != nil {
		return nil, 0, fmt.Errorf("reload corrected fact: %w", err)
	}
	if previous.LifecycleState != "inactive" || previous.ReviewState != "corrected" {
		return nil, 0, fmt.Errorf("corrected fact remained current: %#v", previous)
	}
	return replacement, correctionRevision.Revision, nil
}

func (v *validator) validateBodyRegionIdentity(ctx context.Context) error {
	userID, cleanup, err := v.createUser(ctx)
	if err != nil {
		return fmt.Errorf("create body-region validator user: %w", err)
	}
	defer cleanup()

	expected := int64(0)
	legacy, legacyRevision, err := v.bodyRepo.UpsertFact(ctx, userID, &expected, model.BodyStateFact{
		ConcernKey: "region:legacy", Kind: "discomfort", BodyRegion: "肩颈", Value: "tightness",
		Origin: "user_reported", ReviewState: "confirmed", LifecycleState: "active", Trend: "stable",
		SourceKey: "domain-validator:body-region:legacy",
	}, "domain_validator")
	if err != nil {
		return fmt.Errorf("persist legacy body region: %w", err)
	}
	if legacyRevision == nil || legacyRevision.Revision != 1 || legacy.BodyRegionID != nil {
		return fmt.Errorf("legacy body region must remain readable with null canonical id: fact=%#v revision=%#v", legacy, legacyRevision)
	}

	rightID := "shoulder.right"
	expected = 1
	right, rightRevision, err := v.bodyRepo.UpsertFact(ctx, userID, &expected, model.BodyStateFact{
		ConcernKey: "region:shoulder", Kind: "discomfort", BodyRegion: "右肩", BodyRegionID: &rightID,
		Value: "pain when raising arm", Origin: "user_reported", ReviewState: "confirmed",
		LifecycleState: "active", Trend: "stable", SourceKey: "domain-validator:body-region:right-shoulder",
	}, "domain_validator")
	if err != nil {
		return fmt.Errorf("persist canonical right shoulder: %w", err)
	}
	if rightRevision == nil || rightRevision.Revision != 2 || right.BodyRegionID == nil || *right.BodyRegionID != rightID {
		return fmt.Errorf("canonical right shoulder did not round-trip: fact=%#v revision=%#v", right, rightRevision)
	}

	// Simulate an older source-key producer that does not know body_region_id.
	// The unchanged display region must retain the already-known canonical ID and
	// must not create a meaningless semantic revision.
	replayed, replayRevision, err := v.bodyRepo.UpsertFact(ctx, userID, nil, model.BodyStateFact{
		ConcernKey: "region:shoulder", Kind: "discomfort", BodyRegion: "右肩",
		Value: "pain when raising arm", Origin: "user_reported", ReviewState: "confirmed",
		LifecycleState: "active", Trend: "stable", SourceKey: "domain-validator:body-region:right-shoulder",
	}, "domain_validator")
	if err != nil {
		return fmt.Errorf("legacy source-key replay: %w", err)
	}
	if replayRevision != nil || replayed.ID != right.ID || replayed.BodyRegionID == nil || *replayed.BodyRegionID != rightID {
		return fmt.Errorf("legacy replay erased canonical region or changed revision: fact=%#v revision=%#v", replayed, replayRevision)
	}

	expected = 2
	temporal, temporalRevision, err := v.bodyRepo.UpdateFactTemporal(
		ctx, userID, &expected, right.ID, "active", "improving", nil, "domain_validator",
	)
	if err != nil {
		return fmt.Errorf("temporal change with canonical region: %w", err)
	}
	if temporalRevision == nil || temporalRevision.Revision != 3 || temporal.ID != right.ID || temporal.BodyRegionID == nil || *temporal.BodyRegionID != rightID {
		return fmt.Errorf("temporal change lost historical region identity: fact=%#v revision=%#v", temporal, temporalRevision)
	}

	expected = 3
	retained, retainedRevision, err := v.bodyRepo.CorrectFact(ctx, userID, &expected, right.ID, model.BodyStateFact{
		ConcernKey: "region:shoulder", Kind: "discomfort", BodyRegion: "右肩",
		Value: "pain only above shoulder height", Origin: "user_edited", ReviewState: "confirmed",
		LifecycleState: "active", Trend: "stable", SourceKey: "domain-validator:body-region:right-shoulder:wording",
	}, "domain_validator")
	if err != nil {
		return fmt.Errorf("correction retaining canonical region: %w", err)
	}
	if retainedRevision == nil || retainedRevision.Revision != 4 || retained.BodyRegionID == nil || *retained.BodyRegionID != rightID {
		return fmt.Errorf("same-region correction did not retain canonical identity: fact=%#v revision=%#v", retained, retainedRevision)
	}

	leftID := "shoulder.left"
	expected = 4
	corrected, correctionRevision, err := v.bodyRepo.CorrectFact(ctx, userID, &expected, retained.ID, model.BodyStateFact{
		ConcernKey: "region:shoulder", Kind: "discomfort", BodyRegion: "左肩", BodyRegionID: &leftID,
		Value: "pain only above shoulder height", Origin: "user_edited", ReviewState: "confirmed",
		LifecycleState: "active", Trend: "stable", SourceKey: "domain-validator:body-region:left-shoulder",
	}, "domain_validator")
	if err != nil {
		return fmt.Errorf("correction replacing canonical region: %w", err)
	}
	if correctionRevision == nil || correctionRevision.Revision != 5 || corrected.BodyRegionID == nil || *corrected.BodyRegionID != leftID {
		return fmt.Errorf("laterality correction did not replace canonical identity: fact=%#v revision=%#v", corrected, correctionRevision)
	}
	if corrected.SupersedesFactID == nil || *corrected.SupersedesFactID != retained.ID {
		return fmt.Errorf("laterality correction lost correction history: %#v", corrected)
	}

	var previous model.BodyStateFact
	if err := v.db.WithContext(ctx).Where("id = ?", retained.ID).First(&previous).Error; err != nil {
		return fmt.Errorf("reload superseded right-shoulder fact: %w", err)
	}
	if previous.BodyRegionID == nil || *previous.BodyRegionID != rightID || previous.LifecycleState != "inactive" || previous.ReviewState != "corrected" {
		return fmt.Errorf("correction rewrote historical right-shoulder identity: %#v", previous)
	}

	state, err := v.bodyRepo.GetCurrent(ctx, userID)
	if err != nil {
		return fmt.Errorf("reload body-region projection: %w", err)
	}
	var sawLegacy, sawLeft bool
	for _, fact := range state.Facts {
		if fact.ID == legacy.ID {
			sawLegacy = fact.BodyRegionID == nil && fact.BodyRegion == "肩颈"
		}
		if fact.ID == corrected.ID {
			sawLeft = fact.BodyRegionID != nil && *fact.BodyRegionID == leftID && fact.BodyRegion == "左肩"
		}
	}
	if !sawLegacy || !sawLeft {
		return fmt.Errorf("current projection lost optional canonical region contract: legacy=%v left=%v state=%#v", sawLegacy, sawLeft, state.Facts)
	}
	return nil
}

func (v *validator) createDiagnosis(
	ctx context.Context,
	userID uuid.UUID,
	bodyStateRevision int64,
	factID uuid.UUID,
) (*model.DiagnosisAnalysisRecord, error) {
	raw := json.RawMessage(fmt.Sprintf(`{
		"status":"completed",
		"scope":"full_body",
		"summary":"domain validator analysis",
		"candidates":[{
			"concern_key":"region:neck",
			"name":"load-related neck pattern",
			"confidence":"中",
			"severity":"轻度",
			"evidence_strength":"中",
			"impact":"reduced comfort after prolonged sitting",
			"basis":"confirmed BodyState fact",
			"typical_symptoms":"discomfort after prolonged sitting",
			"differential":"continue temporal review",
			"basis_fact_ids":[%q],
			"basis_observation_ids":[],
			"supporting_evidence_ids":[],
			"counterevidence_ids":[],
			"reasoning_summary":"validation candidate",
			"missing_information":[],
			"safety_notes":[]
		}],
		"cross_concern_patterns":[],
		"information_gaps":[],
		"safety_summary":{},
		"citations":[],
		"governance":{"kind":"diagnosis","verdict":"accepted"}
	}`, factID.String()))
	var fixture struct {
		Candidates []struct {
			Name       string `json:"name"`
			Confidence string `json:"confidence"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Candidates) != 1 || fixture.Candidates[0].Name == "" || fixture.Candidates[0].Confidence == "" {
		return nil, fmt.Errorf("invalid validator diagnosis fixture: %w", err)
	}
	analysis, err := v.diagnosis.PersistAIResult(ctx, userID, bodyStateRevision, raw)
	if err != nil {
		return nil, fmt.Errorf("persist validator diagnosis: %w", err)
	}
	if len(analysis.Candidates) != 1 {
		return nil, fmt.Errorf("expected one diagnosis candidate, got %d", len(analysis.Candidates))
	}
	if _, err := v.diagnosis.AssessCandidates(ctx, userID, analysis.ID, map[uuid.UUID]string{
		analysis.Candidates[0].ID: "confirmed",
	}); err != nil {
		return nil, fmt.Errorf("assess validator diagnosis: %w", err)
	}
	return analysis, nil
}

func (v *validator) createTreatmentProposal(
	ctx context.Context,
	userID uuid.UUID,
	bodyStateRevision int64,
	analysis *model.DiagnosisAnalysisRecord,
) (*model.TreatmentRevision, error) {
	plan := model.TreatmentPlanContent{
		Summary:       "transactional training projection",
		Goal:          "reduce sitting-related neck load",
		DurationWeeks: 4,
		Interventions: []model.TreatmentInterventionDraft{{
			Kind: "exercise", Title: "gentle chin tuck", Description: "controlled range",
			Prescription: map[string]any{"sets": 2, "reps": 8},
		}},
		DailyHabits:      []string{"take short movement breaks"},
		ExpectedTimeline: "2-4 weeks",
		WarningSigns:     []string{"progressive weakness or numbness"},
		ReviewTriggers:   []string{"symptoms worsen"},
	}
	planJSON, _ := json.Marshal(plan)
	_, proposal, err := v.treatmentDB.CreateProposal(ctx, userID, model.TreatmentRevision{
		SourceBodyStateRevision:   bodyStateRevision,
		SourceDiagnosisAnalysisID: analysis.ID,
		Goal:                      plan.Goal,
		DurationWeeks:             plan.DurationWeeks,
		Plan:                      datatypes.JSON(planJSON),
		UserConstraints:           datatypes.JSON(`{}`),
		EvidenceIDs:               datatypes.JSON(`[]`),
		Governance:                datatypes.JSON(`{"kind":"treatment","verdict":"accepted"}`),
		ChangeReason:              "domain validator",
	}, []model.Intervention{{
		Kind: "exercise", Title: "gentle chin tuck", Description: "controlled range",
		Prescription: datatypes.JSON(`{"sets":2,"reps":8}`),
	}})
	if err != nil {
		return nil, fmt.Errorf("create treatment proposal: %w", err)
	}
	return proposal, nil
}

func (v *validator) validateTreatmentActivationAtomicity(
	ctx context.Context,
	userID uuid.UUID,
	proposal *model.TreatmentRevision,
) (*model.TrainingPlan, error) {
	probeRollback := errors.New("validator acceptance probe rollback")
	probeErr := database.NewTransactionManager(v.db).WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := v.treatment.AcceptProposal(txCtx, userID, proposal.ID); err != nil {
			return fmt.Errorf("acceptance preflight: %w", err)
		}
		return probeRollback
	})
	if !errors.Is(probeErr, probeRollback) {
		return nil, fmt.Errorf("treatment acceptance preflight failed: %w", probeErr)
	}
	if err := v.installTrainingFailureTrigger(ctx, proposal.ID); err != nil {
		return nil, err
	}
	_, _, err := v.training.AcceptTreatmentAndEnsurePlan(ctx, userID, proposal.ID, nil)
	if err == nil || !errors.Is(err, service.ErrTrainingProjectionFailed) {
		return nil, fmt.Errorf("forced TrainingPlan failure did not fail activation atomically: %v", err)
	}
	stored, err := v.treatmentDB.GetRevision(ctx, userID, proposal.ID)
	if err != nil {
		return nil, fmt.Errorf("reload proposal after rollback: %w", err)
	}
	if stored == nil || stored.AcceptanceState != model.TreatmentAcceptanceProposed {
		return nil, fmt.Errorf("Treatment acceptance escaped failed projection transaction: %#v", stored)
	}
	var planCount int64
	if err := v.db.WithContext(ctx).Model(&model.TrainingPlan{}).
		Where("treatment_revision_id = ?", proposal.ID).
		Count(&planCount).Error; err != nil {
		return nil, fmt.Errorf("count rolled-back training plans: %w", err)
	}
	if planCount != 0 {
		return nil, fmt.Errorf("failed activation left %d TrainingPlan rows", planCount)
	}
	if err := v.removeFailureTrigger(ctx, "domain_validator_fail_training", "training_plans"); err != nil {
		return nil, err
	}

	_, plan, err := v.training.AcceptTreatmentAndEnsurePlan(ctx, userID, proposal.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("retry treatment activation: %w", err)
	}
	if plan == nil || plan.TreatmentRevisionID == nil || *plan.TreatmentRevisionID != proposal.ID {
		return nil, fmt.Errorf("retry did not create pinned TrainingPlan: %#v", plan)
	}
	return plan, nil
}

func (v *validator) validateOutcomeFeedbackAtomicity(
	ctx context.Context,
	userID uuid.UUID,
	plan *model.TrainingPlan,
	factID uuid.UUID,
) error {
	if err := v.installOutcomeFailureTrigger(ctx); err != nil {
		return err
	}
	sourceKey := "domain-validator:outcome:retry"
	outcome := model.Outcome{
		TreatmentID:         plan.TreatmentID,
		TreatmentRevisionID: plan.TreatmentRevisionID,
		SourceType:          "domain_validator",
		SourceKey:           sourceKey,
		Kind:                "symptom_change",
		ConcernKey:          "region:neck",
		BodyRegion:          "left neck",
		Value:               datatypes.JSON([]byte(fmt.Sprintf(`{"description":"improving","trend":"improving","fact_id":%q}`, factID.String()))),
		Notes:               "forced transactional retry",
		CausalityLevel:      "association_only",
		OccurredAt:          time.Now().UTC(),
		Provenance:          datatypes.JSON(`{"source_type":"domain_validator"}`),
	}
	if _, _, err := v.treatment.RecordOutcome(ctx, userID, outcome); err == nil {
		return errors.New("forced BodyState projection failure unexpectedly succeeded")
	}
	var outcomeCount int64
	if err := v.db.WithContext(ctx).Model(&model.Outcome{}).
		Where("user_id = ? AND source_type = ? AND source_key = ?", userID, outcome.SourceType, sourceKey).
		Count(&outcomeCount).Error; err != nil {
		return fmt.Errorf("count rolled-back outcomes: %w", err)
	}
	if outcomeCount != 0 {
		return fmt.Errorf("failed BodyState projection left %d Outcome rows", outcomeCount)
	}
	if err := v.removeFailureTrigger(ctx, "domain_validator_fail_outcome", "body_state_revisions"); err != nil {
		return err
	}

	stored, created, err := v.treatment.RecordOutcome(ctx, userID, outcome)
	if err != nil {
		return fmt.Errorf("retry outcome projection: %w", err)
	}
	if !created || stored == nil || stored.BodyStateRevision == nil {
		return fmt.Errorf("retry did not atomically link Outcome to BodyState: stored=%#v created=%v", stored, created)
	}
	var linked model.Outcome
	if err := v.db.WithContext(ctx).Where("id = ?", stored.ID).First(&linked).Error; err != nil {
		return fmt.Errorf("reload linked Outcome: %w", err)
	}
	if linked.BodyStateRevision == nil || *linked.BodyStateRevision != *stored.BodyStateRevision {
		return fmt.Errorf("persisted Outcome lost BodyState revision: %#v", linked)
	}
	return nil
}

func (v *validator) installTrainingFailureTrigger(ctx context.Context, revisionID uuid.UUID) error {
	return v.db.WithContext(ctx).Exec(fmt.Sprintf(`
CREATE OR REPLACE FUNCTION domain_validator_fail_training() RETURNS trigger AS $$
BEGIN
    IF NEW.treatment_revision_id = '%s'::uuid THEN
        RAISE EXCEPTION 'forced training projection failure';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS domain_validator_fail_training ON training_plans;
CREATE TRIGGER domain_validator_fail_training
    BEFORE INSERT ON training_plans
    FOR EACH ROW EXECUTE FUNCTION domain_validator_fail_training();`, revisionID)).Error
}

func (v *validator) installOutcomeFailureTrigger(ctx context.Context) error {
	return v.db.WithContext(ctx).Exec(`
CREATE OR REPLACE FUNCTION domain_validator_fail_outcome() RETURNS trigger AS $$
BEGIN
    IF NEW.source = 'outcome' THEN
        RAISE EXCEPTION 'forced outcome projection failure';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS domain_validator_fail_outcome ON body_state_revisions;
CREATE TRIGGER domain_validator_fail_outcome
    BEFORE INSERT ON body_state_revisions
    FOR EACH ROW EXECUTE FUNCTION domain_validator_fail_outcome();`).Error
}

func (v *validator) removeFailureTrigger(ctx context.Context, functionName, tableName string) error {
	statement := fmt.Sprintf(
		"DROP TRIGGER IF EXISTS %s ON %s; DROP FUNCTION IF EXISTS %s() CASCADE;",
		functionName, tableName, functionName,
	)
	if err := v.db.WithContext(ctx).Exec(statement).Error; err != nil {
		return fmt.Errorf("remove validator failure trigger %s: %w", functionName, err)
	}
	return nil
}
