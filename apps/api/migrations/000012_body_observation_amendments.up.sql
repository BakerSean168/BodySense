-- Bounded BS-D9 user-authorized correction/withdrawal of direct-edit
-- BodyObservations. New observations continue to use v11 records; no
-- conversion of legacy body_state_observations or verification of devices.
ALTER TABLE public.body_observation_commits
  ADD COLUMN operation text NOT NULL DEFAULT 'record',
  ADD COLUMN target_observation_id uuid,
  ADD CONSTRAINT body_observation_receipt_operation_check
    CHECK (operation IN ('record', 'correct', 'withdraw')),
  ADD CONSTRAINT body_observation_receipt_operation_target_check
    CHECK (
      (operation = 'record' AND target_observation_id IS NULL)
      OR (operation = 'correct' AND target_observation_id IS NOT NULL
          AND observation_id <> target_observation_id)
      OR (operation = 'withdraw' AND target_observation_id IS NOT NULL
          AND observation_id = target_observation_id)
    ),
  ADD CONSTRAINT body_observation_receipt_target_owner_fk
    FOREIGN KEY (user_id, target_observation_id)
    REFERENCES public.body_observations(user_id, id)
    ON DELETE CASCADE;
CREATE INDEX body_observation_receipts_owner_target
  ON public.body_observation_commits(user_id,target_observation_id)
  WHERE target_observation_id IS NOT NULL;
