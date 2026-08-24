DROP INDEX IF EXISTS idx_runs_lease_owner;

ALTER TABLE runs
    DROP COLUMN IF EXISTS lease_heartbeat_at,
    DROP COLUMN IF EXISTS lease_owner;
