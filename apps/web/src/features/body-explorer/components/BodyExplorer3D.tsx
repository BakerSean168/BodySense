import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import type { VanatomeViewerAtlas } from "@vixotic/vanatome-atlas";
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
import {
  AnatomyInspector,
  type AnatomyStructureSummary,
} from "./AnatomyInspector";
import { getPinnedAtlasStructure } from "../model/atlasRegistry";
import { AnatomyLayerControls } from "./AnatomyLayerControls";
import { BodyExplorerLoadingState } from "./BodyExplorerLoadingState";
import { BodyViewControls } from "./BodyViewControls";
import {
  reportClientDiagnostic,
  type ClientDiagnosticAttributeValue,
} from "@/lib/clientDiagnostics";

const REGION_MODE_LAYERS = ["muscular"] as const;
const WEBGL_RECOVERY_GRACE_MS = 1800;
const MODEL_READY_SLOW_MS = 8_000;
const MODEL_READY_STALLED_MS = 20_000;

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
  resetRequestKey?: string | number;
  onAskContext?: (context: {
    anatomyId?: string | null;
    anatomyName?: string | null;
    regionLabel?: string | null;
  }) => void;
  onFatalError: (error: AnatomyViewerErrorState) => void;
  diagnosticSessionId: string;
  attemptId: string;
}

