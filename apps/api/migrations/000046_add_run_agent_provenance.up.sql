ALTER TABLE runs
    ADD COLUMN agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN agent_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN execution_provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN replay_input JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_runs_agent_configuration
    ON runs (agent_configuration_id, started_at DESC)
    WHERE agent_configuration_id <> '';
