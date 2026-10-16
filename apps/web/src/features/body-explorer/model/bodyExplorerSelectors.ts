import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import {
  BODY_REGION_IDS,
  isBodyRegionId,
  type BodyRegionId,
} from "./bodyRegionOntology";

export type BodyRegionVisualState = "none" | "observed";

export interface BodyRegionVisualSummary {
  regionId: BodyRegionId;
  visualState: BodyRegionVisualState;
  statementCount: number;
  episodeCount: number;
}

export function recordBodyRegions(record: {
  body_region_ids: string[];
}): BodyRegionId[] {
  return [...new Set(record.body_region_ids.filter(isBodyRegionId))];
}

export function selectBodyRegionVisualSummaries(
  records: BodyRecords | null | undefined,
): BodyRegionVisualSummary[] {
  return BODY_REGION_IDS.map((regionId) => {
    const statementCount =
      records?.facts.filter(
        (fact) =>
          fact.assertion_status === "asserted" &&
          recordBodyRegions(fact).includes(regionId),
      ).length ?? 0;
    const episodeCount =
      records?.episodes.filter(
        (episode) =>
          episode.identity_status === "current" &&
          recordBodyRegions(episode).includes(regionId),
      ).length ?? 0;
    // Episode identities are shown separately from statements. Neither implies
    // current symptoms, improvement, or Safety. Observations lack region links.
    return {
      regionId,
      visualState: statementCount || episodeCount ? "observed" : "none",
      statementCount,
      episodeCount,
    };
  });
}

export function selectBodyRegionVisualState(
  records: BodyRecords | null | undefined,
  regionId: BodyRegionId,
): BodyRegionVisualState {
  return (
    selectBodyRegionVisualSummaries(records).find(
      (summary) => summary.regionId === regionId,
    )?.visualState ?? "none"
  );
}
