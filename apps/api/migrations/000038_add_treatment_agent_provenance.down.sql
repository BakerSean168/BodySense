DROP INDEX IF EXISTS idx_treatment_revisions_agent_configuration;

ALTER TABLE treatment_revisions
    DROP COLUMN IF EXISTS execution_provenance,
    DROP COLUMN IF EXISTS agent_configuration,
    DROP COLUMN IF EXISTS agent_configuration_id;
