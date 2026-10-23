import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import {
  ArrowRight,
  CircleHelp,
  LoaderCircle,
  PersonStanding,
  ShieldAlert,
} from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  buildCanvasContext,
  canvasShortcut,
  DEFAULT_BODY_LAYERS,
  getReferenceMotion,
  SHOW_POSE_BAR,
  type BodyCanvasLayers,
  type CanvasPanel,
  type CanvasTool,
} from "../model/bodyCanvas";
import type { BusinessWindowId } from "../model/businessWindows";
import { BodyCanvasDock } from "./BodyCanvasDock";
import type { PersonProfileIdentity } from "../windows/person/types";
import { CanvasToolControls } from "../workspace/CanvasToolControls";
import { CanvasReferenceRibbon } from "../workspace/CanvasReferenceRibbon";
import { CanvasRegionPicker } from "../workspace/CanvasRegionPicker";
import { CanvasRegionDetails } from "../workspace/CanvasRegionDetails";
import { CanvasPlaybackControls } from "../workspace/CanvasPlaybackControls";
import { CanvasWorkspaceDialogs } from "../workspace/CanvasWorkspaceDialogs";
import { CanvasBusinessWindows } from "../workspace/CanvasBusinessWindows";
import type { BodyCanvasWorkspaceProps } from "../workspace/BodyCanvasWorkspace.types";
import { useCanvasAppearance } from "../workspace/useCanvasAppearance";
import { useCanvasBusinessWindowController } from "../workspace/useCanvasBusinessWindowController";
import { useCanvasBodySelection } from "../workspace/useCanvasBodySelection";
import { useCanvasStageLayout } from "../workspace/useCanvasStageLayout";
import { useCanvasMotionController } from "../workspace/useCanvasMotionController";
import type { SkeletonLayerStatus } from "./BodyCanvasScene";
import "../body-canvas.css";
import { IconTip } from "@/components/ui/tooltip";

const BodyCanvasScene = lazy(() => import("./BodyCanvasScene"));
// Penpot Home / trainer-active|paused; lazy for the same chunk budget.
const TrainerStatusChip = lazy(() =>
  import("@/features/training/components/TrainerStatusChip").then((module) => ({
    default: module.TrainerStatusChip,
  })),
);

export type { BodyCanvasWorkspaceProps } from "../workspace/BodyCanvasWorkspace.types";

