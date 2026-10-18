package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/bodysense/api/internal/model"
	"github.com/google/uuid"
)

// addAnsweredAsk records an answered Ask User in turn with its resume receipt
// (answer = the turn's user message, reply = its assistant message).
func (f *retentionFixture) addAnsweredAsk(turn retentionTurn) uuid.UUID {
	f.t.Helper()
	interaction := uuid.New()
	f.exec(`INSERT INTO agent_interactions(id,run_id,conversation_id,tool_call_id,question,answer,status,answered_at)
VALUES (?,?,?,?,'{"q":"where"}','{"a":"knee"}','answered',now())`, interaction, *turn.run, f.conversation, "ask-"+interaction.String())
	f.exec(`INSERT INTO interaction_resumes(interaction_id,user_id,conversation_id,run_id,request_id,answer_fingerprint,question_fingerprint,
user_message_id,assistant_message_id,claimed_at) VALUES (?,?,?,?,?,?,?,?,?,now())`,
		interaction, f.owner, f.conversation, *turn.run, "resume-"+interaction.String(), strings.Repeat("b", 64), strings.Repeat("c", 64),
		turn.user, turn.reply)
	return interaction
}

func (f *retentionFixture) answerStatement(interaction uuid.UUID) uuid.UUID {
	f.t.Helper()
	var ids []uuid.UUID
	if err := f.db.Raw(`SELECT id FROM statement_evidence WHERE source_interaction_id = ?`, interaction).Scan(&ids).Error; err != nil || len(ids) != 1 {
		f.t.Fatalf("answer statement: %v %v", ids, err)
	}
	return ids[0]
}

// AS-002: an erased statement no longer protects its chat source from
// retention. The ordinary prune deletes the message an erased statement was
// taken from, the Ask User answer and receipt an erased structured answer
// cited, and their Runs' copies, while a message an unerased statement cites
// stays in place.
func TestChatRetentionErasedStatementReleasesItsChatSourcePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	said := f.addTurn("completed", "erased BodyFact source") // 1-2
	kept := f.addTurn("completed", "kept BodyFact source")   // 3-4
	ask := f.addTurn("completed", "erased Ask User answer")  // 5-6
	tail := f.addTurn("completed", "tail")                   // 7-8
	erased := f.addEvidence(f.owner, said)
	f.addEvidence(f.owner, kept)
	interaction := f.addAnsweredAsk(ask)
	f.citeAnswer(ask, interaction, ask.user)
	f.exec(`UPDATE statement_evidence SET availability = 'erased', excerpt = '', question_excerpt = NULL
WHERE user_id = ? AND (id = ? OR source_interaction_id = ?)`, f.owner, erased, interaction)

	f.advance(8, 9)
	receipt, err := f.prune(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	// min(C-1, last-1) = 7 splits the tail interaction, so the pass stops at 6.
	if receipt.PrunedThroughSeq != 6 || receipt.DeletedMessages != 5 || receipt.RetainedMessages != 1 || receipt.PrunedRuns != 2 {
		t.Fatalf("receipt=%#v", receipt)
	}
	for _, gone := range []uuid.UUID{said.user, said.reply, kept.reply, ask.user, ask.reply} {
		if f.messageExists(gone) {
			t.Fatalf("message %s survived its erased statement", gone)
		}
	}
	for _, live := range []uuid.UUID{kept.user, tail.user, tail.reply} {
		if !f.messageExists(live) {
			t.Fatalf("cited or live message %s pruned", live)
		}
	}
	if f.count(`SELECT count(*) FROM interaction_resumes WHERE interaction_id = ?`, interaction) != 0 ||
		f.count(`SELECT count(*) FROM agent_interactions WHERE id = ?`, interaction) != 0 {
		t.Fatal("erased structured answer kept its Ask User receipt")
	}
	if f.runEvents(said.run) != 0 || f.runEvents(ask.run) != 0 || f.runEvents(kept.run) != 2 {
		t.Fatal("run copies of erased statements were kept")
	}
	if f.count(`SELECT count(*) FROM statement_evidence WHERE user_id = ?`, f.owner) != 3 {
		t.Fatal("statement tombstones lost")
	}
}

