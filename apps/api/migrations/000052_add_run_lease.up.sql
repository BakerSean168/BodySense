ALTER TABLE runs
    ADD COLUMN lease_expires_at TIMESTAMPTZ;

-- Runs in the running state are lease-bound: a run whose lease expires without
-- renewal is assumed to belong to a dead process and may be reclaimed by the
-- next turn. Waiting-for-user and terminal runs are never reclaimed this way.
CREATE INDEX idx_runs_running_lease_expires_at
    ON runs (status, lease_expires_at)
    WHERE status = 'running';

-- Backfill leases for runs already running at migration time so legacy runs are
-- treated as live from here on (the runtime renews the lease as long as it
-- streams).
UPDATE runs
SET lease_expires_at = NOW() + INTERVAL '30 minutes'
WHERE status = 'running' AND lease_expires_at IS NULL;
