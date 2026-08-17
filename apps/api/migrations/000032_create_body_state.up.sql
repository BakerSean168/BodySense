CREATE TABLE body_states (
    user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_revision     BIGINT NOT NULL DEFAULT 0,
    safety_state         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE body_state_facts (
    id                     UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concern_key            VARCHAR(120) NOT NULL DEFAULT '',
    kind                   VARCHAR(80) NOT NULL,
    body_region            VARCHAR(120) NOT NULL DEFAULT '',
    value                  TEXT NOT NULL DEFAULT '',
    details                JSONB NOT NULL DEFAULT '{}'::jsonb,
    origin                 VARCHAR(40) NOT NULL,
    review_state           VARCHAR(40) NOT NULL DEFAULT 'unverified',
    lifecycle_state        VARCHAR(30) NOT NULL DEFAULT 'active',
    trend                  VARCHAR(30) NOT NULL DEFAULT 'unknown',
    source_key             TEXT NOT NULL DEFAULT '',
    provenance             JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at            TIMESTAMPTZ,
    valid_from             TIMESTAMPTZ,
    valid_until            TIMESTAMPTZ,
    supersedes_fact_id     UUID REFERENCES body_state_facts(id) ON DELETE SET NULL,
    excluded_from_reasoning BOOLEAN NOT NULL DEFAULT FALSE,
    created_revision       BIGINT NOT NULL,
    updated_revision       BIGINT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_body_state_facts_user_current
    ON body_state_facts (user_id, lifecycle_state, updated_at DESC);
CREATE INDEX idx_body_state_facts_user_concern
    ON body_state_facts (user_id, concern_key, updated_at DESC);
CREATE UNIQUE INDEX idx_body_state_facts_user_source_key
    ON body_state_facts (user_id, source_key)
    WHERE source_key <> '';

CREATE TABLE body_state_observations (
    id                     UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concern_key            VARCHAR(120) NOT NULL DEFAULT '',
    kind                   VARCHAR(80) NOT NULL,
    body_region            VARCHAR(120) NOT NULL DEFAULT '',
    method                 VARCHAR(80) NOT NULL DEFAULT '',
    value                  JSONB NOT NULL DEFAULT '{}'::jsonb,
    condition              JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_key             TEXT NOT NULL DEFAULT '',
    provenance             JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at            TIMESTAMPTZ,
    review_state           VARCHAR(40) NOT NULL DEFAULT 'unverified'
        CHECK (review_state IN ('unverified', 'confirmed', 'rejected')),
    lifecycle_state        VARCHAR(30) NOT NULL DEFAULT 'active',
    excluded_from_reasoning BOOLEAN NOT NULL DEFAULT TRUE,
    created_revision       BIGINT NOT NULL,
    updated_revision       BIGINT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_body_state_observations_user_current
    ON body_state_observations (user_id, lifecycle_state, review_state, updated_at DESC);
CREATE INDEX idx_body_state_observations_user_concern
    ON body_state_observations (user_id, concern_key, updated_at DESC);
CREATE UNIQUE INDEX idx_body_state_observations_user_source_key
    ON body_state_observations (user_id, source_key)
    WHERE source_key <> '';

CREATE TABLE body_state_revisions (
    id             UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision       BIGINT NOT NULL,
    change_type    VARCHAR(80) NOT NULL,
    source         VARCHAR(60) NOT NULL,
    changes        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, revision)
);

CREATE INDEX idx_body_state_revisions_user_revision
    ON body_state_revisions (user_id, revision DESC);
