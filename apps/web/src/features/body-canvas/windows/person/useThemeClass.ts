import { useCallback, type RefObject } from "react";

/**
 * Popups portal to <body>, outside the canvas theme root. Read the theme
 * classes (paper tokens, dark mode) from the element that opens the popup.
 */
export function useThemeClass(anchor: RefObject<HTMLElement | null>) {
  return useCallback(() => {
    const root = anchor.current?.closest<HTMLElement>(".body-canvas-theme");
    if (!root) return "body-canvas-theme";
    return root.classList.contains("bc-dark")
      ? "body-canvas-theme bc-dark"
      : "body-canvas-theme";
  }, [anchor]);
}
