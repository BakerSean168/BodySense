-- Bind AI proposals to a real user-authored message, not a model-supplied
-- source label. References remain historical IDs after chat-window pruning;
-- no FK to messages/runs, which would block verified physical message cleanup.
-- Authorization/ownership and exact excerpt equality are verified by Go inside
-- the same transaction that creates the durable BodyFact assertion.
ALTER TABLE public.statement_evidence
    ADD COLUMN source_conversation_id uuid,
    ADD COLUMN source_run_id uuid,
    ADD COLUMN source_message_id uuid;
ALTER TABLE public.statement_evidence
    ADD CONSTRAINT statement_evidence_chat_binding_check CHECK (
        (source_kind = 'user_message'
          AND source_conversation_id IS NOT NULL
          AND source_run_id IS NOT NULL
          AND source_message_id IS NOT NULL)
        OR
        (source_kind <> 'user_message'
          AND source_conversation_id IS NULL
          AND source_run_id IS NULL
          AND source_message_id IS NULL)
    );
CREATE INDEX statement_evidence_source_message
    ON public.statement_evidence (user_id, source_message_id)
    WHERE source_message_id IS NOT NULL;
