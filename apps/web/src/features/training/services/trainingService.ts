import type { Outcome } from "@/features/workspace";
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
  date: string;
  exercises: {
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
  plan_status: string;
}

export interface TrainingFeedbackResult {
  outcome?: Outcome;
}

export interface TrainingLogUpdateResponse {
  message: string;
  result: TrainingFeedbackResult;
}

function toTrainingPlan(input: PublicTrainingPlan): TrainingPlan {
  return {
    id: input.id,
    consultation_id: input.consultation_id,
    status: input.status,
    goal: input.goal,
    duration_weeks: input.duration_weeks,
    current_week: input.current_week,
    phases: input.phases,
    created_at: input.created_at,
  };
}

function toTrainingLog(input: PublicTrainingLog): TrainingLog {
  return {
    id: input.id,
    plan_id: input.plan_id,
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
    plan_status: input.plan_status,
  };
}

function toTrainingFeedbackResult(
  input: PublicTrainingFeedbackResult,
): TrainingFeedbackResult {
  return { outcome: input.outcome };
}

function toTrainingLogUpdateResponse(
  input: PublicTrainingLogUpdateResponse,
): TrainingLogUpdateResponse {
  return {
    message: input.message,
    result: toTrainingFeedbackResult(input.result),
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
