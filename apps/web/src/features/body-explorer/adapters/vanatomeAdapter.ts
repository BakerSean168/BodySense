import {
  AtlasLoaderError,
  OFFICIAL_HUMAN_ATLAS,
  createOfficialHumanAtlas,
  type AtlasCatalog,
  type AtlasLoaderWithProfiles,
  type VanatomeViewerAtlas,
} from "@vixotic/vanatome-atlas";
import type {
  VanatomeDisplayMode,
  VanatomeIsolationMode,
  VanatomeViewerError,
} from "@vixotic/vanatome-react";
import {
  anatomyStructureId,
  type AnatomyDisplayMode,
  type AnatomyIsolationMode,
  type AnatomyLoadState,
  type AnatomyStructureId,
  type AnatomyViewerErrorState,
  type AnatomyViewerPort,
  type AnatomyViewerSnapshot,
  type AnatomyWebGLState,
} from "./anatomyViewerPort";

export const VANATOME_ATLAS_RELEASE = "1.4.0" as const;
export const VANATOME_ATLAS_BUILD_ID = "994e6cc8ffbb212e" as const;
export const VANATOME_ATLAS_CATALOG_URL = OFFICIAL_HUMAN_ATLAS.catalogUrl;
export const VANATOME_INITIAL_SYSTEM_ID = "regional-anatomy" as const;

export interface LoadedVanatomeAtlas {
  /** Initial lightweight body-shell atlas kept for compatibility with callers. */
  atlas: VanatomeViewerAtlas;
  /** Atlases already prepared for composition. Models remain lazy in VanatomeViewer. */
  atlases: readonly VanatomeViewerAtlas[];
  catalog: AtlasCatalog;
  catalogUrl: string;
  loadSystem: (
    systemId: string,
    options?: { signal?: AbortSignal },
  ) => Promise<VanatomeViewerAtlas>;
}

export function resolveVanatomeCatalogUrl(override?: string): string {
  const configured =
    override?.trim() ||
    import.meta.env.VITE_BODYSENSE_ANATOMY_CATALOG_URL?.trim();
  return configured || VANATOME_ATLAS_CATALOG_URL;
}

export async function loadPinnedVanatomeAtlas(options?: {
  signal?: AbortSignal;
  catalogUrl?: string;
}): Promise<LoadedVanatomeAtlas> {
  const catalogUrl = resolveVanatomeCatalogUrl(options?.catalogUrl);
  const loader = createOfficialHumanAtlas({ catalogUrl });
  const catalog = await loader.loadCatalog({ signal: options?.signal });

  if (
    catalog.atlas.version !== VANATOME_ATLAS_RELEASE ||
    catalog.atlas.buildId !== VANATOME_ATLAS_BUILD_ID
  ) {
    throw new Error(
      `Unexpected Vanatome atlas ${catalog.atlas.version}/${catalog.atlas.buildId}; expected ${VANATOME_ATLAS_RELEASE}/${VANATOME_ATLAS_BUILD_ID}`,
    );
  }

  // Do not make the 31.8 MiB monolithic full-body GLB a first-paint
  // dependency. Vanatome supports atlas composition, so start with the 6.3 MiB
  // regional body shell and load anatomy systems only when they are selected.
  // This also keeps each static request small enough to tolerate high-latency
  // tailnet/staging links without sacrificing the pinned atlas contract.
  const initialBundle = await loader.loadSystem(VANATOME_INITIAL_SYSTEM_ID, {
    signal: options?.signal,
  });

  return {
    atlas: initialBundle.atlas,
    atlases: [initialBundle.atlas],
    catalog,
    catalogUrl,
    loadSystem: (systemId, loadOptions) =>
      loadPinnedSystem(loader, systemId, loadOptions),
  };
}

async function loadPinnedSystem(
  loader: AtlasLoaderWithProfiles,
  systemId: string,
  options?: { signal?: AbortSignal },
): Promise<VanatomeViewerAtlas> {
  const bundle = await loader.loadSystem(systemId, options);
  return bundle.atlas;
}

export function normalizeVanatomeError(
  error: unknown,
): AnatomyViewerErrorState {
  if (error instanceof AtlasLoaderError) {
    return {
      kind: "atlas",
      message: error.message,
      retryable: error.code !== "catalog-invalid",
    };
  }

  if (isVanatomeViewerError(error)) {
    if (error.code === "webgl-context-lost") {
      return {
        kind: "webgl",
        message: error.message,
        retryable: true,
      };
    }
    return {
      kind: "model",
      message: error.message,
      retryable: true,
    };
  }

  return {
    kind: "unknown",
    message: error instanceof Error ? error.message : "Unknown anatomy viewer error",
    retryable: true,
  };
}

function isVanatomeViewerError(error: unknown): error is VanatomeViewerError {
  if (!error || typeof error !== "object") return false;
  const code = (error as { code?: unknown }).code;
  return code === "model-load-failed" || code === "webgl-context-lost";
}

export interface VanatomeAdapterBridge {
  getSelectedId(): string | null;
  getHoveredId(): string | null;
  getIsolation(): { id: string; mode: VanatomeIsolationMode } | null;
  getVisibleLayers(): readonly string[];
  getDisplayMode(): VanatomeDisplayMode;
  getLoadState(): AnatomyLoadState;
  getLoadProgress(): number | null;
  getError(): AnatomyViewerErrorState | null;
  getWebGLState(): AnatomyWebGLState;
  select(id: string | null): void;
  hover(id: string | null): void;
  focus(id: string): void;
  isolate(id: string | null, mode: VanatomeIsolationMode): void;
  reset(): void;
  setVisibleLayers(layers: readonly string[]): void;
  setDisplayMode(mode: VanatomeDisplayMode): void;
}

export class VanatomeAdapter implements AnatomyViewerPort {
  constructor(private readonly bridge: VanatomeAdapterBridge) {}

  getSnapshot(): AnatomyViewerSnapshot {
    const isolation = this.bridge.getIsolation();
    return {
      selectedAnatomyId: toAnatomyId(this.bridge.getSelectedId()),
      hoveredAnatomyId: toAnatomyId(this.bridge.getHoveredId()),
      isolatedAnatomyId: toAnatomyId(isolation?.id ?? null),
      isolationMode: (isolation?.mode as AnatomyIsolationMode | undefined) ?? null,
      visibleSystems: [...this.bridge.getVisibleLayers()],
      displayMode: this.bridge.getDisplayMode() as AnatomyDisplayMode,
      loadState: this.bridge.getLoadState(),
      loadProgress: this.bridge.getLoadProgress(),
      error: this.bridge.getError(),
      webglState: this.bridge.getWebGLState(),
    };
  }

  select(id: AnatomyStructureId | null): void {
    this.bridge.select(id);
  }

  hover(id: AnatomyStructureId | null): void {
    this.bridge.hover(id);
  }

  focus(id: AnatomyStructureId): void {
    this.bridge.focus(id);
  }

  isolate(
    id: AnatomyStructureId | null,
    mode: AnatomyIsolationMode = "selected",
  ): void {
    this.bridge.isolate(id, mode as VanatomeIsolationMode);
  }

  resetView(): void {
    this.bridge.reset();
  }

  setVisibleSystems(systemIds: readonly string[]): void {
    this.bridge.setVisibleLayers(systemIds);
  }

  setDisplayMode(mode: AnatomyDisplayMode): void {
    this.bridge.setDisplayMode(mode as VanatomeDisplayMode);
  }
}

function toAnatomyId(value: string | null): AnatomyStructureId | null {
  return value ? anatomyStructureId(value) : null;
}
