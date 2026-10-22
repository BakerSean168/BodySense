import type { BodyFactRecordInput } from "@/features/profile/services/bodyFactRecordsService";

export type SymptomPresence = "present" | "absent";
export type SymptomSide =
  | "unknown"
  | "left"
  | "right"
  | "bilateral"
  | "not_applicable";
export interface EpisodeSymptomDraft {
  statement: string;
  symptomType: string;
  presence: SymptomPresence;
  laterality: SymptomSide;
}

/**
 * Only explicit user text goes into a durable symptom Fact. This command
 * never creates an Episode, infers a medical diagnosis, invents a date, or
 * turns a negative report into recovery.
 */
export function makeEpisodeSymptomRecordInput(
  draft: EpisodeSymptomDraft,
  expectedRevision: number,
  idempotencyKey: string,
): BodyFactRecordInput {
  const statement = draft.statement.trim();
  const symptomType = draft.symptomType.trim();
  if (
    !statement ||
    !symptomType ||
    statement.length > 1000 ||
    symptomType.length > 120 ||
    !Number.isSafeInteger(expectedRevision) ||
    expectedRevision < 0 ||
    !idempotencyKey.trim() ||
    idempotencyKey.length > 128
  ) {
    throw new Error("请提供明确的本人陈述和症状类型，并确认当前记录版本。");
  }
  return {
    idempotency_key: idempotencyKey,
    expected_revision: expectedRevision,
    facts: [
      {
        kind: "symptom",
        statement,
        body_region_ids: [],
        temporal: { precision: "unknown" },
        details: {
          symptom_type: symptomType,
          presence: draft.presence,
          laterality: draft.laterality,
        },
      },
    ],
  };
}
