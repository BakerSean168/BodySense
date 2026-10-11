import type { TrainingPlan } from "@/features/training/services/trainingService";

/**
 * The execution projection keeps the intervention identity on the wire, but the
 * app-level `TrainingExercise` view model drops it. Accept the structural shape
 * so a projected exercise can still be traced back to a real intervention.
 */
export interface ProjectedTrainingExercise {
  intervention_id?: string;
  name: string;
  description: string;
  sets: string;
  reps: string;
  notes?: string;
}
import type {
  HealthWorkspace,
  Intervention,
  TreatmentRevision,
} from "@/features/workspace/types/workspace";

/**
 * TrainingPlan window projection.
 *
 * Product contract: one Plan is a complete prescription — every exercise has a
 * name plus sets/repetitions or duration, and the plan itself has a frequency.
 * Plan identity must be a real `planId`; a static prototype plan must never be
 * presented as the user's plan.
 *
 * Contract gaps recorded here (the UI must show them, not invent content):
 * - The Treatment/Intervention API stores `prescription` as free-form JSON, so
 *   `sets`/`reps`/`duration`/`frequency` may be absent. Missing dosage is
 *   rendered as "—", never guessed.
 * - The TrainingPlan execution projection collapses every intervention into one
 *   `phase`, and the public `TrainingExercise` drops the intervention `kind`.
 *   There is therefore no server-provided warm-up / main / cool-down structure.
 *   Stages are grouped by the real intervention kind from the accepted revision
 *   (`mobility` / `exercise` / `self_test`) instead of inventing a phase, and
 *   `stageStructureUnavailable` marks the gap for the UI.
 */

export type TrainingStagePhase = "mobility" | "main" | "self_check";

export interface TrainingStageMeta {
  phase: TrainingStagePhase;
  label: string;
}

/**
 * Ordered presentation of the real intervention kinds. This is intentionally
 * NOT a warm-up/main/cool-down prescription: mobility work is not automatically
 * a warm-up and the backend never states that it is.
 */
export const TRAINING_STAGES: readonly TrainingStageMeta[] = [
  { phase: "mobility", label: "活动度与放松练习" },
  { phase: "main", label: "训练动作" },
  { phase: "self_check", label: "自测与观察" },
] as const;

const KIND_TO_STAGE: Record<string, TrainingStagePhase> = {
  mobility: "mobility",
  exercise: "main",
  self_test: "self_check",
};

