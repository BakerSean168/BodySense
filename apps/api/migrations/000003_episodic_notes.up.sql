-- #495 S1: inert storage only. No writer, retrieval path or serving flag.
-- The composite owner FK prevents notes from being attached to another user's
-- Conversation. Provenance IDs deliberately do NOT reference messages:
-- raw-message pruning must not be blocked by non-authoritative context.
CREATE UNIQUE INDEX conversations_identity_owner_uidx
    ON public.conversations (id, user_id);

CREATE TABLE public.episodic_notes (
    id uuid DEFAULT uuidv7() PRIMARY KEY,
    user_id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    from_seq integer NOT NULL,
    to_seq integer NOT NULL,
    source_message_ids uuid[] NOT NULL,
    generator_version varchar(128) NOT NULL,
    retention_policy_version varchar(128) NOT NULL,
    content_text text NOT NULL,
    status varchar(24) NOT NULL DEFAULT 'shadow',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT episodic_notes_owner_conversation_fk
        FOREIGN KEY (conversation_id, user_id)
        REFERENCES public.conversations (id, user_id) ON DELETE CASCADE,
    CONSTRAINT episodic_notes_seq_range_check
        CHECK (from_seq > 0 AND to_seq >= from_seq),
    CONSTRAINT episodic_notes_source_ids_check
        CHECK (cardinality(source_message_ids) BETWEEN 1 AND 128
               AND array_position(source_message_ids, NULL) IS NULL),
    CONSTRAINT episodic_notes_generator_check
        CHECK (char_length(generator_version) BETWEEN 1 AND 128),
    CONSTRAINT episodic_notes_policy_check
        CHECK (char_length(retention_policy_version) BETWEEN 1 AND 128),
    CONSTRAINT episodic_notes_content_check
        CHECK (char_length(content_text) BETWEEN 1 AND 4096),
    -- S1/S2 notes are non-serving shadow data. Enabling serving requires
    -- the S3 privacy-erasure gates and a later explicit migration.
    CONSTRAINT episodic_notes_shadow_only_check
        CHECK (status = 'shadow'),
    CONSTRAINT episodic_notes_idempotency_key
        UNIQUE (user_id, conversation_id, from_seq, to_seq,
                generator_version, retention_policy_version)
);

CREATE INDEX episodic_notes_owner_created_idx
    ON public.episodic_notes (user_id, created_at DESC);
CREATE INDEX episodic_notes_source_ids_idx
    ON public.episodic_notes USING gin (source_message_ids);
