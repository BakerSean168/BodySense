import { ChevronRight } from "lucide-react";
import { formatInstantInZone } from "@/features/profile/model/profilePreferences";
import type { BodyFactRecordViewModel } from "../model/bodyFactViewModel";
import {
  groupByRecordedDay,
  recordedClock,
} from "../model/personRecordsTimeline";

/** Render the existing projection; grouping and display clocks stay in model/. */
export function BodyFactRecordList({
  rows,
  habitsOnly,
  displayZone,
  now: clock,
  onSelect,
}: {
  rows: readonly BodyFactRecordViewModel[];
  habitsOnly: boolean;
  displayZone: string;
  now: Date;
  onSelect: (id: string) => void;
}) {
  const occurrenceLine = (occurrence: string, recordedAt: string) =>
    `发生：${occurrence} · 记录 ${
      habitsOnly
        ? (formatInstantInZone(recordedAt, displayZone) ?? "记录时间未知")
        : recordedClock(recordedAt, displayZone)
    }`;
  const row = ({
    id,
    label,
    statement: statementText,
    occurrence,
    recordedAt,
    isCorrection,
  }: BodyFactRecordViewModel) => (
    <li key={id}>
      <button
        type="button"
        className="bc-person-row"
        data-habit={habitsOnly ? "true" : undefined}
        aria-label={`查看记录：${statementText}`}
        onClick={() => onSelect(id)}
      >
        {habitsOnly ? null : (
          <span className="bc-person-row-kind">{label}</span>
        )}
        <span className="bc-person-row-main">
          <span className="bc-record-statement">{statementText}</span>
          <span className="bc-person-row-meta">
            {occurrenceLine(occurrence, recordedAt)}
          </span>
        </span>
        {isCorrection ? (
          <span className="bc-person-chip">更正后的陈述</span>
        ) : null}
        <ChevronRight
          size={17}
          className="bc-person-row-chevron"
          aria-hidden="true"
        />
      </button>
    </li>
  );

  return (
    <>
      {habitsOnly ? (
        rows.length ? (
          <ul className="bc-person-timeline" aria-label="生活习惯">
            {rows.map(row)}
          </ul>
        ) : null
      ) : (
        groupByRecordedDay(
          rows,
          (item) => item.recordedAt,
          displayZone,
          clock,
        ).map((group) => (
          <section
            key={group.key}
            className="bc-person-day"
            aria-label={group.label}
          >
            <h4 className="bc-person-day-label">{group.label}</h4>
            <ul className="bc-person-timeline">{group.items.map(row)}</ul>
          </section>
        ))
      )}
    </>
  );
}
