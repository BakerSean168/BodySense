import { describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { makePlan } from "@/features/body-canvas/model/__fixtures__/trainingPlanFixtures";
import {
  RUN_ID,
  makeCondition,
  makeDiagnosis,
  makeRun,
} from "@/features/body-canvas/model/__fixtures__/nativeDiagnosisFixtures";
import { openOptions, selectOption } from "@/test/selectOption";
import {
  mockDiagnosisRun,
  mockLatestDiagnosisRun,
  mockListTrainingPlans,
  mockNativeDiagnosis,
  mockReadBodyRecords,
  mockUseHealthWorkspaceQuery,
  makeWorkspace,
  openDock,
  renderPage,
} from "./businessWindowsSetup";

describe("body-first business windows", () => {
  it("starts body-first with no business window open", () => {
    renderPage();
    expect(screen.getByTestId("body-canvas-workspace")).toBeInTheDocument();
    expect(screen.queryByTestId("diagnosis-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
    expect(screen.queryByTestId("person-window")).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "我的身体" }),
    ).toBeInTheDocument();
  });

  it("opens the Chat window from the Dock and closes it again", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("opens the V2 Diagnosis selector toolbar with a single Close control", async () => {
    renderPage();
    await openDock("综合诊断");
    expect(await screen.findByTestId("diagnosis-window")).toBeVisible();
    // Penpot Diagnosis / Empty: title only until a condition is adopted.
    expect(
      screen.queryByRole("combobox", { name: "选择已确认病情" }),
    ).toBeNull();
    expect(await screen.findByTestId("diagnosis-empty")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "关闭综合诊断" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "综合诊断", level: 2 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/修订|版本号|revision/i)).not.toBeInTheDocument();
  });

  it("never shows an unconfirmed proposal as a confirmed condition (DX-06)", async () => {
    mockLatestDiagnosisRun.mockResolvedValue({
      runId: RUN_ID,
      trigger: "new_analysis",
      basedOnBodyStateRevision: 5,
      basedOnDiagnosisRevision: 2,
      startedAt: "2026-10-18T03:00:00Z",
      state: "open",
    });
    mockDiagnosisRun.mockResolvedValue(makeRun());
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(await screen.findByTestId("diagnosis-proposals")).toBeVisible();
    // Penpot Confirm first: one proposal at a time, never a condition heading.
    expect(screen.getAllByText("待你决定")).toHaveLength(1);
    expect(screen.queryByRole("heading", { name: "上交叉综合征" })).toBeNull();
    expect(screen.getByTestId("diagnosis-mode")).toHaveTextContent("正式分析");
    expect(
      screen.queryByRole("combobox", { name: "选择已确认病情" }),
    ).toBeNull();
  });

  it("lists an adopted condition with cause, handling boundary and certainty", async () => {
    mockNativeDiagnosis.mockResolvedValue(
      makeDiagnosis({ conditionEntries: [makeCondition()] }),
    );
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    expect(
      await screen.findByRole("button", { name: "综合诊断，1 项已确认病情" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toHaveAttribute("data-value", makeCondition().conditionId);
    expect(screen.getByRole("heading", { name: "肩峰下撞击" })).toBeVisible();
    expect(screen.getByText("肩胛控制不足")).toBeInTheDocument();
    expect(screen.getByText("你已确认的分析结果")).toBeInTheDocument();
    expect(screen.getByText("是否做过影像检查")).toBeInTheDocument();
    expect(screen.getByText(/还没有已接受的处理方案/)).toBeInTheDocument();
  });

  it("shows a real empty state instead of a demo plan when the user has none", async () => {
    renderPage();
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await waitFor(() =>
      expect(screen.getByText("还没有训练计划")).toBeInTheDocument(),
    );
  });

  it("renders the user's V2 plan records with real stages and every plan in the picker", async () => {
    mockListTrainingPlans.mockResolvedValue([
      makePlan({ planId: "plan-1", title: "肩胛稳定训练", status: "paused" }),
      makePlan({ planId: "plan-2", title: "臀腿力量训练" }),
    ]);
    renderPage();
    await openDock("训练计划");
    const trainingWindow = await screen.findByTestId("training-window");

    // The active plan wins over the first one; content is the real revision.
    await waitFor(() =>
      expect(
        within(trainingWindow).getByText("弹力带侧走"),
      ).toBeInTheDocument(),
    );
    expect(within(trainingWindow).getByText("热身")).toBeInTheDocument();
    expect(within(trainingWindow).getByText("主训练")).toBeInTheDocument();
    expect(
      within(trainingWindow).getAllByText("3 组 × 12 次").length,
    ).toBeGreaterThan(0);
    const selector = screen.getByRole("combobox", { name: "选择训练计划" });
    expect(selector).toHaveAttribute("data-value", "plan-2");
    expect(
      (await openOptions(selector)).map((option) => option.textContent),
    ).toContain("肩胛稳定训练");
    await userEvent.keyboard("{Escape}");
    await selectOption(selector, "plan-1");
    await waitFor(() =>
      expect(localStorage.getItem("bodysense.training.selectedPlanId")).toBe(
        "plan-1",
      ),
    );
    expect(
      within(trainingWindow).getByTestId("training-gate"),
    ).toHaveTextContent("计划已暂停");
    expect(
      within(trainingWindow).getByRole("button", { name: "开始训练" }),
    ).toBeDisabled();
  });

  it("preserves one V2 Training window when Dock closes and reopens it", async () => {
    renderPage();
    await openDock("训练计划");
    const panel = await screen.findByTestId("training-window");
    expect(panel).toBeVisible();
    for (let i = 0; i < 3; i++) {
      await openDock("训练计划");
      expect(panel).not.toBeVisible();
      await openDock("训练计划");
      expect(panel).toBeVisible();
      expect(screen.getAllByTestId("training-window")).toHaveLength(1);
      expect(screen.getByTestId("training-window")).toBe(panel);
    }
    expect(document.querySelectorAll(".bc-plan-selector")).toHaveLength(0);
  });

  it("switches Training to Diagnosis without destroying the hidden Training window", async () => {
    renderPage();
    await openDock("训练计划");
    const training = await screen.findByTestId("training-window");
    await openDock("综合诊断");
    expect(await screen.findByTestId("diagnosis-window")).toBeVisible();
    expect(training).not.toBeVisible();
    await openDock("训练计划");
    expect(training).toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    expect(screen.getByTestId("training-window")).toBe(training);
  });

  it("keeps the chat mounted when its window closes so in-flight state survives", async () => {
    renderPage();
    await openDock("对话");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("对话");
    // Still mounted, just hidden: closing a window must not discard the task.
    expect(screen.getByTestId("assistant-chat-panel")).not.toBeVisible();
  });

  it("restores a hidden Diagnosis window using its Dock entry without recreating the task", async () => {
    renderPage();
    await openDock("综合诊断");
    const diagnosis = await screen.findByTestId("diagnosis-window");
    await openDock("对话");
    expect(diagnosis).not.toBeVisible();
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    await openDock("综合诊断");
    expect(diagnosis).toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).toBe(diagnosis);
    expect(screen.getAllByTestId("diagnosis-window")).toHaveLength(1);
  });

  it("keeps at most one V2 business window visible across repeated Dock switches", async () => {
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    await openDock("训练计划");
    await screen.findByTestId("training-window");
    await openDock("对话");
    expect(screen.getByTestId("training-window")).not.toBeVisible();
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeVisible();
    expect(screen.getByRole("button", { name: "对话" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "训练计划" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(
      screen.queryByRole("button", { name: /恢复综合诊断|恢复训练计划/ }),
    ).not.toBeInTheDocument();
  });

  it("closes the topmost window on Escape instead of clearing the body selection", async () => {
    renderPage();
    await openDock("综合诊断");
    await screen.findByTestId("diagnosis-window");
    await userEvent.keyboard("{Escape}");
    expect(screen.getByTestId("diagnosis-window")).not.toBeVisible();
    // Escape retreats one layer; the body stage and its selection survive.
    expect(screen.getByRole("heading", { name: "我的身体" })).toBeVisible();
  });

  it("opens a deep-linked window from the route", async () => {
    renderPage("/consultation/conv-1?view=training");
    expect(await screen.findByTestId("training-window")).toBeInTheDocument();
  });

  it("never mounts a Trainer execution from a deep link without explicit Start", async () => {
    renderPage("/consultation/conv-1?view=trainer");
    expect(await screen.findByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /查看训练计划/ }));
    expect(await screen.findByTestId("training-window")).toBeVisible();
    expect(screen.getByTestId("trainer-window")).not.toBeVisible();
  });

  it("maps Person and Trainer shortcut keys without treating them as a data-write command", async () => {
    renderPage();
    await userEvent.keyboard("m");
    expect(await screen.findByTestId("person-window")).toBeVisible();
    await userEvent.keyboard("{Escape}");
    await userEvent.keyboard("t");
    expect(await screen.findByTestId("trainer-entry-guard")).toBeVisible();
    expect(
      screen.queryByTestId("training-execution-panel"),
    ).not.toBeInTheDocument();
  });

  it("resolves legacy ?view= aliases to the same business object", async () => {
    // `state` was a pre-window panel name; existing links must not silently
    // fall back to the bare body stage.
    renderPage("/consultation/conv-1?view=state");
    expect(await screen.findByTestId("person-window")).toBeInTheDocument();
    expect(screen.queryByTestId("training-window")).not.toBeInTheDocument();
  });

  it("surfaces the Chat window when a question is asked from a canvas surface", async () => {
    const user = userEvent.setup();
    renderPage();
    // Ask from the region card: the canvas sets spatial context and must also
    // open Chat, otherwise the question has nowhere to go.
    // Penpot Home has no "开始一次提问" button: "/" asks from the canvas.
    expect(screen.queryByRole("button", { name: "开始一次提问" })).toBeNull();
    await user.keyboard("/");
    expect(await screen.findByTestId("assistant-chat-panel")).toBeVisible();
    expect(screen.getByTestId("assistant-chat-panel")).toBeInTheDocument();
  });

  it("rehomes theme and logout into the Person header account menu", async () => {
    const user = userEvent.setup();
    renderPage();
    // No app bar: no top-level user menu or theme toggle on Home.
    expect(screen.queryByRole("button", { name: "打开用户菜单" })).toBeNull();
    expect(screen.queryByRole("button", { name: "切换深色" })).toBeNull();
    await openDock("个人档案");
    await screen.findByTestId("person-window");
    await user.click(screen.getByRole("button", { name: "synthetic" }));
    await user.click(await screen.findByRole("menuitem", { name: "设置" }));
    const settings = await screen.findByRole("dialog", { name: "设置" });
    const light = within(settings).getByRole("radio", { name: "浅色" });
    expect(light).toHaveAttribute("aria-checked", "true");
    await user.click(within(settings).getByRole("radio", { name: "深色" }));
    expect(screen.getByTestId("body-canvas-workspace")).toHaveClass("bc-dark");
    // Arrow keys move the radio selection (roving tabindex).
    within(settings).getByRole("radio", { name: "深色" }).focus();
    await user.keyboard("{ArrowRight}");
    expect(
      within(settings).getByRole("radio", { name: "跟随系统" }),
    ).toHaveAttribute("aria-checked", "true");
    await user.click(within(settings).getByRole("radio", { name: "浅色" }));
    expect(screen.getByTestId("body-canvas-workspace")).not.toHaveClass(
      "bc-dark",
    );
    await user.click(within(settings).getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "设置" })).toBeNull();

    await user.click(screen.getByRole("button", { name: "synthetic" }));
    await user.click(await screen.findByRole("menuitem", { name: "退出登录" }));
    const confirm = await screen.findByRole("dialog", { name: "退出登录" });
    // Title and the two buttons say it all: no extra prompt sentence.
    expect(confirm).not.toHaveTextContent("确定");
    await user.click(within(confirm).getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("dialog", { name: "退出登录" })).toBeNull();
  });
});

it("retries failed native body reads from the canvas sync banner", async () => {
  mockReadBodyRecords.mockRejectedValueOnce(new Error("unavailable"));
  mockUseHealthWorkspaceQuery.mockReturnValue({
    data: makeWorkspace(),
    isFetching: false,
    isError: false,
    refetch: vi.fn(),
  });
  renderPage();
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("身体信息暂时无法同步");
  await userEvent.click(within(alert).getByRole("button", { name: "重试" }));
  await waitFor(() => expect(mockReadBodyRecords).toHaveBeenCalledTimes(2));
  await waitFor(() =>
    expect(screen.queryByRole("alert")).not.toBeInTheDocument(),
  );
});
