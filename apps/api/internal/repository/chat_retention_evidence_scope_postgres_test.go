package repository

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// PC-RET-001 deletion scope proof. docs/security/chat-retention-evidence-scope.md
// is the prose half: a pruned window keeps exactly the chat rows a durable
// Diagnosis, BodyState, Safety or Plan record cites, and everything else in it
// goes. These tests pin both halves against the real baseline.

// chatReferenceScope classifies every way the schema can point at a chat row.
// A new reference fails the catalog test until it is classified here and in
// the scope document, so pruning can never silently orphan or keep a source.
var chatReferenceScope = map[string]string{
	// Hard references into raw chat rows.
	"agent_interactions(run_id)->runs ON DELETE CASCADE":                          "run row is never pruned",
	"agent_tool_calls(message_id)->messages ON DELETE SET NULL":                   "run copy, deleted with a pruned Run",
	"agent_tool_calls(run_id)->runs ON DELETE CASCADE":                            "run row is never pruned",
	"ai_output_reviews(run_id)->runs ON DELETE SET NULL":                          "run row is never pruned",
	"consultation_rollout_observations(run_id)->runs ON DELETE CASCADE":           "run row is never pruned",
	"interaction_resumes(assistant_message_id)->messages ON DELETE NO ACTION":     "cited resume protects it; uncited resume deleted explicitly first",
	"interaction_resumes(interaction_id)->agent_interactions ON DELETE NO ACTION": "uncited resume deleted explicitly before its interaction",
	"interaction_resumes(run_id)->runs ON DELETE CASCADE":                         "run row is never pruned",
	"interaction_resumes(user_message_id)->messages ON DELETE NO ACTION":          "cited resume protects it; uncited resume deleted explicitly first",
	"jobs(run_id)->runs ON DELETE SET NULL":                                       "run row is never pruned",
	"messages(run_id)->runs ON DELETE NO ACTION":                                  "run row is never pruned",
	"runtime_events(run_id)->runs ON DELETE CASCADE":                              "run row is never pruned",
	"thread_projection_messages(message_id)->messages ON DELETE CASCADE":          "projection copy of a pruned message",
	"thread_projection_tool_calls(message_id)->messages ON DELETE SET NULL":       "projection copy, deleted with a pruned Run",
	"thread_projection_tool_calls(run_id)->runs ON DELETE CASCADE":                "run row is never pruned",
	// The evidence funnel: every BodyState, Diagnosis and Safety citation of a
	// user statement goes through statement_evidence, which is never pruned and
	// whose source message is protected.
	"body_facts(user_id,source_evidence_id)->statement_evidence ON DELETE NO ACTION":                                                         "evidence funnel",
	"body_hypotheses(user_id,source_evidence_id)->statement_evidence ON DELETE NO ACTION":                                                    "evidence funnel",
	"body_observations(user_id,source_evidence_id)->statement_evidence ON DELETE NO ACTION":                                                  "evidence funnel",
	"condition_analysis_runs(user_id,request_evidence_id,request_message_id,request_source_version)->statement_evidence ON DELETE NO ACTION": "evidence funnel",
	"condition_direct_edits(user_id,request_evidence_id,request_message_id,request_source_version)->statement_evidence ON DELETE NO ACTION":  "evidence funnel",
	"safety_resolutions(user_id,source_evidence_id,source_message_id,source_version)->statement_evidence ON DELETE NO ACTION":                "evidence funnel",
	"safety_resolutions(user_id,check_interaction_id,source_message_id)->interaction_resumes ON DELETE NO ACTION":                            "cited Ask User",
	// Soft (id-only) references: each is in protectedMessages, the cited
	// interaction set or referencedRuns, or is a copy of the row itself.
	"condition_analysis_runs.request_message_id":   "protected message",
	"condition_direct_edits.request_message_id":    "protected message",
	"interaction_resumes.assistant_message_id":     "protected when cited",
	"interaction_resumes.interaction_id":           "cited interaction set",
	"interaction_resumes.user_message_id":          "protected when cited",
	"messages.parent_message_id":                   "self reference",
	"safety_resolutions.check_interaction_id":      "cited interaction set",
	"safety_resolutions.source_message_id":         "protected message",
	"safety_screens.source_interaction_id":         "cited interaction set",
	"safety_screens.source_message_id":             "protected message",
	"safety_screens.source_run_id":                 "referenced run",
	"statement_evidence.source_interaction_id":     "cited interaction set",
	"statement_evidence.source_message_id":         "protected message",
	"statement_evidence.source_run_id":             "referenced run",
	"thread_projection_messages.parent_message_id": "projection copy",
	"agent_tool_calls.message_id":                  "run copy",
	"thread_projection_messages.message_id":        "projection copy",
	"thread_projection_tool_calls.message_id":      "projection copy",
}

