import {
  useCallback,
  useEffect,
  useRef,
  type ComponentProps,
  type RefObject,
} from "react";
import {
  VanatomeViewer,
  type VanatomeViewerError,
} from "@vixotic/vanatome-react";
import {
  normalizeVanatomeError,
  type LoadedVanatomeAtlas,
} from "../adapters/vanatomeAdapter";
import type {
  AnatomyViewerErrorState,
  AnatomyWebGLState,
} from "../adapters/anatomyViewerPort";
import {
  diagnosticNow,
  roundMetric,
  longTaskAttributes,
  resourceTimingAttributes,
  resourceTimingEntryAttributes,
  absoluteResourceUrl,
  expectedModelBytes,
  type LongTaskMetrics,
} from "../adapters/viewerDiagnostics";
import type { ViewerDiagnosticReporter } from "./viewerLifecycle.types";
import { useCanvasContextRestore } from "./useViewerCanvas";
const WEBGL_RECOVERY_GRACE_MS = 1800;
const MODEL_READY_SLOW_MS = 8000;
const MODEL_READY_STALLED_MS = 20000;
export function useBodyExplorerViewerLifecycle({
  rootRef,
  loaded,
  loadedAtlasCount,
  onFatalError,
  reportViewerDiagnostic,
  setLoadState,
  setViewerError,
  setWebglState,
  setLoadProgress,
}: {
  rootRef: RefObject<HTMLDivElement | null>;
  loaded: LoadedVanatomeAtlas | null;
  loadedAtlasCount: number;
  onFatalError: (error: AnatomyViewerErrorState) => void;
  reportViewerDiagnostic: ViewerDiagnosticReporter;
  setLoadState: (state: "idle" | "loading" | "ready" | "error") => void;
  setViewerError: (error: AnatomyViewerErrorState | null) => void;
  setWebglState: (state: AnatomyWebGLState) => void;
  setLoadProgress: (progress: number | null) => void;
}) {
  const recoveryTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
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

  useCanvasContextRestore(rootRef, handleContextRestore, Boolean(loaded));

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

  const onReady: NonNullable<
    ComponentProps<typeof VanatomeViewer>["onReady"]
  > = () => {
    reportViewerDiagnostic({
      event: "viewer_ready",
      severity: "info",
      phase: "first_ready",
      resource: lastModelUrlRef.current,
      attributes: {
        "viewer.model_count": loadedAtlasCount,
        "viewer.progress_pct": 100,
        ...longTaskAttributes(longTaskMetricsRef.current),
        ...resourceTimingAttributes(lastModelUrlRef.current),
      },
    });
    setLoadState("ready");
    setLoadProgress(100);
    setViewerError(null);
    setWebglState("ready");
  };
  const onLoadStart: NonNullable<
    ComponentProps<typeof VanatomeViewer>["onLoadStart"]
  > = (modelUrl) => {
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
        "model.expected_bytes": expectedModelBytes(loaded!, modelUrl),
        "viewer.model_count": loadedAtlasCount,
        "document.visibility_state": document.visibilityState,
      },
    });
    armModelWatchdogs(modelUrl);
    setLoadState("loading");
    setViewerError(null);
  };
  const onLoadProgress: NonNullable<
    ComponentProps<typeof VanatomeViewer>["onLoadProgress"]
  > = ({ loaded: loadedResources, total, percentage }) => {
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
            startedAt == null ? 0 : roundMetric(diagnosticNow() - startedAt),
          "loader.loaded_resources": loadedResources,
          "loader.total_resources": total,
          ...resourceTimingAttributes(modelUrl),
        },
      });
    }
  };
  const onModelReady: NonNullable<
    ComponentProps<typeof VanatomeViewer>["onModelReady"]
  > = (modelUrl) => {
    clearModelWatchdogs(modelUrl);
    const startedAt = modelStartedAtRef.current.get(modelUrl);
    reportViewerDiagnostic({
      event: "model_ready",
      severity: "info",
      phase: "model_parse_gpu",
      resource: modelUrl,
      attributes: {
        "model.ready_duration_ms":
          startedAt == null ? 0 : roundMetric(diagnosticNow() - startedAt),
        "viewer.progress_pct": loadProgressRef.current ?? 100,
        ...longTaskAttributes(longTaskMetricsRef.current),
        ...resourceTimingAttributes(modelUrl),
      },
    });
  };
  return {
    onLoadStart,
    onLoadProgress,
    onModelReady,
    onReady,
    handleViewerError,
  };
}
