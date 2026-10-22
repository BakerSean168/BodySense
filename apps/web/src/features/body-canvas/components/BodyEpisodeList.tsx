import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import { BodyEpisodeStatus } from "./BodyEpisodeStatus";

interface BodyEpisodeListProps {
  episodes: readonly BodyEpisode[];
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  onLoadMore: () => void;
}

/** Read-only projection. Lifecycle and health claims are owned by Go. */
export function BodyEpisodeList({
  episodes,
  hasNextPage,
  isFetchingNextPage,
  onLoadMore,
}: BodyEpisodeListProps) {
  return (
    <>
      <ul className="bc-record-list" aria-label="已读取的症状经历">
        {episodes.map((episode) => (
          <li className="bc-record-item" key={episode.id}>
            <strong>{episode.symptom_type}</strong>
            <BodyEpisodeStatus episode={episode} />
            <span className="bc-window-note">
              关联 {episode.links.length} 条
            </span>
          </li>
        ))}
      </ul>
      {episodes.length === 0 ? (
        <p className="bc-window-empty-text">
          当前已读取的记录中没有症状经历；不代表不存在更早的经历。
        </p>
      ) : null}
      {hasNextPage ? (
        <button
          type="button"
          className="bc-button"
          disabled={isFetchingNextPage}
          onClick={onLoadMore}
        >
          {isFetchingNextPage ? "正在读取" : "读取更早的症状经历"}
        </button>
      ) : null}
    </>
  );
}
