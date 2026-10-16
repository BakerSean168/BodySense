import { describe, expect, it } from "vitest";
import {
  bodyRecordsFixture,
  symptomFact,
} from "@/features/profile/services/__fixtures__/bodyRecordsFixture";
import { selectBodyZoneSummaries } from "./bodyMap";

describe("body zone statements", () => {
  it("counts a multiregion statement once per zone and excludes withdrawn records", () => {
    const records = bodyRecordsFixture();
    records.facts = [
      symptomFact({
        body_region_ids: ["shoulder.left", "shoulder.right", "knee.right"],
      }),
      symptomFact({ id: "withdrawn", assertion_status: "withdrawn" }),
    ];
    expect(
      selectBodyZoneSummaries(records).map(({ zone, count }) => ({
        zone,
        count,
      })),
    ).toEqual([
      { zone: "neck", count: 1 },
      { zone: "legs", count: 1 },
    ]);
  });
  it("does not invent a torso record for an unlocalized statement", () => {
    const records = bodyRecordsFixture();
    records.facts = [symptomFact({ body_region_ids: ["unknown"] })];
    expect(selectBodyZoneSummaries(records)).toEqual([]);
    expect(selectBodyZoneSummaries(null)).toEqual([]);
  });
});
