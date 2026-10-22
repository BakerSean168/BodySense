import { describe, expect, it } from "vitest";
import type {
  BodyFactRecord,
  BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import type {
  BodyEpisode,
  BodyEpisodePage,
} from "@/features/profile/services/bodyEpisodeService";
import {
  episodeSourceOption,
  episodeStateLabel,
  projectEpisodeAssociation,
} from "./bodyEpisodeAssociations";

const owner = "11111111-1111-4111-8111-111111111111";
const factId = "22222222-2222-4222-8222-222222222222";
const episodeId = "33333333-3333-4333-8333-333333333333";
const fact = (
  details: Record<string, unknown> = {
    symptom_type: "右肩疼痛",
    presence: "present",
  },
): BodyFactRecord => ({
  id: factId,
  user_id: owner,
  kind: "symptom",
  subject: "self",
  assertion_status: "asserted",
  details,
  statement: "我昨天右肩疼痛",
  body_region_ids: [],
  temporal: { precision: "unknown" },
  source_evidence_id: factId,
  created_revision: 3,
  recorded_at: "2026-10-13T00:00:00Z",
});
const factPage = (
  records: BodyFactRecord[] = [],
  revision = 3,
  hasMore = false,
): BodyFactRecordsPage => ({
  user_id: owner,
  current_revision: revision,
  facts: records,
  has_more: hasMore,
  ...(hasMore ? { next_before_revision: 3 } : {}),
});
const episode = (): BodyEpisode => ({
  id: episodeId,
  user_id: owner,
  symptom_type: "右肩疼痛",
  body_region_ids: [],
  laterality: "right",
  identity_status: "current",
  created_revision: 3,
  created_at: "2026-10-13T00:00:00Z",
  links: [
    {
      fact_id: factId,
      role: "observation",
      association_method: "explicit",
      is_primary: true,
      linked_revision: 3,
    },
  ],
  projection: {
    state: "unknown",
    reason: "insufficient_evidence",
    basis_fact_ids: [],
    based_on_revision: 3,
  },
});
const episodePage = (
  episodes: BodyEpisode[] = [],
  revision = 3,
): BodyEpisodePage => ({
  user_id: owner,
  current_revision: revision,
  episodes,
  has_more: false,
});

describe("Episode association source projection", () => {
  it("never treats cached facts as an authenticated owner's projection", () => {
    const vm = projectEpisodeAssociation(
      undefined,
      [factPage([fact()])],
      [episodePage()],
    );
    expect(vm.revision).toBeNull();
    expect(vm.facts).toEqual([]);
    expect(vm.episodes).toEqual([]);
  });

  it("rejects cross-read or cross-page revisions instead of fabricating a write revision", () => {
    const vm = projectEpisodeAssociation(
      owner,
      [factPage([fact()], 3)],
      [episodePage([], 4)],
    );
    expect(vm.stale).toBe(true);
    expect(vm.revision).toBeNull();
    expect(vm.facts).toEqual([]);
    const mixedPage = projectEpisodeAssociation(
      owner,
      [factPage([fact()], 3), factPage([], 4)],
      [episodePage()],
    );
    expect(mixedPage.stale).toBe(true);
  });

  it("uses only current, self-owned asserted symptom Facts with typed presence", () => {
    expect(episodeSourceOption(fact())).toMatchObject({
      id: factId,
      symptomType: "右肩疼痛",
      presence: "present",
    });
    expect(
      episodeSourceOption(
        fact({ symptom_type: "肩部疼痛", presence: "absent" }),
      )?.presence,
    ).toBe("absent");
    expect(
      episodeSourceOption(
        fact({ symptom_type: "肩部疼痛", presence: "unknown" }),
      ),
    ).toBeNull();
    expect(
      episodeSourceOption({
        ...fact(),
        kind: "lifestyle",
      } as unknown as BodyFactRecord),
    ).toBeNull();
    expect(
      episodeSourceOption({
        ...fact(),
        subject: "other",
      } as unknown as BodyFactRecord),
    ).toBeNull();
    expect(
      episodeSourceOption({ ...fact(), assertion_status: "withdrawn" }),
    ).toBeNull();
  });

  it("retains server-owned Episode uncertainty and primary evidence links", () => {
    const vm = projectEpisodeAssociation(
      owner,
      [factPage([fact()])],
      [episodePage([episode()])],
    );
    expect(vm.revision).toBe(3);
    expect(vm.stale).toBe(false);
    expect(vm.facts).toHaveLength(1);
    expect(vm.episodes).toHaveLength(1);
    expect(vm.linkedFactIds.has(factId)).toBe(true);
    expect(episodeStateLabel(vm.episodes[0])).toBe("持续状态尚未确认");
  });

  it("distinguishes explicit report as-of from a medical determination", () => {
    const ended = {
      ...episode(),
      projection: {
        state: "ended" as const,
        reason: "explicit_resolution" as const,
        basis_fact_ids: [factId],
        based_on_revision: 3,
      },
    };
    expect(episodeStateLabel(ended)).toBe("有明确的结束报告");
    expect(
      episodeStateLabel({
        ...ended,
        identity_status: "invalidated",
      }),
    ).toBe("经历身份已失效");
  });

  it("does not claim completeness while older pages are unread", () => {
    const vm = projectEpisodeAssociation(
      owner,
      [factPage([], 3, true)],
      [episodePage()],
    );
    expect(vm.facts).toEqual([]);
    expect(vm.factsHaveMore).toBe(true);
    expect(vm.revision).toBe(3);
  });
});
