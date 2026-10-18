import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  EMPTY_DECISION_SESSION,
  projectNativeDiagnosisWindow,
  withCommandError,
  withDecisionReceipt,
} from "../model/diagnosisDecisionViewModel";
import {
  PROPOSAL_A,
  PROPOSAL_B,
  RUN_ID,
  makeCondition,
  makeDecisionReceipt,
  makeDiagnosis,
  makeRun,
} from "../model/__fixtures__/nativeDiagnosisFixtures";
import { ApiRequestError } from "@/lib/api-client";
import {
  DiagnosisWindow,
  type DiagnosisDecisionControls,
} from "./DiagnosisWindow";

function controls(
  overrides: Partial<DiagnosisDecisionControls> = {},
): DiagnosisDecisionControls {
  return {
    selection: {},
    onSelect: vi.fn(),
    onReview: vi.fn(),
    onRecordSelection: vi.fn(),
    onSettle: vi.fn(),
    ...overrides,
  };
}

function proposal(id: string) {
  return document.querySelector<HTMLElement>(`[data-proposal-id="${id}"]`)!;
}

describe("DiagnosisWindow decisions", () => {
  it("keeps AI proposals apart from adopted conditions", () => {
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis({ conditionEntries: [makeCondition()] }),
          run: makeRun(),
        })}
        controls={controls()}
      />,
    );
    expect(screen.getByRole("heading", { name: "肩峰下撞击" })).toBeVisible();
    const panel = screen.getByTestId("diagnosis-proposals");
    expect(within(panel).getByText(/由 AI 分析生成，只是提案/)).toBeVisible();
    expect(within(panel).getByText("上交叉综合征")).toBeVisible();
    expect(screen.queryByRole("heading", { name: "上交叉综合征" })).toBeNull();
  });

  it("supports a partial selection and saves only chosen proposals", async () => {
    const onSelect = vi.fn();
    const onRecordSelection = vi.fn();
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis(),
      run: makeRun(),
    });
    const { rerender } = render(
      <DiagnosisWindow view={view} controls={controls({ onSelect })} />,
    );
    expect(
      screen.getByRole("button", { name: "保存所选的 0 条" }),
    ).toBeDisabled();
    await userEvent.click(
      within(proposal(PROPOSAL_A)).getByRole("radio", { name: "采纳" }),
    );
    expect(onSelect).toHaveBeenCalledWith(PROPOSAL_A, "accept");

    rerender(
      <DiagnosisWindow
        view={view}
        controls={controls({
          selection: { [PROPOSAL_A]: "accept" },
          onRecordSelection,
        })}
      />,
    );
    expect(
      within(proposal(PROPOSAL_B)).getByRole("radio", { name: "暂不决定" }),
    ).toBeChecked();
    await userEvent.click(
      screen.getByRole("button", { name: "保存所选的 1 条" }),
    );
    expect(onRecordSelection).toHaveBeenCalledTimes(1);
    // Settlement waits until the chosen proposals are saved.
    expect(
      screen.getByRole("button", { name: "结束本次分析，不写入诊断" }),
    ).toBeDisabled();
  });

  it("offers to close the rest when only some proposals were accepted", async () => {
    const onSettle = vi.fn();
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis(),
          run: makeRun(),
          session: withDecisionReceipt(
            EMPTY_DECISION_SESSION,
            makeDecisionReceipt(),
          ),
        })}
        controls={controls({ onSettle })}
      />,
    );
    expect(
      within(proposal(PROPOSAL_A)).getByText("已选择采纳，尚未写入诊断"),
    ).toBeVisible();
    expect(screen.queryByText("已写入诊断")).toBeNull();
    await userEvent.click(
      screen.getByRole("button", { name: "写入已采纳的 1 条，其余不采纳" }),
    );
    expect(onSettle).toHaveBeenCalledTimes(1);
  });

  it("asks the user to re-review a proposal after a stale 409", async () => {
    const onReview = vi.fn();
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis(),
          run: makeRun(),
          session: withCommandError(
            EMPTY_DECISION_SESSION,
            RUN_ID,
            new ApiRequestError("changed", 409, "REVISION_CONFLICT"),
            PROPOSAL_A,
          ),
        })}
        controls={controls({ onReview, error: "提案或依据已经变化" })}
      />,
    );
    const item = proposal(PROPOSAL_A);
    expect(within(item).getByText("内容已变化，需要重新核对")).toBeVisible();
    expect(within(item).queryByRole("radio")).toBeNull();
    expect(screen.getByRole("alert")).toHaveTextContent("提案或依据已经变化");
    await userEvent.click(
      within(item).getByRole("button", { name: "我已重新核对" }),
    );
    expect(onReview).toHaveBeenCalledWith(PROPOSAL_A);
  });
});
