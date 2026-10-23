import { useEffect, useId, useLayoutEffect, useRef } from "react";
import { Link2, MoreHorizontal, Pencil, RefreshCw, Undo2 } from "lucide-react";
import { useAuthStore } from "@/stores/authStore";
import { useProfileStore } from "@/stores/profileStore";
import {
  effectiveTimeZone,
  formatInstantInZone,
} from "@/features/profile/model/profilePreferences";
import { useBodyHypothesisController } from "../hooks/useBodyHypothesisController";
import { Checkbox } from "@/components/ui/checkbox";
import { Textarea } from "@/components/ui/textarea";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { PersonConfirmDialog } from "../windows/person/PersonConfirmDialog";
import { IconTip } from "@/components/ui/tooltip";

/** Owner changes discard drafts and prevent late receipts from entering another profile. */
export function BodyHypotheses({
  canEdit,
  addRequested = false,
  onAddHandled,
}: {
  canEdit: boolean;
  /** The Person window header "+" asks for a new hypothesis draft. */
  addRequested?: boolean;
  onAddHandled?: () => void;
}) {
  const owner = useAuthStore((state) => state.user?.id);
  return owner ? (
    <HypothesisPanel
      key={owner}
      owner={owner}
      canEdit={canEdit}
      addRequested={addRequested}
      onAddHandled={onAddHandled}
    />
  ) : null;
}

