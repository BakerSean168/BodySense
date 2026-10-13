import { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { Check, LoaderCircle, Plus, RefreshCw } from "lucide-react";
import {
  bodyFactRecordsService,
  type BodyFactRecordInput,
} from "@/features/profile/services/bodyFactRecordsService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { useAuthStore } from "@/stores/authStore";
import { BodyFactAmendment } from "./BodyFactAmendment";

import {
  BODY_FACT_KIND_LABELS as FACT_KINDS,
  type BodyFactKind as FactKind,
} from "../model/bodyFactPresentation";
import { projectBodyFactSurface } from "../model/bodyFactViewModel";

export const bodyFactRecordsKey = (owner: string | undefined) =>
  ["workspace", "body-fact-records", owner ?? "signed-out"] as const;

function recordedTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "记录时间未知" : date.toLocaleString();
}

/** One live, user-scoped ledger surface; Habits is a filtered view, not a new owner. */
export function BodyFactRecords({
  habitsOnly = false,
  canEdit,
}: {
  habitsOnly?: boolean;
  canEdit: boolean;
}) {
  const owner = useAuthStore((state) => state.user?.id);
  const ownerRef = useRef(owner);
  ownerRef.current = owner;
  const queryClient = useQueryClient();
  const query = useInfiniteQuery({
    queryKey: bodyFactRecordsKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) => {
      const page = await bodyFactRecordsService.list(pageParam);
      if (page.user_id !== owner)
        throw new Error("档案身份已变化，请重新读取。");
      if (
        page.has_more &&
        (!page.next_before_revision ||
          (pageParam !== undefined && page.next_before_revision >= pageParam))
      ) {
        throw new Error("记录分页信息无效，请重新读取。");
      }
      return page;
    },
    getNextPageParam: (page) =>
      page.has_more ? page.next_before_revision : undefined,
    enabled: !!owner,
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
  const [adding, setAdding] = useState(false);
  const [kind, setKind] = useState<FactKind>(
    habitsOnly ? "lifestyle" : "symptom",
  );
  const [statement, setStatement] = useState("");
  const [when, setWhen] = useState("");
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const pending = useRef<{
    owner: string;
    content: string;
    command: BodyFactRecordInput;
  } | null>(null);

  useEffect(() => {
    pending.current = null;
    setStatement("");
    setWhen("");
    setAdding(false);
    setNotice(null);
    setSaveError(null);
    setSaving(false);
  }, [owner]);

  const page = query.data?.pages[0];
  const view = projectBodyFactSurface({
    pages: query.data?.pages,
    surface: habitsOnly ? "habits" : "records",
    hasOwner: Boolean(owner),
    isPending: query.isPending,
    isError: query.isError,
  });

  async function submit() {
    if (
      !owner ||
      !canEdit ||
      !page ||
      view.pagesDisagree ||
      saving ||
      !statement.trim()
    )
      return;
    const submittingOwner = owner;
    const effectiveKind = habitsOnly ? "lifestyle" : kind;
    const content = JSON.stringify([
      effectiveKind,
      statement.trim(),
      when.trim(),
    ]);
    if (
      pending.current?.owner !== owner ||
      pending.current.content !== content
    ) {
      pending.current = {
        owner,
        content,
        command: {
          idempotency_key: crypto.randomUUID(),
          expected_revision: page.current_revision,
          facts: [
            {
              kind: effectiveKind,
              statement: statement.trim(),
              body_region_ids: [],
              temporal: {
                precision: "unknown",
                ...(when.trim() ? { original_expression: when.trim() } : {}),
              },
              details: {},
            },
          ],
        },
      };
    }
    const command = pending.current.command;
    setSaving(true);
    setSaveError(null);
    setNotice(null);
    try {
      const receipt = await bodyFactRecordsService.record(command);
      if (ownerRef.current !== submittingOwner) return;
      if (receipt.user_id !== submittingOwner)
        throw new Error("保存回执的档案身份不一致。");
      // The write is committed before refetch. A failed read must never turn it
      // into another write or claim that persistence failed.
      pending.current = null;
      setStatement("");
      setWhen("");
      setAdding(false);
      setNotice("已保存你的陈述。");
      void queryClient.invalidateQueries({ queryKey: ["workspace"] });
    } catch (cause) {
      if (ownerRef.current !== submittingOwner) return;
      if (cause instanceof ApiRequestError && cause.status === 409) {
        pending.current = null;
        setSaveError("记录已发生变化。已重新读取，请核对内容后再次保存。");
        void query.refetch();
      } else {
        setSaveError(
          errorMessage(
            cause,
            "尚未确认保存结果。重试会使用同一条请求，不会重复创建。",
          ),
        );
      }
    } finally {
      if (ownerRef.current === submittingOwner) setSaving(false);
    }
  }

  if (!owner) return <p className="bc-window-note">请登录后查看个人记录。</p>;

  return (
    <div className="bc-person-category" data-testid="body-fact-records">
      <div className="bc-record-actions">
        <span className="bc-window-note">
          {habitsOnly ? "本人陈述的习惯与变化" : "本人陈述，不代表临床诊断"}
        </span>
        <button
          type="button"
          className="bc-button"
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
          aria-label="刷新身体记录"
        >
          <RefreshCw
            size={15}
            className={query.isFetching ? "animate-spin" : ""}
          />
        </button>
        {canEdit && !adding ? (
          <button
            type="button"
            className="bc-button"
            onClick={() => {
              setAdding(true);
              setNotice(null);
            }}
            disabled={!page || view.pagesDisagree}
          >
            <Plus size={15} />
            补充记录
          </button>
        ) : null}
      </div>
      {query.isPending ? (
        <p role="status" className="bc-person-loading">
          <LoaderCircle size={16} className="animate-spin" />
          正在读取身体记录
        </p>
      ) : null}
      {query.isError ? (
        <p role="alert" className="bc-error">
          {notice
            ? "记录已保存，但列表刷新失败。"
            : page
              ? "部分记录刷新失败，仍显示上次读取的内容。"
              : "身体记录暂时无法读取。"}
          <button
            type="button"
            className="bc-text-button"
            onClick={() => void query.refetch()}
          >
            重新读取
          </button>
        </p>
      ) : null}
      {view.pagesDisagree ? (
        <p role="alert" className="bc-error">
          翻页期间身体记录发生了变化，已暂时隐藏不同版本的历史页面。请刷新后继续查看或修改。
          <button
            type="button"
            className="bc-text-button"
            onClick={() => void query.refetch()}
          >
            重新读取
          </button>
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="bc-record-saved">
          <Check size={15} />
          {notice}
        </p>
      ) : null}
      {!query.isError &&
      (view.state === "empty" || view.state === "partial_empty") ? (
        <p className="bc-window-empty-text">
          {view.state === "partial_empty"
            ? habitsOnly
              ? "当前加载的记录中没有生活习惯，可以继续读取更早的记录。"
              : "当前加载的记录为空，可以继续读取更早的记录。"
            : habitsOnly
              ? "尚未记录生活习惯。可以在对话中描述，或直接补充记录。"
              : "还没有身体记录。可以在对话中描述，或直接补充记录。"}
        </p>
      ) : null}
      {adding ? (
        <form
          className="bc-equipment-form"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          {!habitsOnly ? (
            <label>
              记录类别
              <select
                value={kind}
                onChange={(event) => {
                  setKind(event.target.value as FactKind);
                  pending.current = null;
                }}
                disabled={saving}
              >
                {Object.entries(FACT_KINDS).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          <label>
            你的描述
            <textarea
              value={statement}
              onChange={(event) => setStatement(event.target.value)}
              maxLength={2000}
              rows={3}
              disabled={saving}
              required
            />
          </label>
          <label>
            发生时间<span className="bc-optional">可选，保留你的原话</span>
            <input
              value={when}
              onChange={(event) => setWhen(event.target.value)}
              maxLength={500}
              placeholder="例如：昨天晚上；不清楚可以留空"
              disabled={saving}
            />
          </label>
          <div className="bc-record-actions">
            <button
              type="submit"
              className="bc-button bc-primary"
              disabled={saving || !statement.trim()}
            >
              {saving ? (
                <LoaderCircle size={16} className="animate-spin" />
              ) : null}
              {saving ? "正在保存" : "保存记录"}
            </button>
            <button
              type="button"
              className="bc-button"
              disabled={saving}
              onClick={() => {
                setAdding(false);
                setSaveError(null);
              }}
            >
              收起
            </button>
          </div>
          {saveError ? (
            <p role="alert" className="bc-error">
              {saveError}
            </p>
          ) : null}
        </form>
      ) : null}
      <ul className="bc-record-list">
        {view.records.map(
          ({
            id,
            record,
            label,
            statement: statementText,
            occurrence,
            recordedAt,
            isCorrection,
          }) => (
            <li key={id} className="bc-record-item">
              <span className="bc-status">{label}</span>
              <p className="bc-record-statement">{statementText}</p>
              <p className="bc-window-note">发生时间：{occurrence}</p>
              <time className="bc-window-note" dateTime={recordedAt}>
                记录于 {recordedTime(recordedAt)}
              </time>
              {isCorrection ? (
                <span className="bc-window-note">更正后的陈述</span>
              ) : null}
              {canEdit && page && !view.pagesDisagree ? (
                <BodyFactAmendment
                  record={record}
                  revision={page.current_revision}
                  onRefresh={() => void query.refetch()}
                  onCommitted={(message) => {
                    if (ownerRef.current !== record.user_id) return;
                    setNotice(message);
                    void queryClient.invalidateQueries({
                      queryKey: ["workspace"],
                    });
                  }}
                />
              ) : null}
            </li>
          ),
        )}
      </ul>
      {query.hasNextPage ? (
        <button
          type="button"
          className="bc-button"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {query.isFetchingNextPage ? "正在读取" : "读取更早记录"}
        </button>
      ) : null}
    </div>
  );
}
