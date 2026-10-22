import type { SafetyRestrictionRecord } from "@/features/consultation/services/safetyRestrictionService";

/**
 * Penpot Safety / Restricted. Go owns every restriction, its reason and its
 * lift conditions; the window only shows them and never lifts anything.
 */
export interface SafetyRestrictionView {
  id: string;
  urgent: boolean;
  statusLabel: string;
  scopeLabel: string;
  reason: string;
  meta: string;
  liftConditions: string[];
  liftPolicyLabel: string;
  scopeNote: string;
}

const CONCEPT_LABELS: Record<SafetyRestrictionRecord["concept"], string> = {
  trauma: "外伤或创伤",
  radiating_pain: "放射痛",
  numbness: "麻木",
  weakness: "无力",
  dizziness: "头晕",
};

const LIFT_POLICY_LABELS: Record<
  SafetyRestrictionRecord["lift_policy"],
  string
> = {
  on_expiry: "到期后自动解除",
  user_reverification: "需要你重新确认以下情况",
  professional_clearance: "需要医生或治疗师确认可以恢复训练",
};

function dateLabel(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

export function projectSafetyRestrictions(
  restrictions: readonly SafetyRestrictionRecord[],
): SafetyRestrictionView[] {
  const rank = { urgent: 0, restricted: 1, advisory: 2 } as const;
  return restrictions
    .filter((item) => item.active)
    .sort(
      (a, b) =>
        rank[a.risk_level] - rank[b.risk_level] ||
        Date.parse(b.valid_from) - Date.parse(a.valid_from),
    )
    .map((item) => {
      const urgent = item.risk_level === "urgent";
      const scopeLabel =
        item.scope_kind === "all_training"
          ? "全部训练"
          : item.scope_kind === "plan"
            ? "当前计划"
            : `${item.exercise_ids.length} 个动作`;
      const until =
        item.lift_policy === "on_expiry" && item.valid_until
          ? `（${dateLabel(item.valid_until)}）`
          : "";
      return {
        id: item.id,
        urgent,
        statusLabel: urgent
          ? "建议立即就医"
          : item.risk_level === "advisory"
            ? "注意"
            : "暂不能执行",
        scopeLabel,
        reason: item.reason,
        meta: [
          CONCEPT_LABELS[item.concept],
          dateLabel(item.valid_from),
          "需要复核",
        ]
          .filter(Boolean)
          .join(" · "),
        liftConditions: item.lift_conditions,
        liftPolicyLabel: LIFT_POLICY_LABELS[item.lift_policy] + until,
        scopeNote:
          item.scope_kind === "exercise"
            ? "计划仍启用 · 其他动作按各自范围核验"
            : item.scope_kind === "plan"
              ? "这个计划暂停 · 其他计划按各自范围核验"
              : "所有训练暂停，直到解除条件满足",
      };
    });
}
