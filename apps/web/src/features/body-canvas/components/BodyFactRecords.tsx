import {
  lazy,
  Suspense,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  Check,
  FileText,
  LoaderCircle,
  Plus,
  RefreshCw,
  Repeat2,
  Trash2,
  X,
} from "lucide-react";
import {
  bodyFactRecordsService,
  type BodyFactRecordInput,
} from "@/features/profile/services/bodyFactRecordsService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { useAuthStore } from "@/stores/authStore";
import { useProfileStore } from "@/stores/profileStore";
import {
  effectiveTimeZone,
  formatInstantInZone,
} from "@/features/profile/model/profilePreferences";
import { BodyFactRecordForm } from "./BodyFactRecordForm";
import { BodyFactRecordList } from "./BodyFactRecordList";
import { BodyFactAmendment } from "./BodyFactAmendment";
import type { StatementErasureTarget } from "./StatementErasure";

// Rarely opened; kept out of the ConsultationPage chunk budget.
const StatementErasure = lazy(() =>
  import("./StatementErasure").then((module) => ({
    default: module.StatementErasure,
  })),
);

import { type BodyFactKind as FactKind } from "../model/bodyFactPresentation";
import { projectBodyFactSurface } from "../model/bodyFactViewModel";
import { projectBoundedBodyFactVM } from "../model/bodyFactV2Adapter";
import { recordedClock } from "../model/personRecordsTimeline";

export const bodyFactRecordsKey = (owner: string | undefined) =>
  ["workspace", "body-fact-records", owner ?? "signed-out"] as const;

/** Recorded instant on the preferred display clock; the instant is unchanged. */
function recordedTime(value: string, zone: string): string {
  return formatInstantInZone(value, zone) ?? "记录时间未知";
}

/** Episode chips a Records detail can link to; supplied by the Person window. */
export interface BodyFactRelatedLink {
  id: string;
  label: string;
}

