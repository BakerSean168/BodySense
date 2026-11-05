-- #516 S6-A: a user deletion is a durable *non-clinical* no-regeneration
-- decision. Deleted content/source IDs must not be retained in a tombstone.
-- The tombstone survives chat pruning, note rollover and source-erasure jobs,
-- but is erased with the owning Conversation/account by the composite FK.
CREATE TABLE public.episodic_note_deletions (
    user_id uuid NOT NULL,
    note_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    from_seq integer NOT NULL,
    to_seq integer NOT NULL,
    deleted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, note_id),
    CONSTRAINT episodic_note_deletions_owner_fk
        FOREIGN KEY (conversation_id, user_id)
        REFERENCES public.conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT episodic_note_deletions_seq_check
        CHECK (from_seq > 0 AND to_seq >= from_seq)
);

CREATE INDEX episodic_note_deletions_overlap_idx
    ON public.episodic_note_deletions (user_id, conversation_id, from_seq, to_seq);

-- Direct SQL / a future generator cannot bypass S6 by choosing a new
-- generator_version or enlarging/clipping the raw source window.
-- Lock the owning Conversation with FOR SHARE, which conflicts with the
-- NO KEY UPDATE lock taken by the user delete transaction. This serializes
-- insert/replay vs. user deletion, including callers outside the Go writer.
CREATE FUNCTION public.guard_episodic_note_user_deletion() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    PERFORM c.id FROM public.conversations c
     WHERE c.id = NEW.conversation_id AND c.user_id = NEW.user_id
     FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'episodic note owner conversation missing'
            USING ERRCODE = '23514';
    END IF;

    IF EXISTS (
        SELECT 1 FROM public.episodic_note_deletions d
         WHERE d.user_id = NEW.user_id
           AND d.conversation_id = NEW.conversation_id
           AND d.from_seq <= NEW.to_seq
           AND d.to_seq >= NEW.from_seq
    ) THEN
        RAISE EXCEPTION 'episodic note source window was user-deleted'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER episodic_notes_user_deletion_guard
    BEFORE INSERT OR UPDATE ON public.episodic_notes
    FOR EACH ROW EXECUTE FUNCTION public.guard_episodic_note_user_deletion();
