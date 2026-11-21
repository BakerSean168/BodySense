import {
  SIDE_LABELS,
  STATUS_LABELS,
  STAGE_LABELS,
  formatPrescription,
  formatRest,
  projectGate,
} from "./trainingPlanPresentation";
import type {
  ExerciseChange,
  PlanContent,
  PlanTemplate,
  PlanTemplateView,
  ProposalExpiredReason,
  TrainingFocus,
  TrainingPlanView,
  TrainingScene,
  TrainingWindowView,
} from "./trainingPlanViewTypes";
import { resolveSelectedPlanId } from "./trainingPlanSelection";
import { diffPlanContent } from "./trainingPlanDiff";
import { projectProposal } from "./trainingPlanProposalProjection";
import type {
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
 *
 * The module itself owns the window assembly; presentation, diff and proposal
 * projection live in the sibling `trainingPlan*` modules and are re-exported
 * here so callers keep a single import site.
 */

export type {
  ExerciseChange,
  PlanExerciseView,
  PlanStageView,
  PlanTemplateView,
  ProposalChangeView,
  ProposalExpiredReason,
  ProposalPhase,
  ProposalView,
  TrainingFocus,
  TrainingGateView,
  TrainingPlanSummary,
  TrainingPlanView,
  TrainingScene,
  TrainingTone,
  TrainingWindowView,
} from "./trainingPlanViewTypes";
export {
  STAGE_LABELS,
  formatPrescription,
  formatRest,
  gateReasonLabel,
  projectGate,
} from "./trainingPlanPresentation";
export { diffPlanContent } from "./trainingPlanDiff";
export { projectProposal } from "./trainingPlanProposalProjection";

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
    exerciseCount: stages.reduce(
      (count, stage) => count + stage.exercises.length,
      0,
    ),
    stages,
  };
}

function frequencyLabel(content: PlanContent): string | null {
  if (content.frequency.sessionsPerWeek)
    return `每周 ${content.frequency.sessionsPerWeek} 次`;
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

export function projectTrainingWindow(
  input: ProjectTrainingWindowInput,
): TrainingWindowView {
  const plans = input.plans ?? [];
  const proposals = (input.proposals ?? []).map((proposal) =>
    projectProposal(
      proposal,
      plans,
      input.now,
      input.refused?.[proposal.proposalId],
    ),
  );
  const summaries = plans.map((plan) => ({
    planId: plan.planId,
    title: plan.title,
    status: plan.status,
  }));
  const updatedFocus = input.focus.kind === "updated" ? input.focus : null;
  const updated = updatedFocus
    ? {
        created: updatedFocus.created,
        changes: diffPlanContent(
          updatedFocus.baseContent,
          updatedFocus.acceptedContent,
        ),
      }
    : null;

  const effectiveId =
    updatedFocus?.receipt.planId ??
    resolveSelectedPlanId(plans, input.selectedPlanId);
  const record = effectiveId
    ? plans.find((plan) => plan.planId === effectiveId)
    : undefined;
  // Update only after receipt: the Updated scene waits for the record that
  // carries exactly the accepted revision before it shows any new content.
  const receiptVisible = updatedFocus
    ? record?.currentAcceptedRevisionId === updatedFocus.receipt.revisionId
    : true;
  const marks = new Map<string, ExerciseChange>();
  if (updated && receiptVisible)
    for (const change of updated.changes)
      if (change.kind !== "removed")
        marks.set(change.planExerciseId, change.kind);

  let selected: TrainingPlanView | null = null;
  if (record && receiptVisible) {
    const templates = record.currentContent.sessions.map((template) =>
      projectTemplate(template, marks),
    );
    const gate = projectGate(record.executionGate);
    selected = {
      planId: record.planId,
      title: record.title,
      status: record.status,
      statusLabel: STATUS_LABELS[record.status].label,
      statusTone: STATUS_LABELS[record.status].tone,
      revisionId: record.currentAcceptedRevisionId,
      frequencyLabel: frequencyLabel(record.currentContent),
      scheduleLabel:
        record.currentContent.schedule.mode === "frequency_only"
          ? "仅频率"
          : "按日程",
      templates,
      selectedTemplate:
        templates.find(
          (template) => template.templateId === input.selectedTemplateId,
        ) ??
        templates[0] ??
        null,
      gate,
      canStart: record.status === "active" && gate.permits,
      pendingProposals: proposals.filter(
        (proposal) =>
          proposal.phase === "pending" &&
          proposal.kind === "update" &&
          proposal.planId === record.planId,
      ),
    };
  }

  const pendingCreates = proposals.filter(
    (proposal) => proposal.phase === "pending" && proposal.kind === "create",
  );
  const focused =
    input.focus.kind === "proposal"
      ? (proposals.find(
          (proposal) =>
            proposal.proposalId ===
            (input.focus as { proposalId: string }).proposalId,
        ) ?? null)
      : null;
  // No plan yet: the newest pending create proposal is the window's content.
  const proposal =
    focused ??
    (plans.length === 0 && input.focus.kind === "plan"
      ? (pendingCreates[0] ?? null)
      : null);

  let scene: TrainingScene;
  if (input.error) scene = "error";
  else if (input.loading && !input.plans) scene = "loading";
  else if (updated) scene = receiptVisible && selected ? "updated" : "updating";
  else if (proposal)
    scene = proposal.phase === "pending" ? "proposal" : "expired";
  else if (selected) scene = "plan";
  else scene = "empty";

  return {
    scene,
    plans: summaries,
    selected,
    proposal,
    pendingCreates,
    selectedPlanMissing:
      input.selectedPlanId !== null &&
      plans.length > 0 &&
      !plans.some((plan) => plan.planId === input.selectedPlanId),
    updated,
  };
}
