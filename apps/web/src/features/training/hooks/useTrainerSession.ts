import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectTrainerSession } from "@/features/body-canvas/model/trainerSession";
import { projectGate } from "@/features/body-canvas/model/trainingPlans";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import {
  newIdempotencyKey,
  trainingApi,
  type TrainingSessionRecord,
} from "../services/trainingService";
import { trainingPlanKeys } from "./useTrainingPlanActions";

export const trainerKeys = {
  all: ["training", "trainer"] as const,
  plan: (planId: string) => [...trainerKeys.all, "plan", planId] as const,
  sessions: () => [...trainerKeys.all, "sessions"] as const,
  revision: (planId: string, revisionId: string | null) =>
    [...trainerKeys.all, "revision", planId, revisionId] as const,
};

export type TrainerNotice = "safety_changed" | "blocked" | "changed" | null;

function commandNotice(error: unknown): { notice: TrainerNotice; message: string | null } {
  if (error instanceof ApiRequestError && error.status === 409) {
    switch (error.code) {
      case "SAFETY_STALE":
        return { notice: "safety_changed", message: null };
      case "TRAINER_GATE_BLOCKED":
        return { notice: "blocked", message: null };
      case "SESSION_IN_PROGRESS":
        return { notice: null, message: "另一个训练正在进行，先结束它再开始这一个。" };
      case "PLAN_NOT_ACTIVE":
        return { notice: null, message: "这份计划已不是启用状态，不能开始训练。" };
      default:
        return { notice: "changed", message: null };
    }
  }
  return { notice: null, message: errorMessage(error, "这一步没有被记录，请再试一次。") };
}

export interface UseTrainerSessionOptions {
  planId: string;
  templateId: string | null;
  /**
   * Identity of the explicit Start click that opened this Trainer. One
   * launch starts at most one session; a remount replays the same key.
   */
  launchKey: string;
}

/**
 * Trainer window on the Go runtime. Starts the launched template once (or
 * picks up this plan's live session), then records explicit per-set
 * receipts. Every command carries the session revision and the Safety
 * revision of the gate the user is looking at; a refusal writes nothing,
 * so the window refetches and shows why instead of a stale success.
 */
