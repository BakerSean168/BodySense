import { useLayoutEffect, useRef } from "react";
import type { BusinessWindowId } from "../model/businessWindows";

/**
 * Focus a business window only when it becomes active and visible. When that
 * window is hidden, return keyboard focus to its Dock entry only if focus was
 * actually in the window (or the browser reset it to body on hiding).
 *
 * Merely mounting already-hidden windows must not steal focus. Retaining the
 * same host DOM node also preserves pending Ask User answers on resize.
 */
export function useBusinessWindowFocus(
  windowId: BusinessWindowId,
  isActive: boolean,
  minimized: boolean,
) {
  const hostRef = useRef<HTMLDivElement>(null);
  const wasVisibleRef = useRef(false);

  useLayoutEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const visible = isActive && !minimized;
    const wasVisible = wasVisibleRef.current;
    wasVisibleRef.current = visible;

    if (visible && !wasVisible) {
      host.focus({ preventScroll: true });
      return;
    }

    if (!visible && wasVisible) {
      const focusWasInHiddenWindow =
        host.contains(document.activeElement) ||
        document.activeElement === document.body;
      if (!focusWasInHiddenWindow) return;

      const dockId = windowId === "trainer" ? "training" : windowId;
      document
        .querySelector<HTMLButtonElement>(`.bc-dock-${dockId}`)
        ?.focus({ preventScroll: true });
    }
  }, [isActive, minimized, windowId]);

  return hostRef;
}
