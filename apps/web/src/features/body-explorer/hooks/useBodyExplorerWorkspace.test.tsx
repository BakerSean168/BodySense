import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { getPreferredAnatomyIdForRegion } from "../model/anatomyMapping";
import { useBodyExplorerStore } from "../model/bodyExplorerStore";
import { useBodyExplorerWorkspace } from "./useBodyExplorerWorkspace";

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

describe("useBodyExplorerWorkspace", () => {
  beforeEach(resetStore);

  it("links canonical region selection to the preferred anatomy focus and reset", () => {
    const { result } = renderHook(() => useBodyExplorerWorkspace(null));

    act(() => result.current.selectRegion("shoulder.right"));
    const expected = getPreferredAnatomyIdForRegion("shoulder.right");
    expect(useBodyExplorerStore.getState()).toMatchObject({
      selectedRegionId: "shoulder.right",
      selectedAnatomyId: expected,
      mode: "region",
    });
    expect(useBodyExplorerStore.getState().cameraIntent).toMatchObject({
      kind: "focus",
      anatomyId: expected,
    });

    act(() => result.current.selectRegion(null));
    expect(useBodyExplorerStore.getState()).toMatchObject({
      selectedRegionId: null,
      selectedAnatomyId: null,
      mode: "region",
    });
    expect(useBodyExplorerStore.getState().cameraIntent).toMatchObject({
      kind: "reset",
    });
  });

  it("keeps full-body reset distinct from returning to the selected region", () => {
    const { result } = renderHook(() => useBodyExplorerWorkspace(null));
    const expected = getPreferredAnatomyIdForRegion("shoulder.right");

    act(() => result.current.selectRegion("shoulder.right"));
    act(() => result.current.semanticBridge.onAnatomySelectionChange?.(null));
    act(() => result.current.semanticBridge.onModeChange?.("region"));

    expect(useBodyExplorerStore.getState()).toMatchObject({
      selectedRegionId: "shoulder.right",
      selectedAnatomyId: null,
      mode: "region",
    });

    act(() => result.current.semanticBridge.onRegionModeRequested?.());
    expect(useBodyExplorerStore.getState()).toMatchObject({
      selectedRegionId: "shoulder.right",
      selectedAnatomyId: expected,
      mode: "region",
    });
    expect(useBodyExplorerStore.getState().cameraIntent).toMatchObject({
      kind: "focus",
      anatomyId: expected,
    });
  });

  it("builds removable chat context from the selected BodyRegion", () => {
    const onAskContext = vi.fn();
    const { result } = renderHook(() =>
      useBodyExplorerWorkspace(null, onAskContext),
    );

    act(() => result.current.selectRegion("knee.left"));
    act(() =>
      result.current.semanticBridge.onAskContext?.({
        anatomyId: String(getPreferredAnatomyIdForRegion("knee.left")),
        anatomyName: "Left patella",
        regionLabel: "左膝",
      }),
    );

    expect(onAskContext).toHaveBeenCalledWith({
      body_region_id: "knee.left",
      body_region_label: "左膝",
      anatomy_id: String(getPreferredAnatomyIdForRegion("knee.left")),
      anatomy_name: "Left patella",
    });
  });
});
