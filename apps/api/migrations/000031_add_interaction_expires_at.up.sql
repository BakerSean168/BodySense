-- T0-1: HITL interaction lifecycle — pending ask_user rows expire after a TTL.
ALTER TABLE agent_interactions
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

-- Backfill existing pending rows to expire 24h after creation.
UPDATE agent_interactions
SET expires_at = created_at + INTERVAL '24 hours'
WHERE status = 'pending' AND expires_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_agent_interactions_pending_expires
  ON agent_interactions (expires_at)
  WHERE status = 'pending' AND expires_at IS NOT NULL;
