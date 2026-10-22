import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useVanatomeController } from "@vixotic/vanatome-react";
import type { VanatomeViewerAtlas } from "@vixotic/vanatome-atlas";
import { VanatomeAdapter } from "../adapters/vanatomeAdapter";
import {
  anatomyStructureId,
  type AnatomyDisplayMode,
  type AnatomyViewerErrorState,
  type AnatomyWebGLState,
} from "../adapters/anatomyViewerPort";
import type { AnatomyStructureSummary } from "../components/AnatomyInspector";
import type { BodyExplorer3DProps } from "../components/bodyExplorer3D.types";
import { getPinnedAtlasStructure } from "../model/atlasRegistry";
import { buildBreadcrumb } from "../model/bodyExplorerPresentation";
const REGION_MODE_LAYERS = ["muscular"] as const;
export function useBodyExplorerInteraction({
  props,
  loadedAtlases,
  loadedStructureIds,
  ensureSystem,
  loadState,
  loadProgress,
  viewerError,
  atlasError,
  webglState,
}: {
  props: BodyExplorer3DProps;
  loadedAtlases: readonly VanatomeViewerAtlas[];
  loadedStructureIds: ReadonlySet<string>;
  ensureSystem: (id: string) => Promise<void>;
  loadState: "idle" | "loading" | "ready" | "error";
  loadProgress: number | null;
  viewerError: AnatomyViewerErrorState | null;
  atlasError: AnatomyViewerErrorState | null;
  webglState: AnatomyWebGLState;
}) {
  const {
    selectedAnatomyId,
    onSelectedAnatomyIdChange,
    mode,
    resetRequestKey,
    focusRequest,
  } = props;
  const controller = useVanatomeController(REGION_MODE_LAYERS);
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [displayMode, setDisplayMode] = useState<AnatomyDisplayMode>("normal");
  useEffect(() => {
    if (!selectedAnatomyId) return;
    const system = getPinnedAtlasStructure(selectedAnatomyId)?.system;
    if (system) void ensureSystem(system);
  }, [ensureSystem, selectedAnatomyId]);

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

  return {
    controller,
    hoveredId,
    displayMode,
    adapter,
    handleSelection,
    handleHover,
    selectedStructure,
    hoveredStructure,
    breadcrumb,
  };
}
