DROP INDEX IF EXISTS idx_diagnosis_analyses_agent_configuration;

ALTER TABLE diagnosis_analyses
    DROP COLUMN IF EXISTS evidence_acquisition_trace,
    DROP COLUMN IF EXISTS execution_provenance,
    DROP COLUMN IF EXISTS decision_trace,
    DROP COLUMN IF EXISTS agent_configuration,
    DROP COLUMN IF EXISTS agent_configuration_id;
