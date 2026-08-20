ALTER TABLE treatment_revisions
    ADD COLUMN evidence_acquisition_trace JSONB NOT NULL DEFAULT '{}'::jsonb;