export interface PlanExercise {
  /** Real intervention identity; a user-requested change targets exactly this. */
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
  /** True when this plan is projected from the currently accepted revision. */
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
  /** No server-provided stage structure; stages reflect intervention kinds. */
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

function prescriptionValue(
  prescription: Record<string, unknown> | undefined,
  keys: readonly string[],
): string | null {
  if (!prescription) return null;
  for (const key of keys) {
    const value = text(prescription[key]);
    if (value) return value;
  }
  return null;
}

const SETS_KEYS = ["sets", "组数"] as const;
const REPS_KEYS = ["reps", "repetitions", "repetition", "次数", "times"] as const;
const DURATION_KEYS = ["duration", "duration_seconds", "时长", "seconds"] as const;
const REST_KEYS = ["rest", "rest_seconds", "休息", "interval"] as const;
const FREQUENCY_KEYS = ["frequency", "freq", "training_frequency", "频率"] as const;

function durationText(raw: string | null): string | null {
  if (!raw) return null;
  // A bare number in a duration field is seconds, matching the backend's
  // treatment of prescription.duration when building the execution projection.
  if (/^\d+(\.\d+)?$/.test(raw)) return `${raw} 秒`;
  return raw;
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

function exerciseFromIntervention(intervention: Intervention): PlanExercise {
  const prescription =
    intervention.prescription && typeof intervention.prescription === "object"
      ? (intervention.prescription as Record<string, unknown>)
      : undefined;
  return {
    exerciseId: intervention.id,
    name: intervention.title,
    description: text(intervention.description),
    sets: prescriptionValue(prescription, SETS_KEYS),
    reps: prescriptionValue(prescription, REPS_KEYS),
    duration: durationText(prescriptionValue(prescription, DURATION_KEYS)),
    rest: durationText(prescriptionValue(prescription, REST_KEYS)),
    notes: prescriptionValue(prescription, ["notes", "备注"]),
    kind: text(intervention.kind),
  };
}

/** Execution-projection fallback: no kind, but a real ordered exercise list. */
function exerciseFromProjection(exercise: ProjectedTrainingExercise): PlanExercise {
  return {
    exerciseId: exercise.intervention_id ?? exercise.name,
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

const TRAINING_KINDS = new Set(["exercise", "mobility", "self_test"]);

export function trainingInterventions(
  revision: TreatmentRevision | undefined | null,
): Intervention[] {
  if (!revision) return [];
  return [...(revision.interventions ?? [])]
    .filter((intervention) => TRAINING_KINDS.has(intervention.kind))
    .sort((left, right) => left.position - right.position);
}

function stageExercises(exercises: readonly PlanExercise[]): TrainingStage[] {
  return TRAINING_STAGES.flatMap((stage) => {
    const items = exercises.filter(
      (exercise) => (exercise.kind ? KIND_TO_STAGE[exercise.kind] : null) === stage.phase,
    );
    return items.length ? [{ ...stage, exercises: items }] : [];
  });
}

function planFrequency(
  revision: TreatmentRevision | undefined | null,
  interventions: readonly Intervention[],
): string | null {
  for (const intervention of interventions) {
    const prescription = intervention.prescription as
      | Record<string, unknown>
      | undefined;
    const frequency = prescriptionValue(prescription, FREQUENCY_KEYS);
    if (frequency) return frequency;
  }
  const habit = revision?.plan?.daily_habits?.[0];
  return text(habit);
}

function titleFor(plan: TrainingPlan, revision: TreatmentRevision | undefined): string {
  return text(revision?.goal) ?? text(plan.goal) ?? "训练计划";
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
  const revisions = new Map<string, TreatmentRevision>();
  for (const revision of workspace?.treatment_revisions ?? []) {
    revisions.set(revision.id, revision);
  }
  const acceptedRevisionId = workspace?.treatment?.current?.id ?? null;
  const acceptedPlanId = workspace?.training_plan?.id ?? null;

  const summaries: TrainingPlanSummary[] = plans.map((plan) => {
    const revision = plan.treatment_revision_id
      ? revisions.get(plan.treatment_revision_id)
      : undefined;
    return {
      planId: plan.id,
      title: titleFor(plan, revision),
      status: plan.status,
      isAccepted: plan.id === acceptedPlanId || revision?.id === acceptedRevisionId,
    };
  });

  const effectiveId = resolveSelectedPlanId(plans, selectedPlanId);
  if (!effectiveId) {
    return { plans: summaries, selected: null, selectedPlanMissing: false };
  }
  const plan = plans.find((item) => item.id === effectiveId);
  if (!plan) {
    return { plans: summaries, selected: null, selectedPlanMissing: true };
  }

  const revision = plan.treatment_revision_id
    ? revisions.get(plan.treatment_revision_id)
    : undefined;
  const interventions = trainingInterventions(revision);
  const fromInterventions = interventions.map(exerciseFromIntervention);
  const fromProjection = (plan.phases ?? []).flatMap((phase) =>
    ((phase.exercises ?? []) as ProjectedTrainingExercise[]).map(
      exerciseFromProjection,
    ),
  );
  const exercises = fromInterventions.length ? fromInterventions : fromProjection;
  const stages = stageExercises(exercises);

  return {
    plans: summaries,
    selectedPlanMissing: false,
    selected: {
      planId: plan.id,
      title: titleFor(plan, revision),
      goal: text(revision?.goal) ?? text(plan.goal) ?? "",
      status: plan.status,
      durationWeeks: text(revision?.duration_weeks) ? revision!.duration_weeks : (plan.duration_weeks ?? null),
      frequency: planFrequency(revision, interventions),
      stages:
        stages.length > 0
          ? stages
          : exercises.length > 0
            ? [{ phase: "main", label: "训练动作", exercises }]
            : [],
      // The public contract has no phase field, so kind-based grouping is a
      // deliberate, visible compromise rather than a fabricated prescription.
      stageStructureUnavailable: true,
      isAccepted: plan.id === acceptedPlanId || revision?.id === acceptedRevisionId,
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
