ALTER TABLE diagnosis_analyses
    ADD COLUMN agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN agent_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN decision_trace JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN execution_provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN evidence_acquisition_trace JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_diagnosis_analyses_agent_configuration
    ON diagnosis_analyses (agent_configuration_id, created_at DESC)
    WHERE agent_configuration_id <> '';

-- Preserve provenance already present in immutable historical raw_output without
-- inventing fields that old executions never recorded.
UPDATE diagnosis_analyses
SET
    agent_configuration_id = COALESCE(raw_output->'agent_configuration'->>'id', ''),
    agent_configuration = COALESCE(raw_output->'agent_configuration', '{}'::jsonb),
    execution_provenance = COALESCE(raw_output->'execution_provenance', '{}'::jsonb),
    evidence_acquisition_trace = COALESCE(raw_output->'evidence_acquisition', '{}'::jsonb)
WHERE raw_output IS NOT NULL;
