-- Approved BodyState assertion model (BS-D1..D10, PC-D6, CD-D1..D3).
-- Additive transition: one existing body_states aggregate/revision sequence;
-- no dual-writing or reinterpretation of legacy body_state_facts.
-- Qualify every relation: baseline 000001 leaves search_path empty on the
-- migration connection. A fresh psql process would hide that session boundary.
CREATE TABLE public.statement_evidence (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    source_kind text NOT NULL CHECK (source_kind IN ('direct_edit', 'user_message', 'structured_answer')),
    source_version bigint NOT NULL CHECK (source_version > 0),
    excerpt text NOT NULL CHECK (char_length(excerpt) <= 2000),
    availability text NOT NULL CHECK (availability IN ('available', 'blocked', 'erased')),
    recorded_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    CHECK (availability <> 'erased' OR excerpt = '')
);

CREATE TABLE public.body_facts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    subject text NOT NULL CHECK (subject = 'self'),
    kind text NOT NULL CHECK (kind IN ('symptom', 'health_history', 'activity', 'lifestyle', 'training_feedback')),
    statement text NOT NULL CHECK (char_length(statement) BETWEEN 1 AND 2000 AND char_length(btrim(statement)) > 0),
    body_region_ids jsonb NOT NULL CHECK (jsonb_typeof(body_region_ids) = 'array' AND jsonb_array_length(body_region_ids) <= 20),
    temporal jsonb NOT NULL CHECK (jsonb_typeof(temporal) = 'object' AND temporal ? 'precision'
      AND jsonb_typeof(temporal->'precision') = 'string'
      AND temporal->>'precision' IN ('exact', 'day', 'range', 'approximate', 'unknown')),
    details jsonb NOT NULL CHECK (jsonb_typeof(details) = 'object'),
    source_evidence_id uuid NOT NULL,
    assertion_status text NOT NULL CHECK (assertion_status IN ('asserted', 'superseded', 'withdrawn')),
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    recorded_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    FOREIGN KEY (user_id, source_evidence_id) REFERENCES public.statement_evidence(user_id, id),
    FOREIGN KEY (user_id, created_revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE
);
CREATE INDEX body_facts_user_revision ON public.body_facts(user_id, created_revision DESC, id);
CREATE INDEX body_facts_source ON public.body_facts(user_id, source_evidence_id);

CREATE TABLE public.body_state_outbox (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    revision bigint NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE
);
CREATE INDEX body_state_outbox_replay ON public.body_state_outbox(user_id, revision, id);

CREATE TABLE public.body_fact_commits (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    idempotency_key varchar(128) NOT NULL CHECK (length(trim(idempotency_key)) > 0),
    fingerprint char(64) NOT NULL,
    revision bigint NOT NULL,
    revision_id uuid NOT NULL REFERENCES public.body_state_revisions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    fact_ids jsonb NOT NULL CHECK (jsonb_typeof(fact_ids) = 'array' AND jsonb_array_length(fact_ids) BETWEEN 1 AND 20),
    committed_at timestamptz NOT NULL,
    UNIQUE (user_id, idempotency_key),
    FOREIGN KEY (user_id, revision) REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE,
    FOREIGN KEY (user_id, event_id) REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE
);
