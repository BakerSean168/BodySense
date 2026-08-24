DROP TABLE IF EXISTS privacy_erasure_requests;

ALTER TABLE diagnosis_rollout_observations
    DROP CONSTRAINT IF EXISTS diagnosis_rollout_observations_source_analysis_id_fkey;
ALTER TABLE diagnosis_rollout_observations
    ADD CONSTRAINT diagnosis_rollout_observations_source_analysis_id_fkey
    FOREIGN KEY (source_analysis_id) REFERENCES diagnosis_analyses(id) ON DELETE SET NULL;

ALTER TABLE ai_output_reviews DROP CONSTRAINT IF EXISTS fk_ai_output_reviews_user_erasure;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS fk_jobs_user_erasure;
ALTER TABLE runs DROP CONSTRAINT IF EXISTS fk_runs_user_erasure;
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS fk_conversations_user_erasure;
