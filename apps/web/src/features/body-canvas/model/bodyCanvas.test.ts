import { describe, expect, it } from "vitest";
import {
  buildCanvasContext,
  canvasShortcut,
  demonstrationForTitle,
  EMPTY_SELECTION,
  selectionReducer,
} from "./bodyCanvas";

describe("Body Canvas selection", () => {
  it("deduplicates canonical regions, supports append and preserves undo history", () => {
    const one = selectionReducer(EMPTY_SELECTION, {
      type: "select",
      regions: ["shoulder.right", "shoulder.right"],
    });
    expect(one.regions).toEqual(["shoulder.right"]);

    const two = selectionReducer(one, {
      type: "select",
      regions: ["scapular.right"],
      append: true,
    });
    expect(two.regions).toEqual(["shoulder.right", "scapular.right"]);

    const undone = selectionReducer(two, { type: "undo" });
    expect(undone.regions).toEqual(["shoulder.right"]);
  });
});

describe("Body Canvas consultation context", () => {
  it("marks multi-region motion context as reference animation", () => {
    expect(
      buildCanvasContext(
        ["shoulder.right", "scapular.right"],
        "arm_raise",
        1.8,
        false,
      ),
    ).toEqual({
      body_region_id: "shoulder.right",
      body_region_label: "右肩",
      body_region_ids: ["shoulder.right", "scapular.right"],
      reference_motion: {
        id: "arm_raise",
        label: "抬臂观察",
        phase: 1,
        paused: true,
        source: "reference_animation",
      },
    });
  });

  it("keeps reference motion context even without a selected body region", () => {
    const context = buildCanvasContext([], "stand", 0, true);
    expect(context.body_region_id).toBeUndefined();
    expect(context.reference_motion?.source).toBe("reference_animation");
    expect(context.reference_motion?.paused).toBe(false);
  });

  it("does not invent a motion context while the user is only exploring the body", () => {
    const context = buildCanvasContext(["shoulder.right"], null, 0, false);
    expect(context.body_region_id).toBe("shoulder.right");
    expect(context.reference_motion).toBeUndefined();
  });
});

describe("Body Canvas reference demonstrations", () => {
  it("only maps exercise names with an explicitly authored reference motion", () => {
    expect(demonstrationForTitle("臀桥")).toBe("bridge");
    expect(demonstrationForTitle("Calf Raise")).toBe("calf_raise");
    expect(demonstrationForTitle("髋铰链练习")).toBe("hip_hinge");
    expect(demonstrationForTitle("未知肩部动作")).toBeNull();
  });
});

describe("Body Canvas keyboard contract", () => {
  const event = (key: string, target: EventTarget | null = null) => ({
    key,
    target,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    isComposing: false,
    repeat: false,
    defaultPrevented: false,
  });

  it("maps visible single-key commands", () => {
    expect(canvasShortcut(event("b"))).toEqual({ type: "tool", tool: "brush" });
    expect(canvasShortcut(event("2"))).toEqual({ type: "motion", motion: "run" });
    expect(canvasShortcut(event("p"))).toEqual({
      type: "panel",
      panel: "treatment",
    });
    expect(canvasShortcut(event("Escape"))).toEqual({ type: "escape" });
  });

  it("does not trigger commands while typing", () => {
    const input = document.createElement("input");
    expect(canvasShortcut(event("p", input))).toBeNull();
  });
});
