DROP INDEX IF EXISTS idx_runs_agent_configuration;

ALTER TABLE runs
    DROP COLUMN IF EXISTS replay_input,
    DROP COLUMN IF EXISTS execution_provenance,
    DROP COLUMN IF EXISTS agent_configuration,
    DROP COLUMN IF EXISTS agent_configuration_id;
