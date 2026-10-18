import type { StatementErasureReceipt } from "@/features/profile/services/statementErasureService";

/**
 * Receipt phases the user sees. `retained` is a finished erasure that keeps
 * something by policy (frozen decision inputs, a chat message another record
 * cites); it is never shown as a full erasure.
 */
export type StatementErasurePhase =
  "in_progress" | "outstanding" | "review" | "erased" | "retained";

export interface StatementErasureLine {
  target: string;
  label: string;
  reason: string;
  retained: boolean;
}

export interface StatementErasureReviewItem {
  subject: string;
  subjectId: string;
  label: string;
  /** The review is this very record: the user can withdraw it here. */
  isThisRecord: boolean;
}

export interface StatementErasureView {
  phase: StatementErasurePhase;
  title: string;
  summary: string;
  outstanding: StatementErasureLine[];
  retained: StatementErasureLine[];
  reviews: StatementErasureReviewItem[];
  canWithdrawHere: boolean;
}

export const STATEMENT_ERASURE_TARGET_LABELS: Record<string, string> = {
  diagnosis_records: "诊断条目",
  training_plans: "训练计划",
  body_records: "由它整理出的身体记录",
  assessment_reports: "评估报告副本",
  decision_records: "冻结的诊断与计划决策输入",
  runtime_checkpoints: "对话运行缓存",
  source_message: "陈述所在的聊天消息",
  statement_manifest: "陈述原文",
};

const FAILURE_REASONS: Record<string, string> = {
  diagnosis_review_required:
    "仍被已采纳的诊断条目引用，请先修改诊断或撤回该条目",
  diagnosis_evidence_pinned: "已采纳的诊断条目仍引用它，复核诊断后才能清除",
  training_plan_review_required: "进行中的训练计划依赖它，请先修订或结束计划",
  body_record_active: "由它整理出的记录仍有效，请先撤回",
  assessment_replay_frozen: "引用它的评估报告要等诊断复核后才能删除",
  runtime_thread_busy: "对话正在进行，空闲后会自动重试",
  runtime_unavailable: "对话运行服务暂不可用，稍后会自动重试",
  runtime_erasure_unverified: "对话缓存尚未验证清除，稍后会自动重试",
  source_message_busy: "这条消息所在的对话还在进行，空闲后会自动重试",
  targets_outstanding: "等待其他项目完成后再删除",
  copy_reappeared: "发现新的副本，正在重新清除",
  execution_error: "处理时出错，稍后会自动重试",
};

const RETENTION_REASONS: Record<string, string> = {
  replay_input_frozen:
    "为保证已作出的诊断和计划可按原样复核，冻结的决策输入不会被改写；注销账户时一并删除",
  chat_record_cited:
    "这条聊天消息仍被其他记录引用，作为对话记录保留；随聊天记录到期清理或注销账户时删除",
};

const REVIEW_LABELS: Record<string, string> = {
  diagnosis_condition: "诊断条目：请在诊断窗口修改诊断或撤回该条目",
  training_plan: "训练计划：请修订或结束这个计划",
  body_fact: "身体记录：请先撤回",
  body_observation: "测量记录：请先撤回",
  body_hypothesis: "推测记录：请先撤回",
};

const BLOCKING = new Set(["blocked", "unsupported"]);

/** Pure projection of one statement erasure receipt for the records window. */
export function projectStatementErasure(
  receipt: StatementErasureReceipt,
  recordId: string,
): StatementErasureView {
  const outstanding: StatementErasureLine[] = [];
  const retained: StatementErasureLine[] = [];
  let blocked = false;
  for (const target of receipt.targets) {
    const label =
      STATEMENT_ERASURE_TARGET_LABELS[target.target] ?? target.target;
    if (target.state === "retained_by_policy") {
      retained.push({
        target: target.target,
        label,
        reason:
          RETENTION_REASONS[target.retention_reason ?? ""] ?? "按政策保留",
        retained: true,
      });
    } else if (target.state !== "verified") {
      if (BLOCKING.has(target.state)) blocked = true;
      outstanding.push({
        target: target.target,
        label,
        reason:
          FAILURE_REASONS[target.failure_code ?? ""] ??
          (target.state === "pending" ? "等待处理" : "处理中"),
        retained: false,
      });
    }
  }
  const reviews = receipt.reviews.map((review) => ({
    subject: review.subject,
    subjectId: review.subject_id,
    label: REVIEW_LABELS[review.subject] ?? review.subject,
    isThisRecord:
      review.subject === "body_fact" && review.subject_id === recordId,
  }));
  const base = {
    outstanding,
    retained,
    reviews,
    canWithdrawHere: reviews.some((review) => review.isThisRecord),
  };
  switch (receipt.status) {
    case "erased":
      return {
        ...base,
        phase: "erased",
        title: "已彻底删除",
        summary: "这条陈述及其所有副本都已删除并逐项验证。",
      };
    case "erased_with_retention":
      return {
        ...base,
        phase: "retained",
        title: "已删除，部分内容按政策保留",
        summary:
          "陈述原文和其他副本已删除并验证；以下内容按政策保留，并非全部删除。",
      };
    case "pending":
    case "running":
      return {
        ...base,
        phase: "in_progress",
        title: "正在删除",
        summary: "正在逐项删除并验证这条陈述的副本。",
      };
    default:
      if (blocked || reviews.length > 0) {
        return {
          ...base,
          phase: "review",
          title: "需要你先处理",
          summary:
            "下面的内容仍依赖这条陈述。处理后再次检查即可继续删除，我们不会替你修改它们。",
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
