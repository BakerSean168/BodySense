import type {
  BodyStateFact,
  BodyStateObservation,
  BodyStateSnapshot,
} from "@/features/consultation/types/consultation";
import {
  BODY_REGION_IDS,
  isBodyRegionId,
  resolveBodyRegionInput,
  type BodyRegionId,
} from "./bodyRegionOntology";

export type BodyRegionVisualState =
  | "none"
  | "observed"
  | "stable"
  | "improving"
  | "worsening"
  | "safety_review";

export interface BodyRegionVisualSummary {
  regionId: BodyRegionId;
  visualState: BodyRegionVisualState;
  activeFactCount: number;
  confirmedObservationCount: number;
  pendingObservationCount: number;
}

type RegionAware = {
  body_region_id?: string | null;
  body_region?: string;
};

const visualStateRank: Record<BodyRegionVisualState, number> = {
  none: 0,
  observed: 1,
  stable: 2,
  improving: 3,
  worsening: 4,
  safety_review: 5,
};

function mergeVisualState(
  current: BodyRegionVisualState,
  candidate: BodyRegionVisualState,
): BodyRegionVisualState {
  return visualStateRank[candidate] > visualStateRank[current]
    ? candidate
    : current;
}

export function resolveRecordBodyRegion(
  record: RegionAware,
): BodyRegionId | null {
  if (record.body_region_id != null) {
    return isBodyRegionId(record.body_region_id) ? record.body_region_id : null;
  }
  if (!record.body_region) return null;

  const resolution = resolveBodyRegionInput(record.body_region);
  return resolution.status === "resolved" ? resolution.id : null;
}

function factVisualState(fact: BodyStateFact): BodyRegionVisualState {
  if (fact.kind === "red_flags" || fact.kind === "safety_finding") {
    return "safety_review";
  }
  if (fact.trend === "worsening") return "worsening";
  if (fact.trend === "improving") return "improving";
  if (fact.trend === "stable") return "stable";
  return "observed";
}

function observationIsActive(observation: BodyStateObservation): boolean {
  return observation.lifecycle_state === "active";
}

function explicitSafetyRegions(snapshot: BodyStateSnapshot): BodyRegionId[] {
  const safety = snapshot.safety_state;
  if (
    safety.has_red_flags !== true ||
    (safety.status !== "requires_review" && safety.status !== "active")
  ) {
    return [];
  }

  const regionIds = safety.body_region_ids;
  if (!Array.isArray(regionIds)) return [];
  return regionIds.filter(
    (value): value is BodyRegionId =>
      typeof value === "string" && isBodyRegionId(value),
  );
}

export function selectBodyRegionVisualSummaries(
  snapshot: BodyStateSnapshot | null | undefined,
): BodyRegionVisualSummary[] {
  const summaries = new Map<BodyRegionId, BodyRegionVisualSummary>(
    BODY_REGION_IDS.map((regionId) => [
      regionId,
      {
        regionId,
        visualState: "none",
        activeFactCount: 0,
        confirmedObservationCount: 0,
        pendingObservationCount: 0,
      },
    ]),
  );

  if (!snapshot) return [...summaries.values()];

  for (const fact of snapshot.facts ?? []) {
    if (fact.lifecycle_state !== "active" || fact.review_state === "rejected") {
      continue;
    }
    const regionId = resolveRecordBodyRegion(fact);
    if (!regionId) continue;
    const summary = summaries.get(regionId);
    if (!summary) continue;
    summary.activeFactCount += 1;
    summary.visualState = mergeVisualState(
      summary.visualState,
      factVisualState(fact),
    );
  }

  const seenObservationIds = new Set<string>();
  for (const observation of snapshot.observations ?? []) {
    if (
      !observationIsActive(observation) ||
      observation.review_state !== "confirmed"
    ) {
      continue;
    }
    const regionId = resolveRecordBodyRegion(observation);
    if (!regionId) continue;
    const summary = summaries.get(regionId);
    if (!summary) continue;
    summary.confirmedObservationCount += 1;
    summary.visualState = mergeVisualState(summary.visualState, "observed");
    seenObservationIds.add(observation.id);
  }

  for (const observation of snapshot.pending_observations ?? []) {
    if (
      seenObservationIds.has(observation.id) ||
      !observationIsActive(observation) ||
      observation.review_state === "rejected"
    ) {
      continue;
    }
    const regionId = resolveRecordBodyRegion(observation);
    if (!regionId) continue;
    const summary = summaries.get(regionId);
    if (!summary) continue;
    summary.pendingObservationCount += 1;
    summary.visualState = mergeVisualState(summary.visualState, "observed");
  }

  for (const regionId of explicitSafetyRegions(snapshot)) {
    const summary = summaries.get(regionId);
    if (summary) summary.visualState = "safety_review";
  }

  return [...summaries.values()];
}

export function selectBodyRegionVisualState(
  snapshot: BodyStateSnapshot | null | undefined,
  regionId: BodyRegionId,
): BodyRegionVisualState {
  return (
    selectBodyRegionVisualSummaries(snapshot).find(
      (summary) => summary.regionId === regionId,
    )?.visualState ?? "none"
  );
}
