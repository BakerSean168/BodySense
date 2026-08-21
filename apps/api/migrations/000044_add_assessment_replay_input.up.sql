ALTER TABLE assessment_reports
    ADD COLUMN replay_input JSONB NOT NULL DEFAULT '{}'::jsonb;
