import {
  finishTrainingSession,
  getTrainingPlanRecord,
  listTrainingPlanRecords,
  listTrainingSessions,
  recordTrainingSessionFeedback,
  recordTrainingSetAction,
  startTrainingSession,
} from "@/generated/api/bodysense";
import type {
  PlanExerciseOutput,
  TrainingPlanRecordOutput,
  TrainingSessionRecordOutput,
  TrainingSetActionRecordOutput,
} from "@/generated/api/model";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

/**
 * Plan list item for the TrainingPlan window, projected from the accepted
 * `/training-plans` record (TP-002). Each session template becomes one
 * `phase`; its stage structure is bound by UI-TP-001.
 */
export interface TrainingPlan {
  id: string;
  status: string;
  goal: string;
  duration_weeks?: number;
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

export type TrainingPlanRecord = TrainingPlanRecordOutput;
export type TrainingSessionRecord = TrainingSessionRecordOutput;
export type TrainingSetActionRecord = TrainingSetActionRecordOutput;

/** "8–12 次", "30 秒", "400 米": the prescribed target, never a guess. */
export function formatPlanTarget(exercise: PlanExerciseOutput): string {
  const target = exercise.target;
  if (target.kind === "repetitions" && target.min !== undefined) {
    return target.max !== undefined && target.max !== target.min
      ? `${target.min}–${target.max} 次`
      : `${target.min} 次`;
  }
  if (target.kind === "duration" && target.seconds !== undefined) {
    return `${target.seconds} 秒`;
  }
  if (target.kind === "distance" && target.meters !== undefined) {
    return `${target.meters} 米`;
  }
  return "";
}

function toTrainingPlan(record: TrainingPlanRecordOutput): TrainingPlan {
  return {
    id: record.planId,
    status: record.status,
    goal: record.goal || record.title,
    phases: record.currentContent.sessions.map((template, index) => ({
      week: index + 1,
      focus: template.title,
      exercises: template.stages.flatMap((stage) =>
        stage.exercises.map((exercise) => ({
          name: exercise.nameSnapshot,
          description: exercise.instructions ?? "",
          sets: String(exercise.sets),
          reps: formatPlanTarget(exercise),
          notes: exercise.loadOrIntensity,
        })),
      ),
    })),
    created_at: record.createdAt,
  };
}

function newIdempotencyKey(): string {
  return globalThis.crypto.randomUUID();
}

/** The user's IANA zone; the server fixes the training date from it. */
export function userTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export const trainingApi = {
  listPlans: async (): Promise<TrainingPlan[]> =>
    withOpenApiError(async () => {
      const response = await listTrainingPlanRecords(
        undefined,
        openApiAuthFetch,
      );
      return response.plans.map(toTrainingPlan);
    }),

  getPlanRecord: async (planId: string): Promise<TrainingPlanRecord> =>
    withOpenApiError(() =>
      getTrainingPlanRecord(planId, undefined, openApiAuthFetch),
    ),

  listSessions: async (): Promise<TrainingSessionRecord[]> =>
    withOpenApiError(async () => {
      const response = await listTrainingSessions(
        undefined,
        undefined,
        openApiAuthFetch,
      );
      return response.sessions;
    }),

  startSession: async (
    plan: TrainingPlanRecord,
    templateId: string,
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      startTrainingSession(
        {
          planId: plan.planId,
          expectedPlanRevisionId: plan.currentAcceptedRevisionId,
          templateId,
          timeZone: userTimeZone(),
          expectedSafetyRevision: plan.executionGate.safetyRevision,
          idempotencyKey: newIdempotencyKey(),
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  recordSet: async (
    session: TrainingSessionRecord,
    planExerciseId: string,
    setNumber: number,
    kind: "performed" | "skipped",
    expectedSafetyRevision: number,
  ): Promise<TrainingSetActionRecord> =>
    withOpenApiError(() =>
      recordTrainingSetAction(
        session.sessionId,
        {
          planExerciseId,
          scope: "set",
          setNumber,
          kind,
          expectedSessionRevision: session.revision,
          expectedSafetyRevision,
          idempotencyKey: newIdempotencyKey(),
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  finishSession: async (
    session: TrainingSessionRecord,
    endedReason: "finished" | "stopped_early",
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      finishTrainingSession(
        session.sessionId,
        {
          endedReason,
          expectedSessionRevision: session.revision,
          idempotencyKey: newIdempotencyKey(),
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  recordFeedback: async (
    sessionId: string,
    text: string,
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      recordTrainingSessionFeedback(
        sessionId,
        { text, idempotencyKey: newIdempotencyKey() },
        undefined,
        openApiAuthFetch,
      ),
    ),
};
