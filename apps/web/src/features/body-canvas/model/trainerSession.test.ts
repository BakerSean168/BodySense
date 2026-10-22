import { describe, expect, it } from "vitest";
import type { TrainingSessionRecord } from "@/features/training/services/trainingService";
import {
  effectiveActions,
  elapsedTrainingSeconds,
  formatClock,
  formatRestClock,
  projectTrainerSession,
} from "./trainerSession";
import {
  PLAN_ID,
  REVISION_ID,
  makePlan,
} from "./__fixtures__/trainingPlanFixtures";

const SESSION_ID = "88888888-8888-4888-8888-888888888888";
const gate = {
  result: "allow" as const,
  safetyRevision: 4,
  policyRevision: "p1",
};
let seq = 0;

function action(
  planExerciseId: string,
  setNumber: number | undefined,
  kind: "performed" | "skipped" | "corrected",
  overrides: Partial<TrainingSessionRecord["actions"][number]> = {},
): TrainingSessionRecord["actions"][number] {
  seq += 1;
  return {
    actionId: `00000000-0000-4000-8000-${String(seq).padStart(12, "0")}`,
    sessionId: SESSION_ID,
    planExerciseId,
    exerciseId: planExerciseId,
    scope: setNumber === undefined ? "exercise" : "set",
    setNumber,
    kind,
    sessionRevision: seq,
    gate,
    idempotencyKey: `k${seq}`,
    recordedAt: "2026-10-18T12:05:00+08:00",
    ...overrides,
  };
}

function session(
  overrides: Partial<TrainingSessionRecord> = {},
): TrainingSessionRecord {
  return {
    sessionId: SESSION_ID,
    planId: PLAN_ID,
    planRevisionId: REVISION_ID,
    templateId: "a",
    timeZone: "Asia/Shanghai",
    trainingDate: "2026-10-18",
    state: "in_progress",
    revision: 0,
    startedAt: "2026-10-18T12:00:00+08:00",
    startGate: gate,
    actions: [],
    transitions: [],
    ...overrides,
  };
}

const plan = makePlan();
const at = (time: string) => new Date(`2026-10-18T${time}+08:00`);

