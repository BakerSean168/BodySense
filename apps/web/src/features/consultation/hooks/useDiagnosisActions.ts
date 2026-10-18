import { useCallback, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  EMPTY_DECISION_SESSION,
  decisionAttemptContent,
  projectNativeDiagnosisWindow,
  settlementAttemptContent,
  withCommandError,
  withDecisionReceipt,
  withProposalReviewed,
  withSettlementReceipt,
  type DiagnosisDecisionSession,
  type ProposalChoice,
} from "@/features/body-canvas/model/diagnosisDecisionViewModel";
import { workspaceKeys } from "@/features/workspace/api/workspaceQueryOptions";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { nativeDiagnosisService as api } from "../services/nativeDiagnosisService";

export const nativeDiagnosisKeys = {
  all: ["diagnosis", "native"] as const,
  current: () => [...nativeDiagnosisKeys.all, "current"] as const,
  latestRun: () => [...nativeDiagnosisKeys.all, "latest-run"] as const,
  run: (runId: string | null) =>
    [...nativeDiagnosisKeys.all, "run", runId] as const,
  history: () => [...nativeDiagnosisKeys.all, "history"] as const,
};

/** Penpot Diagnosis boards: the current Diagnosis, or its decision History. */
export type DiagnosisScene = "current" | "history";

interface UseDiagnosisActionsOptions {
  enabled?: boolean;
  /** Workspace Safety review requirement keeps every decision read-only. */
  safetyReview?: boolean;
}

interface CommandIdentity {
  idempotencyKey: string;
  toolCallId: string;
}

function commandMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError && error.status === 409) {
    return error.code === "SAFETY_BLOCKED"
      ? "安全提醒已停止本次分析，提案不会写入诊断。"
      : "提案或依据已经变化，已重新读取，请核对后再决定。";
  }
  return errorMessage(error, fallback);
}

class ReceiptMismatchError extends Error {
  constructor() {
    super("服务器回执与本次操作不一致，未视为成功。请刷新后重试。");
    this.name = "ReceiptMismatchError";
  }
}

/**
 * Diagnosis window on the V2 decision API: reads `GET /diagnosis` and the
 * latest formal run, records explicit per-proposal choices, then settles the
 * whole run. Success is only ever shown from a matching server receipt; every
 * command carries an idempotency key and the expected Diagnosis revision.
 */
