import type { BodyStateSnapshot } from "../types/consultation";

export type BodyZone = "head" | "neck" | "torso" | "pelvis" | "arms" | "legs";

export interface BodyZoneSummary {
  zone: BodyZone;
  label: string;
  count: number;
  trend: "worsening" | "improving" | "stable" | "unknown";
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

const zonePatterns: Array<[BodyZone, RegExp]> = [
  ["head", /头|颅|面|眼|耳|jaw|head|face/i],
  ["neck", /颈|肩|斜方|neck|shoulder/i],
  ["pelvis", /髋|臀|骨盆|骶|hip|glute|pelvis/i],
  ["arms", /手|腕|肘|臂|上肢|arm|elbow|wrist|hand/i],
  ["legs", /腿|膝|踝|足|脚|下肢|leg|knee|ankle|foot/i],
  ["torso", /胸|腹|背|腰|肋|脊柱|torso|chest|abdomen|back|lumbar|spine/i],
];

const trendRank = {
  unknown: 0,
  stable: 1,
  improving: 2,
  worsening: 3,
} as const;

type NormalizedTrend = keyof typeof trendRank;

function normalizeTrend(value: string | undefined): NormalizedTrend {
  return value === "worsening" || value === "improving" || value === "stable"
    ? value
    : "unknown";
}

export function bodyZoneFor(region: string, concernKey = ""): BodyZone {
  const source = `${region} ${concernKey}`.trim();
  for (const [zone, pattern] of zonePatterns) {
    if (pattern.test(source)) return zone;
  }
  return "torso";
}

export function selectBodyZoneSummaries(
  snapshot: BodyStateSnapshot | null | undefined,
): BodyZoneSummary[] {
  if (!snapshot) return [];

  const zones = new Map<BodyZone, BodyZoneSummary>();
  const add = (zone: BodyZone, region: string, trend: NormalizedTrend) => {
    const current = zones.get(zone) ?? {
      zone,
      label: zoneLabels[zone],
      count: 0,
      trend: "unknown" as NormalizedTrend,
      regions: [],
    };
    current.count += 1;
    if (region && !current.regions.includes(region))
      current.regions.push(region);
    if (trendRank[trend] > trendRank[current.trend]) current.trend = trend;
    zones.set(zone, current);
  };

  for (const fact of snapshot.facts ?? []) {
    if (fact.lifecycle_state !== "active") continue;
    const region = fact.body_region || fact.concern_key || "";
    add(
      bodyZoneFor(region, fact.concern_key),
      region,
      normalizeTrend(fact.trend),
    );
  }

  for (const observation of snapshot.observations ?? []) {
    if (observation.lifecycle_state !== "active") continue;
    const region = observation.body_region || observation.concern_key || "";
    add(bodyZoneFor(region, observation.concern_key), region, "unknown");
  }

  return [...zones.values()].sort((left, right) => right.count - left.count);
}