describe("Trainer session view model", () => {
  it("starts at the first set of the first exercise with the prescription prefilled", () => {
    const view = projectTrainerSession(
      plan,
      plan.currentContent,
      session(),
      at("12:00:30"),
    );
    expect(view.templateTitle).toBe("A · 基础课");
    expect(view.canRecord).toBe(true);
    expect(view.current).toMatchObject({
      exerciseIndex: 0,
      setNumber: 1,
      defaultActual: 10,
      actualUnit: "reps",
    });
    expect(view.exercises[0]).toMatchObject({
      name: "猫牛式",
      stageLabel: "热身",
      position: 1,
      stageSize: 1,
    });
    expect(view.next?.name).toBe("臀桥");
    expect(view.elapsedSeconds).toBe(30);
  });

  it("follows effective receipts only: corrections retract, an exercise skip covers its sets", () => {
    const first = action("bridge", 1, "performed", { actualReps: 12 });
    const actions = [
      action("cat-cow", 1, "performed"),
      first,
      action("bridge", 1, "corrected", { correctsActionId: first.actionId }),
      action("wall-sit", undefined, "skipped"),
    ];
    expect(
      effectiveActions(actions).map((item) => item.planExerciseId),
    ).toEqual(["cat-cow", "wall-sit"]);
    const view = projectTrainerSession(
      plan,
      plan.currentContent,
      session({ actions }),
      at("12:10:00"),
    );
    expect(
      view.exercises.find((item) => item.planExerciseId === "bridge")!.sets,
    ).toEqual(["pending", "pending", "pending"]);
    expect(
      view.exercises.find((item) => item.planExerciseId === "wall-sit")!
        .progress,
    ).toBe("done");
    expect(view.current).toMatchObject({ exerciseIndex: 1, setNumber: 1 });
  });

  it("keeps recorded sets under a whole-exercise skip and skips only the open ones", () => {
    const warmup = action("cat-cow", 1, "performed");
    const set1 = action("bridge", 1, "performed", { actualReps: 12 });
    const skip = action("bridge", undefined, "skipped");
    const view = projectTrainerSession(
      plan,
      plan.currentContent,
      session({ actions: [warmup, set1, skip] }),
      at("12:10:00"),
    );
    const bridge = view.exercises.find(
      (item) => item.planExerciseId === "bridge",
    )!;
    expect(bridge.sets).toEqual(["performed", "skipped", "skipped"]);
    expect(bridge).toMatchObject({
      progress: "done",
      performedCount: 1,
      exerciseSkipActionId: skip.actionId,
    });
    expect(bridge.setActionIds).toEqual([set1.actionId, null, null]);
    expect(view.current).toMatchObject({ exerciseIndex: 2, setNumber: 1 });
    expect(view.last).toMatchObject({
      actionId: skip.actionId,
      scope: "exercise",
      kind: "skipped",
      setNumber: null,
    });
  });

  it("reopens a corrected set prefilled with the retracted value and shows the retraction", () => {
    const warmup = action("cat-cow", 1, "performed");
    const first = action("bridge", 1, "performed", { actualReps: 14 });
    const correction = action("bridge", 1, "corrected", {
      correctsActionId: first.actionId,
    });
    const view = projectTrainerSession(
      plan,
      plan.currentContent,
      session({ actions: [warmup, first, correction] }),
      at("12:10:00"),
    );
    expect(view.current).toMatchObject({
      exerciseIndex: 1,
      setNumber: 1,
      defaultActual: 14,
      correcting: true,
    });
    expect(view.last).toMatchObject({
      kind: "corrected",
      setNumber: 1,
      name: "臀桥",
    });
    // A retraction never starts a rest.
    expect(view.restRemainingSeconds).toBe(0);
    const fresh = projectTrainerSession(
      plan,
      plan.currentContent,
      session(),
      at("12:00:10"),
    );
    expect(fresh.current).toMatchObject({
      correcting: false,
      defaultActual: 10,
    });
  });

  it("carries the user's verbatim session feedback", () => {
    const view = projectTrainerSession(
      plan,
      plan.currentContent,
      session({
        state: "ended",
        endedReason: "finished",
        feedback: {
          feedbackId: "f1",
          text: "膝盖有点酸",
          idempotencyKey: "fk",
          recordedAt: at("12:40:00").toISOString(),
        },
      }),
      at("12:41:00"),
    );
    expect(view.feedback?.text).toBe("膝盖有点酸");
    expect(
      projectTrainerSession(
        plan,
        plan.currentContent,
        session(),
        at("12:01:00"),
      ).feedback,
    ).toBeNull();
  });

  it("shows the Set recorded rest countdown from the receipt time and the exercise's rest", () => {
    const actions = [
      action("cat-cow", 1, "performed"),
      action("bridge", 1, "performed", {
        actualReps: 12,
        recordedAt: "2026-10-18T12:05:00+08:00",
      }),
    ];
    const resting = projectTrainerSession(
      plan,
      plan.currentContent,
      session({ actions }),
      at("12:05:13"),
    );
    expect(resting.last).toMatchObject({
      name: "臀桥",
      setNumber: 1,
      kind: "performed",
      actualReps: 12,
    });
    expect(resting.restRemainingSeconds).toBe(32);
    expect(resting.current).toMatchObject({
      exerciseIndex: 1,
      setNumber: 2,
      defaultActual: 12,
    });
    expect(
      projectTrainerSession(
        plan,
        plan.currentContent,
        session({ actions }),
        at("12:06:00"),
      ).restRemainingSeconds,
    ).toBe(0);
  });

  it("never records while the gate blocks or the session is not in progress", () => {
    const blocked = makePlan({
      executionGate: {
        ...plan.executionGate,
        result: "block",
        permitsExecution: false,
        reasons: ["restricted_restriction"],
      },
    });
    expect(
      projectTrainerSession(
        blocked,
        plan.currentContent,
        session(),
        at("12:01:00"),
      ).canRecord,
    ).toBe(false);
    expect(
      projectTrainerSession(
        plan,
        plan.currentContent,
        session({ state: "paused" }),
        at("12:01:00"),
      ).canRecord,
    ).toBe(false);
    // The pinned revision is unavailable: nothing can be recorded against a guess.
    expect(
      projectTrainerSession(plan, null, session(), at("12:01:00")).canRecord,
    ).toBe(false);
  });

  it("excludes paused time from the clock", () => {
    const paused = session({
      state: "paused",
      transitions: [
        {
          transitionId: "t1",
          kind: "pause",
          sessionRevision: 1,
          idempotencyKey: "p",
          recordedAt: "2026-10-18T12:05:00+08:00",
        },
      ],
    });
    expect(elapsedTrainingSeconds(paused, at("12:30:00"))).toBe(300);
    const resumed = session({
      transitions: [
        ...paused.transitions,
        {
          transitionId: "t2",
          kind: "resume",
          sessionRevision: 2,
          idempotencyKey: "r",
          recordedAt: "2026-10-18T12:10:00+08:00",
          gate,
        },
      ],
    });
    expect(elapsedTrainingSeconds(resumed, at("12:12:00"))).toBe(420);
    expect(formatClock(512)).toBe("08:32");
    expect(formatRestClock(32)).toBe("0:32");
  });

  it("marks everything recorded once no set is pending", () => {
    const content = plan.currentContent;
    const actions = content.sessions[0]!.stages.flatMap((stage) =>
      stage.exercises.flatMap((exercise) =>
        Array.from({ length: exercise.sets }, (_, index) =>
          action(exercise.planExerciseId, index + 1, "performed"),
        ),
      ),
    );
    const view = projectTrainerSession(
      plan,
      content,
      session({ actions }),
      at("12:30:00"),
    );
    expect(view.allRecorded).toBe(true);
    expect(view.current).toBeNull();
    expect(view.restRemainingSeconds).toBe(0);
  });
});
