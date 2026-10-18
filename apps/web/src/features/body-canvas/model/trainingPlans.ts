import type { TrainingPlan } from "@/features/training/services/trainingService";

/** One exercise of the legacy /training execution projection. */
export interface ProjectedTrainingExercise {
  name: string;
  description: string;
  sets: string;
  reps: string;
  notes?: string;
}
import type { HealthWorkspace } from "@/features/workspace/types/workspace";

/**
 * TrainingPlan window projection.
 *
 * Product contract: one Plan is a complete prescription — every exercise has a
 * name plus sets/repetitions or duration, and the plan itself has a frequency.
 * Plan identity must be a real `planId`; a static prototype plan must never be
 * presented as the user's plan.
 *
 * Contract gaps recorded here (the UI must show them, not invent content):
 * - The legacy `/training` execution projection stores sets/reps as free text
 *   and may omit them. Missing dosage is rendered as "—", never guessed.
 * - That projection collapses every exercise into one `phase` without a kind,
 *   so there is no server-provided warm-up / main / cool-down structure. All
 *   exercises are shown in one main stage and `stageStructureUnavailable`
 *   marks the gap for the UI. Binding the independent TrainingPlan revision
 *   content (sessions/stages) is UI-TP-001.
 */

export type TrainingStagePhase = "warmup" | "main" | "self_check";

export interface TrainingStageMeta {
  phase: TrainingStagePhase;
  label: string;
}

/**
 * Stage labels follow the design's 热身 / 正式训练 / 放松 ordering. The execution
 * projection has no warm-up/cool-down phase field, so exercises are shown in
 * the main stage and `stageStructureUnavailable` marks the gap.
 */
export const TRAINING_STAGES: readonly TrainingStageMeta[] = [
  { phase: "warmup", label: "热身与活动度" },
  { phase: "main", label: "正式训练" },
  { phase: "self_check", label: "自测与观察" },
] as const;

export interface PlanExercise {
  /** Exercise identity from the execution projection (falls back to its name). */
  exerciseId: string;
  name: string;
  description: string | null;
  sets: string | null;
  reps: string | null;
  duration: string | null;
  rest: string | null;
  notes: string | null;
  kind: string | null;
}

export interface TrainingStage {
  phase: TrainingStagePhase;
  label: string;
  exercises: PlanExercise[];
}

export interface TrainingPlanSummary {
  planId: string;
  title: string;
  status: string;
  /** True when this is the workspace's active execution plan. */
  isAccepted: boolean;
}

export interface TrainingPlanView {
  planId: string;
  title: string;
  goal: string;
  status: string;
  durationWeeks: number | null;
  frequency: string | null;
  stages: TrainingStage[];
  /**
   * True when the backend supplied no phase structure. The window states this
   * instead of implying the backend prescribed a warm-up/cool-down split.
   */
  stageStructureUnavailable: boolean;
  isAccepted: boolean;
  exerciseCount: number;
}

function text(value: unknown): string | null {
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed ? trimmed : null;
  }
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  if (Array.isArray(value)) {
    const parts = value.flatMap((item) => {
      const part = text(item);
      return part ? [part] : [];
    });
    return parts.length ? parts.join("、") : null;
  }
  return null;
}

/** "3 组 × 10 次"; missing dosage stays an em dash instead of a guess. */
export function formatDosage(exercise: PlanExercise): string {
  const parts: string[] = [];
  if (exercise.sets) parts.push(`${exercise.sets} 组`);
  if (exercise.reps) parts.push(`${exercise.reps} 次`);
  else if (exercise.duration) parts.push(exercise.duration);
  if (exercise.rest) parts.push(`休息 ${exercise.rest}`);
  return parts.length ? parts.join(" · ") : "剂量未提供";
}

/** Execution projection: no kind, but a real ordered exercise list. */
function exerciseFromProjection(exercise: ProjectedTrainingExercise): PlanExercise {
  return {
    exerciseId: exercise.name,
    name: exercise.name,
    description: text(exercise.description),
    sets: text(exercise.sets),
    reps: text(exercise.reps),
    duration: null,
    rest: null,
    notes: text(exercise.notes),
    kind: null,
  };
}

