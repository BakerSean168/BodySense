import { LoaderCircle } from "lucide-react";
import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import type {
  EpisodeAssociationMode,
  EpisodeSourceOption,
} from "../model/bodyEpisodeAssociations";
import { SelectField } from "@/components/ui/select";

const MODE_OPTIONS = [
  { value: "create", label: "新建症状经历" },
  { value: "link", label: "关联到已有经历" },
];

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
        <SelectField
          aria-label="操作"
          value={mode}
          disabled={locked}
          options={MODE_OPTIONS}
          onValueChange={(next) => onModeChange(next as EpisodeAssociationMode)}
        />
      </label>
      <label>
        本人症状陈述
        <SelectField
          aria-label="本人症状陈述"
          value={factId}
          disabled={locked}
          placeholder="选择已保存的症状"
          options={sources.map((source) => ({
            value: source.id,
            label: `${source.statement}${source.presence === "absent" ? "（未出现症状）" : ""}`,
          }))}
          onValueChange={onFactChange}
        />
      </label>
      {mode === "link" ? (
        <label>
          目标经历
          <SelectField
            aria-label="目标经历"
            value={episodeId}
            disabled={locked}
            placeholder="选择已有经历"
            options={episodes.map((episode) => ({
              value: episode.id,
              label: episode.symptom_type,
            }))}
            onValueChange={onEpisodeChange}
          />
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
