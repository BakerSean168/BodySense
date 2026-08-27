import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import {
  VanatomeViewer,
  useVanatomeController,
  type VanatomeViewerError,
} from "@vixotic/vanatome-react";
import {
  anatomyStructureId,
  type AnatomyDisplayMode,
  type AnatomyStructureId,
  type AnatomyViewerErrorState,
  type AnatomyWebGLState,
} from "../adapters/anatomyViewerPort";
import {
  VanatomeAdapter,
  loadPinnedVanatomeAtlas,
  normalizeVanatomeError,
  type LoadedVanatomeAtlas,
} from "../adapters/vanatomeAdapter";
import { AnatomyInspector, type AnatomyStructureSummary } from "./AnatomyInspector";
import { AnatomyLayerControls } from "./AnatomyLayerControls";
import { BodyExplorerLoadingState } from "./BodyExplorerLoadingState";
import { BodyViewControls } from "./BodyViewControls";

const REGION_MODE_LAYERS = ["muscular"] as const;
const WEBGL_RECOVERY_GRACE_MS = 1800;

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

export interface BodyExplorer3DProps {
  selectedAnatomyId: AnatomyStructureId | null;
  onSelectedAnatomyIdChange: (id: AnatomyStructureId | null) => void;
  mode: "region" | "anatomy";
  onModeChange: (mode: "region" | "anatomy") => void;
  selectedRegionLabel?: string | null;
  focusRequest?: { id: AnatomyStructureId; key: string | number } | null;
  onFatalError: (error: AnatomyViewerErrorState) => void;
}

