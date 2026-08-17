DROP INDEX IF EXISTS idx_messages_content_text_search;

ALTER TABLE training_logs
    DROP COLUMN IF EXISTS outcome_recorded_at,
    DROP COLUMN IF EXISTS intervention_id,
    DROP COLUMN IF EXISTS treatment_revision_id;

ALTER TABLE training_plans
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS treatment_revision_id,
    DROP COLUMN IF EXISTS treatment_id;

DROP TABLE IF EXISTS outcomes;
DROP TABLE IF EXISTS interventions;
DROP TABLE IF EXISTS treatment_revisions;
DROP TABLE IF EXISTS treatments;
DROP TABLE IF EXISTS diagnosis_analysis_freshness;
DROP TABLE IF EXISTS body_state_hypotheses;
DROP TABLE IF EXISTS body_state_evidence;
