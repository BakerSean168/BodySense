import type { TrainingPlanRecord } from "@/features/training/services/trainingService";

/**
 * The light part of the TrainingPlan window state that the always-loaded
 * workspace needs (header picker, Trainer entry guard, plan persistence).
 * The full view model lives with the lazily loaded window.
 */

export interface TrainingPlanSummary {
  planId: string;
  title: string;
  status: TrainingPlanRecord["status"];
}

export interface TrainingSelection {
  plans: TrainingPlanSummary[];
  selected:
    | (TrainingPlanSummary & {
        gatePermits: boolean;
        templates: { templateId: string; title: string }[];
      })
    | null;
}

/**
 * A persisted id is honoured only when it still exists for this user;
 * otherwise the active plan, then the first plan, wins.
 */
export function resolveSelectedPlanId(
  plans: readonly Pick<TrainingPlanRecord, "planId" | "status">[],
  preferredPlanId: string | null | undefined,
): string | null {
  if (plans.length === 0) return null;
  if (preferredPlanId && plans.some((plan) => plan.planId === preferredPlanId))
    return preferredPlanId;
  return (plans.find((plan) => plan.status === "active") ?? plans[0]!).planId;
}

export function selectTrainingPlan(
  plans: readonly TrainingPlanRecord[] | undefined,
  preferredPlanId: string | null,
): TrainingSelection {
  const records = plans ?? [];
  const id = resolveSelectedPlanId(records, preferredPlanId);
  const record = id ? records.find((plan) => plan.planId === id) : undefined;
  return {
    plans: records.map(({ planId, title, status }) => ({
      planId,
      title,
      status,
    })),
    selected: record
      ? {
          planId: record.planId,
          title: record.title,
          status: record.status,
          gatePermits: record.executionGate.permitsExecution,
          templates: record.currentContent.sessions.map(
            ({ templateId, title }) => ({ templateId, title }),
          ),
        }
      : null,
  };
}

/**
 * Stateless, versioned persistence of the last explicitly opened plan so the
 * Dock reopen restores the user's plan instead of a prototype default.
 */
export const SELECTED_PLAN_STORAGE_KEY = "bodysense.training.selectedPlanId";

export function readSelectedPlanId(
  storage?: Pick<Storage, "getItem">,
): string | null {
  const target =
    storage ??
    (typeof window === "undefined" ? undefined : window.localStorage);
  if (!target) return null;
  try {
    const value = target.getItem(SELECTED_PLAN_STORAGE_KEY);
    return value && value.trim() ? value : null;
  } catch {
    return null;
  }
}

export function writeSelectedPlanId(
  planId: string | null,
  storage?: Pick<Storage, "setItem"> & Partial<Pick<Storage, "removeItem">>,
): void {
  const target =
    storage ??
    (typeof window === "undefined" ? undefined : window.localStorage);
  if (!target) return;
  try {
    if (planId) target.setItem(SELECTED_PLAN_STORAGE_KEY, planId);
    else target.removeItem?.(SELECTED_PLAN_STORAGE_KEY);
  } catch {
    // A blocked storage must never break the plan window.
  }
}
