import { describe, expect, it } from "vitest";
import type { BodyStateSnapshot } from "../types/consultation";
import { bodyZoneFor, selectBodyZoneSummaries } from "./bodyMap";

function snapshot(): BodyStateSnapshot {
  return {
    user_id: "user-1",
    current_revision: 3,
    safety_state: {},
    facts: [
      {
        id: "fact-1",
        kind: "discomfort",
        body_region: "颈肩",
        value: "酸胀",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "worsening",
        updated_revision: 3,
      },
    ],
    observations: [],
    pending_observations: [],
    hypotheses: [],
    recent_revisions: [],
  };
}

describe("body map projection", () => {
  it("normalizes natural-language body regions", () => {
    expect(bodyZoneFor("左侧肩部")).toBe("neck");
    expect(bodyZoneFor("膝盖")).toBe("legs");
    expect(bodyZoneFor("未知区域")).toBe("torso");
  });

  it("projects current facts into accessible body-zone summaries", () => {
    expect(selectBodyZoneSummaries(snapshot())).toEqual([
      expect.objectContaining({
        zone: "neck",
        label: "颈肩",
        count: 1,
        trend: "worsening",
      }),
    ]);
  });
});
