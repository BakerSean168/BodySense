-- SF-20 bounded: exact user-reported recurrence relation between two already
-- separate source-backed Episode identities. It is NOT a diagnosis, automatic
-- recurrence detection, merge or split, and it cannot reopen an ended Episode.
ALTER TABLE public.episode_command_receipts
    DROP CONSTRAINT episode_command_receipts_operation_check;
ALTER TABLE public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_operation_check
    CHECK (operation IN ('create', 'link', 'status', 'recurrence'));

-- Composite owner-bound authorization reference, needed for the relation to
-- point at the exact durable direct-user command. The PK already implies ID
-- uniqueness, but cannot serve a composite (user_id,id) foreign key.
ALTER TABLE public.episode_command_receipts
    ADD CONSTRAINT episode_command_receipts_owner_identity UNIQUE (user_id, id);

CREATE TABLE public.episode_relations (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    from_episode_id uuid NOT NULL,
    to_episode_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind = 'recurrence_reported'),
    basis_fact_id uuid NOT NULL,
    basis_evidence_id uuid NOT NULL,
    authorization_id uuid NOT NULL,
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    UNIQUE (user_id, from_episode_id, to_episode_id, kind),
    UNIQUE (user_id, authorization_id),
    CHECK (from_episode_id <> to_episode_id),
    FOREIGN KEY (user_id, from_episode_id)
        REFERENCES public.symptom_episodes(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, to_episode_id)
        REFERENCES public.symptom_episodes(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,to_episode_id,basis_fact_id,basis_evidence_id)
        REFERENCES public.episode_fact_links(user_id,episode_id,fact_id,basis_evidence_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,authorization_id)
        REFERENCES public.episode_command_receipts(user_id,id) ON DELETE CASCADE,
    FOREIGN KEY (user_id,created_revision)
        REFERENCES public.body_state_revisions(user_id,revision) ON DELETE CASCADE
);
CREATE INDEX episode_relations_owner_from
    ON public.episode_relations(user_id,from_episode_id,created_revision DESC);
CREATE INDEX episode_relations_owner_to
    ON public.episode_relations(user_id,to_episode_id,created_revision DESC);
