import { useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Check,
  ChevronRight,
  CircleHelp,
  Link2,
  LoaderCircle,
} from "lucide-react";
import { useBodyRecordsQuery } from "@/features/profile/hooks/useBodyRecordsQuery";
import { bodyHypothesisService } from "@/features/profile/services/bodyHypothesisService";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import {
  effectiveTimeZone,
  formatInstantInZone,
} from "@/features/profile/model/profilePreferences";
import { useAuthStore } from "@/stores/authStore";
import { useProfileStore } from "@/stores/profileStore";
import { BODY_FACT_KIND_LABELS } from "../model/bodyFactPresentation";
import { describeOccurrence } from "../model/bodyFactViewModel";
import { episodeStateLabel } from "../model/bodyEpisodeAssociations";
import {
  episodeContinuityChip,
  episodeGaps,
  episodeTitle,
} from "../model/personRecordsTimeline";
import { BodyFactRecords, type BodyFactRelatedLink } from "./BodyFactRecords";
import { BodyEpisodeEditor } from "./BodyEpisodeAssociations";
import { BodyHypotheses } from "./BodyHypotheses";

type Episode = BodyRecords["episodes"][number];

/**
 * Person / 身体记录 navigation. The timeline (Penpot Person / Records) is the
 * home; Episodes, the user's own hypotheses (with derived ConcernGroups) and
 * observations open in the same window as Penpot Body / Episode-style detail
 * scenes, each with a footer back to the records. Nothing here infers a
 * clinical state: Episode continuity is Go's projection, groups are derived.
 */
type View =
  | { kind: "timeline" }
  | { kind: "episodes"; editing?: boolean }
  | { kind: "episode"; id: string; from: "timeline" | "episodes" }
  | { kind: "hypotheses" }
  | { kind: "observations" };