func TestChatRetentionEvidenceScopeCoversEveryChatReferencePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	var found []string
	var fks []struct{ Ref string }
	if err := db.Raw(`
SELECT c.conrelid::regclass::text || '(' ||
       (SELECT string_agg(a.attname, ',' ORDER BY k.ord) FROM unnest(c.conkey) WITH ORDINALITY k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum) || ')->' ||
       c.confrelid::regclass::text || ' ON DELETE ' ||
       CASE c.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
                          WHEN 'n' THEN 'SET NULL' ELSE 'SET DEFAULT' END AS ref
  FROM pg_constraint c
 WHERE c.contype = 'f' AND c.connamespace = 'public'::regnamespace
   AND c.confrelid::regclass::text IN ('messages','runs','agent_interactions','interaction_resumes','statement_evidence')`).Scan(&fks).Error; err != nil {
		t.Fatal(err)
	}
	for _, fk := range fks {
		found = append(found, fk.Ref)
	}
	var cols []struct{ Ref string }
	if err := db.Raw(`
SELECT table_name || '.' || column_name AS ref FROM information_schema.columns
 WHERE table_schema = 'public' AND data_type = 'uuid'
   AND (column_name ~ 'message_id$' OR column_name ~ 'interaction_id$' OR column_name = 'source_run_id')`).Scan(&cols).Error; err != nil {
		t.Fatal(err)
	}
	for _, col := range cols {
		found = append(found, col.Ref)
	}
	slices.Sort(found)
	found = slices.Compact(found)
	for _, ref := range found {
		if _, ok := chatReferenceScope[ref]; !ok {
			t.Errorf("unclassified chat reference %q: classify it in chatReferenceScope and chat-retention-evidence-scope.md", ref)
		}
	}
	for ref := range chatReferenceScope {
		if !slices.Contains(found, ref) {
			t.Errorf("classified chat reference %q no longer exists in the schema", ref)
		}
	}
}

