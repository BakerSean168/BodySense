import type { BodyFactRecord } from "@/features/profile/services/bodyFactRecordsService";

export type BodyFactKind = BodyFactRecord["kind"];

/** Labels are presentation only; their discriminators come from the service contract. */
export const BODY_FACT_KIND_LABELS: Record<BodyFactKind, string> = {
  symptom: "身体感受",
  health_history: "健康经历",
  activity: "活动记录",
  lifestyle: "生活习惯",
  training_feedback: "训练反馈",
};
