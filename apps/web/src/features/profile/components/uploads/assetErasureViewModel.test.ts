import { describe, expect, it } from "vitest";
import type { AssetErasureReceipt } from "@/features/profile/services/assetErasureService";
import { projectAssetErasure } from "./assetErasureViewModel";

const targets = [
  "source_objects",
  "derived_jobs",
  "posture_analyses",
  "diagnosis_records",
  "document_extractions",
  "assessment_reports",
  "runtime_checkpoints",
  "asset_manifest",
] as const;

function receipt(
  status: AssetErasureReceipt["status"],
  overrides: Partial<
    Record<
      (typeof targets)[number],
      Partial<AssetErasureReceipt["targets"][number]>
    >
  > = {},
  reviews: AssetErasureReceipt["reviews"] = [],
): AssetErasureReceipt {
  return {
    request_id: "00000000-0000-4000-8000-000000000001",
    asset_id: "00000000-0000-4000-8000-000000000002",
    status,
    inventory_revision: "asset-erasure-inventory-v1",
    requested_at: "2026-10-19T00:00:00Z",
    targets: targets.map((target) => ({
      target,
      state: "verified",
      erased_count: 0,
      outstanding_count: 0,
      ...overrides[target],
    })),
    outstanding_targets: [],
    reviews,
  };
}

describe("projectAssetErasure", () => {
  it("shows done only for a fully verified receipt", () => {
    const view = projectAssetErasure(receipt("erased"));
    expect(view.phase).toBe("erased");
    expect(view.outstanding).toEqual([]);
  });

  it("keeps a transient failure outstanding, not done", () => {
    const view = projectAssetErasure(
      receipt("outstanding", {
        source_objects: {
          state: "failed",
          failure_code: "storage_unavailable",
          outstanding_count: 1,
        },
        asset_manifest: {
          state: "blocked",
          failure_code: "targets_outstanding",
          outstanding_count: 1,
        },
      }),
    );
    expect(view.phase).toBe("outstanding");
    expect(view.title).not.toBe("已彻底删除");
    expect(view.outstanding).toContainEqual(
      expect.objectContaining({
        target: "source_objects",
        reason: expect.stringMatching(/自动重试/),
      }),
    );
  });

  it("asks for a review while an adopted condition pins the file", () => {
    const view = projectAssetErasure(
      receipt(
        "outstanding",
        {
          diagnosis_records: {
            state: "blocked",
            failure_code: "diagnosis_review_required",
            outstanding_count: 1,
          },
        },
        [
          {
            subject: "diagnosis_condition",
            subject_id: "00000000-0000-4000-8000-0000000000aa",
          },
        ],
      ),
    );
    expect(view.phase).toBe("review");
    expect(view.reviews).toEqual([
      "诊断条目：请在诊断窗口修改诊断或撤回该条目",
    ]);
  });

  it("shows in progress while pending or running", () => {
    expect(projectAssetErasure(receipt("running")).phase).toBe("in_progress");
    expect(projectAssetErasure(receipt("pending")).phase).toBe("in_progress");
  });
});
