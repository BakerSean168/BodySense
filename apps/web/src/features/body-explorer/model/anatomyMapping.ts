import mappingData from "../data/vanatome-region-map.v1.json";
import type { AtlasRegistryInventory, AtlasRegistryStructure, AnatomyStructureId } from "./anatomyTypes";
import { asAnatomyStructureId } from "./anatomyTypes";
import {
  BODY_REGION_IDS,
  BODY_REGION_ONTOLOGY_VERSION,
  getBodyRegionDefinition,
  isBodyRegionId,
  type BodyRegionId,
} from "./bodyRegionOntology";

export interface RegionAnatomyMapping {
  preferredFocusAnatomyId: string;
  anatomyIds: string[];
  verification: {
    registry_verified: boolean;
    visual_review: "pending" | "verified";
    basis: string;
    note: string;
  };
}

export interface VanatomeRegionMappingData {
  schemaVersion: number;
  mappingVersion: number;
  ontologyVersion: number;
  atlas: {
    provider: string;
    release: string;
    atlasId: string;
    catalogBuildId: string;
    upstreamCommit: string;
  };
  verificationPolicy: {
    registry: string;
    visual: string;
  };
  regions: Record<string, RegionAnatomyMapping>;
}

const rawMapping = mappingData as VanatomeRegionMappingData;

export const VANATOME_MAPPING_VERSION = rawMapping.mappingVersion;
export const VANATOME_ATLAS_RELEASE = rawMapping.atlas.release;

const mappingByRegion = new Map<BodyRegionId, RegionAnatomyMapping>(
  BODY_REGION_IDS.map((id) => [id, rawMapping.regions[id]]),
);

const reverseOwnership = new Map<string, BodyRegionId>();
for (const id of BODY_REGION_IDS) {
  const mapping = rawMapping.regions[id];
  for (const anatomyId of mapping?.anatomyIds ?? []) {
    const existing = reverseOwnership.get(anatomyId);
    if (existing && existing !== id) {
      throw new Error(`Anatomy ID ${anatomyId} has duplicate BodyRegion ownership: ${existing}, ${id}`);
    }
    reverseOwnership.set(anatomyId, id);
  }
}

export function getAnatomyMappingForRegion(id: BodyRegionId): RegionAnatomyMapping {
  const mapping = mappingByRegion.get(id);
  if (!mapping) throw new Error(`No Vanatome mapping for BodyRegionId ${id}`);
  return mapping;
}

export function getPreferredAnatomyIdForRegion(id: BodyRegionId): AnatomyStructureId {
  return asAnatomyStructureId(getAnatomyMappingForRegion(id).preferredFocusAnatomyId);
}

export function getAnatomyIdsForRegion(id: BodyRegionId): AnatomyStructureId[] {
  return getAnatomyMappingForRegion(id).anatomyIds.map(asAnatomyStructureId);
}

export function getBodyRegionForAnatomy(anatomyId: string): BodyRegionId | null {
  return reverseOwnership.get(anatomyId) ?? null;
}

export function resolveBodyRegionForAnatomy(
  anatomyId: string,
  registry?: Pick<AtlasRegistryInventory, "structures">,
): BodyRegionId | null {
  const exact = getBodyRegionForAnatomy(anatomyId);
  if (exact || !registry) return exact;

  const byId = new Map(registry.structures.map((structure) => [structure.anatomyId, structure]));
  const visited = new Set<string>();
  let current: AtlasRegistryStructure | undefined = byId.get(anatomyId);

  while (current?.parent && !visited.has(current.parent)) {
    visited.add(current.parent);
    const owner = getBodyRegionForAnatomy(current.parent);
    if (owner) return owner;
    current = byId.get(current.parent);
  }
  return null;
}

export function validateVanatomeRegionMapping(
  registry: AtlasRegistryInventory,
  mapping: VanatomeRegionMappingData = rawMapping,
): string[] {
  const errors: string[] = [];
  if (mapping.schemaVersion !== 1) errors.push("mapping schemaVersion must be 1");
  if (mapping.mappingVersion !== 1) errors.push("mappingVersion must be 1");
  if (mapping.ontologyVersion !== BODY_REGION_ONTOLOGY_VERSION) {
    errors.push(
      `mapping ontologyVersion ${mapping.ontologyVersion} does not match ontology ${BODY_REGION_ONTOLOGY_VERSION}`,
    );
  }
  if (mapping.atlas.provider !== registry.atlasProvider) {
    errors.push(`mapping atlas provider ${mapping.atlas.provider} does not match registry ${registry.atlasProvider}`);
  }
  if (mapping.atlas.release !== registry.atlasRelease) {
    errors.push(`mapping atlas release ${mapping.atlas.release} does not match registry ${registry.atlasRelease}`);
  }
  if (mapping.atlas.atlasId !== registry.atlasId) {
    errors.push(`mapping atlas ID ${mapping.atlas.atlasId} does not match registry ${registry.atlasId}`);
  }
  if (mapping.atlas.catalogBuildId !== registry.catalogBuildId) {
    errors.push(
      `mapping catalog build ${mapping.atlas.catalogBuildId} does not match registry ${registry.catalogBuildId}`,
    );
  }

  const registryById = new Map(registry.structures.map((structure) => [structure.anatomyId, structure]));
  const reverse = new Map<string, BodyRegionId>();

  for (const regionId of BODY_REGION_IDS) {
    const regionMapping = mapping.regions[regionId];
    if (!regionMapping) {
      errors.push(`missing mapping for ${regionId}`);
      continue;
    }
    if (regionMapping.anatomyIds.length === 0) {
      errors.push(`mapping ${regionId} must contain at least one anatomy ID`);
    }
    if (!regionMapping.anatomyIds.includes(regionMapping.preferredFocusAnatomyId)) {
      errors.push(`preferred focus ID for ${regionId} must be included in anatomyIds`);
    }
    if (!regionMapping.verification.registry_verified) {
      errors.push(`mapping ${regionId} is not registry_verified`);
    }

    const region = getBodyRegionDefinition(regionId);
    for (const anatomyId of regionMapping.anatomyIds) {
      const structure = registryById.get(anatomyId);
      if (!structure) {
        errors.push(`mapping ${regionId} references unknown atlas anatomy ID ${anatomyId}`);
        continue;
      }
      if (region.side) {
        if (!structure.laterality) {
          errors.push(`mapping ${regionId} uses anatomy ID ${anatomyId} without proven laterality`);
        } else if (structure.laterality !== region.side) {
          errors.push(
            `mapping ${regionId} uses ${structure.laterality}-sided anatomy ID ${anatomyId}`,
          );
        }
      }

      const existing = reverse.get(anatomyId);
      if (existing && existing !== regionId) {
        errors.push(`anatomy ID ${anatomyId} is owned by both ${existing} and ${regionId}`);
      } else {
        reverse.set(anatomyId, regionId);
      }
    }
  }

  for (const regionId of Object.keys(mapping.regions)) {
    if (!isBodyRegionId(regionId)) errors.push(`mapping contains unknown BodyRegionId ${regionId}`);
  }

  return errors;
}
