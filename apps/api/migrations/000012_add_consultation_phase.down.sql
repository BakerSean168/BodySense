DROP INDEX IF EXISTS idx_consultation_sessions_phase;

ALTER TABLE consultation_sessions
DROP COLUMN IF EXISTS phase;
