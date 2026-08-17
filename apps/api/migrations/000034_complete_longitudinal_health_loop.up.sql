-- Complete the ADR 0004 longitudinal domain after the BodyState/Diagnosis checkpoint.
-- Immutable reasoning artifacts are kept separate from mutable current status.

CREATE TABLE body_state_evidence (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type VARCHAR(40) NOT NULL,
    source_key TEXT NOT NULL,
    source_version TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    retrieved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, source_type, source_key, source_version)
);

CREATE INDEX idx_body_state_evidence_user_retrieved
    ON body_state_evidence(user_id, retrieved_at DESC);

CREATE TABLE body_state_hypotheses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concern_key VARCHAR(120) NOT NULL DEFAULT 'general',
    statement TEXT NOT NULL,
    lifecycle_state VARCHAR(30) NOT NULL DEFAULT 'active'
        CHECK (lifecycle_state IN ('active', 'strengthened', 'weakened', 'unsupported', 'retired')),
    confidence VARCHAR(20),
    supporting_fact_ids JSONB NOT NULL DEFAULT '[]',
    supporting_observation_ids JSONB NOT NULL DEFAULT '[]',
    supporting_evidence_ids JSONB NOT NULL DEFAULT '[]',
    counterevidence_ids JSONB NOT NULL DEFAULT '[]',
    source_analysis_id UUID REFERENCES diagnosis_analyses(id) ON DELETE SET NULL,
    provenance JSONB NOT NULL DEFAULT '{}',
    created_revision BIGINT NOT NULL,
    updated_revision BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_body_state_hypotheses_user_state
    ON body_state_hypotheses(user_id, lifecycle_state, updated_at DESC);
CREATE INDEX idx_body_state_hypotheses_concern
    ON body_state_hypotheses(user_id, concern_key);

CREATE TABLE diagnosis_analysis_freshness (
    analysis_id UUID PRIMARY KEY REFERENCES diagnosis_analyses(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state VARCHAR(30) NOT NULL DEFAULT 'fresh'
        CHECK (state IN ('fresh', 'potentially_stale', 'stale')),
    evaluated_against_revision BIGINT NOT NULL,
    reasons JSONB NOT NULL DEFAULT '[]',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_diagnosis_freshness_user_state
    ON diagnosis_analysis_freshness(user_id, state, checked_at DESC);

CREATE TABLE treatments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    current_revision INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'review_recommended'
        CHECK (status IN ('active', 'review_recommended', 'paused', 'superseded', 'completed')),
    source_body_state_revision BIGINT,
    source_diagnosis_analysis_id UUID REFERENCES diagnosis_analyses(id) ON DELETE SET NULL,
    status_reasons JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE treatment_revisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    treatment_id UUID NOT NULL REFERENCES treatments(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    acceptance_state VARCHAR(20) NOT NULL DEFAULT 'proposed'
        CHECK (acceptance_state IN ('proposed', 'accepted', 'rejected')),
    lifecycle_state VARCHAR(30) NOT NULL DEFAULT 'review_recommended'
        CHECK (lifecycle_state IN ('active', 'review_recommended', 'paused', 'superseded', 'completed')),
    source_body_state_revision BIGINT NOT NULL,
    source_diagnosis_analysis_id UUID NOT NULL REFERENCES diagnosis_analyses(id) ON DELETE RESTRICT,
    goal TEXT NOT NULL,
    duration_weeks INTEGER NOT NULL CHECK (duration_weeks > 0),
    plan JSONB NOT NULL,
    user_constraints JSONB NOT NULL DEFAULT '{}',
    evidence_ids JSONB NOT NULL DEFAULT '[]',
    governance JSONB NOT NULL DEFAULT '{}',
    change_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    UNIQUE (treatment_id, revision)
);

CREATE INDEX idx_treatment_revisions_treatment_created
    ON treatment_revisions(treatment_id, revision DESC);
CREATE INDEX idx_treatment_revisions_source_analysis
    ON treatment_revisions(source_diagnosis_analysis_id);

CREATE TABLE interventions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    treatment_id UUID NOT NULL REFERENCES treatments(id) ON DELETE CASCADE,
    treatment_revision_id UUID NOT NULL REFERENCES treatment_revisions(id) ON DELETE CASCADE,
    kind VARCHAR(40) NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    prescription JSONB NOT NULL DEFAULT '{}',
    position INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'proposed'
        CHECK (status IN ('proposed', 'active', 'paused', 'completed', 'cancelled', 'superseded')),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_interventions_user_status
    ON interventions(user_id, status, created_at DESC);
CREATE INDEX idx_interventions_revision
    ON interventions(treatment_revision_id, position);

CREATE TABLE outcomes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    treatment_id UUID REFERENCES treatments(id) ON DELETE SET NULL,
    treatment_revision_id UUID REFERENCES treatment_revisions(id) ON DELETE SET NULL,
    intervention_id UUID REFERENCES interventions(id) ON DELETE SET NULL,
    source_type VARCHAR(40) NOT NULL,
    source_key TEXT NOT NULL,
    kind VARCHAR(50) NOT NULL,
    concern_key VARCHAR(120) NOT NULL DEFAULT 'general',
    body_region TEXT NOT NULL DEFAULT '',
    value JSONB NOT NULL DEFAULT '{}',
    notes TEXT NOT NULL DEFAULT '',
    association_statement TEXT NOT NULL DEFAULT '',
    causality_level VARCHAR(30) NOT NULL DEFAULT 'association_only'
        CHECK (causality_level IN ('association_only', 'user_attributed', 'clinician_attributed')),
    occurred_at TIMESTAMPTZ NOT NULL,
    provenance JSONB NOT NULL DEFAULT '{}',
    body_state_revision BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, source_type, source_key)
);

