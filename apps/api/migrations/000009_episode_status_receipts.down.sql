-- Refuse a downgrade that would invalidate any already accepted
-- explicit user status-evidence receipt, including exact retry records.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM public.episode_command_receipts WHERE operation = 'status') THEN
        RAISE EXCEPTION 'cannot downgrade: Episode status receipts exist';
    END IF;
END $$;
ALTER TABLE public.episode_command_receipts
    DROP CONSTRAINT episode_command_receipts_operation_check;
ALTER TABLE public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_operation_check
    CHECK (operation IN ('create', 'link'));
