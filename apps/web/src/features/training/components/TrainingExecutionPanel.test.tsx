import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiRequestError } from "@/lib/api-client";
import {
  PLAN_ID,
  REVISION_ID,
  makePlan,
} from "@/features/body-canvas/model/__fixtures__/trainingPlanFixtures";
import type { TrainingSessionRecord } from "../services/trainingService";

const api = vi.hoisted(() => ({
  getPlanRecord: vi.fn(),
  listSessions: vi.fn(),
  getRevisionContent: vi.fn(),
  startSession: vi.fn(),
  recordAction: vi.fn(),
  finishSession: vi.fn(),
  pauseSession: vi.fn(),
  resumeSession: vi.fn(),
  recordFeedback: vi.fn(),
}));
vi.mock("../services/trainingService", () => ({
  trainingApi: api,
  newIdempotencyKey: () => crypto.randomUUID(),
}));

import { TrainingExecutionPanel } from "./TrainingExecutionPanel";

const SESSION_ID = "88888888-8888-4888-8888-888888888888";
const gate = {
  result: "allow" as const,
  safetyRevision: 4,
  policyRevision: "p1",
};

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
    startedAt: new Date().toISOString(),
    startGate: gate,
    actions: [],
    transitions: [],
    ...overrides,
  };
}

function performed(
  planExerciseId: string,
  setNumber: number,
  revision: number,
  actualReps?: number,
) {
  return {
    actionId: crypto.randomUUID(),
    sessionId: SESSION_ID,
    planExerciseId,
    exerciseId: planExerciseId,
    scope: "set" as const,
    setNumber,
    kind: "performed" as const,
    actualReps,
    sessionRevision: revision,
    gate,
    idempotencyKey: `k${revision}`,
    recordedAt: new Date().toISOString(),
  };
}

function renderPanel(onOpenPlan = vi.fn(), sessionId?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <TrainingExecutionPanel
        planId={PLAN_ID}
        templateId="a"
        launchKey="launch-1"
        sessionId={sessionId}
        onOpenPlan={onOpenPlan}
      />
    </QueryClientProvider>,
  );
  return onOpenPlan;
}

