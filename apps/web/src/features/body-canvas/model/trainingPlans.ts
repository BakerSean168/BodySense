import { resolveSelectedPlanId, type TrainingPlanSummary } from "./trainingPlanSelection";
import type {
  PlanAcceptanceReceipt,
  PlanProposalRecord,
  TrainingPlanRecord,
} from "@/features/training/services/trainingService";

/**
 * TrainingPlan window view model on the V2 `/training-plans` and
 * `/plan-proposals` records (UI-TP-001), matched to the Penpot "03 App"
 * Training boards: Empty, Accepted, Proposal, Proposal · Expired, Updated.
 *
 * Product rules carried here:
 * - A plan record is always a user-accepted revision; its content is read-only.
 *   Only a PlanProposal the user accepts writes a new revision, and the window
 *   shows the new content only once the plan record carries the receipt's
 *   revision ("update only after receipt").
 * - Execution permission is the Go TrainerGate projection on the record
 *   (`executionGate`), never inferred from plan status alone.
 * - Nothing is invented: a missing prescription value stays missing.
 */

type PlanContent = TrainingPlanRecord["currentContent"];
type PlanTemplate = PlanContent["sessions"][number];
type PlanStageRecord = PlanTemplate["stages"][number];
type PlanExerciseRecord = PlanStageRecord["exercises"][number];
type Gate = TrainingPlanRecord["executionGate"];

export type TrainingTone = "ok" | "idle" | "warn" | "bad" | "ai" | "neutral";

export const STAGE_LABELS: Record<PlanStageRecord["phase"], string> = {
  warmup: "热身",
  main: "主训练",
  cooldown: "放松",
};

const SIDE_LABELS: Record<NonNullable<PlanExerciseRecord["side"]>, string> = {
  left: "左侧",
  right: "右侧",
  both: "两侧",
  alternating: "左右交替",
};

const STATUS_LABELS: Record<
  TrainingPlanRecord["status"],
  { label: string; tone: TrainingTone }
> = {
  active: { label: "启用中", tone: "ok" },
  paused: { label: "已暂停", tone: "idle" },
  completed: { label: "已完成", tone: "idle" },
};

/** Go TrainerGate reasons, worded for the user; unknown codes stay generic. */
const GATE_REASON_LABELS: Record<string, string> = {
  safety_state_unavailable: "安全状态暂时无法读取",
  safety_state_malformed: "安全状态记录异常",
  safety_screening_incomplete: "还没有完成安全筛查",
  urgent_restriction: "有需要尽快就医的安全提醒",
  restricted_restriction: "有限制训练的安全提醒",
  advisory_restriction: "有需要留意的安全提醒",
  custom_exercise_unverifiable: "包含无法核对安全性的自定义动作",
  review_point_reached: "已到安全复查节点",
  restriction_not_yet_effective: "有尚未生效的安全限制",
  invalid_trainer_action: "训练动作无法核对",
};

export function gateReasonLabel(reason: string): string {
  return GATE_REASON_LABELS[reason] ?? "安全检查未通过";
}

export interface TrainingGateView {
  result: Gate["result"];
  permits: boolean;
  tone: TrainingTone;
  /** Footer chip: "可以开始" / "暂不能开始". */
  label: string;
  /** User-facing reasons, de-duplicated, in server order. */
  reasons: string[];
  safetyRevision: number;
}

export function projectGate(gate: Gate): TrainingGateView {
  const reasons = [
    ...(gate.urgentCare ? ["有需要尽快就医的安全提醒"] : []),
    ...gate.reasons.map(gateReasonLabel),
    ...(gate.requiresReview ? ["需要先复查安全状态"] : []),
  ].filter((reason, index, all) => all.indexOf(reason) === index);
  const permits = gate.permitsExecution && (gate.result === "allow" || gate.result === "warn");
  return {
    result: gate.result,
    permits,
    tone: !permits ? "bad" : gate.result === "warn" ? "warn" : "ok",
    label: permits ? "可以开始" : "暂不能开始",
    reasons: permits && gate.result === "allow" ? [] : reasons.length ? reasons : permits ? [] : ["安全检查未通过"],
    safetyRevision: gate.safetyRevision,
  };
}

