-- Explicit user correction preserves the original assertion and its evidence.
-- Temporal changes remain separate new facts; withdrawal is not privacy erasure.
ALTER TABLE public.body_facts
    ADD COLUMN supersedes_fact_id uuid,
    ADD CONSTRAINT body_facts_no_self_supersession
        CHECK (supersedes_fact_id IS NULL OR supersedes_fact_id <> id),
    ADD CONSTRAINT body_facts_supersession_owner_fk
        FOREIGN KEY (user_id, supersedes_fact_id)
        REFERENCES public.body_facts(user_id, id);
CREATE UNIQUE INDEX body_facts_one_corrective_successor
    ON public.body_facts (user_id, supersedes_fact_id)
    WHERE supersedes_fact_id IS NOT NULL;
