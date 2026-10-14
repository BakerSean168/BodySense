-- A downgrade cannot erase accepted answers or their provenance silently.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM public.interaction_resumes)
       OR EXISTS (SELECT 1 FROM public.statement_evidence WHERE source_kind = 'structured_answer') THEN
        RAISE EXCEPTION 'cannot downgrade: interaction resume or structured-answer history exists';
    END IF;
END $$;
DROP INDEX public.statement_evidence_source_interaction;
ALTER TABLE public.statement_evidence DROP CONSTRAINT statement_evidence_chat_binding_check;
ALTER TABLE public.statement_evidence
    DROP COLUMN question_excerpt,
    DROP COLUMN source_tool_call_id,
    DROP COLUMN source_interaction_id;
ALTER TABLE public.statement_evidence ADD CONSTRAINT statement_evidence_chat_binding_check CHECK (
    (source_kind = 'user_message' AND source_conversation_id IS NOT NULL AND source_run_id IS NOT NULL AND source_message_id IS NOT NULL)
    OR (source_kind <> 'user_message' AND source_conversation_id IS NULL AND source_run_id IS NULL AND source_message_id IS NULL)
);
DROP TABLE public.interaction_resumes;
