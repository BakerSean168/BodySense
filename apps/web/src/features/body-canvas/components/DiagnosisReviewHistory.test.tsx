import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { projectNativeDiagnosisWindow } from "../model/diagnosisDecisionViewModel";
import {
  makeCondition,
  makeDiagnosis,
  makeRun,
} from "../model/__fixtures__/nativeDiagnosisFixtures";
import { DiagnosisWindow } from "../windows/diagnosis/DiagnosisWindow";
import { type DiagnosisDecisionControls } from "../windows/diagnosis/types";
import { DiagnosisHistoryList } from "./DiagnosisHistoryPanel";
import { SafetyRestrictedPanel } from "./SafetyRestrictedView";
import { DiagnosisWindowHeader } from "../windows/diagnosis/DiagnosisWindowHeader";

const api = vi.hoisted(() => ({
  bindRequest: vi.fn(),
  start: vi.fn(),
}));
vi.mock(
  "@/features/consultation/services/nativeDiagnosisService",
  async (importOriginal) => {
    const actual = await importOriginal<object>();
    return {
      ...actual,
      nativeDiagnosisService: {
        ...(actual as { nativeDiagnosisService: object })
          .nativeDiagnosisService,
        ...api,
      },
    };
  },
);
import DiagnosisRequestEntry from "./DiagnosisRequestEntry";

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

const reviewed = makeCondition({
  freshness: {
    state: "review_recommended",
    basedOnBodyStateRevision: 1,
    evaluatedBodyStateRevision: 3,
    evaluatedDiagnosisRevision: 2,
    reasons: [
      {
        code: "related_fact_recorded",
        bodyStateRevision: 2,
        evidenceId: "00000000-0000-4000-8000-0000000000f1",
      },
      {
        code: "related_fact_recorded",
        bodyStateRevision: 3,
        evidenceId: "00000000-0000-4000-8000-0000000000f2",
      },
    ],
  },
});

describe("Diagnosis / Current review prompt", () => {
  it("offers re-analysis and 暂不处理 without touching the condition", async () => {
    const onDismissReview = vi.fn();
    const onRequestAnalysis = vi.fn();
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis({ conditionEntries: [reviewed] }),
          run: null,
        })}
        controls={controls({ onDismissReview, onRequestAnalysis })}
      />,
    );
    const banner = screen.getByTestId("diagnosis-review");
    expect(banner).toHaveTextContent("2 条新记录可能相关");
    expect(screen.getByText("已采纳")).toBeVisible();
    await userEvent.click(
      within(banner).getByRole("button", { name: "暂不处理" }),
    );
    expect(onDismissReview).toHaveBeenCalledWith(reviewed.conditionId);
    await userEvent.click(
      within(banner).getByRole("button", { name: "在对话里请求复核分析" }),
    );
    expect(onRequestAnalysis).toHaveBeenCalledTimes(1);
  });

  it("a stale condition cannot be put aside", () => {
    const stale = makeCondition({
      freshness: {
        ...reviewed.freshness,
        state: "stale",
        reasons: [{ code: "cited_evidence_corrected", bodyStateRevision: 2 }],
      },
    });
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis({ conditionEntries: [stale] }),
          run: null,
        })}
        controls={controls({
          onDismissReview: vi.fn(),
          onRequestAnalysis: vi.fn(),
        })}
      />,
    );
    const banner = screen.getByTestId("diagnosis-review");
    expect(banner).toHaveTextContent("依据有变化，需要重新分析");
    expect(banner).toHaveTextContent("引用的记录已被更正");
    expect(
      within(banner).queryByRole("button", { name: "暂不处理" }),
    ).toBeNull();
    expect(screen.getByTestId("diagnosis-stale-note")).toBeVisible();
  });

  it("Safety stopped opens the restriction view", async () => {
    const onOpenSafety = vi.fn();
    render(
      <DiagnosisWindow
        view={projectNativeDiagnosisWindow({
          diagnosis: makeDiagnosis(),
          run: makeRun({ outcome: "safety_blocked" }),
        })}
        controls={controls({ onOpenSafety })}
      />,
    );
    expect(screen.getByTestId("diagnosis-safety")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "查看限制" }));
    expect(onOpenSafety).toHaveBeenCalledTimes(1);
  });
});

