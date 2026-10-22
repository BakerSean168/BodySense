import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  EMPTY_DECISION_SESSION,
  projectNativeDiagnosisWindow,
  withCommandError,
  withDecisionReceipt,
  withSettlementReceipt,
} from "../model/diagnosisDecisionViewModel";
import {
  FINGERPRINT_B,
  PROPOSAL_A,
  PROPOSAL_B,
  RUN_ID,
  makeCondition,
  makeDecisionReceipt,
  makeDiagnosis,
  makeProposal,
  makeRun,
  makeSettlementReceipt,
} from "../model/__fixtures__/nativeDiagnosisFixtures";
import { ApiRequestError } from "@/lib/api-client";
import { DiagnosisWindow } from "../windows/diagnosis/DiagnosisWindow";
import { type DiagnosisDecisionControls } from "../windows/diagnosis/types";

const PROPOSAL_C = "00000000-0000-4000-8000-0000000000b3";

function controls(
  overrides: Partial<DiagnosisDecisionControls> = {},
): DiagnosisDecisionControls {
  return {
    selection: {},
    onSelect: vi.fn(),
    onReview: vi.fn(),
    onSubmit: vi.fn(),
    onDismissOutcome: vi.fn(),
    ...overrides,
  };
}

function view(
  input: Partial<Parameters<typeof projectNativeDiagnosisWindow>[0]> = {},
) {
  return projectNativeDiagnosisWindow({
    diagnosis: makeDiagnosis(),
    run: makeRun(),
    ...input,
  });
}

const footer = () => screen.getByTestId("diagnosis-decide");