export default function BodyExplorer3D({
  selectedAnatomyId,
  onSelectedAnatomyIdChange,
  mode,
  onModeChange,
  selectedRegionLabel,
  focusRequest,
  onFatalError,
}: BodyExplorer3DProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const recoveryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const controller = useVanatomeController(REGION_MODE_LAYERS);
  const [loaded, setLoaded] = useState<LoadedVanatomeAtlas | null>(null);
  const [atlasError, setAtlasError] = useState<AnatomyViewerErrorState | null>(null);
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [displayMode, setDisplayMode] = useState<AnatomyDisplayMode>("normal");
  const [loadProgress, setLoadProgress] = useState<number | null>(null);
  const [loadState, setLoadState] = useState<"idle" | "loading" | "ready" | "error">(
    "idle",
  );
  const [viewerError, setViewerError] = useState<AnatomyViewerErrorState | null>(null);
  const [webglState, setWebglState] = useState<AnatomyWebGLState>("unknown");
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    const abort = new AbortController();
    setLoadState("loading");
    setAtlasError(null);
    loadPinnedVanatomeAtlas({ signal: abort.signal })
      .then((result) => {
        setLoaded(result);
        setLoadState("loading");
      })
      .catch((reason: unknown) => {
        if (abort.signal.aborted) return;
        const normalized = normalizeVanatomeError(reason);
        setAtlasError(normalized);
        setLoadState("error");
        onFatalError(normalized);
      });
    return () => abort.abort();
  }, [onFatalError]);

  useResizeObserver(rootRef, setContainerWidth);

  useEffect(() => {
    if (selectedAnatomyId === controller.selectedId) return;
    if (selectedAnatomyId) controller.focus(selectedAnatomyId);
    else controller.clear();
  }, [controller, selectedAnatomyId]);

  const lastFocusRequest = useRef<string | number | null>(null);
  useEffect(() => {
    if (!focusRequest || lastFocusRequest.current === focusRequest.key) return;
    lastFocusRequest.current = focusRequest.key;
    controller.focus(focusRequest.id);
    onSelectedAnatomyIdChange(focusRequest.id);
  }, [controller, focusRequest, onSelectedAnatomyIdChange]);

  useEffect(() => {
    if (mode !== "region") return;
    setDisplayMode("normal");
    const alreadyRegionLayers =
      controller.visibleLayers.length === REGION_MODE_LAYERS.length &&
      REGION_MODE_LAYERS.every(
        (layer, index) => controller.visibleLayers[index] === layer,
      );
    if (!alreadyRegionLayers) controller.setVisibleLayers(REGION_MODE_LAYERS);
    if (controller.isolation) controller.isolate(null);
  }, [
    controller.isolate,
    controller.isolation,
    controller.setVisibleLayers,
    controller.visibleLayers,
    mode,
  ]);

  const adapter = useMemo(
    () =>
      new VanatomeAdapter({
        getSelectedId: () => controller.selectedId,
        getHoveredId: () => hoveredId,
        getIsolation: () => controller.isolation,
        getVisibleLayers: () => controller.visibleLayers,
        getDisplayMode: () => displayMode,
        getLoadState: () => loadState,
        getLoadProgress: () => loadProgress,
        getError: () => viewerError ?? atlasError,
        getWebGLState: () => webglState,
        select: (id) => controller.select(id),
        hover: (id) => setHoveredId(id),
        focus: (id) => controller.focus(id),
        isolate: (id, isolationMode) => controller.isolate(id, isolationMode),
        reset: () => controller.reset(),
        setVisibleLayers: (layers) => controller.setVisibleLayers(layers),
        setDisplayMode: (nextMode) => setDisplayMode(nextMode),
      }),
    [
      atlasError,
      controller,
      displayMode,
      hoveredId,
      loadProgress,
      loadState,
      viewerError,
      webglState,
    ],
  );

  const handleSelection = useCallback(
    (id: string | null) => {
      // Vanatome reports an empty-canvas click as null. BodySense keeps the
      // current selection stable until an explicit reset/back action.
      if (!id) return;
      const normalized = anatomyStructureId(id);
      adapter.select(normalized);
      onSelectedAnatomyIdChange(normalized);
    },
    [adapter, onSelectedAnatomyIdChange],
  );

  const handleHover = useCallback(
    (id: string | null) => adapter.hover(id ? anatomyStructureId(id) : null),
    [adapter],
  );

  const handleViewerError = useCallback(
    (error: VanatomeViewerError) => {
      const normalized = normalizeVanatomeError(error);
      setViewerError(normalized);
      if (normalized.kind === "webgl") {
        setWebglState("lost");
        if (recoveryTimer.current) clearTimeout(recoveryTimer.current);
        recoveryTimer.current = setTimeout(() => {
          onFatalError(normalized);
        }, WEBGL_RECOVERY_GRACE_MS);
        return;
      }
      setLoadState("error");
      onFatalError(normalized);
    },
    [onFatalError],
  );

  const handleContextRestore = useCallback(() => {
    if (recoveryTimer.current) {
      clearTimeout(recoveryTimer.current);
      recoveryTimer.current = null;
    }
    setViewerError(null);
    setWebglState("ready");
    setLoadState("ready");
  }, []);

  useCanvasContextRestore(rootRef, handleContextRestore);

  useEffect(
    () => () => {
      if (recoveryTimer.current) clearTimeout(recoveryTimer.current);
    },
    [],
  );

  const structureIndex = useMemo(() => {
    const map = new Map<string, AnatomyStructureSummary>();
    for (const structure of loaded?.atlas.structures ?? []) {
      map.set(structure.id, {
        id: anatomyStructureId(structure.id),
        name: structure.name,
        system: structure.system,
        parentId: structure.parentId
          ? anatomyStructureId(structure.parentId)
          : undefined,
      });
    }
    return map;
  }, [loaded]);

  const selectedStructure = selectedAnatomyId
    ? (structureIndex.get(selectedAnatomyId) ?? null)
    : null;
  const hoveredStructure = hoveredId ? structureIndex.get(hoveredId) : null;
  const breadcrumb = useMemo(
    () => buildBreadcrumb(selectedStructure, structureIndex),
    [selectedStructure, structureIndex],
  );

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
    <div ref={rootRef} className="relative flex min-h-[510px] flex-col">
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="relative min-h-[430px] flex-1 overflow-hidden rounded-[18px] bg-black/5">
          <VanatomeViewer
            atlas={loaded.atlas}
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
            onLoadStart={() => {
              setLoadState("loading");
              setViewerError(null);
            }}
            onLoadProgress={({ percentage }) => setLoadProgress(percentage)}
            onReady={() => {
              setLoadState("ready");
              setLoadProgress(100);
              setViewerError(null);
              setWebglState("ready");
            }}
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
              onSelectSystem={(id) => adapter.setVisibleSystems([id])}
              onDisplayModeChange={(nextMode) => adapter.setDisplayMode(nextMode)}
            />
          ) : null}

          {canDrillDown ? (
            <AnatomyInspector
              mode={mode}
              selected={selectedStructure}
              breadcrumb={breadcrumb}
              regionLabel={selectedRegionLabel}
              onEnterAnatomy={() => onModeChange("anatomy")}
              onReturnToRegion={() => onModeChange("region")}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function buildBreadcrumb(
  selected: AnatomyStructureSummary | null,
  index: ReadonlyMap<string, AnatomyStructureSummary>,
): AnatomyStructureSummary[] {
  if (!selected) return [];
  const path: AnatomyStructureSummary[] = [];
  const seen = new Set<string>();
  let current: AnatomyStructureSummary | undefined = selected;
  while (current && !seen.has(current.id)) {
    seen.add(current.id);
    path.unshift(current);
    current = current.parentId ? index.get(current.parentId) : undefined;
  }
  return path;
}

function useResizeObserver(
  ref: RefObject<HTMLElement | null>,
  onWidth: (width: number) => void,
) {
  useEffect(() => {
    const element = ref.current;
    if (!element || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (typeof width === "number") onWidth(width);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [onWidth, ref]);
}

function useCanvasContextRestore(
  ref: RefObject<HTMLElement | null>,
  onRestore: () => void,
) {
  useEffect(() => {
    const root = ref.current;
    if (!root || typeof MutationObserver === "undefined") return;
    let canvas: HTMLCanvasElement | null = null;

    const detach = () => {
      canvas?.removeEventListener("webglcontextrestored", onRestore);
      canvas = null;
    };
    const attach = () => {
      const next = root.querySelector("canvas");
      if (next === canvas) return;
      detach();
      canvas = next;
      canvas?.addEventListener("webglcontextrestored", onRestore);
    };

    attach();
    const observer = new MutationObserver(attach);
    observer.observe(root, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      detach();
    };
  }, [onRestore, ref]);
}
