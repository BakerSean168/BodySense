import { LoaderCircle } from "lucide-react";
import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import type {
  EpisodeAssociationMode,
  EpisodeSourceOption,
} from "../model/bodyEpisodeAssociations";

interface BodyEpisodeAssociationFormProps {
  mode: EpisodeAssociationMode;
  factId: string;
  episodeId: string;
  sources: readonly EpisodeSourceOption[];
  episodes: readonly BodyEpisode[];
  locked: boolean;
  submitting: boolean;
  retry: boolean;
  canSubmit: boolean;
  factsHasNextPage: boolean;
  fetchingMoreFacts: boolean;
  hasUnloadedPages: boolean;
  onModeChange: (mode: EpisodeAssociationMode) => void;
  onFactChange: (id: string) => void;
  onEpisodeChange: (id: string) => void;
  onLoadMoreFacts: () => void;
  onSubmit: () => void;
}

/** Reusable intent form: never mutates a record or infers a clinical status. */
export function BodyEpisodeAssociationForm({
  mode,
  factId,
  episodeId,
  sources,
  episodes,
  locked,
  submitting,
  retry,
  canSubmit,
  factsHasNextPage,
  fetchingMoreFacts,
  hasUnloadedPages,
  onModeChange,
  onFactChange,
  onEpisodeChange,
  onLoadMoreFacts,
  onSubmit,
}: BodyEpisodeAssociationFormProps) {
  return (
    <form
      className="bc-record-editor"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <label>
        操作
        <select
          value={mode}
          disabled={locked}
          onChange={(event) =>
            onModeChange(event.target.value as EpisodeAssociationMode)
          }
        >
          <option value="create">新建症状经历</option>
          <option value="link">关联到已有经历</option>
        </select>
      </label>
      <label>
        本人症状陈述
        <select
          value={factId}
          disabled={locked}
          onChange={(event) => onFactChange(event.target.value)}
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
            disabled={locked}
            onChange={(event) => onEpisodeChange(event.target.value)}
          >
            <option value="">选择已有经历</option>
            {episodes.map((episode) => (
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
      {factsHasNextPage ? (
        <button
          type="button"
          className="bc-button"
          disabled={fetchingMoreFacts || locked}
          onClick={onLoadMoreFacts}
        >
          {fetchingMoreFacts ? "正在读取" : "读取更早的症状陈述"}
        </button>
      ) : null}
      {hasUnloadedPages ? (
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
          {submitting ? (
            <LoaderCircle size={16} className="animate-spin" />
          ) : null}
          {submitting
            ? "正在提交"
            : retry
              ? "重试原请求"
              : mode === "create"
                ? "确认新建经历"
                : "确认关联陈述"}
        </button>
      </div>
    </form>
  );
}