describe("Penpot Trainer boards", () => {
  beforeEach(() => {
    Object.values(api).forEach((fn) => fn.mockReset());
    api.getPlanRecord.mockResolvedValue(makePlan());
  });

  it("retries failed session reads without starting twice", async () => {
    let current: TrainingSessionRecord | null = null;
    api.listSessions
      .mockRejectedValueOnce(new Error("HTTP 503"))
      .mockImplementation(async () => (current ? [current] : []));
    api.startSession.mockImplementation(async () => {
      current = session();
      return current;
    });
    renderPanel();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "训练记录暂时无法读取",
    );
    expect(api.startSession).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
    expect(await screen.findByTestId("trainer-in-progress")).toBeVisible();
    expect(api.startSession).toHaveBeenCalledTimes(1);
  });

  it("starts the launched template once and records the first set with its actual reps", async () => {
    let current = session();
    api.listSessions
      .mockResolvedValueOnce([])
      .mockImplementation(async () => [current]);
    api.startSession.mockImplementation(async () => current);
    renderPanel();
    expect(await screen.findByTestId("trainer-in-progress")).toBeVisible();
    expect(api.startSession).toHaveBeenCalledTimes(1);
    expect(api.startSession.mock.calls[0]![1]).toBe("a");
    expect(screen.getByRole("heading", { name: "猫牛式" })).toBeVisible();
    expect(screen.getByText("热身 · 1 / 1")).toBeVisible();
    expect(screen.getByLabelText("实际次数")).toHaveTextContent("10");
    await userEvent.click(screen.getByRole("button", { name: "增加" }));
    expect(screen.getByLabelText("实际次数")).toHaveTextContent("11");

    api.recordAction.mockImplementation(async () => {
      current = session({
        revision: 1,
        actions: [performed("cat-cow", 1, 1, 11)],
      });
      return current.actions[0];
    });
    await userEvent.click(screen.getByRole("button", { name: "完成本组" }));
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "臀桥" })).toBeVisible(),
    );
    expect(api.recordAction).toHaveBeenCalledTimes(1);
    expect(api.recordAction.mock.calls[0]![0]).toMatchObject({
      planExerciseId: "cat-cow",
      scope: "set",
      setNumber: 1,
      kind: "performed",
      actualReps: 11,
      expectedSafetyRevision: 4,
    });
    expect(screen.getByTestId("trainer-last-receipt")).toHaveTextContent(
      "第 1 组已记录 · 11 次",
    );
  });

  it("picks up this plan's live session instead of starting another", async () => {
    api.listSessions.mockResolvedValue([
      session({ revision: 1, actions: [performed("cat-cow", 1, 1)] }),
    ]);
    renderPanel();
    expect(await screen.findByRole("heading", { name: "臀桥" })).toBeVisible();
    expect(api.startSession).not.toHaveBeenCalled();
  });

  it("keeps recording disabled with the gate's reason while the TrainerGate blocks", async () => {
    api.getPlanRecord.mockResolvedValue(
      makePlan({
        executionGate: {
          ...makePlan().executionGate,
          result: "block",
          permitsExecution: false,
          reasons: ["restricted_restriction"],
        },
      }),
    );
    api.listSessions.mockResolvedValue([session()]);
    renderPanel();
    expect(await screen.findByTestId("trainer-gate")).toHaveTextContent(
      "暂不能继续",
    );
    expect(screen.getByRole("button", { name: "完成本组" })).toBeDisabled();
    expect(screen.getByTestId("trainer-gate-reason")).toHaveTextContent(
      "有限制训练的安全提醒",
    );
    // Stopping is never gated.
    expect(screen.getByRole("button", { name: "结束" })).toBeEnabled();
  });

  it("says the step was not recorded when Safety changed under it", async () => {
    api.listSessions.mockResolvedValue([session()]);
    api.recordAction.mockRejectedValue(
      new ApiRequestError("stale", 409, "SAFETY_STALE"),
    );
    renderPanel();
    await userEvent.click(
      await screen.findByRole("button", { name: "完成本组" }),
    );
    expect(
      await screen.findByTestId("trainer-safety-changed"),
    ).toHaveTextContent("这一步没有记录");
    expect(api.getPlanRecord.mock.calls.length).toBeGreaterThan(1);
  });

  it("ignores a second tap while a set is being recorded", async () => {
    api.listSessions.mockResolvedValue([session()]);
    let resolve!: () => void;
    api.recordAction.mockReturnValue(
      new Promise<void>((done) => (resolve = done)),
    );
    renderPanel();
    const complete = await screen.findByRole("button", { name: "完成本组" });
    await userEvent.dblClick(complete);
    resolve();
    await waitFor(() => expect(api.recordAction).toHaveBeenCalledTimes(1));
  });

  it("pauses into Recovery and resumes at the Safety revision shown", async () => {
    let current = session({
      revision: 1,
      actions: [performed("cat-cow", 1, 1)],
    });
    api.listSessions.mockImplementation(async () => [current]);
    api.pauseSession.mockImplementation(async () => {
      current = { ...current, state: "paused", revision: 2 };
      return current;
    });
    renderPanel();
    await userEvent.click(
      await screen.findByRole("button", { name: "暂停训练" }),
    );
    expect(await screen.findByTestId("trainer-recovery")).toBeVisible();
    expect(screen.getByText("臀桥 · 第 1 组")).toBeVisible();
    api.resumeSession.mockImplementation(async () => {
      current = { ...current, state: "in_progress", revision: 3 };
      return current;
    });
    await userEvent.click(screen.getByRole("button", { name: "继续" }));
    expect(await screen.findByTestId("trainer-in-progress")).toBeVisible();
    expect(api.resumeSession.mock.calls[0]![1]).toBe(4);
  });

  it("finishes early when sets remain and shows the Ended board", async () => {
    let current = session({
      revision: 1,
      actions: [performed("cat-cow", 1, 1)],
    });
    api.listSessions.mockImplementation(async () => [current]);
    api.pauseSession.mockImplementation(async () => {
      current = { ...current, state: "paused", revision: 2 };
      return current;
    });
    api.finishSession.mockImplementation(async () => {
      current = {
        ...current,
        state: "ended",
        endedReason: "stopped_early",
        endedAt: new Date().toISOString(),
        revision: 3,
      };
      return current;
    });
    const open = renderPanel();
    await userEvent.click(
      await screen.findByRole("button", { name: "暂停训练" }),
    );
    await userEvent.click(await screen.findByRole("button", { name: "结束" }));
    expect(await screen.findByTestId("trainer-ended")).toHaveTextContent(
      "提前结束",
    );
    expect(api.finishSession.mock.calls[0]![1]).toBe("stopped_early");
    expect(screen.getByTestId("trainer-recorded")).toHaveTextContent(
      "猫牛式1 / 1 组",
    );
    await userEvent.click(screen.getByRole("button", { name: "完成" }));
    expect(open).toHaveBeenCalled();
  });

  it("retracts a receipt only after the user confirms, then re-records the set", async () => {
    const first = performed("cat-cow", 1, 1, 10);
    const second = performed("bridge", 1, 2, 12);
    let current = session({ revision: 2, actions: [first, second] });
    api.listSessions.mockImplementation(async () => [current]);
    renderPanel();
    expect(await screen.findByTestId("trainer-last-receipt")).toHaveTextContent(
      "第 1 组已记录 · 12 次",
    );
    await userEvent.click(screen.getByRole("button", { name: "更正" }));
    expect(screen.getByTestId("trainer-correction")).toHaveTextContent(
      "撤回「臀桥 · 第 1 组」后重新记录？",
    );
    expect(api.recordAction).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(screen.queryByTestId("trainer-correction")).not.toBeInTheDocument();

    api.recordAction.mockImplementation(async () => {
      const retraction = {
        ...performed("bridge", 1, 3),
        kind: "corrected" as const,
        actualReps: undefined,
        correctsActionId: second.actionId,
      };
      current = session({ revision: 3, actions: [first, second, retraction] });
      return retraction;
    });
    // The recorded set chip opens the same confirmation.
    await userEvent.click(screen.getByRole("button", { name: "更正第 1 组" }));
    await userEvent.click(screen.getByRole("button", { name: "撤回重记" }));
    await waitFor(() =>
      expect(screen.getByTestId("trainer-last-receipt")).toHaveTextContent(
        "第 1 组已撤回 · 请重新记录",
      ),
    );
    expect(api.recordAction.mock.calls[0]![0]).toMatchObject({
      planExerciseId: "bridge",
      scope: "set",
      setNumber: 1,
      kind: "corrected",
      correctsActionId: second.actionId,
      expectedSafetyRevision: 4,
    });
    // The set is open again, prefilled with the value that was retracted.
    expect(screen.getByLabelText("实际次数")).toHaveTextContent("12");
    expect(screen.getByRole("heading", { name: "臀桥" })).toBeVisible();
  });

  it("skips the rest of the current exercise as one receipt", async () => {
    let current = session({
      revision: 1,
      actions: [performed("cat-cow", 1, 1)],
    });
    api.listSessions.mockImplementation(async () => [current]);
    api.recordAction.mockImplementation(async () => {
      const skip = {
        ...performed("bridge", 1, 2),
        scope: "exercise" as const,
        setNumber: undefined,
        kind: "skipped" as const,
      };
      current = session({ revision: 2, actions: [...current.actions, skip] });
      return skip;
    });
    renderPanel();
    await userEvent.click(
      await screen.findByRole("button", { name: "跳过此动作：臀桥" }),
    );
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "靠墙静蹲" })).toBeVisible(),
    );
    expect(api.recordAction.mock.calls[0]![0]).toMatchObject({
      planExerciseId: "bridge",
      scope: "exercise",
      kind: "skipped",
    });
    expect(api.recordAction.mock.calls[0]![0]).not.toHaveProperty("setNumber");
    expect(screen.getByTestId("trainer-last-receipt")).toHaveTextContent(
      "臀桥已跳过",
    );
  });

  it("records 「说说感受」 verbatim once on the Ended board", async () => {
    let current = session({
      state: "ended",
      endedReason: "finished",
      endedAt: new Date().toISOString(),
      revision: 4,
    });
    api.listSessions.mockImplementation(async () => [current]);
    renderPanel(vi.fn(), SESSION_ID);
    await userEvent.click(
      await screen.findByRole("button", { name: "说说感受" }),
    );
    const send = screen.getByRole("button", { name: "发送" });
    expect(send).toBeDisabled();
    await userEvent.type(
      screen.getByLabelText("这次训练感觉怎么样？"),
      "  膝盖有点酸 ",
    );
    api.recordFeedback.mockImplementation(async () => {
      current = {
        ...current,
        feedback: {
          feedbackId: crypto.randomUUID(),
          text: "  膝盖有点酸 ",
          idempotencyKey: "f",
          recordedAt: new Date().toISOString(),
        },
      };
      return current;
    });
    await userEvent.click(send);
    expect(await screen.findByTestId("trainer-feedback")).toHaveTextContent(
      "膝盖有点酸",
    );
    expect(api.recordFeedback).toHaveBeenCalledTimes(1);
    expect(api.recordFeedback.mock.calls[0]![0]).toBe(SESSION_ID);
    expect(api.recordFeedback.mock.calls[0]![1]).toBe("  膝盖有点酸 ");
    expect(
      screen.queryByRole("button", { name: "说说感受" }),
    ).not.toBeInTheDocument();
    expect(api.startSession).not.toHaveBeenCalled();
  });

  it("reopens the given live session from the Home chip without starting one", async () => {
    api.listSessions.mockResolvedValue([
      session({
        state: "paused",
        revision: 2,
        actions: [performed("cat-cow", 1, 1)],
      }),
    ]);
    renderPanel(vi.fn(), SESSION_ID);
    expect(await screen.findByTestId("trainer-recovery")).toBeVisible();
    expect(api.startSession).not.toHaveBeenCalled();
  });
});
