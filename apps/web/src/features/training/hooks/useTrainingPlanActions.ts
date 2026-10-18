import { useCallback, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { selectTrainingPlan } from "@/features/body-canvas/model/trainingPlanSelection";
import type {
  ProjectTrainingWindowInput,
  ProposalExpiredReason,
  TrainingFocus,
} from "@/features/body-canvas/model/trainingPlans";
import { workspaceKeys } from "@/features/workspace/api/workspaceQueryOptions";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
// Direct service import: the training barrel re-exports the execution UI,
// which must stay a separate lazy chunk.
import {
  newPlanDecisionIdentity,
  trainingApi,
  type PlanDecisionIdentity,
  type PlanProposalRecord,
} from "../services/trainingService";

export const trainingPlanKeys = {
  all: ["training", "plans"] as const,
  plans: () => [...trainingPlanKeys.all, "records"] as const,
  proposals: () => [...trainingPlanKeys.all, "proposals"] as const,
};

/** Stale 409s: the proposal can no longer be accepted, the plan is unchanged. */
const REFUSED_REASONS: Record<string, ProposalExpiredReason> = {
  PROPOSAL_EXPIRED: "timeout",
  CONTEXT_CHANGED: "relevant_context_changed",
  REVISION_CONFLICT: "base_revision_changed",
};

class ReceiptMismatchError extends Error {
  constructor() {
    super("服务器回执与本次操作不一致，未视为成功。请刷新后重试。");
    this.name = "ReceiptMismatchError";
  }
}

function decisionMessage(error: unknown): string {
  if (error instanceof ApiRequestError) {
    if (error.status === 409 && error.code === "CONTENT_CHANGED")
      return "建议内容与显示的不一致，已重新读取，请再核对一次。";
    if (error.status === 409 && error.code === "PROPOSAL_NOT_PENDING")
      return "这条建议已经处理过，已重新读取。";
    if (error.status === 503) return "暂时无法核对当前身体与安全情况，计划保持原样，请稍后再试。";
  }
  return errorMessage(error, "这次决定没有记录，计划保持原样。");
}

export interface UseTrainingPlanActionsOptions {
  enabled: boolean;
  selectedPlanId: string | null;
  selectedTemplateId: string | null;
  /** Called with the plan a create/update receipt names, so the header follows it. */
  onPlanAccepted?: (planId: string) => void;
}

/**
 * TrainingPlan window on the V2 plan API. Reads every plan record and
 * proposal; accepts or rejects one proposal as the user's explicit decision
 * on the exact presented content. Success is shown only from a matching
 * receipt, every decision keeps one idempotency identity across retries and
 * double clicks, and the plan content changes only once the refreshed record
 * carries the receipt's revision.
 *
 * It returns the raw projection input; the full view model is computed by
 * the lazily loaded window (chunk budget). `selection` is the light part the
 * always-loaded header and Trainer guard need.
 */
export function useTrainingPlanActions({
  enabled,
  selectedPlanId,
  selectedTemplateId,
  onPlanAccepted,
}: UseTrainingPlanActionsOptions) {
  const client = useQueryClient();
  const plans = useQuery({
    queryKey: trainingPlanKeys.plans(),
    queryFn: trainingApi.listPlanRecords,
    enabled,
    retry: false,
    staleTime: 0,
  });
  const proposals = useQuery({
    queryKey: trainingPlanKeys.proposals(),
    queryFn: trainingApi.listProposals,
    enabled,
    retry: false,
    staleTime: 0,
  });

  const [focus, setFocus] = useState<TrainingFocus>({ kind: "plan" });
  const [refused, setRefused] = useState<Record<string, ProposalExpiredReason>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<"rejected" | null>(null);
  const attempts = useRef(new Map<string, PlanDecisionIdentity>());
  const inFlight = useRef(false);

  const loadError =
    plans.isError || proposals.isError ? "训练计划暂时无法读取。" : null;
  const loading = enabled && (plans.isPending || proposals.isPending);
  const state: ProjectTrainingWindowInput = useMemo(
    () => ({
      plans: plans.data,
      proposals: proposals.data,
      selectedPlanId,
      selectedTemplateId,
      focus,
      refused,
      loading,
      error: loadError,
      now: new Date(),
    }),
    [plans.data, proposals.data, selectedPlanId, selectedTemplateId, focus, refused, loading, loadError],
  );
  const selection = useMemo(
    () =>
      selectTrainingPlan(
        plans.data,
        focus.kind === "updated" ? focus.receipt.planId : selectedPlanId,
      ),
    [plans.data, focus, selectedPlanId],
  );

  const refresh = useCallback(async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: trainingPlanKeys.all }),
      // The workspace projection (active plan, capabilities) follows the plan.
      client.invalidateQueries({ queryKey: workspaceKeys.all }),
    ]);
  }, [client]);

  const decide = useCallback(
    async (proposalId: string, decision: "accept" | "reject") => {
      if (inFlight.current) return;
      const record = proposals.data?.find((item) => item.proposalId === proposalId);
      if (!record) return;
      const key = `${proposalId}:${decision}:${record.contentFingerprint}`;
      let identity = attempts.current.get(key);
      if (!identity) {
        identity = newPlanDecisionIdentity();
        attempts.current.set(key, identity);
      }
      // Snapshot what the user was shown, before the refetch replaces it.
      const base =
        record.kind === "update"
          ? (plans.data?.find((plan) => plan.planId === record.planId)?.currentContent ?? null)
          : null;
      inFlight.current = true;
      setBusy(true);
      setError(null);
      try {
        if (decision === "accept") {
          const receipt = await trainingApi.acceptProposal(record, identity);
          if (receipt.proposalId !== proposalId) throw new ReceiptMismatchError();
          setFocus({
            kind: "updated",
            receipt,
            baseContent: base,
            acceptedContent: record.draft.content,
            created: record.kind === "create",
          });
          onPlanAccepted?.(receipt.planId);
        } else {
          const receipt = await trainingApi.rejectProposal(record, identity);
          if (receipt.proposalId !== proposalId) throw new ReceiptMismatchError();
          setFocus({ kind: "plan" });
          setNotice("rejected");
        }
        attempts.current.delete(key);
        await refresh();
      } catch (caught) {
        const reason =
          caught instanceof ApiRequestError && caught.status === 409 && caught.code
            ? REFUSED_REASONS[caught.code]
            : undefined;
        if (reason) {
          setRefused((current) => ({ ...current, [proposalId]: reason }));
          setFocus({ kind: "proposal", proposalId });
        } else setError(decisionMessage(caught));
        if (caught instanceof ApiRequestError && caught.status === 409) {
          attempts.current.delete(key);
          await refresh();
        }
      } finally {
        inFlight.current = false;
        setBusy(false);
      }
    },
    [proposals.data, plans.data, onPlanAccepted, refresh],
  );

  return {
    state,
    selection,
    busy,
    error,
    notice,
    openProposal: (proposal: Pick<PlanProposalRecord, "proposalId">) => {
      setError(null);
      setNotice(null);
      setFocus({ kind: "proposal", proposalId: proposal.proposalId });
    },
    backToPlan: () => {
      setError(null);
      setFocus({ kind: "plan" });
    },
    dismissNotice: () => setNotice(null),
    accept: (proposalId: string) => void decide(proposalId, "accept"),
    reject: (proposalId: string) => void decide(proposalId, "reject"),
  };
}

export type TrainingPlanActions = ReturnType<typeof useTrainingPlanActions>;
