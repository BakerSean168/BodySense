ALTER TABLE treatment_revisions
    ADD COLUMN generation_decision_trace JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN acceptance_decision_trace JSONB NOT NULL DEFAULT '{}'::jsonb;
