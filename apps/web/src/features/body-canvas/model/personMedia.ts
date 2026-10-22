import type { ProfileUploadLike } from "./profileCategories";

/**
 * Penpot "Person / Media": a thumbnail grid grouped by month ("10月", "9月"),
 * newest first. Each tile states only what the upload manifest says: a video
 * badge for video files, processing / failed while the derived analysis runs
 * or after it failed, and source-unavailable once the original was removed.
 * The manifest carries no image bytes or video duration, so the tile shows the
 * Penpot placeholder glyph and never an invented duration.
 */
export type PersonMediaUpload = ProfileUploadLike;

export type PersonMediaState =
  | "ready"
  | "processing"
  | "failed"
  | "source_unavailable";

export interface PersonMediaTile {
  upload: PersonMediaUpload;
  kind: "image" | "video";
  state: PersonMediaState;
  /** "今天" / "昨天" / "10月12日" / "2025年10月12日". */
  dateLabel: string;
}

export interface PersonMediaGroup {
  /** Stable key, "2026-10". */
  key: string;
  /** "10月" in the current year, otherwise "2025年10月". */
  label: string;
  tiles: PersonMediaTile[];
}

function validDate(value: string): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function sameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function formatMediaDate(value: string, now: Date = new Date()): string {
  const date = validDate(value);
  if (!date) return "日期未知";
  if (sameDay(date, now)) return "今天";
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (sameDay(date, yesterday)) return "昨天";
  const day = `${date.getMonth() + 1}月${date.getDate()}日`;
  return date.getFullYear() === now.getFullYear()
    ? day
    : `${date.getFullYear()}年${day}`;
}

export function mediaState(upload: PersonMediaUpload): PersonMediaState {
  if (upload.source_availability === "source_unavailable")
    return "source_unavailable";
  if (upload.analysis_status === "failed") return "failed";
  if (
    upload.analysis_status === "pending" ||
    upload.analysis_status === "processing"
  )
    return "processing";
  return "ready";
}

export function mediaKind(upload: PersonMediaUpload): "image" | "video" {
  return upload.mime_type.startsWith("video/") ? "video" : "image";
}

/** Newest first, grouped by calendar month in the viewer's time zone. */
export function groupMediaByMonth(
  uploads: readonly PersonMediaUpload[],
  now: Date = new Date(),
): PersonMediaGroup[] {
  const sorted = [...uploads].sort(
    (a, b) =>
      (validDate(b.created_at)?.getTime() ?? 0) -
      (validDate(a.created_at)?.getTime() ?? 0),
  );
  const groups: PersonMediaGroup[] = [];
  for (const upload of sorted) {
    const date = validDate(upload.created_at);
    const key = date
      ? `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`
      : "unknown";
    const label = !date
      ? "日期未知"
      : date.getFullYear() === now.getFullYear()
        ? `${date.getMonth() + 1}月`
        : `${date.getFullYear()}年${date.getMonth() + 1}月`;
    let group = groups[groups.length - 1];
    if (!group || group.key !== key) {
      group = { key, label, tiles: [] };
      groups.push(group);
    }
    group.tiles.push({
      upload,
      kind: mediaKind(upload),
      state: mediaState(upload),
      dateLabel: formatMediaDate(upload.created_at, now),
    });
  }
  return groups;
}

/** Keeps the first `limit` tiles (newest first) across groups. */
export function limitMediaGroups(
  groups: readonly PersonMediaGroup[],
  limit: number,
): PersonMediaGroup[] {
  const out: PersonMediaGroup[] = [];
  let left = limit;
  for (const group of groups) {
    if (left <= 0) break;
    const tiles = group.tiles.slice(0, left);
    left -= tiles.length;
    out.push({ ...group, tiles });
  }
  return out;
}
