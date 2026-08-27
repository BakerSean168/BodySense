import ontologyData from "../data/body-regions.v1.json";

export const BODY_REGION_IDS = [
  "head",
  "neck",
  "shoulder.left",
  "shoulder.right",
  "scapular.left",
  "scapular.right",
  "upper_arm.left",
  "upper_arm.right",
  "elbow.left",
  "elbow.right",
  "forearm.left",
  "forearm.right",
  "wrist.left",
  "wrist.right",
  "hand.left",
  "hand.right",
  "chest",
  "abdomen",
  "upper_back",
  "lower_back",
  "pelvis",
  "hip.left",
  "hip.right",
  "gluteal.left",
  "gluteal.right",
  "thigh.left",
  "thigh.right",
  "knee.left",
  "knee.right",
  "calf.left",
  "calf.right",
  "ankle.left",
  "ankle.right",
  "foot.left",
  "foot.right",
] as const;

export type BodyRegionId = (typeof BODY_REGION_IDS)[number];
export type BodyRegionSide = "left" | "right" | null;

export interface BodyRegionDefinition {
  id: BodyRegionId;
  labels: {
    "zh-CN": string;
    en: string;
  };
  parent: string;
  group: string;
  side: BodyRegionSide;
  aliases: string[];
}

export interface AmbiguousBodyRegionAlias {
  alias: string;
  candidates: BodyRegionId[];
  reason: string;
}

export type BodyRegionResolution =
  | {
      status: "resolved";
      id: BodyRegionId;
      normalizedInput: string;
      matchedBy: "canonical_id" | "alias";
    }
  | {
      status: "ambiguous";
      candidates: BodyRegionId[];
      normalizedInput: string;
      reason: string;
    }
  | {
      status: "unresolved";
      normalizedInput: string;
    };

interface RawBodyRegionOntology {
  schemaVersion: number;
  ontologyVersion: number;
  regions: Array<{
    id: string;
    labels: { "zh-CN": string; en: string };
    parent: string;
    group: string;
    side: string | null;
    aliases: string[];
  }>;
  ambiguousAliases: Array<{
    alias: string;
    candidates: string[];
    reason: string;
  }>;
}

const canonicalIdSet = new Set<string>(BODY_REGION_IDS);
const parentNodeSet = new Set([
  "body",
  "torso",
  "upper_limb.left",
  "upper_limb.right",
  "lower_limb.left",
  "lower_limb.right",
]);
const groupSet = new Set(["head", "neck", "torso", "upper_limb", "lower_limb"]);
const rawOntology = ontologyData as RawBodyRegionOntology;

export const BODY_REGION_ONTOLOGY_VERSION = rawOntology.ontologyVersion;

export function normalizeBodyRegionInput(value: string): string {
  return value.normalize("NFKC").trim().toLocaleLowerCase("en-US").replace(/\s+/g, " ");
}

export function isBodyRegionId(value: string): value is BodyRegionId {
  return canonicalIdSet.has(value);
}

export function parseBodyRegionId(value: string): BodyRegionId | null {
  return isBodyRegionId(value) ? value : null;
}

