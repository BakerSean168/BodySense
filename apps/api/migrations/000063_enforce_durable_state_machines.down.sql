-- Removing constraints does not restore retired aliases or rewrite durable data.
ALTER TABLE user_uploads
    DROP CONSTRAINT IF EXISTS user_uploads_analysis_status_check,
    DROP CONSTRAINT IF EXISTS user_uploads_ocr_status_check;

ALTER TABLE thread_projections
    DROP CONSTRAINT IF EXISTS thread_projections_phase_check;

ALTER TABLE consultation_sessions
    DROP CONSTRAINT IF EXISTS consultation_sessions_phase_check;

ALTER TABLE agent_tool_calls
    DROP CONSTRAINT IF EXISTS agent_tool_calls_status_check;

ALTER TABLE agent_interactions
    DROP CONSTRAINT IF EXISTS agent_interactions_status_check;

DROP INDEX IF EXISTS idx_runs_one_active_per_conversation;
CREATE UNIQUE INDEX idx_runs_one_running_per_conversation
    ON runs (conversation_id)
    WHERE status = 'running';

ALTER TABLE runs
    DROP CONSTRAINT IF EXISTS runs_status_check;

ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_status_check;