function HypothesisPanel({
  owner,
  canEdit,
  addRequested,
  onAddHandled,
}: {
  owner: string;
  canEdit: boolean;
  addRequested: boolean;
  onAddHandled?: () => void;
}) {
  const {
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
  } = useBodyHypothesisController(owner, canEdit);
  const id = useId();
  const zone = effectiveTimeZone(
    useProfileStore((state) => state.profile?.preferences),
  ).zone;
  const root = useRef<HTMLElement>(null);
  const editor = useRef<HTMLFormElement>(null);
  const returnFocus = useRef<string | null>(null);
  const returnElement = useRef<HTMLElement | null>(null);
  const hadDraft = useRef(false);
  useLayoutEffect(() => {
    if (draft && !hadDraft.current) {
      editor.current
        ?.querySelector<HTMLElement>("textarea, button[type=submit]")
        ?.focus();
    } else if (!draft && hadDraft.current) {
      const feedback =
        error || notice
          ? root.current?.querySelector<HTMLElement>("[data-feedback]")
          : null;
      const trigger = returnFocus.current
        ? document.getElementById(returnFocus.current)
        : returnElement.current?.isConnected
          ? returnElement.current
          : null;
      (feedback ?? trigger ?? root.current)?.focus();
    }
    hadDraft.current = Boolean(draft);
  }, [draft, error, notice]);
  useEffect(() => {
    if (saving) editor.current?.focus();
  }, [saving]);
  // Header "+": open a draft when writes are allowed, otherwise drop it.
  useEffect(() => {
    if (!addRequested) return;
    onAddHandled?.();
    if (!canEdit || draft || !ready) return;
    returnFocus.current = null;
    returnElement.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    open({ mode: "record" });
  }, [addRequested, ready, canEdit, draft, open, onAddHandled]);
  function begin(next: Parameters<typeof open>[0], triggerId: string) {
    returnFocus.current = triggerId;
    open(next);
  }
  const toggle = (ids: string[], targetId: string) =>
    ids.includes(targetId)
      ? ids.filter((value) => value !== targetId)
      : [...ids, targetId];
  return (
    <section
      ref={root}
      tabIndex={-1}
      aria-label="我的猜测与相关记录"
      className="bc-hypotheses"
    >
      {notice && (
        <p role="status" tabIndex={-1} data-feedback>
          {notice}
        </p>
      )}
      {error && (
        <p role="alert" tabIndex={-1} data-feedback className="bc-error">
          {error}
        </p>
      )}
      {records.isPending || groups.isPending ? (
        <p role="status">正在读取猜测与分组…</p>
      ) : null}
      {records.isError || groups.isError ? (
        <p role="alert" className="bc-error">
          猜测或相关记录读取失败，请重试。
        </p>
      ) : null}
      {data && groups.data && !consistent && (
        <p role="status">记录已更新，正在等待一致的最新记录。请重新读取。</p>
      )}
      {records.isError ||
      groups.isError ||
      (data && groups.data && !consistent) ? (
        <button
          type="button"
          className="bc-text-button"
          aria-label="重新读取猜测与分组"
          disabled={saving || records.isFetching || groups.isFetching}
          onClick={refresh}
        >
          <RefreshCw size={14} />
          重新读取
        </button>
      ) : null}
      {readable && data ? (
        <>
          {!data.hypotheses.length && (
            <p className="bc-window-empty-text">还没有你提出的猜测。</p>
          )}
          {data.hypotheses.map((record) => (
            <article key={record.id} className="bc-person-hypothesis">
              <div className="bc-person-hypothesis-head">
                <p className="bc-record-statement">{record.statement}</p>
                {canEdit && !draft && (
                  <DropdownMenu>
                    <IconTip label="更多">
                      <DropdownMenuTrigger
                        id={`${id}-menu-${record.id}`}
                        className="bc-icon-button bc-record-menu"
                        aria-label={`更多操作：${record.statement}`}
                        disabled={!ready}
                      >
                        <MoreHorizontal size={17} aria-hidden="true" />
                      </DropdownMenuTrigger>
                    </IconTip>
                    <DropdownMenuContent
                      align="end"
                      className="bc-account-menu"
                    >
                      <DropdownMenuItem
                        onClick={() =>
                          begin(
                            { mode: "correct", target: record },
                            `${id}-menu-${record.id}`,
                          )
                        }
                      >
                        <Pencil />
                        更正
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        variant="destructive"
                        onClick={() =>
                          begin(
                            { mode: "withdraw", target: record },
                            `${id}-menu-${record.id}`,
                          )
                        }
                      >
                        <Undo2 />
                        撤回
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
              <div className="bc-person-chips">
                <span className="bc-person-chip" title="你本人提出，不代表诊断">
                  本人猜测
                </span>
                {record.supersedes_hypothesis_id ? (
                  <span className="bc-person-chip">更正后的版本</span>
                ) : null}
                {record.related_fact_ids.length +
                  record.related_observation_ids.length >
                0 ? (
                  <span className="bc-person-chip">
                    关联{" "}
                    {record.related_fact_ids.length +
                      record.related_observation_ids.length}{" "}
                    条记录
                  </span>
                ) : null}
              </div>
              <p className="bc-person-row-meta">
                记录{" "}
                {formatInstantInZone(record.recorded_at, zone) ?? "时间未知"}
              </p>
            </article>
          ))}
        </>
      ) : null}
      {draft?.mode === "withdraw" && (
        <PersonConfirmDialog
          open
          onOpenChange={(next) => {
            if (!next) cancel();
          }}
          title="撤回这条猜测？"
          description="关联的记录不受影响。"
          confirmLabel="撤回"
          working={saving}
          error={error}
          onConfirm={() => void submit()}
        />
      )}
      {draft && draft.mode !== "withdraw" && (
        <form
          ref={editor}
          tabIndex={-1}
          className="bc-equipment-form bc-hypothesis-form"
          aria-label="编辑我的猜测"
          aria-describedby={`${id}-save-state`}
          aria-busy={saving}
          onKeyDown={(event) => {
            if (event.key !== "Escape") return;
            event.preventDefault();
            event.stopPropagation();
            if (!saving) cancel();
          }}
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <p
            id={`${id}-save-state`}
            role={saving ? "status" : undefined}
            className="bc-window-note"
          >
            {saving ? "正在提交，等待保存回执。" : "尚未保存。"}
          </p>
          <label htmlFor={`${id}-statement`}>我的完整猜测</label>
          <Textarea
            id={`${id}-statement`}
            value={statement}
            maxLength={2000}
            rows={3}
            disabled={saving}
            required
            onChange={(event) => setStatement(event.target.value)}
          />
          <p className="bc-window-note">每类最多选 20 条。</p>
          <fieldset disabled={saving || !ready}>
            <legend>相关事实</legend>
            {data?.facts.map((fact) => (
              <label key={fact.id} className="bc-hypothesis-choice">
                <Checkbox
                  checked={facts.includes(fact.id)}
                  disabled={
                    saving ||
                    !ready ||
                    (!facts.includes(fact.id) && facts.length >= 20)
                  }
                  onCheckedChange={() => setFacts(toggle(facts, fact.id))}
                />
                {fact.statement}
              </label>
            ))}
          </fieldset>
          <fieldset disabled={saving || !ready}>
            <legend>相关观察</legend>
            {data?.observations.map((observation) => (
              <label key={observation.id} className="bc-hypothesis-choice">
                <Checkbox
                  checked={observations.includes(observation.id)}
                  disabled={
                    saving ||
                    !ready ||
                    (!observations.includes(observation.id) &&
                      observations.length >= 20)
                  }
                  onCheckedChange={() =>
                    setObservations(toggle(observations, observation.id))
                  }
                />
                {observation.statement}
              </label>
            ))}
          </fieldset>
          <div className="bc-record-actions">
            <button
              className="bc-button"
              type="submit"
              disabled={saving || !ready || !canEdit || !statement.trim()}
            >
              {saving
                ? "正在提交"
                : draft.mode === "record"
                  ? "保存猜测"
                  : "保存更正"}
            </button>
            <button
              className="bc-button"
              type="button"
              disabled={saving}
              onClick={cancel}
            >
              取消
            </button>
          </div>
        </form>
      )}
      <h4 className="bc-person-section-label">相关分组</h4>
      {readable && data && groups.data ? (
        groups.data.groups.length ? (
          groups.data.groups.map((group) => (
            <article
              key={group.id}
              className="bc-person-group"
              title="由你的记录自动整理，不形成新的事实或诊断"
            >
              <h5 className="bc-person-group-title">
                <Link2 size={13} aria-hidden="true" />
                {group.display_title}
              </h5>
              {group.related_hypothesis_ids.map((hypothesisId) => (
                <p key={hypothesisId}>
                  用户猜测：
                  {data.hypotheses.find((row) => row.id === hypothesisId)
                    ?.statement ?? "当前记录不可用"}
                </p>
              ))}
              {group.related_fact_ids.map((factId) => (
                <p key={factId}>
                  来源事实：
                  {data.facts.find((row) => row.id === factId)?.statement ??
                    "当前记录不可用"}
                </p>
              ))}
              {group.related_observation_ids.map((observationId) => {
                const observation = data.observations.find(
                  (row) => row.id === observationId,
                );
                return (
                  <p key={observationId}>
                    来源观察：
                    {observation
                      ? [
                          observation.statement,
                          observation.raw_value,
                          observation.raw_unit,
                        ]
                          .filter(Boolean)
                          .join(" · ")
                      : "当前记录不可用"}
                  </p>
                );
              })}
              {group.related_episode_ids.map((episodeId) => (
                <p key={episodeId}>
                  相关经历：
                  {data.episodes.find((row) => row.id === episodeId)
                    ?.symptom_type ?? "当前记录不可用"}
                </p>
              ))}
            </article>
          ))
        ) : (
          <p className="bc-window-empty-text">暂无相关记录分组。</p>
        )
      ) : null}
    </section>
  );
}
