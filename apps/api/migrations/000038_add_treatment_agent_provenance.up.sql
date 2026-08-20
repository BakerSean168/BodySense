ALTER TABLE treatment_revisions
    ADD COLUMN agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN agent_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN execution_provenance JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_treatment_revisions_agent_configuration
    ON treatment_revisions (agent_configuration_id, created_at DESC)
    WHERE agent_configuration_id <> '';
