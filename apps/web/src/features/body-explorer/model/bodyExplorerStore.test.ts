import { beforeEach, describe, expect, it } from "vitest";
import { asAnatomyStructureId } from "./anatomyTypes";
import { useBodyExplorerStore } from "./bodyExplorerStore";

function resetStore() {
  useBodyExplorerStore.setState({
    mode: "region",
    selectedRegionId: null,
    hoveredRegionId: null,
    selectedAnatomyId: null,
    hoveredAnatomyId: null,
    isolatedAnatomyId: null,
    visibleSystems: [],
    displayMode: "normal",
    cameraPreset: "free",
    cameraIntent: null,
    cameraRequestSequence: 0,
  });
}

describe("BodyExplorerStore", () => {
  beforeEach(() => {
    resetStore();
    window.localStorage.clear();
  });

  it("owns presentation selection without copying health truth", () => {
    const anatomyId = asAnatomyStructureId(
      "appendicular-skeleton-clavicle-right",
    );
    const store = useBodyExplorerStore.getState();

    store.selectRegion("shoulder.right");
    store.hoverRegion("shoulder.left");
    store.setMode("anatomy");
    useBodyExplorerStore.getState().selectAnatomy(
      anatomyId,
      "shoulder.right",
    );
    useBodyExplorerStore.getState().isolateAnatomy(anatomyId);

    const state = useBodyExplorerStore.getState();
    expect(state.mode).toBe("anatomy");
    expect(state.selectedRegionId).toBe("shoulder.right");
    expect(state.hoveredRegionId).toBe("shoulder.left");
    expect(state.selectedAnatomyId).toBe(anatomyId);
    expect(state.isolatedAnatomyId).toBe(anatomyId);
    expect("bodyState" in state).toBe(false);
    expect("facts" in state).toBe(false);
    expect("observations" in state).toBe(false);
    expect(window.localStorage.length).toBe(0);
  });

  it("clears anatomy-only state when returning to region mode", () => {
    const anatomyId = asAnatomyStructureId(
      "appendicular-skeleton-scapula-left",
    );
    const store = useBodyExplorerStore.getState();
    store.selectRegion("scapular.left");
    store.setMode("anatomy");
    useBodyExplorerStore.getState().selectAnatomy(anatomyId, "scapular.left");
    useBodyExplorerStore.getState().isolateAnatomy(anatomyId);

    useBodyExplorerStore.getState().setMode("region");
    expect(useBodyExplorerStore.getState()).toMatchObject({
      mode: "region",
      selectedRegionId: "scapular.left",
      selectedAnatomyId: null,
      hoveredAnatomyId: null,
      isolatedAnatomyId: null,
    });
  });

  it("deduplicates visible systems and emits monotonic camera intents", () => {
    const anatomyId = asAnatomyStructureId(
      "appendicular-skeleton-patella-left",
    );
    const store = useBodyExplorerStore.getState();
    store.setVisibleSystems(["skeletal", "muscular", "skeletal", ""]);
    store.requestFocus(anatomyId);
    const focus = useBodyExplorerStore.getState().cameraIntent;
    useBodyExplorerStore.getState().requestCameraPreset("back");
    const preset = useBodyExplorerStore.getState().cameraIntent;
    useBodyExplorerStore.getState().requestReset();
    const reset = useBodyExplorerStore.getState().cameraIntent;

    expect(useBodyExplorerStore.getState().visibleSystems).toEqual([
      "skeletal",
      "muscular",
    ]);
    expect(focus).toEqual({ requestId: 1, kind: "focus", anatomyId });
    expect(preset).toEqual({ requestId: 2, kind: "preset", preset: "back" });
    expect(reset).toEqual({ requestId: 3, kind: "reset" });
  });
});