/** "3 组 × 12 次", "3 组 × 8–12 次", "1 组 × 30 秒", "2 组 × 400 米". */
export function formatPrescription(exercise: PlanExerciseRecord): string {
  const { target } = exercise;
  let amount = "";
  if (target.kind === "repetitions" && target.min !== undefined) {
    amount =
      target.max !== undefined && target.max !== target.min
        ? `${target.min}–${target.max} 次`
        : `${target.min} 次`;
  } else if (target.kind === "duration" && target.seconds !== undefined) {
    amount = `${target.seconds} 秒`;
  } else if (target.kind === "distance" && target.meters !== undefined) {
    amount = `${target.meters} 米`;
  }
  return amount ? `${exercise.sets} 组 × ${amount}` : `${exercise.sets} 组`;
}

export function formatRest(exercise: PlanExerciseRecord): string | null {
  return exercise.restSeconds !== undefined ? `休息 ${exercise.restSeconds} 秒` : null;
}

export type ExerciseChange = "modified" | "added";

export interface PlanExerciseView {
  planExerciseId: string;
  name: string;
  prescription: string;
  rest: string | null;
  side: string | null;
  load: string | null;
  /** Updated board: marks what the just-accepted revision changed. */
  change: ExerciseChange | null;
}

export interface PlanStageView {
  phase: PlanStageRecord["phase"];
  label: string;
  exercises: PlanExerciseView[];
}

export interface PlanTemplateView {
  templateId: string;
  title: string;
  exerciseCount: number;
  stages: PlanStageView[];
}

export interface ProposalChangeView {
  kind: "modified" | "added" | "removed";
  planExerciseId: string;
  name: string;
  before: string | null;
  after: string | null;
}

export type ProposalPhase = "pending" | "expired" | "accepted" | "rejected";

export interface ProposalView {
  proposalId: string;
  kind: PlanProposalRecord["kind"];
  phase: ProposalPhase;
  planId: string | null;
  title: string;
  byAgent: boolean;
  /** Whole days until the 7-day TTL ends (pending only, ≥ 0). */
  expiresInDays: number | null;
  expiresAt: string;
  expiredReason: ProposalExpiredReason | null;
  changes: ProposalChangeView[];
}

export type { TrainingPlanSummary };

export interface TrainingPlanView {
  planId: string;
  title: string;
  status: TrainingPlanRecord["status"];
  statusLabel: string;
  statusTone: TrainingTone;
  revisionId: string;
  frequencyLabel: string | null;
  scheduleLabel: string | null;
  templates: PlanTemplateView[];
  selectedTemplate: PlanTemplateView | null;
  gate: TrainingGateView;
  /** Only an active plan with a permitting gate can start a session. */
  canStart: boolean;
  pendingProposals: ProposalView[];
}

export type TrainingFocus =
  | { kind: "plan" }
  | { kind: "proposal"; proposalId: string }
  | {
      kind: "updated";
      receipt: PlanAcceptanceReceipt;
      /** The content the accepted proposal was compared against (null: create). */
      baseContent: PlanContent | null;
      /** The exact presented content the user accepted. */
      acceptedContent: PlanContent;
      created: boolean;
    };

export type TrainingScene =
  | "loading"
  | "error"
  | "empty"
  | "plan"
  | "proposal"
  | "expired"
  | "updated"
  | "updating";

export interface TrainingWindowView {
  scene: TrainingScene;
  plans: TrainingPlanSummary[];
  selected: TrainingPlanView | null;
  /** The proposal the Proposal / Expired scene shows. */
  proposal: ProposalView | null;
  /** Pending create proposals (a new plan the user has not accepted yet). */
  pendingCreates: ProposalView[];
  /** Persisted id no longer exists for this user. */
  selectedPlanMissing: boolean;
  updated: { created: boolean; changes: ProposalChangeView[] } | null;
}

function flattenExercises(content: PlanContent | null | undefined) {
  const map = new Map<string, PlanExerciseRecord>();
  for (const template of content?.sessions ?? [])
    for (const stage of template.stages)
      for (const exercise of stage.exercises)
        if (!map.has(exercise.planExerciseId)) map.set(exercise.planExerciseId, exercise);
  return map;
}

function describe(exercise: PlanExerciseRecord): string {
  return [formatPrescription(exercise), formatRest(exercise)]
    .filter(Boolean)
    .join(" · ");
}

