ALTER TABLE user_uploads
    ADD COLUMN agent_configuration_id VARCHAR(80) NOT NULL DEFAULT '';

CREATE INDEX idx_user_uploads_agent_configuration
    ON user_uploads (agent_configuration_id)
    WHERE agent_configuration_id <> '';
