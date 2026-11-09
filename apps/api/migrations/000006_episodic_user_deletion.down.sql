-- #516 S6-A rollback: no user-delete markers may be discarded.
-- A migration down with nonempty tombstones would enable generation of
-- user-deleted content, so reject until the owning accounts have been erased.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.episodic_note_deletions LIMIT 1) THEN
        RAISE EXCEPTION 'cannot drop episodic user-deletion tombstones'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

DROP TRIGGER IF EXISTS episodic_notes_user_deletion_guard ON public.episodic_notes;
DROP FUNCTION IF EXISTS public.guard_episodic_note_user_deletion();
DROP TABLE IF EXISTS public.episodic_note_deletions;
