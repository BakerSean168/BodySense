import type {
  ProposalExpiredReason,
  ProposalPhase,
  ProposalView,
} from "./trainingPlanViewTypes";
import type {
  PlanProposalRecord,
  TrainingPlanRecord,
} from "@/features/training/services/trainingService";
import { diffPlanContent } from "./trainingPlanDiff";

/** The plan-proposal TTL is 7 days; the UI shows whole days remaining. */
const DAY_MS = 86_400_000;

export function projectProposal(
  proposal: PlanProposalRecord,
  plans: readonly TrainingPlanRecord[],
  now: Date,
  /** A visible 409 on this proposal: it can no longer be accepted. */
  refused?: ProposalExpiredReason,
): ProposalView {
  const plan = proposal.planId
    ? plans.find((item) => item.planId === proposal.planId)
    : undefined;
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
  const base =
    proposal.kind === "update" && plan && !baseMoved
      ? plan.currentContent
      : null;
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
      (phase === "expired"
        ? (refused ?? (baseMoved ? "base_revision_changed" : "timeout"))
        : null),
    changes: diffPlanContent(base, proposal.draft.content),
  };
}
