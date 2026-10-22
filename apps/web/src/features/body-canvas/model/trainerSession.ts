import type {
  TrainingPlanRecord,
  TrainingSessionRecord,
} from "@/features/training/services/trainingService";
import {
  STAGE_LABELS,
  formatPrescription,
  projectGate,
  type TrainingGateView,
} from "./trainingPlans";

/**
 * Trainer window view model on `/training-sessions` (UI-TP-001), matched to
 * the Penpot Trainer boards In progress / Set recorded / Recovery / Ended.
 * Everything shown is derived from the session's immutable receipts: a set
 * is done only when an effective `performed`/`skipped` receipt exists, a
 * whole-exercise skip covers only the sets still open, and a correction
 * retracts one receipt so its target can be recorded again.
 */

type Exercise =
  TrainingPlanRecord["currentContent"]["sessions"][number]["stages"][number]["exercises"][number];
type Action = TrainingSessionRecord["actions"][number];

export type SetState = "performed" | "skipped" | "pending";

export interface TrainerExerciseView {
  planExerciseId: string;
  name: string;
  stageLabel: string;
  /** 1-based position within its stage and the stage's size: "主训练 · 2 / 5". */
  position: number;
  stageSize: number;
  prescription: string;
  target: Exercise["target"];
  restSeconds: number | null;
  sets: SetState[];
  /** The effective receipt behind each set, for corrections (null when pending). */
  setActionIds: (string | null)[];
  /** The effective whole-exercise skip receipt, if any. */
  exerciseSkipActionId: string | null;
  /** "done" when no set is pending; "partial" when some are recorded. */
  progress: "done" | "partial" | "pending";
  performedCount: number;
}

export interface TrainerCurrentSet {
  exerciseIndex: number;
  setNumber: number;
  /**
   * Prefilled actual value the stepper starts from: the retracted value when
   * the set is being recorded again, otherwise the prescription.
   */
  defaultActual: number | null;
  /** The set's earlier receipt was retracted; this is its re-record. */
  correcting: boolean;
  actualUnit: "reps" | "seconds" | null;
}

export interface TrainerLastRecord {
  actionId: string;
  planExerciseId: string;
  name: string;
  scope: "set" | "exercise";
  setNumber: number | null;
  /** `corrected`: the latest action retracted an earlier receipt. */
  kind: "performed" | "skipped" | "corrected";
  actualReps: number | null;
  actualDurationSeconds: number | null;
  recordedAt: string;
  restSeconds: number | null;
}

export interface TrainerSessionView {
  sessionId: string;
  state: TrainingSessionRecord["state"];
  endedReason: TrainingSessionRecord["endedReason"] | null;
  templateTitle: string;
  gate: TrainingGateView;
  /** Set actions need an in-progress session and a permitting gate. */
  canRecord: boolean;
  exercises: TrainerExerciseView[];
  current: TrainerCurrentSet | null;
  next: TrainerExerciseView | null;
  last: TrainerLastRecord | null;
  allRecorded: boolean;
  /** The user's verbatim feedback on the ended session, once recorded. */
  feedback: { text: string; recordedAt: string } | null;
  /** Seconds of training so far, paused time excluded. */
  elapsedSeconds: number;
  /** Remaining rest after the last performed set, or 0. */
  restRemainingSeconds: number;
}

/** Effective receipts: corrections and the receipts they retract drop out. */
export function effectiveActions(actions: readonly Action[]): Action[] {
  const retracted = new Set(
    actions.flatMap((action) =>
      action.correctsActionId ? [action.correctsActionId] : [],
    ),
  );
  return actions.filter(
    (action) => action.kind !== "corrected" && !retracted.has(action.actionId),
  );
}

function secondsBetween(from: string, to: number): number {
  const start = Date.parse(from);
  return Number.isFinite(start)
    ? Math.max(0, Math.floor((to - start) / 1000))
    : 0;
}

/** Paused time is derivable from the pause/resume receipts. */
export function elapsedTrainingSeconds(
  session: TrainingSessionRecord,
  now: Date,
): number {
  const end = session.endedAt ? Date.parse(session.endedAt) : now.getTime();
  let paused = 0;
  let pausedAt: number | null = null;
  for (const transition of session.transitions) {
    const at = Date.parse(transition.recordedAt);
    if (!Number.isFinite(at)) continue;
    if (transition.kind === "pause") pausedAt = at;
    else if (transition.kind === "resume" && pausedAt !== null) {
      paused += at - pausedAt;
      pausedAt = null;
    }
  }
  if (pausedAt !== null) paused += end - pausedAt;
  const started = Date.parse(session.startedAt);
  return Number.isFinite(started)
    ? Math.max(0, Math.floor((end - started - paused) / 1000))
    : 0;
}

