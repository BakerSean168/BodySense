import { describe, expect, it } from "vitest";
import { RecordBodyFactsRequest } from "./generated/api/model/recordBodyFactsRequest.zod";
import { BodyFactCommit } from "./generated/api/model/bodyFactCommit.zod";
import { BodyFactRecordPage } from "./generated/api/model/bodyFactRecordPage.zod";

const id = "12345678-1234-4234-9234-123456789abc";
const request = (kind = "symptom") => ({
  idempotency_key: "original-command",
  expected_revision: 0,
  facts: [
    {
      kind,
      statement: "Synthetic user statement",
      body_region_ids: [],
      temporal: { precision: "unknown" },
      details: {},
    },
  ],
});

// These are handwritten tests of the generated runtime boundary, not a second
// schema. A regenerated client must preserve the same five-way discriminator.
describe("BodyFact generated browser contract", () => {
  it.each([
    "symptom",
    "health_history",
    "activity",
    "lifestyle",
    "training_feedback",
  ])(
    "accepts optional unknown details for %s without inventing occurrence time",
    (kind) => {
      const parsed = RecordBodyFactsRequest.parse(request(kind));
      expect(parsed.facts[0]?.temporal).toEqual({ precision: "unknown" });
    },
  );

  it("rejects old lifecycle fields and untrusted source identities", () => {
    const base = request();
    expect(
      RecordBodyFactsRequest.safeParse({ ...base, user_id: id }).success,
    ).toBe(false);
    expect(
      RecordBodyFactsRequest.safeParse({
        ...base,
        facts: [{ ...base.facts[0], source_evidence_id: id }],
      }).success,
    ).toBe(false);
    expect(
      RecordBodyFactsRequest.safeParse({
        ...base,
        facts: [{ ...base.facts[0], details: { lifecycle_state: "resolved" } }],
      }).success,
    ).toBe(false);
  });

  it("rejects details belonging to another fact kind", () => {
    const base = request("lifestyle");
    expect(
      RecordBodyFactsRequest.safeParse({
        ...base,
        facts: [{ ...base.facts[0], details: { presence: "present" } }],
      }).success,
    ).toBe(false);
  });

  it("requires real receipt identities rather than a generic success flag", () => {
    expect(BodyFactCommit.safeParse({ success: true }).success).toBe(false);
    const receipt = {
      id,
      user_id: id,
      revision: 1,
      revision_id: id,
      event_id: id,
      fact_ids: [id],
      committed_at: "2026-10-13T00:00:00Z",
    };
    expect(BodyFactCommit.safeParse(receipt).success).toBe(true);
    expect(
      BodyFactCommit.safeParse({
        ...receipt,
        fingerprint: "private-derived-data",
      }).success,
    ).toBe(false);
  });

  it("distinguishes an empty page from a malformed or missing fact collection", () => {
    const page = {
      user_id: id,
      current_revision: 0,
      facts: [],
      has_more: false,
    };
    expect(BodyFactRecordPage.safeParse(page).success).toBe(true);
    expect(BodyFactRecordPage.safeParse({ ...page, facts: null }).success).toBe(
      false,
    );
    expect(
      BodyFactRecordPage.safeParse({ ...page, next_before_revision: -1 })
        .success,
    ).toBe(false);
  });
});