describe("Diagnosis / History", () => {
  it("renders the decision timeline and opens an inactive entry read-only", async () => {
    const inactive = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({
        conditionEntries: [
          makeCondition({ disposition: "inactive", medicalName: "紧张型头痛" }),
        ],
      }),
      run: null,
    }).inactive;
    render(
      <DiagnosisHistoryList
        entries={[
          {
            key: "a",
            at: "2026-10-13T09:00:00+08:00",
            dateLabel: "10月13日",
            title: "偏头痛",
            outcome: "adopted",
            outcomeLabel: "采纳",
            levelLabel: "AI 疑似",
            note: null,
          },
          {
            key: "b",
            at: "2026-08-15T09:00:00+08:00",
            dateLabel: "8月15日",
            title: "颈椎病",
            outcome: "rejected",
            outcomeLabel: "未采纳",
            levelLabel: null,
            note: "提案已拒绝",
          },
        ]}
        inactive={inactive}
        renderCondition={(condition) => (
          <p data-testid="shown">{condition.name}</p>
        )}
      />,
    );
    const rows = screen
      .getAllByRole("listitem")
      .filter((row) => row.classList.contains("bc-dx-timeline-row"));
    expect(rows.map((row) => row.getAttribute("data-outcome"))).toEqual([
      "adopted",
      "rejected",
    ]);
    expect(rows[1]).toHaveTextContent("提案已拒绝");
    await userEvent.click(
      screen.getByRole("button", { name: "查看已停用的“紧张型头痛”" }),
    );
    expect(screen.getByTestId("shown")).toHaveTextContent("紧张型头痛");
    await userEvent.click(screen.getByRole("button", { name: "返回历史" }));
    expect(screen.getByTestId("diagnosis-history")).toBeVisible();
  });

  it("the header picker switches between a condition and 历史", async () => {
    const onOpenHistory = vi.fn();
    const onCloseHistory = vi.fn();
    const onSelectCondition = vi.fn();
    const view = projectNativeDiagnosisWindow({
      diagnosis: makeDiagnosis({ conditionEntries: [reviewed] }),
      run: null,
    });
    const { rerender } = render(
      <DiagnosisWindowHeader
        view={view}
        selectedConditionId={null}
        onSelectCondition={onSelectCondition}
        onOpenHistory={onOpenHistory}
        onCloseHistory={onCloseHistory}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "选择已确认病情" }),
      "历史",
    );
    expect(onOpenHistory).toHaveBeenCalledTimes(1);
    rerender(
      <DiagnosisWindowHeader
        view={view}
        selectedConditionId={null}
        history
        onSelectCondition={onSelectCondition}
        onOpenHistory={onOpenHistory}
        onCloseHistory={onCloseHistory}
      />,
    );
    const picker = screen.getByRole("combobox", {
      name: "选择已确认病情",
    }) as HTMLSelectElement;
    expect(picker.selectedOptions[0].textContent).toBe("历史");
    await userEvent.selectOptions(picker, reviewed.medicalName);
    expect(onSelectCondition).toHaveBeenCalledWith(reviewed.conditionId);
    expect(onCloseHistory).toHaveBeenCalledTimes(1);
  });
});

describe("Safety / Restricted", () => {
  it("shows the Go reason and lift condition with 返回计划 / 补充复核", async () => {
    const onBack = vi.fn();
    const onAddReview = vi.fn();
    render(
      <SafetyRestrictedPanel
        restrictions={[
          {
            id: "r1",
            urgent: false,
            statusLabel: "暂不能执行",
            scopeLabel: "全部训练",
            reason: "安全筛查确认“麻木”",
            meta: "麻木 · 10月13日 · 需要复核",
            liftConditions: [],
            liftPolicyLabel: "需要医生或治疗师确认可以恢复训练",
            scopeNote: "所有训练暂停，直到解除条件满足",
          },
        ]}
        onBack={onBack}
        onAddReview={onAddReview}
      />,
    );
    expect(
      screen.getByRole("region", { name: "安全限制内容" }),
    ).toHaveTextContent("需要医生或治疗师确认可以恢复训练");
    await userEvent.click(screen.getByRole("button", { name: "返回计划" }));
    await userEvent.click(screen.getByRole("button", { name: "补充复核" }));
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onAddReview).toHaveBeenCalledTimes(1);
  });
});

describe("Chat / Diagnosis request", () => {
  beforeEach(() => {
    api.bindRequest.mockReset();
    api.start.mockReset();
  });

  function renderEntry(
    message: { id: string; text: string } | null,
    onOpenDiagnosis = vi.fn(),
  ) {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <DiagnosisRequestEntry
          message={message}
          expectedRevision={2}
          onClose={vi.fn()}
          onOpenDiagnosis={onOpenDiagnosis}
          onOpenSafety={vi.fn()}
        />
      </QueryClientProvider>,
    );
    return onOpenDiagnosis;
  }

  it("binds the user's own message and starts a run only on click", async () => {
    const message = {
      id: "00000000-0000-4000-8000-00000000aaaa",
      text: "帮我正式分析一下头痛。",
    };
    api.bindRequest.mockResolvedValue({
      requestEvidenceId: "00000000-0000-4000-8000-00000000bbbb",
      messageId: message.id,
      excerpt: message.text,
    });
    api.start.mockResolvedValue(
      makeRun({ requestEvidenceId: "00000000-0000-4000-8000-00000000bbbb" }),
    );
    const onOpenDiagnosis = renderEntry(message);
    expect(screen.getByTestId("diagnosis-request-quote")).toHaveTextContent(
      message.text,
    );
    expect(api.bindRequest).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "开始正式分析" }));
    await waitFor(() =>
      expect(screen.getByTestId("diagnosis-request")).toHaveAttribute(
        "data-state",
        "started",
      ),
    );
    expect(api.bindRequest).toHaveBeenCalledWith(message.id);
    expect(api.start).toHaveBeenCalledWith(
      expect.objectContaining({
        requestEvidenceId: "00000000-0000-4000-8000-00000000bbbb",
        trigger: "new_analysis",
        expectedRevision: 2,
      }),
    );
    expect(screen.getAllByText("AI 提案 · 待你决定").length).toBeGreaterThan(0);
    await userEvent.click(screen.getByRole("button", { name: /去决定/ }));
    expect(onOpenDiagnosis).toHaveBeenCalledTimes(1);
  });

  it("asks the user to write the request first when there is no message", () => {
    renderEntry(null);
    expect(screen.getByTestId("diagnosis-request-empty")).toBeVisible();
    expect(screen.queryByRole("button", { name: "开始正式分析" })).toBeNull();
  });
});
