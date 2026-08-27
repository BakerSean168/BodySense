import { describe, expect, it } from "vitest";
import mappingData from "../data/vanatome-region-map.v1.json";
import registryData from "../data/vanatome-1.4.0-registry.generated.json";
import type { AtlasRegistryInventory } from "./anatomyTypes";
import {
  getAnatomyIdsForRegion,
  getBodyRegionForAnatomy,
  getPreferredAnatomyIdForRegion,
  resolveBodyRegionForAnatomy,
  validateVanatomeRegionMapping,
  type VanatomeRegionMappingData,
} from "./anatomyMapping";
import { BODY_REGION_IDS } from "./bodyRegionOntology";

const registry = registryData as unknown as AtlasRegistryInventory;
const mapping = mappingData as unknown as VanatomeRegionMappingData;

describe("Vanatome 1.4.0 BodyRegion mapping", () => {
  it("validates every curated anatomy ID against the pinned registry", () => {
    expect(validateVanatomeRegionMapping(registry)).toEqual([]);
    expect(Object.keys(mapping.regions)).toHaveLength(BODY_REGION_IDS.length);

    const mapped = BODY_REGION_IDS.flatMap((regionId) =>
      getAnatomyIdsForRegion(regionId),
    );
    expect(mapped).toHaveLength(65);
    expect(new Set(mapped).size).toBe(65);
  });

  it("keeps bilateral reverse ownership deterministic", () => {
    expect(
      getBodyRegionForAnatomy("appendicular-skeleton-clavicle-left"),
    ).toBe("shoulder.left");
    expect(
      getBodyRegionForAnatomy("appendicular-skeleton-clavicle-right"),
    ).toBe("shoulder.right");
    expect(getPreferredAnatomyIdForRegion("knee.left")).toBe(
      "appendicular-skeleton-patella-left",
    );
  });

  it("can inherit reverse ownership through the verified atlas parent hierarchy", () => {
    expect(
      resolveBodyRegionForAnatomy(
        "neck-muscles-longus-colli-muscle-right",
        registry,
      ),
    ).toBe("neck");
    expect(resolveBodyRegionForAnatomy("heart-left-atrium", registry)).toBeNull();
  });

  it("fails validation when a mapping invents an anatomy ID", () => {
    const invalid = structuredClone(mapping);
    invalid.regions.head.anatomyIds.push("invented-anatomy-id");

    expect(validateVanatomeRegionMapping(registry, invalid)).toContain(
      "mapping head references unknown atlas anatomy ID invented-anatomy-id",
    );
  });

  it("fails validation on atlas-version drift and duplicate reverse ownership", () => {
    const invalid = structuredClone(mapping);
    invalid.atlas.release = "1.5.0";
    invalid.regions["shoulder.right"].anatomyIds.push(
      "appendicular-skeleton-clavicle-left",
    );

    const errors = validateVanatomeRegionMapping(registry, invalid);
    expect(errors).toContain(
      "mapping atlas release 1.5.0 does not match registry 1.4.0",
    );
    expect(errors.some((error) => error.includes("owned by both"))).toBe(true);
    expect(
      errors.some((error) => error.includes("left-sided anatomy ID")),
    ).toBe(true);
  });
});
