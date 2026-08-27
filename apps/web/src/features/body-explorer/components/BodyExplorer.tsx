import {
  Component,
  Suspense,
  lazy,
  useCallback,
  useEffect,
  useState,
  type ErrorInfo,
  type ReactNode,
} from "react";
import type { BodyStateSnapshot } from "@/features/consultation/types/consultation";
import { cn } from "@/lib/utils";
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
  onAnatomySelectionChange?: (id: AnatomyStructureId | null) => void;
  onRegionModeRequested?: () => void;
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
  const [internalSelectedId, setInternalSelectedId] =
    useState<AnatomyStructureId | null>(null);
  const [mode, setMode] = useState<"region" | "anatomy">("region");

  const controlledSelection = semanticBridge?.selectedAnatomyId;
  const selectedAnatomyId =
    controlledSelection === undefined
      ? internalSelectedId
      : controlledSelection;

  useEffect(() => {
    setWebgl(detectWebGLSupport() ? "available" : "unavailable");
  }, [retryKey]);

  const setSelection = useCallback(
    (id: AnatomyStructureId | null) => {
      if (controlledSelection === undefined) setInternalSelectedId(id);
      semanticBridge?.onAnatomySelectionChange?.(id);
    }, [controlledSelection, semanticBridge],
  );

  const handleModeChange = useCallback(
    (nextMode: "region" | "anatomy") => {
      setMode(nextMode);
      if (nextMode === "region") semanticBridge?.onRegionModeRequested?.();
    },
    [semanticBridge],
  );

  const handleFailure = useCallback((error: AnatomyViewerErrorState) => {
    setFailure(error);
  }, []);

  const retry = useCallback(() => {
    setFailure(null);
    setWebgl("checking");
    setRetryKey((key) => key + 1);
  }, []);

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
          selectionRetained={Boolean(selectedAnatomyId || semanticBridge?.selectedRegionLabel)}
        />
      ) : (
        <ViewerErrorBoundary
          key={retryKey}
          onError={() =>
            handleFailure({
              kind: "unknown",
              message: "The 3D viewer failed to render.",
              retryable: true,
            })
          }
        >
          <Suspense fallback={<BodyExplorerLoadingState />}>
            <LazyBodyExplorer3D
              selectedAnatomyId={selectedAnatomyId}
              onSelectedAnatomyIdChange={setSelection}
              mode={mode}
              onModeChange={handleModeChange}
              selectedRegionLabel={semanticBridge?.selectedRegionLabel}
              focusRequest={semanticBridge?.focusRequest}
              onFatalError={handleFailure}
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

export function detectWebGLSupport(): boolean {
  if (typeof document === "undefined") return false;
  try {
    const canvas = document.createElement("canvas");
    return Boolean(
      canvas.getContext("webgl2") || canvas.getContext("webgl"),
    );
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