/** One live, user-scoped ledger surface; Habits is a filtered view, not a new owner. */
export function BodyFactRecords({
  habitsOnly = false,
  canEdit,
  related,
  relatedLinks,
  onOpenRelated,
  now,
}: {
  habitsOnly?: boolean;
  canEdit: boolean;
  /** Penpot Body / Region "相关" chips, rendered under the loaded count. */
  related?: ReactNode;
  /** Organising links (Episodes) of one fact, for its detail view. */
  relatedLinks?: (factId: string) => readonly BodyFactRelatedLink[];
  onOpenRelated?: (link: BodyFactRelatedLink) => void;
  /** Clock for "今天 / 昨天" day labels; tests pin it. */
  now?: Date;
}) {
  const owner = useAuthStore((state) => state.user?.id);
  const displayZone = effectiveTimeZone(
    useProfileStore((state) => state.profile?.preferences),
  ).zone;
  const ownerRef = useRef(owner);
  // Commit identity before asynchronous receipts can update this surface.
  useLayoutEffect(() => {
    ownerRef.current = owner;
  }, [owner]);
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
  // A durable receipt can arrive before an invalidated ledger query refetches.
  // Do not call an older snapshot empty or use its revision for another write.
  const [savedRevision, setSavedRevision] = useState<number | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  // The erasure panel outlives the record: a withdrawn record leaves the list.
  const [erasing, setErasing] = useState<StatementErasureTarget | null>(null);
  // Penpot Records rows open one statement's detail inside the same window.
  const [selectedId, setSelectedId] = useState<string | null>(null);
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
    setSavedRevision(null);
    setSaveError(null);
    setSaving(false);
    setErasing(null);
    setSelectedId(null);
  }, [owner]);

  const page = query.data?.pages[0];
  const snapshotBehindCommit =
    savedRevision !== null && (page?.current_revision ?? -1) < savedRevision;
  const view = useMemo(
    () =>
      projectBodyFactSurface({
        pages: query.data?.pages,
        surface: habitsOnly ? "habits" : "records",
        hasOwner: Boolean(owner),
        isPending: query.isPending,
        isError: query.isError,
      }),
    [query.data?.pages, habitsOnly, owner, query.isPending, query.isError],
  );

  // A bounded transport projection is not a complete V2 health read-model.
  // Keep ledger parsing stable while the user types into the local draft form.
  const boundedVM = useMemo(
    () => projectBoundedBodyFactVM(view, habitsOnly ? "habits" : "records"),
    [view, habitsOnly],
  );

  async function submit() {
    if (
      !owner ||
      !canEdit ||
      !page ||
      view.pagesDisagree ||
      snapshotBehindCommit ||
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
      setSavedRevision(receipt.revision);
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

  const clock = now ?? new Date();
  const rows = snapshotBehindCommit ? [] : view.records;
  const selected = selectedId
    ? (rows.find((row) => row.id === selectedId) ?? null)
    : null;
  const readFailed = query.isError && !page;
  const stale = query.isError && Boolean(page);
  const lastSync = query.dataUpdatedAt
    ? recordedClock(new Date(query.dataUpdatedAt).toISOString(), displayZone)
    : null;
  const addDisabled = !page || view.pagesDisagree || snapshotBehindCommit;
  const openAdd = () => {
    setAdding(true);
    setNotice(null);
    setSelectedId(null);
  };
  const emptyState =
    !query.isError &&
    !query.isPending &&
    !snapshotBehindCommit &&
    !adding &&
    boundedVM.state === "empty";

  const addForm = adding ? (
    <BodyFactRecordForm
      habitsOnly={habitsOnly}
      kind={kind}
      statement={statement}
      when={when}
      saving={saving}
      saveError={saveError}
      onKindChange={(value) => {
        setKind(value);
        pending.current = null;
      }}
      onStatementChange={setStatement}
      onWhenChange={setWhen}
      onSubmit={() => void submit()}
      onClose={() => {
        setAdding(false);
        setSaveError(null);
      }}
    />
  ) : null;

  const erasure =
    erasing && canEdit ? (
      <Suspense
        fallback={
          <p role="status" className="bc-person-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在打开删除
          </p>
        }
      >
        <StatementErasure
          key={erasing.statementId}
          target={erasing}
          revision={page?.current_revision}
          onClose={() => setErasing(null)}
          onChanged={(message, committedRevision) => {
            if (ownerRef.current !== erasing.ownerId) return;
            if (committedRevision !== undefined)
              setSavedRevision((prev) =>
                Math.max(prev ?? 0, committedRevision),
              );
            setNotice(message);
            void queryClient.invalidateQueries({ queryKey: ["workspace"] });
          }}
        />
      </Suspense>
    ) : null;

  const feedback = (
    <>
      {snapshotBehindCommit && !query.isError ? (
        <p role="status" className="bc-person-loading">
          <LoaderCircle size={16} className="animate-spin" />
          正在同步已保存的记录，暂不显示旧列表
        </p>
      ) : null}
      {stale ? (
        <div role="alert" className="bc-person-stale">
          <span className="bc-person-stale-chip">
            {notice ? "记录已保存，但列表刷新失败。" : "可能不是最新"}
          </span>
          {lastSync ? (
            <span className="bc-person-stale-time">上次同步 {lastSync}</span>
          ) : null}
          <button
            type="button"
            className="bc-icon-button"
            aria-label="重新读取"
            title="重新读取"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw
              size={16}
              className={query.isFetching ? "animate-spin" : ""}
            />
          </button>
        </div>
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
    </>
  );

  // Penpot Records detail: one statement, its organising links and the only
  // places it can be amended. Leaving the list never discards the ledger read.
  if (selected && !erasing) {
    const links = relatedLinks?.(selected.id) ?? [];
    return (
      <div
        className="bc-person-category bc-person-detail"
        data-testid="body-fact-records"
        data-vm-contract={boundedVM.contract}
        data-vm-state={boundedVM.state}
      >
        <div className="bc-person-detail-body" data-testid="body-fact-detail">
          <h3 className="bc-person-detail-title">{selected.label}</h3>
          {selected.isCorrection || links.length ? (
            <div className="bc-person-chips">
              {selected.isCorrection ? (
                <span className="bc-person-chip">更正后的陈述</span>
              ) : null}
              {links.map((link) => (
                <button
                  key={link.id}
                  type="button"
                  className="bc-person-chip bc-person-chip-link"
                  onClick={() => onOpenRelated?.(link)}
                >
                  {link.label}
                </button>
              ))}
            </div>
          ) : null}
          {feedback}
          <p className="bc-person-quote">“{selected.statement}”</p>
          <p className="bc-person-row-meta">
            发生：{selected.occurrence} · 记录{" "}
            <time dateTime={selected.recordedAt}>
              {recordedTime(selected.recordedAt, displayZone)}
            </time>
          </p>
          {canEdit && page && !view.pagesDisagree ? (
            <div className="bc-person-detail-actions">
              <BodyFactAmendment
                record={selected.record}
                revision={page.current_revision}
                onRefresh={() => void query.refetch()}
                onCommitted={(message, committedRevision) => {
                  if (ownerRef.current !== selected.record.user_id) return;
                  // A correction/withdrawal also invalidates the old live
                  // ledger snapshot; never display a revoked assertion while
                  // its authoritative revision is still being fetched.
                  setSavedRevision((prev) =>
                    Math.max(prev ?? 0, committedRevision),
                  );
                  setNotice(message);
                  setSelectedId(null);
                  void queryClient.invalidateQueries({
                    queryKey: ["workspace"],
                  });
                }}
              />
              {selected.record.source_evidence_id ? (
                <button
                  type="button"
                  className="bc-text-button bc-person-danger"
                  onClick={() => {
                    setNotice(null);
                    setErasing({
                      recordId: selected.record.id,
                      statementId: selected.record.source_evidence_id,
                      ownerId: selected.record.user_id,
                    });
                  }}
                >
                  <Trash2 size={13} />
                  彻底删除
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
        <footer className="bc-person-footer">
          <button
            type="button"
            className="bc-person-footer-link"
            onClick={() => setSelectedId(null)}
          >
            {habitsOnly ? "返回生活习惯" : "返回记录"}
          </button>
        </footer>
      </div>
    );
  }

  return (
    <div
      className="bc-person-category bc-person-records"
      data-testid="body-fact-records"
      data-vm-contract={boundedVM.contract}
      data-vm-state={boundedVM.state}
    >
      {query.isPending ? (
        <div className="bc-person-skeleton" aria-hidden="true">
          {[0, 1, 2, 3, 4].map((index) => (
            <span key={index} />
          ))}
        </div>
      ) : null}
      {query.isPending ? (
        <p role="status" className="bc-person-loading bc-person-loading-pill">
          正在读取记录
        </p>
      ) : null}
      {readFailed ? (
        <div role="alert" className="bc-person-state" data-tone="danger">
          <span className="bc-person-state-icon" aria-hidden="true">
            <X size={18} />
          </span>
          <strong>记录没有读取出来</strong>
          <span>
            {notice
              ? "记录已保存，但列表刷新失败。"
              : "网络或服务暂时不可用，已有记录不会丢失"}
          </span>
          <button
            type="button"
            className="bc-icon-button"
            aria-label="重新读取"
            title="重新读取"
            disabled={query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw size={16} />
          </button>
        </div>
      ) : null}
      {emptyState ? (
        <div className="bc-person-state">
          <span className="bc-person-state-icon" aria-hidden="true">
            {habitsOnly ? <Repeat2 size={18} /> : <FileText size={18} />}
          </span>
          <strong>{habitsOnly ? "还没有生活习惯" : "还没有身体记录"}</strong>
          <span>
            {habitsOnly
              ? "作息、久坐、饮食这些，在对话里说过就会整理到这里"
              : "在对话里说一句身体感受，就会出现在这里"}
          </span>
          {canEdit ? (
            <button
              type="button"
              className="bc-person-add"
              aria-label="补充记录"
              title="补充记录"
              disabled={addDisabled}
              onClick={openAdd}
            >
              <Plus size={17} />
            </button>
          ) : null}
          {notice ? (
            <p role="status" className="bc-record-saved">
              <Check size={15} />
              {notice}
            </p>
          ) : null}
          {/* Episodes and the user's own hypotheses stay reachable with no
              statements yet (the Episode editor records its own symptom). */}
          {related ? (
            <div className="bc-person-state-related">{related}</div>
          ) : null}
        </div>
      ) : null}
      {!query.isPending && !readFailed && !emptyState ? (
        <>
          <div className="bc-person-listbar">
            <span className="bc-person-count">
              {habitsOnly
                ? `已加载 ${rows.length} 项`
                : `已加载 ${rows.length} 条`}
              {view.hasMore ? " · 还有更早记录" : ""}
            </span>
            {canEdit && !adding ? (
              <button
                type="button"
                className="bc-person-add"
                aria-label="补充记录"
                title="补充记录"
                disabled={addDisabled}
                onClick={openAdd}
              >
                <Plus size={17} />
              </button>
            ) : null}
          </div>
          {related}
          {feedback}
        </>
      ) : null}
      {!query.isError &&
      !snapshotBehindCommit &&
      boundedVM.state === "partial_empty" ? (
        <p className="bc-window-empty-text">
          {habitsOnly
            ? "当前加载的记录中没有生活习惯，可以继续读取更早的记录。"
            : "当前加载的记录为空，可以继续读取更早的记录。"}
        </p>
      ) : null}
      {addForm}
      {erasure}
      <BodyFactRecordList
        rows={rows}
        habitsOnly={habitsOnly}
        displayZone={displayZone}
        now={clock}
        onSelect={(id) => {
          setSelectedId(id);
          setNotice(null);
          setAdding(false);
        }}
      />
      {query.hasNextPage ? (
        <button
          type="button"
          className="bc-person-more"
          disabled={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          {query.isFetchingNextPage ? "正在读取" : "读取更早记录"}
        </button>
      ) : null}
    </div>
  );
}