function sameExercise(a: PlanExerciseRecord, b: PlanExerciseRecord): boolean {
  return (
    a.nameSnapshot === b.nameSnapshot &&
    a.exerciseId === b.exerciseId &&
    a.sets === b.sets &&
    a.restSeconds === b.restSeconds &&
    a.side === b.side &&
    a.loadOrIntensity === b.loadOrIntensity &&
    a.instructions === b.instructions &&
    JSON.stringify(a.target) === JSON.stringify(b.target)
  );
}

/**
 * Exercise-level diff of a proposal against the content it was proposed on.
 * Exercises are matched by their stable `planExerciseId`; a create proposal
 * compares against nothing, so every exercise is "added".
 */
export function diffPlanContent(
  base: PlanContent | null | undefined,
  next: PlanContent,
): ProposalChangeView[] {
  const before = flattenExercises(base);
  const after = flattenExercises(next);
  const changes: ProposalChangeView[] = [];
  for (const [id, exercise] of after) {
    const previous = before.get(id);
    if (!previous)
      changes.push({ kind: "added", planExerciseId: id, name: exercise.nameSnapshot, before: null, after: describe(exercise) });
    else if (!sameExercise(previous, exercise))
      changes.push({
        kind: "modified",
        planExerciseId: id,
        name: exercise.nameSnapshot,
        before: describe(previous),
        after: describe(exercise),
      });
  }
  for (const [id, exercise] of before)
    if (!after.has(id))
      changes.push({ kind: "removed", planExerciseId: id, name: exercise.nameSnapshot, before: describe(exercise), after: null });
  return changes;
}

const DAY_MS = 86_400_000;

export type ProposalExpiredReason = NonNullable<PlanProposalRecord["expiredReason"]>;

export function projectProposal(
  proposal: PlanProposalRecord,
  plans: readonly TrainingPlanRecord[],
  now: Date,
  /** A visible 409 on this proposal: it can no longer be accepted. */
  refused?: ProposalExpiredReason,
): ProposalView {
  const plan = proposal.planId ? plans.find((item) => item.planId === proposal.planId) : undefined;
  const expiresAt = Date.parse(proposal.expiresAt);
  // A pending update whose base revision is no longer current can never be
  // accepted; the server expires it on the next command, the UI says so now.
  const baseMoved =
    proposal.kind === "update" &&
    plan !== undefined &&
    proposal.basedOnRevisionId !== plan.currentAcceptedRevisionId;
  const timedOut = Number.isFinite(expiresAt) && expiresAt <= now.getTime();
  const phase: ProposalPhase =
    proposal.state === "pending"
      ? baseMoved || timedOut || refused
        ? "expired"
        : "pending"
      : proposal.state;
  const base = proposal.kind === "update" && plan && !baseMoved ? plan.currentContent : null;
  return {
    proposalId: proposal.proposalId,
    kind: proposal.kind,
    phase,
    planId: proposal.planId ?? null,
    title: proposal.draft.title,
    byAgent: proposal.initiatedBy === "agent",
    expiresInDays:
      phase === "pending" && Number.isFinite(expiresAt)
        ? Math.max(0, Math.ceil((expiresAt - now.getTime()) / DAY_MS))
        : null,
    expiresAt: proposal.expiresAt,
    expiredReason:
      proposal.expiredReason ??
      (phase === "expired" ? (refused ?? (baseMoved ? "base_revision_changed" : "timeout")) : null),
    changes: diffPlanContent(base, proposal.draft.content),
  };
}

function projectTemplate(
  template: PlanTemplate,
  marks: ReadonlyMap<string, ExerciseChange>,
): PlanTemplateView {
  const stages = template.stages
    .filter((stage) => stage.exercises.length > 0)
    .map((stage) => ({
      phase: stage.phase,
      label: STAGE_LABELS[stage.phase],
      exercises: stage.exercises.map((exercise) => ({
        planExerciseId: exercise.planExerciseId,
        name: exercise.nameSnapshot,
        prescription: formatPrescription(exercise),
        rest: formatRest(exercise),
        side: exercise.side ? SIDE_LABELS[exercise.side] : null,
        load: exercise.loadOrIntensity?.trim() || null,
        change: marks.get(exercise.planExerciseId) ?? null,
      })),
    }));
  return {
    templateId: template.templateId,
    title: template.title,
    exerciseCount: stages.reduce((count, stage) => count + stage.exercises.length, 0),
    stages,
  };
}

