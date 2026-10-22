import { createRef } from "react";
import { renderHook } from "@testing-library/react";
import { PerspectiveCamera } from "three";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSurfacePainting } from "./useSurfacePainting";
import type { ReferenceCanvasContentProps } from "./types";

const mocks = vi.hoisted(() => ({
  read: vi.fn(),
  render: vi.fn(),
  dispose: vi.fn(),
}));

vi.mock("./surfacePicker", () => ({
  SurfacePicker: class {
    read = mocks.read;
    render = mocks.render;
    dispose = mocks.dispose;
  },
}));

function pointer(canvas: HTMLCanvasElement, type: string, id = 1) {
  const event = new Event(type, { bubbles: true });
  Object.assign(event, {
    button: 0,
    pointerId: id,
    clientX: 40,
    clientY: 50,
    shiftKey: false,
  });
  canvas.dispatchEvent(event);
}

function fixture() {
  const canvas = document.createElement("canvas");
  vi.spyOn(canvas, "getBoundingClientRect").mockReturnValue({
    left: 0,
    top: 0,
    width: 200,
    height: 200,
    right: 200,
    bottom: 200,
    x: 0,
    y: 0,
    toJSON: () => "",
  });
  canvas.hasPointerCapture = vi.fn(() => false);
  canvas.setPointerCapture = vi.fn();
  canvas.releasePointerCapture = vi.fn();
  const rig = { skeleton: { update: vi.fn() }, meshes: [] };
  const camera = new PerspectiveCamera();
  const onSelect = vi.fn();
  const onHover = vi.fn();
  const latest = createRef<ReferenceCanvasContentProps>() as {
    current: ReferenceCanvasContentProps;
  };
  latest.current = {
    tool: "orbit",
    cursor: null,
    box: null,
    onSelect,
    onHover,
  } as unknown as ReferenceCanvasContentProps;
  const invalidate = vi.fn();
  const gl = { domElement: canvas };
  return { canvas, rig, camera, onSelect, onHover, latest, invalidate, gl };
}

beforeEach(() => {
  mocks.read.mockReset().mockReturnValue(["shoulder.right"]);
  mocks.render.mockReset();
  mocks.dispose.mockReset();
});

describe("SurfacePicker ownership and pointer events", () => {
  it("selects a clicked body region and cleans up pointer handlers on unmount", () => {
    const f = fixture();
    const on = vi.spyOn(f.canvas, "addEventListener");
    const off = vi.spyOn(f.canvas, "removeEventListener");
    const { unmount } = renderHook(() =>
      useSurfacePainting({
        camera: f.camera,
        gl: f.gl as never,
        rig: f.rig as never,
        invalidate: f.invalidate,
        latest: f.latest,
        anchors: { current: [] },
      }),
    );
    pointer(f.canvas, "pointerdown");
    pointer(f.canvas, "pointerup");
    expect(f.onSelect).toHaveBeenCalledExactlyOnceWith(
      ["shoulder.right"],
      false,
    );
    expect(f.rig.skeleton.update).toHaveBeenCalled();
    unmount();
    expect(mocks.dispose).toHaveBeenCalledOnce();
    for (const type of [
      "pointerdown",
      "pointermove",
      "pointerup",
      "pointercancel",
      "pointerleave",
    ]) {
      expect(on).toHaveBeenCalledWith(type, expect.any(Function));
      expect(off).toHaveBeenCalledWith(type, expect.any(Function));
    }
    pointer(f.canvas, "pointerdown");
    pointer(f.canvas, "pointerup");
    expect(f.onSelect).toHaveBeenCalledTimes(1);
  });

  it("does not leave an abandoned orbit drag blocking the next pointer", () => {
    const f = fixture();
    const { unmount } = renderHook(() =>
      useSurfacePainting({
        camera: f.camera,
        gl: f.gl as never,
        rig: f.rig as never,
        invalidate: f.invalidate,
        latest: f.latest,
        anchors: { current: [] },
      }),
    );
    pointer(f.canvas, "pointerdown", 1);
    pointer(f.canvas, "pointerleave", 1);
    pointer(f.canvas, "pointerdown", 2);
    pointer(f.canvas, "pointerup", 2);
    expect(f.onSelect).toHaveBeenCalledExactlyOnceWith(
      ["shoulder.right"],
      false,
    );
    unmount();
  });

  it("uses the latest callbacks without recreating the WebGL picker", () => {
    const f = fixture();
    const { rerender, unmount } = renderHook(() =>
      useSurfacePainting({
        camera: f.camera,
        gl: f.gl as never,
        rig: f.rig as never,
        invalidate: f.invalidate,
        latest: f.latest,
        anchors: { current: [] },
      }),
    );
    const newerSelect = vi.fn();
    f.latest.current.onSelect = newerSelect;
    rerender();
    pointer(f.canvas, "pointerdown");
    pointer(f.canvas, "pointerup");
    expect(newerSelect).toHaveBeenCalledOnce();
    expect(f.onSelect).not.toHaveBeenCalled();
    expect(mocks.dispose).not.toHaveBeenCalled();
    unmount();
    expect(mocks.dispose).toHaveBeenCalledOnce();
  });
});
