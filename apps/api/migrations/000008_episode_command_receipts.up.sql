-- A replayable receipt for one user-authorized Episode identity/link command.
-- Separates source/statement truth from the organizational Episode projection.
-- Only IDs and a fingerprint are retained; no copied user health text.
CREATE TABLE public.episode_command_receipts (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    idempotency_key varchar(128) NOT NULL CHECK (
        length(idempotency_key) BETWEEN 1 AND 128
        AND idempotency_key = btrim(idempotency_key)
    ),
    fingerprint char(64) NOT NULL CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
    operation text NOT NULL CHECK (operation IN ('create', 'link')),
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    revision_id uuid NOT NULL REFERENCES public.body_state_revisions(id) ON DELETE CASCADE,
    event_id uuid NOT NULL,
    committed_at timestamptz NOT NULL,
    UNIQUE (user_id, idempotency_key),
    FOREIGN KEY (user_id, episode_id)
      REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, fact_id)
      REFERENCES public.body_facts(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, revision)
      REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE,
    FOREIGN KEY (user_id, event_id)
      REFERENCES public.body_state_outbox(user_id, id) ON DELETE CASCADE
);
CREATE INDEX episode_command_receipts_owner_episode
    ON public.episode_command_receipts (user_id, episode_id, revision DESC);
