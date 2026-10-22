import {
  Suspense,
  useEffect,
  useCallback,
  useMemo,
  useRef,
  useState,
} from "react";
import { VanatomeViewer } from "@vixotic/vanatome-react";
import type {
  AnatomyViewerErrorState,
  AnatomyWebGLState,
} from "../adapters/anatomyViewerPort";
import { AnatomyInspector } from "./AnatomyInspector";
import { AnatomyLayerControls } from "./AnatomyLayerControls";
import { BodyExplorerLoadingState } from "./BodyExplorerLoadingState";
import { BodyViewControls } from "./BodyViewControls";
import {
  reportClientDiagnostic,
  type ClientDiagnosticAttributeValue,
} from "@/lib/clientDiagnostics";
import { diagnosticNow } from "../adapters/viewerDiagnostics";
import { useVanatomeAtlas } from "../hooks/useVanatomeAtlas";
import { useBodyExplorerInteraction } from "../hooks/useBodyExplorerInteraction";
import { useBodyExplorerViewerLifecycle } from "../hooks/useBodyExplorerViewerLifecycle";
import { useResizeObserver } from "../hooks/useViewerCanvas";
import {
  atlasHasBodyShell,
  TranslucentShellPrep,
  ShellPrepBoundary,
} from "./TranslucentShellPrep";
import type { BodyExplorer3DProps } from "./bodyExplorer3D.types";
export type { BodyExplorer3DProps } from "./bodyExplorer3D.types";

// Measured from the pinned atlas 1.4.0 GLB: full body bounds are roughly
// 0.68 x 1.74 x 0.29 with a vertical center at y ~= 0.87. Keeping these
// viewer-only framing constants next to the adapter avoids encoding any
// BodyRegion semantics in the camera layer.
// Vanatome 0.1.6 does not apply initialCameraTarget to OrbitControls on
// first mount (it only uses it when resetViewKey changes). Centering the
// measured atlas geometry at the viewer origin therefore gives both the
// initial mount and reset path the same deterministic full-body framing.
const FULL_BODY_MODEL_POSITION = [0, -0.8714788446903425, 0] as const;
const FULL_BODY_CAMERA_TARGET = [0, 0, 0] as const;
const FULL_BODY_CAMERA_POSITION = [0, 0, 3.05] as const;