export function formatClock(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

export function formatRestClock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

export type PlanContent = TrainingPlanRecord["currentContent"];

/**
 * `pinned` is the content of the revision the session pinned at start
 * (the plan's current content when that revision is still current); the
 * gate is always the plan record's current TrainerGate projection.
 */
export function projectTrainerSession(
  plan: TrainingPlanRecord,
  pinned: PlanContent | null,
  session: TrainingSessionRecord,
  now: Date,
): TrainerSessionView {
  const template = pinned?.sessions.find(
    (item) => item.templateId === session.templateId,
  );
  const effective = effectiveActions(session.actions);
  const exercises: TrainerExerciseView[] = [];
  const byId = new Map<string, Exercise>();
  for (const stage of template?.stages ?? []) {
    stage.exercises.forEach((exercise, index) => {
      byId.set(exercise.planExerciseId, exercise);
      const wholeSkip = effective.find(
        (action) =>
          action.planExerciseId === exercise.planExerciseId &&
          action.scope === "exercise",
      );
      const receipts = Array.from({ length: exercise.sets }, (_, setIndex) =>
        effective.find(
          (action) =>
            action.planExerciseId === exercise.planExerciseId &&
            action.scope === "set" &&
            action.setNumber === setIndex + 1,
        ),
      );
      // A whole-exercise skip covers the sets still open; recorded sets keep
      // their own receipts.
      const sets: SetState[] = receipts.map((receipt) =>
        receipt
          ? (receipt.kind as "performed" | "skipped")
          : wholeSkip
            ? "skipped"
            : "pending",
      );
      const pending = sets.filter((state) => state === "pending").length;
      exercises.push({
        planExerciseId: exercise.planExerciseId,
        name: exercise.nameSnapshot,
        stageLabel: STAGE_LABELS[stage.phase],
        position: index + 1,
        stageSize: stage.exercises.length,
        prescription: formatPrescription(exercise),
        target: exercise.target,
        restSeconds: exercise.restSeconds ?? null,
        sets,
        setActionIds: receipts.map((receipt) => receipt?.actionId ?? null),
        exerciseSkipActionId: wholeSkip?.actionId ?? null,
        progress:
          pending === 0
            ? "done"
            : pending === sets.length
              ? "pending"
              : "partial",
        performedCount: sets.filter((state) => state === "performed").length,
      });
    });
  }

  const currentIndex = exercises.findIndex((exercise) =>
    exercise.sets.includes("pending"),
  );
  let current: TrainerCurrentSet | null = null;
  if (currentIndex >= 0) {
    const exercise = exercises[currentIndex];
    const { target } = exercise;
    const setNumber = exercise.sets.indexOf("pending") + 1;
    const actualUnit =
      target.kind === "repetitions"
        ? "reps"
        : target.kind === "duration"
          ? "seconds"
          : null;
    // The newest retracted receipt of this very set, if the user corrected it.
    const retracted = new Set(
      session.actions.flatMap((action) =>
        action.correctsActionId ? [action.correctsActionId] : [],
      ),
    );
    const previous = [...session.actions]
      .filter(
        (action) =>
          retracted.has(action.actionId) &&
          action.planExerciseId === exercise.planExerciseId &&
          action.scope === "set" &&
          action.setNumber === setNumber,
      )
      .toSorted((a, b) => b.sessionRevision - a.sessionRevision)[0];
    const previousActual =
      previous?.kind === "performed"
        ? actualUnit === "reps"
          ? previous.actualReps
          : actualUnit === "seconds"
            ? previous.actualDurationSeconds
            : undefined
        : undefined;
    current = {
      exerciseIndex: currentIndex,
      setNumber,
      defaultActual:
        previousActual ??
        (target.kind === "repetitions"
          ? (target.min ?? null)
          : target.kind === "duration"
            ? (target.seconds ?? null)
            : null),
      correcting: previous !== undefined,
      actualUnit,
    };
  }

  // The newest receipt of any kind: a correction is shown as such, and an
  // effective receipt can be corrected from its banner.
  const lastAction = session.actions.toSorted(
    (a, b) => b.sessionRevision - a.sessionRevision,
  )[0];
  const lastExercise = lastAction
    ? byId.get(lastAction.planExerciseId)
    : undefined;
  const last: TrainerLastRecord | null =
    lastAction && lastExercise
      ? {
          actionId: lastAction.actionId,
          planExerciseId: lastExercise.planExerciseId,
          name: lastExercise.nameSnapshot,
          scope: lastAction.scope,
          setNumber: lastAction.setNumber ?? null,
          kind: lastAction.kind,
          actualReps: lastAction.actualReps ?? null,
          actualDurationSeconds: lastAction.actualDurationSeconds ?? null,
          recordedAt: lastAction.recordedAt,
          restSeconds: lastExercise.restSeconds ?? null,
        }
      : null;
  const restRemainingSeconds =
    session.state === "in_progress" &&
    last?.kind === "performed" &&
    last.restSeconds &&
    current
      ? Math.max(
          0,
          last.restSeconds - secondsBetween(last.recordedAt, now.getTime()),
        )
      : 0;

  const gate = projectGate(plan.executionGate);
  return {
    sessionId: session.sessionId,
    state: session.state,
    endedReason: session.endedReason ?? null,
    templateTitle: template?.title ?? "",
    gate,
    canRecord:
      session.state === "in_progress" && gate.permits && template !== undefined,
    exercises,
    current,
    next:
      currentIndex >= 0
        ? (exercises
            .slice(currentIndex + 1)
            .find((exercise) => exercise.progress !== "done") ?? null)
        : null,
    last,
    allRecorded: exercises.length > 0 && currentIndex < 0,
    feedback: session.feedback
      ? { text: session.feedback.text, recordedAt: session.feedback.recordedAt }
      : null,
    elapsedSeconds: elapsedTrainingSeconds(session, now),
    restRemainingSeconds,
  };
}
