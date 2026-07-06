ALTER TABLE thread_projections
ADD COLUMN interaction_history JSONB NOT NULL DEFAULT '[]';
