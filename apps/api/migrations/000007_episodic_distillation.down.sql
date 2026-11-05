-- #515 S2 rollback must not silently discard an unresolved distillation gap.
-- A pending or gapped row is the only record that a pruned segment was not
-- distilled. Refuse the downgrade until the owning accounts/conversations
-- have been erased (the cascade then empties the ledger).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.episodic_distillation_segments
         WHERE status IN ('pending', 'gapped') LIMIT 1
    ) THEN
        RAISE EXCEPTION 'cannot drop unresolvable episodic distillation gaps'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

DROP TABLE IF EXISTS public.episodic_distillation_segments;

ALTER TABLE public.conversations
    DROP CONSTRAINT IF EXISTS conversations_distilled_cursor_check;

ALTER TABLE public.conversations
    DROP COLUMN IF EXISTS distilled_through_seq;
