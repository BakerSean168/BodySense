import type { TrainingPlanSummary } from "./trainingPlanSelection";
import type {
  PlanAcceptanceReceipt,
  PlanProposalRecord,
  TrainingPlanRecord,
} from "@/features/training/services/trainingService";

/**
 * Shape of the TrainingPlan window view model (UI-TP-001), projected from the
 * V2 `/training-plans` and `/plan-proposals` records. The projections live in
 * the sibling `trainingPlan*` modules; these types carry the read-only rules:
 * a plan record is always a user-accepted revision, execution permission comes
 * from the Go TrainerGate projection, and a missing prescription value stays
 * missing.
 */

export type PlanContent = TrainingPlanRecord["currentContent"];
export type PlanTemplate = PlanContent["sessions"][number];
export type PlanStageRecord = PlanTemplate["stages"][number];
export type PlanExerciseRecord = PlanStageRecord["exercises"][number];
export type Gate = TrainingPlanRecord["executionGate"];

export type TrainingTone = "ok" | "idle" | "warn" | "bad" | "ai" | "neutral";

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

export type ProposalExpiredReason = NonNullable<
  PlanProposalRecord["expiredReason"]
>;

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
