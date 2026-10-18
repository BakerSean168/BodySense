import { describe, expect, it } from "vitest";
import type { StatementErasureReceipt } from "@/features/profile/services/statementErasureService";
import { projectStatementErasure } from "./statementErasureViewModel";

const targets = [
  "diagnosis_records",
  "training_plans",
  "body_records",
  "assessment_reports",
  "decision_records",
  "runtime_checkpoints",
  "source_message",
  "statement_manifest",
] as const;

function receipt(
  status: StatementErasureReceipt["status"],
  overrides: Partial<
    Record<
      (typeof targets)[number],
      Partial<StatementErasureReceipt["targets"][number]>
    >
  > = {},
  reviews: StatementErasureReceipt["reviews"] = [],
): StatementErasureReceipt {
  return {
    request_id: "00000000-0000-4000-8000-000000000001",
    statement_id: "00000000-0000-4000-8000-000000000002",
    status,
    inventory_revision: "statement-erasure-inventory-v2",
    requested_at: "2026-10-19T00:00:00Z",
    targets: targets.map((target) => ({
      target,
      state: "verified",
      erased_count: 0,
      outstanding_count: 0,
      ...overrides[target],
    })),
    outstanding_targets: [],
    retained_targets: [],
    reviews,
  };
}

const record = "00000000-0000-4000-8000-0000000000aa";

describe("projectStatementErasure", () => {
  it("asks for a review and offers withdrawing this record here", () => {
    const view = projectStatementErasure(
      receipt(
        "outstanding",
        {
          body_records: {
            state: "blocked",
            failure_code: "body_record_active",
            outstanding_count: 1,
          },
          statement_manifest: {
            state: "blocked",
            failure_code: "targets_outstanding",
          },
        },
        [{ subject: "body_fact", subject_id: record }],
      ),
      record,
    );
    expect(view.phase).toBe("review");
    expect(view.canWithdrawHere).toBe(true);
    expect(view.outstanding.map((line) => line.reason)).toContain(
      "由它整理出的记录仍有效，请先撤回",
    );
  });

  it("keeps a transient failure outstanding, not a review", () => {
    const view = projectStatementErasure(
      receipt("outstanding", {
        runtime_checkpoints: {
          state: "failed",
          failure_code: "runtime_thread_busy",
        },
        statement_manifest: {
          state: "blocked",
          failure_code: "targets_outstanding",
        },
      }),
      record,
    );
    expect(view.phase).toBe("outstanding");
    expect(view.outstanding[0].reason).toMatch(/自动重试/);
  });

  it("never reports a policy retention as a full erasure", () => {
    const view = projectStatementErasure(
      receipt("erased_with_retention", {
        decision_records: {
          state: "retained_by_policy",
          retention_reason: "replay_input_frozen",
          outstanding_count: 3,
        },
      }),
      record,
    );
    expect(view.phase).toBe("retained");
    expect(view.title).not.toBe("已彻底删除");
    expect(view.retained).toHaveLength(1);
    expect(view.outstanding).toHaveLength(0);
  });

  it("names a source chat message another record still cites", () => {
    const view = projectStatementErasure(
      receipt("erased_with_retention", {
        source_message: {
          state: "retained_by_policy",
          retention_reason: "chat_record_cited",
          outstanding_count: 1,
        },
      }),
      record,
    );
    expect(view.phase).toBe("retained");
    expect(view.retained).toEqual([
      expect.objectContaining({
        target: "source_message",
        label: "陈述所在的聊天消息",
        reason: expect.stringMatching(/仍被其他记录引用/),
      }),
    ]);
  });

  it("retries a source message whose conversation is still running", () => {
    const view = projectStatementErasure(
      receipt("outstanding", {
        source_message: {
          state: "failed",
          failure_code: "source_message_busy",
          outstanding_count: 1,
        },
      }),
      record,
    );
    expect(view.phase).toBe("outstanding");
    expect(view.outstanding[0].reason).toMatch(/自动重试/);
  });

  it("shows in progress and done", () => {
    expect(projectStatementErasure(receipt("running"), record).phase).toBe(
      "in_progress",
    );
    expect(projectStatementErasure(receipt("erased"), record).phase).toBe(
      "erased",
    );
  });
});
