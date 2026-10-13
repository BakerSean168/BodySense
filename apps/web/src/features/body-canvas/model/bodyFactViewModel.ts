import type {
  BodyFactRecord,
  BodyFactRecordsPage,
} from "@/features/profile/services/bodyFactRecordsService";
import { BODY_FACT_KIND_LABELS } from "./bodyFactPresentation";

/**
 * Contract-bound presentation projection for the Person Records/Habits tabs.
 * Never infer an episode state or a medical conclusion from assertion recency.
 * The same source ledger powers both tabs, with no second Habits authority.
 */
export type BodyFactSurface = "records" | "habits";
export type BodyFactSurfaceState =
  | "unauthenticated"
  | "loading"
  | "unavailable"
  | "empty"
  | "partial_empty"
  | "ready"
  | "stale";

export interface BodyFactRecordViewModel {
  id: string;
  record: BodyFactRecord;
  label: string;
  statement: string;
  occurrence: string;
  recordedAt: string;
  isCorrection: boolean;
}

export interface BodyFactSurfaceViewModel {
  state: BodyFactSurfaceState;
  records: readonly BodyFactRecordViewModel[];
  currentRevision: number | null;
  hasMore: boolean;
  /** A concurrent amendment can make different pages disagree about revision. */
  pagesDisagree: boolean;
}

export function projectBodyFactSurface({
  pages,
  surface,
  hasOwner,
  isPending,
  isError,
}: {
  pages: readonly BodyFactRecordsPage[] | undefined;
  surface: BodyFactSurface;
  hasOwner: boolean;
  isPending: boolean;
  isError: boolean;
}): BodyFactSurfaceViewModel {
  if (!hasOwner) {
    return {
      state: "unauthenticated",
      records: [],
      currentRevision: null,
      hasMore: false,
      pagesDisagree: false,
    };
  }

  const head = pages?.[0];
  if (!head) {
    return {
      state: isPending ? "loading" : "unavailable",
      records: [],
      currentRevision: null,
      hasMore: false,
      pagesDisagree: false,
    };
  }

  // A page reads one internally consistent snapshot; two pages obtained at
  // different revisions cannot be combined as if they were a single view.
  const pagesDisagree = pages.some(
    (page) =>
      page.user_id !== head.user_id ||
      page.current_revision !== head.current_revision,
  );
  const consistentPages = pagesDisagree ? [head] : pages;
  const records = consistentPages
    .flatMap((page) => page.facts)
    .filter((record) => surface === "records" || record.kind === "lifestyle")
    .map((record): BodyFactRecordViewModel => ({
      id: record.id,
      record,
      label: BODY_FACT_KIND_LABELS[record.kind],
      statement: record.statement,
      occurrence: describeOccurrence(record.temporal),
      recordedAt: record.recorded_at,
      isCorrection: Boolean(record.supersedes_fact_id),
    }));
  const lastPage = consistentPages.at(-1);
  const hasMore = Boolean(lastPage?.has_more);

  let state: BodyFactSurfaceState = "ready";
  if (pagesDisagree) state = "stale";
  else if (!records.length && hasMore) state = "partial_empty";
  else if (!records.length && isError) state = "unavailable";
  else if (!records.length) state = "empty";

  return {
    state,
    records,
    currentRevision: head.current_revision,
    hasMore,
    pagesDisagree,
  };
}

export function describeOccurrence(
  temporal: BodyFactRecord["temporal"],
): string {
  // An expression is authoritative user wording, not a normalized date.
  if (temporal.original_expression?.trim()) return temporal.original_expression;
  if (temporal.start && temporal.end)
    return `${temporal.start} ～ ${temporal.end}`;
  return temporal.start || temporal.end || "未说明";
}
