DROP INDEX IF EXISTS idx_agent_interactions_pending_expires;
ALTER TABLE agent_interactions DROP COLUMN IF EXISTS expires_at;
