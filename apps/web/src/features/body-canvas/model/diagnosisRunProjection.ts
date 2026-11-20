import type {
  NativeDiagnosisCondition as NativeDiagnosisConditionOutput,
  NativeDiagnosisProposal as NativeDiagnosisRunProposalOutput,
  NativeDiagnosisRun as NativeDiagnosisRunProjectionOutput,
} from "@/features/consultation/services/nativeDiagnosisService";
import {
  EMPTY_DECISION_SESSION,
  PROPOSAL_STATE_LABELS,
  type DiagnosisDecisionSession,
  type DiagnosisRunPhase,
  type NativeDiagnosisWindowView,
  type ProjectNativeDiagnosisInput,
  type ProposalRow,
  type ProposalUiState,
} from "./diagnosisDecisionTypes";
import {
  describeProposal,
  proposalDetail,
  proposalKind,
  projectAdoptedCondition,
} from "./diagnosisProposalProjection";
import { projectRunReview } from "./diagnosisReview";

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
  const inactive = conditions
    .filter((condition) => condition.disposition === "inactive")
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
      inactive,
      run: null,
      safetyBlocked,
      safetyReview,
    };

  const runOpen = run.state === "open" && !ownSession.settlement;
  const review = runOpen ? projectRunReview(run) : null;
  const runChanged =
    review?.state === "stale" ||
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
    inactive,
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
      review,
    },
  };
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