CREATE INDEX idx_outcomes_user_occurred
    ON outcomes(user_id, occurred_at DESC);
CREATE INDEX idx_outcomes_treatment
    ON outcomes(treatment_id, occurred_at DESC);
CREATE INDEX idx_outcomes_concern
    ON outcomes(user_id, concern_key, occurred_at DESC);

ALTER TABLE training_plans
    ADD COLUMN treatment_id UUID REFERENCES treatments(id) ON DELETE SET NULL,
    ADD COLUMN treatment_revision_id UUID REFERENCES treatment_revisions(id) ON DELETE SET NULL,
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'active';

ALTER TABLE training_logs
    ADD COLUMN treatment_revision_id UUID REFERENCES treatment_revisions(id) ON DELETE SET NULL,
    ADD COLUMN intervention_id UUID REFERENCES interventions(id) ON DELETE SET NULL,
    ADD COLUMN outcome_recorded_at TIMESTAMPTZ;

CREATE UNIQUE INDEX idx_training_plans_treatment_revision
    ON training_plans(treatment_revision_id)
    WHERE treatment_revision_id IS NOT NULL;
CREATE UNIQUE INDEX idx_training_plans_user_active
    ON training_plans(user_id)
    WHERE status = 'active';
CREATE INDEX idx_training_logs_intervention
    ON training_logs(intervention_id, date DESC);

-- Backfill the plain-text search copy for historical messages created before
-- application code maintained content_text consistently. Extract in a CTE so
-- PostgreSQL never needs a LATERAL subquery to reference the UPDATE target row.
WITH extracted AS (
    SELECT
        message.id,
        string_agg(item.part->>'text', E'\n' ORDER BY item.ordinal) AS content
    FROM messages AS message
    CROSS JOIN LATERAL jsonb_array_elements(message.parts) WITH ORDINALITY
        AS item(part, ordinal)
    WHERE item.part->>'type' = 'text'
      AND COALESCE(item.part->>'text', '') <> ''
    GROUP BY message.id
)
UPDATE messages AS message
SET content_text = extracted.content
FROM extracted
WHERE extracted.id = message.id
  AND COALESCE(message.content_text, '') = ''
  AND COALESCE(extracted.content, '') <> '';

-- Supports bounded retrieval of old relevant messages without replaying the full transcript.
CREATE INDEX idx_messages_content_text_search
    ON messages USING GIN (to_tsvector('simple', COALESCE(content_text, '')));
