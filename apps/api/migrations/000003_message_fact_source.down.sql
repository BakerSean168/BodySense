-- Reverse the optional message provenance extension without touching the
-- statement and fact ledger introduced by migration 000002.
DROP INDEX IF EXISTS public.statement_evidence_source_message;
ALTER TABLE public.statement_evidence
    DROP CONSTRAINT IF EXISTS statement_evidence_chat_binding_check;
ALTER TABLE public.statement_evidence
    DROP COLUMN IF EXISTS source_message_id,
    DROP COLUMN IF EXISTS source_run_id,
    DROP COLUMN IF EXISTS source_conversation_id;
