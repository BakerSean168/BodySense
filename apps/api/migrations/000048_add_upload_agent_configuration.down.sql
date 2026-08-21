DROP INDEX IF EXISTS idx_user_uploads_agent_configuration;

ALTER TABLE user_uploads
    DROP COLUMN IF EXISTS agent_configuration_id;
