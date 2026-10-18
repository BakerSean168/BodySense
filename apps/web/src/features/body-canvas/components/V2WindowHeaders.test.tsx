import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  EMPTY_DIAGNOSIS_VIEW,
  type ConfirmedCondition,
} from "../model/confirmedConditions";
import type { NativeDiagnosisWindowView } from "../model/diagnosisDecisionViewModel";
import type { ProjectedTrainingPlans } from "../model/trainingPlans";
import {
  ChatWindowHeader,
  DiagnosisWindowHeader,
  TrainingWindowHeader,
} from "./V2WindowHeaders";
import { DiagnosisWindow } from "./DiagnosisWindow";
import { TrainingPlanWindow } from "./TrainingPlanWindow";

function condition(conditionId: string, name: string): ConfirmedCondition {
  return {
    conditionId,
    name,
    certainty: "user_confirmed_agent_analysis",
    lifecycle: "active",
    cause: {
      summary: "仅限目前的分析资料",
      basis: "用户报告的身体描述",
      basisFactIds: [],
      evidenceStrength: null,
      counterEvidenceCount: 0,
      informationGaps: [],
      differential: null,
    },
    handling: { state: "not_confirmed" },
    safetyNotes: [],
  };
}

const diagnosis: NativeDiagnosisWindowView = {
  currentRevision: 2,
  confirmed: [
    condition("condition-1", "左肩病情"),
    condition("condition-2", "右膝病情"),
  ],
  run: null,
  safetyBlocked: false,
};

const plans: ProjectedTrainingPlans = {
  plans: [
    {
      planId: "plan-a",
      title: "肩胛训练",
      status: "accepted",
      isAccepted: true,
    },
    {
      planId: "plan-b",
      title: "下肢训练",
      status: "accepted",
      isAccepted: true,
    },
  ],
  selected: {
    planId: "plan-a",
    title: "肩胛训练",
    goal: "改善控制能力",
    status: "accepted",
    durationWeeks: null,
    frequency: "每周三次",
    isAccepted: true,
    exerciseCount: 1,
    stageStructureUnavailable: false,
    stages: [
      {
        phase: "main",
        label: "正式训练",
        exercises: [
          {
            exerciseId: "exercise-a",
            name: "墙上滑动",
            description: null,
            sets: "3",
            reps: "10",
            duration: null,
            rest: null,
            notes: null,
            kind: "exercise",
          },
        ],
      },
    ],
  },
  selectedPlanMissing: false,
};

describe("Penpot V2 Diagnosis, Training and Chat headers", () => {
  it("lets the user select an adopted condition in the header without promoting pending proposals", async () => {
    const onSelectCondition = vi.fn();
    const { rerender } = render(
      <DiagnosisWindowHeader
        view={diagnosis}
        selectedConditionId="condition-1"
        onSelectCondition={onSelectCondition}
      />,
    );
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toHaveValue("condition-1");
    expect(screen.queryByText("尚未确认")).not.toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
      "condition-2",
    );
    expect(onSelectCondition).toHaveBeenCalledWith("condition-2");
    rerender(
      <DiagnosisWindowHeader
        view={{ ...EMPTY_DIAGNOSIS_VIEW, confirmed: [] }}
        selectedConditionId={null}
        onSelectCondition={onSelectCondition}
      />,
    );
    expect(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
    ).toBeDisabled();
    expect(screen.getByText("暂无已确认病情")).toBeVisible();
  });

  it("renders only the selected adopted diagnosis and leaves pending proposals unconfirmed", () => {
    const { rerender } = render(
      <DiagnosisWindow view={diagnosis} selectedConditionId="condition-2" />,
    );
    expect(screen.getByText("右膝病情")).toBeVisible();
    expect(screen.queryByText("左肩病情")).not.toBeInTheDocument();
    expect(screen.queryByTestId("diagnosis-proposals")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "确认诊断" }),
    ).not.toBeInTheDocument();
    rerender(
      <DiagnosisWindow view={diagnosis} selectedConditionId="stale-id" />,
    );
    expect(screen.getByText("左肩病情")).toBeVisible();
  });

  it("keeps plan selection in the header while execution is still permission-gated", async () => {
    const selectPlan = vi.fn();
    const start = vi.fn();
    const { container } = render(
      <>
        <TrainingWindowHeader projected={plans} onSelectPlan={selectPlan} />
        <TrainingPlanWindow
          projected={plans}
          executionBlocked
          canExecute
          onStartTraining={start}
        />
      </>,
    );
    expect(screen.getAllByRole("combobox")).toHaveLength(1);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "选择训练计划" }),
      "plan-b",
    );
    expect(selectPlan).toHaveBeenCalledWith("plan-b");
    expect(container.querySelectorAll(".bc-plan-selector")).toHaveLength(0);
    expect(screen.getByRole("button", { name: "开始训练" })).toBeDisabled();
    expect(screen.getByText(/当前安全状态不允许执行训练/)).toBeVisible();
    expect(start).not.toHaveBeenCalled();
  });

  it("does not offer unaccepted draft plans in the header or allow their execution", () => {
    const draft = {
      ...plans,
      selected: {
        ...plans.selected!,
        planId: "unaccepted",
        title: "尚待确认",
        isAccepted: false,
      },
      plans: [
        ...plans.plans,
        {
          planId: "unaccepted",
          title: "尚待确认",
          status: "draft",
          isAccepted: false,
        },
      ],
    };
    const onStartTraining = vi.fn();
    render(
      <>
        <TrainingWindowHeader projected={draft} onSelectPlan={vi.fn()} />
        <TrainingPlanWindow
          projected={draft}
          executionBlocked={false}
          canExecute
          onStartTraining={onStartTraining}
        />
      </>,
    );
    const picker = screen.getByRole("combobox", { name: "选择训练计划" });
    expect(picker).toHaveValue("");
    expect(
      screen.queryByRole("option", { name: "尚待确认" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始训练" })).toBeDisabled();
    expect(screen.getByText(/尚未被接受/)).toBeVisible();
    expect(onStartTraining).not.toHaveBeenCalled();
  });

  it("labels the chat time as a local clock, not the message timestamp", () => {
    render(<ChatWindowHeader now={new Date("2026-10-13T13:30:00.000Z")} />);
    const time = screen.getByLabelText(/当前本地时间/);
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("datetime", "2026-10-13T13:30:00.000Z");
    expect(screen.getByText("对话")).toBeVisible();
  });
});
