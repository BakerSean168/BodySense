import type { TrainerSessionView } from "@/features/body-canvas/model/trainerSession";

/** "臀桥 · 第 2 组 · 12 次" for an effective receipt shown in this session. */
export function findReceipt(
  view: TrainerSessionView | null,
  actionId: string,
): string | undefined {
  for (const exercise of view?.exercises ?? []) {
    if (exercise.exerciseSkipActionId === actionId)
      return `${exercise.name} · 跳过此动作`;
    const index = exercise.setActionIds.indexOf(actionId);
    if (index >= 0) {
      return `${exercise.name} · 第 ${index + 1} 组${exercise.sets[index] === "skipped" ? " · 跳过" : ""}`;
    }
  }
  return undefined;
}

export function setLabel(state: string, index: number) {
  return `第 ${index + 1} 组${state === "skipped" ? " · 跳过" : ""}`;
}
