import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
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
import { bodyFactRecordsKey } from "../components/BodyFactRecords";
import {
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

/**
 * Single owner of Person Episode reads, revisions, user intents and idempotent
 * receipts. Final Penpot UI can rehost these commands without another source.
 */
export function useBodyEpisodeController(canEdit: boolean) {
  const owner = useAuthStore((state) => state.user?.id);
  const ownerRef = useRef(owner);
  // Commit identity before asynchronous receipts can update this surface.
  useLayoutEffect(() => {
    ownerRef.current = owner;
  }, [owner]);
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
  // An ambiguous command is an unresolved durable write, not a dismissible
  // draft. Keep its intent immutable until a real receipt or definite reject.
  const episodeLocked = !!pending.current && pending.current.owner === owner;
  const replay = episodeLocked && pending.current?.intent === intent;
  const loaded = !factsQuery.isPending && !episodesQuery.isPending;
  const readFailed = factsQuery.isError || episodesQuery.isError;
  const fresh = view.revision !== null && !view.stale;
  // Cached pages may exist after a refetch error; never present that as a
  // successful empty projection or use it for a new revision-bound write.
  const readReady = loaded && !readFailed && fresh;
  const canWriteFromRead =
    readReady && !factsQuery.isFetching && !episodesQuery.isFetching;
  const validSelection =
    sources.some((source) => source.id === factId) &&
    (mode === "create" ||
      currentEpisodes.some((episode) => episode.id === episodeId));
  const symptomLocked =
    !!pendingSymptom.current && pendingSymptom.current.owner === owner;
  const canSubmit =
    !!owner &&
    canEdit &&
    !saving &&
    !recording &&
    !symptomLocked &&
    (replay || (!episodeLocked && canWriteFromRead && validSelection));
  const symptomIntent = JSON.stringify([
    symptom.statement.trim(),
    symptom.symptomType.trim(),
    symptom.presence,
    symptom.laterality,
  ]);
  const symptomReplay =
    symptomLocked && pendingSymptom.current?.intent === symptomIntent;
  const canRecord =
    !!owner &&
    canEdit &&
    !recording &&
    !saving &&
    !episodeLocked &&
    (symptomReplay ||
      (!symptomLocked &&
        canWriteFromRead &&
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

  return {
    owner,
    factsQuery,
    episodesQuery,
    view,
    mode,
    factId,
    episodeId,
    saving,
    recording,
    addingSymptom,
    symptom,
    recordError,
    notice,
    saveError,
    episodeLocked,
    symptomLocked,
    readFailed,
    readReady,
    canSubmit,
    canRecord,
    symptomReplay,
    replay,
    sources,
    currentEpisodes,
    setAddingSymptom,
    setRecordError,
    setSymptom,
    setMode,
    setFactId,
    setEpisodeId,
    setSaveError,
    refresh,
    recordSymptom,
    submit,
  };
}
