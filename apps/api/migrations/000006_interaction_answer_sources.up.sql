-- A resume is an answer to one durable interaction on the SAME Run. This
-- metadata-only receipt fences concurrent execution without a second Run.
CREATE TABLE public.interaction_resumes (
    interaction_id uuid PRIMARY KEY REFERENCES public.agent_interactions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES public.conversations(id) ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES public.runs(id) ON DELETE CASCADE,
    request_id text NOT NULL CHECK (length(request_id) BETWEEN 1 AND 200),
    answer_fingerprint text NOT NULL CHECK (answer_fingerprint ~ '^[0-9a-f]{64}$'),
    question_fingerprint text NOT NULL CHECK (question_fingerprint ~ '^[0-9a-f]{64}$'),
    user_message_id uuid NOT NULL UNIQUE REFERENCES public.messages(id) ON DELETE CASCADE,
    assistant_message_id uuid NOT NULL UNIQUE REFERENCES public.messages(id) ON DELETE CASCADE,
    claimed_at timestamptz NOT NULL,
    UNIQUE (user_id, request_id),
    CHECK (user_message_id <> assistant_message_id)
);
CREATE INDEX interaction_resumes_run ON public.interaction_resumes (user_id, run_id);

-- Evidence outlives prunable messages, interactions and execution receipts.
-- No raw FK/cascade may destroy a long-lived, legitimately committed assertion.
ALTER TABLE public.statement_evidence
    ADD COLUMN source_interaction_id uuid,
    ADD COLUMN source_tool_call_id text,
    ADD COLUMN question_excerpt text;
ALTER TABLE public.statement_evidence DROP CONSTRAINT statement_evidence_chat_binding_check;
ALTER TABLE public.statement_evidence ADD CONSTRAINT statement_evidence_chat_binding_check CHECK (
    (source_kind = 'direct_edit'
      AND source_conversation_id IS NULL AND source_run_id IS NULL AND source_message_id IS NULL
      AND source_interaction_id IS NULL AND source_tool_call_id IS NULL AND question_excerpt IS NULL)
    OR
    (source_kind = 'user_message'
      AND source_conversation_id IS NOT NULL AND source_run_id IS NOT NULL AND source_message_id IS NOT NULL
      AND source_interaction_id IS NULL AND source_tool_call_id IS NULL AND question_excerpt IS NULL)
    OR
    (source_kind = 'structured_answer'
      AND source_conversation_id IS NOT NULL AND source_run_id IS NOT NULL AND source_message_id IS NOT NULL
      AND source_interaction_id IS NOT NULL AND source_tool_call_id IS NOT NULL
      AND length(source_tool_call_id) BETWEEN 1 AND 256
      AND question_excerpt IS NOT NULL AND length(question_excerpt) BETWEEN 1 AND 2000)
);
CREATE INDEX statement_evidence_source_interaction ON public.statement_evidence (user_id, source_interaction_id)
    WHERE source_interaction_id IS NOT NULL;
