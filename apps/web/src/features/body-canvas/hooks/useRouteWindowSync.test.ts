import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { BusinessWindowId } from "../model/businessWindows";
import { useRouteWindowSync } from "./useRouteWindowSync";

type Props = {
  requestedWindow: BusinessWindowId | null;
  activeWindow: BusinessWindowId | null;
};

function setup(initial: Props) {
  const openWindow = vi.fn();
  const onWindowChange = vi.fn();
  const view = renderHook(
    (props: Props) =>
      useRouteWindowSync({ ...props, openWindow, onWindowChange }),
    { initialProps: initial },
  );
  return { ...view, openWindow, onWindowChange };
}

describe("useRouteWindowSync", () => {
  it("opens a deep-linked window once", () => {
    const { rerender, openWindow } = setup({
      requestedWindow: "training",
      activeWindow: null,
    });
    expect(openWindow).toHaveBeenCalledTimes(1);
    expect(openWindow).toHaveBeenCalledWith("training");
    rerender({ requestedWindow: "training", activeWindow: "training" });
    expect(openWindow).toHaveBeenCalledTimes(1);
  });

  it("reports the active window so the route can mirror it", () => {
    const { rerender, onWindowChange } = setup({
      requestedWindow: null,
      activeWindow: null,
    });
    rerender({ requestedWindow: null, activeWindow: "trainer" });
    expect(onWindowChange).toHaveBeenLastCalledWith("trainer");
    rerender({ requestedWindow: null, activeWindow: null });
    expect(onWindowChange).toHaveBeenLastCalledWith(null);
  });

  it("never reopens a window when the route echo lands after the user closed it", () => {
    // Shortcut opens the Trainer; Escape closes it before the router commits
    // the ?view=trainer mirror. The late echo must not reopen it.
    const { rerender, openWindow } = setup({
      requestedWindow: null,
      activeWindow: null,
    });
    rerender({ requestedWindow: null, activeWindow: "trainer" });
    rerender({ requestedWindow: null, activeWindow: null });
    rerender({ requestedWindow: "trainer", activeWindow: null });
    expect(openWindow).not.toHaveBeenCalled();
  });

  it("treats a request after the route cleared its view as a new deep link", () => {
    const { rerender, openWindow } = setup({
      requestedWindow: null,
      activeWindow: null,
    });
    rerender({ requestedWindow: null, activeWindow: "training" });
    rerender({ requestedWindow: "training", activeWindow: "training" });
    rerender({ requestedWindow: "training", activeWindow: null });
    rerender({ requestedWindow: null, activeWindow: null });
    expect(openWindow).not.toHaveBeenCalled();
    rerender({ requestedWindow: "training", activeWindow: null });
    expect(openWindow).toHaveBeenCalledTimes(1);
    expect(openWindow).toHaveBeenCalledWith("training");
  });
});
