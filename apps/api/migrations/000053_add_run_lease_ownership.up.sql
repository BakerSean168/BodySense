ALTER TABLE runs
    ADD COLUMN lease_owner VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN lease_heartbeat_at TIMESTAMPTZ;

CREATE INDEX idx_runs_lease_owner ON runs (lease_owner) WHERE status = 'running';
