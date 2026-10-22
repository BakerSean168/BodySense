import { useQuery } from "@tanstack/react-query";
import { Check, CircleHelp, Link2 } from "lucide-react";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { bodyHypothesisService } from "@/features/profile/services/bodyHypothesisService";
import { formatInstantInZone } from "@/features/profile/model/profilePreferences";
import { useAuthStore } from "@/stores/authStore";
import { PersonDataError } from "../PersonDataError";
import { BODY_FACT_KIND_LABELS } from "../../../../model/bodyFactPresentation";
import { describeOccurrence } from "../../../../model/bodyFactViewModel";
import {
  episodeGaps,
  episodeTitle,
} from "../../../../model/personRecordsTimeline";
import {
  ContinuityChip,
  PersonRecordsLoading,
  PersonSubview,
} from "./PersonRecordsElements";

type Episode = BodyRecords["episodes"][number];

export function EpisodeDetail({
  episode,
  data,
  zone,
  loading,
  error,
  onRetry,
  backLabel,
  onBack,
  onSupplement,
  onOpenGroups,
}: {
  episode: Episode | null;
  data: BodyRecords | undefined;
  zone: string;
  loading: boolean;
  error: boolean;
  onRetry: () => void;
  backLabel: string;
  onBack: () => void;
  onSupplement: () => void;
  onOpenGroups: () => void;
}) {
  const owner = useAuthStore((state) => state.user?.id);
  // Same read the hypothesis panel uses; groups are derived, never stored.
  const groups = useQuery({
    queryKey: ["workspace", "concern-groups", owner],
    queryFn: async () => {
      const page = await bodyHypothesisService.groups();
      if (page.user_id !== owner)
        throw new Error("档案身份不一致，请重新读取。");
      return page;
    },
    enabled: Boolean(owner && episode),
    retry: false,
    staleTime: 0,
  });
  if (!episode) {
    return (
      <PersonSubview
        title="经历"
        testId="person-episode"
        onBack={onBack}
        backLabel={backLabel}
      >
        {error ? (
          <PersonDataError
            title="经历暂时无法同步"
            message="当前读取失败，不能据此判断这段经历已移除。"
            onRetry={onRetry}
          />
        ) : loading ? (
          <PersonRecordsLoading label="正在读取经历" />
        ) : (
          <p role="status" className="bc-window-empty-text">
            这段经历已不在当前记录中。
          </p>
        )}
      </PersonSubview>
    );
  }
  const facts = episode.links
    .map((link) => data?.facts.find((fact) => fact.id === link.fact_id))
    .filter((fact): fact is NonNullable<typeof fact> => Boolean(fact));
  const gaps = episodeGaps(episode, facts);
  // Only groups pinned to the same revision as the records snapshot.
  const memberGroups =
    groups.data &&
    data &&
    groups.data.current_revision === data.current_revision
      ? groups.data.groups.filter((group) =>
          group.related_episode_ids.includes(episode.id),
        )
      : [];
  return (
    <PersonSubview
      title={episodeTitle(episode)}
      testId="person-episode"
      onBack={onBack}
      backLabel={backLabel}
      chips={
        <>
          <ContinuityChip episode={episode} />
          <span className="bc-person-chip">{episode.links.length} 条陈述</span>
        </>
      }
      action={
        <button
          type="button"
          className="bc-person-footer-action"
          onClick={onSupplement}
        >
          补充说明
        </button>
      }
    >
      {error ? (
        <PersonDataError
          title="经历暂时无法同步"
          message="以下为上次读取的经历，请重新读取以确认最新状态。"
          onRetry={onRetry}
        />
      ) : null}
      <section className="bc-person-section" aria-label="陈述">
        <h4 className="bc-person-section-label">陈述</h4>
        <ul className="bc-person-evidence">
          {facts.map((fact) => (
            <li key={fact.id}>
              <span className="bc-person-evidence-icon" aria-hidden="true">
                <Check size={13} />
              </span>
              <span>
                <span className="bc-person-quote">“{fact.statement}”</span>
                <span className="bc-person-row-meta">
                  {BODY_FACT_KIND_LABELS[fact.kind]} · 发生{" "}
                  {describeOccurrence(fact.temporal)} · 记录{" "}
                  {formatInstantInZone(fact.recorded_at, zone) ?? "时间未知"}
                </span>
              </span>
            </li>
          ))}
        </ul>
        {facts.length < episode.links.length ? (
          <p className="bc-window-note">
            另有 {episode.links.length - facts.length}{" "}
            条陈述不在当前读取范围内。
          </p>
        ) : null}
      </section>
      {gaps.length ? (
        <section className="bc-person-section" aria-label="信息缺口">
          <h4 className="bc-person-section-label">信息缺口</h4>
          <ul className="bc-person-evidence" data-tone="gap">
            {gaps.map((gap) => (
              <li key={gap.id}>
                <span className="bc-person-evidence-icon" aria-hidden="true">
                  <CircleHelp size={13} />
                </span>
                <span>{gap.label}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      {groups.isError ? (
        <PersonDataError
          title="相关猜测暂时无法读取"
          message="经历和陈述仍可查看，不能据此判断没有关联。"
          onRetry={() => void groups.refetch()}
        />
      ) : null}
      {memberGroups.length ? (
        <section className="bc-person-section" aria-label="相关">
          <h4 className="bc-person-section-label">相关</h4>
          <div className="bc-person-chips">
            {memberGroups.map((group) => (
              <button
                key={group.id}
                type="button"
                className="bc-person-chip bc-person-chip-link"
                onClick={onOpenGroups}
              >
                <Link2 size={12} aria-hidden="true" />
                {group.display_title}
              </button>
            ))}
          </div>
        </section>
      ) : null}
    </PersonSubview>
  );
}
