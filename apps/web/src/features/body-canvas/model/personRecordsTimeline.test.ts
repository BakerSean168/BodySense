import { describe, expect, it } from "vitest";
import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import type { BodyFactRecord } from "@/features/profile/services/bodyFactRecordsService";
import {
  dayLabel,
  episodeContinuityChip,
  episodeGaps,
  episodeTitle,
  groupByRecordedDay,
  recordedClock,
} from "./personRecordsTimeline";

const zone = "Asia/Shanghai";
const now = new Date("2026-10-14T04:00:00Z"); // 12:00 in Shanghai

const episode = (
  state: "ongoing" | "ended" | "unknown",
  reason: BodyEpisode["projection"]["reason"],
): BodyEpisode =>
  ({
    id: "e1",
    user_id: "u",
    symptom_type: "头痛",
    body_region_ids: [],
    laterality: "unknown",
    identity_status: "current",
    created_revision: 1,
    created_at: "2026-10-14T01:00:00Z",
    links: [],
    projection: { state, reason, basis_fact_ids: ["f1"], based_on_revision: 1 },
  }) as unknown as BodyEpisode;

describe("Person Records timeline", () => {
  it("labels recorded days on the display clock, never by UTC", () => {
    expect(dayLabel("2026-10-14", zone, now)).toBe("今天 · 10月14日");
    expect(dayLabel("2026-10-13", zone, now)).toBe("昨天 · 10月13日");
    expect(dayLabel("2026-10-11", zone, now)).toBe("10月11日");
    expect(dayLabel("2025-10-11", zone, now)).toBe("2025年10月11日");
    // 23:30 UTC on the 13th is already the 14th in Shanghai.
    const groups = groupByRecordedDay(
      [
        { at: "2026-10-13T23:30:00Z" },
        { at: "2026-10-13T01:00:00Z" },
        { at: "bad" },
      ],
      (row) => row.at,
      zone,
      now,
    );
    expect(groups.map((group) => group.label)).toEqual([
      "今天 · 10月14日",
      "昨天 · 10月13日",
      "记录时间未知",
    ]);
    expect(recordedClock("2026-10-13T23:30:00Z", zone)).toBe("07:30");
  });

  it("states Episode continuity only from Go's evidence-qualified projection", () => {
    expect(
      episodeContinuityChip(episode("ongoing", "explicit_current_report")),
    ).toBe("是否持续 · 仍在持续");
    expect(episodeContinuityChip(episode("ended", "explicit_resolution"))).toBe(
      "是否持续 · 已结束",
    );
    // An unqualified "ongoing" is still unknown to the user.
    expect(
      episodeContinuityChip(episode("ongoing", "insufficient_evidence")),
    ).toBe("是否持续 · 未知");
    expect(episodeTitle(episode("unknown", "insufficient_evidence"))).toBe(
      "头痛经历",
    );
  });

  it("lists information gaps as missing answers, not findings", () => {
    const fact = (details: Record<string, unknown>) =>
      ({ id: "f1", details }) as unknown as BodyFactRecord;
    expect(
      episodeGaps(episode("unknown", "insufficient_evidence"), [fact({})]).map(
        (gap) => gap.label,
      ),
    ).toEqual(["严重程度", "现在是否还在持续"]);
    expect(
      episodeGaps(episode("ended", "explicit_resolution"), [
        fact({ severity: "mild" }),
      ]),
    ).toEqual([]);
  });
});
