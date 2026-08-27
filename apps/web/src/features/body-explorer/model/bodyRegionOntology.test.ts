import { describe, expect, it } from "vitest";
import {
  BODY_REGION_IDS,
  bodyRegionDefinitions,
  getBodyRegionDefinition,
  parseBodyRegionId,
  resolveBodyRegionInput,
  validateBodyRegionOntology,
} from "./bodyRegionOntology";

describe("BodyRegionOntology v1", () => {
  it("contains the complete canonical region vocabulary with explicit laterality", () => {
    expect(validateBodyRegionOntology()).toEqual([]);
    expect(BODY_REGION_IDS).toHaveLength(35);
    expect(bodyRegionDefinitions).toHaveLength(35);
    expect(getBodyRegionDefinition("shoulder.left").side).toBe("left");
    expect(getBodyRegionDefinition("shoulder.right").side).toBe("right");
    expect(getBodyRegionDefinition("lower_back").side).toBeNull();
  });

  it("parses canonical IDs without depending on atlas identity", () => {
    expect(parseBodyRegionId("shoulder.right")).toBe("shoulder.right");
    expect(parseBodyRegionId("appendicular-skeleton-clavicle-right")).toBeNull();
  });

  it("resolves deterministic side-specific aliases", () => {
    expect(resolveBodyRegionInput("右肩")).toEqual({
      status: "resolved",
      id: "shoulder.right",
      normalizedInput: "右肩",
      matchedBy: "alias",
    });
    expect(resolveBodyRegionInput(" 左膝 ")).toEqual({
      status: "resolved",
      id: "knee.left",
      normalizedInput: "左膝",
      matchedBy: "alias",
    });
    expect(resolveBodyRegionInput("RIGHT SHOULDER")).toMatchObject({
      status: "resolved",
      id: "shoulder.right",
      matchedBy: "alias",
    });
    expect(resolveBodyRegionInput("腰")).toMatchObject({
      status: "resolved",
      id: "lower_back",
    });
  });

  it("returns explicit ambiguity instead of dropping laterality", () => {
    expect(resolveBodyRegionInput("肩膀")).toEqual({
      status: "ambiguous",
      candidates: ["shoulder.left", "shoulder.right"],
      normalizedInput: "肩膀",
      reason: "laterality_or_region_scope_missing",
    });
    expect(resolveBodyRegionInput("肩颈")).toEqual({
      status: "ambiguous",
      candidates: ["neck", "shoulder.left", "shoulder.right"],
      normalizedInput: "肩颈",
      reason: "cross_region_scope",
    });
  });

  it("does not fuzzy-guess arbitrary free text into a durable ID", () => {
    expect(resolveBodyRegionInput("右肩抬手的时候痛")).toEqual({
      status: "unresolved",
      normalizedInput: "右肩抬手的时候痛",
    });
    expect(resolveBodyRegionInput("肩膀附近有点不舒服")).toEqual({
      status: "unresolved",
      normalizedInput: "肩膀附近有点不舒服",
    });
  });
});
