import { describe, expect, it } from "vitest";
import type {
  BodyFactRecord,
  BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import {
  describeOccurrence,
  projectBodyFactSurface,
} from "./bodyFactViewModel";

const owner = "11111111-1111-4111-8111-111111111111";
function fact(kind: BodyFactRecord["kind"] = "symptom"): BodyFactRecord {
  return {
    id: "22222222-2222-4222-8222-222222222222",
    user_id: owner,
    subject: "self",
    kind,
    statement: "昨晚睡得不好",
    body_region_ids: [],
    temporal: { precision: "unknown" },
    details: {},
    source_evidence_id: "33333333-3333-4333-8333-333333333333",
    assertion_status: "asserted",
    created_revision: 3,
    recorded_at: "2026-10-13T00:00:00Z",
  };
}
function page({
  facts = [],
  revision = 3,
  hasMore = false,
}: {
  facts?: BodyFactRecord[];
  revision?: number;
  hasMore?: boolean;
} = {}): BodyFactRecordsPage {
  return {
    user_id: owner,
    current_revision: revision,
    facts,
    has_more: hasMore,
    ...(hasMore ? { next_before_revision: revision } : {}),
  };
}
const view = (
  pages: readonly BodyFactRecordsPage[] | undefined,
  surface: "records" | "habits" = "records",
  extras: Partial<{
    hasOwner: boolean;
    isPending: boolean;
    isError: boolean;
  }> = {},
) =>
  projectBodyFactSurface({
    pages,
    surface,
    hasOwner: true,
    isPending: false,
    isError: false,
    ...extras,
  });

describe("BodyFact Person ViewModel", () => {
  it("does not disclose cached facts for an unauthenticated owner", () => {
    expect(
      view([page({ facts: [fact()] })], "records", { hasOwner: false }),
    ).toEqual({
      state: "unauthenticated",
      records: [],
      currentRevision: null,
      hasMore: false,
      pagesDisagree: false,
    });
  });
  it("keeps loading, unavailable and truly empty separate", () => {
    expect(view(undefined, "records", { isPending: true }).state).toBe(
      "loading",
    );
    expect(view(undefined, "records", { isError: true }).state).toBe(
      "unavailable",
    );
    expect(view([page()], "records").state).toBe("empty");
  });
  it("never calls a partial history scan an empty Habits history", () => {
    const vm = view([page({ facts: [fact()], hasMore: true })], "habits");
    expect(vm.records).toHaveLength(0);
    expect(vm.hasMore).toBe(true);
    expect(vm.state).toBe("partial_empty");
  });
  it("projects lifestyle from the same source and preserves original statement", () => {
    const vm = view(
      [page({ facts: [fact("symptom"), fact("lifestyle")] })],
      "habits",
    );
    expect(vm.records).toHaveLength(1);
    expect(vm.records[0].statement).toBe("昨晚睡得不好");
    expect(vm.records[0].label).toBe("生活习惯");
    expect(vm.records[0].occurrence).toBe("未说明");
    expect(vm.currentRevision).toBe(3);
  });
  it("refuses to merge pages obtained at incompatible revisions", () => {
    const vm = view([
      page({ facts: [fact("symptom")], revision: 5, hasMore: true }),
      page({ facts: [fact("lifestyle")], revision: 4 }),
    ]);
    expect(vm.state).toBe("stale");
    expect(vm.pagesDisagree).toBe(true);
    expect(vm.records).toHaveLength(1);
    expect(vm.hasMore).toBe(true);
  });
  it("does not infer temporal boundaries from recorded-at or a negative symptom", () => {
    const vm = view([page({ facts: [fact()] })]);
    expect(vm.records[0].occurrence).toBe("未说明");
    expect(
      describeOccurrence({
        precision: "unknown",
        original_expression: "大约上周",
      }),
    ).toBe("大约上周");
    expect(
      describeOccurrence({
        precision: "range",
        start: "2026-10-01",
        end: "2026-10-03",
      }),
    ).toBe("2026-10-01 ～ 2026-10-03");
  });
});