export function BodyCanvasWorkspace(props: BodyCanvasWorkspaceProps) {
  const reducedMotion = useMediaQuery("(prefers-reduced-motion: reduce)");
  const mobile = useMediaQuery("(max-width: 760px)");
  const { themePreference, setThemePreference, dark } = useCanvasAppearance();
  const [tool, setTool] = useState<CanvasTool>("orbit");
  const [canvasOverlay, setCanvasOverlay] = useState<"help" | null>(null);
  // 精细解剖 (L) acts on the Home body: a layer panel beside the tool rail.
  const [anatomyMode, setAnatomyMode] = useState(false);
  const [bodyLayers, setBodyLayers] =
    useState<BodyCanvasLayers>(DEFAULT_BODY_LAYERS);
  const [skeletonStatus, setSkeletonStatus] =
    useState<SkeletonLayerStatus>("idle");
  const toggleAnatomy = useCallback(() => {
    setAnatomyMode((open) => !open);
    // Leaving (or entering) anatomy mode always starts from the default view.
    setBodyLayers(DEFAULT_BODY_LAYERS);
  }, []);
  const [regionListOpen, setRegionListOpen] = useState(false);
  const [textOnly, setTextOnly] = useState(false);
  const [regionCardOpen, setRegionCardOpen] = useState(true);
  const safetyReview =
    props.workspace?.capabilities.requires_safety_review === true;
  const {
    motion,
    motionMode,
    setMotionMode,
    playing,
    setPlaying,
    speed,
    setSpeed,
    phase,
    setPhase,
    seek,
    setSeek,
    resetKey,
    setResetKey,
    annotationEpoch,
    setAnnotationEpoch,
    choosePose,
  } = useCanvasMotionController({
    safetyReview,
    reducedMotion,
    setTool,
    setRegionCardOpen,
    setCanvasOverlay,
    demonstrationRequest: props.demonstrationRequest,
  });
  const sceneRef = useRef<HTMLDivElement>(null);
  const { stageRef, safetyBannerHeight, dockReserve, measureSafetyBanner } =
    useCanvasStageLayout();
  const business = useCanvasBusinessWindowController({
    props,
    safetyReview,
    setRegionListOpen,
    setCanvasOverlay,
  });
  const {
    accountDialog,
    setAccountDialog,
    activeWindow,
    personInfoOpen,
    setPersonInfoOpen,
    diagnosisView,
    openWindow,
    closeWindow,
    toggleWindow,
    reopenTrainer,
  } = business;
  const {
    selection,
    primary,
    regionFacts,
    selectionLabel,
    select,
    clear,
    undo,
    removeRegion,
  } = useCanvasBodySelection({
    bodyState: props.bodyState,
    bodyExplorer: props.bodyExplorer,
    mobile,
    setPlaying,
    setTool,
    setAnnotationEpoch,
    setRegionCardOpen,
  });
  const definition = getReferenceMotion(motion);
  // Playback is a background behaviour: any open window pauses it.
  const effectivePlaying = playing && !safetyReview && !activeWindow;
  const ask = useCallback(() => {
    setPlaying(false);
    props.onAskContext(
      buildCanvasContext(
        selection.regions,
        motionMode ? motion : null,
        phase,
        effectivePlaying,
      ),
    );
    openWindow("chat");
  }, [
    selection.regions,
    motion,
    motionMode,
    phase,
    effectivePlaying,
    props.onAskContext,
    openWindow,
  ]);
  const onTextFallback = useCallback(() => {
    setTextOnly(true);
    setRegionListOpen(true);
  }, []);

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      if (event.key === "Escape" && !event.defaultPrevented) {
        // The actual modal/menu owns its Escape; a non-modal mobile business
        // sheet does not. Do not dismiss a native select's open options.
        if (target?.closest("[role='menu'], [aria-modal='true'], select"))
          return;
        // Close the transient Person overlay before hiding the whole window.
        if (personInfoOpen) setPersonInfoOpen(false);
        else if (activeWindow) closeWindow(activeWindow);
        else if (canvasOverlay) setCanvasOverlay(null);
        else if (anatomyMode) toggleAnatomy();
        else if (tool !== "orbit") setTool("orbit");
        else if (regionListOpen) setRegionListOpen(false);
        else clear();
        event.preventDefault();
        return;
      }
      // ⌘K / Ctrl+K opens the region palette anywhere; Base UI owns other
      // keys in menus/dialogs and form fields.
      const action = canvasShortcut(event);
      if (action?.type === "palette") {
        setCanvasOverlay(null);
        setRegionListOpen(true);
        event.preventDefault();
        return;
      }
      if (target?.closest("[role='dialog'], [role='menu']")) return;
      if (!action) return;
      // Pose bar hidden: 1-4 and F / Space do nothing, the body stays standing.
      if (
        !SHOW_POSE_BAR &&
        (action.type === "motion" || action.type === "pause")
      )
        return;
      if (action.type === "panel") {
        if (action.panel === "anatomy") {
          setCanvasOverlay(null);
          toggleAnatomy();
        } else if (action.panel === "help") {
          setCanvasOverlay((current) => (current === "help" ? null : "help"));
        } else if (CANVAS_PANEL_TO_WINDOW[action.panel]) {
          toggleWindow(CANVAS_PANEL_TO_WINDOW[action.panel]!);
        } else {
          setCanvasOverlay("help");
        }
      } else if (action.type === "tool") {
        setTool(action.tool);
        if (action.tool !== "orbit") setPlaying(false);
      } else if (action.type === "motion") choosePose(action.motion);
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
    anatomyMode,
    ask,
    canvasOverlay,
    choosePose,
    clear,
    closeWindow,
    personInfoOpen,
    regionListOpen,
    safetyReview,
    tool,
    toggleAnatomy,
    toggleWindow,
    undo,
  ]);

  const showRegionCard =
    primary && regionCardOpen && !activeWindow && !regionListOpen;
  const sceneFallback = (
    <div className="bc-scene-status" role="status">
      <LoaderCircle className="animate-spin" size={28} />
      <p>正在准备身体视图</p>
      <button className="bc-button" onClick={onTextFallback}>
        按名称选择区域
      </button>
    </div>
  );
  const themeClass = `body-canvas-theme${dark ? " bc-dark" : ""}`;
  const identity: PersonProfileIdentity = props.identity ?? { nickname: "" };

  return (
    <main
      className={`${themeClass} bc-workspace`}
      data-testid="body-canvas-workspace"
    >
      {/* Penpot Home has no app bar: the account menu, Settings and Logout
          live in the Person window header. The page heading stays for AT. */}
      <h1 className="sr-only">我的身体</h1>

      <section
        className={`bc-stage${activeWindow || showRegionCard ? " bc-stage-context" : ""}${motionMode ? " bc-stage-motion" : ""}${anatomyMode ? " bc-stage-anatomy" : ""}`}
        aria-label="身体画布"
        ref={stageRef}
        style={
          safetyBannerHeight || dockReserve
            ? ({
                ...(safetyBannerHeight
                  ? { "--bc-safety-banner-height": `${safetyBannerHeight}px` }
                  : {}),
                ...(dockReserve
                  ? { "--bc-dock-reserve": `${dockReserve}px` }
                  : {}),
              } as CSSProperties)
            : undefined
        }
      >
        <div className="bc-render-area" ref={sceneRef}>
          {!textOnly ? (
            <Suspense fallback={sceneFallback}>
              <BodyCanvasScene
                tool={tool}
                selectedRegions={selection.regions}
                motion={motion}
                layers={anatomyMode ? bodyLayers : DEFAULT_BODY_LAYERS}
                onSkeletonStatus={setSkeletonStatus}
                playing={effectivePlaying}
                speed={speed}
                seek={seek}
                resetKey={resetKey}
                annotationEpoch={annotationEpoch}
                dark={dark}
                onSelect={select}
                onGestureStart={() => setPlaying(false)}
                onPhaseChange={setPhase}
                onTextFallback={onTextFallback}
              />
            </Suspense>
          ) : (
            <div className="bc-text-scene">
              <PersonStanding size={100} strokeWidth={0.7} />
              <h2>用区域名称，一样能开始。</h2>
              <button className="bc-button" onClick={() => setTextOnly(false)}>
                恢复 3D 视图
              </button>
            </div>
          )}
        </div>

        {/* Penpot mint backdrop disc behind the reference body. */}
        <div className="bc-backdrop-disc" aria-hidden="true" />
        <CanvasReferenceRibbon
          motion={motion}
          motionMode={motionMode}
          onChoosePose={choosePose}
        />
        {safetyReview && (
          <div
            ref={measureSafetyBanner}
            role="alert"
            className="bc-safety-banner"
            data-testid="safety-banner"
          >
            <ShieldAlert size={19} />
            <span>需要先处理安全提醒，动作播放已暂停。</span>
            <button onClick={() => openWindow("person")}>
              查看记录
              <ArrowRight size={15} />
            </button>
          </div>
        )}
        {props.error && (
          <div role="alert" className="bc-sync-banner">
            身体信息暂时无法同步。
            {props.onRetry && <button onClick={props.onRetry}>重试</button>}
            现有对话与参考模型仍可使用。
          </div>
        )}
        {props.loading && (
          <span className="bc-sync-indicator" role="status">
            <LoaderCircle size={14} className="animate-spin" />
            正在同步身体记录
          </span>
        )}

        {props.workspace && !activeWindow ? (
          <Suspense fallback={null}>
            <TrainerStatusChip onOpen={reopenTrainer} />
          </Suspense>
        ) : null}

        <CanvasToolControls
          tool={tool}
          onSelectTool={(value) => {
            setTool(value);
            setRegionListOpen(false);
            if (value !== "orbit") setPlaying(false);
          }}
          onExitTool={() => setTool("orbit")}
          onReset={() => setResetKey((value) => value + 1)}
          onOpenRegionPalette={() => {
            setCanvasOverlay(null);
            setRegionListOpen(true);
          }}
          anatomyMode={anatomyMode}
          onToggleAnatomy={toggleAnatomy}
          bodyLayers={bodyLayers}
          skeletonStatus={skeletonStatus}
          onToggleLayer={(id) =>
            setBodyLayers((current) => ({ ...current, [id]: !current[id] }))
          }
        />
        <CanvasRegionPicker
          visible={regionListOpen}
          selection={selection}
          themeClass={themeClass}
          onSelect={select}
          onVisibleChange={setRegionListOpen}
          onCardOpenChange={setRegionCardOpen}
        />
        <CanvasRegionDetails
          primary={primary}
          selection={selection}
          regionFacts={regionFacts}
          hasBodyState={Boolean(props.bodyState)}
          loading={props.loading}
          error={props.error}
          regionCardOpen={regionCardOpen}
          regionListOpen={regionListOpen}
          activeWindow={Boolean(activeWindow)}
          motionMode={motionMode}
          definition={definition}
          phase={phase}
          selectionLabel={selectionLabel}
          onCardOpenChange={setRegionCardOpen}
          onRemoveRegion={removeRegion}
          onUndo={undo}
          onClear={clear}
          onAsk={ask}
          onOpenPerson={() => openWindow("person")}
        />
        <CanvasPlaybackControls
          motionMode={motionMode}
          definition={definition}
          phase={phase}
          speed={speed}
          effectivePlaying={effectivePlaying}
          safetyReview={safetyReview}
          onTogglePlaying={() => setPlaying((value) => !value)}
          onSpeedChange={setSpeed}
          onSeek={(next) => {
            setPlaying(false);
            setPhase(next);
            setSeek((value) => ({ phase: next, revision: value.revision + 1 }));
          }}
          onAsk={ask}
        />
        <CanvasBusinessWindows
          props={props}
          business={business}
          mobile={mobile}
          identity={identity}
          safetyReview={safetyReview}
        />
        <IconTip label="快捷键" shortcut="?">
          <button
            className="bc-help-entry"
            aria-label="快捷键"
            onClick={() => setCanvasOverlay("help")}
          >
            <CircleHelp size={20} strokeWidth={1.8} aria-hidden="true" />
          </button>
        </IconTip>
        <BodyCanvasDock
          activeWindow={activeWindow}
          confirmedDiagnosisCount={diagnosisView.confirmed.length}
          onToggle={toggleWindow}
        />
      </section>

      <CanvasWorkspaceDialogs
        canvasOverlay={canvasOverlay}
        onCanvasOverlayChange={setCanvasOverlay}
        accountDialog={accountDialog}
        onAccountDialogChange={setAccountDialog}
        themePreference={themePreference}
        onThemePreferenceChange={setThemePreference}
        themeClass={themeClass}
      />
    </main>
  );
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
