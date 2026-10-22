import { ChevronRight } from "lucide-react";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { formatInstantInZone } from "@/features/profile/model/profilePreferences";
import { episodeTitle } from "../../../../model/personRecordsTimeline";
import { BodyEpisodeEditor } from "../../../../components/BodyEpisodeAssociations";
import {
  PersonRecordsLoading,
  ContinuityChip,
  PersonSubview,
} from "./PersonRecordsElements";

export function EpisodesScene({
  canEdit,
  editing,
  data,
  loading,
  error,
  onBack,
  onEdit,
  onSelectEpisode,
}: {
  canEdit: boolean;
  editing: boolean;
  data: BodyRecords | undefined;
  loading: boolean;
  error: boolean;
  onBack: () => void;
  onEdit: () => void;
  onSelectEpisode: (id: string) => void;
}) {
  const episodes = (data?.episodes ?? []).filter(
    (episode) => episode.identity_status === "current",
  );
  return (
    <PersonSubview
      title="经历"
      testId="person-episodes"
      onBack={onBack}
      action={
        canEdit && !editing ? (
          <button
            type="button"
            className="bc-person-footer-action"
            onClick={onEdit}
          >
            整理经历
          </button>
        ) : null
      }
    >
      {loading ? <PersonRecordsLoading label="正在读取经历" /> : null}
      {error ? (
        <p role="alert" className="bc-error">
          经历没有读取出来，不能据此判断没有记录。
        </p>
      ) : null}
      {data && !episodes.length ? (
        <p className="bc-window-empty-text">
          还没有整理出经历。把同一症状的陈述整理到一段经历里，就会出现在这里。
        </p>
      ) : null}
      {episodes.length ? (
        <ul className="bc-person-timeline" aria-label="已读取的症状经历">
          {episodes.map((episode) => (
            <li key={episode.id}>
              <button
                type="button"
                className="bc-person-row"
                onClick={() => onSelectEpisode(episode.id)}
              >
                <span className="bc-person-row-main">
                  <span className="bc-record-statement">
                    {episodeTitle(episode)}
                  </span>
                  <span className="bc-person-chips">
                    <ContinuityChip episode={episode} />
                    <span className="bc-person-chip">
                      {episode.links.length} 条陈述
                    </span>
                  </span>
                </span>
                <ChevronRight
                  size={17}
                  className="bc-person-row-chevron"
                  aria-hidden="true"
                />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      {canEdit && editing ? (
        <div className="bc-person-section">
          <h4 className="bc-person-section-label">整理经历</h4>
          <BodyEpisodeEditor canEdit={canEdit} />
        </div>
      ) : null}
    </PersonSubview>
  );
}

export function ObservationsScene({
  observations,
  dataLoaded,
  loading,
  error,
  zone,
  onBack,
  onRetry,
}: {
  observations: BodyRecords["observations"];
  dataLoaded: boolean;
  loading: boolean;
  error: boolean;
  zone: string;
  onBack: () => void;
  onRetry: () => void;
}) {
  return (
    <PersonSubview title="观察" testId="person-observations" onBack={onBack}>
      {loading ? <PersonRecordsLoading label="正在读取观察记录" /> : null}
      {error ? (
        <div role="alert" className="bc-error">
          暂时无法读取观察记录
          <button type="button" className="bc-text-button" onClick={onRetry}>
            重试
          </button>
        </div>
      ) : null}
      {dataLoaded && !observations.length ? (
        <p className="bc-window-empty-text">暂无观察记录</p>
      ) : null}
      {observations.length ? (
        <ul className="bc-person-timeline" aria-label="观察记录">
          {observations.map((observation) => (
            <li key={observation.id}>
              <div className="bc-person-row" data-static="true">
                <span className="bc-person-row-kind">观察</span>
                <span className="bc-person-row-main">
                  <span className="bc-record-statement">
                    {observation.statement}
                  </span>
                  <span className="bc-person-row-meta">
                    {[
                      [observation.raw_value, observation.raw_unit]
                        .filter(Boolean)
                        .join(" "),
                      observation.method,
                      "记录 " +
                        (formatInstantInZone(observation.recorded_at, zone) ??
                          "时间未知"),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </span>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
    </PersonSubview>
  );
}
