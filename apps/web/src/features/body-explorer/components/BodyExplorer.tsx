import {
  Component,
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useRef,
  useState,
  type ErrorInfo,
  type ReactNode,
} from "react";
import type { BodyStateSnapshot } from "@/features/consultation/types/consultation";
import { cn } from "@/lib/utils";
import {
  createClientDiagnosticId,
  reportClientDiagnostic,
} from "@/lib/clientDiagnostics";
import type {
  AnatomyStructureId,
  AnatomyViewerErrorState,
} from "../adapters/anatomyViewerPort";
import { BodyExplorerFallback2D } from "./BodyExplorerFallback2D";
import { BodyExplorerLoadingState } from "./BodyExplorerLoadingState";

const LazyBodyExplorer3D = lazy(() => import("./BodyExplorer3D"));

export interface BodyExplorerSemanticBridge {
  selectedAnatomyId?: AnatomyStructureId | null;
  selectedRegionLabel?: string | null;
  focusRequest?: { id: AnatomyStructureId; key: string | number } | null;
  resetRequestKey?: string | number;
  mode?: "region" | "anatomy";
  onModeChange?: (mode: "region" | "anatomy") => void;
  onAnatomySelectionChange?: (id: AnatomyStructureId | null) => void;
  onRegionModeRequested?: () => void;
  onAskContext?: (context: {
    anatomyId?: string | null;
    anatomyName?: string | null;
    regionLabel?: string | null;
  }) => void;
  semanticRegionTree?: ReactNode;
}

export interface BodyExplorerProps {
  snapshot: BodyStateSnapshot | null;
  className?: string;
  semanticBridge?: BodyExplorerSemanticBridge;
}