export function validateBodyRegionOntology(
  input: RawBodyRegionOntology = rawOntology,
): string[] {
  const errors: string[] = [];
  if (input.schemaVersion !== 1) errors.push("ontology schemaVersion must be 1");
  if (input.ontologyVersion !== 1) errors.push("ontologyVersion must be 1");

  const seenIds = new Set<string>();
  const deterministicAliases = new Map<string, string>();

  for (const region of input.regions) {
    if (!isBodyRegionId(region.id)) {
      errors.push(`unknown canonical region id: ${region.id}`);
      continue;
    }
    if (seenIds.has(region.id)) errors.push(`duplicate canonical region id: ${region.id}`);
    seenIds.add(region.id);

    const expectedSide = region.id.endsWith(".left")
      ? "left"
      : region.id.endsWith(".right")
        ? "right"
        : null;
    if (region.side !== expectedSide) {
      errors.push(`region ${region.id} has side ${String(region.side)}, expected ${String(expectedSide)}`);
    }
    if (!region.labels["zh-CN"] || !region.labels.en) {
      errors.push(`region ${region.id} is missing labels`);
    }
    if (!region.parent || !region.group) {
      errors.push(`region ${region.id} is missing parent/group`);
    } else {
      if (!parentNodeSet.has(region.parent)) {
        errors.push(`region ${region.id} references unknown parent node ${region.parent}`);
      }
      if (!groupSet.has(region.group)) {
        errors.push(`region ${region.id} references unknown group ${region.group}`);
      }
    }

    for (const alias of region.aliases) {
      const normalized = normalizeBodyRegionInput(alias);
      if (!normalized) {
        errors.push(`region ${region.id} has an empty alias`);
        continue;
      }
      const owner = deterministicAliases.get(normalized);
      if (owner && owner !== region.id) {
        errors.push(`deterministic alias ${alias} is owned by both ${owner} and ${region.id}`);
      } else {
        deterministicAliases.set(normalized, region.id);
      }
    }
  }

  for (const expectedId of BODY_REGION_IDS) {
    if (!seenIds.has(expectedId)) errors.push(`missing canonical region id: ${expectedId}`);
  }
  if (seenIds.size !== BODY_REGION_IDS.length) {
    errors.push(`expected ${BODY_REGION_IDS.length} canonical regions, found ${seenIds.size}`);
  }

  const seenAmbiguous = new Set<string>();
  for (const entry of input.ambiguousAliases) {
    const normalized = normalizeBodyRegionInput(entry.alias);
    if (!normalized) {
      errors.push("ambiguous alias cannot be empty");
      continue;
    }
    if (seenAmbiguous.has(normalized)) {
      errors.push(`duplicate ambiguous alias: ${entry.alias}`);
    }
    seenAmbiguous.add(normalized);
    if (deterministicAliases.has(normalized)) {
      errors.push(`alias ${entry.alias} cannot be both deterministic and ambiguous`);
    }
    if (entry.candidates.length < 2) {
      errors.push(`ambiguous alias ${entry.alias} must have at least two candidates`);
    }
    for (const candidate of entry.candidates) {
      if (!isBodyRegionId(candidate)) {
        errors.push(`ambiguous alias ${entry.alias} references unknown ${candidate}`);
      }
    }
  }

  return errors;
}

const ontologyErrors = validateBodyRegionOntology();
if (ontologyErrors.length > 0) {
  throw new Error(`Invalid BodyRegionOntology v1:\n${ontologyErrors.join("\n")}`);
}

export const bodyRegionDefinitions = rawOntology.regions as BodyRegionDefinition[];
export const ambiguousBodyRegionAliases = rawOntology.ambiguousAliases as AmbiguousBodyRegionAlias[];

const definitionById = new Map<BodyRegionId, BodyRegionDefinition>(
  bodyRegionDefinitions.map((definition) => [definition.id, definition]),
);
const aliasToId = new Map<string, BodyRegionId>();
for (const definition of bodyRegionDefinitions) {
  for (const alias of definition.aliases) {
    aliasToId.set(normalizeBodyRegionInput(alias), definition.id);
  }
}
const ambiguousByAlias = new Map<string, AmbiguousBodyRegionAlias>(
  ambiguousBodyRegionAliases.map((entry) => [normalizeBodyRegionInput(entry.alias), entry]),
);

export function getBodyRegionDefinition(id: BodyRegionId): BodyRegionDefinition {
  const definition = definitionById.get(id);
  if (!definition) throw new Error(`Unknown BodyRegionId: ${id}`);
  return definition;
}

export function aliasesForBodyRegion(id: BodyRegionId): readonly string[] {
  return getBodyRegionDefinition(id).aliases;
}

export function resolveBodyRegionInput(value: string): BodyRegionResolution {
  const raw = value.trim();
  if (isBodyRegionId(raw)) {
    return {
      status: "resolved",
      id: raw,
      normalizedInput: raw,
      matchedBy: "canonical_id",
    };
  }

  const normalizedInput = normalizeBodyRegionInput(value);
  const deterministic = aliasToId.get(normalizedInput);
  if (deterministic) {
    return {
      status: "resolved",
      id: deterministic,
      normalizedInput,
      matchedBy: "alias",
    };
  }

  const ambiguous = ambiguousByAlias.get(normalizedInput);
  if (ambiguous) {
    return {
      status: "ambiguous",
      candidates: [...ambiguous.candidates],
      normalizedInput,
      reason: ambiguous.reason,
    };
  }

  return { status: "unresolved", normalizedInput };
}
