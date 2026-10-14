-- A populated source-backed observation is user-authored health information.
-- Refuse a destructive down-migration, even if a historical receipt is absent.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM public.body_observations)
     OR EXISTS (SELECT 1 FROM public.body_observation_commits) THEN
    RAISE EXCEPTION 'cannot downgrade: accepted BodyObservation records exist';
  END IF;
END $$;
DROP TABLE public.body_observation_commits;
DROP TABLE public.body_observations;
