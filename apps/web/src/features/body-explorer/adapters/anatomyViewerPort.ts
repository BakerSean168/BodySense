export type AnatomyStructureId = string & {
  readonly __brand: "AnatomyStructureId";
};

export type AnatomyDisplayMode = "normal" | "xray" | "ghost";
export type AnatomyIsolationMode = "selected" | "parent" | "parent-context";
export type AnatomyLoadState = "idle" | "loading" | "ready" | "error";
export type AnatomyWebGLState = "unknown" | "ready" | "lost" | "unavailable";

export interface AnatomyViewerErrorState {
  kind: "atlas" | "model" | "webgl" | "unknown";
  message: string;
  retryable: boolean;
}

export interface AnatomyViewerSnapshot {
  selectedAnatomyId: AnatomyStructureId | null;
  hoveredAnatomyId: AnatomyStructureId | null;
  isolatedAnatomyId: AnatomyStructureId | null;
  isolationMode: AnatomyIsolationMode | null;
  visibleSystems: readonly string[];
  displayMode: AnatomyDisplayMode;
  loadState: AnatomyLoadState;
  loadProgress: number | null;
  error: AnatomyViewerErrorState | null;
  webglState: AnatomyWebGLState;
}

export interface AnatomyViewerPort {
  getSnapshot(): AnatomyViewerSnapshot;
  select(id: AnatomyStructureId | null): void;
  hover(id: AnatomyStructureId | null): void;
  focus(id: AnatomyStructureId): void;
  isolate(
    id: AnatomyStructureId | null,
    mode?: AnatomyIsolationMode,
  ): void;
  resetView(): void;
  setVisibleSystems(systemIds: readonly string[]): void;
  setDisplayMode(mode: AnatomyDisplayMode): void;
}

export function anatomyStructureId(value: string): AnatomyStructureId {
  return value as AnatomyStructureId;
}
