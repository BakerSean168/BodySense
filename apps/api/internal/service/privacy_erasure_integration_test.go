package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bodysense/api/internal/auth"
	"github.com/bodysense/api/internal/cache"
	"github.com/bodysense/api/internal/database"
	"github.com/bodysense/api/internal/dto"
	"github.com/bodysense/api/internal/model"
	"github.com/bodysense/api/internal/repository"
	"github.com/bodysense/api/internal/uploadstorage"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPrivacyErasureSyntheticUserPostgres(t *testing.T) {
	dsn := os.Getenv("BODYSENSE_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BODYSENSE_INTEGRATION_DATABASE_URL to run privacy erasure integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open integration database: %v", err)
	}
	ctx := context.Background()

	userID := uuid.New()
	conversationID := uuid.New()
	runID := uuid.New()
	turnID := uuid.New()
	analysisID := uuid.New()
	treatmentID := uuid.New()
	revisionID := uuid.New()
	uploadID := uuid.New()
	email := "privacy-" + userID.String() + "@example.test"

	cleanup := func() { _ = db.Exec("DELETE FROM users WHERE id = ?", userID).Error }
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(query string, args ...any) {
		t.Helper()
		if err := db.WithContext(ctx).Exec(query, args...).Error; err != nil {
			t.Fatalf("seed query failed: %v\nquery: %s", err, query)
		}
	}

	mustExec("INSERT INTO users(id,email,password_hash) VALUES (?,?,?)", userID, email, "synthetic-hash")
	mustExec("INSERT INTO user_profiles(user_id,birth_date) VALUES (?,?)", userID, "1998-05-20")
	mustExec("INSERT INTO conversations(id,user_id,title) VALUES (?,?,?)", conversationID, userID, "private conversation")
	mustExec("INSERT INTO messages(id,conversation_id,turn_id,role,seq,parts,content_text) VALUES (?,?,?,?,?,?,?)", uuid.New(), conversationID, turnID, "user", 1, `[{"type":"text","text":"private symptom"}]`, "private symptom")
	mustExec("INSERT INTO runs(id,conversation_id,turn_id,request_id,user_id,model,status) VALUES (?,?,?,?,?,?,?)", runID, conversationID, turnID, "privacy-run-"+userID.String(), userID, "synthetic", "completed")
	mustExec("INSERT INTO conversation_shares(conversation_id,share_token,snapshot_messages,snapshot_title) VALUES (?,?,?,?)", conversationID, "share"+userID.String()[:20], `[{"role":"user","text":"private symptom"}]`, "private")
	mustExec("INSERT INTO jobs(id,run_id,conversation_id,user_id,job_type,status,input) VALUES (?,?,?,?,?,?,?)", uuid.New(), runID, conversationID, userID, "synthetic", "completed", `{"private":"job"}`)
	mustExec("INSERT INTO ai_output_reviews(run_id,conversation_id,user_id,output_type,raw_output) VALUES (?,?,?,?,?)", runID, conversationID, userID, "diagnosis", `{"private":"raw model output"}`)

	uploadRoot := t.TempDir()
	uploadStorage, err := uploadstorage.NewRegistry(uploadstorage.Config{
		Environment: "test", Backend: "local", LocalRoot: uploadRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	uploadKey := userID.String() + "/" + uploadID.String() + "/original.jpg"
	payload := []byte("private image bytes")
	if err := uploadStorage.DefaultStore().Put(ctx, uploadKey, bytes.NewReader(payload), int64(len(payload)), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	mustExec("INSERT INTO user_uploads(id,user_id,file_type,original_name,storage_backend,storage_key,file_size,mime_type,ocr_result,ocr_status) VALUES (?,?,?,?,?,?,?,?,?,?)", uploadID, userID, "photo_front", "photo.jpg", "local", uploadKey, len(payload), "image/jpeg", `{"private":"ocr"}`, "completed")

	mustExec("INSERT INTO body_states(user_id,current_revision,safety_state) VALUES (?,?,?)", userID, 1, `{"private":"safety"}`)
	mustExec("INSERT INTO body_state_revisions(user_id,revision,change_type,source,changes) VALUES (?,?,?,?,?)", userID, 1, "synthetic", "integration", `{"private":"state"}`)
	mustExec("INSERT INTO body_state_facts(user_id,kind,value,origin,created_revision,updated_revision,details) VALUES (?,?,?,?,?,?,?)", userID, "symptom", "private pain", "user", 1, 1, `{"private":true}`)

	mustExec("INSERT INTO diagnosis_analyses(id,user_id,body_state_revision,status,summary,raw_output) VALUES (?,?,?,?,?,?)", analysisID, userID, 1, "completed", "private diagnosis", `{"private":"diagnosis reasoning"}`)
	mustExec("INSERT INTO diagnosis_candidates(analysis_id,ordinal,name,confidence,basis,raw_payload) VALUES (?,?,?,?,?,?)", analysisID, 1, "synthetic candidate", "medium", "private basis", `{"private":true}`)
	mustExec("INSERT INTO diagnosis_rollout_observations(source_analysis_id,stage,subject_bucket,champion_configuration_id,challenger_configuration_id,served_configuration_id,comparison) VALUES (?,?,?,?,?,?,?)", analysisID, "shadow", 7, "champion", "challenger", "champion", `{"private":"comparison"}`)

	mustExec("INSERT INTO treatments(id,user_id,current_revision,status,source_body_state_revision,source_diagnosis_analysis_id) VALUES (?,?,?,?,?,?)", treatmentID, userID, 1, "active", 1, analysisID)
	mustExec("INSERT INTO treatment_revisions(id,treatment_id,revision,acceptance_state,lifecycle_state,source_body_state_revision,source_diagnosis_analysis_id,goal,duration_weeks,plan) VALUES (?,?,?,?,?,?,?,?,?,?)", revisionID, treatmentID, 1, "accepted", "active", 1, analysisID, "private goal", 4, `{"private":"treatment"}`)
	mustExec("INSERT INTO outcomes(user_id,treatment_id,treatment_revision_id,source_type,source_key,kind,value,notes,occurred_at) VALUES (?,?,?,?,?,?,?,?,?)", userID, treatmentID, revisionID, "synthetic", "privacy-outcome", "pain", `{"score":4}`, "private outcome", time.Now().UTC())

	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	defer redisClient.Close()
	jwtConfig := auth.JWTConfig{SecretKey: "integration-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 30 * 24 * time.Hour}
	sessionCache := cache.NewUserSessionCache(redisClient, jwtConfig.RefreshTokenTTL)
	userRepo := repository.NewUserRepository(db)
	authService := NewAuthService(userRepo, jwtConfig, sessionCache, redisClient)
	refreshSessionID := uuid.New()
	tokens, err := authService.generateTokens(ctx, &model.User{ID: userID, Email: email}, refreshSessionID)
	if err != nil {
		t.Fatalf("seed auth session: %v", err)
	}

	privacyRepo := repository.NewPrivacyErasureRepository(db)
	privacyService := NewPrivacyErasureService(
		privacyRepo,
		userRepo,
		authService,
		uploadStorage,
		database.NewTransactionManager(db),
	)
	request, err := privacyService.Request(ctx, userID, PrivacyErasureConfirmationPhrase)
	if err != nil {
		t.Fatalf("Request erasure: %v", err)
	}
	if request == nil || request.Status != "completed" || request.SubjectUserID != nil {
		t.Fatalf("request=%+v, want anonymous completed audit", request)
	}

	checks := []struct {
		name  string
		query string
		arg   any
	}{
		{name: "users", query: "SELECT COUNT(*) FROM users WHERE id = ?", arg: userID},
		{name: "user_profiles", query: "SELECT COUNT(*) FROM user_profiles WHERE user_id = ?", arg: userID},
		{name: "conversations", query: "SELECT COUNT(*) FROM conversations WHERE user_id = ?", arg: userID},
		{name: "messages", query: "SELECT COUNT(*) FROM messages WHERE conversation_id = ?", arg: conversationID},
		{name: "runs", query: "SELECT COUNT(*) FROM runs WHERE user_id = ?", arg: userID},
		{name: "conversation_shares", query: "SELECT COUNT(*) FROM conversation_shares WHERE conversation_id = ?", arg: conversationID},
		{name: "jobs", query: "SELECT COUNT(*) FROM jobs WHERE user_id = ?", arg: userID},
		{name: "ai_output_reviews", query: "SELECT COUNT(*) FROM ai_output_reviews WHERE user_id = ?", arg: userID},
		{name: "user_uploads", query: "SELECT COUNT(*) FROM user_uploads WHERE user_id = ?", arg: userID},
		{name: "body_states", query: "SELECT COUNT(*) FROM body_states WHERE user_id = ?", arg: userID},
		{name: "body_state_revisions", query: "SELECT COUNT(*) FROM body_state_revisions WHERE user_id = ?", arg: userID},
		{name: "body_state_facts", query: "SELECT COUNT(*) FROM body_state_facts WHERE user_id = ?", arg: userID},
		{name: "diagnosis_analyses", query: "SELECT COUNT(*) FROM diagnosis_analyses WHERE user_id = ?", arg: userID},
		{name: "diagnosis_candidates", query: "SELECT COUNT(*) FROM diagnosis_candidates WHERE analysis_id = ?", arg: analysisID},
		{name: "diagnosis_rollout_observations", query: "SELECT COUNT(*) FROM diagnosis_rollout_observations WHERE source_analysis_id = ?", arg: analysisID},
		{name: "treatments", query: "SELECT COUNT(*) FROM treatments WHERE user_id = ?", arg: userID},
		{name: "treatment_revisions", query: "SELECT COUNT(*) FROM treatment_revisions WHERE treatment_id = ?", arg: treatmentID},
		{name: "outcomes", query: "SELECT COUNT(*) FROM outcomes WHERE user_id = ?", arg: userID},
	}
	for _, check := range checks {
		var count int64
		if err := db.Raw(check.query, check.arg).Scan(&count).Error; err != nil {
			t.Fatalf("count %s: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("table %s retained %d synthetic user rows", check.name, count)
		}
	}
	if _, err := uploadStorage.DefaultStore().Stat(ctx, uploadKey); err == nil {
		t.Fatal("physical upload survived erasure")
	}
	var auditCount int64
	if err := db.Table("privacy_erasure_requests").Where("id = ? AND subject_user_id IS NULL AND status = 'completed'", request.ID).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("anonymous audit count=%d err=%v", auditCount, err)
	}
	if exists, err := sessionCache.Exists(ctx, refreshSessionID); err != nil || exists {
		t.Fatalf("session survived erasure: exists=%v err=%v", exists, err)
	}
	if _, err := authService.RefreshToken(ctx, dto.RefreshRequest{RefreshToken: tokens.RefreshToken}); err == nil || !(errors.Is(err, ErrInvalidRefresh) || errors.Is(err, ErrRefreshReuse)) {
		t.Fatalf("erased refresh credential remained usable: %v", err)
	}
}
