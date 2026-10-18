import { describe, expect, it } from "vitest";
import {
  formatDosage,
  projectTrainingPlans,
  readSelectedPlanId,
  resolveSelectedPlanId,
  SELECTED_PLAN_STORAGE_KEY,
  writeSelectedPlanId,
} from "./trainingPlans";
import { makeWorkspace } from "./__fixtures__/diagnosisFixtures";
import type { TrainingPlan } from "@/features/training/services/trainingService";
function plan(overrides: Partial<TrainingPlan> = {}): TrainingPlan {
  return {
    id: "plan-1",
    status: "active",
    goal: "肩胛稳定训练",
    duration_weeks: 6,
    current_week: 1,
    phases: [],
    created_at: "2026-10-11T00:00:00Z",
    ...overrides,
  };
}

describe("training plan projection uses real plan identity", () => {
  it("has no plan when the user has no plans (empty, not a demo plan)", () => {
    const result = projectTrainingPlans(makeWorkspace(), [], null);
    expect(result.selected).toBeNull();
    expect(result.plans).toEqual([]);
  });

  it("honours a persisted planId that still exists", () => {
    const plans = [plan({ id: "plan-1" }), plan({ id: "plan-2" })];
    expect(resolveSelectedPlanId(plans, "plan-2")).toBe("plan-2");
  });

  it("falls back to an active plan when the persisted id is stale", () => {
    const plans = [
      plan({ id: "plan-1", status: "superseded" }),
      plan({ id: "plan-2", status: "active" }),
    ];
    expect(resolveSelectedPlanId(plans, "deleted-plan")).toBe("plan-2");
    expect(resolveSelectedPlanId(plans, null)).toBe("plan-2");
    expect(resolveSelectedPlanId(plans, undefined)).toBe("plan-2");
  });

  it("flags a missing selected plan instead of silently substituting content", () => {
    const result = projectTrainingPlans(makeWorkspace(), [], "ghost-plan");
    expect(result.selected).toBeNull();
    expect(result.selectedPlanMissing).toBe(false);
  });

  it("lists every real plan with its own title", () => {
    const plans = [
      plan({ id: "plan-1", goal: "肩胛稳定训练" }),
      plan({ id: "plan-2", goal: "臀腿力量训练" }),
    ];
    const result = projectTrainingPlans(makeWorkspace(), plans, "plan-2");
    expect(result.plans.map((item) => item.title)).toEqual([
      "肩胛稳定训练",
      "臀腿力量训练",
    ]);
    expect(result.selected?.planId).toBe("plan-2");
    expect(result.selected?.title).toBe("臀腿力量训练");
  });

  it("does not invent dosage when the projection omits it", () => {
    const plans = [
      plan({
        phases: [
          { week: 1, exercises: [{ name: "胸椎活动", description: "" }] },
        ] as TrainingPlan["phases"],
      }),
    ];
    const selected = projectTrainingPlans(makeWorkspace(), plans, null).selected!;
    const exercise = selected.stages[0].exercises[0];
    expect(exercise.sets).toBeNull();
    expect(exercise.reps).toBeNull();
    expect(formatDosage(exercise)).toBe("剂量未提供");
  });

  it("projects the execution exercises without inventing stages", () => {
    const projectedExercises = [
      {
        intervention_id: "int-9",
        name: "靠墙肩胛滑动",
        description: "缓慢控制",
        sets: "3",
        reps: "10",
      },
    ];
    const plans = [
      plan({
        phases: [
          {
            week: 1,
            focus: "肩胛稳定训练",
            exercises: projectedExercises,
          },
        ] as TrainingPlan["phases"],
      }),
    ];
    const selected = projectTrainingPlans(makeWorkspace(), plans, null).selected!;
    expect(selected.stages.map((stage) => stage.phase)).toEqual(["main"]);
    expect(selected.stages[0].exercises[0].exerciseId).toBe("int-9");
    expect(formatDosage(selected.stages[0].exercises[0])).toBe("3 组 · 10 次");
    expect(selected.frequency).toBeNull();
    expect(selected.stageStructureUnavailable).toBe(true);
  });

  it("marks only the workspace's active execution plan", () => {
    const workspace = makeWorkspace({
      training_plan: {
        id: "plan-1",
        status: "active",
        goal: "肩胛稳定训练",
        duration_weeks: 6,
        current_week: 1,
        phases: [],
        created_at: "Sun 2026-10-11 8:00 AM CST (UTC+08:00)",
      },
    });
    const result = projectTrainingPlans(
      workspace,
      [plan({ id: "plan-1" }), plan({ id: "plan-2" })],
      null,
    );
    expect(result.plans.filter((item) => item.isAccepted).map((item) => item.planId)).toEqual([
      "plan-1",
    ]);
  });
});

describe("selected plan persistence", () => {
  function memoryStorage() {
    const store = new Map<string, string>();
    return {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => void store.set(key, value),
      removeItem: (key: string) => void store.delete(key),
      has: (key: string) => store.has(key),
    };
  }

  it("round-trips the last opened plan", () => {
    const storage = memoryStorage();
    writeSelectedPlanId("plan-7", storage);
    expect(storage.has(SELECTED_PLAN_STORAGE_KEY)).toBe(true);
    expect(readSelectedPlanId(storage)).toBe("plan-7");
  });

  it("clears the stored plan when asked", () => {
    const storage = memoryStorage();
    writeSelectedPlanId("plan-7", storage);
    writeSelectedPlanId(null, storage);
    expect(readSelectedPlanId(storage)).toBeNull();
  });

  it("returns null for blank or throwing storage", () => {
    expect(readSelectedPlanId({ getItem: () => "   " })).toBeNull();
    expect(
      readSelectedPlanId({
        getItem: () => {
          throw new Error("blocked");
        },
      }),
    ).toBeNull();
  });
});
