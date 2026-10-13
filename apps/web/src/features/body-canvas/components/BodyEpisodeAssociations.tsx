import { useState } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";
import { BodyEpisodeList } from "./BodyEpisodeList";
import { BodyEpisodeSymptomForm } from "./BodyEpisodeSymptomForm";
import { BodyEpisodeAssociationForm } from "./BodyEpisodeAssociationForm";
import { useBodyEpisodeController } from "../hooks/useBodyEpisodeController";

/** Functional record-to-Episode bridge. No independent business data store. */
function BodyEpisodeEditor({ canEdit }: { canEdit: boolean }) {
  const {
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
  } = useBodyEpisodeController(canEdit);

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
      {readFailed ? (
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
      {readReady ? (
        <>
          <BodyEpisodeList
            episodes={view.episodes}
            hasNextPage={Boolean(episodesQuery.hasNextPage)}
            isFetchingNextPage={episodesQuery.isFetchingNextPage}
            onLoadMore={() => void episodesQuery.fetchNextPage()}
          />
          {canEdit && !addingSymptom ? (
            <button
              type="button"
              className="bc-button"
              disabled={saving || recording || episodeLocked || symptomLocked}
              onClick={() => {
                setAddingSymptom(true);
                setRecordError(null);
              }}
            >
              补充可关联的本人症状
            </button>
          ) : null}
          {canEdit && addingSymptom ? (
            <BodyEpisodeSymptomForm
              draft={symptom}
              busy={recording}
              unresolved={symptomLocked}
              canSubmit={canRecord}
              retry={symptomReplay}
              error={recordError}
              onChange={(next) => {
                setSymptom(next);
                setRecordError(null);
              }}
              onSubmit={() => void recordSymptom()}
              onCollapse={() => setAddingSymptom(false)}
            />
          ) : null}
          {canEdit ? (
            <BodyEpisodeAssociationForm
              mode={mode}
              factId={factId}
              episodeId={episodeId}
              sources={sources}
              episodes={currentEpisodes}
              locked={saving || recording || episodeLocked || symptomLocked}
              submitting={saving}
              retry={replay}
              canSubmit={canSubmit}
              factsHasNextPage={Boolean(factsQuery.hasNextPage)}
              fetchingMoreFacts={factsQuery.isFetchingNextPage}
              hasUnloadedPages={view.factsHaveMore || view.episodesHaveMore}
              onModeChange={(next) => {
                setMode(next);
                setFactId("");
                setEpisodeId("");
                setSaveError(null);
              }}
              onFactChange={(next) => {
                setFactId(next);
                setSaveError(null);
              }}
              onEpisodeChange={(next) => {
                setEpisodeId(next);
                setSaveError(null);
              }}
              onLoadMoreFacts={() => void factsQuery.fetchNextPage()}
              onSubmit={() => void submit()}
            />
          ) : null}
        </>
      ) : null}
      {/* Retain the original idempotency key when cached reads are stale or unavailable. */}
      {symptomReplay && !readReady && canEdit ? (
        <button
          type="button"
          className="bc-button"
          disabled={recording || saving}
          onClick={() => void recordSymptom()}
        >
          重试原症状记录
        </button>
      ) : null}
      {replay && !readReady && canEdit ? (
        <button
          type="button"
          className="bc-button"
          disabled={saving || recording}
          onClick={() => void submit()}
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
