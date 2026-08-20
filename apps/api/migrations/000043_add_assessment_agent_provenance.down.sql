DROP INDEX IF EXISTS idx_assessment_reports_agent_configuration;

ALTER TABLE assessment_reports
    DROP COLUMN IF EXISTS generation_decision_trace,
    DROP COLUMN IF EXISTS execution_provenance,
    DROP COLUMN IF EXISTS agent_configuration,
    DROP COLUMN IF EXISTS agent_configuration_id;
