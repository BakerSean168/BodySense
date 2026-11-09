-- #514 S3-B rollback restores the legacy eight-target asset contract.
DROP TRIGGER IF EXISTS episodic_notes_source_asset_guard ON public.episodic_notes;
DROP FUNCTION IF EXISTS public.guard_episodic_note_source_assets();

DELETE FROM public.asset_erasure_targets WHERE target = 'episodic_notes';
ALTER TABLE public.asset_erasure_targets
    DROP CONSTRAINT asset_erasure_targets_target_check;
ALTER TABLE public.asset_erasure_targets
    ADD CONSTRAINT asset_erasure_targets_target_check
    CHECK (target IN ('source_objects','derived_jobs','posture_analyses',
        'document_extractions','assessment_reports','diagnosis_records',
        'runtime_checkpoints','asset_manifest'));

CREATE OR REPLACE FUNCTION public.guard_asset_erasure_request() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'erased' THEN
        RAISE EXCEPTION 'asset erasure request is already erased' USING ERRCODE = '23514';
    END IF;
    IF NEW.status = 'erased' THEN
        IF (SELECT count(*) FROM public.asset_erasure_targets t
             WHERE t.user_id = NEW.user_id AND t.request_id = NEW.id AND t.state = 'verified') <> 8
           OR NOT EXISTS (SELECT 1 FROM public.user_assets a
             WHERE a.user_id = NEW.user_id AND a.id = NEW.asset_id AND a.availability = 'erased') THEN
            RAISE EXCEPTION 'asset erasure has unverified targets' USING ERRCODE = '23514';
        END IF;
    END IF;
    IF (NEW.id, NEW.user_id, NEW.asset_id, NEW.idempotency_key, NEW.inventory_revision, NEW.requested_at)
       IS DISTINCT FROM
       (OLD.id, OLD.user_id, OLD.asset_id, OLD.idempotency_key, OLD.inventory_revision, OLD.requested_at) THEN
        RAISE EXCEPTION 'asset erasure request identity is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

DROP INDEX IF EXISTS public.episodic_notes_source_asset_ids_idx;
ALTER TABLE public.episodic_notes DROP COLUMN IF EXISTS source_asset_ids;
