import { useState } from "react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { MessageCircle } from "lucide-react";
import { BusinessWindowHost } from "./BusinessWindowHost";
import { DiagnosisWindow } from "../windows/diagnosis/DiagnosisWindow";
import { type DiagnosisDecisionControls } from "../windows/diagnosis/types";
import { DiagnosisHistoryList } from "./DiagnosisHistoryPanel";
import { TrainingPlanWindow } from "../windows/training/TrainingPlanWindow";
import { type TrainingPlanControls } from "../windows/training/types";
import { DiagnosisWindowHeader } from "../windows/diagnosis/DiagnosisWindowHeader";
import { TrainingWindowHeader } from "../windows/training/TrainingWindowHeader";
import {
  diagnosisHeaderMode,
  projectNativeDiagnosisWindow,
  type ProposalChoice,
} from "../model/diagnosisDecisionViewModel";
import {
  makeCondition,
  makeDiagnosis,
  makeRun,
} from "../model/__fixtures__/nativeDiagnosisFixtures";
import {
  makePlan,
  makeProposal,
  NOW,
} from "../model/__fixtures__/trainingPlanFixtures";
import type {
  ProjectTrainingWindowInput,
  TrainingFocus,
} from "../model/trainingPlans";
import { selectTrainingPlan } from "../model/trainingPlanSelection";
import { windowCanvasDecorator } from "./windowStorySupport";

type Scenario =
  | "diagnosis-loading"
  | "diagnosis-empty"
  | "diagnosis-current"
  | "diagnosis-analysis"
  | "diagnosis-history"
  | "diagnosis-safety"
  | "diagnosis-error"
  | "training-loading"
  | "training-empty"
  | "training-plan"
  | "training-proposal"
  | "training-expired"
  | "training-blocked"
  | "training-error";

function ContentPreviewBody({ scenario }: { scenario: Scenario }) {
  const mobile = useMediaQuery("(max-width: 760px)");
  const [closed, setClosed] = useState(false);
  const [selectedConditionId, setSelectedConditionId] = useState<string | null>(
    null,
  );
  const [selection, setSelection] = useState<Record<string, ProposalChoice>>(
    {},
  );
  const [selectedPlanId, setSelectedPlanId] = useState<string | null>(null);
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [focus, setFocus] = useState<TrainingFocus>(
    ["training-proposal", "training-expired"].includes(scenario)
      ? { kind: "proposal", proposalId: makeProposal().proposalId }
      : { kind: "plan" },
  );

  if (closed) {
    return (
      <button
        type="button"
        onClick={() => setClosed(false)}
        style={{
          position: "absolute",
          top: 110,
          left: 40,
          padding: 12,
          background: "white",
          borderRadius: 8,
        }}
      >
        重新打开窗口
      </button>
    );
  }

  if (scenario.startsWith("diagnosis")) {
    const diagnosis = [
      "diagnosis-empty",
      "diagnosis-loading",
      "diagnosis-error",
      "diagnosis-history",
    ].includes(scenario)
      ? makeDiagnosis()
      : makeDiagnosis({ conditionEntries: [makeCondition()] });
    const run = scenario === "diagnosis-analysis" ? makeRun() : null;
    const view = projectNativeDiagnosisWindow({ diagnosis, run });
    if (scenario === "diagnosis-safety") view.safetyReview = true;
    const controls: DiagnosisDecisionControls = {
      selection,
      onSelect: (id, choice) =>
        setSelection((current) => {
          const next = { ...current };
          if (choice) next[id] = choice;
          else delete next[id];
          return next;
        }),
      onReview: () => {},
      onSubmit: () => {},
      onDismissOutcome: () => {},
      onOpenChat: () => {},
      onRequestAnalysis: () => {},
    };
    return (
      <BusinessWindowHost
        windowId="diagnosis"
        mobile={mobile}
        stackIndex={0}
        isActive
        onClose={() => setClosed(true)}
        hideTitle
        headerLeft={
          <DiagnosisWindowHeader
            view={view}
            selectedConditionId={selectedConditionId}
            onSelectCondition={setSelectedConditionId}
            mode={
              scenario === "diagnosis-history"
                ? null
                : diagnosisHeaderMode(view)
            }
            history={scenario === "diagnosis-history"}
            onOpenHistory={() => {}}
          />
        }
        headerActions={
          <button
            type="button"
            className="bc-icon-button"
            aria-label="对话操作（预览）"
            title="预览不连接对话"
          >
            <MessageCircle size={19} />
          </button>
        }
      >
        {scenario === "diagnosis-history" ? (
          <section
            className="bc-business-window-body bc-dx"
            aria-label="综合诊断"
          >
            <div className="bc-window-body" role="region" aria-label="诊断历史">
              <DiagnosisHistoryList
                entries={[
                  {
                    key: "storybook:adopted",
                    at: "2026-10-19T10:00:00+08:00",
                    dateLabel: "10月19日",
                    title: "示例病情",
                    outcome: "adopted",
                    outcomeLabel: "采纳",
                    levelLabel: "AI 疑似",
                    note: null,
                  },
                  {
                    key: "storybook:rejected",
                    at: "2026-10-18T10:00:00+08:00",
                    dateLabel: "10月18日",
                    title: "另一项分析提案",
                    outcome: "rejected",
                    outcomeLabel: "未采纳",
                    levelLabel: null,
                    note: "提案已拒绝",
                  },
                ]}
                inactive={[]}
                renderCondition={() => null}
              />
            </div>
          </section>
        ) : (
          <DiagnosisWindow
            view={view}
            selectedConditionId={selectedConditionId}
            controls={controls}
            loading={scenario === "diagnosis-loading"}
            error={
              scenario === "diagnosis-error" ? "当前诊断暂时无法同步。" : null
            }
          />
        )}
      </BusinessWindowHost>
    );
  }

  const plan =
    scenario === "training-blocked"
      ? makePlan({
          executionGate: {
            ...makePlan().executionGate,
            result: "block",
            permitsExecution: false,
            reasons: ["safety_screening_incomplete"],
          },
        })
      : makePlan();
  const plans = [
    "training-empty",
    "training-loading",
    "training-error",
  ].includes(scenario)
    ? []
    : [plan];
  const proposals = ["training-proposal", "training-expired"].includes(scenario)
    ? [makeProposal()]
    : [];
  const state: ProjectTrainingWindowInput = {
    plans: scenario === "training-loading" ? undefined : plans,
    proposals,
    selectedPlanId,
    selectedTemplateId: templateId,
    focus,
    loading: scenario === "training-loading",
    error: scenario === "training-error" ? "当前训练计划暂时无法读取。" : null,
    refused:
      scenario === "training-expired"
        ? { [makeProposal().proposalId]: "base_revision_changed" }
        : {},
    now: NOW,
  };
  const controls: TrainingPlanControls = {
    onOpenProposal: (proposalId) => setFocus({ kind: "proposal", proposalId }),
    onBackToPlan: () => setFocus({ kind: "plan" }),
    onAccept: () => {},
    onReject: () => {},
    onDismissNotice: () => {},
    onSelectTemplate: setTemplateId,
    onStart: () => {},
    onOpenChat: () => {},
  };
  return (
    <BusinessWindowHost
      windowId="training"
      mobile={mobile}
      stackIndex={0}
      isActive
      onClose={() => setClosed(true)}
      hideTitle
      headerLeft={
        <TrainingWindowHeader
          view={selectTrainingPlan(plans, selectedPlanId)}
          onSelectPlan={(id) => {
            setSelectedPlanId(id);
            setFocus({ kind: "plan" });
          }}
        />
      }
      headerActions={
        <button
          type="button"
          className="bc-icon-button"
          aria-label="对话操作（预览）"
          title="预览不连接对话"
        >
          <MessageCircle size={19} />
        </button>
      }
    >
      <TrainingPlanWindow
        state={state}
        controls={controls}
        canExecute={false}
      />
    </BusinessWindowHost>
  );
}