function frequencyLabel(content: PlanContent): string | null {
  if (content.frequency.sessionsPerWeek) return `每周 ${content.frequency.sessionsPerWeek} 次`;
  return content.frequency.description?.trim() || null;
}

export interface ProjectTrainingWindowInput {
  plans: readonly TrainingPlanRecord[] | undefined;
  proposals: readonly PlanProposalRecord[] | undefined;
  selectedPlanId: string | null;
  selectedTemplateId: string | null;
  focus: TrainingFocus;
  /** Proposals the server refused with a stale 409 in this page. */
  refused?: Readonly<Record<string, ProposalExpiredReason>>;
  loading: boolean;
  error: string | null;
  now: Date;
}

export function projectTrainingWindow(input: ProjectTrainingWindowInput): TrainingWindowView {
  const plans = input.plans ?? [];
  const proposals = (input.proposals ?? []).map((proposal) =>
    projectProposal(proposal, plans, input.now, input.refused?.[proposal.proposalId]),
  );
  const summaries = plans.map((plan) => ({ planId: plan.planId, title: plan.title, status: plan.status }));
  const updatedFocus = input.focus.kind === "updated" ? input.focus : null;
  const updated = updatedFocus
    ? {
        created: updatedFocus.created,
        changes: diffPlanContent(updatedFocus.baseContent, updatedFocus.acceptedContent),
      }
    : null;

  const effectiveId = updatedFocus?.receipt.planId ?? resolveSelectedPlanId(plans, input.selectedPlanId);
  const record = effectiveId ? plans.find((plan) => plan.planId === effectiveId) : undefined;
  // Update only after receipt: the Updated scene waits for the record that
  // carries exactly the accepted revision before it shows any new content.
  const receiptVisible = updatedFocus
    ? record?.currentAcceptedRevisionId === updatedFocus.receipt.revisionId
    : true;
  const marks = new Map<string, ExerciseChange>();
  if (updated && receiptVisible)
    for (const change of updated.changes)
      if (change.kind !== "removed") marks.set(change.planExerciseId, change.kind);

  let selected: TrainingPlanView | null = null;
  if (record && receiptVisible) {
    const templates = record.currentContent.sessions.map((template) => projectTemplate(template, marks));
    const gate = projectGate(record.executionGate);
    selected = {
      planId: record.planId,
      title: record.title,
      status: record.status,
      statusLabel: STATUS_LABELS[record.status].label,
      statusTone: STATUS_LABELS[record.status].tone,
      revisionId: record.currentAcceptedRevisionId,
      frequencyLabel: frequencyLabel(record.currentContent),
      scheduleLabel: record.currentContent.schedule.mode === "frequency_only" ? "仅频率" : "按日程",
      templates,
      selectedTemplate:
        templates.find((template) => template.templateId === input.selectedTemplateId) ?? templates[0] ?? null,
      gate,
      canStart: record.status === "active" && gate.permits,
      pendingProposals: proposals.filter(
        (proposal) => proposal.phase === "pending" && proposal.kind === "update" && proposal.planId === record.planId,
      ),
    };
  }

  const pendingCreates = proposals.filter((proposal) => proposal.phase === "pending" && proposal.kind === "create");
  const focused =
    input.focus.kind === "proposal"
      ? (proposals.find((proposal) => proposal.proposalId === (input.focus as { proposalId: string }).proposalId) ?? null)
      : null;
  // No plan yet: the newest pending create proposal is the window's content.
  const proposal = focused ?? (plans.length === 0 && input.focus.kind === "plan" ? (pendingCreates[0] ?? null) : null);

  let scene: TrainingScene;
  if (input.error) scene = "error";
  else if (input.loading && !input.plans) scene = "loading";
  else if (updated) scene = receiptVisible && selected ? "updated" : "updating";
  else if (proposal) scene = proposal.phase === "pending" ? "proposal" : "expired";
  else if (selected) scene = "plan";
  else scene = "empty";

  return {
    scene,
    plans: summaries,
    selected,
    proposal,
    pendingCreates,
    selectedPlanMissing:
      input.selectedPlanId !== null && plans.length > 0 && !plans.some((plan) => plan.planId === input.selectedPlanId),
    updated,
  };
}
