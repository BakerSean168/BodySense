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
	db       *gorm.DB
	bodyRepo *repository.BodyStateRepository
	body     *service.BodyStateService
	outcomes *service.OutcomeService
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	flag.Parse()

	var db *gorm.DB
	var err error
	if *databaseURL != "" {
		db, err = gorm.Open(postgres.Open(*databaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	} else {
		db, err = database.Connect(database.ConfigFromEnv())
	}
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

	if err := v.seedSyntheticSafetyScreen(ctx, userID, fact.ID, revision); err != nil {
		log.Fatal(err)
	}
	fmt.Println("NATIVE_SAFETY_COVERAGE=PASS")

	if err := v.validateOutcomeFeedbackAtomicity(ctx, userID); err != nil {
		log.Fatal(err)
	}
	fmt.Println("OUTCOME_FEEDBACK_ATOMICITY=PASS")
	fmt.Println("DOMAIN_SEMANTICS=PASS")
}

func newValidator(db *gorm.DB) *validator {
	bodyRepo := repository.NewBodyStateRepository(db)
	bodyService := service.NewBodyStateService(bodyRepo).WithBodyRegionIDValidator(
		service.NewCanonicalBodyRegionIDValidator(),
	).WithAgentContextSources(database.NewTransactionManager(db), repository.NewEquipmentAssetRepository(db))
	unitOfWork := database.NewTransactionManager(db)
	outcomeService := service.NewOutcomeService(repository.NewOutcomeRepository(db), bodyService, unitOfWork)
	return &validator{
		db: db, bodyRepo: bodyRepo, body: bodyService, outcomes: outcomeService,
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

func validatorFact(statement string, regions ...string) model.BodyFactInput {
	return model.BodyFactInput{Kind: "symptom", Statement: statement, BodyRegionIDs: regions, Temporal: model.FactTemporal{Precision: "unknown"}, Details: json.RawMessage(`{}`)}
}

func (v *validator) validateBodyStateSemantics(ctx context.Context, owner uuid.UUID) (*model.BodyFact, int64, error) {
	command := model.RecordBodyFactsCommand{IdempotencyKey: "validator:neck", Facts: []model.BodyFactInput{validatorFact("discomfort after prolonged sitting", "neck")}}
	first, err := v.bodyRepo.RecordDirectFacts(ctx, owner, command)
	if err != nil {
		return nil, 0, err
	}
	replay, err := v.bodyRepo.RecordDirectFacts(ctx, owner, command)
	if err != nil {
		return nil, 0, err
	}
	if first.Revision != 1 || replay.RevisionID != first.RevisionID || string(replay.FactIDs) != string(first.FactIDs) {
		return nil, 0, errors.New("native fact replay changed identity")
	}
	firstID, err := validatorCommitFactID(first)
	if err != nil {
		return nil, 0, err
	}
	replacement := validatorFact("left-sided discomfort after prolonged sitting", "neck")
	corrected, err := v.bodyRepo.CorrectDirectFact(ctx, owner, firstID, model.AmendBodyFactCommand{IdempotencyKey: "validator:correct", ExpectedRevision: 1, Replacement: &replacement})
	if err != nil {
		return nil, 0, err
	}
	var previous model.BodyFact
	if err := v.db.WithContext(ctx).First(&previous, "id = ? AND user_id = ?", firstID, owner).Error; err != nil {
		return nil, 0, err
	}
	correctedID, err := validatorCommitFactID(corrected)
	if err != nil {
		return nil, 0, err
	}
	var current model.BodyFact
	if err := v.db.WithContext(ctx).First(&current, "id = ? AND user_id = ?", correctedID, owner).Error; err != nil {
		return nil, 0, err
	}
	if previous.AssertionStatus != "superseded" || previous.Statement != command.Facts[0].Statement || corrected.Revision != 2 || correctedID == previous.ID {
		return nil, 0, errors.New("native correction lost immutable history")
	}
	return &current, corrected.Revision, nil
}

func (v *validator) validateBodyRegionIdentity(ctx context.Context) error {
	owner, cleanup, err := v.createUser(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	facts := service.NewBodyFactService(v.bodyRepo, service.NewCanonicalBodyRegionIDValidator())
	command := model.RecordBodyFactsCommand{IdempotencyKey: "validator:region", Facts: []model.BodyFactInput{validatorFact("pain when raising arm", "shoulder.middle")}}
	if _, err := facts.RecordDirectFacts(ctx, owner, command); !errors.Is(err, service.ErrUnknownBodyRegionID) {
		return fmt.Errorf("invalid region accepted: %v", err)
	}
	command.Facts[0].BodyRegionIDs = []string{"shoulder.right"}
	first, err := facts.RecordDirectFacts(ctx, owner, command)
	if err != nil {
		return err
	}
	replay, err := facts.RecordDirectFacts(ctx, owner, command)
	if err != nil {
		return err
	}
	if replay.RevisionID != first.RevisionID {
		return errors.New("region replay changed revision")
	}
	firstID, err := validatorCommitFactID(first)
	if err != nil {
		return err
	}
	replacement := validatorFact("pain only above shoulder height", "shoulder.left")
	corrected, err := facts.CorrectDirectFact(ctx, owner, firstID, model.AmendBodyFactCommand{IdempotencyKey: "validator:laterality", ExpectedRevision: 1, Replacement: &replacement})
	if err != nil {
		return err
	}
	correctedID, err := validatorCommitFactID(corrected)
	if err != nil {
		return err
	}
	var current model.BodyFact
	if err := v.db.WithContext(ctx).First(&current, "id = ? AND user_id = ?", correctedID, owner).Error; err != nil {
		return err
	}
	if string(current.BodyRegionIDs) != `["shoulder.left"]` {
		return errors.New("canonical laterality lost")
	}
	var previous model.BodyFact
	if err := v.db.WithContext(ctx).First(&previous, "id = ? AND user_id = ?", firstID, owner).Error; err != nil {
		return err
	}
	if string(previous.BodyRegionIDs) != `["shoulder.right"]` || previous.AssertionStatus != "superseded" {
		return errors.New("historical laterality rewritten")
	}
	return nil
}

func (v *validator) validateOutcomeFeedbackAtomicity(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if err := v.installOutcomeFailureTrigger(ctx); err != nil {
		return err
	}
	sourceKey := "domain-validator:outcome:retry"
	outcome := model.Outcome{
		SourceType: "domain_validator",
		SourceKey:  sourceKey,
		Kind:       "symptom_change",
		ConcernKey: "region:neck",
		BodyRegion: "left neck",
		// The user's own words become a V2 symptom fact; no legacy fact is touched.
		Value:          datatypes.JSON(`{"description":"neck feels better after the session","trend":"improving"}`),
		Notes:          "forced transactional retry",
		CausalityLevel: "association_only",
		OccurredAt:     time.Now().UTC(),
		Provenance:     datatypes.JSON(`{"source_type":"domain_validator"}`),
	}
	if _, _, err := v.outcomes.RecordOutcome(ctx, userID, outcome); err == nil {
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

	stored, created, err := v.outcomes.RecordOutcome(ctx, userID, outcome)
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
	var symptom model.BodyFact
	if err := v.db.WithContext(ctx).Where("user_id = ? AND created_revision = ?", userID, *stored.BodyStateRevision).First(&symptom).Error; err != nil {
		return fmt.Errorf("outcome symptom fact missing: %w", err)
	}
	if symptom.Kind != "symptom" || symptom.Statement != "neck feels better after the session" || symptom.AssertionStatus != "asserted" {
		return fmt.Errorf("outcome symptom is not the user's exact V2 statement: %#v", symptom)
	}
	var screens int64
	if err := v.db.WithContext(ctx).Model(&model.SafetyScreen{}).Where("user_id = ? AND body_state_revision >= ?", userID, *stored.BodyStateRevision).Count(&screens).Error; err != nil {
		return fmt.Errorf("count safety screens: %w", err)
	}
	if screens != 0 {
		return fmt.Errorf("outcome symptom must stay unscreened (fail-closed), found %d covering screens", screens)
	}
	return nil
}

func (v *validator) installOutcomeFailureTrigger(ctx context.Context) error {
	return v.db.WithContext(ctx).Exec(`
CREATE OR REPLACE FUNCTION domain_validator_fail_outcome() RETURNS trigger AS $$
BEGIN
    IF NEW.change_type = 'facts.recorded' THEN
        RAISE EXCEPTION 'forced outcome symptom fact failure';
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

func validatorCommitFactID(commit *model.BodyFactCommit) (uuid.UUID, error) {
	var ids []uuid.UUID
	if err := json.Unmarshal(commit.FactIDs, &ids); err != nil {
		return uuid.Nil, err
	}
	if len(ids) != 1 {
		return uuid.Nil, errors.New("expected one committed fact")
	}
	return ids[0], nil
}

// This validator owns a disposable synthetic user. The all-negative screen is
// fixture data, never a production command or an AI authority path. Prove that
// its absence blocks before installing it for treatment transaction probes.
func (v *validator) seedSyntheticSafetyScreen(ctx context.Context, owner, factID uuid.UUID, revision int64) error {
	snapshot, err := v.body.GetBodyState(ctx, owner, 10)
	if err != nil {
		return err
	}
	envelope, err := service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		return err
	}
	if envelope.Coverage.Complete {
		return errors.New("unscreened native symptom was treated as covered")
	}
	capture := "0123456789abcdef01234567"
	screen := model.SafetyScreen{ID: uuid.New(), UserID: owner, CaptureID: capture, BodyFactID: factID,
		SourceConversationID: uuid.New(), SourceRunID: uuid.New(), SourceMessageID: uuid.New(), SourceInteractionID: uuid.New(),
		SourceToolCallID: "intake-" + capture, BodyStateRevision: revision, RecordedAt: time.Now().UTC()}
	if err := v.db.WithContext(ctx).Create(&screen).Error; err != nil {
		return err
	}
	snapshot, err = v.body.GetBodyState(ctx, owner, 10)
	if err != nil {
		return err
	}
	envelope, err = service.ProjectSafetyEnvelopeV2(snapshot)
	if err != nil {
		return err
	}
	if !envelope.Coverage.Complete {
		return errors.New("synthetic native screen did not cover exact symptom")
	}
	return nil
}
