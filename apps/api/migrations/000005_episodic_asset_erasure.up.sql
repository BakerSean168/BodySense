-- #514 S3-B: explicit asset provenance for inert episodic notes.
-- New asset-derived notes remain unavailable until Go source validation,
-- clinical policy and user controls are approved in S2/S4/S6.
ALTER TABLE public.episodic_notes
    ADD COLUMN source_asset_ids uuid[] NOT NULL DEFAULT '{}'::uuid[],
    ADD CONSTRAINT episodic_notes_asset_source_ids_check
        CHECK (cardinality(source_asset_ids) <= 32 AND
               array_position(source_asset_ids, NULL) IS NULL);

CREATE INDEX episodic_notes_source_asset_ids_idx
    ON public.episodic_notes USING gin (source_asset_ids);

-- Same lock contract as S3-A statement notes. An asset erasure takes FOR
-- UPDATE, so note admission cannot race the manifest and resurrect content.
-- Require every specified asset id to resolve to the same owner; duplicated
-- ids are rejected by the row-count mismatch.
CREATE FUNCTION public.guard_episodic_note_source_assets() RETURNS trigger
    LANGUAGE plpgsql AS $$
DECLARE
    matched integer;
BEGIN
    IF cardinality(NEW.source_asset_ids) = 0 THEN
        RETURN NEW;
    END IF;
    PERFORM a.id FROM public.user_assets a
      WHERE a.user_id = NEW.user_id
        AND a.id = ANY(NEW.source_asset_ids)
      ORDER BY a.id
      FOR SHARE;
    GET DIAGNOSTICS matched = ROW_COUNT;
    IF matched <> cardinality(NEW.source_asset_ids) THEN
        RAISE EXCEPTION 'episodic note references missing, foreign or repeated assets'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1 FROM public.user_assets a
         WHERE a.user_id = NEW.user_id
           AND a.id = ANY(NEW.source_asset_ids)
           AND a.availability = 'erased'
    ) THEN
        RAISE EXCEPTION 'episodic note source asset has been erased'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER episodic_notes_source_asset_guard
    BEFORE INSERT OR UPDATE ON public.episodic_notes
    FOR EACH ROW EXECUTE FUNCTION public.guard_episodic_note_source_assets();

ALTER TABLE public.asset_erasure_targets
    DROP CONSTRAINT asset_erasure_targets_target_check;
ALTER TABLE public.asset_erasure_targets
    ADD CONSTRAINT asset_erasure_targets_target_check
    CHECK (target IN ('source_objects','derived_jobs','posture_analyses',
        'document_extractions','assessment_reports','diagnosis_records',
        'runtime_checkpoints','episodic_notes','asset_manifest'));

INSERT INTO public.asset_erasure_targets
    (user_id, request_id, target, state, updated_at)
SELECT user_id, id, 'episodic_notes', 'pending', now()
  FROM public.asset_erasure_requests
 WHERE status <> 'erased'
ON CONFLICT (user_id, request_id, target) DO NOTHING;

-- Existing nonterminal v1 requests cannot pass a legacy eight-target terminal
-- check. The immutable inventory_revision remains the original admission ID.
CREATE OR REPLACE FUNCTION public.guard_asset_erasure_request() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'erased' THEN
        RAISE EXCEPTION 'asset erasure request is already erased' USING ERRCODE = '23514';
    END IF;
    IF NEW.status = 'erased' THEN
        IF (SELECT count(*) FROM public.asset_erasure_targets t
             WHERE t.user_id = NEW.user_id AND t.request_id = NEW.id AND t.state = 'verified') <> 9
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
