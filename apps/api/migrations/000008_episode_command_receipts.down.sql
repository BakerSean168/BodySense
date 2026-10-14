-- Dropping successful user commands would destroy exact idempotent replay
-- and durable evidence of authorization; refuse downgrade with live receipts.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM public.episode_command_receipts) THEN
        RAISE EXCEPTION 'cannot downgrade: episode command receipts exist';
    END IF;
END $$;
DROP TABLE public.episode_command_receipts;
