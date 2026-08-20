ALTER TABLE diagnosis_analyses
    ADD COLUMN replay_input JSONB NOT NULL DEFAULT '{}'::jsonb;
