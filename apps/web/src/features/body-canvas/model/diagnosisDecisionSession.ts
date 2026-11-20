import type {
  NativeDiagnosisDecisionReceipt as NativeDiagnosisDecisionReceiptOutput,
  NativeDiagnosisSettlementReceipt as NativeDiagnosisSettlementReceiptOutput,
} from "@/features/consultation/services/nativeDiagnosisService";
import { ApiRequestError } from "@/lib/api-client";
import {
  EMPTY_DECISION_SESSION,
  type DiagnosisDecisionSession,
  type ProposalChoice,
  type ProposalRow,
} from "./diagnosisDecisionTypes";

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