export default function BodyExplorer3D({
  selectedAnatomyId,
  onSelectedAnatomyIdChange,
  mode,
  onModeChange,
  onRegionModeRequested,
  selectedRegionLabel,
  focusRequest,
  resetRequestKey,
  onAskContext,
  onFatalError,
  diagnosticSessionId,
  attemptId,
}: BodyExplorer3DProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const mountedAtRef = useRef(diagnosticNow());
  const [loadProgress, setLoadProgress] = useState<number | null>(null);
  const [loadState, setLoadState] = useState<
    "idle" | "loading" | "ready" | "error"
  >("idle");
  const [viewerError, setViewerError] =
    useState<AnatomyViewerErrorState | null>(null);
  const [webglState, setWebglState] = useState<AnatomyWebGLState>("unknown");
  const [containerWidth, setContainerWidth] = useState(0);

  const reportViewerDiagnostic = useCallback(
    (input: {
      event: string;
      severity?: "info" | "warn" | "error";
      code?: string;
      message?: string;
      phase: string;
      resource?: string | null;
      attributes?: Record<string, ClientDiagnosticAttributeValue>;
    }) => {
      reportClientDiagnostic({
        category: "body3d.viewer",
        ...input,
        diagnosticSessionId,
        attemptId,
        elapsedMs: diagnosticNow() - mountedAtRef.current,
      });
    },
    [attemptId, diagnosticSessionId],
  );

  const {
    failedSystemId,
    loaded,
    loadedAtlases,
    loadedSystems,
    atlasError,
    ensureSystem,
    loadedStructureIds,
  } = useVanatomeAtlas({
    onFatalError,
    reportViewerDiagnostic,
    setLoadState,
    setViewerError,
  });
  const {
    controller,
    hoveredId,
    displayMode,
    adapter,
    handleSelection,
    handleHover,
    selectedStructure,
    hoveredStructure,
    breadcrumb,
  } = useBodyExplorerInteraction({
    props: {
      selectedAnatomyId,
      onSelectedAnatomyIdChange,
      mode,
      onModeChange,
      onRegionModeRequested,
      selectedRegionLabel,
      focusRequest,
      resetRequestKey,
      onAskContext,
      onFatalError,
      diagnosticSessionId,
      attemptId,
    },
    loadedAtlases,
    loadedStructureIds,
    ensureSystem,
    loadState,
    loadProgress,
    viewerError,
    atlasError,
    webglState,
  });
  const {
    onLoadStart,
    onLoadProgress,
    onModelReady,
    onReady,
    handleViewerError,
  } = useBodyExplorerViewerLifecycle({
    rootRef,
    loaded,
    loadedAtlasCount: loadedAtlases.length,
    onFatalError,
    reportViewerDiagnostic,
    setLoadState,
    setViewerError,
    setWebglState,
    setLoadProgress,
  });
  useResizeObserver(rootRef, setContainerWidth, Boolean(loaded));

  const [preparedShells, setPreparedShells] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const markShellPrepared = useCallback((url: string) => {
    setPreparedShells((current) =>
      current.has(url) ? current : new Set(current).add(url),
    );
  }, []);
  const viewerAtlases = useMemo(
    () =>
      loadedAtlases.filter(
        (atlas) =>
          !atlasHasBodyShell(atlas) || preparedShells.has(atlas.modelUrl),
      ),
    [loadedAtlases, preparedShells],
  );
  const pendingShellUrls = loadedAtlases
    .filter(
      (atlas) =>
        atlasHasBodyShell(atlas) && !preparedShells.has(atlas.modelUrl),
    )
    .map((atlas) => atlas.modelUrl);

  // Region mode shows the muscular layer by default; fetch it once the shell is up.
  useEffect(() => {
    if (mode !== "region" || !loaded || !viewerAtlases.length) return;
    void ensureSystem("muscular");
  }, [ensureSystem, loaded, mode, viewerAtlases.length]);

  if (!loaded && !atlasError) {
    return <BodyExplorerLoadingState label="正在准备 3D 身体视图" />;
  }

  if (!loaded) return null;

  const snapshot = adapter.getSnapshot();
  const compact = containerWidth > 0 && containerWidth < 390;
  const viewerHeight = Math.round(
    Math.min(620, Math.max(430, (containerWidth || 360) * 1.2)),
  );
  const canDrillDown = Boolean(selectedStructure || selectedRegionLabel);

  return (
    <div
      ref={rootRef}
      className="relative flex min-h-[510px] flex-col"
      data-testid="body-explorer-3d"
      data-viewer-state={loadState}
      data-loaded-systems={[...loadedSystems].toSorted().join(",")}
    >
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="relative min-h-[430px] flex-1 overflow-hidden rounded-[18px] bg-black/5">
          {pendingShellUrls.map((url) => (
            <ShellPrepBoundary key={url} url={url} onReady={markShellPrepared}>
              <Suspense fallback={null}>
                <TranslucentShellPrep url={url} onReady={markShellPrepared} />
              </Suspense>
            </ShellPrepBoundary>
          ))}
          {viewerAtlases.length === 0 ? (
            <div className="grid" style={{ height: viewerHeight }}>
              <BodyExplorerLoadingState label="正在加载人体模型" />
            </div>
          ) : (
            <VanatomeViewer
              atlases={viewerAtlases}
              selectedId={controller.selectedId}
              hoveredId={hoveredId}
              isolation={controller.isolation}
              visibleLayers={controller.visibleLayers}
              alwaysVisibleIds={["body-shell"]}
              displayMode={displayMode}
              focusRequestKey={controller.focusRequestKey}
              resetViewKey={controller.resetViewKey}
              onSelect={handleSelection}
              onHover={handleHover}
              onEscape={() => {
                controller.clear();
                onSelectedAnatomyIdChange(null);
              }}
              onLoadStart={onLoadStart}
              onLoadProgress={onLoadProgress}
              onModelReady={onModelReady}
              onReady={onReady}
              onError={handleViewerError}
              loadingFallback={
                <BodyExplorerLoadingState
                  label="正在加载人体模型"
                  progress={loadProgress}
                />
              }
              errorFallback={
                <p className="px-5 text-center text-xs text-muted-foreground">
                  3D 身体视图正在恢复；身体记录仍然可用。
                </p>
              }
              modelPosition={[...FULL_BODY_MODEL_POSITION]}
              initialCameraPosition={[...FULL_BODY_CAMERA_POSITION]}
              initialCameraTarget={[...FULL_BODY_CAMERA_TARGET]}
              focusDistance={0.65}
              focusPadding={1.35}
              cameraAnimationDuration={420}
              respectReducedMotion
              enablePan
              minDistance={0.35}
              maxDistance={10}
              appearance={{
                bodyShellId: null,
                skeletonId: null,
                defaultOpacity: 0.7,
                xrayOpacity: 0.24,
                ghostOpacity: 0.08,
                parentContextOpacity: 0.14,
                hoverEmissiveIntensity: 0.35,
                selectedDescendantEmissiveIntensity: 0.5,
                selectedEmissiveIntensity: 0.85,
                pulseSelection: false,
              }}
              ariaLabel="BodySense 交互式 3D 身体探索视图"
              className="w-full"
              style={{ height: viewerHeight }}
            />
          )}

          <div className="pointer-events-none absolute inset-x-0 top-0 flex items-start justify-between gap-2 p-2.5">
            <div className="pointer-events-auto rounded-lg bg-background/72 px-2 py-1 text-[11px] text-muted-foreground backdrop-blur-sm">
              拖动旋转 · 滚轮/双指缩放 · 右键拖动平移
            </div>
            {hoveredStructure ? (
              <div className="pointer-events-none max-w-[52%] truncate rounded-lg bg-background/82 px-2 py-1 text-[11px] font-medium text-foreground backdrop-blur-sm">
                {hoveredStructure.name}
              </div>
            ) : null}
          </div>

          {failedSystemId ? (
            <div
              role="alert"
              className="absolute inset-x-3 bottom-3 rounded-lg bg-background/92 px-3 py-2 text-xs"
            >
              部分解剖图层暂时无法读取，当前模型仍可使用。
              <button
                type="button"
                className="ml-2 underline"
                onClick={() => void ensureSystem(failedSystemId)}
              >
                重试图层
              </button>
            </div>
          ) : null}
          {snapshot.webglState === "lost" ? (
            <div
              role="status"
              aria-live="polite"
              className="absolute inset-x-3 bottom-3 rounded-lg border border-warning/30 bg-background/92 px-3 py-2 text-xs text-warning-foreground backdrop-blur-sm"
            >
              3D 图形上下文正在恢复；若恢复失败会自动切换到 2D 概览。
            </div>
          ) : null}
        </div>

        <div className={`mt-2.5 space-y-2.5 ${compact ? "px-0" : "px-1"}`}>
          <div className="flex flex-wrap items-start justify-between gap-2">
            <BodyViewControls
              hasSelection={Boolean(selectedAnatomyId)}
              isolated={Boolean(controller.isolation)}
              onFocus={() => {
                if (selectedAnatomyId) adapter.focus(selectedAnatomyId);
              }}
              onToggleIsolation={() => {
                if (!selectedAnatomyId) return;
                adapter.isolate(
                  controller.isolation ? null : selectedAnatomyId,
                  "parent-context",
                );
              }}
              onReset={() => {
                adapter.resetView();
                onSelectedAnatomyIdChange(null);
                onModeChange("region");
              }}
            />
            <span className="text-[10px] text-muted-foreground">
              Atlas 1.4.0
            </span>
          </div>

          {mode === "anatomy" ? (
            <AnatomyLayerControls
              systems={loaded.catalog.systems}
              visibleSystems={controller.visibleLayers}
              displayMode={displayMode}
              onSelectSystem={(id) => {
                adapter.setVisibleSystems([id]);
                void ensureSystem(id);
              }}
              onDisplayModeChange={(nextMode) =>
                adapter.setDisplayMode(nextMode)
              }
            />
          ) : null}

          {canDrillDown ? (
            <AnatomyInspector
              mode={mode}
              selected={selectedStructure}
              breadcrumb={breadcrumb}
              regionLabel={selectedRegionLabel}
              onEnterAnatomy={() => onModeChange("anatomy")}
              onReturnToRegion={() => {
                onModeChange("region");
                onRegionModeRequested?.();
              }}
              onAsk={
                onAskContext
                  ? () =>
                      onAskContext({
                        anatomyId: selectedStructure?.id ?? selectedAnatomyId,
                        anatomyName: selectedStructure?.name,
                        regionLabel: selectedRegionLabel,
                      })
                  : undefined
              }
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
