DROP TRIGGER IF EXISTS update_jobs_updated_at ON jobs;
DROP INDEX IF EXISTS idx_jobs_idempotency_key;
ALTER TABLE jobs
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS max_attempts,
    DROP COLUMN IF EXISTS updated_at;
