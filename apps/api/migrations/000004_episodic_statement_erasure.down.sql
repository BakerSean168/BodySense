-- #514 S3-A rollback: remove the new target before restoring the older
-- eight-target guard; never reopen an already terminal erasure.
DROP TRIGGER IF EXISTS episodic_notes_erased_statement_guard ON public.episodic_notes;
DROP FUNCTION IF EXISTS public.guard_episodic_note_erased_statement();

DELETE FROM public.statement_erasure_targets WHERE target = 'episodic_notes';

ALTER TABLE public.statement_erasure_targets
    DROP CONSTRAINT statement_erasure_targets_target_check;
ALTER TABLE public.statement_erasure_targets
    ADD CONSTRAINT statement_erasure_targets_target_check
    CHECK (target IN ('diagnosis_records','training_plans','body_records',
        'assessment_reports','decision_records','runtime_checkpoints',
        'source_message','statement_manifest'));

CREATE OR REPLACE FUNCTION public.guard_statement_erasure_request() RETURNS trigger
    LANGUAGE plpgsql AS $$
DECLARE
    verified integer;
    retained integer;
BEGIN
    IF OLD.status IN ('erased','erased_with_retention') THEN
        RAISE EXCEPTION 'statement erasure request is already terminal' USING ERRCODE = '23514';
    END IF;
    IF NEW.status IN ('erased','erased_with_retention') THEN
        SELECT count(*) FILTER (WHERE t.state = 'verified'), count(*) FILTER (WHERE t.state = 'retained_by_policy')
          INTO verified, retained
          FROM public.statement_erasure_targets t WHERE t.user_id = NEW.user_id AND t.request_id = NEW.id;
        IF NOT EXISTS (SELECT 1 FROM public.statement_evidence e
             WHERE e.user_id = NEW.user_id AND e.id = NEW.statement_id AND e.availability = 'erased')
           OR (NEW.status = 'erased' AND (verified <> 8 OR retained <> 0))
           OR (NEW.status = 'erased_with_retention' AND (verified + retained <> 8 OR retained < 1)) THEN
            RAISE EXCEPTION 'statement erasure has unverified targets' USING ERRCODE = '23514';
        END IF;
    END IF;
    IF (NEW.id, NEW.user_id, NEW.statement_id, NEW.idempotency_key, NEW.inventory_revision, NEW.requested_at)
       IS DISTINCT FROM
       (OLD.id, OLD.user_id, OLD.statement_id, OLD.idempotency_key, OLD.inventory_revision, OLD.requested_at) THEN
        RAISE EXCEPTION 'statement erasure request identity is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
