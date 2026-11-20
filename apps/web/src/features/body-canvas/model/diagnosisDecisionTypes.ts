import type {
  NativeDiagnosisDecisionReceipt as NativeDiagnosisDecisionReceiptOutput,
  NativeDiagnosisProjection as NativeDiagnosisProjectionOutput,
  NativeDiagnosisProposal as NativeDiagnosisRunProposalOutput,
  NativeDiagnosisRun as NativeDiagnosisRunProjectionOutput,
  NativeDiagnosisSettlementReceipt as NativeDiagnosisSettlementReceiptOutput,
} from "@/features/consultation/services/nativeDiagnosisService";
import type { ConfirmedCondition } from "./confirmedConditions";
import type { DiagnosisFreshnessState } from "./diagnosisReview";

export type ProposalChoice = "accept" | "reject";
export type ProposalUiState =
  | "undecided"
  | "pending"
  | "rejected"
  | "changed"
  | "committed";

export const PROPOSAL_STATE_LABELS: Record<ProposalUiState, string> = {
  undecided: "待你决定",
  pending: "已选择采纳，尚未写入诊断",
  rejected: "未采纳",
  changed: "内容已变化，需要重新核对",
  committed: "已写入诊断",
};

/** Client-held results of this window session; never inferred from AI output. */
export interface DiagnosisDecisionSession {
  runId: string | null;
  /** Decision receipts returned by the decide command, keyed by proposal. */
  decisions: Record<string, NativeDiagnosisDecisionReceiptOutput>;
  settlement: NativeDiagnosisSettlementReceiptOutput | null;
  /** Proposals the server rejected as stale (409) and the user has not re-reviewed. */
  changedProposalIds: string[];
  /** A settlement or run-level command returned a revision conflict. */
  runChanged: boolean;
  /** Go-owned urgent Safety stopped the run (409 SAFETY_BLOCKED). */
  safetyBlocked: boolean;
}

export const EMPTY_DECISION_SESSION: DiagnosisDecisionSession = {
  runId: null,
  decisions: {},
  settlement: null,
  changedProposalIds: [],
  runChanged: false,
  safetyBlocked: false,
};

export interface ProposalRow {
  proposalId: string;
  changeKind: NativeDiagnosisRunProposalOutput["changeKind"];
  title: string;
  summary: string | null;
  levelLabel: string | null;
  state: ProposalUiState;
  stateLabel: string;
  decision: ProposalChoice | null;
  /** True only while the user may still record a choice for this proposal. */
  selectable: boolean;
  /** The user must acknowledge a changed proposal before choosing again. */
  needsReview: boolean;
  contentFingerprint: string;
  /** 1-based place in the run, for the Penpot "proposal n / total" marker. */
  position: number;
  total: number;
  /** Penpot proposal kind chip: 新建 / 更新 / 停用 / 调整状态 / 调整关联. */
  kindLabel: string;
  kindTone: "ok" | "idle";
  /** Evidence counts the Agent bound to this proposal (references only). */
  supportCount: number;
  counterCount: number;
  /** Cause rationale and open information gaps the proposal carries. */
  rationale: string | null;
  gaps: string[];
}

/**
 * Which Penpot Diagnosis scene the run is in. Every terminal phase except
 * `safety_stopped` comes from a receipt or the server run state, never from
 * a click alone.
 * - `deciding`: Confirm first / Confirm second / First rejected
 * - `committed`: Receipt (only new conditions adopted)
 * - `updated`: Update receipt (an existing condition changed)
 * - `closed_without_change`: No changes
 * - `safety_stopped`: Safety stopped (zero Diagnosis writes)
 */
export type DiagnosisRunPhase =
  | "deciding"
  | "committed"
  | "updated"
  | "closed_without_change"
  | "safety_stopped";

export interface DiagnosisRunView {
  runId: string;
  state: NativeDiagnosisRunProjectionOutput["state"];
  outcome: NativeDiagnosisRunProjectionOutput["outcome"] | null;
  expectedRevision: number;
  proposals: ProposalRow[];
  /** Whole run no longer matches the current Diagnosis revision. */
  changed: boolean;
  canDecide: boolean;
  canSettle: boolean;
  undecidedCount: number;
  acceptedCount: number;
  /** Settling now would close the remaining undecided proposals unadopted. */
  closesRemaining: boolean;
  /** Receipt id proving adoption; null means nothing may be shown as committed. */
  committedReceiptId: string | null;
  /** When the committed settlement receipt was issued (ISO with offset). */
  committedAt: string | null;
  phase: DiagnosisRunPhase;
  /** Proposals the receipt proves were written; 0 unless committed/updated. */
  committedCount: number;
  /** Derived freshness of an open run; stale runs cannot be decided. */
  review: {
    state: DiagnosisFreshnessState;
    title: string;
    details: string[];
  } | null;
}

export interface NativeDiagnosisWindowView {
  currentRevision: number | null;
  confirmed: ConfirmedCondition[];
  /** Withdrawn/deactivated entries, shown only in Diagnosis History. */
  inactive: ConfirmedCondition[];
  run: DiagnosisRunView | null;
  safetyBlocked: boolean;
  /** Workspace-level Safety review is pending; every decision is read-only. */
  safetyReview: boolean;
}

export interface ProjectNativeDiagnosisInput {
  diagnosis: NativeDiagnosisProjectionOutput | null | undefined;
  run: NativeDiagnosisRunProjectionOutput | null | undefined;
  session?: DiagnosisDecisionSession;
  /** Workspace-level Safety review requirement; decisions stay read-only. */
  safetyReview?: boolean;
  busy?: boolean;
}

export interface DiagnosisStepDecision {
  row: ProposalRow;
  choice: ProposalChoice;
  /** Only a local, not yet recorded choice can still be changed. */
  changeable: boolean;
}

/**
 * Penpot Confirm first / Confirm second / First rejected: one proposal at a
 * time over the same decisions. Local choices stay in the selection until the
 * final step records them and settles the run in one go.
 */
export interface DiagnosisStepper {
  /** 1-based step shown as "n / total". */
  step: number;
  total: number;
  decided: DiagnosisStepDecision[];
  /** The proposal the user acts on now; null once every proposal has a choice. */
  current: ProposalRow | null;
  next: ProposalRow | null;
  /** Deciding the current proposal finishes the run: show "采纳并提交". */
  isLast: boolean;
  /** Something will be adopted; without it the last reject is "不更改". */
  anyAccepted: boolean;
  /** Choices exist and later proposals remain: settling now closes them. */
  canSubmitEarly: boolean;
}
