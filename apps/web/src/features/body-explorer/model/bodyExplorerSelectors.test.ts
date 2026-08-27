import { describe, expect, it } from "vitest";
import type { BodyStateSnapshot } from "@/features/consultation/types/consultation";
import {
  resolveRecordBodyRegion,
  selectBodyRegionVisualState,
  selectBodyRegionVisualSummaries,
} from "./bodyExplorerSelectors";

function snapshot(): BodyStateSnapshot {
  return {
    user_id: "user-1",
    current_revision: 9,
    safety_state: {},
    facts: [],
    observations: [],
    pending_observations: [],
    hypotheses: [],
    recent_revisions: [],
  };
}

describe("Body Explorer BodyState selectors", () => {
  it("prefers explicit canonical body_region_id and never fuzzy-guesses raw text", () => {
    expect(
      resolveRecordBodyRegion({
        body_region_id: "shoulder.left",
        body_region: "肩膀",
      }),
    ).toBe("shoulder.left");
    expect(resolveRecordBodyRegion({ body_region: "右肩" })).toBe(
      "shoulder.right",
    );
    expect(
      resolveRecordBodyRegion({
        body_region_id: "not-a-region",
        body_region: "右肩",
      }),
    ).toBeNull();
    expect(resolveRecordBodyRegion({ body_region: "肩膀" })).toBeNull();
    expect(resolveRecordBodyRegion({ body_region: "右肩抬手时疼痛" })).toBeNull();
  });

  it("derives visual precedence safety > worsening > improving > stable > observed > none", () => {
    const state = snapshot();
    state.facts = [
      {
        id: "f-observed",
        kind: "discomfort",
        body_region: "右肩",
        value: "酸胀",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "unknown",
        updated_revision: 9,
      },
      {
        id: "f-stable",
        kind: "discomfort",
        body_region: "右肩",
        value: "稳定",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "stable",
        updated_revision: 9,
      },
      {
        id: "f-improving",
        kind: "limitation",
        body_region: "右肩",
        value: "活动范围改善",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "improving",
        updated_revision: 9,
      },
      {
        id: "f-worsening",
        kind: "discomfort",
        body_region: "右肩",
        value: "夜间疼痛加重",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "worsening",
        updated_revision: 9,
      },
      {
        id: "f-safety",
        kind: "red_flags",
        body_region: "右肩",
        value: "needs review",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "active",
        trend: "unknown",
        updated_revision: 9,
      },
    ];

    expect(selectBodyRegionVisualState(state, "shoulder.right")).toBe(
      "safety_review",
    );
  });

  it("uses confirmed and pending observations as observed without painting hypotheses", () => {
    const state = snapshot();
    state.observations = [
      {
        id: "o1",
        kind: "range_of_motion",
        body_region: "左膝",
        value: { label: "ROM" },
        review_state: "confirmed",
        lifecycle_state: "active",
        updated_revision: 9,
      },
    ];
    state.pending_observations = [
      {
        id: "o2",
        kind: "posture",
        body_region: "颈部",
        value: { label: "pending" },
        review_state: "unverified",
        lifecycle_state: "active",
        updated_revision: 9,
      },
    ];
    state.hypotheses = [
      {
        id: "h1",
        concern_key: "region:右肩",
        statement: "possible explanation",
        lifecycle_state: "active",
        supporting_fact_ids: [],
        supporting_observation_ids: [],
        supporting_evidence_ids: [],
        counterevidence_ids: [],
        created_revision: 9,
        updated_revision: 9,
      },
    ];

    expect(selectBodyRegionVisualState(state, "knee.left")).toBe("observed");
    expect(selectBodyRegionVisualState(state, "neck")).toBe("observed");
    expect(selectBodyRegionVisualState(state, "shoulder.right")).toBe("none");
  });

  it("does not apply a global safety flag to every region without explicit region IDs", () => {
    const state = snapshot();
    state.safety_state = {
      has_red_flags: true,
      status: "requires_review",
    };
    expect(
      selectBodyRegionVisualSummaries(state).every(
        (summary) => summary.visualState !== "safety_review",
      ),
    ).toBe(true);

    state.safety_state = {
      has_red_flags: true,
      status: "requires_review",
      body_region_ids: ["knee.right", "not-a-region"],
    };
    expect(selectBodyRegionVisualState(state, "knee.right")).toBe(
      "safety_review",
    );
    expect(selectBodyRegionVisualState(state, "knee.left")).toBe("none");
  });

  it("ignores inactive/rejected records and keeps one deterministic row per region", () => {
    const state = snapshot();
    state.facts = [
      {
        id: "resolved",
        kind: "discomfort",
        body_region: "右肩",
        value: "old",
        origin: "user_reported",
        review_state: "confirmed",
        lifecycle_state: "resolved",
        trend: "worsening",
        updated_revision: 8,
      },
      {
        id: "rejected",
        kind: "discomfort",
        body_region: "左膝",
        value: "rejected",
        origin: "ai_extracted",
        review_state: "rejected",
        lifecycle_state: "active",
        trend: "worsening",
        updated_revision: 9,
      },
    ];

    const summaries = selectBodyRegionVisualSummaries(state);
    expect(summaries).toHaveLength(35);
    expect(new Set(summaries.map((summary) => summary.regionId)).size).toBe(35);
    expect(selectBodyRegionVisualState(state, "shoulder.right")).toBe("none");
    expect(selectBodyRegionVisualState(state, "knee.left")).toBe("none");
  });
});
