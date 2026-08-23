-- 000054: Privacy erasure boundary
--
-- This migration closes user-ownership gaps introduced by the session/runtime
-- redesign and adds a durable, non-health-data erasure audit record. Published
-- migrations remain immutable; all constraints are added here.

-- Backfill review ownership from its strongest remaining parent before adding
-- a user FK. Reviews that are truly system/global may remain NULL.
UPDATE ai_output_reviews AS review
SET user_id = COALESCE(
    (SELECT run.user_id FROM runs AS run WHERE run.id = review.run_id),
    (SELECT job.user_id FROM jobs AS job WHERE job.id = review.job_id),
    (SELECT conversation.user_id FROM conversations AS conversation WHERE conversation.id = review.conversation_id)
)
WHERE review.user_id IS NULL
  AND (review.run_id IS NOT NULL OR review.job_id IS NOT NULL OR review.conversation_id IS NOT NULL);

-- Conservatively remove already-orphaned user-derived rows. These rows cannot
-- be served to a valid account and retaining their payload would defeat the
-- privacy boundary we are establishing.
DELETE FROM runs AS run
WHERE NOT EXISTS (SELECT 1 FROM users AS users WHERE users.id = run.user_id);

DELETE FROM conversations AS conversation
WHERE NOT EXISTS (SELECT 1 FROM users AS users WHERE users.id = conversation.user_id);

DELETE FROM jobs AS job
WHERE NOT EXISTS (SELECT 1 FROM users AS users WHERE users.id = job.user_id);

DELETE FROM ai_output_reviews AS review
WHERE review.user_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM users AS users WHERE users.id = review.user_id);

ALTER TABLE conversations
    ADD CONSTRAINT fk_conversations_user_erasure
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE runs
    ADD CONSTRAINT fk_runs_user_erasure
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE jobs
    ADD CONSTRAINT fk_jobs_user_erasure
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ai_output_reviews
    ADD CONSTRAINT fk_ai_output_reviews_user_erasure
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Diagnosis rollout comparison payload is derived from a user's analysis. It
-- must not survive privacy erasure as an anonymous-looking orphan containing
-- potentially sensitive semantic output.
DELETE FROM diagnosis_rollout_observations WHERE source_analysis_id IS NULL;

ALTER TABLE diagnosis_rollout_observations
    DROP CONSTRAINT diagnosis_rollout_observations_source_analysis_id_fkey;
ALTER TABLE diagnosis_rollout_observations
    ADD CONSTRAINT diagnosis_rollout_observations_source_analysis_id_fkey
    FOREIGN KEY (source_analysis_id) REFERENCES diagnosis_analyses(id) ON DELETE CASCADE;

CREATE TABLE privacy_erasure_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Present only while work is pending/retryable. Set NULL at completion so
    -- the durable audit row cannot be joined back to a deleted account.
    subject_user_id UUID,
    subject_digest CHAR(64) NOT NULL UNIQUE,
    status VARCHAR(24) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'retryable', 'completed')),
    attempt_count INTEGER NOT NULL DEFAULT 0,
    report JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error TEXT,
    lease_owner VARCHAR(120),
    lease_expires_at TIMESTAMPTZ,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_privacy_erasure_requests_active_subject
    ON privacy_erasure_requests(subject_user_id)
    WHERE subject_user_id IS NOT NULL;

CREATE INDEX idx_privacy_erasure_requests_recovery
    ON privacy_erasure_requests(status, lease_expires_at, updated_at)
    WHERE status IN ('pending', 'running', 'retryable');
