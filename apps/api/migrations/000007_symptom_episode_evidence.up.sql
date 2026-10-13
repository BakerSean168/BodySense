-- SF-20 foundation: stable symptom-experience identities are NOT diagnosis
-- entries and do not persist a guessed "ongoing/resolved" lifecycle.
-- No public write path is introduced in this migration. A later Go command
-- must authorize evidence-backed links and status claims transactionally.
CREATE UNIQUE INDEX body_facts_episode_link_source_identity
    ON public.body_facts (user_id, id, kind, source_evidence_id);

CREATE TABLE public.symptom_episodes (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    symptom_type text NOT NULL CHECK (char_length(btrim(symptom_type)) BETWEEN 1 AND 120),
    body_region_ids jsonb NOT NULL CHECK (
        jsonb_typeof(body_region_ids) = 'array' AND jsonb_array_length(body_region_ids) <= 20
    ),
    laterality text NOT NULL CHECK (
        laterality IN ('left', 'right', 'bilateral', 'not_applicable', 'unknown')
    ),
    identity_status text NOT NULL DEFAULT 'current'
        CHECK (identity_status IN ('current', 'superseded', 'invalidated')),
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    created_at timestamptz NOT NULL,
    UNIQUE (user_id, id),
    FOREIGN KEY (user_id, created_revision)
        REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE
);
CREATE INDEX symptom_episodes_owner_identity
    ON public.symptom_episodes(user_id, identity_status, created_revision DESC);

-- The exact original symptom Fact and its source must belong to this owner;
-- other categories cannot be linked as symptoms by changing an arbitrary FK.
CREATE TABLE public.episode_fact_links (
    user_id uuid NOT NULL,
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    fact_kind text NOT NULL DEFAULT 'symptom' CHECK (fact_kind = 'symptom'),
    basis_evidence_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('onset', 'observation', 'change', 'resolution')),
    association_method text NOT NULL CHECK (
        association_method IN ('explicit', 'inferred', 'user_corrected')
    ),
    is_primary boolean NOT NULL,
    authorization_id uuid,
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    PRIMARY KEY (user_id, episode_id, fact_id),
    UNIQUE (user_id, episode_id, fact_id, basis_evidence_id),
    CHECK (association_method <> 'user_corrected' OR authorization_id IS NOT NULL),
    FOREIGN KEY (user_id, episode_id)
        REFERENCES public.symptom_episodes(user_id, id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, fact_id, fact_kind, basis_evidence_id)
        REFERENCES public.body_facts(user_id, id, kind, source_evidence_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, created_revision)
        REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE
);
-- One symptom Fact may have one primary episode, plus explicit secondary links
-- for a future audited relation. Never duplicate the Fact to form a group.
CREATE UNIQUE INDEX episode_fact_one_primary
    ON public.episode_fact_links(user_id, fact_id) WHERE is_primary;
CREATE INDEX episode_fact_links_by_fact
    ON public.episode_fact_links(user_id, fact_id);

-- This row is an explicitly interpreted user claim, not an inference based on
-- silence, lack of new symptoms or a generic negative symptom Fact. The future
-- command must validate that the source text supports the exact claim.
-- The current projection is rebuilt from valid linked claims, not stored here.
CREATE TABLE public.episode_state_evidence (
    user_id uuid NOT NULL,
    episode_id uuid NOT NULL,
    fact_id uuid NOT NULL,
    source_evidence_id uuid NOT NULL,
    claim text NOT NULL CHECK (claim IN ('explicit_current_report', 'explicit_resolution')),
    as_of jsonb NOT NULL CHECK (
        jsonb_typeof(as_of) = 'object' AND as_of ? 'precision'
        AND as_of->>'precision' IN ('exact', 'day', 'range', 'approximate', 'unknown')
    ),
    created_revision bigint NOT NULL CHECK (created_revision > 0),
    PRIMARY KEY (user_id, episode_id, fact_id),
    FOREIGN KEY (user_id, episode_id, fact_id, source_evidence_id)
        REFERENCES public.episode_fact_links(user_id, episode_id, fact_id, basis_evidence_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id, created_revision)
        REFERENCES public.body_state_revisions(user_id, revision) ON DELETE CASCADE
);
