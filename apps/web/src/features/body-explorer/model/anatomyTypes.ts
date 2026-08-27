export type AnatomyStructureId = string & {
  readonly __brand: "AnatomyStructureId";
};

export type AnatomyLaterality = "left" | "right" | null;

export interface AtlasRegistryStructure {
  anatomyId: string;
  name: string;
  kind: string | null;
  system: string | null;
  layer: string | null;
  parent: string | null;
  laterality: AnatomyLaterality;
  lateralityEvidence: string | null;
  focus: [number, number, number] | null;
  selectable: boolean;
  geometry: {
    directMappedNodeCount: number;
    hasDirectGeometry: boolean;
  };
}

export interface AtlasRegistryInventory {
  schemaVersion: number;
  atlasProvider: string;
  atlasRelease: string;
  atlasId: string;
  catalogBuildId: string;
  bundleId: string;
  summary: {
    structureCount: number;
    directGeometryStructureCount: number;
    mappedNodeCount: number;
    fullBodyNodeCount: number;
  };
  structures: AtlasRegistryStructure[];
}

export function asAnatomyStructureId(value: string): AnatomyStructureId {
  return value as AnatomyStructureId;
}
