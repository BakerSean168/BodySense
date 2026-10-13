import { useEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { LoaderCircle, RefreshCw } from "lucide-react";
import {
  bodyFactRecordsService,
  type BodyFactRecordInput,
} from "@/features/profile/services/bodyFactRecordsService";
import {
  bodyEpisodeService,
  type BodyEpisodeAssociationInput,
  type BodyEpisodeCommandReceipt,
} from "@/features/profile/services/bodyEpisodeService";
import { ApiRequestError, errorMessage } from "@/lib/api-client";
import { useAuthStore } from "@/stores/authStore";
import { bodyFactRecordsKey } from "./BodyFactRecords";
import {
  episodeStateLabel,
  projectEpisodeAssociation,
  type EpisodeAssociationMode,
} from "../model/bodyEpisodeAssociations";
import {
  makeEpisodeSymptomRecordInput,
  type EpisodeSymptomDraft,
} from "../model/bodyEpisodeSymptomDraft";

const bodyEpisodeKey = (owner: string | undefined) =>
  ["workspace", "body-episodes", owner ?? "signed-out"] as const;

interface PendingCommand {
  owner: string;
  intent: string;
  input: BodyEpisodeAssociationInput;
}
interface PendingSymptomRecord {
  owner: string;
  intent: string;
  input: BodyFactRecordInput;
}
const initialSymptomDraft: EpisodeSymptomDraft = {
  statement: "",
  symptomType: "",
  presence: "present",
  laterality: "unknown",
};

/** Functional record-to-Episode bridge; no claim of medical diagnosis or onset. */
function BodyEpisodeEditor({ canEdit }: { canEdit: boolean }) {
  const owner = useAuthStore((state) => state.user?.id);
  const ownerRef = useRef(owner);
  ownerRef.current = owner;
  const queryClient = useQueryClient();
  const factsQuery = useInfiniteQuery({
    queryKey: bodyFactRecordsKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) => {
      const page = await bodyFactRecordsService.list(pageParam);
      if (page.user_id !== owner) throw new Error("身体记录的档案身份不一致。");
      if (
        page.has_more &&
        (!page.next_before_revision ||
          (pageParam !== undefined && page.next_before_revision >= pageParam))
      ) {
        throw new Error("身体记录分页信息无效，请刷新。");
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
  const episodesQuery = useInfiniteQuery({
    queryKey: bodyEpisodeKey(owner),
    initialPageParam: undefined as number | undefined,
    queryFn: async ({ pageParam }) => {
      const page = await bodyEpisodeService.list(pageParam);
      if (page.user_id !== owner) throw new Error("症状经历的档案身份不一致。");
      if (
        page.has_more &&
        (!page.next_before_revision ||
          (pageParam !== undefined && page.next_before_revision >= pageParam))
      ) {
        throw new Error("症状经历分页信息无效，请刷新。");
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

  const view = projectEpisodeAssociation(
    owner,
    factsQuery.data?.pages,
    episodesQuery.data?.pages,
  );
  const [mode, setMode] = useState<EpisodeAssociationMode>("create");
  const [factId, setFactId] = useState("");
  const [episodeId, setEpisodeId] = useState("");
  const [saving, setSaving] = useState(false);
  const [recording, setRecording] = useState(false);
  const [addingSymptom, setAddingSymptom] = useState(false);
  const [symptom, setSymptom] =
    useState<EpisodeSymptomDraft>(initialSymptomDraft);
  const [recordError, setRecordError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const pending = useRef<PendingCommand | null>(null);
  const pendingSymptom = useRef<PendingSymptomRecord | null>(null);

  useEffect(() => {
    pending.current = null;
    pendingSymptom.current = null;
    setMode("create");
    setFactId("");
    setEpisodeId("");
    setAddingSymptom(false);
    setSymptom(initialSymptomDraft);
    setRecordError(null);
    setRecording(false);
    setNotice(null);
    setSaveError(null);
    setSaving(false);
  }, [owner]);

  const sources = view.facts.filter(
    (fact) =>
      (mode === "link" || fact.presence === "present") &&
      !view.linkedFactIds.has(fact.id),
  );
  const currentEpisodes = view.episodes.filter(
    (episode) => episode.identity_status === "current",
  );
  const intent = JSON.stringify([
    mode,
    factId,
    mode === "link" ? episodeId : "",
  ]);
  const replay =
    !!pending.current &&
    pending.current.owner === owner &&
    pending.current.intent === intent;
  const loaded = !factsQuery.isPending && !episodesQuery.isPending;
  const fresh = view.revision !== null && !view.stale;
  const validSelection =
    sources.some((source) => source.id === factId) &&
    (mode === "create" ||
      currentEpisodes.some((episode) => episode.id === episodeId));
  const canSubmit =
    !!owner &&
    canEdit &&
    !saving &&
    !recording &&
    (replay || (loaded && fresh && validSelection));
  const symptomIntent = JSON.stringify([
    symptom.statement.trim(),
    symptom.symptomType.trim(),
    symptom.presence,
    symptom.laterality,
  ]);
  const symptomReplay =
    !!pendingSymptom.current &&
    pendingSymptom.current.owner === owner &&
    pendingSymptom.current.intent === symptomIntent;
  const canRecord =
    !!owner &&
    canEdit &&
    !recording &&
    !saving &&
    (symptomReplay ||
      (loaded &&
        fresh &&
        !!symptom.statement.trim() &&
        !!symptom.symptomType.trim()));

  async function refresh(clearError = true) {
    if (clearError) setSaveError(null);
    await Promise.all([factsQuery.refetch(), episodesQuery.refetch()]);
  }

  async function recordSymptom() {
    if (!owner || !canRecord) return;
    const submittingOwner = owner;
    let attempt = pendingSymptom.current;
    if (
      !attempt ||
      attempt.owner !== owner ||
      attempt.intent !== symptomIntent
    ) {
      if (!fresh || view.revision === null) return;
      try {
        attempt = {
          owner,
          intent: symptomIntent,
          input: makeEpisodeSymptomRecordInput(
            symptom,
            view.revision,
            crypto.randomUUID(),
          ),
        };
      } catch (cause) {
        setRecordError(errorMessage(cause, "请输入明确的症状陈述和类型。"));
        return;
      }
      pendingSymptom.current = attempt;
    }
    const request = attempt.input;
    setRecording(true);
    setRecordError(null);
    setNotice(null);
    try {
      const receipt = await bodyFactRecordsService.record(request);
      if (ownerRef.current !== submittingOwner) return;
      if (
        receipt.user_id !== submittingOwner ||
        receipt.revision <= request.expected_revision ||
        receipt.fact_ids.length !== 1
      ) {
        throw new Error("症状记录回执不完整，尚不能确认保存。");
      }
      pendingSymptom.current = null;
      setAddingSymptom(false);
      setFactId(
        symptom.presence === "present" || mode === "link"
          ? receipt.fact_ids[0]
          : "",
      );
      setSymptom(initialSymptomDraft);
      setNotice(
        "本人症状陈述已保存。请单独确认创建或关联经历；这不是病情诊断。",
      );
      void queryClient.invalidateQueries({ queryKey: ["workspace"] });
      try {
        await Promise.all([
          factsQuery.refetch({ throwOnError: true }),
          episodesQuery.refetch({ throwOnError: true }),
        ]);
      } catch {
        if (ownerRef.current === submittingOwner) {
          setNotice(
            "症状陈述已保存，但列表更新失败。请刷新后继续，勿重复提交。",
          );
        }
      }
    } catch (cause) {
      if (ownerRef.current !== submittingOwner) return;
      if (
        cause instanceof ApiRequestError &&
        (cause.status === 400 || cause.status === 404 || cause.status === 409)
      ) {
        pendingSymptom.current = null;
        setRecordError("症状记录或档案版本已变化。请刷新核对后重新保存。");
        void refresh(false);
      } else {
        setRecordError(
          errorMessage(cause, "提交结果尚未确认；重试将使用完全相同的请求。"),
        );
      }
    } finally {
      if (ownerRef.current === submittingOwner) setRecording(false);
    }
  }

  async function submit() {
    if (!owner || !canEdit || saving || recording || !factId || !canSubmit)
      return;
    const submittingOwner = owner;
    let attempt = pending.current;
    if (!attempt || attempt.owner !== owner || attempt.intent !== intent) {
      if (!fresh || !validSelection || view.revision === null) return;
      attempt = {
        owner,
        intent,
        input: {
          idempotency_key: crypto.randomUUID(),
          expected_revision: view.revision,
          fact_id: factId,
        },
      };
      pending.current = attempt;
    }
    const request = attempt.input;
    const target = mode === "link" ? episodeId : null;
    setSaving(true);
    setSaveError(null);
    setNotice(null);
    try {
      const receipt: BodyEpisodeCommandReceipt = target
        ? await bodyEpisodeService.linkFact(target, request)
        : await bodyEpisodeService.create(request);
      if (ownerRef.current !== submittingOwner) return;
      if (
        receipt.user_id !== submittingOwner ||
        receipt.fact_id !== factId ||
        receipt.operation !== mode ||
        (target !== null && receipt.episode_id !== target) ||
        receipt.revision <= request.expected_revision
      ) {
        throw new Error("提交回执与所选择的档案或症状经历不一致。");
      }
      // The domain mutation already committed; never resend it just because
      // the following refresh fails or the client lost visibility.
      pending.current = null;
      setFactId("");
      setEpisodeId("");
      setNotice("关联已提交；仅整理本人症状陈述，不代表确诊或症状已结束。");
      void queryClient.invalidateQueries({ queryKey: ["workspace"] });
      try {
        await Promise.all([
          factsQuery.refetch({ throwOnError: true }),
          episodesQuery.refetch({ throwOnError: true }),
        ]);
      } catch {
        if (ownerRef.current === submittingOwner) {
          setNotice(
            "关联已提交，但列表刷新失败。请使用刷新按钮重新读取；不要重复创建。",
          );
        }
      }
    } catch (cause) {
      if (ownerRef.current !== submittingOwner) return;
      if (
        cause instanceof ApiRequestError &&
        (cause.status === 404 || cause.status === 409 || cause.status === 400)
      ) {
        pending.current = null;
        setSaveError(
          "记录或经历已经变化，当前操作未获确认。请刷新后核对，再重新提交。",
        );
        void refresh(false);
      } else {
        setSaveError(
          errorMessage(
            cause,
            "尚未确认是否提交成功；重试将使用相同请求和幂等键，不会另建经历。",
          ),
        );
      }
    } finally {
      if (ownerRef.current === submittingOwner) setSaving(false);
    }
  }

  if (!owner) return <p className="bc-window-note">请登录后查看症状经历。</p>;

  return (
    <div
      data-testid="body-episode-editor"
      className="bc-record-editor bc-episode-editor"
    >
      <div className="bc-record-actions">
        <span className="bc-window-note">
          症状经历仅组织已记录的本人陈述；持续或结束状态必须有明确证据。
        </span>
        <button
          className="bc-button"
          type="button"
          aria-label="刷新症状经历和身体记录"
          onClick={() => void refresh()}
          disabled={
            saving ||
            recording ||
            factsQuery.isFetching ||
            episodesQuery.isFetching
          }
        >
          <RefreshCw size={15} />
        </button>
      </div>
      {factsQuery.isPending || episodesQuery.isPending ? (
        <p role="status" className="bc-window-loading">
          <LoaderCircle size={16} className="animate-spin" />
          正在读取症状经历
        </p>
      ) : null}
      {factsQuery.isError || episodesQuery.isError ? (
        <p role="alert" className="bc-error">
          资料读取失败，请刷新重试；不能据此判断没有记录。
        </p>
      ) : null}
      {view.stale ? (
        <p role="alert" className="bc-error">
          身体记录和症状经历版本不一致，请刷新后再操作。
        </p>
      ) : null}
      {notice ? (
        <p role="status" className="bc-record-saved">
          {notice}
        </p>
      ) : null}
      {saveError ? (
        <p role="alert" className="bc-error">
          {saveError}
        </p>
      ) : null}
      {loaded && !view.stale ? (
        <>
          <ul className="bc-record-list" aria-label="已读取的症状经历">
            {view.episodes.map((episode) => (
              <li className="bc-record-item" key={episode.id}>
                <strong>{episode.symptom_type}</strong>
                <p className="bc-window-note">{episodeStateLabel(episode)}</p>
                <span className="bc-window-note">
                  有效关联陈述 {episode.links.length} 条
                </span>
              </li>
            ))}
          </ul>
          {view.episodes.length === 0 ? (
            <p className="bc-window-empty-text">
              当前已读取的记录中没有症状经历；不代表不存在更早的经历。
            </p>
          ) : null}
          {episodesQuery.hasNextPage ? (
            <button
              type="button"
              className="bc-button"
              disabled={episodesQuery.isFetchingNextPage}
              onClick={() => void episodesQuery.fetchNextPage()}
            >
              {episodesQuery.isFetchingNextPage
                ? "正在读取"
                : "读取更早的症状经历"}
            </button>
          ) : null}
          {canEdit && !addingSymptom ? (
            <button
              type="button"
              className="bc-button"
              disabled={saving || recording}
              onClick={() => {
                setAddingSymptom(true);
                setRecordError(null);
              }}
            >
              补充可关联的本人症状
            </button>
          ) : null}
          {canEdit && addingSymptom ? (
            <form
              className="bc-record-editor"
              aria-label="记录结构化本人症状"
              onSubmit={(event) => {
                event.preventDefault();
                void recordSymptom();
              }}
            >
              <p className="bc-window-note">
                请按本人实际陈述填写；症状类型不是诊断，保存陈述后仍需单独确认关联。
              </p>
              <label>
                本人完整症状陈述
                <textarea
                  value={symptom.statement}
                  rows={2}
                  maxLength={1000}
                  disabled={recording}
                  placeholder="例如：我今天右肩感觉疼痛"
                  onChange={(event) => {
                    setSymptom((current) => ({
                      ...current,
                      statement: event.target.value,
                    }));
                    setRecordError(null);
                  }}
                />
              </label>
              <label>
                症状类型（本人描述）
                <input
                  type="text"
                  value={symptom.symptomType}
                  maxLength={120}
                  disabled={recording}
                  placeholder="例如：肩部疼痛"
                  onChange={(event) => {
                    setSymptom((current) => ({
                      ...current,
                      symptomType: event.target.value,
                    }));
                    setRecordError(null);
                  }}
                />
              </label>
              <label>
                当前陈述是否存在症状
                <select
                  value={symptom.presence}
                  disabled={recording}
                  onChange={(event) =>
                    setSymptom((current) => ({
                      ...current,
                      presence: event.target
                        .value as EpisodeSymptomDraft["presence"],
                    }))
                  }
                >
                  <option value="present">有症状</option>
                  <option value="absent">无症状</option>
                </select>
              </label>
              <label>
                侧别（不确定可以不指定）
                <select
                  value={symptom.laterality}
                  disabled={recording}
                  onChange={(event) =>
                    setSymptom((current) => ({
                      ...current,
                      laterality: event.target
                        .value as EpisodeSymptomDraft["laterality"],
                    }))
                  }
                >
                  <option value="unknown">不确定</option>
                  <option value="left">左侧</option>
                  <option value="right">右侧</option>
                  <option value="bilateral">双侧</option>
                  <option value="not_applicable">不适用</option>
                </select>
              </label>
              {recordError ? (
                <p role="alert" className="bc-error">
                  {recordError}
                </p>
              ) : null}
              <div className="bc-record-actions">
                <button
                  type="submit"
                  className="bc-button bc-primary"
                  disabled={!canRecord}
                >
                  {recording
                    ? "正在保存陈述"
                    : symptomReplay
                      ? "重试原症状记录"
                      : "先保存症状陈述"}
                </button>
                <button
                  type="button"
                  className="bc-button"
                  disabled={recording}
                  onClick={() => setAddingSymptom(false)}
                >
                  收起症状录入
                </button>
              </div>
            </form>
          ) : null}
          {canEdit ? (
            <form
              className="bc-record-editor"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              <label>
                操作
                <select
                  value={mode}
                  disabled={saving}
                  onChange={(event) => {
                    setMode(event.target.value as EpisodeAssociationMode);
                    setFactId("");
                    setEpisodeId("");
                    setSaveError(null);
                  }}
                >
                  <option value="create">新建症状经历</option>
                  <option value="link">关联到已有经历</option>
                </select>
              </label>
              <label>
                本人症状陈述
                <select
                  value={factId}
                  disabled={saving}
                  onChange={(event) => {
                    setFactId(event.target.value);
                    setSaveError(null);
                  }}
                >
                  <option value="">选择已保存的症状</option>
                  {sources.map((source) => (
                    <option key={source.id} value={source.id}>
                      {source.statement}
                      {source.presence === "absent" ? "（未出现症状）" : ""}
                    </option>
                  ))}
                </select>
              </label>
              {mode === "link" ? (
                <label>
                  目标经历
                  <select
                    value={episodeId}
                    disabled={saving}
                    onChange={(event) => {
                      setEpisodeId(event.target.value);
                      setSaveError(null);
                    }}
                  >
                    <option value="">选择已有经历</option>
                    {currentEpisodes.map((episode) => (
                      <option key={episode.id} value={episode.id}>
                        {episode.symptom_type}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
              {sources.length === 0 ? (
                <p className="bc-window-note">
                  当前已读取的记录中没有可用于此操作的症状陈述。
                </p>
              ) : null}
              {factsQuery.hasNextPage ? (
                <button
                  type="button"
                  className="bc-button"
                  disabled={factsQuery.isFetchingNextPage}
                  onClick={() => void factsQuery.fetchNextPage()}
                >
                  {factsQuery.isFetchingNextPage
                    ? "正在读取"
                    : "读取更早的症状陈述"}
                </button>
              ) : null}
              {view.factsHaveMore || view.episodesHaveMore ? (
                <p className="bc-window-note">
                  仍有更早的记录未加载；服务端会验证完整关联关系，防止重复创建。
                </p>
              ) : null}
              <div className="bc-record-actions">
                <button
                  type="submit"
                  className="bc-button bc-primary"
                  disabled={!canSubmit}
                >
                  {saving ? (
                    <LoaderCircle size={16} className="animate-spin" />
                  ) : null}
                  {saving
                    ? "正在提交"
                    : replay
                      ? "重试原请求"
                      : mode === "create"
                        ? "确认新建经历"
                        : "确认关联陈述"}
                </button>
              </div>
            </form>
          ) : null}
        </>
      ) : null}
      {/* An uncertain symptom write can be replayed even when both read models are stale. */}
      {symptomReplay && (!loaded || view.stale) && canEdit ? (
        <button
          type="button"
          className="bc-button"
          disabled={recording || saving}
          onClick={() => void recordSymptom()}
        >
          重试原症状记录
        </button>
      ) : null}
      {/* A pending retry survives collapsing the disclosure and revision changes. */}
      {replay && loaded && view.stale && canEdit ? (
        <button
          type="button"
          className="bc-button"
          onClick={() => void submit()}
          disabled={saving}
        >
          重试原请求
        </button>
      ) : null}
    </div>
  );
}

/** Reuses the current Person window chrome without replacing Penpot's window layout. */
export function BodyEpisodeAssociations({ canEdit }: { canEdit: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const [loadedOnce, setLoadedOnce] = useState(false);
  return (
    <section
      aria-label="症状经历"
      className="bc-record-item"
      data-testid="body-episode-associations"
    >
      <div className="bc-record-actions">
        <span className="bc-window-note">症状经历</span>
        <button
          type="button"
          className="bc-text-button"
          aria-expanded={expanded}
          onClick={() => {
            setExpanded((previous) => !previous);
            setLoadedOnce(true);
          }}
        >
          {expanded ? "收起经历" : "查看与关联经历"}
        </button>
      </div>
      {loadedOnce ? (
        <div hidden={!expanded}>
          <BodyEpisodeEditor canEdit={canEdit} />
        </div>
      ) : null}
    </section>
  );
}
