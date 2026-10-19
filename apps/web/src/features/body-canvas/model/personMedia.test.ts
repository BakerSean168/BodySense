import { describe, expect, it } from "vitest";
import {
  formatMediaDate,
  groupMediaByMonth,
  limitMediaGroups,
  mediaState,
  type PersonMediaUpload,
} from "./personMedia";

const now = new Date(2026, 9, 19, 12, 0, 0);

function upload(
  id: string,
  createdAt: Date,
  extra: Partial<PersonMediaUpload> = {},
): PersonMediaUpload {
  return {
    id,
    file_type: "consultation_photo",
    original_name: `${id}.jpg`,
    file_size: 2_100_000,
    mime_type: "image/jpeg",
    ocr_status: "pending",
    analysis_status: "none",
    created_at: createdAt.toISOString(),
    source_availability: "available",
    ...extra,
  };
}

describe("person media grid model", () => {
  it("labels today, yesterday, this year and older dates", () => {
    expect(formatMediaDate(new Date(2026, 9, 19, 8).toISOString(), now)).toBe(
      "今天",
    );
    expect(formatMediaDate(new Date(2026, 9, 18, 23).toISOString(), now)).toBe(
      "昨天",
    );
    expect(formatMediaDate(new Date(2026, 9, 12).toISOString(), now)).toBe(
      "10月12日",
    );
    expect(formatMediaDate(new Date(2025, 8, 3).toISOString(), now)).toBe(
      "2025年9月3日",
    );
    expect(formatMediaDate("not a date", now)).toBe("日期未知");
  });

  it("groups newest first by month, naming other years", () => {
    const groups = groupMediaByMonth(
      [
        upload("sep", new Date(2026, 8, 28)),
        upload("oct-old", new Date(2026, 9, 8)),
        upload("last-year", new Date(2025, 9, 1)),
        upload("oct-new", new Date(2026, 9, 19, 9)),
      ],
      now,
    );
    expect(groups.map((g) => g.label)).toEqual(["10月", "9月", "2025年10月"]);
    expect(groups[0].tiles.map((t) => t.upload.id)).toEqual([
      "oct-new",
      "oct-old",
    ]);
    expect(groups[0].tiles[0].dateLabel).toBe("今天");
  });

  it("marks video, processing, failed and source-removed tiles from the manifest", () => {
    const [group] = groupMediaByMonth(
      [
        upload("video", new Date(2026, 9, 19, 11), { mime_type: "video/mp4" }),
        upload("busy", new Date(2026, 9, 19, 10), {
          analysis_status: "processing",
        }),
        upload("queued", new Date(2026, 9, 19, 9), {
          analysis_status: "pending",
        }),
        upload("broken", new Date(2026, 9, 19, 8), {
          analysis_status: "failed",
        }),
        upload("gone", new Date(2026, 9, 19, 7), {
          analysis_status: "failed",
          source_availability: "source_unavailable",
        }),
      ],
      now,
    );
    expect(group.tiles.map((t) => [t.kind, t.state])).toEqual([
      ["video", "ready"],
      ["image", "processing"],
      ["image", "processing"],
      ["image", "failed"],
      ["image", "source_unavailable"],
    ]);
    expect(mediaState(upload("plain", now))).toBe("ready");
  });

  it("limits the visible tiles across groups, newest first", () => {
    const groups = groupMediaByMonth(
      [
        upload("a", new Date(2026, 9, 19)),
        upload("b", new Date(2026, 9, 18)),
        upload("c", new Date(2026, 8, 18)),
        upload("d", new Date(2026, 8, 17)),
      ],
      now,
    );
    const limited = limitMediaGroups(groups, 3);
    expect(limited.map((g) => g.tiles.map((t) => t.upload.id))).toEqual([
      ["a", "b"],
      ["c"],
    ]);
    expect(limitMediaGroups(groups, 0)).toEqual([]);
  });
});
