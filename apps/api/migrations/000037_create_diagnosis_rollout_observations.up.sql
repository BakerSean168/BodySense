CREATE TABLE diagnosis_rollout_observations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_analysis_id UUID REFERENCES diagnosis_analyses(id) ON DELETE SET NULL,
    stage VARCHAR(24) NOT NULL,
    subject_bucket INTEGER NOT NULL CHECK (subject_bucket >= 0 AND subject_bucket < 10000),
    canary_bps INTEGER NOT NULL DEFAULT 0 CHECK (canary_bps >= 0 AND canary_bps <= 10000),
    champion_configuration_id VARCHAR(80) NOT NULL,
    challenger_configuration_id VARCHAR(80) NOT NULL,
    served_configuration_id VARCHAR(80) NOT NULL,
    shadow_configuration_id VARCHAR(80) NOT NULL DEFAULT '',
    comparison JSONB NOT NULL DEFAULT '{}'::jsonb,
    unsafe_relaxation BOOLEAN NOT NULL DEFAULT FALSE,
    forbidden_side_effect BOOLEAN NOT NULL DEFAULT FALSE,
    configuration_mismatch BOOLEAN NOT NULL DEFAULT FALSE,
    shadow_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_diagnosis_rollout_observations_pair_stage
    ON diagnosis_rollout_observations (champion_configuration_id, challenger_configuration_id, stage, canary_bps, created_at DESC);
