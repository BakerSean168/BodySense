-- Refuse to discard a live correction chain. A rollback with data requires an
-- explicit migration/reset plan, not silent provenance loss.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.body_facts WHERE supersedes_fact_id IS NOT NULL) THEN
        RAISE EXCEPTION 'Cannot roll back BodyFact corrections while correction history exists';
    END IF;
END $$;
DROP INDEX IF EXISTS public.body_facts_one_corrective_successor;
ALTER TABLE public.body_facts
    DROP CONSTRAINT IF EXISTS body_facts_supersession_owner_fk,
    DROP CONSTRAINT IF EXISTS body_facts_no_self_supersession,
    DROP COLUMN IF EXISTS supersedes_fact_id;
