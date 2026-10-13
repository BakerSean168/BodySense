-- Explicit, owner-authorized status-evidence admission is a distinct
-- semantic command from creating an Episode or linking a symptom Fact.
-- Existing v8 receipts are immutable; only extend the allowed operation.
ALTER TABLE public.episode_command_receipts
    DROP CONSTRAINT episode_command_receipts_operation_check;
ALTER TABLE public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_operation_check
    CHECK (operation IN ('create', 'link', 'status'));
