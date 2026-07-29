/**
 * Health Journey contract — mirrors `apps/api/internal/dto/health_journey.go`.
 *
 * The journey stage and its `available_actions` are derived server-side from
 * profile, uploads, consultation, assessment and training state. Clients render
 * what the backend reports; they must not re-derive the next step locally.
 */

export const JOURNEY_STAGES = [
  'profile_incomplete',
  'profile_ready',
  'assets_uploaded',
  'assessment_ready',
  'consulting',
  'diagnosis_ready',
  'plan_ready',
  'training_active',
  'reassessment_due',
  'completed',
] as const;

export type JourneyStage = (typeof JOURNEY_STAGES)[number];

export const JOURNEY_ACTIONS = [
  'complete_profile',
  'upload_report',
  'upload_photo',
  'start_assessment',
  'start_consultation',
  'continue_consultation',
  'request_analysis',
  'confirm_diagnosis',
  'generate_treatment',
  'view_treatment',
  'start_training',
  'view_progress',
  'log_training',
  'reassess',
  'review_summary',
] as const;

export type JourneyAction = (typeof JOURNEY_ACTIONS)[number];

/** Summary of what the user has completed so far in their journey. */
export interface JourneyArtifacts {
  has_profile: boolean;
  has_upload: boolean;
  has_consultation: boolean;
  has_diagnosis: boolean;
  has_treatment: boolean;
  has_training: boolean;
  needs_reassessment: boolean;
  has_reassessment: boolean;
  active_consultation_id?: string | null;
  latest_assessment_id?: string | null;
  active_training_plan_id?: string | null;
  missing_requirements?: string[] | null;
  derived_from?: string[] | null;
}

/** Response of `GET /api/v1/journey`. */
export interface HealthJourneyState {
  stage: JourneyStage;
  /** Human-readable reason the user is at this stage. */
  stage_reason: string;
  available_actions: JourneyAction[];
  artifacts: JourneyArtifacts;
}
