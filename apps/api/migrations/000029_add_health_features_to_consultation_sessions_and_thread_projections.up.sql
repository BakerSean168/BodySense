ALTER TABLE consultation_sessions
ADD COLUMN health_features JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE thread_projections
ADD COLUMN health_features JSONB NOT NULL DEFAULT '{}'::jsonb;