function titleFor(plan: TrainingPlan): string {
  return text(plan.goal) ?? "训练计划";
}

export interface ProjectedTrainingPlans {
  plans: TrainingPlanSummary[];
  selected: TrainingPlanView | null;
  /** The selected plan's identity could not be resolved to a real plan. */
  selectedPlanMissing: boolean;
}

/**
 * Resolve which plan is selected. A persisted id is honoured only when it still
 * exists for this user; otherwise the first real plan wins. Returning null means
 * "no plan yet", which the UI must show as an empty state.
 */
export function resolveSelectedPlanId(
  plans: readonly TrainingPlan[],
  preferredPlanId: string | null | undefined,
): string | null {
  if (plans.length === 0) return null;
  if (preferredPlanId && plans.some((plan) => plan.id === preferredPlanId)) {
    return preferredPlanId;
  }
  const active = plans.find((plan) => plan.status === "active");
  return (active ?? plans[0]).id;
}

export function projectTrainingPlans(
  workspace: HealthWorkspace | undefined,
  plans: readonly TrainingPlan[],
  selectedPlanId: string | null,
): ProjectedTrainingPlans {
  const acceptedPlanId = workspace?.training_plan?.id ?? null;

  const summaries: TrainingPlanSummary[] = plans.map((plan) => ({
    planId: plan.id,
    title: titleFor(plan),
    status: plan.status,
    isAccepted: plan.id === acceptedPlanId,
  }));

  const effectiveId = resolveSelectedPlanId(plans, selectedPlanId);
  if (!effectiveId) {
    return { plans: summaries, selected: null, selectedPlanMissing: false };
  }
  const plan = plans.find((item) => item.id === effectiveId);
  if (!plan) {
    return { plans: summaries, selected: null, selectedPlanMissing: true };
  }

  const exercises = (plan.phases ?? []).flatMap((phase) =>
    ((phase.exercises ?? []) as ProjectedTrainingExercise[]).map(
      exerciseFromProjection,
    ),
  );

  return {
    plans: summaries,
    selectedPlanMissing: false,
    selected: {
      planId: plan.id,
      title: titleFor(plan),
      goal: text(plan.goal) ?? "",
      status: plan.status,
      durationWeeks: plan.duration_weeks ?? null,
      frequency: null,
      stages:
        exercises.length > 0
          ? [{ phase: "main" as const, label: "正式训练", exercises }]
          : [],
      // The execution projection has no phase field, so one main stage is a
      // deliberate, visible compromise rather than a fabricated prescription.
      stageStructureUnavailable: true,
      isAccepted: plan.id === acceptedPlanId,
      exerciseCount: exercises.length,
    },
  };
}

/**
 * Stateless, versioned persistence of the last explicitly opened plan so the
 * Dock reopen restores the user's plan instead of a prototype default.
 */
export const SELECTED_PLAN_STORAGE_KEY = "bodysense.training.selectedPlanId";

export function readSelectedPlanId(storage?: Pick<Storage, "getItem">): string | null {
  const target = storage ?? (typeof window === "undefined" ? undefined : window.localStorage);
  if (!target) return null;
  try {
    const value = target.getItem(SELECTED_PLAN_STORAGE_KEY);
    return value && value.trim() ? value : null;
  } catch {
    return null;
  }
}

export function writeSelectedPlanId(
  planId: string | null,
  storage?: Pick<Storage, "setItem"> & Partial<Pick<Storage, "removeItem">>,
): void {
  const target = storage ?? (typeof window === "undefined" ? undefined : window.localStorage);
  if (!target) return;
  try {
    if (planId) target.setItem(SELECTED_PLAN_STORAGE_KEY, planId);
    else target.removeItem?.(SELECTED_PLAN_STORAGE_KEY);
  } catch {
    // A blocked storage must never break the plan window.
  }
}
