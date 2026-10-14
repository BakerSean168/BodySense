import { describe, expect, it } from "vitest";
import {
  activeBusinessWindow,
  businessWindowReducer,
  businessWindowStackIndex,
  BUSINESS_WINDOW_LAYOUT,
  computeBmi,
  desktopWindowOffset,
  EMPTY_BUSINESS_WINDOWS,
  formatCompactDate,
  formatGender,
  formatMetricValue,
  isBusinessWindowOpen,
  isBusinessWindowVisible,
  minimizedBusinessWindows,
  visibleBusinessWindows,
  type BusinessWindowState,
} from "./businessWindows";

describe("business window lifecycle", () => {
  it("opens a window once and makes it active", () => {
    const state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "diagnosis",
    });
    expect(state.order).toEqual(["diagnosis"]);
    expect(activeBusinessWindow(state)).toBe("diagnosis");
  });

  it("never leaves a duplicate Training window when switching plan variants", () => {
    // A plan switch re-opens the same business identity; the selectable plan is
    // separate state, so only one Training window may ever exist.
    let state: BusinessWindowState = EMPTY_BUSINESS_WINDOWS;
    state = businessWindowReducer(state, { type: "open", window: "training" });
    state = businessWindowReducer(state, { type: "open", window: "training" });
    state = businessWindowReducer(state, { type: "open", window: "training" });
    expect(state.order).toEqual(["training"]);
    expect(state.order).toHaveLength(1);
  });

  it("never shows two windows at once when another opens on demand (V5.5 Dock exclusion)", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    // Dock/Open/Active are mutually exclusive: the previous window is folded
    // away, not destroyed, so no two windows can overlap or steal each other's
    // controls.
    expect(visibleBusinessWindows(state)).toEqual(["diagnosis"]);
    expect(activeBusinessWindow(state)).toBe("diagnosis");
    expect(isBusinessWindowOpen(state, "training")).toBe(true);
    expect(minimizedBusinessWindows(state)).toEqual(["training"]);
  });

  it("restores the folded window instead of reopening it twice", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    state = businessWindowReducer(state, {
      type: "restore",
      window: "training",
    });
    expect(state.order).toEqual(["diagnosis", "training"]);
    expect(visibleBusinessWindows(state)).toEqual(["training"]);
    expect(state.order).toHaveLength(2);
  });

  it("toggles a window away without destroying its identity", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "toggle",
      window: "chat",
    });
    expect(isBusinessWindowVisible(state, "chat")).toBe(true);
    state = businessWindowReducer(state, { type: "toggle", window: "chat" });
    // Folded away but still recoverable, so closing the Dock toggle never
    // discards an in-flight conversation.
    expect(isBusinessWindowVisible(state, "chat")).toBe(false);
    expect(isBusinessWindowOpen(state, "chat")).toBe(true);
    state = businessWindowReducer(state, { type: "toggle", window: "chat" });
    expect(isBusinessWindowVisible(state, "chat")).toBe(true);
    expect(state.order).toEqual(["chat"]);
  });

  it("keeps minimized windows open but not visible, and restores them", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, {
      type: "minimize",
      window: "training",
    });
    expect(isBusinessWindowOpen(state, "training")).toBe(true);
    expect(isBusinessWindowVisible(state, "training")).toBe(false);
    expect(minimizedBusinessWindows(state)).toEqual(["training"]);
    expect(activeBusinessWindow(state)).toBeNull();
    state = businessWindowReducer(state, {
      type: "restore",
      window: "training",
    });
    expect(isBusinessWindowVisible(state, "training")).toBe(true);
    expect(activeBusinessWindow(state)).toBe("training");
    expect(state.order).toEqual(["training"]);
  });

  it("restoring a closed window opens it exactly once", () => {
    const state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "restore",
      window: "person",
    });
    expect(state.order).toEqual(["person"]);
    expect(state.minimized).toEqual([]);
  });

  it("does not duplicate minimized entries when a window is folded repeatedly", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    state = businessWindowReducer(state, { type: "open", window: "training" });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    expect(state.minimized).toEqual(["training"]);
    expect(new Set(state.minimized).size).toBe(state.minimized.length);
    expect(visibleBusinessWindows(state)).toEqual(["diagnosis"]);
  });

  it("closing clears minimized state for that window", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "person",
    });
    state = businessWindowReducer(state, {
      type: "minimize",
      window: "person",
    });
    state = businessWindowReducer(state, { type: "close", window: "person" });
    expect(state).toEqual(EMPTY_BUSINESS_WINDOWS);
  });

  it("refocuses an already open window on repeated open", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "chat",
    });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    expect(state.order).toEqual(["chat", "diagnosis"]);
    state = businessWindowReducer(state, { type: "open", window: "chat" });
    expect(state.order).toEqual(["diagnosis", "chat"]);
    expect(state.order).toHaveLength(2);
  });

  it("closeAll returns the body-only stage", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "chat",
    });
    state = businessWindowReducer(state, { type: "open", window: "training" });
    state = businessWindowReducer(state, { type: "closeAll" });
    expect(state).toEqual(EMPTY_BUSINESS_WINDOWS);
    expect(activeBusinessWindow(state)).toBeNull();
  });

  it("places only the visible window at the base position", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "diagnosis",
    });
    expect(businessWindowStackIndex(state, "diagnosis")).toBe(0);
    expect(businessWindowStackIndex(state, "chat")).toBe(-1);
    state = businessWindowReducer(state, { type: "open", window: "chat" });
    // The folded window is not placed on the stage at all.
    expect(businessWindowStackIndex(state, "chat")).toBe(0);
    expect(businessWindowStackIndex(state, "diagnosis")).toBe(-1);
  });
});

