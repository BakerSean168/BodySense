import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useBodyRecordsQuery } from "@/features/profile/hooks/useBodyRecordsQuery";
import {
  bodyHypothesisService as api,
  type HypothesisInput,
  type HypothesisCorrection,
  type HypothesisWithdrawal,
} from "@/features/profile/services/bodyHypothesisService";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { useAuthStore } from "@/stores/authStore";

type Hypothesis = BodyRecords["hypotheses"][number];
type Draft = { mode: "record" | "correct" | "withdraw"; target?: Hypothesis };
type Attempt =
  | { mode: "record"; content: string; command: HypothesisInput }
  | {
      mode: "correct";
      content: string;
      target: string;
      command: HypothesisCorrection;
    }
  | {
      mode: "withdraw";
      content: string;
      target: string;
      command: HypothesisWithdrawal;
    };

export function useBodyHypothesisController(owner: string, canEdit: boolean) {
  const records = useBodyRecordsQuery();
  const client = useQueryClient();
  const groups = useQuery({
    queryKey: ["workspace", "concern-groups", owner],
    queryFn: async () => {
      const page = await api.groups();
      if (page.user_id !== owner)
        throw new Error("档案身份不一致，请重新读取。");
      return page;
    },
    retry: false,
    staleTime: 0,
  });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [statement, setStatement] = useState("");
  const [facts, setFacts] = useState<string[]>([]);
  const [observations, setObservations] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [committedRevision, setCommittedRevision] = useState(0);
  const pending = useRef<Attempt | null>(null);
  const active = useRef(true);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  const data = records.data;
  const consistent =
    !!data &&
    data.user_id === owner &&
    !!groups.data &&
    data.current_revision === groups.data.current_revision &&
    data.current_revision >= committedRevision;
  const readable = consistent && !records.isError && !groups.isError;
  const ready = readable && !records.isFetching && !groups.isFetching;
  function refresh() {
    void records.refetch();
    void groups.refetch();
  }
  function open(next: Draft) {
    pending.current = null;
    setDraft(next);
    setStatement(next.target?.statement ?? "");
    setFacts(next.target?.related_fact_ids ?? []);
    setObservations(next.target?.related_observation_ids ?? []);
    setError(null);
    setNotice(null);
  }
  async function submit() {
    if (!draft || !ready || !data || !canEdit || saving || !statement.trim())
      return;
    const content = JSON.stringify([
      draft.mode,
      draft.target?.id,
      statement.trim(),
      facts,
      observations,
    ]);
    if (pending.current?.content !== content) {
      const identity = {
        idempotency_key: crypto.randomUUID(),
        expected_revision: data.current_revision,
      };
      const replacement = {
        statement: statement.trim(),
        related_fact_ids: facts,
        related_observation_ids: observations,
      };
      if (draft.mode === "record")
        pending.current = {
          mode: "record",
          content,
          command: { ...identity, ...replacement },
        };
      else if (draft.target)
        pending.current =
          draft.mode === "correct"
            ? {
                mode: "correct",
                content,
                target: draft.target.id,
                command: { ...identity, replacement },
              }
            : {
                mode: "withdraw",
                content,
                target: draft.target.id,
                command: identity,
              };
    }
    const attempt = pending.current;
    if (!attempt) return;
    setSaving(true);
    setError(null);
    try {
      const receipt =
        attempt.mode === "record"
          ? await api.record(attempt.command)
          : attempt.mode === "correct"
            ? await api.correct(attempt.target, attempt.command)
            : await api.withdraw(attempt.target, attempt.command);
      if (!active.current || useAuthStore.getState().user?.id !== owner) return;
      if (
        receipt.user_id !== owner ||
        receipt.operation !== attempt.mode ||
        receipt.revision <= attempt.command.expected_revision ||
        (attempt.mode !== "record" &&
          receipt.target_hypothesis_id !== attempt.target) ||
        (attempt.mode === "withdraw" &&
          receipt.hypothesis_id !== attempt.target) ||
        (attempt.mode === "correct" && receipt.hypothesis_id === attempt.target)
      )
        throw new Error("尚未收到匹配的提交回执，请重试核实。");
      pending.current = null;
      setDraft(null);
      setCommittedRevision(receipt.revision);
      setNotice(
        attempt.mode === "record"
          ? "你的猜测已保存。"
          : attempt.mode === "correct"
            ? "猜测更正已保存，原版本保留在历史中。"
            : "猜测已撤回，原事实和观察仍保留。历史保留不表示隐私数据已删除。",
      );
      void client.invalidateQueries({ queryKey: ["workspace"] });
    } catch (cause) {
      if (!active.current || useAuthStore.getState().user?.id !== owner) return;
      if (
        cause instanceof ApiRequestError &&
        (cause.status === 409 || cause.status === 404)
      ) {
        pending.current = null;
        // Require a new explicit edit after refresh; never silently retarget a correction.
        setDraft(null);
        setError("记录已变化或不可用。请重新读取并核对后操作。");
        refresh();
      } else
        setError(
          errorMessage(
            cause,
            "尚未确认提交结果，请重试；相同内容会使用同一请求。",
          ),
        );
    } finally {
      if (active.current) setSaving(false);
    }
  }
  function cancel() {
    setDraft(null);
    setError(null);
    pending.current = null;
  }
  return {
    records,
    groups,
    data,
    consistent,
    readable,
    ready,
    draft,
    statement,
    setStatement,
    facts,
    setFacts,
    observations,
    setObservations,
    saving,
    error,
    notice,
    refresh,
    open,
    submit,
    cancel,
  };
}
