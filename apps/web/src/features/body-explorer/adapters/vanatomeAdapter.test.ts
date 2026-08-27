import { AtlasLoaderError } from "@vixotic/vanatome-atlas";
import { describe, expect, it, vi } from "vitest";
import {
  anatomyStructureId,
  type AnatomyViewerErrorState,
} from "./anatomyViewerPort";
import {
  VANATOME_ATLAS_BUILD_ID,
  VANATOME_ATLAS_CATALOG_URL,
  VANATOME_ATLAS_RELEASE,
  VanatomeAdapter,
  normalizeVanatomeError,
  resolveVanatomeCatalogUrl,
  type VanatomeAdapterBridge,
} from "./vanatomeAdapter";

function createBridge() {
  const state: {
    selectedId: string | null;
    hoveredId: string | null;
    isolation: { id: string; mode: "selected" | "parent" | "parent-context" } | null;
    visibleLayers: readonly string[];
    displayMode: "normal" | "xray" | "ghost";
    loadState: "idle" | "loading" | "ready" | "error";
    progress: number | null;
    error: AnatomyViewerErrorState | null;
    webglState: "unknown" | "ready" | "lost" | "unavailable";
  } = {
    selectedId: "structure-a",
    hoveredId: "structure-b",
    isolation: { id: "structure-a", mode: "parent-context" },
    visibleLayers: ["muscular", "skeletal"],
    displayMode: "xray",
    loadState: "ready",
    progress: 100,
    error: null,
    webglState: "ready",
  };

  const actions = {
    select: vi.fn((id: string | null) => {
      state.selectedId = id;
    }),
    hover: vi.fn((id: string | null) => {
      state.hoveredId = id;
    }),
    focus: vi.fn(),
    isolate: vi.fn(
      (id: string | null, mode: "selected" | "parent" | "parent-context") => {
        state.isolation = id ? { id, mode } : null;
      },
    ),
    reset: vi.fn(),
    setVisibleLayers: vi.fn((layers: readonly string[]) => {
      state.visibleLayers = [...layers];
    }),
    setDisplayMode: vi.fn((mode: "normal" | "xray" | "ghost") => {
      state.displayMode = mode;
    }),
  };

  const bridge: VanatomeAdapterBridge = {
    getSelectedId: () => state.selectedId,
    getHoveredId: () => state.hoveredId,
    getIsolation: () => state.isolation,
    getVisibleLayers: () => state.visibleLayers,
    getDisplayMode: () => state.displayMode,
    getLoadState: () => state.loadState,
    getLoadProgress: () => state.progress,
    getError: () => state.error,
    getWebGLState: () => state.webglState,
    ...actions,
  };

  return { state, actions, bridge };
}

describe("VanatomeAdapter", () => {
  it("normalizes upstream viewer state behind the BodySense port", () => {
    const { bridge } = createBridge();
    const adapter = new VanatomeAdapter(bridge);

    expect(adapter.getSnapshot()).toEqual({
      selectedAnatomyId: "structure-a",
      hoveredAnatomyId: "structure-b",
      isolatedAnatomyId: "structure-a",
      isolationMode: "parent-context",
      visibleSystems: ["muscular", "skeletal"],
      displayMode: "xray",
      loadState: "ready",
      loadProgress: 100,
      error: null,
      webglState: "ready",
    });
  });

  it("forwards controlled commands without exposing Vanatome to product callers", () => {
    const { actions, bridge } = createBridge();
    const adapter = new VanatomeAdapter(bridge);
    const selected = anatomyStructureId("structure-c");

    adapter.select(selected);
    adapter.hover(null);
    adapter.focus(selected);
    adapter.isolate(selected, "parent");
    adapter.setVisibleSystems(["nervous"]);
    adapter.setDisplayMode("ghost");
    adapter.resetView();

    expect(actions.select).toHaveBeenCalledWith("structure-c");
    expect(actions.hover).toHaveBeenCalledWith(null);
    expect(actions.focus).toHaveBeenCalledWith("structure-c");
    expect(actions.isolate).toHaveBeenCalledWith("structure-c", "parent");
    expect(actions.setVisibleLayers).toHaveBeenCalledWith(["nervous"]);
    expect(actions.setDisplayMode).toHaveBeenCalledWith("ghost");
    expect(actions.reset).toHaveBeenCalledOnce();
  });
});

describe("Vanatome integration contract", () => {
  it("pins the official immutable atlas release used by the loader", () => {
    expect(VANATOME_ATLAS_RELEASE).toBe("1.4.0");
    expect(VANATOME_ATLAS_BUILD_ID).toBe("994e6cc8ffbb212e");
    expect(VANATOME_ATLAS_CATALOG_URL).toContain("/releases/1.4.0/catalog.json");
    expect(resolveVanatomeCatalogUrl()).toBe(VANATOME_ATLAS_CATALOG_URL);
  });

  it("normalizes atlas and WebGL failures into product-safe error states", () => {
    const atlasError = normalizeVanatomeError(
      new AtlasLoaderError("catalog-fetch", "catalog offline"),
    );
    const webglError = normalizeVanatomeError({
      code: "webgl-context-lost",
      message: "context lost",
      modelUrl: "model.glb",
    });

    expect(atlasError).toMatchObject({
      kind: "atlas",
      message: "catalog offline",
      retryable: true,
    });
    expect(webglError).toEqual({
      kind: "webgl",
      message: "context lost",
      retryable: true,
    });
  });
});
