-- TP-D1 PREVIEW ONLY. A reverse migration cannot silently deactivate
-- legitimate multi-active plans to recreate the old uniqueness invariant.
-- Approval of history reconciliation and a roll-forward plan is mandatory.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.training_plans
      WHERE status = 'active'
      GROUP BY user_id HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'TP-D1 reverse blocked: multiple active plans require explicit owner-approved reconciliation';
  END IF;
  IF EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'training_plans'
      AND indexname = 'idx_training_plans_user_active'
  ) THEN
    RAISE EXCEPTION 'TP-D1 reverse blocked: legacy active index already exists';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'training_plans'
      AND indexname = 'idx_training_plans_treatment_revision'
  ) THEN
    RAISE EXCEPTION 'TP-D1 reverse prerequisite: TreatmentRevision index missing';
  END IF;
END $$;
CREATE UNIQUE INDEX idx_training_plans_user_active ON public.training_plans
  USING btree (user_id) WHERE ((status)::text = 'active'::text);
