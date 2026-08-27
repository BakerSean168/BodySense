import { describe, expect, it } from "vitest";
import registryData from "../data/vanatome-1.4.0-registry.generated.json";
import type { AtlasRegistryInventory } from "./anatomyTypes";
import { validateAtlasRegistryInventory } from "./atlasRegistry";

const registry = registryData as unknown as AtlasRegistryInventory;

describe("Vanatome 1.4.0 registry inventory", () => {
  it("matches the pinned full-body registry and geometry evidence", () => {
    expect(validateAtlasRegistryInventory(registry)).toEqual([]);
    expect(registry.atlasRelease).toBe("1.4.0");
    expect(registry.catalogBuildId).toBe("994e6cc8ffbb212e");
    expect(registry.summary).toEqual({
      structureCount: 807,
      directGeometryStructureCount: 749,
      mappedNodeCount: 984,
      fullBodyNodeCount: 984,
    });
  });

  it("preserves hierarchy, focus, and explicit laterality evidence", () => {
    const rightClavicle = registry.structures.find(
      (structure) =>
        structure.anatomyId === "appendicular-skeleton-clavicle-right",
    );
    expect(rightClavicle).toMatchObject({
      name: "Clavicle.r",
      system: "skeletal",
      layer: "skeletal",
      parent: "appendicular-skeleton",
      laterality: "right",
      geometry: {
        directMappedNodeCount: 1,
        hasDirectGeometry: true,
      },
    });
    expect(rightClavicle?.lateralityEvidence).toContain("anatomy_id");
    expect(rightClavicle?.focus).toHaveLength(3);
  });
});
