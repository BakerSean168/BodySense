import {
  useCallback,
  useEffect,
  useReducer,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import type { BodyCanvasWorkspaceProps } from "./BodyCanvasWorkspace.types";
import {
  activeBusinessWindow,
  businessWindowReducer,
  EMPTY_BUSINESS_WINDOWS,
  isBusinessWindowVisible,
  type BusinessWindowId,
} from "../model/businessWindows";
import { useDiagnosisActions } from "@/features/consultation/hooks/useDiagnosisActions";
import { useTrainingPlanActions } from "@/features/training/hooks/useTrainingPlanActions";
import {
  readSelectedPlanId,
  resolveSelectedPlanId,
  writeSelectedPlanId,
} from "../model/trainingPlanSelection";
import { useRouteWindowSync } from "../hooks/useRouteWindowSync";
import type { ProfileCategoryId } from "../model/profileCategories";
import type { TrainerLaunch } from "../components/TrainerEntryGuard";

/** Business window state is orthogonal to the 3D scene and body selection.
 * Crucially every window once opened remains mounted across Dock switches.
 * This controller only coordinates routes and window UI; the Diagnosis and
 * Training action hooks remain the authorities for API commands.
 */
export function useCanvasBusinessWindowController({
  props,
  safetyReview,
  setRegionListOpen,
  setCanvasOverlay,
}: {
  props: BodyCanvasWorkspaceProps;
  safetyReview: boolean;
  setRegionListOpen: Dispatch<SetStateAction<boolean>>;
  setCanvasOverlay: Dispatch<SetStateAction<"help" | null>>;
}) {
  const [accountDialog, setAccountDialog] = useState<
    "settings" | "logout" | null
  >(null);
  const [windows, dispatchWindow] = useReducer(
    businessWindowReducer,
    EMPTY_BUSINESS_WINDOWS,
  );
  // A window that has been opened stays mounted (hidden when closed/minimized) so
  // closing, minimizing or switching never discards the conversation, a pending
  // interaction, scroll position or unsaved composer input.
  const [mountedWindows, setMountedWindows] = useState<
    readonly BusinessWindowId[]
  >([]);
  // Declarative mount tracking: any window that the reducer has ever opened stays
  // mounted for the rest of the session, so no open path (Dock toggle, shortcut,
  // deep link or programmatic open) can accidentally unmount a live task. Closing
  // a window hides it; it does not destroy the conversation or a pending form.
  useEffect(() => {
    setMountedWindows((current) => {
      const missing = windows.order.filter(
        (window) => !current.includes(window),
      );
      return missing.length === 0 ? current : [...current, ...missing];
    });
  }, [windows.order]);
  // The Person identity popover is anchored in the shared header, so its open
  // state lives with the header owner rather than inside the body.
  const [personInfoOpen, setPersonInfoOpen] = useState(false);
  // Penpot Person header picker ("BakerSean / 身体记录") owns the category.
  const [personCategory, setPersonCategory] =
    useState<ProfileCategoryId>("records");
  const [selectedPlanId, setSelectedPlanId] = useState<string | null>(() =>
    readSelectedPlanId(),
  );
  const [selectedConditionId, setSelectedConditionId] = useState<string | null>(
    null,
  );
  /** Training window body: the plan, or Penpot Safety / Restricted. */
  const [trainingView, setTrainingView] = useState<"plan" | "safety">("plan");
  /** Chat shows the formal-analysis request entry (explicit user intent only). */
  const [analysisRequest, setAnalysisRequest] = useState(false);
  // Chat header day: 今天 until the user jumps; a new message brings it back.
  const chatHistory = props.chatHistory;
  const [chatJump, setChatJump] = useState<{
    day: string;
    latest: string | undefined;
  } | null>(null);
  const latestChatMessage = chatHistory?.days[0]?.firstMessageId;
  const chatDay =
    chatJump && chatJump.latest === latestChatMessage ? chatJump.day : null;
  const jumpToChatDay = useCallback(
    (day: string) => {
      setChatJump({ day, latest: latestChatMessage });
      const viewport = document.querySelector<HTMLElement>(
        ".bc-chat-content .chat-thread-viewport",
      );
      if (!viewport) return;
      const entry = chatHistory?.days.find((item) => item.day === day);
      const target = entry
        ? viewport.querySelector<HTMLElement>(
            `[data-message-id="${entry.firstMessageId}"]`,
          )
        : null;
      if (target) target.scrollIntoView({ block: "start", behavior: "smooth" });
      else if (day === chatHistory?.todayKey)
        viewport.scrollTo({ top: viewport.scrollHeight, behavior: "smooth" });
    },
    [chatHistory, latestChatMessage],
  );
  /** Page-local explicit Start intent. A deep link or shortcut never supplies it. */
  const [trainerLaunch, setTrainerLaunch] = useState<TrainerLaunch | null>(
    null,
  );
  const activeWindow = activeBusinessWindow(windows);
  const trainingOpen =
    isBusinessWindowVisible(windows, "training") ||
    isBusinessWindowVisible(windows, "trainer");
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(
    null,
  );
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

  // Mirror the active window into the route and open a deep-linked window
  // once, without letting the route's echo reopen a window the user closed.
  useRouteWindowSync({
    requestedWindow: props.initialWindow ?? null,
    activeWindow,
    openWindow,
    onWindowChange: props.onWindowChange,
  });

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

  return {
    accountDialog,
    setAccountDialog,
    windows,
    mountedWindows,
    activeWindow,
    personInfoOpen,
    setPersonInfoOpen,
    personCategory,
    setPersonCategory,
    selectedConditionId,
    setSelectedConditionId,
    trainingView,
    setTrainingView,
    analysisRequest,
    setAnalysisRequest,
    chatHistory,
    chatDay,
    jumpToChatDay,
    trainerLaunch,
    setTrainerLaunch,
    setSelectedTemplateId,
    chooseTrainingPlan,
    trainingActions,
    trainingSelection,
    diagnosisActions,
    diagnosisView,
    openWindow,
    closeWindow,
    requestAnalysis,
    openSafety,
    toggleWindow,
    reopenTrainer,
  };
}
