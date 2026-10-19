import { useEffect, useRef } from "react";
import type { BusinessWindowId } from "../model/businessWindows";

export interface RouteWindowSyncOptions {
  /** The window the route asks for (`?view=`), or null. */
  requestedWindow: BusinessWindowId | null;
  /** The window the workspace currently shows, or null. */
  activeWindow: BusinessWindowId | null;
  openWindow: (window: BusinessWindowId) => void;
  /** Lets the route mirror the active window. */
  onWindowChange?: (window: BusinessWindowId | null) => void;
}

/**
 * Two-way link between the active business window and the route's `?view=`.
 *
 * The workspace reports every active-window change so the route can mirror
 * it, and opens a window the route requests (a deep link) once. The mirror
 * comes back as a new route value, and the router may commit it only after
 * the user has acted again (for example pressed Escape). That echo is the
 * workspace's own report, not a new request, so it must never reopen a window
 * the user just closed. Once the route stops requesting a window, a later
 * request for the same window is a new deep link again.
 */
export function useRouteWindowSync({
  requestedWindow,
  activeWindow,
  openWindow,
  onWindowChange,
}: RouteWindowSyncOptions) {
  const appliedRef = useRef<BusinessWindowId | null>(null);

  useEffect(() => {
    // Mark before reporting, so the route's echo of this report is applied.
    if (activeWindow) appliedRef.current = activeWindow;
    onWindowChange?.(activeWindow);
  }, [activeWindow, onWindowChange]);

  useEffect(() => {
    if (!requestedWindow) {
      appliedRef.current = null;
      return;
    }
    if (appliedRef.current === requestedWindow) return;
    appliedRef.current = requestedWindow;
    openWindow(requestedWindow);
  }, [requestedWindow, openWindow]);
}
