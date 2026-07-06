ALTER TABLE thread_projections
DROP COLUMN IF EXISTS health_features;

ALTER TABLE consultation_sessions
DROP COLUMN IF EXISTS health_features;
