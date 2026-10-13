-- Do not discard user-authored episode/link/status evidence in a down step.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM public.symptom_episodes)
       OR EXISTS (SELECT 1 FROM public.episode_fact_links)
       OR EXISTS (SELECT 1 FROM public.episode_state_evidence) THEN
        RAISE EXCEPTION 'cannot downgrade: symptom episode identity or evidence exists';
    END IF;
END $$;
DROP TABLE public.episode_state_evidence;
DROP INDEX public.episode_fact_one_primary;
DROP INDEX public.episode_fact_links_by_fact;
DROP TABLE public.episode_fact_links;
DROP INDEX public.symptom_episodes_owner_identity;
DROP TABLE public.symptom_episodes;
DROP INDEX public.body_facts_episode_link_source_identity;
