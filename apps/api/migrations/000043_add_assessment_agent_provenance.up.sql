ALTER TABLE assessment_reports
    ADD COLUMN agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN agent_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN execution_provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN generation_decision_trace JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_assessment_reports_agent_configuration
    ON assessment_reports (agent_configuration_id, created_at DESC)
    WHERE agent_configuration_id <> '';
