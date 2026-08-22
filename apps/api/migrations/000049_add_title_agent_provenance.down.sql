DROP INDEX IF EXISTS idx_conversations_title_agent_configuration;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS title_decision_trace,
    DROP COLUMN IF EXISTS title_execution_provenance,
    DROP COLUMN IF EXISTS title_agent_configuration,
    DROP COLUMN IF EXISTS title_agent_configuration_id;
