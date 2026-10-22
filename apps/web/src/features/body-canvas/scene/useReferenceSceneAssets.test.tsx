import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useReferenceSceneAssets } from "./useReferenceSceneAssets";

const mocks = vi.hoisted(() => ({
  skin: vi.fn(),
  muscles: vi.fn(),
  skeleton: vi.fn(),
  makeRig: vi.fn(),
  attachMuscles: vi.fn(),
  attachSkeleton: vi.fn(),
}));

vi.mock("./referenceAtlasLoader", () => ({
  loadPinnedReferenceAtlasSurface: mocks.skin,
  loadPinnedReferenceMuscles: mocks.muscles,
  loadPinnedReferenceSkeleton: mocks.skeleton,
}));
vi.mock("./referenceRig", () => ({
  buildReferenceRig: mocks.makeRig,
  attachReferenceMuscles: mocks.attachMuscles,
  attachReferenceSkeleton: mocks.attachSkeleton,
}));

function surface() {
  return { group: {}, dispose: vi.fn() };
}

function fixture() {
  const skin = surface();
  const muscle = surface();
  const bone = surface();
  const rig = { dispose: vi.fn() };
  const muscles = { dispose: vi.fn() };
  const skeleton = { dispose: vi.fn() };
  mocks.skin.mockResolvedValue(skin);
  mocks.muscles.mockResolvedValue(muscle);
  mocks.skeleton.mockResolvedValue(bone);
  mocks.makeRig.mockReturnValue(rig);
  mocks.attachMuscles.mockReturnValue(muscles);
  mocks.attachSkeleton.mockReturnValue(skeleton);
  return { skin, muscle, bone, rig, muscles, skeleton };
}

beforeEach(() => vi.resetAllMocks());
afterEach(() => vi.useRealTimers());

describe("3D scene asset ownership", () => {
  it("loads skin and muscles without downloading the optional skeleton; releases each asset on unmount", async () => {
    const resources = fixture();
    const status = vi.fn();
    const { result, rerender, unmount } = renderHook(
      ({ enabled }) => useReferenceSceneAssets(enabled, status),
      { initialProps: { enabled: false } },
    );
    await waitFor(() => expect(result.current.rig).toBe(resources.rig));
    await waitFor(() => expect(result.current.muscles).toBe(resources.muscles));
    expect(mocks.skeleton).not.toHaveBeenCalled();
    expect(resources.skin.dispose).toHaveBeenCalledOnce();
    expect(resources.muscle.dispose).toHaveBeenCalledOnce();

    rerender({ enabled: true });
    await waitFor(() => expect(result.current.skeletonStatus).toBe("ready"));
    expect(result.current.skeleton).toBe(resources.skeleton);
    expect(mocks.skeleton).toHaveBeenCalledOnce();
    expect(status).toHaveBeenCalledWith("ready");
    rerender({ enabled: false });
    expect(result.current.skeletonStatus).toBe("ready");
    expect(mocks.skeleton).toHaveBeenCalledOnce();
    unmount();
    expect(resources.bone.dispose).toHaveBeenCalledOnce();
    expect(resources.skeleton.dispose).toHaveBeenCalledOnce();
    expect(resources.rig.dispose).toHaveBeenCalledOnce();
    expect(resources.muscles.dispose).toHaveBeenCalledOnce();
  });

  it("reports a skin download failure and supports a retry with a fresh AbortController", async () => {
    const resources = fixture();
    mocks.skin
      .mockRejectedValueOnce(new Error("atlas unavailable"))
      .mockResolvedValueOnce(resources.skin);
    const { result, unmount } = renderHook(() =>
      useReferenceSceneAssets(false),
    );
    await waitFor(() => expect(result.current.error).toBe("atlas unavailable"));
    expect(result.current.rig).toBeNull();
    await act(async () => {
      result.current.setAttempt((attempt) => attempt + 1);
    });
    await waitFor(() => expect(result.current.rig).toBe(resources.rig));
    expect(result.current.error).toBeNull();
    expect(mocks.skin).toHaveBeenCalledTimes(2);
    expect(mocks.skin.mock.calls[0]?.[0]?.signal).not.toBe(
      mocks.skin.mock.calls[1]?.[0]?.signal,
    );
    unmount();
  });

  it("aborts an in-flight skin request without building a 3D rig", async () => {
    const resources = fixture();
    let resolveSkin: ((value: typeof resources.skin) => void) | null = null;
    mocks.skin.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSkin = resolve;
        }),
    );
    const { unmount } = renderHook(() => useReferenceSceneAssets(false));
    const signal = mocks.skin.mock.calls[0]?.[0]?.signal as AbortSignal;
    expect(signal.aborted).toBe(false);
    unmount();
    expect(signal.aborted).toBe(true);
    await act(async () => {
      resolveSkin?.(resources.skin);
    });
    expect(resources.skin.dispose).toHaveBeenCalledOnce();
    expect(mocks.makeRig).not.toHaveBeenCalled();
  });
});
