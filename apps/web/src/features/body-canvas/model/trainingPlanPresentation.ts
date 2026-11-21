import type {
  Gate,
  PlanExerciseRecord,
  PlanStageRecord,
  TrainingGateView,
  TrainingTone,
} from "./trainingPlanViewTypes";
import type { TrainingPlanRecord } from "@/features/training/services/trainingService";

/**
 * User-facing wording for the TrainingPlan window: stage/status/side labels,
 * prescription formatting and the TrainerGate projection. Nothing here may
 * invent a value the server did not send.
 */

export const STAGE_LABELS: Record<PlanStageRecord["phase"], string> = {
  warmup: "热身",
  main: "主训练",
  cooldown: "放松",
};

export const SIDE_LABELS: Record<
  NonNullable<PlanExerciseRecord["side"]>,
  string
> = {
  left: "左侧",
  right: "右侧",
  both: "两侧",
  alternating: "左右交替",
};

export const STATUS_LABELS: Record<
  TrainingPlanRecord["status"],
  { label: string; tone: TrainingTone }
> = {
  active: { label: "启用中", tone: "ok" },
  paused: { label: "已暂停", tone: "idle" },
  completed: { label: "已完成", tone: "idle" },
};

/** Go TrainerGate reasons, worded for the user; unknown codes stay generic. */
const GATE_REASON_LABELS: Record<string, string> = {
  safety_state_unavailable: "安全状态暂时无法读取",
  safety_state_malformed: "安全状态记录异常",
  safety_screening_incomplete: "还没有完成安全筛查",
  urgent_restriction: "有需要尽快就医的安全提醒",
  restricted_restriction: "有限制训练的安全提醒",
  advisory_restriction: "有需要留意的安全提醒",
  custom_exercise_unverifiable: "包含无法核对安全性的自定义动作",
  review_point_reached: "已到安全复查节点",
  restriction_not_yet_effective: "有尚未生效的安全限制",
  invalid_trainer_action: "训练动作无法核对",
};

export function gateReasonLabel(reason: string): string {
  return GATE_REASON_LABELS[reason] ?? "安全检查未通过";
}

export function projectGate(gate: Gate): TrainingGateView {
  const reasons = [
    ...(gate.urgentCare ? ["有需要尽快就医的安全提醒"] : []),
    ...gate.reasons.map(gateReasonLabel),
    ...(gate.requiresReview ? ["需要先复查安全状态"] : []),
  ].filter((reason, index, all) => all.indexOf(reason) === index);
  const permits =
    gate.permitsExecution &&
    (gate.result === "allow" || gate.result === "warn");
  return {
    result: gate.result,
    permits,
    tone: !permits ? "bad" : gate.result === "warn" ? "warn" : "ok",
    label: permits ? "可以开始" : "暂不能开始",
    reasons:
      permits && gate.result === "allow"
        ? []
        : reasons.length
          ? reasons
          : permits
            ? []
            : ["安全检查未通过"],
    safetyRevision: gate.safetyRevision,
  };
}

/** "3 组 × 12 次", "3 组 × 8–12 次", "1 组 × 30 秒", "2 组 × 400 米". */
export function formatPrescription(exercise: PlanExerciseRecord): string {
  const { target } = exercise;
  let amount = "";
  if (target.kind === "repetitions" && target.min !== undefined) {
    amount =
      target.max !== undefined && target.max !== target.min
        ? `${target.min}–${target.max} 次`
        : `${target.min} 次`;
  } else if (target.kind === "duration" && target.seconds !== undefined) {
    amount = `${target.seconds} 秒`;
  } else if (target.kind === "distance" && target.meters !== undefined) {
    amount = `${target.meters} 米`;
  }
  return amount ? `${exercise.sets} 组 × ${amount}` : `${exercise.sets} 组`;
}

export function formatRest(exercise: PlanExerciseRecord): string | null {
  return exercise.restSeconds !== undefined
    ? `休息 ${exercise.restSeconds} 秒`
    : null;
}