// The source_message target deletes the chat message a statement was taken
// from when nothing else cites it (with the uncited Ask User receipt naming
// it), keeps a message another unerased statement cites as a conversation
// record (retained_by_policy, chat_record_cited, so the request ends
// erased_with_retention), and retries a message whose Run has not settled or
// that is the newest message. Once the other statement is erased too, its
// own erasure deletes the shared message.
func TestBodyFactStatementErasureSourceMessagePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()
	repo := NewStatementErasureRepository(db)

	alone := f.addTurn("completed", "only this statement cites me")
	shared := f.addTurn("completed", "two statements cite me")
	ask := f.addTurn("completed", "structured answer")
	busy := f.addTurn("running", "still running")
	last := f.addTurn("completed", "newest")
	sAlone := f.addEvidence(f.owner, alone)
	sShared, sOther := f.addEvidence(f.owner, shared), f.addEvidence(f.owner, shared)
	interaction := f.addAnsweredAsk(ask)
	f.citeAnswer(ask, interaction, ask.user)
	sAnswer := f.answerStatement(interaction)
	sBusy := f.addEvidence(f.owner, busy)
	sNewest := f.addEvidence(f.owner, retentionTurn{run: last.run, user: last.reply})

	requests := map[uuid.UUID]*model.StatementErasureRequest{}
	sourceMessage := func(statement uuid.UUID) model.AssetErasureOutcome {
		t.Helper()
		if requests[statement] == nil {
			receipt, err := repo.CreateOrGet(ctx, f.owner, statement, "erase-"+statement.String())
			if err != nil {
				t.Fatal(err)
			}
			requests[statement] = &receipt.Request
		}
		return execStatementTarget(t, repo, requests[statement], model.StatementErasureSourceMessage)
	}

	if got := sourceMessage(sAlone); got.State != model.AssetErasureTargetVerified || got.Erased != 1 {
		t.Fatalf("alone: %+v", got)
	}
	if f.messageExists(alone.user) || !f.messageExists(alone.reply) {
		t.Fatal("alone: wrong message deleted")
	}
	if got := sourceMessage(sAlone); got.State != model.AssetErasureTargetVerified || got.Erased != 0 {
		t.Fatalf("alone repeat: %+v", got)
	}

	if got := sourceMessage(sAnswer); got.State != model.AssetErasureTargetVerified || got.Erased != 1 {
		t.Fatalf("answer: %+v", got)
	}
	if f.messageExists(ask.user) || !f.messageExists(ask.reply) ||
		f.count(`SELECT count(*) FROM interaction_resumes WHERE interaction_id = ?`, interaction) != 0 {
		t.Fatal("answer: message or receipt kept")
	}

	wantStatementOutcome(t, "busy", sourceMessage(sBusy), model.AssetErasureTargetFailed, model.ErasureCodeSourceMessageBusy, 1)
	wantStatementOutcome(t, "newest", sourceMessage(sNewest), model.AssetErasureTargetFailed, model.ErasureCodeSourceMessageBusy, 1)
	if !f.messageExists(busy.user) || !f.messageExists(last.reply) {
		t.Fatal("busy or newest message deleted")
	}

	got := sourceMessage(sShared)
	if got.State != model.ErasureTargetRetainedByPolicy || got.RetentionReason != model.ErasureRetentionChatRecordCited || got.Outstanding != 1 {
		t.Fatalf("shared: %+v", got)
	}
	if !f.messageExists(shared.user) {
		t.Fatal("shared message deleted while another statement cites it")
	}
	// Finish the shared statement's erasure: a cited source makes it
	// erased_with_retention, never erased.
	request := requests[sShared]
	for _, target := range model.StatementErasureTargets {
		switch target {
		case model.StatementErasureSourceMessage, model.StatementErasureManifest:
		case model.StatementErasureRuntimeCheckpoints:
			if err := repo.RecordOutcome(ctx, request, target, model.AssetErasureOutcome{State: model.AssetErasureTargetVerified}); err != nil {
				t.Fatal(err)
			}
		default:
			if got := execStatementTarget(t, repo, request, target); got.State != model.AssetErasureTargetVerified {
				t.Fatalf("%s: %+v", target, got)
			}
		}
	}
	if terminal, err := repo.Finalize(ctx, request); err != nil || !terminal {
		t.Fatalf("finalize: %v %v", terminal, err)
	}
	receipt, err := repo.GetByStatement(ctx, f.owner, sShared)
	if err != nil || receipt.Request.Status != model.StatementErasureErasedWithRetention {
		t.Fatalf("shared receipt=%+v err=%v", receipt, err)
	}
	retained := statementReceiptTarget(t, receipt, model.StatementErasureSourceMessage)
	if retained.State != model.ErasureTargetRetainedByPolicy || retained.RetentionReason == nil || *retained.RetentionReason != model.ErasureRetentionChatRecordCited {
		t.Fatalf("retained source=%+v", retained)
	}

	// The erased statement no longer cites the message: the other one's
	// erasure now deletes it.
	if got := sourceMessage(sOther); got.State != model.AssetErasureTargetVerified || got.Erased != 1 {
		t.Fatalf("other after shared erased: %+v", got)
	}
	if f.messageExists(shared.user) || !f.messageExists(shared.reply) {
		t.Fatal("shared message kept after its last citing statement was erased")
	}
}
