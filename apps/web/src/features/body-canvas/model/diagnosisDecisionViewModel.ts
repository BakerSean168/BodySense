import type {
  NativeDiagnosisCondition as NativeDiagnosisConditionOutput,
  NativeDiagnosisDecisionReceipt as NativeDiagnosisDecisionReceiptOutput,
  NativeDiagnosisProjection as NativeDiagnosisProjectionOutput,
  NativeDiagnosisProposal as NativeDiagnosisRunProposalOutput,
  NativeDiagnosisRun as NativeDiagnosisRunProjectionOutput,
  NativeDiagnosisSettlementReceipt as NativeDiagnosisSettlementReceiptOutput,
} from "@/features/consultation/services/nativeDiagnosisService";
import { ApiRequestError } from "@/lib/api-client";
import type {
  ConditionCertainty,
  ConfirmedCondition,
} from "./confirmedConditions";

/**
 * Diagnosis window view model on the V2 decision API.
 *
 * Sources of truth:
 * - `GET /diagnosis` is the only list of conditions. Only `adopted` entries are
 *   shown as conditions; Agent proposals never are.
 * - The latest formal run carries the Agent proposals. They are AI output, so
 *   they stay proposals until the user decides and Go settles the whole run.
 * - A decision receipt only records a choice. A proposal is `committed` only
 *   when a settlement receipt with outcome `committed` exists for its run.
 */

export type ProposalChoice = "accept" | "reject";
export type ProposalUiState =
  "undecided" | "pending" | "rejected" | "changed" | "committed";

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
}

