-- BS-D9 bounded: distinct user-reported observation records, NOT an AI
-- hypothesis, diagnosis, device import or clinical-document measurement.
-- No dual write to legacy body_state_observations.
CREATE TABLE public.body_observations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    kind varchar(64) NOT NULL CHECK (kind ~ '^[a-z][a-z0-9_]{1,63}$'),
    raw_value text NOT NULL DEFAULT '' CHECK (char_length(raw_value) <= 120),
    raw_unit text NOT NULL DEFAULT '' CHECK (char_length(raw_unit) <= 48),
    method text NOT NULL DEFAULT '' CHECK (char_length(method) <= 240),
    measurement_context text NOT NULL DEFAULT '' CHECK (char_length(measurement_context) <= 500),
    observed_at jsonb NOT NULL CHECK (
        jsonb_typeof(observed_at) = 'object' AND observed_at ? 'precision'
        AND observed_at->>'precision' IN ('exact', 'day', 'range', 'approximate', 'unknown')
    ),
    source_evidence_id uuid NOT NULL,
    assertion_status text NOT NULL CHECK (assertion_status IN ('asserted','superseded','withdrawn')),
    supersedes_observation_id uuid,
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    recorded_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    -- Tie the receipt to this observation and its exact source as one identity.
    UNIQUE (user_id, id, source_evidence_id),
    CHECK (raw_value <> '' OR raw_unit = ''),
    CHECK (supersedes_observation_id IS NULL OR supersedes_observation_id <> id),
    FOREIGN KEY (user_id, source_evidence_id)
      REFERENCES public.statement_evidence(user_id,id),
    FOREIGN KEY (user_id, supersedes_observation_id)
      REFERENCES public.body_observations(user_id,id),
    FOREIGN KEY (user_id, created_revision)
      REFERENCES public.body_state_revisions(user_id,revision) ON DELETE CASCADE
);
CREATE UNIQUE INDEX body_observations_one_corrective_successor
    ON public.body_observations(user_id,supersedes_observation_id)
    WHERE supersedes_observation_id IS NOT NULL;
CREATE INDEX body_observations_owner_revision
    ON public.body_observations(user_id,created_revision DESC,id);
CREATE INDEX body_observations_owner_source
    ON public.body_observations(user_id,source_evidence_id);

-- Commit receipt is distinct from the existing Fact receipt. Its fingerprint
-- includes the original typed input; values are NOT copied to audit/outbox.
CREATE TABLE public.body_observation_commits (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    idempotency_key varchar(128) NOT NULL CHECK (
        char_length(idempotency_key) BETWEEN 1 AND 128
        AND idempotency_key = btrim(idempotency_key)
    ),
    fingerprint char(64) NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    observation_id uuid NOT NULL,
    source_evidence_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    revision_id uuid NOT NULL REFERENCES public.body_state_revisions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    committed_at timestamptz NOT NULL,
    UNIQUE(user_id,idempotency_key),
    FOREIGN KEY (user_id,observation_id,source_evidence_id)
      REFERENCES public.body_observations(user_id,id,source_evidence_id)
      ON DELETE CASCADE,
    FOREIGN KEY (user_id,revision)
      REFERENCES public.body_state_revisions(user_id,revision) ON DELETE CASCADE,
    FOREIGN KEY (user_id,event_id)
      REFERENCES public.body_state_outbox(user_id,id) ON DELETE CASCADE
);
CREATE INDEX body_observation_commits_owner_rev
    ON public.body_observation_commits(user_id,revision DESC);
