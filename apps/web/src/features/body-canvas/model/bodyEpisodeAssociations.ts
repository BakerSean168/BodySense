import { assertNever } from "@/lib/assertNever";
import type {
  BodyFactRecord,
  BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import type {
  BodyEpisode,
  BodyEpisodePage,
} from "@/features/profile/services/bodyEpisodeService";

export type EpisodeAssociationMode = "create" | "link";

export interface EpisodeSourceOption {
  id: string;
  statement: string;
  symptomType: string;
  presence: "present" | "absent";
}

export interface EpisodeAssociationProjection {
  /** There is no authoritative revision until both owner-scoped reads agree. */
  revision: number | null;
  stale: boolean;
  facts: readonly EpisodeSourceOption[];
  episodes: readonly BodyEpisode[];
  linkedFactIds: ReadonlySet<string>;
  factsHaveMore: boolean;
  episodesHaveMore: boolean;
}

/** Bounded functional projection. Episode state still belongs to Go, never to React. */
export function projectEpisodeAssociation(
  owner: string | undefined,
  factPages: readonly BodyFactRecordsPage[] | undefined,
  episodePages: readonly BodyEpisodePage[] | undefined,
): EpisodeAssociationProjection {
  const headFacts = factPages?.[0];
  const headEpisodes = episodePages?.[0];
  const revision =
    owner &&
    headFacts &&
    headEpisodes &&
    headFacts.user_id === owner &&
    headEpisodes.user_id === owner &&
    headFacts.current_revision === headEpisodes.current_revision
      ? headFacts.current_revision
      : null;
  const pages = [...(factPages ?? []), ...(episodePages ?? [])];
  const stale =
    !!headFacts &&
    !!headEpisodes &&
    (revision === null ||
      pages.some(
        (page) => page.user_id !== owner || page.current_revision !== revision,
      ));
  if (revision === null || stale) {
    return {
      revision: null,
      stale,
      facts: [],
      episodes: [],
      linkedFactIds: new Set(),
      factsHaveMore: false,
      episodesHaveMore: false,
    };
  }

  const seen = new Set<string>();
  const facts: EpisodeSourceOption[] = [];
  for (const fact of (factPages ?? []).flatMap((page) => page.facts)) {
    const option = episodeSourceOption(fact);
    if (!option || seen.has(option.id)) continue;
    seen.add(option.id);
    facts.push(option);
  }
  const episodes = (episodePages ?? []).flatMap((page) => page.episodes);
  const linkedFactIds = new Set(
    episodes.flatMap((episode) => episode.links.map((link) => link.fact_id)),
  );

  return {
    revision,
    stale: false,
    facts,
    episodes,
    linkedFactIds,
    factsHaveMore: Boolean(factPages?.at(-1)?.has_more),
    episodesHaveMore: Boolean(episodePages?.at(-1)?.has_more),
  };
}

export function episodeSourceOption(
  fact: BodyFactRecord,
): EpisodeSourceOption | null {
  if (
    fact.subject !== "self" ||
    fact.kind !== "symptom" ||
    fact.assertion_status !== "asserted"
  )
    return null;
  const details: unknown = fact.details;
  if (!details || typeof details !== "object" || Array.isArray(details))
    return null;
  const data = details as Record<string, unknown>;
  if (
    typeof data.symptom_type !== "string" ||
    !data.symptom_type.trim() ||
    (data.presence !== "present" && data.presence !== "absent")
  )
    return null;
  return {
    id: fact.id,
    statement: fact.statement,
    symptomType: data.symptom_type,
    presence: data.presence,
  };
}

/** Presentation stays conservative when a source-qualified projection is incomplete. */
export function episodeDisplayState(
  episode: BodyEpisode,
): BodyEpisode["projection"]["state"] {
  const { state, reason, basis_fact_ids } = episode.projection;
  if (episode.identity_status !== "current" || !basis_fact_ids.length)
    return "unknown";
  if (state === "ongoing" && reason === "explicit_current_report")
    return "ongoing";
  if (state === "ended" && reason === "explicit_resolution") return "ended";
  return "unknown";
}

export function episodeStateLabel(episode: BodyEpisode): string {
  if (episode.identity_status === "superseded") return "经历身份已被替代";
  if (episode.identity_status === "invalidated") return "经历身份已失效";
  const displayState = episodeDisplayState(episode);
  switch (displayState) {
    case "ongoing":
      return "截至明确报告时仍在持续";
    case "ended":
      return "有明确的结束报告";
    case "unknown":
      return "持续状态尚未确认";
  }
  return assertNever(displayState);
}
