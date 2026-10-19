import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ConfirmedCondition } from "../model/confirmedConditions";
import type { NativeDiagnosisWindowView } from "../model/diagnosisDecisionViewModel";
import { ChatWindowHeader, DiagnosisWindowHeader } from "./V2WindowHeaders";
import { DiagnosisWindow } from "./DiagnosisWindow";

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
  inactive: [],
  run: null,
  safetyBlocked: false,
  safetyReview: false,
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
        view={{ ...diagnosis, confirmed: [] }}
        selectedConditionId={null}
        onSelectCondition={onSelectCondition}
      />,
    );
    // Penpot Diagnosis / Empty shows only the window title.
    expect(
      screen.queryByRole("combobox", { name: "选择已确认病情" }),
    ).toBeNull();
    expect(screen.getByText("诊断")).toBeVisible();
  });

  it("shows the Penpot run label instead of the picker during a formal analysis", () => {
    const { rerender } = render(
      <DiagnosisWindowHeader
        view={diagnosis}
        selectedConditionId="condition-1"
        onSelectCondition={vi.fn()}
        mode="analysis"
      />,
    );
    expect(screen.getByTestId("diagnosis-mode")).toHaveTextContent("正式分析");
    expect(screen.queryByRole("combobox")).toBeNull();
    rerender(
      <DiagnosisWindowHeader
        view={diagnosis}
        selectedConditionId="condition-1"
        onSelectCondition={vi.fn()}
        mode="receipt"
      />,
    );
    expect(screen.getByTestId("diagnosis-mode")).toHaveTextContent("提交回执");
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

  it("shows 今天 alone when the conversation has no earlier day", () => {
    render(
      <ChatWindowHeader
        history={{ todayKey: "2026-10-20", days: [] }}
        onSelectDay={vi.fn()}
      />,
    );
    expect(screen.getByText("对话")).toBeVisible();
    expect(screen.getByTestId("chat-day")).toHaveTextContent("今天");
    expect(screen.queryByRole("combobox")).toBeNull();
  });

  it("jumps to a day the user wrote in from the 今天 picker", async () => {
    const user = userEvent.setup();
    const onSelectDay = vi.fn();
    render(
      <ChatWindowHeader
        history={{
          todayKey: "2026-10-20",
          days: [
            {
              day: "2026-10-14",
              label: "10月14日",
              count: 2,
              preview: "昨天有头痛",
              firstMessageId: "a",
            },
          ],
        }}
        selectedDay="2026-10-20"
        onSelectDay={onSelectDay}
      />,
    );
    const picker = screen.getByRole("combobox", { name: "跳到某天的对话" });
    expect(picker).toHaveDisplayValue("今天");
    await user.selectOptions(picker, "10月14日");
    expect(onSelectDay).toHaveBeenCalledWith("2026-10-14");
  });
});
