import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ProjectTrainingWindowInput } from "../model/trainingPlans";
import { selectTrainingPlan } from "../model/trainingPlanSelection";
import {
  NEXT_REVISION_ID,
  NOW,
  PLAN_ID,
  PROPOSAL_ID,
  makePlan,
  makeProposal,
  updatedContent,
  makeContent,
} from "../model/__fixtures__/trainingPlanFixtures";
import { TrainingPlanWindow } from "../windows/training/TrainingPlanWindow";
import { type TrainingPlanControls } from "../windows/training/types";
import { TrainingWindowHeader } from "../windows/training/TrainingWindowHeader";
import { selectOption } from "@/test/selectOption";

function view(
  overrides: Partial<ProjectTrainingWindowInput> = {},
): ProjectTrainingWindowInput {
  return {
    plans: [makePlan()],
    proposals: [],
    selectedPlanId: null,
    selectedTemplateId: null,
    focus: { kind: "plan" },
    loading: false,
    error: null,
    now: NOW,
    ...overrides,
  };
}

function controls(
  overrides: Partial<TrainingPlanControls> = {},
): TrainingPlanControls {
  return {
    onOpenProposal: vi.fn(),
    onBackToPlan: vi.fn(),
    onAccept: vi.fn(),
    onReject: vi.fn(),
    onDismissNotice: vi.fn(),
    onSelectTemplate: vi.fn(),
    onStart: vi.fn(),
    onOpenChat: vi.fn(),
    ...overrides,
  };
}

