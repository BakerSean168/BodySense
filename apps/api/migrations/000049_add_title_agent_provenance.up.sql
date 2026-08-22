ALTER TABLE conversations
    ADD COLUMN title_agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN title_agent_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN title_execution_provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN title_decision_trace JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX idx_conversations_title_agent_configuration
    ON conversations (title_agent_configuration_id)
    WHERE title_agent_configuration_id <> '';