export function useDiagnosisActions({
  enabled = true,
  safetyReview = false,
}: UseDiagnosisActionsOptions = {}) {
  const client = useQueryClient();
  const diagnosis = useQuery({
    queryKey: nativeDiagnosisKeys.current(),
    queryFn: api.current,
    enabled,
    retry: false,
    staleTime: 0,
  });
  const latest = useQuery({
    queryKey: nativeDiagnosisKeys.latestRun(),
    queryFn: api.latestRun,
    enabled,
    retry: false,
    staleTime: 0,
  });
  const runId = latest.data?.runId ?? null;
  const run = useQuery({
    queryKey: nativeDiagnosisKeys.run(runId),
    queryFn: () => api.run(runId!),
    enabled: enabled && runId !== null,
    retry: false,
    staleTime: 0,
  });

  const [session, setSession] = useState<DiagnosisDecisionSession>(
    EMPTY_DECISION_SESSION,
  );
  const [selection, setSelection] = useState<Record<string, ProposalChoice>>(
    {},
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [scene, setScene] = useState<DiagnosisScene>("current");
  const attempts = useRef(new Map<string, CommandIdentity>());

  const fetching = diagnosis.isFetching || latest.isFetching || run.isFetching;
  const view = useMemo(
    () =>
      projectNativeDiagnosisWindow({
        diagnosis: diagnosis.data,
        run: runId ? run.data : null,
        session,
        safetyReview,
        busy: busy || fetching,
      }),
    [diagnosis.data, run.data, runId, session, safetyReview, busy, fetching],
  );

  const identity = (content: string): CommandIdentity => {
    const existing = attempts.current.get(content);
    if (existing) return existing;
    const created = {
      idempotencyKey: crypto.randomUUID(),
      toolCallId: `web:diagnosis-window:${crypto.randomUUID()}`,
    };
    attempts.current.set(content, created);
    return created;
  };

  const refresh = useCallback(async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: nativeDiagnosisKeys.all }),
      client.invalidateQueries({ queryKey: workspaceKeys.all }),
    ]);
  }, [client]);

  const select = useCallback(
    (proposalId: string, choice: ProposalChoice | null) => {
      setSelection((current) => {
        const next = { ...current };
        if (choice) next[proposalId] = choice;
        else delete next[proposalId];
        return next;
      });
    },
    [],
  );

  const reviewProposal = useCallback((proposalId: string) => {
    setSession((current) => withProposalReviewed(current, proposalId));
  }, []);

  /**
   * Records the chosen proposals one by one. Returns false (and leaves the
   * failure on screen) as soon as one command fails; nothing is shown as
   * recorded without its matching decision receipt.
   */
  async function recordChoices(
    target: NonNullable<typeof view.run>,
    choices: Record<string, ProposalChoice>,
  ): Promise<boolean> {
    const chosen = target.proposals.filter(
      (row) => row.selectable && choices[row.proposalId],
    );
    for (const row of chosen) {
      const decision = choices[row.proposalId]!;
      const expectedRevision = target.expectedRevision;
      const ids = identity(
        decisionAttemptContent(target.runId, row, decision, expectedRevision),
      );
      try {
        const receipt = await api.decide(target.runId, {
          proposalId: row.proposalId,
          presentedFingerprint: row.contentFingerprint,
          expectedRevision,
          decision,
          ...ids,
        });
        if (
          receipt.runId !== target.runId ||
          receipt.proposalId !== row.proposalId ||
          receipt.contentFingerprint !== row.contentFingerprint ||
          receipt.decision !== decision ||
          receipt.expectedRevision !== expectedRevision ||
          receipt.idempotencyKey !== ids.idempotencyKey
        )
          throw new ReceiptMismatchError();
        setSession((current) => withDecisionReceipt(current, receipt));
        select(row.proposalId, null);
      } catch (commandError) {
        setSession((current) =>
          withCommandError(current, target.runId, commandError, row.proposalId),
        );
        if (
          commandError instanceof ApiRequestError &&
          commandError.status === 409
        )
          select(row.proposalId, null);
        setError(
          commandMessage(commandError, "保存你的选择失败，请稍后重试。"),
        );
        return false;
      }
    }
    return true;
  }

  async function settleRun(
    target: NonNullable<typeof view.run>,
    closeRemaining: boolean,
  ) {
    const expectedRevision = target.expectedRevision;
    const ids = identity(
      settlementAttemptContent(target.runId, expectedRevision, closeRemaining),
    );
    try {
      const receipt = await api.settle(target.runId, {
        expectedRevision,
        closeRemaining,
        ...ids,
      });
      const consistent =
        receipt.runId === target.runId &&
        receipt.expectedRevision === expectedRevision &&
        receipt.idempotencyKey === ids.idempotencyKey &&
        (receipt.outcome === "committed"
          ? !!receipt.diagnosisRevisionId &&
            receipt.revision === expectedRevision + 1
          : !receipt.diagnosisRevisionId &&
            receipt.revision === expectedRevision);
      if (!consistent) throw new ReceiptMismatchError();
      setSession((current) => withSettlementReceipt(current, receipt));
      setSelection({});
    } catch (commandError) {
      setSession((current) =>
        withCommandError(current, target.runId, commandError),
      );
      setError(commandMessage(commandError, "写入诊断失败，请稍后重试。"));
    }
  }

  /** Records only the proposals the user chose; others stay undecided. */
  async function recordSelection() {
    const target = view.run;
    if (!target?.canDecide || busy) return;
    if (
      !target.proposals.some(
        (row) => row.selectable && selection[row.proposalId],
      )
    )
      return;
    setBusy(true);
    setError(null);
    try {
      await recordChoices(target, selection);
    } finally {
      setBusy(false);
      await refresh();
    }
  }

  /** Whole-run settlement; committed only after a matching receipt. */
  async function settle() {
    const target = view.run;
    if (!target?.canSettle || busy) return;
    setBusy(true);
    setError(null);
    try {
      await settleRun(target, target.closesRemaining);
    } finally {
      setBusy(false);
      await refresh();
    }
  }

  /**
   * Penpot "采纳并提交" / "不更改" / early submit: records every local choice
   * (plus the final one) and then settles the run. Proposals left without a
   * choice are closed unadopted (closeRemaining); the settlement receipt
   * alone decides what is shown as committed.
   */
  async function submit(final?: {
    proposalId: string;
    choice: ProposalChoice;
  }) {
    const target = view.run;
    if (!target?.canDecide || busy) return;
    const choices = final
      ? { ...selection, [final.proposalId]: final.choice }
      : selection;
    const selectable = target.proposals.filter((row) => row.selectable);
    const chosen = selectable.filter((row) => choices[row.proposalId]).length;
    setBusy(true);
    setError(null);
    try {
      if (!(await recordChoices(target, choices))) return;
      const blocked = target.proposals.some((row) => row.needsReview);
      if (blocked) return;
      await settleRun(target, selectable.length > chosen);
    } finally {
      setBusy(false);
      await refresh();
    }
  }

  /**
   * §3.6 "暂不处理" on the review prompt the user is looking at. Only a
   * matching server receipt hides it; the Condition itself never changes.
   */
  async function dismissReview(conditionId: string) {
    const condition = view.confirmed.find(
      (item) => item.conditionId === conditionId,
    );
    const review = condition?.review;
    const expectedRevision = view.currentRevision;
    if (!review?.canDismiss || expectedRevision === null || busy) return;
    const evaluatedBodyStateRevision = review.evaluatedBodyStateRevision;
    const { idempotencyKey } = identity(
      JSON.stringify(["dismiss-review", conditionId, expectedRevision, evaluatedBodyStateRevision]),
    );
    setBusy(true);
    setError(null);
    try {
      const receipt = await api.dismissReviewPrompt(conditionId, {
        expectedRevision,
        evaluatedBodyStateRevision,
        idempotencyKey,
      });
      if (
        receipt.conditionId !== conditionId ||
        receipt.diagnosisRevision !== expectedRevision ||
        receipt.dismissedThroughBodyStateRevision !== evaluatedBodyStateRevision ||
        receipt.idempotencyKey !== idempotencyKey
      )
        throw new ReceiptMismatchError();
    } catch (commandError) {
      setError(
        commandError instanceof ApiRequestError && commandError.status === 409
          ? "复核提示已经变化，已重新读取。"
          : errorMessage(commandError, "暂时无法收起复核提示，请稍后重试。"),
      );
    } finally {
      setBusy(false);
      await refresh();
    }
  }

  const openHistory = useCallback(() => setScene("history"), []);
  const closeHistory = useCallback(() => setScene("current"), []);

  /** Receipt / No changes "查看当前": leave the outcome and show the Diagnosis. */
  const dismissOutcome = useCallback(() => {
    setSession(EMPTY_DECISION_SESSION);
    setSelection({});
    setError(null);
  }, []);

  const loadError = diagnosis.error ?? latest.error ?? run.error;
  return {
    view,
    selection,
    select,
    reviewProposal,
    recordSelection,
    settle,
    submit,
    dismissOutcome,
    dismissReview,
    scene,
    openHistory,
    closeHistory,
    busy,
    error,
    loading: diagnosis.isPending && enabled,
    loadError: loadError
      ? errorMessage(loadError, "当前诊断暂时无法同步。")
      : null,
    refresh,
  };
}

export type DiagnosisActions = ReturnType<typeof useDiagnosisActions>;
