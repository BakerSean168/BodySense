-- #514 S3-A: protect source-linked narrative notes during statement erasure.
-- No writer/reader is enabled by this migration. Source asset handling remains
-- forbidden until a separate versioned asset provenance + erasure inventory.
ALTER TABLE public.statement_erasure_targets
    DROP CONSTRAINT statement_erasure_targets_target_check;
ALTER TABLE public.statement_erasure_targets
    ADD CONSTRAINT statement_erasure_targets_target_check
    CHECK (target IN ('diagnosis_records','training_plans','body_records',
        'assessment_reports','decision_records','runtime_checkpoints',
        'episodic_notes','source_message','statement_manifest'));

-- Existing nonterminal receipts also need this target before a worker may
-- complete; do not modify the immutable inventory_revision of old requests.
INSERT INTO public.statement_erasure_targets
    (user_id, request_id, target, state, updated_at)
SELECT user_id, id, 'episodic_notes', 'pending', now()
  FROM public.statement_erasure_requests
 WHERE status NOT IN ('erased','erased_with_retention')
ON CONFLICT (user_id, request_id, target) DO NOTHING;

-- The terminal DB guard requires all *nine* targets. An old worker that does
-- not know the new target cannot silently complete an erasure after upgrade.
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
           OR (NEW.status = 'erased' AND (verified <> 9 OR retained <> 0))
           OR (NEW.status = 'erased_with_retention' AND (verified + retained <> 9 OR retained < 1)) THEN
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

-- An episodic note must not be created or rewritten to cite an *erased*
-- statement's message. FOR SHARE serializes this admission with the statement
-- row's FOR UPDATE in erasure/finalize; a pre-finalize racing write is
-- rechecked/deleted by the Go worker, while a post-finalize write fails here.
-- No FK to raw messages is introduced: ordinary chat prune remains unblocked.
CREATE FUNCTION public.guard_episodic_note_erased_statement() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    PERFORM e.id FROM public.statement_evidence e
     WHERE e.user_id = NEW.user_id
       AND e.source_message_id = ANY(NEW.source_message_ids)
     ORDER BY e.id
     FOR SHARE;

    IF EXISTS (
        SELECT 1 FROM public.statement_evidence e
         WHERE e.user_id = NEW.user_id
           AND e.source_message_id = ANY(NEW.source_message_ids)
           AND e.availability = 'erased'
    ) THEN
        RAISE EXCEPTION 'episodic note source statement has been erased'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER episodic_notes_erased_statement_guard
    BEFORE INSERT OR UPDATE ON public.episodic_notes
    FOR EACH ROW EXECUTE FUNCTION public.guard_episodic_note_erased_statement();
