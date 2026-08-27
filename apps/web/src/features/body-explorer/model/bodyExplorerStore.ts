import { create } from "zustand";
import type { AnatomyStructureId } from "./anatomyTypes";
import type { BodyRegionId } from "./bodyRegionOntology";

export type BodyExplorerMode = "region" | "anatomy";
export type BodyExplorerDisplayMode = "normal" | "xray" | "ghost";
export type BodyExplorerCameraPreset = "free" | "front" | "back" | "left" | "right";

export type BodyExplorerCameraIntent =
  | { requestId: number; kind: "focus"; anatomyId: AnatomyStructureId }
  | { requestId: number; kind: "preset"; preset: BodyExplorerCameraPreset }
  | { requestId: number; kind: "reset" };

export interface BodyExplorerState {
  mode: BodyExplorerMode;
  selectedRegionId: BodyRegionId | null;
  hoveredRegionId: BodyRegionId | null;
  selectedAnatomyId: AnatomyStructureId | null;
  hoveredAnatomyId: AnatomyStructureId | null;
  isolatedAnatomyId: AnatomyStructureId | null;
  visibleSystems: string[];
  displayMode: BodyExplorerDisplayMode;
  cameraPreset: BodyExplorerCameraPreset;
  cameraIntent: BodyExplorerCameraIntent | null;
  cameraRequestSequence: number;
  setMode: (mode: BodyExplorerMode) => void;
  selectRegion: (regionId: BodyRegionId | null) => void;
  hoverRegion: (regionId: BodyRegionId | null) => void;
  selectAnatomy: (
    anatomyId: AnatomyStructureId | null,
    owningRegionId?: BodyRegionId | null,
  ) => void;
  hoverAnatomy: (anatomyId: AnatomyStructureId | null) => void;
  isolateAnatomy: (anatomyId: AnatomyStructureId | null) => void;
  setVisibleSystems: (systemIds: string[]) => void;
  setDisplayMode: (displayMode: BodyExplorerDisplayMode) => void;
  requestFocus: (anatomyId: AnatomyStructureId) => void;
  requestCameraPreset: (preset: BodyExplorerCameraPreset) => void;
  requestReset: () => void;
  resetPresentation: () => void;
}

const initialPresentationState = {
  mode: "region" as const,
  selectedRegionId: null,
  hoveredRegionId: null,
  selectedAnatomyId: null,
  hoveredAnatomyId: null,
  isolatedAnatomyId: null,
  visibleSystems: [] as string[],
  displayMode: "normal" as const,
  cameraPreset: "free" as const,
  cameraIntent: null,
  cameraRequestSequence: 0,
};

function uniqueSystems(systemIds: string[]): string[] {
  return [...new Set(systemIds.filter((systemId) => systemId.trim()).map((systemId) => systemId.trim()))];
}

export const useBodyExplorerStore = create<BodyExplorerState>()((set) => ({
  ...initialPresentationState,
  setMode: (mode) =>
    set((state) =>
      mode === "region"
        ? {
            mode,
            selectedAnatomyId: null,
            hoveredAnatomyId: null,
            isolatedAnatomyId: null,
          }
        : { mode, selectedRegionId: state.selectedRegionId },
    ),
  selectRegion: (selectedRegionId) =>
    set({
      selectedRegionId,
      selectedAnatomyId: null,
      isolatedAnatomyId: null,
    }),
  hoverRegion: (hoveredRegionId) => set({ hoveredRegionId }),
  selectAnatomy: (selectedAnatomyId, owningRegionId) =>
    set((state) => ({
      selectedAnatomyId,
      selectedRegionId:
        owningRegionId === undefined ? state.selectedRegionId : owningRegionId,
    })),
  hoverAnatomy: (hoveredAnatomyId) => set({ hoveredAnatomyId }),
  isolateAnatomy: (isolatedAnatomyId) => set({ isolatedAnatomyId }),
  setVisibleSystems: (visibleSystems) => set({ visibleSystems: uniqueSystems(visibleSystems) }),
  setDisplayMode: (displayMode) => set({ displayMode }),
  requestFocus: (anatomyId) =>
    set((state) => {
      const requestId = state.cameraRequestSequence + 1;
      return {
        cameraRequestSequence: requestId,
        cameraIntent: { requestId, kind: "focus", anatomyId },
      };
    }),
  requestCameraPreset: (cameraPreset) =>
    set((state) => {
      const requestId = state.cameraRequestSequence + 1;
      return {
        cameraPreset,
        cameraRequestSequence: requestId,
        cameraIntent: { requestId, kind: "preset", preset: cameraPreset },
      };
    }),
  requestReset: () =>
    set((state) => {
      const requestId = state.cameraRequestSequence + 1;
      return {
        cameraPreset: "free",
        cameraRequestSequence: requestId,
        cameraIntent: { requestId, kind: "reset" },
      };
    }),
  resetPresentation: () =>
    set((state) => {
      const requestId = state.cameraRequestSequence + 1;
      return {
        ...initialPresentationState,
        cameraRequestSequence: requestId,
        cameraIntent: { requestId, kind: "reset" },
      };
    }),
}));
