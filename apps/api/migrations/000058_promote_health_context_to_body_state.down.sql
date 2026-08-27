ALTER TABLE user_profiles
    ADD COLUMN age INTEGER,
    ADD COLUMN height_cm DECIMAL(5,1),
    ADD COLUMN weight_kg DECIMAL(5,1),
    ADD COLUMN bmi DECIMAL(4,1),
    ADD COLUMN occupation VARCHAR(100),
    ADD COLUMN sleep_time TIME,
    ADD COLUMN wake_time TIME,
    ADD COLUMN exercise_type VARCHAR(100),
    ADD COLUMN exercise_frequency VARCHAR(50),
    ADD COLUMN injury_history TEXT,
    ADD COLUMN self_description TEXT,
    ADD COLUMN schedule_mode VARCHAR(20) DEFAULT 'fixed_calendar',
    ADD COLUMN activity_pattern TEXT,
    ADD COLUMN sleep_pattern TEXT;

DROP INDEX IF EXISTS idx_body_state_observations_active_anthropometry_kind;
DROP INDEX IF EXISTS idx_body_state_facts_active_context_kind;
DROP INDEX IF EXISTS idx_body_state_observations_supersedes;
ALTER TABLE body_state_observations DROP COLUMN IF EXISTS supersedes_observation_id;