export function BodyExplorer({
  snapshot,
  className,
  semanticBridge,
}: BodyExplorerProps) {
  const [webgl, setWebgl] = useState<"checking" | "available" | "unavailable">(
    "checking",
  );
  const [failure, setFailure] = useState<AnatomyViewerErrorState | null>(null);
  const [retryKey, setRetryKey] = useState(0);
  const automaticWebGLRetryRef = useRef(0);
  const diagnosticSessionIdRef = useRef(createClientDiagnosticId("body3d"));
  const attemptId = `${diagnosticSessionIdRef.current}-attempt-${retryKey + 1}`;
  const [internalSelectedId, setInternalSelectedId] =
    useState<AnatomyStructureId | null>(null);
  const [internalMode, setInternalMode] = useState<"region" | "anatomy">(
    "region",
  );

  const controlledSelection = semanticBridge?.selectedAnatomyId;
  const mode = semanticBridge?.mode ?? internalMode;
  const selectedAnatomyId =
    controlledSelection === undefined
      ? internalSelectedId
      : controlledSelection;

  useEffect(() => {
    const startedAt = diagnosticNow();
    const available = detectWebGLSupport();
    const elapsedMs = diagnosticNow() - startedAt;
    setWebgl(available ? "available" : "unavailable");
    reportClientDiagnostic({
      category: "body3d.viewer",
      event: "webgl_capability_checked",
      severity: available ? "info" : "error",
      code: available ? undefined : "webgl-unavailable",
      phase: "capability_check",
      diagnosticSessionId: diagnosticSessionIdRef.current,
      attemptId,
      elapsedMs,
      attributes: {
        "webgl.available": available,
        "document.visibility_state": document.visibilityState,
      },
    });
  }, [attemptId, retryKey]);

  const setSelection = useCallback(
    (id: AnatomyStructureId | null) => {
      if (controlledSelection === undefined) setInternalSelectedId(id);
      semanticBridge?.onAnatomySelectionChange?.(id);
    },
    [controlledSelection, semanticBridge],
  );

  const handleModeChange = useCallback(
    (nextMode: "region" | "anatomy") => {
      if (semanticBridge?.onModeChange) semanticBridge.onModeChange(nextMode);
      else setInternalMode(nextMode);
      if (nextMode === "region") semanticBridge?.onRegionModeRequested?.();
    },
    [semanticBridge],
  );

  const handleFailure = useCallback(
    (error: AnatomyViewerErrorState) => {
      reportClientDiagnostic({
        category: "body3d.viewer",
        event: "viewer_failure",
        severity: "error",
        code: error.kind,
        message: error.message,
        phase: "viewer",
        diagnosticSessionId: diagnosticSessionIdRef.current,
        attemptId,
      });

      if (
        error.kind === "webgl" &&
        error.retryable &&
        automaticWebGLRetryRef.current < 1
      ) {
        automaticWebGLRetryRef.current += 1;
        reportClientDiagnostic({
          category: "body3d.viewer",
          event: "webgl_auto_retry",
          severity: "warn",
          code: error.kind,
          message: error.message,
          phase: "viewer_recreate",
          diagnosticSessionId: diagnosticSessionIdRef.current,
          attemptId,
        });
        setFailure(null);
        setWebgl("checking");
        setRetryKey((key) => key + 1);
        return;
      }

      setFailure(error);
    },
    [attemptId],
  );

  const retry = useCallback(() => {
    reportClientDiagnostic({
      category: "body3d.viewer",
      event: "manual_retry",
      severity: "info",
      phase: "viewer_recreate",
      diagnosticSessionId: diagnosticSessionIdRef.current,
      attemptId,
    });
    automaticWebGLRetryRef.current = 0;
    setFailure(null);
    setWebgl("checking");
    setRetryKey((key) => key + 1);
  }, [attemptId]);

  const fallbackError: AnatomyViewerErrorState | null =
    failure ??
    (webgl === "unavailable"
      ? {
          kind: "webgl",
          message: "WebGL is unavailable in this browser or device.",
          retryable: true,
        }
      : null);

  return (
    <section
      aria-labelledby="body-explorer-title"
      className={cn("relative flex min-h-0 flex-col", className)}
    >
      <h2 id="body-explorer-title" className="sr-only">
        3D 身体探索
      </h2>

      {webgl === "checking" ? (
        <BodyExplorerLoadingState label="正在检查 3D 图形支持" />
      ) : fallbackError ? (
        <BodyExplorerFallback2D
          snapshot={snapshot}
          error={fallbackError}
          canRetry={fallbackError.retryable}
          onRetry={retry}
          selectionRetained={Boolean(
            selectedAnatomyId || semanticBridge?.selectedRegionLabel,
          )}
        />
      ) : (
        <ViewerErrorBoundary
          key={retryKey}
          onError={(error, info) => {
            reportClientDiagnostic({
              category: "body3d.viewer",
              event: "react_render_failure",
              severity: "error",
              code: error.name,
              message: error.message,
              phase: "react_error_boundary",
              resource: info.componentStack?.slice(0, 256),
              diagnosticSessionId: diagnosticSessionIdRef.current,
              attemptId,
            });
            handleFailure({
              kind: "unknown",
              message: "The 3D viewer failed to render.",
              retryable: true,
            });
          }}
        >
          <Suspense fallback={<BodyExplorerLoadingState />}>
            <LazyBodyExplorer3D
              selectedAnatomyId={selectedAnatomyId}
              onSelectedAnatomyIdChange={setSelection}
              mode={mode}
              onModeChange={handleModeChange}
              selectedRegionLabel={semanticBridge?.selectedRegionLabel}
              focusRequest={semanticBridge?.focusRequest}
              resetRequestKey={semanticBridge?.resetRequestKey}
              onAskContext={semanticBridge?.onAskContext}
              onFatalError={handleFailure}
              diagnosticSessionId={diagnosticSessionIdRef.current}
              attemptId={attemptId}
            />
          </Suspense>
        </ViewerErrorBoundary>
      )}

      {semanticBridge?.semanticRegionTree ? (
        <div className="mt-3">{semanticBridge.semanticRegionTree}</div>
      ) : null}
    </section>
  );
}

function diagnosticNow(): number {
  return typeof performance !== "undefined" ? performance.now() : Date.now();
}

export function detectWebGLSupport(): boolean {
  if (typeof document === "undefined") return false;
  try {
    const canvas = document.createElement("canvas");
    const context = canvas.getContext("webgl2") || canvas.getContext("webgl");
    if (!context) return false;

    // Capability detection must not consume one of the browser's finite WebGL
    // contexts. Repeated mounts/reloads otherwise make the real Three.js
    // renderer more likely to become the context the browser evicts.
    const loseContext = context.getExtension?.("WEBGL_lose_context");
    loseContext?.loseContext();
    canvas.width = 1;
    canvas.height = 1;
    return true;
  } catch {
    return false;
  }
}

class ViewerErrorBoundary extends Component<
  { children: ReactNode; onError: (error: Error, info: ErrorInfo) => void },
  { failed: boolean }
> {
  override state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    this.props.onError(error, info);
  }

  override render() {
    return this.state.failed ? null : this.props.children;
  }
}