export default function BodyExplorer3D({
  selectedAnatomyId,
  onSelectedAnatomyIdChange,
  mode,
  onModeChange,
  selectedRegionLabel,
  focusRequest,
  resetRequestKey,
  onAskContext,
  onFatalError,
  diagnosticSessionId,
  attemptId,
}: BodyExplorer3DProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const recoveryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedAtRef = useRef(diagnosticNow());
  const lastModelUrlRef = useRef<string | null>(null);
  const modelStartedAtRef = useRef(new Map<string, number>());
  const modelWatchdogsRef = useRef(
    new Map<
      string,
      {
        slow: ReturnType<typeof setTimeout>;
        stalled: ReturnType<typeof setTimeout>;
      }
    >(),
  );
  const loaderCompleteReportedRef = useRef(new Set<string>());
  const networkCompleteReportedRef = useRef(new Set<string>());
  const loadProgressRef = useRef<number | null>(null);
  const longTaskMetricsRef = useRef<LongTaskMetrics>({
    count: 0,
    totalMs: 0,
    maxMs: 0,
  });
  const controller = useVanatomeController(REGION_MODE_LAYERS);
  const [loaded, setLoaded] = useState<LoadedVanatomeAtlas | null>(null);
  const [loadedAtlases, setLoadedAtlases] = useState<
    readonly VanatomeViewerAtlas[]
  >([]);
  const [loadedSystems, setLoadedSystems] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const loadingSystemsRef = useRef(new Map<string, Promise<void>>());
  const [atlasError, setAtlasError] = useState<AnatomyViewerErrorState | null>(
    null,
  );
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [displayMode, setDisplayMode] = useState<AnatomyDisplayMode>("normal");
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

  useEffect(() => {
    if (
      typeof PerformanceObserver === "undefined" ||
      !PerformanceObserver.supportedEntryTypes?.includes("longtask")
    ) {
      return;
    }
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        longTaskMetricsRef.current.count += 1;
        longTaskMetricsRef.current.totalMs += entry.duration;
        longTaskMetricsRef.current.maxMs = Math.max(
          longTaskMetricsRef.current.maxMs,
          entry.duration,
        );
      }
    });
    observer.observe({ type: "longtask", buffered: true });
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (
      typeof PerformanceObserver === "undefined" ||
      !PerformanceObserver.supportedEntryTypes?.includes("resource")
    ) {
      return;
    }
    const observer = new PerformanceObserver((list) => {
      for (const rawEntry of list.getEntries()) {
        if (!(rawEntry instanceof PerformanceResourceTiming)) continue;
        const entry = rawEntry;
        const modelUrl = [...modelStartedAtRef.current.keys()].find(
          (candidate) => absoluteResourceUrl(candidate) === entry.name,
        );
        if (!modelUrl || networkCompleteReportedRef.current.has(modelUrl)) {
          continue;
        }
        networkCompleteReportedRef.current.add(modelUrl);
        const modelStartedAt = modelStartedAtRef.current.get(modelUrl);
        reportViewerDiagnostic({
          event: "model_network_complete",
          severity: "info",
          phase: "model_network",
          resource: modelUrl,
          attributes: {
            "network.from_model_start_ms":
              modelStartedAt == null
                ? 0
                : roundMetric(entry.responseEnd - modelStartedAt),
            "network.observer_delay_ms": roundMetric(
              Math.max(0, diagnosticNow() - entry.responseEnd),
            ),
            ...resourceTimingEntryAttributes(entry),
          },
        });
      }
    });
    observer.observe({ type: "resource", buffered: true });
    return () => observer.disconnect();
  }, [reportViewerDiagnostic]);

  useEffect(() => {
    const abort = new AbortController();
    const startedAt = diagnosticNow();
    reportViewerDiagnostic({
      event: "atlas_metadata_started",
      severity: "info",
      phase: "atlas_metadata",
    });
    setLoadState("loading");
    setAtlasError(null);
    loadPinnedVanatomeAtlas({ signal: abort.signal })
      .then((result) => {
        reportViewerDiagnostic({
          event: "atlas_metadata_ready",
          severity: "info",
          phase: "atlas_metadata",
          attributes: {
            "atlas.duration_ms": roundMetric(diagnosticNow() - startedAt),
            "atlas.initial_model_count": result.atlases.length,
            "atlas.system_count": result.catalog.systems.length,
          },
        });
        setLoaded(result);
        setLoadedAtlases(result.atlases);
        setLoadedSystems(
          new Set(
            result.atlases.flatMap((atlas) =>
              atlas.structures.map((structure) => structure.system),
            ),
          ),
        );
        setLoadState("loading");
      })
      .catch((reason: unknown) => {
        if (abort.signal.aborted) return;
        const normalized = normalizeVanatomeError(reason);
        reportViewerDiagnostic({
          event: "atlas_load_failed",
          severity: "error",
          code: normalized.kind,
          message: normalized.message,
          phase: "atlas_metadata",
          attributes: {
            "atlas.duration_ms": roundMetric(diagnosticNow() - startedAt),
          },
        });
        setAtlasError(normalized);
        setLoadState("error");
        onFatalError(normalized);
      });
    return () => abort.abort();
  }, [onFatalError, reportViewerDiagnostic]);

  const ensureSystem = useCallback(
    async (systemId: string) => {
      if (!loaded || loadedSystems.has(systemId)) return;

      const active = loadingSystemsRef.current.get(systemId);
      if (active) return active;

      const systemStartedAt = diagnosticNow();
      reportViewerDiagnostic({
        event: "system_metadata_started",
        severity: "info",
        phase: "system_metadata",
        attributes: { "anatomy.system_id": systemId },
      });
      const pending = loaded
        .loadSystem(systemId)
        .then((atlas) => {
          reportViewerDiagnostic({
            event: "system_metadata_ready",
            severity: "info",
            phase: "system_metadata",
            resource: atlas.modelUrl,
            attributes: {
              "anatomy.system_id": systemId,
              "system.duration_ms": roundMetric(
                diagnosticNow() - systemStartedAt,
              ),
            },
          });
          setLoadedAtlases((current) =>
            current.some((item) => item.modelUrl === atlas.modelUrl)
              ? current
              : [...current, atlas],
          );
          setLoadedSystems((current) => {
            const next = new Set(current);
            next.add(systemId);
            return next;
          });
        })
        .catch((reason: unknown) => {
          const normalized = normalizeVanatomeError(reason);
          reportViewerDiagnostic({
            event: "system_metadata_failed",
            severity: "warn",
            code: normalized.kind,
            message: normalized.message,
            phase: "system_metadata",
            attributes: {
              "anatomy.system_id": systemId,
              "system.duration_ms": roundMetric(
                diagnosticNow() - systemStartedAt,
              ),
            },
          });
          setViewerError(normalized);
          // The regional shell remains usable if an optional system metadata
          // request fails. Do not collapse the entire body surface to 2D.
        })
        .finally(() => {
          loadingSystemsRef.current.delete(systemId);
        });

      loadingSystemsRef.current.set(systemId, pending);
      return pending;
    },
    [loaded, loadedSystems, reportViewerDiagnostic],
  );

  const loadedStructureIds = useMemo(
    () =>
      new Set(
        loadedAtlases.flatMap((atlas) =>
          atlas.structures.map((structure) => structure.id),
        ),
      ),
    [loadedAtlases],
  );

  useEffect(() => {
    if (!selectedAnatomyId) return;
    const system = getPinnedAtlasStructure(selectedAnatomyId)?.system;
    if (system) void ensureSystem(system);
  }, [ensureSystem, selectedAnatomyId]);

  useResizeObserver(rootRef, setContainerWidth);

  useEffect(() => {
    if (selectedAnatomyId === controller.selectedId) return;
    if (selectedAnatomyId) {
      if (!loadedStructureIds.has(selectedAnatomyId)) return;
      controller.focus(selectedAnatomyId);
    } else {
      controller.clear();
    }
  }, [controller, loadedStructureIds, selectedAnatomyId]);

  const lastResetRequest = useRef<string | number | null>(null);
  useEffect(() => {
    if (
      resetRequestKey === undefined ||
      lastResetRequest.current === resetRequestKey
    )
      return;
    lastResetRequest.current = resetRequestKey;
    controller.reset();
  }, [controller, resetRequestKey]);

  const lastFocusRequest = useRef<string | number | null>(null);
  useEffect(() => {
    if (!focusRequest || lastFocusRequest.current === focusRequest.key) return;
    if (!loadedStructureIds.has(focusRequest.id)) {
      const system = getPinnedAtlasStructure(focusRequest.id)?.system;
      if (system) void ensureSystem(system);
      return;
    }
    lastFocusRequest.current = focusRequest.key;
    controller.focus(focusRequest.id);
    onSelectedAnatomyIdChange(focusRequest.id);
  }, [
    controller,
    ensureSystem,
    focusRequest,
    loadedStructureIds,
    onSelectedAnatomyIdChange,
  ]);

  useEffect(() => {
    if (mode !== "region") return;
    setDisplayMode("normal");
    const selectedLayer = selectedAnatomyId
      ? loadedAtlases
          .flatMap((atlas) => atlas.structures)
          .find((structure) => structure.id === selectedAnatomyId)?.layer
      : null;
    const regionLayers = selectedLayer
      ? [selectedLayer]
      : [...REGION_MODE_LAYERS];
    const alreadyRegionLayers =
      controller.visibleLayers.length === regionLayers.length &&
      regionLayers.every(
        (layer, index) => controller.visibleLayers[index] === layer,
      );
    if (!alreadyRegionLayers) controller.setVisibleLayers(regionLayers);
    if (controller.isolation) controller.isolate(null);
  }, [
    controller.isolate,
    controller.isolation,
    controller.setVisibleLayers,
    controller.visibleLayers,
    loadedAtlases,
    mode,
    selectedAnatomyId,
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

  const clearModelWatchdogs = useCallback((modelUrl: string) => {
    const timers = modelWatchdogsRef.current.get(modelUrl);
    if (!timers) return;
    clearTimeout(timers.slow);
    clearTimeout(timers.stalled);
    modelWatchdogsRef.current.delete(modelUrl);
  }, []);

  const armModelWatchdogs = useCallback(
    (modelUrl: string) => {
      clearModelWatchdogs(modelUrl);
      const reportThreshold = (
        event: string,
        severity: "warn" | "error",
        thresholdMs: number,
      ) => {
        reportViewerDiagnostic({
          event,
          severity,
          phase: "model_parse_gpu",
          resource: modelUrl,
          attributes: {
            "watchdog.threshold_ms": thresholdMs,
            "viewer.progress_pct": loadProgressRef.current ?? 0,
            "document.visibility_state": document.visibilityState,
            ...longTaskAttributes(longTaskMetricsRef.current),
            ...resourceTimingAttributes(modelUrl),
          },
        });
      };
      modelWatchdogsRef.current.set(modelUrl, {
        slow: setTimeout(
          () =>
            reportThreshold("model_ready_slow", "warn", MODEL_READY_SLOW_MS),
          MODEL_READY_SLOW_MS,
        ),
        stalled: setTimeout(
          () =>
            reportThreshold(
              "model_ready_stalled",
              "error",
              MODEL_READY_STALLED_MS,
            ),
          MODEL_READY_STALLED_MS,
        ),
      });
    },
    [clearModelWatchdogs, reportViewerDiagnostic],
  );

  const handleViewerError = useCallback(
    (error: VanatomeViewerError) => {
      const normalized = normalizeVanatomeError(error);
      clearModelWatchdogs(error.modelUrl);
      reportViewerDiagnostic({
        event: "vanatome_viewer_error",
        severity: "error",
        code: error.code,
        message: error.message,
        phase: "viewer",
        resource: error.modelUrl,
        attributes: {
          ...longTaskAttributes(longTaskMetricsRef.current),
          ...resourceTimingAttributes(error.modelUrl),
        },
      });
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
    [clearModelWatchdogs, onFatalError, reportViewerDiagnostic],
  );

  const handleContextRestore = useCallback(() => {
    reportViewerDiagnostic({
      event: "webgl_context_restored",
      severity: "info",
      phase: "viewer",
      resource: lastModelUrlRef.current,
      attributes: {
        ...longTaskAttributes(longTaskMetricsRef.current),
        ...resourceTimingAttributes(lastModelUrlRef.current),
      },
    });
    if (recoveryTimer.current) {
      clearTimeout(recoveryTimer.current);
      recoveryTimer.current = null;
    }
    setViewerError(null);
    setWebglState("ready");
    setLoadState("ready");
  }, [reportViewerDiagnostic]);

  useCanvasContextRestore(rootRef, handleContextRestore);

  useEffect(
    () => () => {
      if (recoveryTimer.current) clearTimeout(recoveryTimer.current);
      for (const timers of modelWatchdogsRef.current.values()) {
        clearTimeout(timers.slow);
        clearTimeout(timers.stalled);
      }
      modelWatchdogsRef.current.clear();
    },
    [],
  );

  const structureIndex = useMemo(() => {
    const map = new Map<string, AnatomyStructureSummary>();
    for (const structure of loadedAtlases.flatMap(
      (atlas) => atlas.structures,
    )) {
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
  }, [loadedAtlases]);

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
    <div
      ref={rootRef}
      className="relative flex min-h-[510px] flex-col"
      data-testid="body-explorer-3d"
      data-viewer-state={loadState}
      data-loaded-systems={[...loadedSystems].sort().join(",")}
    >
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="relative min-h-[430px] flex-1 overflow-hidden rounded-[18px] bg-black/5">
          <VanatomeViewer
            atlases={loadedAtlases}
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
            onLoadStart={(modelUrl) => {
              const startedAt = diagnosticNow();
              lastModelUrlRef.current = modelUrl;
              modelStartedAtRef.current.set(modelUrl, startedAt);
              loaderCompleteReportedRef.current.delete(modelUrl);
              networkCompleteReportedRef.current.delete(modelUrl);
              longTaskMetricsRef.current = { count: 0, totalMs: 0, maxMs: 0 };
              reportViewerDiagnostic({
                event: "model_load_started",
                severity: "info",
                phase: "model_download",
                resource: modelUrl,
                attributes: {
                  "model.expected_bytes": expectedModelBytes(loaded, modelUrl),
                  "viewer.model_count": loadedAtlases.length,
                  "document.visibility_state": document.visibilityState,
                },
              });
              armModelWatchdogs(modelUrl);
              setLoadState("loading");
              setViewerError(null);
            }}
            onLoadProgress={({
              loaded: loadedResources,
              total,
              percentage,
            }) => {
              loadProgressRef.current = percentage;
              setLoadProgress(percentage);
              const modelUrl = lastModelUrlRef.current;
              if (
                modelUrl &&
                percentage >= 100 &&
                !loaderCompleteReportedRef.current.has(modelUrl)
              ) {
                loaderCompleteReportedRef.current.add(modelUrl);
                const startedAt = modelStartedAtRef.current.get(modelUrl);
                reportViewerDiagnostic({
                  event: "model_loader_complete",
                  severity: "info",
                  phase: "gltf_loader",
                  resource: modelUrl,
                  attributes: {
                    "loader.from_model_start_ms":
                      startedAt == null
                        ? 0
                        : roundMetric(diagnosticNow() - startedAt),
                    "loader.loaded_resources": loadedResources,
                    "loader.total_resources": total,
                    ...resourceTimingAttributes(modelUrl),
                  },
                });
              }
            }}
            onModelReady={(modelUrl) => {
              clearModelWatchdogs(modelUrl);
              const startedAt = modelStartedAtRef.current.get(modelUrl);
              reportViewerDiagnostic({
                event: "model_ready",
                severity: "info",
                phase: "model_parse_gpu",
                resource: modelUrl,
                attributes: {
                  "model.ready_duration_ms":
                    startedAt == null
                      ? 0
                      : roundMetric(diagnosticNow() - startedAt),
                  "viewer.progress_pct": loadProgressRef.current ?? 100,
                  ...longTaskAttributes(longTaskMetricsRef.current),
                  ...resourceTimingAttributes(modelUrl),
                },
              });
            }}
            onReady={() => {
              reportViewerDiagnostic({
                event: "viewer_ready",
                severity: "info",
                phase: "first_ready",
                resource: lastModelUrlRef.current,
                attributes: {
                  "viewer.model_count": loadedAtlases.length,
                  "viewer.progress_pct": 100,
                  ...longTaskAttributes(longTaskMetricsRef.current),
                  ...resourceTimingAttributes(lastModelUrlRef.current),
                },
              });
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
              onReturnToRegion={() => onModeChange("region")}
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

interface LongTaskMetrics {
  count: number;
  totalMs: number;
  maxMs: number;
}

function diagnosticNow(): number {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

function roundMetric(value: number): number {
  return Math.round(value * 10) / 10;
}

function longTaskAttributes(
  metrics: LongTaskMetrics,
): Record<string, ClientDiagnosticAttributeValue> {
  return {
    "main_thread.long_task_count": metrics.count,
    "main_thread.long_task_total_ms": roundMetric(metrics.totalMs),
    "main_thread.long_task_max_ms": roundMetric(metrics.maxMs),
  };
}

function resourceTimingAttributes(
  resource: string | null | undefined,
): Record<string, ClientDiagnosticAttributeValue> {
  if (!resource || typeof performance === "undefined") return {};
  let absolute: string;
  try {
    absolute = new URL(resource, window.location.href).href;
  } catch {
    return {};
  }
  const entries = performance.getEntriesByName(absolute, "resource");
  const entry = entries.at(-1);
  if (!(entry instanceof PerformanceResourceTiming)) return {};
  return resourceTimingEntryAttributes(entry);
}

function resourceTimingEntryAttributes(
  entry: PerformanceResourceTiming,
): Record<string, ClientDiagnosticAttributeValue> {
  // Cross-origin Resource Timing is redacted unless the asset host opts in via
  // Timing-Allow-Origin. When responseStart/size fields are zeroed, computing
  // derived TTFB/transfer values creates impossible negative or huge numbers.
  // Keep the coarse duration and explicitly mark the record as restricted.
  const timingRestricted =
    entry.responseStart <= 0 || entry.responseStart < entry.startTime;
  if (timingRestricted) {
    return {
      "resource.duration_ms": roundMetric(entry.duration),
      "resource.timing_restricted": true,
      "network.protocol": entry.nextHopProtocol || "unknown",
    };
  }

  return {
    "resource.duration_ms": roundMetric(entry.duration),
    "resource.ttfb_ms": roundMetric(entry.responseStart - entry.startTime),
    "resource.transfer_ms": roundMetric(
      entry.responseEnd - entry.responseStart,
    ),
    "resource.transfer_size": entry.transferSize,
    "resource.encoded_body_size": entry.encodedBodySize,
    "resource.decoded_body_size": entry.decodedBodySize,
    "resource.cache_hit": entry.transferSize === 0 && entry.decodedBodySize > 0,
    "network.protocol": entry.nextHopProtocol || "unknown",
  };
}

function absoluteResourceUrl(resource: string): string {
  try {
    return new URL(resource, window.location.href).href;
  } catch {
    return resource;
  }
}

function expectedModelBytes(
  loaded: LoadedVanatomeAtlas,
  modelUrl: string,
): number {
  let absoluteModelUrl: string;
  let absoluteCatalogUrl: string;
  try {
    absoluteModelUrl = new URL(modelUrl, window.location.href).href;
    absoluteCatalogUrl = new URL(loaded.catalogUrl, window.location.href).href;
  } catch {
    return 0;
  }
  for (const bundle of loaded.catalog.bundles) {
    try {
      if (
        new URL(bundle.modelUrl, absoluteCatalogUrl).href === absoluteModelUrl
      ) {
        return bundle.bytes ?? 0;
      }
    } catch {
      continue;
    }
  }
  return 0;
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
