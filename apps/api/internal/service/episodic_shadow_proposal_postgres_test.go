package service

import (
	"context"
	"errors"
	"testing"

	runtimev1 "github.com/bodysense/api/internal/generated/runtimeproto/v1"
	"github.com/bodysense/api/internal/repository"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func quoteProposal(policy, msgID, quote, category string) *runtimev1.EpisodicShadowDraftProposal {
	return &runtimev1.EpisodicShadowDraftProposal{
		PolicyRevision: policy,
		Quotes: []*runtimev1.EpisodicShadowQuote{{
			SourceMessageId: msgID, Quote: quote, Preference: category,
		}},
	}
}

// Real isolated PostgreSQL 18; no model or network call. Requires
// BODYSENSE_INTEGRATION_DATABASE_URL. All chat and generated content are
// synthetic. The Go source lock, not a model-facing payload, is the authority
// for message identity and exact source text.
func TestEpisodicShadowProposalOwnerSourceAndErasureGatesPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(4, 5)
	var rows []struct {
		ID   uuid.UUID
		Seq  int
		Role string
	}
	if err := f.db.Raw("SELECT id,seq,role FROM messages WHERE conversation_id=? AND seq BETWEEN 1 AND 2 ORDER BY seq", f.conversation).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("missing synthetic messages: %d", len(rows))
	}
	const phrase = "请简短回答。"
	f.exec("UPDATE messages SET content_text=? WHERE id=?", phrase, rows[0].ID)
	cmd := &runtimev1.EpisodicShadowGenerationCommand{
		UserId: f.owner.String(), ConversationId: f.conversation.String(),
		FromSeq: 1, ToSeq: 2, PolicyRevision: EpisodicShadowQuotePolicyRevision,
		SourceMessages: []*runtimev1.EpisodicShadowSourceMessage{
			{Id: rows[0].ID.String(), Seq: 1, Role: "user", Text: phrase},
			{Id: rows[1].ID.String(), Seq: 2, Role: "assistant", Text: "synthetic"},
		},
	}
	proposal := quoteProposal(EpisodicShadowQuotePolicyRevision, rows[0].ID.String(), phrase, "brief_answers")
	writer := repository.NewEpisodicShadowRepository(f.db)
	if _, _, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal); !errors.Is(err, repository.ErrEpisodicShadowDisabled) {
		t.Fatalf("default-off is not fail-closed: %v", err)
	}
	writer.WithShadowAdmissionEnabled(true)
	// Python command content can be fabricated; Go must read its own
	// persisted source to validate each proposal inside the row lock.
	tampered := proto.Clone(cmd).(*runtimev1.EpisodicShadowGenerationCommand)
	tampered.SourceMessages[0] = &runtimev1.EpisodicShadowSourceMessage{
		Id: rows[0].ID.String(), Seq: 1, Role: "user", Text: "请分点回答。",
	}
	for _, bad := range []struct {
		name string
		cmd  *runtimev1.EpisodicShadowGenerationCommand
		p    *runtimev1.EpisodicShadowDraftProposal
		own  uuid.UUID
	}{
		{"foreign owner", cmd, proposal, uuid.New()},
		{"untrusted quote from another message", cmd, quoteProposal(EpisodicShadowQuotePolicyRevision, rows[1].ID.String(), phrase, "brief_answers"), f.owner},
		{"wrong preference category", cmd, quoteProposal(EpisodicShadowQuotePolicyRevision, rows[0].ID.String(), phrase, "bullet_answers"), f.owner},
		{"unapproved medical content", cmd, quoteProposal(EpisodicShadowQuotePolicyRevision, rows[0].ID.String(), "我被确诊为高血压。", "brief_answers"), f.owner},
		{"forged source payload", tampered, quoteProposal(EpisodicShadowQuotePolicyRevision, rows[0].ID.String(), "请分点回答。", "bullet_answers"), f.owner},
	} {
		if _, _, err := ApplyEpisodicShadowProposal(ctx, bad.own, writer, bad.cmd, bad.p); !errors.Is(err, repository.ErrEpisodicSourceInvalid) {
			t.Fatalf("%s accepted: %v", bad.name, err)
		}
	}
	if n := f.messages(); n != 4 {
		t.Fatalf("rejected proposals mutated chat messages: %d", n)
	}
	var noteID uuid.UUID
	note, created, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal)
	if err != nil || !created || note.Status != "shadow" || note.ContentText != "对话偏好：回答简短。" ||
		note.UserID != f.owner || len(note.SourceMessageIDs) != 2 {
		t.Fatalf("verified proposal: note=%+v created=%v err=%v", note, created, err)
	}
	noteID = note.ID
	replay, duplicated, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal)
	if err != nil || duplicated || replay.ID != noteID {
		t.Fatalf("idempotent replay: %+v %v %v", replay, duplicated, err)
	}
	// Even if the synthetic user source is later changed, a model may not
	// rewrite the note. S3 erasure gates revoke it after source erasure.
	if err := f.db.Exec("UPDATE messages SET content_text=? WHERE id=?", "clinically different", rows[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	// Same-version replay returns the exact prior note, never a new text.
	replay, duplicated, err = ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal)
	if err != nil || duplicated || replay.ID != noteID {
		t.Fatalf("invalid replay must not produce new note: %v %v %v", replay, duplicated, err)
	}
	// New versions force a new admission and must re-verify DB source text.
	// S2-B policy currently fixes the version, so simulate a deleted prior
	// note (later S6 adds explicit deletion tombstones before live writes).
	if err := f.db.Exec("DELETE FROM episodic_notes WHERE user_id=? AND id=?", f.owner, noteID).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal); !errors.Is(err, repository.ErrEpisodicSourceInvalid) {
		t.Fatalf("stale source was accepted: %v", err)
	}
	if n := f.messages(); n != 4 {
		t.Fatalf("shadow rejection should not delete raw chat: %d", n)
	}
}

