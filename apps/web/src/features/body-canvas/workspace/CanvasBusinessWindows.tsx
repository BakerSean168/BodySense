import { lazy, Suspense, useCallback, useState, type ReactNode } from "react";
import { Clock3, MessageCircle } from "lucide-react";
import type { BodyCanvasWorkspaceProps } from "./BodyCanvasWorkspace.types";
import type { BusinessWindowId } from "../model/businessWindows";
import { businessWindowStackIndex } from "../model/businessWindows";
import { diagnosisHeaderMode } from "../model/diagnosisDecisionViewModel";
import { BusinessWindowHost } from "../components/BusinessWindowHost";
import { WindowLoading } from "../components/WindowLoading";
import { ChatHistoryMenu } from "../../consultation/components/chat/ChatHistoryMenu";
import { ChatWindowHeader } from "../../consultation/components/chat/ChatWindowHeader";
import { DiagnosisWindowHeader } from "../windows/diagnosis/DiagnosisWindowHeader";
import { TrainerWindowHeader } from "../windows/training/TrainerWindowHeader";
import {
  TrainingSafetyHeader,
  TrainingWindowHeader,
} from "../windows/training/TrainingWindowHeader";
import { PersonWindowHeader } from "../windows/person/PersonWindowHeader";
import { PersonAddButton } from "../windows/person/PersonAddButton";

import { type PersonProfileIdentity } from "../windows/person/types";
import { TrainerEntryGuard } from "../components/TrainerEntryGuard";

import type { useCanvasBusinessWindowController } from "./useCanvasBusinessWindowController";
import { IconTip } from "@/components/ui/tooltip";
// First open loads the Person surface; mountedWindows preserves its drafts thereafter.
const PersonWindow = lazy(() =>
  import("../windows/person/PersonWindow").then((module) => ({
    default: module.PersonWindow,
  })),
);
// Lazy so the decision-state UI stays out of the ConsultationPage chunk budget.
const DiagnosisWindow = lazy(() =>
  import("../windows/diagnosis/DiagnosisWindow").then((module) => ({
    default: module.DiagnosisWindow,
  })),
);
// Penpot Safety / Restricted and Chat / Diagnosis request: lazy for the budget.
const SafetyRestrictedView = lazy(
  () => import("../components/SafetyRestrictedView"),
);
const DiagnosisRequestEntry = lazy(
  () => import("../components/DiagnosisRequestEntry"),
);
// Lazy for the same 500 kB ConsultationPage chunk budget (UI-TP-001).
const TrainingPlanWindow = lazy(() =>
  import("../windows/training/TrainingPlanWindow").then((module) => ({
    default: module.TrainingPlanWindow,
  })),
);
export interface CanvasBusinessWindowsProps {
  props: BodyCanvasWorkspaceProps;
  business: ReturnType<typeof useCanvasBusinessWindowController>;
  mobile: boolean;
  identity: PersonProfileIdentity;
  safetyReview: boolean;
}

