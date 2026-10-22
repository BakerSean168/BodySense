import { describe, expect, it } from "vitest";
import { boundedBodyFactVM } from "@bodysense/contracts/presentation-v2";
import type {
  BodyFactRecord,
  BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import { projectBodyFactSurface } from "./bodyFactViewModel";
import { projectBoundedBodyFactVM } from "./bodyFactV2Adapter";

const owner = "11111111-1111-4111-8111-111111111111";
const factId = "22222222-2222-4222-8222-222222222222";
const sourceId = "33333333-3333-4333-8333-333333333333";
function fact(kind: BodyFactRecord["kind"] = "symptom"): BodyFactRecord {
  return {
    kind,
    id: factId,
    user_id: owner,
    subject: "self",
    statement: "昨天有过肩痛",
    body_region_ids: ["right_shoulder"],
    temporal: { precision: "unknown", original_expression: "昨天" },
    details: {},
    source_evidence_id: sourceId,
    assertion_status: "asserted",
    created_revision: 3,
    recorded_at: "2026-10-13T00:00:00Z",
  };
}
function page(
  facts: BodyFactRecord[],
  hasMore = false,
  revision = 3,
): BodyFactRecordsPage {
  return {
    user_id: owner,
    current_revision: revision,
    facts,
    has_more: hasMore,
    ...(hasMore ? { next_before_revision: revision } : {}),
  };
}
const projection = (
  pages: BodyFactRecordsPage[] | undefined,
  surface: "records" | "habits" = "records",
  hasOwner = true,
) =>
  projectBoundedBodyFactVM(
    projectBodyFactSurface({
      pages,
      surface,
      hasOwner,
      isPending: false,
      isError: false,
    }),
    surface,
  );

describe("BodyFact current-transport to bounded V2 contract", () => {
  it("uses actual source_evidence_id without manufacturing attribution or clinical admissibility", () => {
    const result = projection([page([fact()])]);
    expect(result).toMatchObject({
      contract: "bodyfact-ledger-page/v1",
      query: "person.records",
      state: "ready",
    });
    expect(result.records).toHaveLength(1);
    expect(result.records[0]).toMatchObject({
      factId,
      kind: "symptom",
      sourceEvidenceId: sourceId,
      statement: "昨天有过肩痛",
      assertionStatus: "asserted",
      temporal: { precision: "unknown", originalExpression: "昨天" },
    });
    expect(result.records[0]).not.toHaveProperty("source");
    expect(result.records[0]).not.toHaveProperty("medicalName");
    expect(result.missingSections).toEqual([
      "source_metadata",
      "observations",
      "hypotheses",
      "episodes",
      "concern_groups",
    ]);
  });

  it("filters Habits on the same ledger, keeping unresolved older pages", () => {
    const result = projection([page([fact()], true)], "habits");
    expect(result.state).toBe("partial_empty");
    expect(result.records).toHaveLength(0);
    expect(result.hasMore).toBe(true);
    expect(
      projection([page([fact("lifestyle")])], "habits").records[0].kind,
    ).toBe("lifestyle");
  });

  it("denies access to another owner's cached records after signout", () => {
    const result = projection([page([fact()])], "records", false);
    expect(result).toMatchObject({
      state: "unauthenticated",
      records: [],
      currentRevision: null,
    });
    expect(result).not.toHaveProperty("ownerId");
  });

  it("cannot turn a paginated revision conflict into current stable truth", () => {
    const view = projectBodyFactSurface({
      pages: [page([fact()], true, 5), page([fact("lifestyle")], false, 4)],
      surface: "records",
      hasOwner: true,
      isPending: false,
      isError: false,
    });
    const result = projectBoundedBodyFactVM(view, "records");
    expect(result.state).toBe("stale");
    expect(result.pagesDisagree).toBe(true);
    expect(result.records).toHaveLength(1);
  });

  it("rejects a fake complete clinical source, false Habits count, or mismatched stale flag", () => {
    const base = projection([page([fact()])]);
    expect(
      boundedBodyFactVM.safeParse({
        ...base,
        records: [
          { ...base.records[0], source: { attribution: "clinical_document" } },
        ],
      }).success,
    ).toBe(false);
    expect(
      boundedBodyFactVM.safeParse({ ...base, query: "person.habits" }).success,
    ).toBe(false);
    expect(
      boundedBodyFactVM.safeParse({
        ...base,
        state: "stale",
        pagesDisagree: false,
      }).success,
    ).toBe(false);
  });

  it("separates a truly exhausted empty scope from a partially scanned one", () => {
    expect(projection([page([])], "habits").state).toBe("empty");
    expect(projection([page([], true)], "habits").state).toBe("partial_empty");
  });

  it("does not silently synthesize occurrence date or clinical certainty", () => {
    const result = projection([page([fact()])]);
    expect(result.records[0].temporal).not.toHaveProperty("start");
    expect(result.records[0]).not.toHaveProperty("assessment");
    expect(result.records[0].recordedAt).toBe("2026-10-13T00:00:00Z");
  });
});