function ContentPreview({ scenario }: { scenario: Scenario }) {
  return <ContentPreviewBody key={scenario} scenario={scenario} />;
}

const meta = {
  title: "BodySense/Windows/02 Content",
  component: ContentPreview,
  decorators: [windowCanvasDecorator],
  argTypes: {
    scenario: {
      control: "select",
      options: [
        "diagnosis-loading",
        "diagnosis-empty",
        "diagnosis-current",
        "diagnosis-analysis",
        "diagnosis-history",
        "diagnosis-safety",
        "diagnosis-error",
        "training-loading",
        "training-empty",
        "training-plan",
        "training-proposal",
        "training-expired",
        "training-blocked",
        "training-error",
      ],
    },
  },
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "真实 DiagnosisWindow / TrainingPlanWindow 及 Header。使用仓库现有测试数据，预览操作不会写入服务端。训练执行在预览中保持禁用。",
      },
    },
  },
} satisfies Meta<typeof ContentPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const DiagnosisLoading: Story = {
  args: { scenario: "diagnosis-loading" },
};
export const DiagnosisEmpty: Story = { args: { scenario: "diagnosis-empty" } };
export const DiagnosisCurrent: Story = {
  args: { scenario: "diagnosis-current" },
};
export const DiagnosisAnalysis: Story = {
  args: { scenario: "diagnosis-analysis" },
};
export const DiagnosisHistory: Story = {
  args: { scenario: "diagnosis-history" },
};
export const DiagnosisSafety: Story = {
  args: { scenario: "diagnosis-safety" },
};
export const DiagnosisError: Story = { args: { scenario: "diagnosis-error" } };
export const TrainingLoading: Story = {
  args: { scenario: "training-loading" },
};
export const TrainingEmpty: Story = { args: { scenario: "training-empty" } };
export const TrainingPlan: Story = { args: { scenario: "training-plan" } };
export const TrainingProposal: Story = {
  args: { scenario: "training-proposal" },
};
export const TrainingExpired: Story = {
  args: { scenario: "training-expired" },
};
export const TrainingBlocked: Story = {
  args: { scenario: "training-blocked" },
};
export const TrainingError: Story = { args: { scenario: "training-error" } };