describe("DiagnosisWindow (Penpot Diagnosis boards)", () => {
  it("shows one AI proposal at a time and never as a condition", () => {
    render(
      <DiagnosisWindow
        view={view({
          diagnosis: makeDiagnosis({ conditionEntries: [makeCondition()] }),
        })}
        controls={controls()}
      />,
    );
    const panel = screen.getByTestId("diagnosis-proposals");
    expect(within(panel).getByTestId("diagnosis-step")).toHaveTextContent(
      "1 / 2AI 建议尚未写入",
    );
    expect(within(panel).getByText("上交叉综合征")).toBeVisible();
    // Kind chip on the current card and on the "下一项" line.
    expect(within(panel).getAllByText("新建")).toHaveLength(2);
    expect(within(panel).getByText("AI 疑似")).toBeVisible();
    expect(within(panel).getByText("下一项")).toBeVisible();
    expect(within(panel).getByText("胸椎活动受限")).toBeVisible();
    expect(screen.queryByRole("heading", { name: "上交叉综合征" })).toBeNull();
    // Current conditions stay behind the formal analysis until it is settled.
    expect(screen.queryByRole("heading", { name: "肩峰下撞击" })).toBeNull();
  });

  it("Confirm first: choosing moves to the next proposal locally", async () => {
    const onSelect = vi.fn();
    const onSubmit = vi.fn();
    render(
      <DiagnosisWindow
        view={view()}
        controls={controls({ onSelect, onSubmit })}
      />,
    );
    await userEvent.click(
      within(footer()).getByRole("button", { name: "采纳" }),
    );
    expect(onSelect).toHaveBeenCalledWith(PROPOSAL_A, "accept");
    await userEvent.click(
      within(footer()).getByRole("button", { name: "不采纳" }),
    );
    expect(onSelect).toHaveBeenCalledWith(PROPOSAL_A, "reject");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Confirm second: the last proposal accepts and commits in one step", async () => {
    const onSelect = vi.fn();
    const onSubmit = vi.fn();
    render(
      <DiagnosisWindow
        view={view()}
        controls={controls({
          selection: { [PROPOSAL_A]: "accept" },
          onSelect,
          onSubmit,
        })}
      />,
    );
    expect(screen.getByTestId("diagnosis-step")).toHaveTextContent("2 / 2");
    const decided = document.querySelector<HTMLElement>(
      `.bc-dx-decided[data-proposal-id="${PROPOSAL_A}"]`,
    )!;
    expect(within(decided).getByText("已采纳")).toBeVisible();
    await userEvent.click(
      within(decided).getByRole("button", {
        name: "更改对“上交叉综合征”的决定",
      }),
    );
    expect(onSelect).toHaveBeenCalledWith(PROPOSAL_A, null);

    expect(
      within(footer()).getByRole("button", { name: "不采纳" }),
    ).toBeEnabled();
    await userEvent.click(
      within(footer()).getByRole("button", { name: "采纳并提交" }),
    );
    expect(onSubmit).toHaveBeenCalledWith({
      proposalId: PROPOSAL_B,
      choice: "accept",
    });
  });

  it("First rejected: nothing adopted makes the last reject 不更改", async () => {
    const onSubmit = vi.fn();
    render(
      <DiagnosisWindow
        view={view()}
        controls={controls({ selection: { [PROPOSAL_A]: "reject" }, onSubmit })}
      />,
    );
    const decided = document.querySelector<HTMLElement>(
      `.bc-dx-decided[data-proposal-id="${PROPOSAL_A}"]`,
    )!;
    expect(within(decided).getByText("未采纳")).toBeVisible();
    expect(within(decided).getByText("不等于排除")).toBeVisible();
    await userEvent.click(
      within(footer()).getByRole("button", { name: "不更改" }),
    );
    expect(onSubmit).toHaveBeenCalledWith({
      proposalId: PROPOSAL_B,
      choice: "reject",
    });
  });

  it("keeps partial decisions: submit early closes the remaining proposals", async () => {
    const onSubmit = vi.fn();
    const run = makeRun({
      proposals: [
        makeProposal(),
        makeProposal({
          proposalId: PROPOSAL_B,
          contentFingerprint: FINGERPRINT_B,
          content: { proposedEntry: { medicalName: "胸椎活动受限" } },
        }),
        makeProposal({
          proposalId: PROPOSAL_C,
          contentFingerprint: "c".repeat(64),
          content: { proposedEntry: { medicalName: "肩胛运动障碍" } },
        }),
      ],
    });
    render(
      <DiagnosisWindow
        view={view({
          run,
          session: withDecisionReceipt(
            EMPTY_DECISION_SESSION,
            makeDecisionReceipt(),
          ),
        })}
        controls={controls({ onSubmit })}
      />,
    );
    // A recorded decision receipt is a choice only: decided, not changeable.
    const decided = document.querySelector<HTMLElement>(
      `.bc-dx-decided[data-proposal-id="${PROPOSAL_A}"]`,
    )!;
    expect(decided).toHaveAttribute("data-proposal-state", "pending");
    expect(within(decided).queryByRole("button")).toBeNull();
    expect(screen.queryByTestId("diagnosis-receipt")).toBeNull();
    expect(screen.getByTestId("diagnosis-step")).toHaveTextContent("2 / 3");
    await userEvent.click(
      within(footer()).getByRole("button", { name: "其余暂不决定，先提交" }),
    );
    expect(onSubmit).toHaveBeenCalledWith();
  });

  it("asks the user to re-review a proposal after a stale 409", async () => {
    const onReview = vi.fn();
    render(
      <DiagnosisWindow
        view={view({
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
    expect(
      document.querySelector(".bc-dx-banner[data-tone='warn']"),
    ).toHaveTextContent("内容已变化，需要重新核对");
    expect(screen.getByRole("alert")).toHaveTextContent("提案或依据已经变化");
    expect(
      within(footer()).getByRole("button", { name: "采纳" }),
    ).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "我已重新核对" }));
    expect(onReview).toHaveBeenCalledWith(PROPOSAL_A);
  });

  it("shows the Receipt scene only with a committed settlement receipt", async () => {
    const decision = makeDecisionReceipt();
    const pending = withDecisionReceipt(EMPTY_DECISION_SESSION, decision);
    const onDismissOutcome = vi.fn();
    const { rerender } = render(
      <DiagnosisWindow
        view={view({ session: pending })}
        controls={controls()}
      />,
    );
    expect(screen.queryByTestId("diagnosis-receipt")).toBeNull();

    rerender(
      <DiagnosisWindow
        view={view({
          session: withSettlementReceipt(
            pending,
            makeSettlementReceipt({ decisionIds: [decision.decisionId] }),
          ),
        })}
        controls={controls({ onDismissOutcome })}
      />,
    );
    const receipt = screen.getByTestId("diagnosis-receipt");
    expect(receipt).toHaveTextContent("诊断已更新");
    expect(receipt).toHaveTextContent("1 项决定已提交");
    expect(within(receipt).getByText("已采纳")).toBeVisible();
    expect(receipt).toHaveTextContent(/回执 [0-9A-F-]{8}/);
    expect(screen.queryByTestId("diagnosis-decide")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "查看当前" }));
    expect(onDismissOutcome).toHaveBeenCalledTimes(1);
  });

  it("shows No changes after a closed-without-change receipt", () => {
    render(
      <DiagnosisWindow
        view={view({
          session: withSettlementReceipt(
            EMPTY_DECISION_SESSION,
            makeSettlementReceipt({
              outcome: "closed_without_change",
              revision: 2,
              diagnosisRevisionId: undefined,
            }),
          ),
        })}
        controls={controls()}
      />,
    );
    expect(screen.getByTestId("diagnosis-no-change")).toHaveTextContent(
      "诊断未更改你的决定已记录",
    );
    expect(screen.getByRole("button", { name: "返回当前" })).toBeVisible();
    expect(screen.queryByTestId("diagnosis-receipt")).toBeNull();
  });

  it("Safety stopped keeps every decision out of reach", () => {
    render(
      <DiagnosisWindow
        view={view({ safetyReview: true })}
        controls={controls({ selection: { [PROPOSAL_A]: "accept" } })}
      />,
    );
    const safety = screen.getByTestId("diagnosis-safety");
    expect(safety).toHaveAttribute("data-reason", "safety_review");
    expect(safety).toHaveTextContent("分析已暂停");
    expect(screen.queryByRole("button", { name: /采纳/ })).toBeNull();
    expect(screen.queryByTestId("diagnosis-decide")).toBeNull();
  });

  it("a pending safety review keeps Empty/Current visible with a note", () => {
    const { rerender } = render(
      <DiagnosisWindow
        view={view({ run: null, safetyReview: true })}
        controls={controls()}
      />,
    );
    expect(screen.getByTestId("diagnosis-empty")).toHaveTextContent(
      "还没有诊断",
    );
    expect(screen.getByTestId("diagnosis-safety-note")).toHaveTextContent(
      "需要先处理安全提醒",
    );
    expect(screen.queryByTestId("diagnosis-safety")).toBeNull();

    rerender(
      <DiagnosisWindow
        view={view({
          diagnosis: makeDiagnosis({ conditionEntries: [makeCondition()] }),
          run: null,
          safetyReview: true,
        })}
        controls={controls()}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "肩峰下撞击" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("diagnosis-safety-note")).toBeVisible();
  });

  it("Current shows the adopted condition; Empty offers Chat", async () => {
    const { rerender } = render(
      <DiagnosisWindow
        view={view({
          diagnosis: makeDiagnosis({ conditionEntries: [makeCondition()] }),
          run: null,
        })}
        controls={controls()}
      />,
    );
    expect(
      screen.getByRole("heading", { name: "肩峰下撞击" }),
    ).toBeInTheDocument();
    expect(screen.getByText("已采纳")).toBeVisible();
    expect(screen.getByText("是否做过影像检查")).toBeVisible();

    const onOpenChat = vi.fn();
    rerender(
      <DiagnosisWindow
        view={view({ run: null })}
        controls={controls({ onOpenChat })}
      />,
    );
    expect(screen.getByTestId("diagnosis-empty")).toHaveTextContent(
      "还没有诊断",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "在对话里请求正式分析" }),
    );
    expect(onOpenChat).toHaveBeenCalledTimes(1);
  });
});
