DROP INDEX IF EXISTS idx_runs_running_lease_expires_at;

ALTER TABLE runs
    DROP COLUMN IF EXISTS lease_expires_at;