describe("Penpot Training boards", () => {
  it("offers a real read retry instead of showing an empty plan after failure", async () => {
    const retry = vi.fn();
    render(
      <TrainingPlanWindow
        state={view({ error: "offline", plans: undefined })}
        controls={controls({ onRetry: retry })}
        canExecute={false}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("暂时无法读取");
    expect(screen.queryByTestId("training-empty")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("keeps a loaded plan visible while disabling Start during partial failure", async () => {
    const retry = vi.fn();
    render(
      <TrainingPlanWindow
        state={view()}
        partialError="提案暂时无法读取"
        controls={controls({ onRetry: retry })}
        canExecute
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("提案暂时无法读取");
    expect(screen.getByText("启用中")).toBeVisible();
    expect(screen.getByRole("button", { name: "开始训练" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "重新读取" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("puts every user plan in the header picker and the title alone when there is none", async () => {
    const select = vi.fn();
    const plans = [
      makePlan(),
      makePlan({ planId: "plan-b", title: "下肢训练", status: "paused" }),
    ];
    const { rerender } = render(
      <TrainingWindowHeader
        view={selectTrainingPlan(plans, null)}
        onSelectPlan={select}
      />,
    );
    expect(screen.getByText("训练")).toBeVisible();
    await selectOption(
      screen.getByRole("combobox", { name: "选择训练计划" }),
      "plan-b",
    );
    expect(select).toHaveBeenCalledWith("plan-b");
    rerender(
      <TrainingWindowHeader
        view={selectTrainingPlan([], null)}
        onSelectPlan={select}
      />,
    );
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("renders the Accepted board read-only with a gated Start", async () => {
    const start = vi.fn();
    const pickTemplate = vi.fn();
    render(
      <TrainingPlanWindow
        state={view()}
        canExecute
        controls={controls({ onStart: start, onSelectTemplate: pickTemplate })}
      />,
    );
    expect(screen.getByText("启用中")).toBeVisible();
    expect(screen.getByText("每周 3 次")).toBeVisible();
    expect(screen.getByRole("region", { name: "热身" })).toHaveTextContent(
      "猫牛式1 组 × 10 次",
    );
    expect(screen.getByRole("region", { name: "主训练" })).toHaveTextContent(
      "弹力带侧走2 组 × 12 次休息 45 秒左右交替",
    );
    expect(screen.getByTestId("training-gate")).toHaveTextContent("可以开始");
    // Read-only: no inputs other than the template picker.
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    await selectOption(
      screen.getByRole("combobox", { name: "选择训练课" }),
      "b",
    );
    expect(pickTemplate).toHaveBeenCalledWith("b");
    await userEvent.click(screen.getByRole("button", { name: "开始训练" }));
    expect(start).toHaveBeenCalledWith(PLAN_ID, "a");
  });

  it("keeps Start disabled with the gate's reason when the TrainerGate blocks", () => {
    const blocked = makePlan({
      executionGate: {
        ...makePlan().executionGate,
        result: "block",
        permitsExecution: false,
        reasons: ["safety_screening_incomplete"],
      },
    });
    render(
      <TrainingPlanWindow
        state={view({ plans: [blocked] })}
        canExecute
        controls={controls()}
      />,
    );
    const start = screen.getByRole("button", { name: "开始训练" });
    expect(start).toBeDisabled();
    expect(start).toHaveAccessibleDescription("暂不能开始：还没有完成安全筛查");
    expect(screen.getByTestId("training-gate")).toHaveTextContent("暂不能开始");
  });

  it("keeps Start disabled while the workspace has no execution capability", () => {
    render(
      <TrainingPlanWindow
        state={view()}
        canExecute={false}
        controls={controls()}
      />,
    );
    expect(screen.getByRole("button", { name: "开始训练" })).toBeDisabled();
  });

  it("shows the Empty board with a chat entry", async () => {
    const chat = vi.fn();
    render(
      <TrainingPlanWindow
        state={view({ plans: [] })}
        canExecute
        controls={controls({ onOpenChat: chat })}
      />,
    );
    expect(screen.getByText("还没有训练计划")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "去对话" }));
    expect(chat).toHaveBeenCalled();
  });

  it("opens a pending proposal from the plan and decides it explicitly", async () => {
    const open = vi.fn();
    const { rerender } = render(
      <TrainingPlanWindow
        state={view({ proposals: [makeProposal()] })}
        canExecute
        controls={controls({ onOpenProposal: open })}
      />,
    );
    expect(screen.getByTestId("training-pending-proposal")).toHaveTextContent(
      "6 天内2 项调整建议待确认",
    );
    await userEvent.click(screen.getByRole("button", { name: "查看调整建议" }));
    expect(open).toHaveBeenCalledWith(PROPOSAL_ID);

    const accept = vi.fn();
    const reject = vi.fn();
    rerender(
      <TrainingPlanWindow
        state={view({
          proposals: [makeProposal()],
          focus: { kind: "proposal", proposalId: PROPOSAL_ID },
        })}
        canExecute
        controls={controls({ onAccept: accept, onReject: reject })}
      />,
    );
    const scene = screen.getByTestId("training-proposal");
    expect(within(scene).getByText("待确认")).toBeVisible();
    expect(within(scene).getByText("6 天后过期")).toBeVisible();
    expect(within(scene).getByText("AI 建议")).toBeVisible();
    expect(within(scene).getByText("3 组 × 12 次 · 休息 45 秒").tagName).toBe(
      "S",
    );
    expect(within(scene).getByText("3 组 × 15 次 · 休息 45 秒")).toBeVisible();
    expect(within(scene).getByText("死虫式")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "接受" }));
    expect(accept).toHaveBeenCalledWith(PROPOSAL_ID);
    await userEvent.click(screen.getByRole("button", { name: "拒绝" }));
    expect(reject).toHaveBeenCalledWith(PROPOSAL_ID);
  });

  it("disables both decisions while one is in flight", () => {
    render(
      <TrainingPlanWindow
        state={view({
          proposals: [makeProposal()],
          focus: { kind: "proposal", proposalId: PROPOSAL_ID },
        })}
        canExecute
        controls={controls({ busy: true })}
      />,
    );
    expect(screen.getByRole("button", { name: "接受" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "拒绝" })).toBeDisabled();
  });

  it("shows the Expired board after a stale refusal, with the plan unchanged", async () => {
    const back = vi.fn();
    render(
      <TrainingPlanWindow
        state={view({
          proposals: [makeProposal()],
          focus: { kind: "proposal", proposalId: PROPOSAL_ID },
          refused: { [PROPOSAL_ID]: "base_revision_changed" },
        })}
        canExecute
        controls={controls({ onBackToPlan: back })}
      />,
    );
    expect(screen.getByTestId("training-proposal-expired")).toBeVisible();
    expect(screen.getByText("已过期")).toBeVisible();
    expect(screen.getByTestId("training-proposal-stale")).toHaveTextContent(
      "计划已有新的修订",
    );
    expect(
      screen.queryByRole("button", { name: "接受" }),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "返回计划" }));
    expect(back).toHaveBeenCalled();
  });

  it("marks what the accepted revision changed once its receipt is visible", () => {
    const receipt = {
      receiptId: "66666666-6666-4666-8666-666666666666",
      proposalId: PROPOSAL_ID,
      decisionId: "77777777-7777-4777-8777-777777777777",
      planId: PLAN_ID,
      revisionId: NEXT_REVISION_ID,
      idempotencyKey: "k",
      decidedAt: "2026-10-18T12:00:00+08:00",
      executionGate: makePlan().executionGate,
    };
    const focus = {
      kind: "updated" as const,
      receipt,
      baseContent: makeContent(),
      acceptedContent: updatedContent(),
      created: false,
    };
    const { rerender } = render(
      <TrainingPlanWindow
        state={view({ focus })}
        canExecute
        controls={controls()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(
      "正在读取新的计划修订",
    );
    expect(
      screen.queryByText("已采纳调整 · 新修订已生效"),
    ).not.toBeInTheDocument();
    rerender(
      <TrainingPlanWindow
        state={view({
          plans: [
            makePlan({
              currentAcceptedRevisionId: NEXT_REVISION_ID,
              currentContent: updatedContent(),
            }),
          ],
          focus,
        })}
        canExecute
        controls={controls()}
      />,
    );
    expect(screen.getByTestId("training-receipt")).toHaveTextContent(
      "已采纳调整 · 新修订已生效",
    );
    expect(
      screen
        .getAllByTestId("training-exercise-change")
        .map((chip) => chip.textContent),
    ).toEqual(["已调整", "新增"]);
  });
});
