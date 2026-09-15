import { z } from "zod";
import registryData from "../data/vanatome-1.4.0-registry.generated.json";
import type {
  AtlasRegistryInventory,
  AtlasRegistryStructure,
} from "./anatomyTypes";

const atlasRegistryStructureSchema = z
  .object({
    anatomyId: z.string().min(1),
    name: z.string().min(1),
    kind: z.string().nullable(),
    system: z.string().nullable(),
    layer: z.string().nullable(),
    parent: z.string().nullable(),
    laterality: z.enum(["left", "right"]).nullable(),
    lateralityEvidence: z.string().nullable(),
    focus: z.tuple([z.number(), z.number(), z.number()]).nullable(),
    selectable: z.boolean(),
    geometry: z
      .object({
        directMappedNodeCount: z.number().int().nonnegative(),
        hasDirectGeometry: z.boolean(),
      })
      .strict(),
  })
  .strict();

const atlasRegistryInventorySchema = z
  .object({
    schemaVersion: z.number().int(),
    atlasProvider: z.string().min(1),
    atlasRelease: z.string().min(1),
    atlasId: z.string().min(1),
    catalogBuildId: z.string().min(1),
    bundleId: z.string().min(1),
    source: z
      .object({
        officialCatalogUrl: z.url(),
        upstreamRepository: z.url(),
        upstreamCommit: z.string().regex(/^[0-9a-f]{40}$/),
        catalogFixture: z.url(),
        metadataFixture: z.url(),
        catalogSha256: z.string().regex(/^[0-9a-f]{64}$/),
        metadataSha256: z.string().regex(/^[0-9a-f]{64}$/),
      })
      .strict(),
    summary: z
      .object({
        structureCount: z.number().int().nonnegative(),
        directGeometryStructureCount: z.number().int().nonnegative(),
        mappedNodeCount: z.number().int().nonnegative(),
        fullBodyNodeCount: z.number().int().nonnegative(),
      })
      .strict(),
    structures: z.array(atlasRegistryStructureSchema),
  })
  .strict();

export function parseAtlasRegistryInventory(
  input: unknown,
): AtlasRegistryInventory {
  return atlasRegistryInventorySchema.parse(input);
}

const pinnedRegistry = parseAtlasRegistryInventory(registryData);
const pinnedRegistryErrors = validateAtlasRegistryInventory(pinnedRegistry);
if (pinnedRegistryErrors.length > 0) {
  throw new Error(
    `Invalid pinned atlas registry: ${pinnedRegistryErrors.join("; ")}`,
  );
}
const pinnedStructureIndex = new Map(
  pinnedRegistry.structures.map((structure) => [
    structure.anatomyId,
    structure,
  ]),
);

export function getPinnedAtlasStructure(
  anatomyId: string,
): AtlasRegistryStructure | null {
  return pinnedStructureIndex.get(anatomyId) ?? null;
}

export function validateAtlasRegistryInventory(
  registry: AtlasRegistryInventory,
): string[] {
  const errors: string[] = [];

  if (registry.schemaVersion !== 1)
    errors.push("registry schemaVersion must be 1");
  if (registry.atlasProvider !== "vanatome") {
    errors.push(
      `registry atlasProvider must be vanatome, got ${registry.atlasProvider}`,
    );
  }
  if (registry.atlasRelease !== "1.4.0") {
    errors.push(
      `registry atlasRelease must be 1.4.0, got ${registry.atlasRelease}`,
    );
  }

  const ids = new Set<string>();
  for (const structure of registry.structures) {
    if (!structure.anatomyId)
      errors.push("registry structure has an empty anatomyId");
    if (ids.has(structure.anatomyId)) {
      errors.push(`duplicate registry anatomyId: ${structure.anatomyId}`);
    }
    ids.add(structure.anatomyId);

    if (!structure.name)
      errors.push(
        `registry structure ${structure.anatomyId} is missing a name`,
      );
    if (structure.focus && structure.focus.length !== 3) {
      errors.push(
        `registry structure ${structure.anatomyId} has invalid focus metadata`,
      );
    }
    if (structure.geometry.directMappedNodeCount < 0) {
      errors.push(
        `registry structure ${structure.anatomyId} has a negative mapped node count`,
      );
    }
    if (
      structure.geometry.hasDirectGeometry !==
      structure.geometry.directMappedNodeCount > 0
    ) {
      errors.push(
        `registry structure ${structure.anatomyId} has inconsistent geometry evidence`,
      );
    }
    if (structure.laterality && !structure.lateralityEvidence) {
      errors.push(
        `registry structure ${structure.anatomyId} lacks laterality evidence`,
      );
    }
  }

  for (const structure of registry.structures) {
    if (structure.parent && !ids.has(structure.parent)) {
      errors.push(
        `registry structure ${structure.anatomyId} references unknown parent ${structure.parent}`,
      );
    }
  }

  const directGeometryStructureCount = registry.structures.filter(
    (structure) => structure.geometry.hasDirectGeometry,
  ).length;
  const mappedNodeCount = registry.structures.reduce(
    (sum, structure) => sum + structure.geometry.directMappedNodeCount,
    0,
  );

  if (registry.summary.structureCount !== registry.structures.length) {
    errors.push(
      `registry summary structureCount ${registry.summary.structureCount} does not match ${registry.structures.length}`,
    );
  }
  if (
    registry.summary.directGeometryStructureCount !==
    directGeometryStructureCount
  ) {
    errors.push(
      `registry summary directGeometryStructureCount ${registry.summary.directGeometryStructureCount} does not match ${directGeometryStructureCount}`,
    );
  }
  if (registry.summary.mappedNodeCount !== mappedNodeCount) {
    errors.push(
      `registry summary mappedNodeCount ${registry.summary.mappedNodeCount} does not match ${mappedNodeCount}`,
    );
  }
  if (registry.summary.fullBodyNodeCount !== mappedNodeCount) {
    errors.push(
      `registry fullBodyNodeCount ${registry.summary.fullBodyNodeCount} does not match mapped node evidence ${mappedNodeCount}`,
    );
  }

  return errors;
}