export function PersonRecordsView({
  canEdit,
  now,
}: {
  canEdit: boolean;
  now?: Date;
}) {
  const [view, setView] = useState<View>({ kind: "timeline" });
  const owner = useAuthStore((state) => state.user?.id);
  const zone = effectiveTimeZone(
    useProfileStore((state) => state.profile?.preferences),
  ).zone;
  const records = useBodyRecordsQuery();
  const data = records.data?.user_id === owner ? records.data : undefined;

  const episodesByFact = useMemo(() => {
    const map = new Map<string, Episode[]>();
    for (const episode of data?.episodes ?? []) {
      if (episode.identity_status !== "current") continue;
      for (const link of episode.links) {
        map.set(link.fact_id, [...(map.get(link.fact_id) ?? []), episode]);
      }
    }
    return map;
  }, [data]);

  const back = () => setView({ kind: "timeline" });

  if (view.kind === "episode") {
    const episode = data?.episodes.find((item) => item.id === view.id);
    return (
      <EpisodeDetail
        episode={episode ?? null}
        data={data}
        zone={zone}
        loading={records.isPending}
        backLabel={view.from === "episodes" ? "返回经历" : "返回记录"}
        onBack={() =>
          setView(
            view.from === "episodes"
              ? { kind: "episodes" }
              : { kind: "timeline" },
          )
        }
        onSupplement={() => setView({ kind: "episodes", editing: true })}
        onOpenGroups={() => setView({ kind: "hypotheses" })}
      />
    );
  }

  if (view.kind === "episodes") {
    const episodes = (data?.episodes ?? []).filter(
      (episode) => episode.identity_status === "current",
    );
    return (
      <PersonSubview
        title="经历"
        testId="person-episodes"
        onBack={back}
        action={
          canEdit && !view.editing ? (
            <button
              type="button"
              className="bc-person-footer-action"
              onClick={() => setView({ kind: "episodes", editing: true })}
            >
              整理经历
            </button>
          ) : null
        }
      >
        {records.isPending ? <Loading label="正在读取经历" /> : null}
        {records.isError ? (
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
                  onClick={() =>
                    setView({
                      kind: "episode",
                      id: episode.id,
                      from: "episodes",
                    })
                  }
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
        {canEdit && view.editing ? (
          <div className="bc-person-section">
            <h4 className="bc-person-section-label">整理经历</h4>
            <BodyEpisodeEditor canEdit={canEdit} />
          </div>
        ) : null}
      </PersonSubview>
    );
  }

  if (view.kind === "hypotheses") {
    return (
      <PersonSubview title="我的猜测" testId="person-hypotheses" onBack={back}>
        <BodyHypotheses canEdit={canEdit} />
      </PersonSubview>
    );
  }

  if (view.kind === "observations") {
    const observations = data?.observations ?? [];
    return (
      <PersonSubview title="观察" testId="person-observations" onBack={back}>
        {records.isPending ? <Loading label="正在读取观察记录" /> : null}
        {records.isError ? (
          <div role="alert" className="bc-error">
            暂时无法读取观察记录
            <button
              type="button"
              className="bc-text-button"
              onClick={() => void records.refetch()}
            >
              重试
            </button>
          </div>
        ) : null}
        {data && !observations.length ? (
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
                        `记录 ${formatInstantInZone(observation.recorded_at, zone) ?? "时间未知"}`,
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

  const currentEpisodes = (data?.episodes ?? []).filter(
    (episode) => episode.identity_status === "current",
  );
  const related = data ? (
    <nav className="bc-person-related" aria-label="相关记录">
      <span className="bc-person-related-label">相关</span>
      <button
        type="button"
        className="bc-person-chip bc-person-chip-link"
        onClick={() => setView({ kind: "episodes" })}
      >
        经历 {currentEpisodes.length} 段
      </button>
      <button
        type="button"
        className="bc-person-chip bc-person-chip-link"
        onClick={() => setView({ kind: "hypotheses" })}
      >
        我的猜测 {data.hypotheses.length} 条
      </button>
      <button
        type="button"
        className="bc-person-chip bc-person-chip-link"
        onClick={() => setView({ kind: "observations" })}
      >
        观察 {data.observations.length} 条
      </button>
    </nav>
  ) : null;

  return (
    <BodyFactRecords
      canEdit={canEdit}
      now={now}
      related={related}
      relatedLinks={(factId): BodyFactRelatedLink[] =>
        (episodesByFact.get(factId) ?? []).map((episode) => ({
          id: episode.id,
          label: episodeTitle(episode),
        }))
      }
      onOpenRelated={(link) =>
        setView({ kind: "episode", id: link.id, from: "timeline" })
      }
    />
  );
}

function Loading({ label }: { label: string }) {
  return (
    <p role="status" className="bc-person-loading">
      <LoaderCircle size={16} className="animate-spin" />
      {label}
    </p>
  );
}

function ContinuityChip({ episode }: { episode: Episode }) {
  return (
    <span
      className="bc-person-chip bc-person-continuity"
      data-state={episode.projection.state}
      title={episodeStateLabel(episode)}
    >
      {episodeContinuityChip(episode)}
    </span>
  );
}

/** Penpot Body / Episode chrome: title, chips, sections, footer back link. */
export function PersonSubview({
  title,
  chips,
  testId,
  onBack,
  backLabel = "返回记录",
  action,
  children,
}: {
  title: string;
  chips?: ReactNode;
  testId: string;
  onBack: () => void;
  backLabel?: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="bc-person-category bc-person-detail" data-testid={testId}>
      <div className="bc-person-detail-body">
        <h3 className="bc-person-detail-title">{title}</h3>
        {chips ? <div className="bc-person-chips">{chips}</div> : null}
        {children}
      </div>
      <footer className="bc-person-footer">
        <button
          type="button"
          className="bc-person-footer-link"
          onClick={onBack}
        >
          {backLabel}
        </button>
        {action}
      </footer>
    </div>
  );
}

function EpisodeDetail({
  episode,
  data,
  zone,
  loading,
  backLabel,
  onBack,
  onSupplement,
  onOpenGroups,
}: {
  episode: Episode | null;
  data: BodyRecords | undefined;
  zone: string;
  loading: boolean;
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
        {loading ? (
          <Loading label="正在读取经历" />
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
