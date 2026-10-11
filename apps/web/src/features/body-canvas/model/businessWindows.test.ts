import { describe, expect, it } from "vitest";
import {
  activeBusinessWindow,
  businessWindowReducer,
  businessWindowStackIndex,
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

  it("closes the Training window when another business window opens on demand", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, { type: "open", window: "diagnosis" });
    // Both may be visible, but the design contract requires an exclusive
    // single active business window on the shared stage.
    expect(visibleBusinessWindows(state)).toEqual(["training", "diagnosis"]);
    state = businessWindowReducer(state, { type: "close", window: "training" });
    expect(state.order).toEqual(["diagnosis"]);
    expect(isBusinessWindowOpen(state, "training")).toBe(false);
  });

  it("toggles a window off and on without duplicating identity", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "toggle",
      window: "chat",
    });
    expect(isBusinessWindowVisible(state, "chat")).toBe(true);
    state = businessWindowReducer(state, { type: "toggle", window: "chat" });
    expect(state.order).toEqual([]);
    state = businessWindowReducer(state, { type: "toggle", window: "chat" });
    expect(state.order).toEqual(["chat"]);
  });

  it("keeps minimized windows open but not visible, and restores them", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "training",
    });
    state = businessWindowReducer(state, { type: "open", window: "chat" });
    state = businessWindowReducer(state, { type: "minimize", window: "training" });
    expect(isBusinessWindowOpen(state, "training")).toBe(true);
    expect(isBusinessWindowVisible(state, "training")).toBe(false);
    expect(minimizedBusinessWindows(state)).toEqual(["training"]);
    expect(activeBusinessWindow(state)).toBe("chat");
    state = businessWindowReducer(state, { type: "restore", window: "training" });
    expect(isBusinessWindowVisible(state, "training")).toBe(true);
    expect(activeBusinessWindow(state)).toBe("training");
    expect(state.order).toEqual(["chat", "training"]);
  });

  it("restoring a closed window opens it exactly once", () => {
    const state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "restore",
      window: "person",
    });
    expect(state.order).toEqual(["person"]);
    expect(state.minimized).toEqual([]);
  });

  it("closing clears minimized state for that window", () => {
    let state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "person",
    });
    state = businessWindowReducer(state, { type: "minimize", window: "person" });
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

  it("reports stack index and is stable for unknown windows", () => {
    const state = businessWindowReducer(EMPTY_BUSINESS_WINDOWS, {
      type: "open",
      window: "diagnosis",
    });
    expect(businessWindowStackIndex(state, "diagnosis")).toBe(0);
    expect(businessWindowStackIndex(state, "chat")).toBe(-1);
  });
});

describe("desktop window placement adapts instead of hardcoding the prototype frame", () => {
  it("bases the first window at the Penpot offset and cascades later ones", () => {
    expect(desktopWindowOffset(0)).toEqual({ left: 430, top: 18 });
    expect(desktopWindowOffset(1)).toEqual({ left: 456, top: 44 });
  });

  it("bounds the cascade so windows cannot walk off stage", () => {
    expect(desktopWindowOffset(99)).toEqual(desktopWindowOffset(3));
    expect(desktopWindowOffset(-5)).toEqual(desktopWindowOffset(0));
    expect(desktopWindowOffset(Number.NaN)).toEqual(desktopWindowOffset(0));
  });
});

describe("profile value formatting never invents data", () => {
  it("derives BMI only from valid height and weight", () => {
    expect(computeBmi({ height: { value: 175 }, weight: { value: 67.5 } })).toBe(22);
    expect(computeBmi({ height: { value: 160 }, weight: { value: 50 } })).toBe(19.5);
  });

  it("prefers an explicit server BMI", () => {
    expect(computeBmi({ bmi: 23.44 })).toBe(23.4);
  });

  it("returns null for missing or invalid inputs", () => {
    expect(computeBmi(null)).toBeNull();
    expect(computeBmi({ height: { value: 175 } })).toBeNull();
    expect(computeBmi({ height: { value: 0 }, weight: { value: 60 } })).toBeNull();
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
