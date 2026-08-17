CREATE TABLE diagnosis_analyses (
    id                       UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body_state_revision      BIGINT NOT NULL,
    status                   VARCHAR(40) NOT NULL,
    scope                    VARCHAR(40) NOT NULL DEFAULT 'full_body',
    summary                  TEXT NOT NULL DEFAULT '',
    cross_concern_patterns   JSONB NOT NULL DEFAULT '[]'::jsonb,
    information_gaps         JSONB NOT NULL DEFAULT '[]'::jsonb,
    safety_summary           JSONB NOT NULL DEFAULT '{}'::jsonb,
    citations                JSONB NOT NULL DEFAULT '[]'::jsonb,
    governance               JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_output               JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_diagnosis_analyses_user_created
    ON diagnosis_analyses (user_id, created_at DESC);
CREATE INDEX idx_diagnosis_analyses_user_revision
    ON diagnosis_analyses (user_id, body_state_revision DESC);

CREATE TABLE diagnosis_candidates (
    id                         UUID PRIMARY KEY DEFAULT uuidv7(),
    analysis_id                UUID NOT NULL REFERENCES diagnosis_analyses(id) ON DELETE CASCADE,
    ordinal                    INT NOT NULL,
    concern_key                VARCHAR(120) NOT NULL DEFAULT '',
    name                       TEXT NOT NULL,
    confidence                 VARCHAR(20) NOT NULL,
    severity                   VARCHAR(20),
    evidence_strength          VARCHAR(20),
    impact                     TEXT,
    basis                      TEXT NOT NULL DEFAULT '',
    typical_symptoms           TEXT NOT NULL DEFAULT '',
    differential               TEXT,
    basis_fact_ids             JSONB NOT NULL DEFAULT '[]'::jsonb,
    basis_observation_ids      JSONB NOT NULL DEFAULT '[]'::jsonb,
    supporting_evidence_ids    JSONB NOT NULL DEFAULT '[]'::jsonb,
    counterevidence_ids        JSONB NOT NULL DEFAULT '[]'::jsonb,
    reasoning_summary          TEXT NOT NULL DEFAULT '',
    missing_information        JSONB NOT NULL DEFAULT '[]'::jsonb,
    safety_notes               JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_payload                JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (analysis_id, ordinal)
);

CREATE INDEX idx_diagnosis_candidates_analysis
    ON diagnosis_candidates (analysis_id, ordinal ASC);

CREATE TABLE diagnosis_candidate_assessments (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    analysis_id     UUID NOT NULL REFERENCES diagnosis_analyses(id) ON DELETE CASCADE,
    candidate_id    UUID NOT NULL REFERENCES diagnosis_candidates(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state           VARCHAR(30) NOT NULL,
    assessed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (candidate_id, user_id)
);

CREATE INDEX idx_diagnosis_candidate_assessments_user
    ON diagnosis_candidate_assessments (user_id, assessed_at DESC);
