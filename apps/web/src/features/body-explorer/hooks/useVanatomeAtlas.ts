import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { VanatomeViewerAtlas } from "@vixotic/vanatome-atlas";
import {
  loadPinnedVanatomeAtlas,
  normalizeVanatomeError,
  type LoadedVanatomeAtlas,
} from "../adapters/vanatomeAdapter";
import type { AnatomyViewerErrorState } from "../adapters/anatomyViewerPort";
import { diagnosticNow, roundMetric } from "../adapters/viewerDiagnostics";
import type { ViewerDiagnosticReporter } from "./viewerLifecycle.types";
export function useVanatomeAtlas({
  onFatalError,
  reportViewerDiagnostic,
  setLoadState,
  setViewerError,
}: {
  onFatalError: (error: AnatomyViewerErrorState) => void;
  reportViewerDiagnostic: ViewerDiagnosticReporter;
  setLoadState: (state: "idle" | "loading" | "ready" | "error") => void;
  setViewerError: (error: AnatomyViewerErrorState | null) => void;
}) {
  const [loaded, setLoaded] = useState<LoadedVanatomeAtlas | null>(null);
  const [loadedAtlases, setLoadedAtlases] = useState<
    readonly VanatomeViewerAtlas[]
  >([]);
  const [loadedSystems, setLoadedSystems] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const [failedSystemId, setFailedSystemId] = useState<string | null>(null);
  const requestController = useRef<AbortController | null>(null);
  const loadingSystemsRef = useRef(new Map<string, Promise<void>>());
  const [atlasError, setAtlasError] = useState<AnatomyViewerErrorState | null>(
    null,
  );
  useEffect(() => {
    const abort = new AbortController();
    requestController.current = abort;
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
        if (abort.signal.aborted) return;
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
    return () => {
      abort.abort();
      loadingSystemsRef.current.clear();
    };
  }, [onFatalError, reportViewerDiagnostic]);

  const ensureSystem = useCallback(
    async (systemId: string) => {
      const signal = requestController.current?.signal;
      if (!loaded || loadedSystems.has(systemId) || !signal || signal.aborted)
        return;

      const active = loadingSystemsRef.current.get(systemId);
      if (active) return active;

      setFailedSystemId(null);
      const systemStartedAt = diagnosticNow();
      reportViewerDiagnostic({
        event: "system_metadata_started",
        severity: "info",
        phase: "system_metadata",
        attributes: { "anatomy.system_id": systemId },
      });
      const pending = loaded
        .loadSystem(systemId, { signal })
        .then((atlas) => {
          if (signal.aborted) return;
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
          if (signal.aborted) return;
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
          setFailedSystemId(systemId);
          // The regional shell remains usable if an optional system metadata
          // request fails. Do not collapse the entire body surface to 2D.
        })
        .finally(() => {
          if (loadingSystemsRef.current.get(systemId) === pending)
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

  return {
    failedSystemId,
    loaded,
    loadedAtlases,
    loadedSystems,
    atlasError,
    ensureSystem,
    loadedStructureIds,
  };
}
