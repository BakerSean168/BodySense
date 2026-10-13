import { describe, expect, it } from "vitest";
import {
  makeEpisodeSymptomRecordInput,
  type EpisodeSymptomDraft,
} from "./bodyEpisodeSymptomDraft";

const draft: EpisodeSymptomDraft = {
  statement: "我今天右肩疼痛",
  symptomType: "肩部疼痛",
  presence: "present",
  laterality: "right",
};

describe("direct user symptom command for Episode source", () => {
  it("creates a real Fact request from explicit user fields without an Episode conclusion", () => {
    expect(makeEpisodeSymptomRecordInput(draft, 3, "user-assertion")).toEqual({
      idempotency_key: "user-assertion",
      expected_revision: 3,
      facts: [
        {
          kind: "symptom",
          statement: "我今天右肩疼痛",
          body_region_ids: [],
          temporal: { precision: "unknown" },
          details: {
            symptom_type: "肩部疼痛",
            presence: "present",
            laterality: "right",
          },
        },
      ],
    });
  });
  it("preserves negative symptom semantics without inferring an Episode end", () => {
    const result = makeEpisodeSymptomRecordInput(
      {
        ...draft,
        statement: "我现在没有肩痛",
        presence: "absent",
      },
      4,
      "no-pain-record",
    );
    expect(result.facts[0].details).toMatchObject({ presence: "absent" });
    expect(result.facts[0].details).not.toHaveProperty("episode_state");
    expect(result.facts[0].temporal.precision).toBe("unknown");
  });
  it("refuses missing source details, unsafe lengths, and bad revisions", () => {
    expect(() =>
      makeEpisodeSymptomRecordInput({ ...draft, statement: " " }, 3, "key"),
    ).toThrow();
    expect(() =>
      makeEpisodeSymptomRecordInput({ ...draft, symptomType: "" }, 3, "key"),
    ).toThrow();
    expect(() => makeEpisodeSymptomRecordInput(draft, -1, "key")).toThrow();
    expect(() => makeEpisodeSymptomRecordInput(draft, 3.14, "key")).toThrow();
    expect(() => makeEpisodeSymptomRecordInput(draft, 3, "")).toThrow();
    expect(() =>
      makeEpisodeSymptomRecordInput(
        { ...draft, statement: "疼".repeat(1001) },
        3,
        "key",
      ),
    ).toThrow();
  });
});
