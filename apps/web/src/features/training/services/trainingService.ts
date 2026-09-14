import type { Outcome, TreatmentRevision } from "@/features/workspace";
import {
  checkInTrainingPlan,
  getTrainingPlan,
  getTrainingProgress,
  getTrainingTodayTask,
  listTrainingPlans,
  reassessTrainingPlan,
  updateTrainingLog,
} from "@/generated/api/bodysense";
import type {
  TrainingFeedbackResultOutput as PublicTrainingFeedbackResult,
  TrainingLogOutput as PublicTrainingLog,
  TrainingLogUpdateResponseOutput as PublicTrainingLogUpdateResponse,
  TrainingPlanOutput as PublicTrainingPlan,
  TrainingProgressOutput as PublicTrainingProgress,
} from "@/generated/api/model";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export interface TrainingPlan {
  id: string;
  consultation_id?: string;
  treatment_id?: string;
  treatment_revision_id?: string;
  status: string;
  goal: string;
  duration_weeks: number;
  current_week: number;
  phases: TrainingPhase[];
  created_at: string;
}

export interface TrainingPhase {
  week: number;
  focus: string;
  exercises: TrainingExercise[];
}

export interface TrainingExercise {
  name: string;
  description: string;
  sets: string;
  reps: string;
  notes?: string;
}

export interface TrainingLog {
  id: string;
  plan_id: string;
  treatment_revision_id?: string;
  intervention_id?: string;
  date: string;
  exercises: {
    intervention_id?: string;
    name: string;
    completed: boolean;
  }[];
  notes?: string;
  is_checked_in: boolean;
  outcome_recorded_at?: string;
  created_at: string;
}

export interface TrainingProgress {
  consecutive_days: number;
  total_checkins: number;
  current_week: number;
  total_weeks: number;
  treatment_revision_id?: string | null;
  plan_status: string;
}

export interface TrainingFeedbackResult {
  outcome?: Outcome;
  treatment_status?: string;
  review_recommended?: boolean;
  paused?: boolean;
  proposal?: TreatmentRevision;
  has_proposal?: boolean;
  requires_new_diagnosis?: boolean;
}

export interface TrainingLogUpdateResponse {
  message: string;
  has_proposal: boolean;
  result: TrainingFeedbackResult;
  proposal?: TreatmentRevision;
}

function toTrainingPlan(input: PublicTrainingPlan): TrainingPlan {
  return {
    id: input.id,
    consultation_id: input.consultation_id,
    treatment_id: input.treatment_id,
    treatment_revision_id: input.treatment_revision_id,
    status: input.status,
    goal: input.goal,
    duration_weeks: input.duration_weeks,
    current_week: input.current_week,
    phases: input.phases as unknown as TrainingPhase[],
    created_at: input.created_at,
  };
}

function toTrainingLog(input: PublicTrainingLog): TrainingLog {
  return {
    id: input.id,
    plan_id: input.plan_id,
    treatment_revision_id: input.treatment_revision_id,
    intervention_id: input.intervention_id,
    date: input.date,
    exercises: input.exercises,
    notes: input.notes,
    is_checked_in: input.is_checked_in,
    outcome_recorded_at: input.outcome_recorded_at,
    created_at: input.created_at,
  };
}

function toTrainingProgress(input: PublicTrainingProgress): TrainingProgress {
  return {
    consecutive_days: input.consecutive_days,
    total_checkins: input.total_checkins,
    current_week: input.current_week,
    total_weeks: input.total_weeks,
    treatment_revision_id: input.treatment_revision_id,
    plan_status: input.plan_status,
  };
}

function toTrainingFeedbackResult(
  input: PublicTrainingFeedbackResult,
): TrainingFeedbackResult {
  return {
    outcome: input.outcome as Outcome | undefined,
    treatment_status: input.treatment_status,
    review_recommended: input.review_recommended,
    paused: input.paused,
    proposal: input.proposal as TreatmentRevision | undefined,
    has_proposal: input.has_proposal,
    requires_new_diagnosis: input.requires_new_diagnosis,
  };
}

function toTrainingLogUpdateResponse(
  input: PublicTrainingLogUpdateResponse,
): TrainingLogUpdateResponse {
  return {
    message: input.message,
    has_proposal: input.has_proposal,
    result: toTrainingFeedbackResult(input.result),
    proposal: input.proposal as TreatmentRevision | undefined,
  };
}

export const trainingApi = {
  getPlan: async (id: string): Promise<TrainingPlan> =>
    withOpenApiError(async () =>
      toTrainingPlan(await getTrainingPlan(id, undefined, openApiAuthFetch)),
    ),

  listPlans: async (): Promise<TrainingPlan[]> =>
    withOpenApiError(async () => {
      const response = await listTrainingPlans(undefined, openApiAuthFetch);
      return response.plans.map(toTrainingPlan);
    }),

  getTodayTask: async (planId: string): Promise<TrainingLog> =>
    withOpenApiError(async () =>
      toTrainingLog(
        await getTrainingTodayTask(planId, undefined, openApiAuthFetch),
      ),
    ),

  checkIn: async (planId: string): Promise<void> =>
    withOpenApiError(async () => {
      await checkInTrainingPlan(planId, undefined, openApiAuthFetch);
    }),

  updateLog: async (
    planId: string,
    notes: string,
    exercises: TrainingLog["exercises"],
  ): Promise<TrainingLogUpdateResponse> =>
    withOpenApiError(async () =>
      toTrainingLogUpdateResponse(
        await updateTrainingLog(
          planId,
          { notes, exercises },
          undefined,
          openApiAuthFetch,
        ),
      ),
    ),

  getProgress: async (planId: string): Promise<TrainingProgress> =>
    withOpenApiError(async () =>
      toTrainingProgress(
        await getTrainingProgress(planId, undefined, openApiAuthFetch),
      ),
    ),

  submitReassessment: async (
    planId: string,
    feedback: {
      symptom_changes: string;
      training_feeling: string;
      difficulties: string;
    },
  ): Promise<TrainingFeedbackResult> =>
    withOpenApiError(async () =>
      toTrainingFeedbackResult(
        await reassessTrainingPlan(
          planId,
          { feedback },
          undefined,
          openApiAuthFetch,
        ),
      ),
    ),
};
