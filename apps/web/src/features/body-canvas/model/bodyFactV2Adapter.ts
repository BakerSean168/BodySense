import {
  boundedBodyFactVM,
  type BoundedBodyFactVM,
} from "@bodysense/contracts/presentation-v2";
import type {
  BodyFactSurface,
  BodyFactSurfaceViewModel,
} from "./bodyFactViewModel";

/**
 * Existing authenticated BodyFact HTTP ledger -> bounded V2 presentation contract.
 *
 * This does not manufacture the richer Person Records VM. The current API
 * exposes only source_evidence_id and facts; the unresolved source details,
 * observations, hypotheses, episodes and concern groups stay explicitly absent.
 * The full V2 model needs a server-side joined projection.
 */
export function projectBoundedBodyFactVM(
  view: BodyFactSurfaceViewModel,
  surface: BodyFactSurface,
): BoundedBodyFactVM {
  return boundedBodyFactVM.parse({
    contract: "bodyfact-ledger-page/v1",
    query: surface === "habits" ? "person.habits" : "person.records",
    state: view.state,
    currentRevision: view.currentRevision,
    hasMore: view.hasMore,
    pagesDisagree: view.pagesDisagree,
    records: view.records.map(({ record }) => ({
      factId: record.id,
      kind: record.kind,
      statement: record.statement,
      bodyRegionIds: record.body_region_ids,
      sourceEvidenceId: record.source_evidence_id,
      recordedAt: record.recorded_at,
      temporal: {
        precision: record.temporal.precision,
        ...(record.temporal.start ? { start: record.temporal.start } : {}),
        ...(record.temporal.end ? { end: record.temporal.end } : {}),
        ...(record.temporal.original_expression
          ? { originalExpression: record.temporal.original_expression }
          : {}),
        ...(record.temporal.timezone
          ? { timeZone: record.temporal.timezone }
          : {}),
      },
      assertionStatus: record.assertion_status,
      ...(record.supersedes_fact_id
        ? { supersedesFactId: record.supersedes_fact_id }
        : {}),
    })),
    missingSections: [
      "source_metadata",
      "observations",
      "hypotheses",
      "episodes",
      "concern_groups",
    ],
  });
}