func TestEpisodicShadowProposalContextIsBoundedAndNotProvenancePostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	ctx := context.Background()
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(4, 5)
	var rows []struct {
		ID   uuid.UUID
		Seq  int
		Role string
	}
	if err := f.db.Raw("SELECT id,seq,role FROM messages WHERE conversation_id=? AND seq BETWEEN 1 AND 4 ORDER BY seq", f.conversation).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("missing synthetic messages: %d", len(rows))
	}
	const phrase = "请简短回答。"
	f.exec("UPDATE messages SET content_text=? WHERE id=?", phrase, rows[2].ID)
	source := []*runtimev1.EpisodicShadowSourceMessage{
		{Id: rows[2].ID.String(), Seq: 3, Role: "user", Text: phrase},
		{Id: rows[3].ID.String(), Seq: 4, Role: "assistant", Text: "synthetic"},
	}
	context := []*runtimev1.EpisodicShadowSourceMessage{
		{Id: rows[0].ID.String(), Seq: 1, Role: "user", Text: "synthetic previous"},
		{Id: rows[1].ID.String(), Seq: 2, Role: "assistant", Text: "synthetic previous reply"},
	}
	cmd := &runtimev1.EpisodicShadowGenerationCommand{
		UserId: f.owner.String(), ConversationId: f.conversation.String(),
		FromSeq: 3, ToSeq: 4, PolicyRevision: EpisodicShadowQuotePolicyRevision,
		SourceMessages: source, ContextMessages: context,
	}
	proposal := quoteProposal(EpisodicShadowQuotePolicyRevision, rows[2].ID.String(), phrase, "brief_answers")
	writer := repository.NewEpisodicShadowRepository(f.db).WithShadowAdmissionEnabled(true)

	note, created, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, cmd, proposal)
	if err != nil || !created || note.FromSeq != 3 || note.ToSeq != 4 ||
		len(note.SourceMessageIDs) != 2 || note.SourceMessageIDs[0] != rows[2].ID.String() ||
		note.SourceMessageIDs[1] != rows[3].ID.String() {
		t.Fatalf("context leaked into provenance: note=%+v created=%v err=%v", note, created, err)
	}
	// Context is bounded, contiguous and immediately before the source.
	for name, bad := range map[string]*runtimev1.EpisodicShadowGenerationCommand{
		"too many": func() *runtimev1.EpisodicShadowGenerationCommand {
			x := proto.Clone(cmd).(*runtimev1.EpisodicShadowGenerationCommand)
			x.ContextMessages = append(x.ContextMessages, &runtimev1.EpisodicShadowSourceMessage{
				Id: uuid.NewString(), Seq: 1, Role: "user", Text: "third",
			})
			return x
		}(),
		"not adjacent": func() *runtimev1.EpisodicShadowGenerationCommand {
			x := proto.Clone(cmd).(*runtimev1.EpisodicShadowGenerationCommand)
			x.ContextMessages = x.ContextMessages[:1]
			return x
		}(),
		"source reused as context": func() *runtimev1.EpisodicShadowGenerationCommand {
			x := proto.Clone(cmd).(*runtimev1.EpisodicShadowGenerationCommand)
			x.ContextMessages[1] = &runtimev1.EpisodicShadowSourceMessage{
				Id: rows[2].ID.String(), Seq: 2, Role: "assistant", Text: "synthetic previous reply",
			}
			return x
		}(),
	} {
		if _, _, err := ApplyEpisodicShadowProposal(ctx, f.owner, writer, bad, proposal); !errors.Is(err, repository.ErrEpisodicSourceInvalid) {
			t.Fatalf("%s context accepted: %v", name, err)
		}
	}
}

func TestEpisodicShadowProposalAbstainsWithoutStoredDataPostgres(t *testing.T) {
	f := newPruneJobFixture(t)
	writer := repository.NewEpisodicShadowRepository(f.db).WithShadowAdmissionEnabled(true)
	f.addTurn("completed")
	f.addTurn("completed")
	f.advance(4, 5)
	var ids []uuid.UUID
	if err := f.db.Raw("SELECT id FROM messages WHERE conversation_id=? AND seq BETWEEN 1 AND 2 ORDER BY seq", f.conversation).Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	cmd := &runtimev1.EpisodicShadowGenerationCommand{
		UserId: f.owner.String(), ConversationId: f.conversation.String(),
		FromSeq: 1, ToSeq: 2, PolicyRevision: EpisodicShadowQuotePolicyRevision,
		SourceMessages: []*runtimev1.EpisodicShadowSourceMessage{
			{Id: ids[0].String(), Seq: 1, Role: "user", Text: "synthetic"},
			{Id: ids[1].String(), Seq: 2, Role: "assistant", Text: "synthetic"},
		},
	}
	result, created, err := ApplyEpisodicShadowProposal(context.Background(), f.owner, writer, cmd,
		&runtimev1.EpisodicShadowDraftProposal{PolicyRevision: EpisodicShadowQuotePolicyRevision})
	if err != nil || created || result != nil {
		t.Fatalf("abstention is not empty memory: result=%+v %v %v", result, created, err)
	}
}