describe("desktop window placement adapts instead of hardcoding the prototype frame", () => {
  it("bases the first window at the Penpot offset and cascades later ones", () => {
    expect(desktopWindowOffset(0)).toEqual({ left: 40, top: 28 });
    expect(desktopWindowOffset(1)).toEqual({ left: 66, top: 54 });
  });

  it("bounds the cascade so windows cannot walk off stage", () => {
    expect(desktopWindowOffset(99)).toEqual(desktopWindowOffset(3));
    expect(desktopWindowOffset(-5)).toEqual(desktopWindowOffset(0));
    expect(desktopWindowOffset(Number.NaN)).toEqual(desktopWindowOffset(0));
  });
});

describe("Penpot V2 business window geometry", () => {
  it("keeps desktop window sizes aligned with the actual composed Penpot scenes", () => {
    expect(BUSINESS_WINDOW_LAYOUT.person.desktop).toEqual({
      width: 780,
      height: 680,
    });
    expect(BUSINESS_WINDOW_LAYOUT.diagnosis.desktop).toEqual({
      width: 560,
      height: 680,
    });
    expect(BUSINESS_WINDOW_LAYOUT.training.desktop).toEqual({
      width: 560,
      height: 720,
    });
    expect(BUSINESS_WINDOW_LAYOUT.chat.desktop).toEqual({
      width: 560,
      height: 640,
    });
    expect(BUSINESS_WINDOW_LAYOUT.trainer.desktop).toEqual({
      width: 560,
      height: 700,
    });
  });

  it("shares the 358 x 704 mobile reference window without hiding any module", () => {
    for (const layout of Object.values(BUSINESS_WINDOW_LAYOUT)) {
      expect(layout.mobile).toEqual({ width: 358, height: 704 });
    }
  });
});

describe("profile value formatting never invents data", () => {
  it("derives BMI only from valid height and weight", () => {
    expect(
      computeBmi({ height: { value: 175 }, weight: { value: 67.5 } }),
    ).toBe(22);
    expect(computeBmi({ height: { value: 160 }, weight: { value: 50 } })).toBe(
      19.5,
    );
  });

  it("prefers an explicit server BMI", () => {
    expect(computeBmi({ bmi: 23.44 })).toBe(23.4);
  });

  it("returns null for missing or invalid inputs", () => {
    expect(computeBmi(null)).toBeNull();
    expect(computeBmi({ height: { value: 175 } })).toBeNull();
    expect(
      computeBmi({ height: { value: 0 }, weight: { value: 60 } }),
    ).toBeNull();
    expect(computeBmi({ bmi: 0 })).toBeNull();
  });

  it("renders em dashes instead of placeholder values", () => {
    expect(formatMetricValue(null, "cm")).toBe("— cm");
    expect(formatMetricValue(67.5, "kg", { digits: 1 })).toBe("67.5 kg");
    expect(formatCompactDate(undefined)).toBe("—");
    expect(formatCompactDate("1998-01-13T00:00:00Z")).toBe("1998.01.13");
    expect(formatGender(null)).toBe("—");
    expect(formatGender("male")).toBe("男");
  });
});
