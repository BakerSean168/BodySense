-- Onboarding step 05 "目前哪些部位有不适感？": the user's own coarse answer.
-- A self-report kept apart from the BodyState ledger on purpose: it is not a
-- symptom record, a safety screen, an episode or a diagnosis, and nothing
-- derives clinical meaning from it. One row per user; a later onboarding
-- answer replaces it. The codes are onboarding areas, not BodyRegion ids;
-- "none" is an explicit answer and is only valid on its own.
CREATE TABLE public.reported_discomfort_areas (
    user_id uuid NOT NULL,
    area_codes jsonb NOT NULL,
    source text NOT NULL,
    reported_at timestamp with time zone NOT NULL,
    CONSTRAINT reported_discomfort_areas_pkey PRIMARY KEY (user_id),
    CONSTRAINT reported_discomfort_areas_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE,
    CONSTRAINT reported_discomfort_areas_source_check CHECK ((source = 'onboarding'::text)),
    CONSTRAINT reported_discomfort_areas_codes_check CHECK (((jsonb_typeof(area_codes) = 'array'::text) AND ((jsonb_array_length(area_codes) >= 1) AND (jsonb_array_length(area_codes) <= 7)) AND (area_codes <@ '["neck", "shoulder", "upper_back", "lower_back", "hip", "leg", "arm_wrist", "none"]'::jsonb) AND ((NOT (area_codes ? 'none'::text)) OR (jsonb_array_length(area_codes) = 1))))
);

COMMENT ON TABLE public.reported_discomfort_areas IS 'Onboarding self-report of current discomfort areas. Not a symptom, safety screen or diagnosis.';
