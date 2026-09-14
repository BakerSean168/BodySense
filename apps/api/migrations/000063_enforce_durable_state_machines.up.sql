-- Phase 06: make durable lifecycle vocabularies finite at the database boundary.
-- This project is still pre-user, so obsolete development aliases are normalized
-- in place rather than preserved through compatibility reads/writes.

UPDATE jobs
SET status = 'completed'
WHERE status = 'succeeded';

UPDATE consultation_sessions
SET phase = 'ready_for_analysis'
WHERE phase = 'analysis_ready';

UPDATE thread_projections
SET phase = 'ready_for_analysis'
WHERE phase = 'analysis_ready';

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_status_check,
    ADD CONSTRAINT jobs_status_check
        CHECK (status IN ('pending', 'running', 'waiting_user', 'completed', 'failed', 'cancelled', 'timed_out'));

ALTER TABLE runs
    DROP CONSTRAINT IF EXISTS runs_status_check,
    ADD CONSTRAINT runs_status_check
        CHECK (status IN ('running', 'waiting_user', 'completed', 'failed', 'cancelled'));

-- waiting_user still owns the active turn. Prevent a second run from being
-- created merely because the first run released its execution lease for HITL.
DROP INDEX IF EXISTS idx_runs_one_running_per_conversation;
CREATE UNIQUE INDEX idx_runs_one_active_per_conversation
    ON runs (conversation_id)
    WHERE status IN ('running', 'waiting_user');

ALTER TABLE agent_interactions
    DROP CONSTRAINT IF EXISTS agent_interactions_status_check,
    ADD CONSTRAINT agent_interactions_status_check
        CHECK (status IN ('pending', 'answered', 'cancelled', 'expired'));

ALTER TABLE agent_tool_calls
    DROP CONSTRAINT IF EXISTS agent_tool_calls_status_check,
    ADD CONSTRAINT agent_tool_calls_status_check
        CHECK (status IN ('running', 'succeeded', 'failed'));

ALTER TABLE consultation_sessions
    DROP CONSTRAINT IF EXISTS consultation_sessions_phase_check,
    ADD CONSTRAINT consultation_sessions_phase_check
        CHECK (phase IN ('collecting', 'ready_for_analysis'));

ALTER TABLE thread_projections
    DROP CONSTRAINT IF EXISTS thread_projections_phase_check,
    ADD CONSTRAINT thread_projections_phase_check
        CHECK (phase IN ('collecting', 'ready_for_analysis'));

ALTER TABLE user_uploads
    DROP CONSTRAINT IF EXISTS user_uploads_ocr_status_check,
    ADD CONSTRAINT user_uploads_ocr_status_check
        CHECK (ocr_status IN ('pending', 'processing', 'completed', 'failed')),
    DROP CONSTRAINT IF EXISTS user_uploads_analysis_status_check,
    ADD CONSTRAINT user_uploads_analysis_status_check
        CHECK (analysis_status IN ('none', 'pending', 'processing', 'completed', 'failed'));
