import type { BodyRecords } from "@/features/profile/services/bodyRecordsService";
import { lazy, Suspense, useCallback, useEffect, useMemo, useReducer, useRef, useState, type ReactNode } from "react";
import { ArrowRight, BoxSelect, Brush, ChevronDown, Clock3, Layers3, List, LoaderCircle, MessageCircle, Moon, MousePointer2, Pause, PersonStanding, Play, RotateCcw, ShieldAlert, Sun, Undo2, X } from "lucide-react";
import { AppUserMenu } from "@/components/layout/AppUserMenu";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import type { ConsultationSpatialContext } from "@/features/consultation/types/consultation";
import type { HealthWorkspace } from "@/features/workspace/types/workspace";
import { BodyExplorer } from "@/features/body-explorer/components/BodyExplorer";
import type { BodyExplorerWorkspaceController } from "@/features/body-explorer/hooks/useBodyExplorerWorkspace";
import { BODY_REGION_IDS, getBodyRegionDefinition, isBodyRegionId, type BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { recordBodyRegions, selectBodyRegionVisualSummaries } from "@/features/body-explorer/model/bodyExplorerSelectors";
import { buildCanvasContext, canvasShortcut, EMPTY_SELECTION, getReferenceMotion, REFERENCE_MOTIONS, selectionReducer, type CanvasPanel, type CanvasTool, type MotionId } from "../model/bodyCanvas";
import {
  activeBusinessWindow,
  businessWindowReducer,
  businessWindowStackIndex,
  EMPTY_BUSINESS_WINDOWS,
  isBusinessWindowVisible,
  type BusinessWindowId,
} from "../model/businessWindows";
import { useDiagnosisActions } from "@/features/consultation/hooks/useDiagnosisActions";
import { readSelectedPlanId, resolveSelectedPlanId, writeSelectedPlanId } from "../model/trainingPlanSelection";
import { useTrainingPlanActions } from "@/features/training/hooks/useTrainingPlanActions";
import { BusinessWindowHost } from "./BusinessWindowHost";
import { BodyCanvasDock } from "./BodyCanvasDock";
import { ChatWindowHeader, DiagnosisWindowHeader, TrainerWindowHeader, TrainingSafetyHeader, TrainingWindowHeader } from "./V2WindowHeaders";
import type { DiagnosisRequestMessage } from "./DiagnosisRequestEntry";
import {
  PersonIdentityTrigger,
  PersonWindow,
  type PersonProfileIdentity,
} from "./PersonWindow";
import { TrainerEntryGuard, type TrainerLaunch } from "./TrainerEntryGuard";
import { diagnosisHeaderMode } from "../model/diagnosisDecisionViewModel";
import "../body-canvas.css";

const BodyCanvasScene = lazy(() => import("./BodyCanvasScene"));
// Lazy so the decision-state UI stays out of the ConsultationPage chunk budget.
const DiagnosisWindow = lazy(() =>
  import("./DiagnosisWindow").then((module) => ({ default: module.DiagnosisWindow })),
);
// Penpot Safety / Restricted and Chat / Diagnosis request: lazy for the budget.
const SafetyRestrictedView = lazy(() => import("./SafetyRestrictedView"));
const DiagnosisRequestEntry = lazy(() => import("./DiagnosisRequestEntry"));
// Lazy for the same 500 kB ConsultationPage chunk budget (UI-TP-001).
const TrainingPlanWindow = lazy(() =>
  import("./TrainingPlanWindow").then((module) => ({ default: module.TrainingPlanWindow })),
);
// Penpot Home / trainer-active|paused; lazy for the same chunk budget.
const TrainerStatusChip = lazy(() =>
  import("@/features/training/components/TrainerStatusChip").then((module) => ({
    default: module.TrainerStatusChip,
  })),
);

export interface BodyCanvasWorkspaceProps {
  bodyState: BodyRecords | null;
  workspace?: HealthWorkspace;
  bodyExplorer: BodyExplorerWorkspaceController;
  chat: ReactNode;
  /** The user's latest completed message: the only candidate analysis request. */
  latestUserMessage?: DiagnosisRequestMessage | null;
  onAskContext: (context: ConsultationSpatialContext) => void;
  onOpenProfile: () => void;
  /** Real profile values for the Person window; never invented when absent. */
  identity?: PersonProfileIdentity;
  /** Deep link (?view=…) opens the matching business window once. */
  initialWindow?: BusinessWindowId | null;
  onWindowChange?: (window: BusinessWindowId | null) => void;
  /**
   * Requests a business window from outside the canvas (for example the anatomy
   * overlay's "ask" action). The revision counter makes repeated requests for the
   * same window observable without re-opening on unrelated re-renders.
   */
  windowRequest?: { window: BusinessWindowId; revision: number } | null;
  demonstrationRequest?: { motion: MotionId; revision: number } | null;
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
}

const TOOLS = [
  ["orbit", "旋转与点选", "V", MousePointer2],
  ["brush", "涂抹选区", "B", Brush],
  ["box", "框选区域", "S", BoxSelect],
] as const;

const CANVAS_OVERLAY_META: Record<"anatomy" | "help", { title: string; description: string }> = {
  anatomy: { title: "精细解剖", description: "探索结构与标准身体区域。解剖图谱不是个人扫描。" },
  help: { title: "让操作顺手一点", description: "每个快捷操作都有可见按钮，也支持按名称选择区域。" },
};

function savedTheme(): boolean {
  try { return localStorage.getItem("bodysense.canvas.theme") === "dark"; } catch { return false; }
}

export function BodyCanvasWorkspace(props: BodyCanvasWorkspaceProps) {
  const reducedMotion = useMediaQuery("(prefers-reduced-motion: reduce)");
  const mobile = useMediaQuery("(max-width: 760px)");
  const [dark, setDark] = useState(savedTheme);
  const [tool, setTool] = useState<CanvasTool>("orbit");
  const [selection, dispatch] = useReducer(selectionReducer, EMPTY_SELECTION);
  const [canvasOverlay, setCanvasOverlay] = useState<"anatomy" | "help" | null>(null);
  const [motion, setMotion] = useState<MotionId>("stand");
  const [motionMode, setMotionMode] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [phase, setPhase] = useState(0);
  const [seek, setSeek] = useState({ phase: 0, revision: 0 });
  const [resetKey, setResetKey] = useState(0);
  const [annotationEpoch, setAnnotationEpoch] = useState(0);
  const [regionListOpen, setRegionListOpen] = useState(false);
  const [textOnly, setTextOnly] = useState(false);
  const [regionCardOpen, setRegionCardOpen] = useState(true);
  const [windows, dispatchWindow] = useReducer(businessWindowReducer, EMPTY_BUSINESS_WINDOWS);
  // A window that has been opened stays mounted (hidden when closed/minimized) so
  // closing, minimizing or switching never discards the conversation, a pending
  // interaction, scroll position or unsaved composer input.
  const [mountedWindows, setMountedWindows] = useState<readonly BusinessWindowId[]>([]);
  // Declarative mount tracking: any window that the reducer has ever opened stays
  // mounted for the rest of the session, so no open path (Dock toggle, shortcut,
  // deep link or programmatic open) can accidentally unmount a live task. Closing
  // a window hides it; it does not destroy the conversation or a pending form.
  useEffect(() => {
    setMountedWindows((current) => {
      const missing = windows.order.filter((window) => !current.includes(window));
      return missing.length === 0 ? current : [...current, ...missing];
    });
  }, [windows.order]);
  // The Person identity popover is anchored in the shared header, so its open
  // state lives with the header owner rather than inside the body.
  const [personInfoOpen, setPersonInfoOpen] = useState(false);
  const [selectedPlanId, setSelectedPlanId] = useState<string | null>(() => readSelectedPlanId());
  const [selectedConditionId, setSelectedConditionId] = useState<string | null>(null);
  /** Training window body: the plan, or Penpot Safety / Restricted. */
  const [trainingView, setTrainingView] = useState<"plan" | "safety">("plan");
  /** Chat shows the formal-analysis request entry (explicit user intent only). */
  const [analysisRequest, setAnalysisRequest] = useState(false);
  /** Page-local explicit Start intent. A deep link or shortcut never supplies it. */
  const [trainerLaunch, setTrainerLaunch] = useState<TrainerLaunch | null>(null);
  const sceneRef = useRef<HTMLDivElement>(null);
  const selectRef = useRef<HTMLSelectElement>(null);
  const activeWindow = activeBusinessWindow(windows);
  const trainingOpen = isBusinessWindowVisible(windows, "training") || isBusinessWindowVisible(windows, "trainer");
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const chooseTrainingPlan = useCallback((planId: string) => {
    setSelectedPlanId(planId);
    setSelectedTemplateId(null);
    writeSelectedPlanId(planId);
  }, []);
  const trainingActions = useTrainingPlanActions({
    enabled: trainingOpen,
    selectedPlanId,
    selectedTemplateId,
    onPlanAccepted: chooseTrainingPlan,
  });
  const trainingSelection = trainingActions.selection;

  const definition = getReferenceMotion(motion);
  const primary = selection.regions[0] ?? null;
  const safetyReview = props.workspace?.capabilities.requires_safety_review === true;
  // Playback is a background behaviour: any open window pauses it.
  const effectivePlaying = playing && !safetyReview && !activeWindow;
  const summaries = useMemo(() => selectBodyRegionVisualSummaries(props.bodyState), [props.bodyState]);
  const attention = summaries.filter((summary) => summary.visualState !== "none");
  const regionFacts = useMemo(() => (props.bodyState?.facts ?? []).filter((fact) => {
    return recordBodyRegions(fact).some((region) => selection.regions.includes(region)) && fact.assertion_status === "asserted";
  }), [props.bodyState, selection.regions]);
  const selectionLabel = selection.regions.map((id) => getBodyRegionDefinition(id).labels["zh-CN"]).join("、");
  const diagnosisActions = useDiagnosisActions({
    enabled: !!props.workspace,
    safetyReview,
  });
  const diagnosisView = diagnosisActions.view;

  const openWindow = useCallback((window: BusinessWindowId) => {
    dispatchWindow({ type: "open", window });
    setRegionListOpen(false);
  }, []);
  const closeWindow = useCallback((window: BusinessWindowId) => {
    dispatchWindow({ type: "close", window });
  }, []);
  const requestAnalysis = useCallback(() => {
    setAnalysisRequest(true);
    dispatchWindow({ type: "open", window: "chat" });
    setRegionListOpen(false);
  }, []);
  const openSafety = useCallback(() => {
    setTrainingView("safety");
    dispatchWindow({ type: "open", window: "training" });
    setRegionListOpen(false);
  }, []);
  const trainingVisible = isBusinessWindowVisible(windows, "training");
  useEffect(() => {
    if (!trainingVisible) setTrainingView("plan");
  }, [trainingVisible]);
  const diagnosisVisible = isBusinessWindowVisible(windows, "diagnosis");
  const closeDiagnosisHistory = diagnosisActions.closeHistory;
  useEffect(() => {
    if (!diagnosisVisible) closeDiagnosisHistory();
  }, [diagnosisVisible, closeDiagnosisHistory]);
  const toggleWindow = useCallback((window: BusinessWindowId) => {
    dispatchWindow({ type: "toggle", window });
    setRegionListOpen(false);
  }, []);
  /** Home chip: reopen the live session itself; nothing new is started. */
  const reopenTrainer = useCallback(
    (session: { sessionId: string; planId: string; templateId: string }) => {
      chooseTrainingPlan(session.planId);
      setTrainerLaunch({
        planId: session.planId,
        templateId: session.templateId,
        sessionId: session.sessionId,
        key: crypto.randomUUID(),
      });
      openWindow("trainer");
    },
    [chooseTrainingPlan, openWindow],
  );
  const closeAllWindows = useCallback(() => {
    dispatchWindow({ type: "closeAll" });
  }, []);

  // Reconcile a stale persisted plan id against the real plan list so the window
  // never displays another plan's identity.
  const trainingPlans = trainingSelection.plans;
  useEffect(() => {
    if (trainingPlans.length === 0) return;
    setSelectedPlanId((current) => {
      const next = resolveSelectedPlanId(trainingPlans, current);
      if (next !== current) writeSelectedPlanId(next);
      return next;
    });
  }, [trainingPlans]);

  useEffect(() => { if (reducedMotion) setPlaying(false); }, [reducedMotion]);
  useEffect(() => {
    try { localStorage.setItem("bodysense.canvas.theme", dark ? "dark" : "light"); } catch { /* Preferences never gate the workspace. */ }
  }, [dark]);
  useEffect(() => {
    const external = props.bodyExplorer.selectedRegionId;
    if (external !== (selection.regions[0] ?? null)) {
      dispatch({ type: "select", regions: external ? [external] : [] });
      setRegionCardOpen(true);
    }
    // A bridge selection from the anatomy panel takes priority only when that bridge changes.
  }, [props.bodyExplorer.selectedRegionId]);

  // Report the active business window so the route can deep-link it.
  const reportedWindow = props.onWindowChange;
  useEffect(() => {
    reportedWindow?.(activeWindow);
  }, [activeWindow, reportedWindow]);

  // Deep link: open the requested window once, without fighting later closes.
  const initialWindow = props.initialWindow ?? null;
  const appliedInitialRef = useRef<string | null>(null);
  useEffect(() => {
    if (!initialWindow || appliedInitialRef.current === initialWindow) return;
    appliedInitialRef.current = initialWindow;
    openWindow(initialWindow);
  }, [initialWindow, openWindow]);

  // An external request (e.g. asking a question from the anatomy overlay) must
  // both open the target window and dismiss the modal that triggered it, so the
  // user never ends up with a covered window behind a dialog.
  const windowRequest = props.windowRequest ?? null;
  const windowRequestRevision = windowRequest?.revision;
  useEffect(() => {
    if (!windowRequest) return;
    setCanvasOverlay(null);
    openWindow(windowRequest.window);
  }, [windowRequest, windowRequestRevision, openWindow]);

  const select = useCallback((regions: BodyRegionId[], append = false) => {
    const next = selectionReducer(selection, { type: "select", regions, append });
    dispatch({ type: "select", regions, append });
    props.bodyExplorer.selectRegion(next.regions[0] ?? null);
    setPlaying(false); setRegionCardOpen(true);
    if (mobile) setTool("orbit");
  }, [selection, props.bodyExplorer.selectRegion, mobile]);
  const clear = useCallback(() => {
    dispatch({ type: "clear" }); props.bodyExplorer.selectRegion(null);
    setAnnotationEpoch((value) => value + 1);
  }, [props.bodyExplorer.selectRegion]);
  const undo = useCallback(() => {
    const next = selectionReducer(selection, { type: "undo" });
    dispatch({ type: "undo" }); props.bodyExplorer.selectRegion(next.regions[0] ?? null);
    setAnnotationEpoch((value) => value + 1);
  }, [selection, props.bodyExplorer.selectRegion]);
  const removeRegion = (region: BodyRegionId) => {
    const next = selectionReducer(selection, { type: "remove", region });
    dispatch({ type: "remove", region }); props.bodyExplorer.selectRegion(next.regions[0] ?? null);
  };
  const ask = useCallback(() => {
    setPlaying(false);
    props.onAskContext(buildCanvasContext(selection.regions, motionMode ? motion : null, phase, effectivePlaying));
    openWindow("chat");
  }, [selection.regions, motion, motionMode, phase, effectivePlaying, props.onAskContext, openWindow]);
  const chooseMotion = useCallback((id: MotionId) => {
    setMotion(id); setMotionMode(true); setPhase(0); setSeek((value) => ({ phase: 0, revision: value.revision + 1 }));
    setPlaying(!safetyReview && !reducedMotion); setTool("orbit"); setRegionCardOpen(false);
    setCanvasOverlay(null);
  }, [safetyReview, reducedMotion]);
  const requestRevision = props.demonstrationRequest?.revision;
  useEffect(() => {
    if (props.demonstrationRequest) chooseMotion(props.demonstrationRequest.motion);
  }, [requestRevision, chooseMotion]);
  const onTextFallback = useCallback(() => {
    setTextOnly(true); setRegionListOpen(true);
    requestAnimationFrame(() => selectRef.current?.focus());
  }, []);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      if (event.key === "Escape" && !event.defaultPrevented) {
        // The actual modal/menu owns its Escape; a non-modal mobile business
        // sheet does not. Do not dismiss a native select's open options.
        if (
          target?.closest("[role='menu'], [aria-modal='true'], select")
        ) return;
        // Close the transient Person overlay before hiding the whole window.
        if (personInfoOpen) setPersonInfoOpen(false);
        else if (activeWindow) closeWindow(activeWindow);
        else if (canvasOverlay) setCanvasOverlay(null);
        else if (tool !== "orbit") setTool("orbit");
        else if (regionListOpen) setRegionListOpen(false);
        else clear();
        event.preventDefault();
        return;
      }
      // Base UI owns keyboard input in menus/dialogs and form fields.
      if (target?.closest("[role='dialog'], [role='menu']")) return;
      const action = canvasShortcut(event);
      if (!action) return;
      if (action.type === "panel") {
        if (action.panel === "anatomy" || action.panel === "help") {
          const overlay = action.panel;
          setCanvasOverlay((current) => (current === overlay ? null : overlay));
        } else if (CANVAS_PANEL_TO_WINDOW[action.panel]) {
          toggleWindow(CANVAS_PANEL_TO_WINDOW[action.panel]!);
        } else {
          setCanvasOverlay("help");
        }
      } else if (action.type === "tool") {
        setTool(action.tool);
        if (action.tool !== "orbit") setPlaying(false);
      } else if (action.type === "motion") chooseMotion(action.motion);
      else if (action.type === "pause") {
        setMotionMode(true);
        if (!safetyReview) setPlaying((value) => !value);
      } else if (action.type === "reset") setResetKey((value) => value + 1);
      else if (action.type === "undo") undo();
      else if (action.type === "chat") ask();
      event.preventDefault();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [
    activeWindow,
    ask,
    canvasOverlay,
    chooseMotion,
    clear,
    closeWindow,
    personInfoOpen,
    regionListOpen,
    safetyReview,
    tool,
    toggleWindow,
    undo,
  ]);

  const showRegionCard = primary && regionCardOpen && !activeWindow && !regionListOpen;
  const sceneFallback = <div className="bc-scene-status" role="status"><LoaderCircle className="animate-spin" size={28} /><p>正在准备身体视图</p><button className="bc-button" onClick={onTextFallback}>按名称选择区域</button></div>;
  const themeClass = `body-canvas-theme${dark ? " bc-dark" : ""}`;
  const identity: PersonProfileIdentity = props.identity ?? { nickname: "" };

  const renderWindowBody = (window: BusinessWindowId): ReactNode => {
    switch (window) {
      case "chat":
        return (
          <div className="bc-chat-content" data-request={analysisRequest || undefined}>
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
          <PersonWindow
            identityInfoOpen={personInfoOpen}
            onIdentityInfoOpenChange={setPersonInfoOpen}
            canEdit={props.workspace?.capabilities.can_edit_body_state === true}
            onOpenRecords={props.onOpenProfile}
          />
        );
      case "diagnosis":
        return (
          <Suspense
            fallback={
              <p role="status" className="bc-window-loading">
                正在读取当前诊断
              </p>
            }
          >
          <DiagnosisWindow
            view={diagnosisView}
            scene={diagnosisActions.scene}
            selectedConditionId={selectedConditionId}
            loading={(props.loading && !props.workspace) || diagnosisActions.loading}
            error={
              props.error && !props.workspace
                ? "当前诊断暂时无法同步。"
                : diagnosisActions.loadError
            }
            controls={{
              selection: diagnosisActions.selection,
              onSelect: diagnosisActions.select,
              onReview: diagnosisActions.reviewProposal,
              onSubmit: (final) => void diagnosisActions.submit(final),
              onDismissOutcome: diagnosisActions.dismissOutcome,
              onOpenChat: requestAnalysis,
              onRequestAnalysis: requestAnalysis,
              onDismissReview: (conditionId) => void diagnosisActions.dismissReview(conditionId),
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
            <Suspense
              fallback={
                <p role="status" className="bc-window-loading">
                  正在读取安全限制
                </p>
              }
            >
              <SafetyRestrictedView
                onBack={() => setTrainingView("plan")}
                onAddReview={() => openWindow("chat")}
              />
            </Suspense>
          );
        return (
          <Suspense
            fallback={
              <p role="status" className="bc-window-loading">
                正在读取训练计划
              </p>
            }
          >
          <TrainingPlanWindow
            state={trainingActions.state}
            canExecute={
              props.workspace?.capabilities.can_start_training === true && !safetyReview
            }
            controls={{
              busy: trainingActions.busy,
              error: trainingActions.error,
              notice: trainingActions.notice,
              onOpenProposal: (proposalId) => trainingActions.openProposal({ proposalId }),
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
                ) return;
                setTrainerLaunch({ planId, templateId, key: crypto.randomUUID() });
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
            canExecute={props.workspace?.capabilities.can_start_training === true}
            safetyBlocked={safetyReview || trainingSelection.selected?.gatePermits === false}
            loading={trainingActions.state.loading}
            error={trainingActions.state.error}
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
          <PersonIdentityTrigger
            identity={identity}
            open={personInfoOpen}
            onToggle={() => setPersonInfoOpen((open) => !open)}
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
              trainerLaunch && trainingSelection.selected?.planId === trainerLaunch.planId
                ? (trainingSelection.selected.templates.find(
                    (template) => template.templateId === trainerLaunch.templateId,
                  )?.title ?? null)
                : null
            }
          />
        );
      case "chat":
        return <ChatWindowHeader />;
      default:
        return undefined;
    }
  };

  const renderHeaderActions = (window: BusinessWindowId): ReactNode => {
    // Penpot Diagnosis / History has only the close button.
    if (window === "diagnosis" && diagnosisActions.scene === "history") return null;
    const historyAction =
      window === "diagnosis" &&
      !diagnosisHeaderMode(diagnosisView) &&
      diagnosisView.confirmed.length + diagnosisView.inactive.length > 0 ? (
        <button
          type="button"
          className="bc-icon-button bc-header-chat-action"
          aria-label="查看诊断历史"
          title="查看诊断历史"
          onClick={diagnosisActions.openHistory}
        >
          <Clock3 size={19} strokeWidth={1.8} />
        </button>
      ) : null;
    return window === "diagnosis" || window === "training" ? (
      <>
      {historyAction}
      <button
        type="button"
        className="bc-icon-button bc-header-chat-action"
        aria-label={
          window === "diagnosis" ? "与助手讨论当前诊断" : "与助手调整训练计划"
        }
        title="打开对话，不会自动提交或修改记录"
        onClick={() => openWindow("chat")}
      >
        <MessageCircle size={19} strokeWidth={1.8} />
      </button>
      </>
    ) : null;
  };

  return <main className={`${themeClass} bc-workspace`} data-testid="body-canvas-workspace">
    <header className="bc-topbar">
      <button className="bc-brand" onClick={() => { closeAllWindows(); setCanvasOverlay(null); setRegionListOpen(false); }} aria-label="返回身体画布"><PersonStanding size={26} strokeWidth={1.7} /><span>BodySense</span></button>
      <div className="bc-mode-switch" aria-label="画布模式">
        <button aria-pressed={!motionMode} onClick={() => { setMotionMode(false); setPlaying(false); setMotion("stand"); setSeek((value) => ({ phase: 0, revision: value.revision + 1 })); }}>探索身体</button>
        <button aria-pressed={motionMode} onClick={() => { setMotionMode(true); setRegionCardOpen(false); }}>动作演示</button>
      </div>
      <div className="bc-account"><span className="bc-private-label">你的身体，由你确认</span><button className="bc-icon-button" onClick={() => setDark((value) => !value)} aria-label={dark ? "切换浅色" : "切换深色"} title={dark ? "切换浅色" : "切换深色"}>{dark ? <Sun size={19} /> : <Moon size={19} />}</button><AppUserMenu compact onOpenProfile={() => openWindow("person")} /></div>
    </header>

    <section className={`bc-stage${activeWindow || showRegionCard ? " bc-stage-context" : ""}${motionMode ? " bc-stage-motion" : ""}`} aria-label="身体画布">
      <div className="bc-render-area" ref={sceneRef}>
        {!textOnly ? <Suspense fallback={sceneFallback}><BodyCanvasScene tool={tool} selectedRegions={selection.regions} motion={motion} playing={effectivePlaying} speed={speed} seek={seek} resetKey={resetKey} annotationEpoch={annotationEpoch} dark={dark} onSelect={select} onGestureStart={() => setPlaying(false)} onPhaseChange={setPhase} onTextFallback={onTextFallback} /></Suspense> : <div className="bc-text-scene"><PersonStanding size={100} strokeWidth={.7} /><h2>用区域名称，一样能开始。</h2><p>3D 不是查看身体记录或提问的前提。</p><button className="bc-button" onClick={() => setTextOnly(false)}>恢复 3D 视图</button></div>}
      </div>

      <div className="bc-intro"><span className="bc-eyebrow">{motionMode ? "MOTION STUDIO" : "BODY CANVAS"}</span><h1>{motionMode ? <>观察动作，<br />问得更具体。</> : "我的身体"}</h1><p>{motionMode ? "暂停、旋转，再选中关心的部位。" : "从身体开始，理解每一次变化。"}</p>
        {!motionMode && !props.loading && !props.error && <div className="bc-attention"><span className="bc-status">{attention.length ? `${attention.length} 个有记录的区域` : "还没有区域记录"}</span>{attention.slice(0, 3).map((summary) => <button className="bc-region-link" key={summary.regionId} onClick={() => select([summary.regionId])}><span>{getBodyRegionDefinition(summary.regionId).labels["zh-CN"]}</span><ArrowRight size={15} /></button>)}{!attention.length && <p className="bc-onboarding-note">点击身体，或直接描述你的感受。<br />不需要先填满所有信息。</p>}</div>}
      </div>

      {safetyReview && <div role="alert" className="bc-safety-banner"><ShieldAlert size={19} /><span>需要先处理安全提醒，动作播放已暂停。</span><button onClick={() => openWindow("person")}>查看记录<ArrowRight size={15} /></button></div>}
      {props.error && <div role="alert" className="bc-sync-banner">身体信息暂时无法同步。{props.onRetry && <button onClick={props.onRetry}>重试</button>}现有对话与参考模型仍可使用。</div>}
      {props.loading && <span className="bc-sync-indicator" role="status"><LoaderCircle size={14} className="animate-spin" />正在同步身体记录</span>}

      {props.workspace && !activeWindow ? (
        <Suspense fallback={null}>
          <TrainerStatusChip onOpen={reopenTrainer} />
        </Suspense>
      ) : null}

      <div className="bc-tool-rail" role="toolbar" aria-label="身体画布工具">
        {TOOLS.map(([value, label, key, Icon]) => <button key={value} className="bc-icon-button" aria-label={label} title={`${label} · ${key}`} aria-pressed={tool === value} onClick={() => { setTool(value); setRegionListOpen(false); if (value !== "orbit") setPlaying(false); }}><Icon size={20} strokeWidth={1.7} /><span className="bc-tool-caption">{label}</span></button>)}
        <span className="bc-tool-divider" />
        <button className="bc-icon-button" aria-label="按名称选择区域" title="按名称选择区域" aria-expanded={regionListOpen} onClick={() => { setRegionListOpen((open) => !open); requestAnimationFrame(() => selectRef.current?.focus()); }}><List size={20} /></button>
        <button className="bc-icon-button" aria-label="精细解剖" title="精细解剖 · X" aria-pressed={canvasOverlay === "anatomy"} onClick={() => setCanvasOverlay((current) => (current === "anatomy" ? null : "anatomy"))}><Layers3 size={20} /></button>
        <button className="bc-icon-button" aria-label="重置视角" title="重置视角 · R" onClick={() => setResetKey((value) => value + 1)}><RotateCcw size={19} /></button>
      </div>
      {tool !== "orbit" && <div className="bc-tool-hint" role="status">{tool === "brush" ? "涂抹可见表面，选择需要询问的区域" : "拖出选框，只选择可见身体区域"}<span>Shift 追加 · Esc 返回</span><button aria-label="退出选区模式" onClick={() => setTool("orbit")}><X size={15} /></button></div>}

      {regionListOpen && <aside className="bc-region-picker bc-floating-panel" aria-label="区域名称选择">
        <div className="bc-panel-heading"><div><span className="bc-eyebrow">BODY REGIONS</span><h2>从区域开始</h2></div><button className="bc-icon-button" onClick={() => setRegionListOpen(false)} aria-label="关闭区域列表"><X size={20} /></button></div>
        <label className="bc-field">选择身体区域<select ref={selectRef} aria-label="选择身体区域" value={primary ?? ""} onChange={(event) => { if (isBodyRegionId(event.target.value)) select([event.target.value]); }}><option value="">请选择区域</option>{BODY_REGION_IDS.map((id) => <option key={id} value={id}>{getBodyRegionDefinition(id).labels["zh-CN"]}</option>)}</select></label>
        <p className="bc-muted">左右以人体自身为准。可按名称追加多个区域。</p>
        <div className="bc-region-grid">{BODY_REGION_IDS.map((id) => <button key={id} aria-pressed={selection.regions.includes(id)} onClick={() => select([id], true)}>{getBodyRegionDefinition(id).labels["zh-CN"]}</button>)}</div>
        <button className="bc-button bc-primary" disabled={!primary} onClick={() => { setRegionListOpen(false); setRegionCardOpen(true); }}>查看已选区域<ArrowRight size={16} /></button>
      </aside>}

      {showRegionCard && <aside className="bc-region-card bc-floating-panel" aria-label="已选区域当前状况">
        <div className="bc-panel-heading"><div><span className="bc-eyebrow">{selection.regions.length > 1 ? "SELECTION / 选区预览" : "CURRENT REGION"}</span><h2>{selection.regions.length > 1 ? `${selection.regions.length} 个区域` : getBodyRegionDefinition(primary).labels["zh-CN"]}</h2></div><button className="bc-icon-button" onClick={() => setRegionCardOpen(false)} aria-label="收起区域详情"><X size={20} /></button></div>
        <div className="bc-selection-chips">{selection.regions.map((id) => <button key={id} onClick={() => removeRegion(id)} aria-label={`取消选择${getBodyRegionDefinition(id).labels["zh-CN"]}`}>{getBodyRegionDefinition(id).labels["zh-CN"]}<X size={12} /></button>)}</div>
        <div className="bc-region-facts">{props.loading ? <p role="status">正在读取这个区域的记录…</p> : props.error && !props.bodyState ? <p>暂时无法读取记录；这不代表这个区域没有问题。</p> : regionFacts.length ? <><span className="bc-small-label">用户陈述 {regionFacts.length} 条</span>{regionFacts.slice(0, 3).map((fact) => <article key={fact.id}><strong>{fact.statement}</strong><span>用户陈述 · {new Date(fact.recorded_at).toLocaleDateString("zh-CN")}</span></article>)}</> : <><h3>这里还没有已记录的信息</h3><p>可以告诉我你的真实感受。选中一个位置，不代表那里一定存在问题。</p></>}</div>
        {motionMode && <div className="bc-context-note"><Play size={15} /><span>{definition.label} · {(phase * definition.duration).toFixed(2)}s<br /><small>通用参考动作，不是你的实际动作证据</small></span></div>}
        <div className="bc-panel-actions"><button className="bc-button bc-primary" onClick={ask}>针对这里提问<ArrowRight size={17} /></button><button className="bc-button" onClick={() => openWindow("person")}>查看完整记录</button><div className="bc-actions"><button className="bc-text-button" onClick={undo} disabled={!selection.history.length}><Undo2 size={15} />撤销选择</button><button className="bc-text-button" onClick={clear}>清除选区</button></div></div>
      </aside>}
      {primary && !showRegionCard && !activeWindow && !regionListOpen && <button className="bc-reopen-selection bc-button" onClick={() => setRegionCardOpen(true)}>{selectionLabel}<ChevronDown size={16} /></button>}
      {!primary && !activeWindow && !regionListOpen && !motionMode && <div className="bc-ask-entry"><button className="bc-button bc-primary" onClick={ask}>开始一次提问<MessageCircle size={18} /></button><p>明确的本人陈述会自动记录；有歧义时再向你确认。</p></div>}

      {motionMode && <>
        <div className="bc-motion-switch" aria-label="参考动作">{REFERENCE_MOTIONS.slice(0, 4).map((item, index) => <button key={item.id} aria-pressed={motion === item.id} onClick={() => chooseMotion(item.id)}>{item.label}<kbd>{index + 1}</kbd></button>)}</div>
        <section className="bc-playback" aria-label="动作播放控制">
          <button className="bc-play-toggle" disabled={safetyReview} onClick={() => setPlaying((value) => !value)} aria-label={effectivePlaying ? "暂停动作" : "播放动作"}>{effectivePlaying ? <Pause size={20} /> : <Play size={20} />}</button>
          <div className="bc-playback-track"><div><strong>{definition.label}</strong><span>{(phase * definition.duration).toFixed(2)}s / {definition.duration.toFixed(2)}s</span></div><input type="range" min="0" max="1" step="0.001" value={phase} aria-label="动作进度" onChange={(event) => { const next = Number(event.target.value); setPlaying(false); setPhase(next); setSeek((value) => ({ phase: next, revision: value.revision + 1 })); }} /></div>
          <label className="bc-speed"><span className="sr-only">播放速度</span><select aria-label="播放速度" value={speed} onChange={(event) => setSpeed(Number(event.target.value))}><option value={0.5}>0.5×</option><option value={1}>1×</option><option value={1.5}>1.5×</option></select></label>
          <button className="bc-icon-button" onClick={ask} aria-label="询问当前动作"><MessageCircle size={19} /></button>
        </section>
        <span className="bc-motion-disclaimer">参考动画 · 不估算真实受力，不作为医学评估</span>
      </>}

      {/* Business windows: desktop floats over the body, mobile becomes a sheet.
          The same reducer drives both responsive shells; a closed-but-mounted
          task stays hidden without losing unsaved draft or scroll state. */}
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
              hideTitle={["person", "diagnosis", "training", "trainer", "chat"].includes(
                window,
              )}
            >
              {renderWindowBody(window)}
            </BusinessWindowHost>
          ))}
        </div>

      <div className="bc-attribution"><span>参考人体 · 非个人扫描</span><span>Vanatome / Z-Anatomy · <a href="https://creativecommons.org/licenses/by-sa/4.0/" target="_blank" rel="noreferrer">CC BY-SA 4.0</a></span><span>拖动旋转 · 滚轮缩放 · 点击选区</span></div>
      <button className="bc-help-entry bc-button" onClick={() => setCanvasOverlay("help")}>快捷键<kbd>?</kbd></button>
      <BodyCanvasDock activeWindow={activeWindow} confirmedDiagnosisCount={diagnosisView.confirmed.length} onToggle={toggleWindow} />
    </section>

    <Dialog open={canvasOverlay !== null} onOpenChange={(open) => { if (!open) setCanvasOverlay(null); }}>
      <DialogContent showCloseButton={false} className={`${themeClass} bc-inventory-dialog${canvasOverlay === "anatomy" ? " bc-anatomy-dialog" : ""}`}>
        <div className="bc-inventory-heading"><div><span className="bc-eyebrow">{canvasOverlay ? `CANVAS TOOL / ${canvasOverlay.toUpperCase()}` : "CANVAS TOOL"}</span><DialogTitle className="bc-inventory-title">{canvasOverlay ? CANVAS_OVERLAY_META[canvasOverlay].title : "画布工具"}</DialogTitle><DialogDescription>{canvasOverlay ? CANVAS_OVERLAY_META[canvasOverlay].description : ""}</DialogDescription></div><button className="bc-icon-button" aria-label="关闭当前面板" onClick={() => setCanvasOverlay(null)}><X size={21} /></button></div>
        <div className="bc-inventory-content">
          {canvasOverlay === "anatomy" && <BodyExplorer snapshot={props.bodyState} semanticBridge={props.bodyExplorer.semanticBridge} className="bc-anatomy-viewer" />}
          {canvasOverlay === "help" && <div className="bc-keyboard-help">{[["V / B / S", "旋转点选、涂抹、框选"], ["1 / 2 / 3 / 4", "站立、跑步、跳跃、坐姿"], ["F / Space", "暂停或继续动作"], ["/", "打开对话窗口"], ["P / D / I / T", "训练计划、诊断、档案、训练执行"], ["X / R", "精细解剖、复位视角"], ["Z / Esc", "撤销选区、逐层返回"]].map(([key, label]) => <div key={key}><kbd>{key}</kbd><span>{label}</span></div>)}<p>输入文字、使用输入法或打开模态面板时，不触发背景单键命令。涂抹标记跟随模型表面；清除选区不会删除身体记录。窗口关闭或通过 Dock 切换不会丢失选区、对话或未完成操作。</p><button className="bc-button" onClick={() => { setCanvasOverlay(null); setRegionListOpen(true); }}>按名称选择身体区域<List size={17} /></button></div>}
        </div>
      </DialogContent>
    </Dialog>
  </main>;
}

/** Canvas shortcuts reuse the business windows instead of a second panel system. */
const CANVAS_PANEL_TO_WINDOW: Partial<Record<CanvasPanel, BusinessWindowId>> = {
  state: "person",
  equipment: "person",
  diagnosis: "diagnosis",
  training: "training",
  progress: "person",
  trainer: "trainer",
};

export type { CanvasPanel };
