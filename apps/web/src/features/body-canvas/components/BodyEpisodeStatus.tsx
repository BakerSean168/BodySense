import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import {
  episodeDisplayState,
  episodeStateLabel,
} from "../model/bodyEpisodeAssociations";

const reasons = {
  insufficient_evidence: "状态依据不足",
  conflicting_evidence: "关联陈述存在冲突，持续状态保留不确定",
  explicit_current_report: "来源含明确的持续报告",
  explicit_resolution: "来源含明确的结束报告",
} satisfies Record<BodyEpisode["projection"]["reason"], string>;

/** Read-only status: server projection plus its source qualification, never a UI transition. */
export function BodyEpisodeStatus({ episode }: { episode: BodyEpisode }) {
  const state = episodeDisplayState(episode);
  return (
    <div className="bc-episode-status">
      <span className="bc-episode-badge" data-state={state}>
        {episodeStateLabel(episode)}
      </span>
      <p className="bc-window-note">
        依据已关联陈述推导 · {reasons[episode.projection.reason]} · 状态依据{" "}
        {episode.projection.basis_fact_ids.length} 条
      </p>
      <p className="bc-window-note">
        {episode.projection.state_as_of ? (
          <>
            报告发生时间：
            <time dateTime={episode.projection.state_as_of}>
              {episode.projection.state_as_of}
            </time>
          </>
        ) : (
          "报告发生时间未提供，不以保存时间代替。"
        )}
      </p>
    </div>
  );
}
