import { describe, expect, it } from "vitest";
import * as zod from "zod/mini";
import { AssetErasureReceipt } from "@/generated/api/model/assetErasureReceipt.zod";
import { StatementErasureReceipt } from "@/generated/api/model/statementErasureReceipt.zod";

// Only trust-boundary service tests import generated REST schemas. The
// component view-model tests operate on typed DTOs and must not reach across
// the approved boundary.
const uuid = "00000000-0000-4000-8000-000000000001";
const at = "2026-11-05T00:00:00Z";

const assetTargets = [
  "source_objects",
  "derived_jobs",
  "posture_analyses",
  "diagnosis_records",
  "document_extractions",
  "assessment_reports",
  "runtime_checkpoints",
  "episodic_notes",
  "asset_manifest",
] as const;

const statementTargets = [
  "diagnosis_records",
  "training_plans",
  "body_records",
  "assessment_reports",
  "decision_records",
  "runtime_checkpoints",
  "episodic_notes",
  "source_message",
  "statement_manifest",
] as const;

function targets(values: readonly string[]) {
  return values.map((target) =>
    target === "episodic_notes"
      ? {
          target,
          state: "failed",
          erased_count: 0,
          outstanding_count: 1,
          failure_code: "copy_reappeared",
        }
      : {
          target,
          state: "verified",
          erased_count: 0,
          outstanding_count: 0,
          verified_at: at,
        },
  );
}

describe("generated public REST erasure receipts", () => {
  it("accepts the full nine-target asset receipt, retaining strict parser validation", () => {
    const receipt = {
      request_id: uuid,
      asset_id: uuid,
      inventory_revision: "asset-erasure-inventory-v2",
      status: "outstanding",
      requested_at: at,
      targets: targets(assetTargets),
      outstanding_targets: ["episodic_notes"],
      reviews: [],
    };
    expect(zod.safeParse(AssetErasureReceipt, receipt).success).toBe(true);
    expect(
      zod.safeParse(AssetErasureReceipt, {
        ...receipt,
        targets: [
          ...receipt.targets,
          { target: "unrecognized", state: "verified" },
        ],
      }).success,
    ).toBe(false);
  });

  it("accepts the full nine-target statement receipt, retaining strict parser validation", () => {
    const receipt = {
      request_id: uuid,
      statement_id: uuid,
      inventory_revision: "statement-erasure-inventory-v3",
      status: "outstanding",
      requested_at: at,
      targets: targets(statementTargets),
      outstanding_targets: ["episodic_notes"],
      retained_targets: [],
      reviews: [],
    };
    expect(zod.safeParse(StatementErasureReceipt, receipt).success).toBe(true);
    expect(
      zod.safeParse(StatementErasureReceipt, {
        ...receipt,
        targets: [
          ...receipt.targets,
          { target: "unrecognized", state: "verified" },
        ],
      }).success,
    ).toBe(false);
  });
});
