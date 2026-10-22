import * as React from "react";

/**
 * Popups are portaled. Inside the body canvas we portal into the nearest
 * `.body-canvas-theme` element so the popup keeps the theme tokens (paper,
 * dark mode); anywhere else Base UI falls back to document.body.
 */
export type PortalScope = React.RefObject<HTMLElement | null>;

export const PortalScopeContext = React.createContext<PortalScope | null>(null);

export function usePortalScopeRef(): PortalScope {
  return React.useRef<HTMLElement | null>(null);
}

/** Ref callback for a trigger: records the themed ancestor it lives in. */
export function useScopeTriggerRef<T extends HTMLElement>(
  forwarded?: React.Ref<T>,
) {
  const scope = React.useContext(PortalScopeContext);
  return React.useCallback(
    (node: T | null) => {
      if (scope && node) {
        scope.current = node.closest<HTMLElement>(".body-canvas-theme");
      }
      if (typeof forwarded === "function") forwarded(node);
      else if (forwarded) {
        (forwarded as React.RefObject<T | null>).current = node;
      }
    },
    [scope, forwarded],
  );
}

export function usePortalContainer(): PortalScope | undefined {
  return React.useContext(PortalScopeContext) ?? undefined;
}
