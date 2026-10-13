-- A user-confirmed historical recurrence cannot be discarded just to
-- downgrade the Schema. Never delete source, authorization or relation rows.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM public.episode_relations)
       OR EXISTS (SELECT 1 FROM public.episode_command_receipts WHERE operation='recurrence') THEN
        RAISE EXCEPTION 'cannot downgrade: accepted Episode recurrence relations exist';
    END IF;
END $$;
DROP TABLE public.episode_relations;
ALTER TABLE public.episode_command_receipts
    DROP CONSTRAINT episode_command_receipts_owner_identity;
ALTER TABLE public.episode_command_receipts
    DROP CONSTRAINT episode_command_receipts_operation_check;
ALTER TABLE public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_operation_check
    CHECK (operation IN ('create', 'link', 'status'));