export function useTrainerSession({ planId, templateId, launchKey }: UseTrainerSessionOptions) {
  const client = useQueryClient();
  const plan = useQuery({
    queryKey: trainerKeys.plan(planId),
    queryFn: () => trainingApi.getPlanRecord(planId),
    retry: false,
    staleTime: 0,
  });
  const sessions = useQuery({
    queryKey: trainerKeys.sessions(),
    queryFn: trainingApi.listSessions,
    retry: false,
    staleTime: 0,
  });
  /** The session this window shows; kept after it ends (lists drop nothing, but order may change). */
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<TrainerNotice>(null);
  const [error, setError] = useState<string | null>(null);
  const keys = useRef(new Map<string, string>());
  const inFlight = useRef(false);
  const started = useRef(false);

  const live = useMemo(
    () => sessions.data?.find((item) => item.planId === planId && item.state !== "ended") ?? null,
    [sessions.data, planId],
  );
  const session: TrainingSessionRecord | null =
    (sessionId ? sessions.data?.find((item) => item.sessionId === sessionId) : null) ?? live;

  const pinnedRevision = session?.planRevisionId ?? null;
  const pinnedIsCurrent = plan.data && pinnedRevision === plan.data.currentAcceptedRevisionId;
  const revision = useQuery({
    queryKey: trainerKeys.revision(planId, pinnedRevision),
    queryFn: () => trainingApi.getRevisionContent(planId, pinnedRevision!),
    enabled: !!plan.data && pinnedRevision !== null && !pinnedIsCurrent,
    retry: false,
    staleTime: Infinity,
  });
  const pinned = pinnedIsCurrent ? plan.data!.currentContent : (revision.data ?? null);

  const refresh = useCallback(async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: trainerKeys.all }),
      client.invalidateQueries({ queryKey: trainingPlanKeys.all }),
    ]);
  }, [client]);

  const run = useCallback(
    async (key: string, command: (idempotencyKey: string) => Promise<TrainingSessionRecord | void>) => {
      if (inFlight.current) return;
      let idempotencyKey = keys.current.get(key);
      if (!idempotencyKey) {
        idempotencyKey = newIdempotencyKey();
        keys.current.set(key, idempotencyKey);
      }
      inFlight.current = true;
      setBusy(true);
      setError(null);
      try {
        const result = await command(idempotencyKey);
        if (result) setSessionId(result.sessionId);
        setNotice(null);
        keys.current.delete(key);
        await refresh();
      } catch (caught) {
        const { notice: next, message } = commandNotice(caught);
        setNotice(next);
        setError(message);
        // A refusal is final for that request: the next attempt is a new
        // decision at the then-current revisions.
        if (caught instanceof ApiRequestError && caught.status === 409) keys.current.delete(key);
        await refresh();
      } finally {
        inFlight.current = false;
        setBusy(false);
      }
    },
    [refresh],
  );

  // One explicit Start → at most one session for this launch.
  useEffect(() => {
    if (started.current || !plan.data || !sessions.data || !templateId) return;
    started.current = true;
    if (live) {
      setSessionId(live.sessionId);
      return;
    }
    const record = plan.data;
    void run(`start:${launchKey}`, (key) => trainingApi.startSession(record, templateId, key));
  }, [plan.data, sessions.data, live, templateId, launchKey, run]);

  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    if (!session || session.state === "ended") return;
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, [session]);

  const view = useMemo(
    () => (plan.data && session ? projectTrainerSession(plan.data, pinned, session, now) : null),
    [plan.data, pinned, session, now],
  );

  const recordSet = (kind: "performed" | "skipped", actual: number | null) => {
    if (!view?.current || !session || !plan.data || !view.canRecord) return;
    const exercise = view.exercises[view.current.exerciseIndex]!;
    const { setNumber, actualUnit } = view.current;
    const safetyRevision = plan.data.executionGate.safetyRevision;
    void run(`set:${session.sessionId}:${exercise.planExerciseId}:${setNumber}:${kind}:${actual}`, async (key) => {
      await trainingApi.recordSet({
        session,
        planExerciseId: exercise.planExerciseId,
        setNumber,
        kind,
        ...(kind === "performed" && actual !== null && actualUnit === "reps" ? { actualReps: actual } : {}),
        ...(kind === "performed" && actual !== null && actualUnit === "seconds"
          ? { actualDurationSeconds: actual }
          : {}),
        expectedSafetyRevision: safetyRevision,
        idempotencyKey: key,
      });
    });
  };

  const pause = () => {
    if (!session || session.state !== "in_progress") return;
    void run(`pause:${session.sessionId}:${session.revision}`, (key) => trainingApi.pauseSession(session, key));
  };
  const resume = () => {
    if (!session || session.state !== "paused" || !plan.data) return;
    const safetyRevision = plan.data.executionGate.safetyRevision;
    void run(`resume:${session.sessionId}:${session.revision}`, (key) =>
      trainingApi.resumeSession(session, safetyRevision, key),
    );
  };

  return {
    view,
    gate: plan.data ? projectGate(plan.data.executionGate) : null,
    pause,
    resume,
    loading: plan.isPending || sessions.isPending || (!!session && !pinned && revision.isPending),
    loadError: plan.isError || sessions.isError || revision.isError,
    busy,
    notice,
    error,
    complete: (actual: number | null) => recordSet("performed", actual),
    skip: () => recordSet("skipped", null),
    finish: () => {
      if (!session || session.state === "ended") return;
      const reason = view?.allRecorded ? "finished" : "stopped_early";
      void run(`finish:${session.sessionId}:${reason}`, (key) => trainingApi.finishSession(session, reason, key));
    },
  };
}

export type TrainerSession = ReturnType<typeof useTrainerSession>;