export interface NativeDiagnosisWindowView {
  currentRevision: number | null;
  confirmed: ConfirmedCondition[];
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

const LEVEL_LABELS: Record<string, string> = {
  suspected: "疑似",
  likely: "较可能",
};

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function text(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function texts(values: readonly string[] | undefined): string[] {
  return (values ?? []).flatMap((value) => {
    const trimmed = value.trim();
    return trimmed ? [trimmed] : [];
  });
}

function certaintyOf(
  condition: NativeDiagnosisConditionOutput,
): ConditionCertainty {
  const kinds = condition.sourceClaimRefs.map((claim) => claim.sourceKind);
  if (kinds.includes("clinical_document")) return "clinically_documented";
  if (kinds.includes("user_reported_clinical"))
    return "user_reported_clinical_diagnosis";
  return "user_confirmed_agent_analysis";
}

export function projectAdoptedCondition(
  condition: NativeDiagnosisConditionOutput,
): ConfirmedCondition {
  const assessment = condition.assessment;
  const handling = condition.handlingPlan;
  const hasHandling =
    !!text(handling.summary) ||
    texts(handling.actions).length > 0 ||
    texts(handling.cautionNotes).length > 0;
  return {
    conditionId: condition.conditionId,
    name: condition.medicalName,
    certainty: certaintyOf(condition),
    lifecycle: "active",
    cause: {
      summary: text(condition.causeAnalysis.summary),
      basis: text(condition.causeAnalysis.rationale),
      basisFactIds: condition.evidenceLinks
        .filter((link) => link.role === "supporting")
        .map((link) => link.evidenceId),
      evidenceStrength: assessment
        ? (LEVEL_LABELS[assessment.level] ?? null)
        : null,
      counterEvidenceCount: condition.evidenceLinks.filter(
        (link) => link.role === "counter",
      ).length,
      informationGaps: texts(condition.informationGaps),
      differential: assessment
        ? texts(assessment.differentialNotes).join("；") || null
        : null,
    },
    handling: hasHandling
      ? {
          state: "condition_level",
          summary: text(handling.summary) ?? undefined,
          actions: texts(handling.actions),
          cautionNotes: texts(handling.cautionNotes),
        }
      : { state: "not_confirmed" },
    safetyNotes: [],
  };
}

function strings(value: unknown): string[] {
  return Array.isArray(value)
    ? value.flatMap((item) => {
        const trimmed = text(item);
        return trimmed ? [trimmed] : [];
      })
    : [];
}

function proposalKind(
  proposal: NativeDiagnosisRunProposalOutput,
): Pick<ProposalRow, "kindLabel" | "kindTone"> {
  switch (proposal.changeKind) {
    case "create":
      return { kindLabel: "新建", kindTone: "ok" };
    case "update":
      return { kindLabel: "更新", kindTone: "ok" };
    case "set_disposition":
      return text(record(proposal.content)?.disposition) === "inactive"
        ? { kindLabel: "停用", kindTone: "idle" }
        : { kindLabel: "调整状态", kindTone: "idle" };
    case "update_relation":
      return { kindLabel: "调整关联", kindTone: "idle" };
  }
}

function proposalDetail(
  proposal: NativeDiagnosisRunProposalOutput,
): Pick<ProposalRow, "supportCount" | "counterCount" | "rationale" | "gaps"> {
  const entry = record(record(proposal.content)?.proposedEntry);
  const cause = record(entry?.causeAnalysis);
  const assessment = record(entry?.assessment);
  const gaps = strings(entry?.informationGaps);
  return {
    supportCount: proposal.evidenceRefs.length,
    counterCount: proposal.counterEvidenceRefs.length,
    rationale: text(cause?.rationale) ?? text(assessment?.rationale),
    gaps: gaps.length > 0 ? gaps : strings(assessment?.informationGaps),
  };
}

function describeProposal(
  proposal: NativeDiagnosisRunProposalOutput,
  conditions: readonly NativeDiagnosisConditionOutput[],
): Pick<ProposalRow, "title" | "summary" | "levelLabel"> {
  const content = record(proposal.content);
  const entry = record(content?.proposedEntry);
  const target = conditions.find(
    (condition) => condition.conditionId === proposal.targetConditionId,
  );
  const targetName = target?.medicalName ?? "现有病情";
  const assessment = record(entry?.assessment);
  const level = text(assessment?.level);
  const cause = record(entry?.causeAnalysis);
  switch (proposal.changeKind) {
    case "create":
      return {
        title: text(entry?.medicalName) ?? "新的病情提案",
        summary: text(cause?.summary),
        levelLabel: level ? (LEVEL_LABELS[level] ?? null) : null,
      };
    case "update":
      return { title: `更新：${targetName}`, summary: null, levelLabel: null };
    case "set_disposition":
      return {
        title: `调整状态：${targetName}`,
        summary: text(content?.dispositionReason),
        levelLabel: null,
      };
    case "update_relation":
      return {
        title: `调整关联：${targetName}`,
        summary: null,
        levelLabel: null,
      };
  }
}

interface CommittedReceipt {
  receiptId: string;
  decisionIds: string[] | null;
  settledAt: string;
}

function committedReceipt(
  run: NativeDiagnosisRunProjectionOutput,
  session: DiagnosisDecisionSession,
): CommittedReceipt | null {
  const local = session.settlement;
  // A receipt this session holds is authoritative for its run.
  if (local && local.runId === run.runId)
    return local.outcome === "committed"
      ? {
          receiptId: local.receiptId,
          decisionIds: local.decisionIds,
          settledAt: local.settledAt,
        }
      : null;
  const server = run.settlement;
  if (run.state === "committed" && server?.outcome === "committed")
    return {
      receiptId: server.receiptId,
      decisionIds: null,
      settledAt: server.settledAt,
    };
  return null;
}

function proposalRow(
  proposal: NativeDiagnosisRunProposalOutput,
  input: {
    conditions: readonly NativeDiagnosisConditionOutput[];
    session: DiagnosisDecisionSession;
    expectedRevision: number;
    runChanged: boolean;
    runOpen: boolean;
    canDecide: boolean;
    receipt: CommittedReceipt | null;
    position: number;
    total: number;
  },
): ProposalRow {
  const { session } = input;
  const localDecision = session.decisions[proposal.proposalId];
  const serverDecision = proposal.decision;
  const decision =
    localDecision &&
    localDecision.contentFingerprint === proposal.contentFingerprint
      ? {
          decision: localDecision.decision,
          decisionId: localDecision.decisionId,
        }
      : serverDecision
        ? {
            decision: serverDecision.decision,
            decisionId: serverDecision.decisionId,
          }
        : null;
  const flagged = session.changedProposalIds.includes(proposal.proposalId);
  const closedWithRun =
    proposal.state === "expired" && proposal.reason === "run_closed";
  const changed =
    flagged ||
    (!decision && input.runOpen && input.runChanged) ||
    proposal.state === "superseded" ||
    (proposal.state === "expired" && !closedWithRun) ||
    (!decision &&
      input.runOpen &&
      proposal.baseRevision !== input.expectedRevision);

  let state: ProposalUiState;
  if (changed) state = "changed";
  else if (decision?.decision === "reject" || closedWithRun) state = "rejected";
  else if (decision?.decision === "accept")
    state =
      input.receipt &&
      (input.receipt.decisionIds === null ||
        input.receipt.decisionIds.includes(decision.decisionId))
        ? "committed"
        : "pending";
  else state = "undecided";

  return {
    proposalId: proposal.proposalId,
    changeKind: proposal.changeKind,
    ...describeProposal(proposal, input.conditions),
    ...proposalKind(proposal),
    ...proposalDetail(proposal),
    state,
    stateLabel: closedWithRun
      ? "未决定，已随本次分析关闭"
      : PROPOSAL_STATE_LABELS[state],
    decision: decision?.decision ?? null,
    selectable: state === "undecided" && input.canDecide,
    needsReview: flagged,
    contentFingerprint: proposal.contentFingerprint,
    position: input.position,
    total: input.total,
  };
}

function runPhase(
  run: NativeDiagnosisRunProjectionOutput,
  session: DiagnosisDecisionSession,
  safetyBlocked: boolean,
  proposals: readonly ProposalRow[],
  receipt: { receiptId: string } | null,
): DiagnosisRunPhase {
  if (safetyBlocked) return "safety_stopped";
  if (receipt)
    return proposals.some(
      (row) => row.state === "committed" && row.changeKind !== "create",
    )
      ? "updated"
      : "committed";
  const settledWithoutChange =
    session.settlement?.runId === run.runId &&
    session.settlement.outcome === "closed_without_change";
  if (settledWithoutChange || run.state !== "open")
    return "closed_without_change";
  return "deciding";
}

export function projectNativeDiagnosisWindow({
  diagnosis,
  run,
  session = EMPTY_DECISION_SESSION,
  safetyReview = false,
  busy = false,
}: ProjectNativeDiagnosisInput): NativeDiagnosisWindowView {
  const conditions = diagnosis?.conditionEntries ?? [];
  const confirmed = conditions
    .filter((condition) => condition.disposition === "adopted")
    .map(projectAdoptedCondition);
  const currentRevision = diagnosis ? diagnosis.currentRevision : null;
  const ownSession =
    run && session.runId === run.runId ? session : EMPTY_DECISION_SESSION;
  const safetyBlocked =
    !!run &&
    (run.outcome === "safety_blocked" ||
      run.settlement?.outcome === "safety_blocked" ||
      ownSession.settlement?.outcome === "safety_blocked" ||
      ownSession.safetyBlocked);

  // Terminal runs are only listed when this session acted on them, so the
  // window can show the receipt outcome; otherwise the committed list rules.
  const visible =
    !!run &&
    currentRevision !== null &&
    (run.state === "open" || ownSession.runId === run.runId);
  if (!run || !visible || currentRevision === null)
    return {
      currentRevision,
      confirmed,
      run: null,
      safetyBlocked,
      safetyReview,
    };

  const runOpen = run.state === "open" && !ownSession.settlement;
  const runChanged =
    ownSession.runChanged ||
    run.basedOnDiagnosisRevision !== currentRevision ||
    run.currentDiagnosisRevision !== currentRevision;
  const canDecide =
    runOpen && !runChanged && !safetyBlocked && !safetyReview && !busy;
  const receipt = committedReceipt(run, ownSession);
  const total = run.proposals.length;
  const proposals = run.proposals.map((proposal, index) =>
    proposalRow(proposal, {
      conditions,
      session: ownSession,
      expectedRevision: currentRevision,
      runChanged,
      runOpen,
      canDecide,
      receipt,
      position: index + 1,
      total,
    }),
  );
  const undecidedCount = proposals.filter(
    (row) => row.state === "undecided",
  ).length;
  const acceptedCount = proposals.filter(
    (row) => row.state === "pending",
  ).length;
  const blockedByChange = proposals.some((row) => row.needsReview);
  return {
    currentRevision,
    confirmed,
    safetyBlocked,
    safetyReview,
    run: {
      runId: run.runId,
      state: run.state,
      outcome: run.outcome ?? null,
      expectedRevision: currentRevision,
      proposals,
      changed: runChanged,
      canDecide,
      canSettle: canDecide && !blockedByChange,
      undecidedCount,
      acceptedCount,
      closesRemaining: undecidedCount > 0,
      committedReceiptId: receipt?.receiptId ?? null,
      committedAt: receipt?.settledAt ?? null,
      phase: runPhase(run, ownSession, safetyBlocked, proposals, receipt),
      committedCount: receipt
        ? proposals.filter((row) => row.state === "committed").length
        : 0,
    },
  };
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

export function projectDiagnosisStepper(
  run: DiagnosisRunView,
  selection: Readonly<Record<string, ProposalChoice>>,
): DiagnosisStepper {
  const decided: DiagnosisStepDecision[] = [];
  const open: ProposalRow[] = [];
  for (const row of run.proposals) {
    const local = row.selectable ? selection[row.proposalId] : undefined;
    if (local) decided.push({ row, choice: local, changeable: true });
    else if (row.state === "pending" || row.state === "committed")
      decided.push({ row, choice: "accept", changeable: false });
    else if (row.state === "rejected")
      decided.push({ row, choice: "reject", changeable: false });
    else open.push(row);
  }
  const current = open[0] ?? null;
  const total = run.proposals.length;
  return {
    step: Math.min(decided.length + 1, total),
    total,
    decided,
    current,
    next: open[1] ?? null,
    isLast: open.length <= 1,
    anyAccepted: decided.some((item) => item.choice === "accept"),
    canSubmitEarly: decided.length > 0 && open.length > 1,
  };
}

function forRun(
  session: DiagnosisDecisionSession,
  runId: string,
): DiagnosisDecisionSession {
  return session.runId === runId
    ? session
    : { ...EMPTY_DECISION_SESSION, runId };
}

export function withDecisionReceipt(
  session: DiagnosisDecisionSession,
  receipt: NativeDiagnosisDecisionReceiptOutput,
): DiagnosisDecisionSession {
  const next = forRun(session, receipt.runId);
  return {
    ...next,
    decisions: { ...next.decisions, [receipt.proposalId]: receipt },
    changedProposalIds: next.changedProposalIds.filter(
      (id) => id !== receipt.proposalId,
    ),
  };
}

export function withSettlementReceipt(
  session: DiagnosisDecisionSession,
  receipt: NativeDiagnosisSettlementReceiptOutput,
): DiagnosisDecisionSession {
  const next = forRun(session, receipt.runId);
  return {
    ...next,
    settlement: receipt,
    safetyBlocked: next.safetyBlocked || receipt.outcome === "safety_blocked",
  };
}

/**
 * Applies a failed command. A 409 never leaves an optimistic success behind:
 * revision conflicts mark the proposal (or run) changed so the user must
 * re-review the refreshed content; SAFETY_BLOCKED freezes the run.
 */
export function withCommandError(
  session: DiagnosisDecisionSession,
  runId: string,
  error: unknown,
  proposalId?: string,
): DiagnosisDecisionSession {
  const next = forRun(session, runId);
  if (!(error instanceof ApiRequestError) || error.status !== 409) return next;
  if (error.code === "SAFETY_BLOCKED") return { ...next, safetyBlocked: true };
  if (!proposalId) return { ...next, runChanged: true };
  return next.changedProposalIds.includes(proposalId)
    ? next
    : { ...next, changedProposalIds: [...next.changedProposalIds, proposalId] };
}

/** The user re-read a changed proposal and may choose again. */
export function withProposalReviewed(
  session: DiagnosisDecisionSession,
  proposalId: string,
): DiagnosisDecisionSession {
  return {
    ...session,
    changedProposalIds: session.changedProposalIds.filter(
      (id) => id !== proposalId,
    ),
  };
}

/** Same content ⇒ same idempotency identity, so a retry never double-records. */
export function decisionAttemptContent(
  runId: string,
  row: Pick<ProposalRow, "proposalId" | "contentFingerprint">,
  decision: ProposalChoice,
  expectedRevision: number,
): string {
  return JSON.stringify([
    "decide",
    runId,
    row.proposalId,
    row.contentFingerprint,
    decision,
    expectedRevision,
  ]);
}

export function settlementAttemptContent(
  runId: string,
  expectedRevision: number,
  closeRemaining: boolean,
): string {
  return JSON.stringify(["settle", runId, expectedRevision, closeRemaining]);
}

/** Penpot toolbar label: "正式分析" while a run is shown, "提交回执" after a commit. */
export function diagnosisHeaderMode(
  view: Pick<NativeDiagnosisWindowView, "run">,
): "analysis" | "receipt" | null {
  const run = view.run;
  if (!run || run.proposals.length === 0) return null;
  return run.phase === "committed" || run.phase === "updated"
    ? "receipt"
    : "analysis";
}
