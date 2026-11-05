-- #515 S2 best-effort trigger: a per-Conversation distillation watermark and
-- a content-free per-segment attempt ledger.
--
-- distilled_through_seq (D) is monotonic and only moves over segments whose
-- outcome is final: note persisted (or replayed), no quotation proposed, the
-- source was ineligible, a user deletion tombstone won, or the bounded retry
-- budget was exhausted and a gap was recorded. It never blocks pruning.
--
-- The ledger stores only the seq range, status, attempt count, a stable
-- reason code and an optional note id. It never stores chat text, message
-- UUIDs, model output or clinical detail. Source rows and notes remain
-- separate controlled copies with their own erasure inventory.
ALTER TABLE public.conversations
    ADD COLUMN distilled_through_seq integer NOT NULL DEFAULT 0;

ALTER TABLE public.conversations
    ADD CONSTRAINT conversations_distilled_cursor_check
    CHECK (distilled_through_seq >= 0 AND distilled_through_seq <= processed_through_seq);

CREATE TABLE public.episodic_distillation_segments (
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    from_seq integer NOT NULL,
    to_seq integer NOT NULL,
    status character varying(16) NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    note_id uuid,
    reason character varying(64) NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, from_seq, to_seq),
    CONSTRAINT episodic_distillation_segments_owner_fk
        FOREIGN KEY (conversation_id, user_id)
        REFERENCES public.conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT episodic_distillation_segments_seq_check
        CHECK (from_seq > 0 AND to_seq >= from_seq),
    CONSTRAINT episodic_distillation_segments_status_check
        CHECK (status IN ('pending', 'distilled', 'skipped', 'gapped')),
    CONSTRAINT episodic_distillation_segments_attempts_check
        CHECK (attempts >= 0 AND attempts <= 16),
    CONSTRAINT episodic_distillation_segments_note_fk
        FOREIGN KEY (note_id) REFERENCES public.episodic_notes (id) ON DELETE SET NULL
);

CREATE INDEX episodic_distillation_segments_status_idx
    ON public.episodic_distillation_segments (conversation_id, status, from_seq);
