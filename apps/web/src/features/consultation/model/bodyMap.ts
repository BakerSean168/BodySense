import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { recordBodyRegions } from "@/features/body-explorer/model/bodyExplorerSelectors";
import { getBodyRegionDefinition } from "@/features/body-explorer/model/bodyRegionOntology";

export type BodyZone = "head" | "neck" | "torso" | "pelvis" | "arms" | "legs";
export interface BodyZoneSummary {
  zone: BodyZone;
  label: string;
  count: number;
  regions: string[];
}
const zoneLabels: Record<BodyZone, string> = {
  head: "头面部",
  neck: "颈肩",
  torso: "躯干 / 背腰",
  pelvis: "骨盆 / 髋臀",
  arms: "上肢",
  legs: "下肢",
};

export function bodyZoneFor(region: string): BodyZone {
  if (/head|face|jaw/.test(region)) return "head";
  if (/neck|shoulder/.test(region)) return "neck";
  if (/hip|glute|pelvis/.test(region)) return "pelvis";
  if (/arm|elbow|wrist|hand/.test(region)) return "arms";
  if (/leg|knee|ankle|foot|thigh|calf/.test(region)) return "legs";
  return "torso";
}

export function selectBodyZoneSummaries(
  records: BodyRecords | null | undefined,
): BodyZoneSummary[] {
  const zones = new Map<BodyZone, BodyZoneSummary>();
  for (const fact of records?.facts ?? []) {
    if (fact.assertion_status !== "asserted") continue;
    const seenZones = new Set<BodyZone>();
    for (const regionId of recordBodyRegions(fact)) {
      const zone = bodyZoneFor(regionId);
      const summary = zones.get(zone) ?? {
        zone,
        label: zoneLabels[zone],
        count: 0,
        regions: [],
      };
      if (!seenZones.has(zone)) summary.count += 1;
      seenZones.add(zone);
      const label = getBodyRegionDefinition(regionId).labels["zh-CN"];
      if (!summary.regions.includes(label)) summary.regions.push(label);
      zones.set(zone, summary);
    }
  }
  return [...zones.values()].toSorted(
    (left, right) => right.count - left.count,
  );
}
