-- 000058: Promote mutable health context out of user_profiles and into BodyState.
--
-- BodySense is still in rapid development and does not preserve the temporary
-- profile-as-health-record contract. user_profiles now owns stable identity only.
-- Lifestyle facts and anthropometric observations use BodyState temporal history.

ALTER TABLE body_state_observations
    ADD COLUMN supersedes_observation_id UUID REFERENCES body_state_observations(id) ON DELETE SET NULL;

CREATE INDEX idx_body_state_observations_supersedes
    ON body_state_observations (supersedes_observation_id)
    WHERE supersedes_observation_id IS NOT NULL;

-- Lifestyle sections are singleton current facts. Historical rows remain in the
-- same table after being marked inactive and linked by supersedes_fact_id.
CREATE UNIQUE INDEX idx_body_state_facts_active_context_kind
    ON body_state_facts (user_id, kind)
    WHERE lifecycle_state = 'active'
      AND review_state = 'confirmed'
      AND excluded_from_reasoning = FALSE
      AND kind IN (
        'lifestyle.activity',
        'lifestyle.sleep',
        'lifestyle.exercise',
        'lifestyle.nutrition',
        'lifestyle.substances',
        'lifestyle.recovery',
        'history.injury_summary'
      );

-- Height and weight are measurements, not identity attributes. Only one current
-- observation of each kind is active; replacements retain historical rows.
CREATE UNIQUE INDEX idx_body_state_observations_active_anthropometry_kind
    ON body_state_observations (user_id, kind)
    WHERE lifecycle_state = 'active'
      AND excluded_from_reasoning = FALSE
      AND kind IN ('anthropometry.height', 'anthropometry.weight');

ALTER TABLE user_profiles
    DROP COLUMN age,
    DROP COLUMN height_cm,
    DROP COLUMN weight_kg,
    DROP COLUMN bmi,
    DROP COLUMN occupation,
    DROP COLUMN sleep_time,
    DROP COLUMN wake_time,
    DROP COLUMN exercise_type,
    DROP COLUMN exercise_frequency,
    DROP COLUMN injury_history,
    DROP COLUMN self_description,
    DROP COLUMN schedule_mode,
    DROP COLUMN activity_pattern,
    DROP COLUMN sleep_pattern;
