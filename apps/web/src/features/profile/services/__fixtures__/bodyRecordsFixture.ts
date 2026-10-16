import type { BodyRecords } from "../bodyRecordsService";
import type { BodyFactRecord } from "../bodyFactRecordsService";

export function bodyRecordsFixture(): BodyRecords {
  return {
    user_id: "owner",
    current_revision: 4,
    facts: [],
    observations: [],
    hypotheses: [],
    episodes: [],
  };
}

export function symptomFact(
  overrides: Partial<Extract<BodyFactRecord, { kind: "symptom" }>> = {},
): BodyFactRecord {
  return {
    id: "fact",
    user_id: "owner",
    subject: "self",
    source_evidence_id: "evidence",
    kind: "symptom",
    statement: "右肩抬手时疼",
    body_region_ids: ["shoulder.right"],
    temporal: { precision: "unknown" },
    details: {},
    assertion_status: "asserted",
    created_revision: 4,
    recorded_at: "2026-10-16T00:00:00Z",
    ...overrides,
  };
}
