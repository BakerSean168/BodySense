import {
  acceptPlanProposal,
  finishTrainingSession,
  getTrainingPlanRecord,
  getTrainingPlanRevisionRecord,
  pauseTrainingSession,
  resumeTrainingSession,
  listPlanProposalRecords,
  listTrainingPlanRecords,
  listTrainingSessions,
  recordTrainingSessionFeedback,
  recordTrainingSetAction,
  rejectPlanProposal,
  startTrainingSession,
} from "@/generated/api/bodysense";
import type {
  PlanAcceptanceReceiptOutput,
  PlanExerciseOutput,
  PlanProposalRecordOutput,
  PlanRejectionReceiptOutput,
  TrainingPlanRecordOutput,
  TrainingSessionRecordOutput,
  TrainingSetActionRecordOutput,
} from "@/generated/api/model";
import { openApiAuthFetch, withOpenApiError } from "@/lib/openapi-client";

export type TrainingPlanRecord = TrainingPlanRecordOutput;
export type TrainingSessionRecord = TrainingSessionRecordOutput;
export type TrainingSetActionRecord = TrainingSetActionRecordOutput;
export type PlanProposalRecord = PlanProposalRecordOutput;
export type PlanAcceptanceReceipt = PlanAcceptanceReceiptOutput;
export type PlanRejectionReceipt = PlanRejectionReceiptOutput;

/**
 * One explicit user decision on the exact presented proposal content. The
 * caller keeps the same identity for retries of the same decision, so a
 * repeated click or a network retry replays the original receipt.
 */
export interface PlanDecisionIdentity {
  decisionId: string;
  idempotencyKey: string;
}

function decisionRequest(
  proposal: PlanProposalRecord,
  identity: PlanDecisionIdentity,
) {
  return {
    decisionId: identity.decisionId,
    presentedFingerprint: proposal.contentFingerprint,
    // An update is pinned to the plan revision it was proposed against;
    // create proposals carry neither field.
    ...(proposal.kind === "update" && proposal.planId && proposal.basedOnRevisionId
      ? {
          targetPlanId: proposal.planId,
          expectedRevisionId: proposal.basedOnRevisionId,
        }
      : {}),
    idempotencyKey: identity.idempotencyKey,
  };
}

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

export function newIdempotencyKey(): string {
  return globalThis.crypto.randomUUID();
}

/** The user's IANA zone; the server fixes the training date from it. */
export function userTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function newPlanDecisionIdentity(): PlanDecisionIdentity {
  return {
    decisionId: globalThis.crypto.randomUUID(),
    idempotencyKey: newIdempotencyKey(),
  };
}

export const trainingApi = {
  listPlanRecords: async (): Promise<TrainingPlanRecord[]> =>
    withOpenApiError(async () => {
      const response = await listTrainingPlanRecords(
        undefined,
        openApiAuthFetch,
      );
      return response.plans;
    }),

  /** Every proposal of the user, newest first (pending, decided and expired). */
  listProposals: async (): Promise<PlanProposalRecord[]> =>
    withOpenApiError(async () => {
      const response = await listPlanProposalRecords(
        undefined,
        undefined,
        openApiAuthFetch,
      );
      return response.proposals;
    }),

  acceptProposal: async (
    proposal: PlanProposalRecord,
    identity: PlanDecisionIdentity,
  ): Promise<PlanAcceptanceReceipt> =>
    withOpenApiError(() =>
      acceptPlanProposal(
        proposal.proposalId,
        decisionRequest(proposal, identity),
        undefined,
        openApiAuthFetch,
      ),
    ),

  rejectProposal: async (
    proposal: PlanProposalRecord,
    identity: PlanDecisionIdentity,
  ): Promise<PlanRejectionReceipt> =>
    withOpenApiError(() =>
      rejectPlanProposal(
        proposal.proposalId,
        decisionRequest(proposal, identity),
        undefined,
        openApiAuthFetch,
      ),
    ),

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

  /** The content of one accepted revision (a session pins its revision). */
  getRevisionContent: async (
    planId: string,
    revisionId: string,
  ): Promise<TrainingPlanRecord["currentContent"]> =>
    withOpenApiError(async () => {
      const revision = await getTrainingPlanRevisionRecord(
        planId,
        revisionId,
        undefined,
        openApiAuthFetch,
      );
      return revision.snapshot.content;
    }),

  startSession: async (
    plan: TrainingPlanRecord,
    templateId: string,
    idempotencyKey: string = newIdempotencyKey(),
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      startTrainingSession(
        {
          planId: plan.planId,
          expectedPlanRevisionId: plan.currentAcceptedRevisionId,
          templateId,
          timeZone: userTimeZone(),
          expectedSafetyRevision: plan.executionGate.safetyRevision,
          idempotencyKey,
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  /**
   * One explicit set action. The caller keeps `idempotencyKey` for retries
   * of the same action so a repeated tap replays the original receipt.
   */
  recordSet: async (command: {
    session: TrainingSessionRecord;
    planExerciseId: string;
    setNumber: number;
    kind: "performed" | "skipped";
    actualReps?: number;
    actualDurationSeconds?: number;
    expectedSafetyRevision: number;
    idempotencyKey: string;
  }): Promise<TrainingSetActionRecord> =>
    withOpenApiError(() =>
      recordTrainingSetAction(
        command.session.sessionId,
        {
          planExerciseId: command.planExerciseId,
          scope: "set",
          setNumber: command.setNumber,
          kind: command.kind,
          ...(command.kind === "performed" && command.actualReps !== undefined
            ? { actualReps: command.actualReps }
            : {}),
          ...(command.kind === "performed" &&
          command.actualDurationSeconds !== undefined
            ? { actualDurationSeconds: command.actualDurationSeconds }
            : {}),
          expectedSessionRevision: command.session.revision,
          expectedSafetyRevision: command.expectedSafetyRevision,
          idempotencyKey: command.idempotencyKey,
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  finishSession: async (
    session: TrainingSessionRecord,
    endedReason: "finished" | "stopped_early",
    idempotencyKey: string = newIdempotencyKey(),
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      finishTrainingSession(
        session.sessionId,
        {
          endedReason,
          expectedSessionRevision: session.revision,
          idempotencyKey,
        },
        undefined,
        openApiAuthFetch,
      ),
    ),

  /** Pausing is never gated; only an in-progress session can pause. */
  pauseSession: async (
    session: TrainingSessionRecord,
    idempotencyKey: string = newIdempotencyKey(),
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      pauseTrainingSession(
        session.sessionId,
        { expectedSessionRevision: session.revision, idempotencyKey },
        undefined,
        openApiAuthFetch,
      ),
    ),

  /** Resuming re-gates at exactly the Safety revision the user was shown. */
  resumeSession: async (
    session: TrainingSessionRecord,
    expectedSafetyRevision: number,
    idempotencyKey: string = newIdempotencyKey(),
  ): Promise<TrainingSessionRecord> =>
    withOpenApiError(() =>
      resumeTrainingSession(
        session.sessionId,
        {
          expectedSessionRevision: session.revision,
          expectedSafetyRevision,
          idempotencyKey,
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