// addAskUser records an answered Ask User on turn's Run: its own answer and
// reply messages, the interaction and the resume receipt.
func (f *retentionFixture) addAskUser(turn retentionTurn) (interaction, answer, reply uuid.UUID) {
	f.t.Helper()
	interaction, answer, reply = uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO agent_interactions(id,run_id,conversation_id,tool_call_id,question,answer,status,answered_at)
VALUES (?,?,?,?,'{"q":"where"}','{"a":"knee"}','answered',now())`, interaction, *turn.run, f.conversation, "ask-"+interaction.String())
	for _, m := range []struct {
		id   uuid.UUID
		role string
	}{{answer, "user"}, {reply, "assistant"}} {
		f.seq++
		f.exec(`INSERT INTO messages(id,conversation_id,turn_id,run_id,role,status,seq,content_text) VALUES (?,?,?,?,?,'completed',?,'synthetic ask')`,
			m.id, f.conversation, turn.turn, turn.run, m.role, f.seq)
	}
	f.exec(`INSERT INTO interaction_resumes(interaction_id,user_id,conversation_id,run_id,request_id,answer_fingerprint,question_fingerprint,
user_message_id,assistant_message_id,claimed_at) VALUES (?,?,?,?,?,?,?,?,?,now())`,
		interaction, f.owner, f.conversation, *turn.run, "resume-"+interaction.String(), strings.Repeat("b", 64), strings.Repeat("c", 64), answer, reply)
	return interaction, answer, reply
}

// citeAnswer is the structured-answer statement evidence a BodyFact,
// Observation or Hypothesis is committed from.
func (f *retentionFixture) citeAnswer(turn retentionTurn, interaction, answer uuid.UUID) {
	f.t.Helper()
	f.exec(`INSERT INTO statement_evidence(id,user_id,source_kind,source_version,excerpt,availability,recorded_at,source_conversation_id,
source_run_id,source_message_id,source_interaction_id,source_tool_call_id,question_excerpt)
VALUES (?,?,'structured_answer',1,'knee','available',now(),?,?,?,?,?,'where')`,
		uuid.New(), f.owner, f.conversation, turn.run, answer, interaction, "ask-"+interaction.String())
}

func TestChatRetentionPruneKeepsOnlyCitedEvidencePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	f := newRetentionFixture(t, db)
	ctx := context.Background()

	plain := f.addTurn("completed", "plain chat")                    // seq 1-2
	uncited := f.addTurn("completed", "asked, never cited")          // seq 3-4
	uncitedAsk, uncitedAnswer, uncitedReply := f.addAskUser(uncited) // 5-6
	cited := f.addTurn("completed", "asked, fact recorded")          // 7-8
	citedAsk, citedAnswer, citedReply := f.addAskUser(cited)         // 9-10
	f.citeAnswer(cited, citedAsk, citedAnswer)
	held := f.addTurn("completed", "fact from the message") // 11-12
	heldAsk, heldAnswer, heldReply := f.addAskUser(held)    // 13-14
	f.addEvidence(f.owner, held)
	tail := f.addTurn("completed", "tail") // 15-16

	f.advance(16, 17)
	receipt, err := f.prune(ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PrunedThroughSeq != 14 {
		t.Fatalf("receipt=%#v, want pruned through 14", receipt)
	}

	// Kept: what statement evidence cites, both messages of a cited resume,
	// and the live tail.
	for name, id := range map[string]uuid.UUID{
		"cited answer": citedAnswer, "cited reply": citedReply, "held source": held.user,
		"tail": tail.user, "tail reply": tail.reply,
	} {
		if !f.messageExists(id) {
			t.Errorf("%s pruned", name)
		}
	}
	// Gone: everything no durable record cites, Ask User answers included.
	for name, id := range map[string]uuid.UUID{
		"plain": plain.user, "plain reply": plain.reply, "uncited turn": uncited.user,
		"uncited answer": uncitedAnswer, "uncited reply": uncitedReply, "cited turn": cited.user,
		"held reply": held.reply, "held-run answer": heldAnswer, "held-run reply": heldReply,
	} {
		if f.messageExists(id) {
			t.Errorf("%s survived", name)
		}
	}
	resume := func(id uuid.UUID) int64 {
		return f.count("SELECT COUNT(*) FROM interaction_resumes WHERE interaction_id = ?", id)
	}
	interaction := func(id uuid.UUID) int64 {
		return f.count("SELECT COUNT(*) FROM agent_interactions WHERE id = ?", id)
	}
	if resume(citedAsk) != 1 || interaction(citedAsk) != 1 {
		t.Error("cited Ask User receipt lost")
	}
	if resume(uncitedAsk) != 0 || interaction(uncitedAsk) != 0 {
		t.Error("uncited Ask User of a pruned Run kept its raw question and answer")
	}
	// The held Run keeps its run-scoped copies, but the receipt of an answer
	// that left the window is removed explicitly, never by cascade.
	if resume(heldAsk) != 0 || interaction(heldAsk) != 1 || f.runEvents(held.run) != 2 {
		t.Error("held Run retention mismatch")
	}
	if f.runEvents(plain.run) != 0 || f.runEvents(uncited.run) != 0 || f.runEvents(cited.run) != 2 {
		t.Error("run event retention mismatch")
	}
	if receipt.PrunedRuns != 2 || receipt.DeletedInteractions != 1 || receipt.RetainedMessages != 3 || receipt.DeletedMessages != 11 {
		t.Errorf("receipt=%#v", receipt)
	}
	if f.count("SELECT COUNT(*) FROM statement_evidence WHERE user_id = ?", f.owner) != 2 {
		t.Error("statement evidence lost")
	}

	// Fail closed: a cited receipt can never be cascaded away by a raw delete
	// of its source message or interaction.
	if err := db.Exec("DELETE FROM messages WHERE id = ?", citedAnswer).Error; err == nil ||
		!strings.Contains(err.Error(), "interaction_resumes_user_message_id_fkey") {
		t.Fatalf("raw delete of a cited answer: %v", err)
	}
	if err := db.Exec("DELETE FROM agent_interactions WHERE id = ?", citedAsk).Error; err == nil ||
		!strings.Contains(err.Error(), "interaction_resumes_interaction_id_fkey") {
		t.Fatalf("raw delete of a cited interaction: %v", err)
	}
	if c := f.cursor(); c.PrunedThroughSeq != 14 {
		t.Fatalf("cursor=%#v", c)
	}
}

// PC3-19 / PC-D8: the legacy public share state is gone from the schema, so no
// token or snapshot can outlive the vNext reset or be written again.
func TestChatRetentionLegacyShareStateIsGonePostgres(t *testing.T) {
	db := openSingletonTestDB(t)
	var leftovers []string
	if err := db.Raw(`
SELECT table_name || COALESCE('.' || column_name, '') FROM (
  SELECT table_name, NULL::text AS column_name FROM information_schema.tables
   WHERE table_schema = 'public' AND table_name ~ 'share'
  UNION ALL
  SELECT table_name, column_name FROM information_schema.columns
   WHERE table_schema = 'public' AND column_name ~ '(^|_)(share|shared|public)_?(token|id|snapshot|url|at)$'
) leftovers`).Scan(&leftovers).Error; err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("legacy share state still in the schema: %v", leftovers)
	}
}
