import { useCallback, useMemo } from "react";
import type {
  BodyStateSnapshot,
  ConsultationSpatialContext,
} from "@/features/consultation/types/consultation";
import type { BodyExplorerSemanticBridge } from "../components/BodyExplorer";
import { BodyRegionNavigator } from "../components/BodyRegionNavigator";
import {
  getBodyRegionForAnatomy,
  getPreferredAnatomyIdForRegion,
} from "../model/anatomyMapping";
import {
  getBodyRegionDefinition,
  type BodyRegionId,
} from "../model/bodyRegionOntology";
import { useBodyExplorerStore } from "../model/bodyExplorerStore";

export interface BodyExplorerWorkspaceController {
  selectedRegionId: BodyRegionId | null;
  selectedRegionLabel: string | null;
  selectRegion: (regionId: BodyRegionId | null) => void;
  semanticBridge: BodyExplorerSemanticBridge;
}

export function useBodyExplorerWorkspace(
  snapshot: BodyStateSnapshot | null,
  onAskContext?: (context: ConsultationSpatialContext) => void,
): BodyExplorerWorkspaceController {
  const mode = useBodyExplorerStore((state) => state.mode);
  const selectedRegionId = useBodyExplorerStore(
    (state) => state.selectedRegionId,
  );
  const selectedAnatomyId = useBodyExplorerStore(
    (state) => state.selectedAnatomyId,
  );
  const cameraIntent = useBodyExplorerStore((state) => state.cameraIntent);
  const setMode = useBodyExplorerStore((state) => state.setMode);
  const selectRegionState = useBodyExplorerStore((state) => state.selectRegion);
  const selectAnatomy = useBodyExplorerStore((state) => state.selectAnatomy);
  const requestFocus = useBodyExplorerStore((state) => state.requestFocus);
  const requestReset = useBodyExplorerStore((state) => state.requestReset);

  const selectedRegionLabel = selectedRegionId
    ? getBodyRegionDefinition(selectedRegionId).labels["zh-CN"]
    : null;

  const selectRegion = useCallback(
    (regionId: BodyRegionId | null) => {
      if (!regionId) {
        selectRegionState(null);
        selectAnatomy(null, null);
        setMode("region");
        requestReset();
        return;
      }

      const focusAnatomyId = getPreferredAnatomyIdForRegion(regionId);
      setMode("region");
      selectRegionState(regionId);
      selectAnatomy(focusAnatomyId, regionId);
      requestFocus(focusAnatomyId);
    },
    [requestFocus, requestReset, selectAnatomy, selectRegionState, setMode],
  );

  const handleAnatomySelection = useCallback(
    (anatomyId: typeof selectedAnatomyId) => {
      if (!anatomyId) {
        selectAnatomy(null);
        return;
      }
      const owner = getBodyRegionForAnatomy(anatomyId);
      selectAnatomy(anatomyId, owner ?? undefined);
    },
    [selectAnatomy],
  );

  const handleRegionModeRequested = useCallback(() => {
    setMode("region");
    if (!selectedRegionId) return;
    const focusAnatomyId = getPreferredAnatomyIdForRegion(selectedRegionId);
    selectAnatomy(focusAnatomyId, selectedRegionId);
    requestFocus(focusAnatomyId);
  }, [requestFocus, selectAnatomy, selectedRegionId, setMode]);

  const handleAskContext = useCallback(
    (context: {
      anatomyId?: string | null;
      anatomyName?: string | null;
      regionLabel?: string | null;
    }) => {
      if (!onAskContext) return;
      const regionId = selectedRegionId;
      const regionLabel =
        context.regionLabel ??
        (regionId ? getBodyRegionDefinition(regionId).labels["zh-CN"] : null);
      if (!regionId && !context.anatomyId) return;
      onAskContext({
        body_region_id: regionId ?? undefined,
        body_region_label: regionLabel ?? undefined,
        anatomy_id: context.anatomyId ?? undefined,
        anatomy_name: context.anatomyName ?? undefined,
      });
    },
    [onAskContext, selectedRegionId],
  );

  const semanticRegionTree = useMemo(
    () => (
      <BodyRegionNavigator
        snapshot={snapshot}
        selectedRegionId={selectedRegionId}
        onSelectRegion={selectRegion}
      />
    ),
    [selectRegion, selectedRegionId, snapshot],
  );

  const focusRequest =
    cameraIntent?.kind === "focus"
      ? { id: cameraIntent.anatomyId, key: cameraIntent.requestId }
      : null;
  const resetRequestKey =
    cameraIntent?.kind === "reset" ? cameraIntent.requestId : undefined;

  return {
    selectedRegionId,
    selectedRegionLabel,
    selectRegion,
    semanticBridge: {
      selectedAnatomyId,
      selectedRegionLabel,
      focusRequest,
      resetRequestKey,
      mode,
      onModeChange: setMode,
      onAnatomySelectionChange: handleAnatomySelection,
      onRegionModeRequested: handleRegionModeRequested,
      onAskContext: handleAskContext,
      semanticRegionTree,
    },
  };
}
