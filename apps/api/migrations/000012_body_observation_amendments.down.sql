-- A corrective or withdrawal receipt is an accepted user authorization.
-- Never pretend an old binary can replay history whose operation it cannot
-- represent; refuse this down-migration until a separate retention policy
-- has explicitly handled those receipts.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM public.body_observation_commits
     WHERE operation <> 'record') THEN
     RAISE EXCEPTION 'cannot downgrade: accepted BodyObservation amendments exist';
  END IF;
END $$;
DROP INDEX public.body_observation_receipts_owner_target;
ALTER TABLE public.body_observation_commits
  DROP CONSTRAINT body_observation_receipt_target_owner_fk,
  DROP CONSTRAINT body_observation_receipt_operation_target_check,
  DROP CONSTRAINT body_observation_receipt_operation_check,
  DROP COLUMN target_observation_id,
  DROP COLUMN operation;
