import type { AssetErasureReceipt } from "@/features/profile/services/assetErasureService";

/** Receipt phases the user sees; `erased` is shown only for a fully verified receipt. */
export type AssetErasurePhase =
  "in_progress" | "outstanding" | "review" | "erased";

export interface AssetErasureLine {
  target: string;
  label: string;
  reason: string;
}

export interface AssetErasureView {
  phase: AssetErasurePhase;
  title: string;
  summary: string;
  outstanding: AssetErasureLine[];
  reviews: string[];
}

export const ASSET_ERASURE_TARGET_LABELS: Record<string, string> = {
  source_objects: "原始文件及其存储副本",
  derived_jobs: "识别与分析任务的结果",
  posture_analyses: "体态分析记录",
  diagnosis_records: "诊断条目",
  document_extractions: "报告识别结果与指标复核",
  assessment_reports: "评估报告副本",
  runtime_checkpoints: "对话运行缓存",
  asset_manifest: "文件记录",
};

const FAILURE_REASONS: Record<string, string> = {
  storage_unavailable: "文件存储暂不可用，稍后会自动重试",
  object_delete_failed: "文件删除未成功，稍后会自动重试",
  object_still_present: "文件仍可读取，稍后会自动重试",
  derived_work_active: "识别或分析仍在进行，完成后会自动重试",
  diagnosis_review_required:
    "仍被已采纳的诊断条目引用，请先修改诊断或撤回该条目",
  diagnosis_evidence_pinned: "已采纳的诊断条目仍引用它，复核诊断后才能清除",
  assessment_replay_frozen: "引用它的评估报告要等诊断复核后才能删除",
  runtime_thread_busy: "对话正在进行，空闲后会自动重试",
  runtime_unavailable: "对话运行服务暂不可用，稍后会自动重试",
  runtime_erasure_unverified: "对话缓存尚未验证清除，稍后会自动重试",
  targets_outstanding: "等待其他项目完成后再删除",
  copy_reappeared: "发现新的副本，正在重新清除",
  execution_error: "处理时出错，稍后会自动重试",
};

const REVIEW_LABELS: Record<string, string> = {
  diagnosis_condition: "诊断条目：请在诊断窗口修改诊断或撤回该条目",
  training_plan: "训练计划：请修订或结束这个计划",
};

const BLOCKING = new Set(["blocked", "unsupported"]);

/** Pure projection of one asset erasure receipt for the upload list. */
export function projectAssetErasure(
  receipt: AssetErasureReceipt,
): AssetErasureView {
  const outstanding: AssetErasureLine[] = [];
  let blocked = false;
  for (const target of receipt.targets) {
    if (target.state === "verified") continue;
    // The manifest waits on the others (targets_outstanding): that alone is
    // not something the user has to review.
    if (
      BLOCKING.has(target.state) &&
      target.failure_code !== "targets_outstanding"
    )
      blocked = true;
    outstanding.push({
      target: target.target,
      label: ASSET_ERASURE_TARGET_LABELS[target.target] ?? target.target,
      reason:
        FAILURE_REASONS[target.failure_code ?? ""] ??
        (target.state === "pending" ? "等待处理" : "处理中"),
    });
  }
  const reviews = receipt.reviews.map(
    (review) => REVIEW_LABELS[review.subject] ?? review.subject,
  );
  const base = { outstanding, reviews };
  switch (receipt.status) {
    case "erased":
      return {
        ...base,
        phase: "erased",
        title: "已彻底删除",
        summary: "原始文件和它的所有副本都已删除并逐项验证。",
      };
    case "pending":
    case "running":
      return {
        ...base,
        phase: "in_progress",
        title: "正在删除",
        summary: "正在逐项删除并验证这个文件的副本。",
      };
    default:
      if (blocked || reviews.length > 0) {
        return {
          ...base,
          phase: "review",
          title: "需要你先处理",
          summary:
            "下面的内容仍依赖这个文件。处理后再次检查即可继续删除，我们不会替你修改它们。",
        };
      }
      return {
        ...base,
        phase: "outstanding",
        title: "尚未完成",
        summary: "部分副本还在清除中，后台会自动重试。",
      };
  }
}

/**
 * Penpot "Assets / Erasure" rows: the receipt's targets folded into the three
 * families the scenes name. A family is verified only when every one of its
 * targets is; the first unfinished target supplies the reason. Counts come
 * from the receipt (erased + outstanding copies), never estimated up front.
 */
export type AssetErasureFamilyId = "original" | "derived" | "checkpoints";

export interface AssetErasureFamily {
  id: AssetErasureFamilyId;
  label: string;
  verified: boolean;
  copies: number;
  reason: string | null;
}

export const ASSET_ERASURE_FAMILIES: ReadonlyArray<{
  id: AssetErasureFamilyId;
  label: string;
  targets: readonly string[];
}> = [
  { id: "original", label: "原件", targets: ["source_objects", "asset_manifest"] },
  {
    id: "derived",
    label: "派生内容",
    targets: [
      "derived_jobs",
      "posture_analyses",
      "document_extractions",
      "diagnosis_records",
      "assessment_reports",
    ],
  },
  { id: "checkpoints", label: "检查点副本", targets: ["runtime_checkpoints"] },
];

export function groupAssetErasure(
  receipt: AssetErasureReceipt,
): AssetErasureFamily[] {
  return ASSET_ERASURE_FAMILIES.map((family) => {
    const rows = receipt.targets.filter((target) =>
      family.targets.includes(target.target),
    );
    const open = rows.find((row) => row.state !== "verified");
    return {
      id: family.id,
      label: family.label,
      verified: !open,
      copies: rows.reduce(
        (sum, row) => sum + row.erased_count + row.outstanding_count,
        0,
      ),
      reason: open
        ? (FAILURE_REASONS[open.failure_code ?? ""] ??
          (open.state === "pending" ? "等待处理" : "处理中"))
        : null,
    };
  });
}
