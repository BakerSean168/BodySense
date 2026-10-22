import { useEffect, useMemo, useState } from "react";
import { useBodyRecordsQuery } from "@/features/profile/hooks/useBodyRecordsQuery";
import { effectiveTimeZone } from "@/features/profile/model/profilePreferences";
import { useAuthStore } from "@/stores/authStore";
import { useProfileStore } from "@/stores/profileStore";
import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import {
  BodyFactRecords,
  type BodyFactRelatedLink,
} from "../../../../components/BodyFactRecords";
import { BodyHypotheses } from "../../../../components/BodyHypotheses";
import { episodeTitle } from "../../../../model/personRecordsTimeline";
import { EpisodeDetail } from "./EpisodeDetail";
import { EpisodesScene, ObservationsScene } from "./PersonRecordScenes";
import { PersonDataError } from "../PersonDataError";
import { PersonSubview } from "./PersonRecordsElements";

type Episode = BodyRecords["episodes"][number];

/** The five body-record scenes remain one navigation state with one authoritative data source. */
type View =
  | { kind: "timeline" }
  | { kind: "episodes"; editing?: boolean }
  | { kind: "episode"; id: string; from: "timeline" | "episodes" }
  | { kind: "hypotheses" }
  | { kind: "observations" };

export function PersonRecordsView({
  canEdit,
  now,
  addOpen = false,
  onAddOpenChange,
  onAddLabelChange,
}: {
  canEdit: boolean;
  now?: Date;
  /** The window header "+": a hypothesis in 我的猜测, a statement elsewhere. */
  addOpen?: boolean;
  onAddOpenChange?: (open: boolean) => void;
  /** Renames the header "+" while 我的猜测 is open. */
  onAddLabelChange?: (label: string | null) => void;
}) {
  const [view, setView] = useState<View>({ kind: "timeline" });
  // Statements are added from the timeline; other subviews hand back to it.
  useEffect(() => {
    if (!addOpen) return;
    if (view.kind !== "timeline" && view.kind !== "hypotheses")
      setView({ kind: "timeline" });
  }, [addOpen, view.kind]);
  const hypotheses = view.kind === "hypotheses";
  useEffect(() => {
    onAddLabelChange?.(hypotheses ? "添加我的猜测" : null);
    return () => onAddLabelChange?.(null);
  }, [hypotheses, onAddLabelChange]);
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
        error={records.isError}
        onRetry={() => void records.refetch()}
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
    return (
      <EpisodesScene
        canEdit={canEdit}
        editing={view.editing === true}
        data={data}
        loading={records.isPending}
        error={records.isError}
        onBack={back}
        onEdit={() => setView({ kind: "episodes", editing: true })}
        onSelectEpisode={(id) =>
          setView({ kind: "episode", id, from: "episodes" })
        }
      />
    );
  }
  if (view.kind === "hypotheses") {
    return (
      <PersonSubview title="我的猜测" testId="person-hypotheses" onBack={back}>
        <BodyHypotheses
          canEdit={canEdit}
          addRequested={addOpen}
          onAddHandled={() => onAddOpenChange?.(false)}
        />
      </PersonSubview>
    );
  }
  if (view.kind === "observations") {
    return (
      <ObservationsScene
        observations={data?.observations ?? []}
        dataLoaded={Boolean(data)}
        loading={records.isPending}
        error={records.isError}
        zone={zone}
        onBack={back}
        onRetry={() => void records.refetch()}
      />
    );
  }

  const currentEpisodes = (data?.episodes ?? []).filter(
    (episode) => episode.identity_status === "current",
  );
  const relatedNavigation = data ? (
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
      addOpen={addOpen}
      onAddOpenChange={onAddOpenChange}
      related={
        <>
          {records.isError ? (
            <PersonDataError
              title="相关记录暂时无法读取"
              message="身体陈述仍可查看，不能据此判断没有相关经历。"
              onRetry={() => void records.refetch()}
            />
          ) : null}
          {relatedNavigation}
        </>
      }
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
