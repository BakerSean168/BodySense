import { assertNever } from "@/lib/assertNever";
import type { BodyEpisode } from "@/features/profile/services/bodyEpisodeService";
import type { BodyFactRecord } from "@/features/profile/services/bodyFactRecordsService";
import { episodeDisplayState } from "./bodyEpisodeAssociations";

/**
 * Penpot Person / Records: a date-grouped timeline. Grouping uses the instant
 * the statement was *recorded* on the user's display clock; the occurrence the
 * user described stays verbatim and is never turned into a date.
 */
export interface TimelineGroup<T> {
  key: string;
  label: string;
  items: T[];
}

function dayParts(instant: Date, zone: string) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(instant);
  const get = (type: string) =>
    Number(parts.find((part) => part.type === type)?.value);
  return { year: get("year"), month: get("month"), day: get("day") };
}

/** Calendar-day key (YYYY-MM-DD) of an instant in a time zone. */
export function dayKey(iso: string, zone: string): string | null {
  const instant = new Date(iso);
  if (Number.isNaN(instant.getTime())) return null;
  const { year, month, day } = dayParts(instant, zone);
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

function dayNumber(key: string): number {
  const [year, month, day] = key.split("-").map(Number);
  return Date.UTC(year, month - 1, day) / 86_400_000;
}

/** "今天 · 10月14日", "昨天 · 10月13日", "10月11日", or with a year when it differs. */
export function dayLabel(key: string, zone: string, now: Date): string {
  const today = dayKey(now.toISOString(), zone)!;
  const [year, month, day] = key.split("-").map(Number);
  const [currentYear] = today.split("-").map(Number);
  const date =
    year === currentYear ? `${month}月${day}日` : `${year}年${month}月${day}日`;
  const delta = dayNumber(today) - dayNumber(key);
  if (delta === 0) return `今天 · ${date}`;
  if (delta === 1) return `昨天 · ${date}`;
  return date;
}

/** Clock time of the recorded instant, e.g. "09:12". */
export function recordedClock(iso: string, zone: string): string {
  const instant = new Date(iso);
  if (Number.isNaN(instant.getTime())) return "时间未知";
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: zone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(instant);
}

/** Groups newest-first rows by recorded day, keeping the input order inside a day. */
export function groupByRecordedDay<T>(
  rows: readonly T[],
  recordedAt: (row: T) => string,
  zone: string,
  now: Date,
): TimelineGroup<T>[] {
  const groups: TimelineGroup<T>[] = [];
  for (const row of rows) {
    const key = dayKey(recordedAt(row), zone) ?? "unknown";
    let group = groups.find((item) => item.key === key);
    if (!group) {
      group = {
        key,
        label: key === "unknown" ? "记录时间未知" : dayLabel(key, zone, now),
        items: [],
      };
      groups.push(group);
    }
    group.items.push(row);
  }
  return groups;
}

/** Penpot Episode chip: "是否持续 · 未知". Go's projection is the only source. */
export function episodeContinuityChip(episode: BodyEpisode): string {
  if (episode.identity_status !== "current") return "经历已不再使用";
  const displayState = episodeDisplayState(episode);
  switch (displayState) {
    case "ongoing":
      return "是否持续 · 仍在持续";
    case "ended":
      return "是否持续 · 已结束";
    case "unknown":
      return "是否持续 · 未知";
  }
  return assertNever(displayState);
}

export interface EpisodeGap {
  id: "continuity" | "severity";
  label: string;
}

/**
 * Penpot "信息缺口": what the linked statements do not say. A gap is only the
 * absence of an explicit answer; it never asserts anything about the body.
 */
export function episodeGaps(
  episode: BodyEpisode,
  linkedFacts: readonly BodyFactRecord[],
): EpisodeGap[] {
  const gaps: EpisodeGap[] = [];
  const hasSeverity = linkedFacts.some((fact) => {
    const severity = (fact.details as Record<string, unknown> | undefined)
      ?.severity;
    return severity !== undefined && severity !== null && severity !== "";
  });
  if (!hasSeverity) gaps.push({ id: "severity", label: "严重程度" });
  if (episodeDisplayState(episode) === "unknown")
    gaps.push({ id: "continuity", label: "现在是否还在持续" });
  return gaps;
}

/** Episode title in the user's own symptom wording: "头痛经历". */
export function episodeTitle(episode: BodyEpisode): string {
  const symptom = episode.symptom_type.trim();
  return symptom.endsWith("经历") ? symptom : `${symptom}经历`;
}
