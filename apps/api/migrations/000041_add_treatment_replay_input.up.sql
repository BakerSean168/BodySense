ALTER TABLE treatment_revisions
    ADD COLUMN replay_input JSONB NOT NULL DEFAULT '{}'::jsonb;
