import { describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useCanvasMotionController } from "./useCanvasMotionController";

const props = {
  safetyReview: false,
  reducedMotion: false,
  setTool: vi.fn(),
  setRegionCardOpen: vi.fn(),
  setCanvasOverlay: vi.fn(),
  demonstrationRequest: null,
};

describe("useCanvasMotionController", () => {
  it("starts a reference animation but never opens an API or health record", () => {
    const callbacks = {
      ...props,
      setTool: vi.fn(),
      setRegionCardOpen: vi.fn(),
      setCanvasOverlay: vi.fn(),
    };
    const { result } = renderHook(() => useCanvasMotionController(callbacks));
    act(() => result.current.choosePose("run"));
    expect(result.current.motion).toBe("run");
    expect(result.current.motionMode).toBe(true);
    expect(result.current.playing).toBe(true);
    expect(callbacks.setTool).toHaveBeenCalledWith("orbit");
    act(() => result.current.choosePose("stand"));
    expect(result.current.motionMode).toBe(false);
    expect(result.current.playing).toBe(false);
  });

  it("respects reduced motion and safety holds on reference animation", () => {
    const { result } = renderHook(() =>
      useCanvasMotionController({
        ...props,
        reducedMotion: true,
        safetyReview: true,
      }),
    );
    act(() => result.current.choosePose("run"));
    expect(result.current.playing).toBe(false);
  });
});
