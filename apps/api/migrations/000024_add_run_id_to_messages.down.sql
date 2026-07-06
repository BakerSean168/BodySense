DROP INDEX IF EXISTS idx_messages_run_id;
ALTER TABLE messages DROP COLUMN IF EXISTS run_id;