/** Stable window keys and renderer composition for the workspace controller. */
export function CanvasBusinessWindows({
  props,
  business,
  mobile,
  identity,
  safetyReview,
}: CanvasBusinessWindowsProps) {
  const {
    mountedWindows,
    windows,
    activeWindow,
    openWindow,
    closeWindow,
    personCategory,
    setPersonCategory,
    personInfoOpen,
    setPersonInfoOpen,
    setAccountDialog,
    selectedConditionId,
    setSelectedConditionId,
    diagnosisActions,
    requestAnalysis,
    openSafety,
    trainingView,
    setTrainingView,
    trainingActions,
    setSelectedTemplateId,
    chooseTrainingPlan,
    trainerLaunch,
    setTrainerLaunch,
    chatHistory,
    chatDay,
    jumpToChatDay,
    analysisRequest,
    setAnalysisRequest,
  } = business;
  // Header "+" of the Person window; cleared whenever the category changes.
  const [personAddOpen, setPersonAddOpen] = useState(false);
  const [personAddLabel, setPersonAddLabel] = useState<string | null>(null);
  const changePersonCategory = useCallback(
    (next: Parameters<typeof setPersonCategory>[0]) => {
      setPersonAddOpen(false);
      setPersonCategory(next);
    },
    [setPersonCategory],
  );
  const canEditBody =
    props.workspace?.capabilities.can_edit_body_state === true;
  const diagnosisView = diagnosisActions.view;
  const trainingSelection = trainingActions.selection;

  const renderWindowBody = (window: BusinessWindowId): ReactNode => {
    switch (window) {
      case "chat":
        return (
          <div
            className="bc-chat-content"
            data-request={analysisRequest || undefined}
          >
            {analysisRequest ? (
              <Suspense fallback={null}>
                <DiagnosisRequestEntry
                  message={props.latestUserMessage ?? null}
                  expectedRevision={diagnosisView.currentRevision}
                  onClose={() => setAnalysisRequest(false)}
                  onOpenDiagnosis={() => {
                    setAnalysisRequest(false);
                    diagnosisActions.closeHistory();
                    openWindow("diagnosis");
                  }}
                  onOpenSafety={openSafety}
                />
              </Suspense>
            ) : null}
            {props.chat}
          </div>
        );
      case "person":
        return (
          <Suspense fallback={<WindowLoading label="正在读取个人档案" />}>
            <PersonWindow
              category={personCategory}
              onCategoryChange={changePersonCategory}
              identityInfoOpen={personInfoOpen}
              onIdentityInfoOpenChange={setPersonInfoOpen}
              canEdit={canEditBody}
              addOpen={personAddOpen}
              onAddOpenChange={setPersonAddOpen}
              onAddLabelChange={setPersonAddLabel}
              onOpenRecords={props.onOpenProfile}
            />
          </Suspense>
        );
      case "diagnosis":
        return (
          <Suspense fallback={<WindowLoading label="正在读取当前诊断" />}>
            <DiagnosisWindow
              view={diagnosisView}
              scene={diagnosisActions.scene}
              selectedConditionId={selectedConditionId}
              loading={
                (props.loading && !props.workspace) || diagnosisActions.loading
              }
              error={
                props.error && !props.workspace
                  ? "当前诊断暂时无法同步。"
                  : diagnosisActions.loadError
              }
              partialError={diagnosisActions.partialLoadError}
              onRetry={() => {
                void diagnosisActions.refresh();
                if (!props.workspace) props.onRetry?.();
              }}
              controls={{
                selection: diagnosisActions.selection,
                onSelect: diagnosisActions.select,
                onReview: diagnosisActions.reviewProposal,
                onSubmit: (final) => void diagnosisActions.submit(final),
                onDismissOutcome: diagnosisActions.dismissOutcome,
                onOpenChat: requestAnalysis,
                onRequestAnalysis: requestAnalysis,
                onDismissReview: (conditionId) =>
                  void diagnosisActions.dismissReview(conditionId),
                onOpenSafety: openSafety,
                busy: diagnosisActions.busy,
                error: diagnosisActions.error,
              }}
            />
          </Suspense>
        );
      case "training":
        if (trainingView === "safety")
          return (
            <Suspense fallback={<WindowLoading label="正在读取安全限制" />}>
              <SafetyRestrictedView
                onBack={() => setTrainingView("plan")}
                onAddReview={() => openWindow("chat")}
              />
            </Suspense>
          );
        return (
          <Suspense fallback={<WindowLoading label="正在读取训练计划" />}>
            <TrainingPlanWindow
              state={trainingActions.state}
              partialError={trainingActions.partialLoadError}
              canExecute={
                props.workspace?.capabilities.can_start_training === true &&
                !safetyReview
              }
              controls={{
                busy: trainingActions.busy,
                onRetry: () => void trainingActions.refresh(),
                error: trainingActions.error,
                notice: trainingActions.notice,
                onOpenProposal: (proposalId) =>
                  trainingActions.openProposal({ proposalId }),
                onBackToPlan: trainingActions.backToPlan,
                onAccept: trainingActions.accept,
                onReject: trainingActions.reject,
                onDismissNotice: trainingActions.dismissNotice,
                onSelectTemplate: (templateId) => {
                  setSelectedTemplateId(templateId);
                  trainingActions.backToPlan();
                },
                onOpenChat: () => openWindow("chat"),
                onStart: (planId, templateId) => {
                  // UI admission only: the Go service remains authoritative.
                  const plan = trainingSelection.selected;
                  if (
                    plan?.planId !== planId ||
                    plan.status !== "active" ||
                    !plan.gatePermits ||
                    props.workspace?.capabilities.can_start_training !== true ||
                    safetyReview
                  )
                    return;
                  setTrainerLaunch({
                    planId,
                    templateId,
                    key: crypto.randomUUID(),
                  });
                  openWindow("trainer");
                },
              }}
            />
          </Suspense>
        );
      case "trainer":
        return (
          <TrainerEntryGuard
            planId={trainingSelection.selected?.planId ?? null}
            launch={trainerLaunch}
            accepted={trainingSelection.selected?.status === "active"}
            canExecute={
              props.workspace?.capabilities.can_start_training === true
            }
            safetyBlocked={
              safetyReview || trainingSelection.selected?.gatePermits === false
            }
            loading={trainingActions.state.loading}
            error={trainingActions.state.error}
            onRetry={() => void trainingActions.refresh()}
            onOpenTraining={() => openWindow("training")}
          />
        );
      default:
        return null;
    }
  };

  const renderWindowHeader = (window: BusinessWindowId): ReactNode => {
    switch (window) {
      case "person":
        return (
          <PersonWindowHeader
            identity={identity}
            open={personInfoOpen}
            onToggle={() => setPersonInfoOpen((open) => !open)}
            onOpenSettings={() => setAccountDialog("settings")}
            onRequestLogout={() => setAccountDialog("logout")}
            category={personCategory}
            onCategoryChange={changePersonCategory}
          />
        );
      case "diagnosis":
        return (
          <DiagnosisWindowHeader
            view={diagnosisView}
            selectedConditionId={selectedConditionId}
            onSelectCondition={setSelectedConditionId}
            mode={diagnosisHeaderMode(diagnosisView)}
            history={diagnosisActions.scene === "history"}
            onOpenHistory={diagnosisActions.openHistory}
            onCloseHistory={diagnosisActions.closeHistory}
          />
        );
      case "training":
        if (trainingView === "safety") return <TrainingSafetyHeader />;
        return (
          <TrainingWindowHeader
            view={trainingSelection}
            onSelectPlan={(planId) => {
              chooseTrainingPlan(planId);
              trainingActions.backToPlan();
            }}
          />
        );
      case "trainer":
        return (
          <TrainerWindowHeader
            templateTitle={
              trainerLaunch &&
              trainingSelection.selected?.planId === trainerLaunch.planId
                ? (trainingSelection.selected.templates.find(
                    (template) =>
                      template.templateId === trainerLaunch.templateId,
                  )?.title ?? null)
                : null
            }
          />
        );
      case "chat":
        return (
          <ChatWindowHeader
            history={chatHistory}
            selectedDay={chatDay ?? chatHistory?.todayKey}
            onSelectDay={jumpToChatDay}
          />
        );
      default:
        return undefined;
    }
  };

  const renderHeaderActions = (window: BusinessWindowId): ReactNode => {
    if (window === "person") {
      const upload = personCategory === "media" || personCategory === "reports";
      return (
        <PersonAddButton
          category={personCategory}
          label={personCategory === "records" ? personAddLabel : null}
          disabled={!upload && !canEditBody}
          onAdd={() => {
            setPersonInfoOpen(false);
            setPersonAddOpen(true);
          }}
        />
      );
    }
    if (window === "chat")
      return chatHistory ? (
        <ChatHistoryMenu history={chatHistory} onSelectDay={jumpToChatDay} />
      ) : null;
    // Penpot Diagnosis / History has only the close button.
    if (window === "diagnosis" && diagnosisActions.scene === "history")
      return null;
    const historyAction =
      window === "diagnosis" &&
      !diagnosisHeaderMode(diagnosisView) &&
      diagnosisView.confirmed.length + diagnosisView.inactive.length > 0 ? (
        <IconTip label="查看诊断历史">
          <button
            type="button"
            className="bc-icon-button bc-header-chat-action"
            aria-label="查看诊断历史"
            onClick={diagnosisActions.openHistory}
          >
            <Clock3 size={19} strokeWidth={1.8} />
          </button>
        </IconTip>
      ) : null;
    return window === "diagnosis" || window === "training" ? (
      <>
        {historyAction}
        <IconTip label={window === "diagnosis" ? "讨论诊断" : "调整训练计划"}>
          <button
            type="button"
            className="bc-icon-button bc-header-chat-action"
            aria-label={
              window === "diagnosis"
                ? "与助手讨论当前诊断"
                : "与助手调整训练计划"
            }
            onClick={() => openWindow("chat")}
          >
            <MessageCircle size={19} strokeWidth={1.8} />
          </button>
        </IconTip>
      </>
    ) : null;
  };

  // The reducer keeps closed and minimized windows mounted for draft continuity.
  return (
    <div className="bc-business-layer">
      {mountedWindows.map((window) => (
        <BusinessWindowHost
          key={window}
          windowId={window}
          mobile={mobile}
          minimized={activeWindow !== window}
          stackIndex={businessWindowStackIndex(windows, window)}
          isActive={activeWindow === window}
          onClose={() => closeWindow(window)}
          headerLeft={renderWindowHeader(window)}
          headerActions={renderHeaderActions(window)}
          hideTitle={[
            "person",
            "diagnosis",
            "training",
            "trainer",
            "chat",
          ].includes(window)}
        >
          {renderWindowBody(window)}
        </BusinessWindowHost>
      ))}
    </div>
  );
}
