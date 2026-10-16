import { describe, expect, it } from "vitest";
import {
  bodyRecordsFixture,
  symptomFact,
} from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import {
  recordBodyRegions,
  selectBodyRegionVisualSummaries,
  selectBodyRegionVisualState,
} from "./bodyExplorerSelectors";

describe("native region records", () => {
  it("uses every canonical region once without guessing unknown values", () => {
    expect(
      recordBodyRegions({
        body_region_ids: [
          "shoulder.right",
          "shoulder.left",
          "shoulder.right",
          "右肩疼",
        ],
      }),
    ).toEqual(["shoulder.right", "shoulder.left"]);
  });
  it("shows assertions as records without inferring clinical trend", () => {
    const records = bodyRecordsFixture();
    records.facts = [
      symptomFact(),
      symptomFact({ id: "withdrawn", assertion_status: "withdrawn" }),
      symptomFact({ id: "replaced", assertion_status: "superseded" }),
    ];
    expect(
      selectBodyRegionVisualSummaries(records).find(
        (row) => row.regionId === "shoulder.right",
      ),
    ).toEqual({
      regionId: "shoulder.right",
      visualState: "observed",
      statementCount: 1,
      episodeCount: 0,
    });
    expect(selectBodyRegionVisualState(records, "shoulder.left")).toBe("none");
  });
  it("does not localize observations or user hypotheses from their prose", () => {
    const records = bodyRecordsFixture();
    records.observations = [
      {
        id: "observation",
        user_id: "owner",
        kind: "range",
        statement: "右肩抬手90度",
        observed_at: { precision: "unknown" },
        source: { kind: "user_report", id: "source", version: 1 },
        assertion_status: "asserted",
        created_revision: 4,
        recorded_at: "2026-10-16T00:00:00Z",
      },
    ];
    records.hypotheses = [
      {
        id: "hypothesis",
        user_id: "owner",
        statement: "怀疑右肩有问题",
        source: {
          actor: "user",
          kind: "direct_edit",
          id: "source",
          version: 1,
        },
        lineage_id: "hypothesis",
        related_fact_ids: [],
        related_observation_ids: [],
        assertion_status: "asserted",
        created_revision: 4,
        recorded_at: "2026-10-16T00:00:00Z",
      },
    ];
    expect(
      selectBodyRegionVisualSummaries(records).every(
        (row) => row.visualState === "none",
      ),
    ).toBe(true);
  });
});
