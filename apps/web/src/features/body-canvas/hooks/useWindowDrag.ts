import { useCallback, useEffect, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent, RefObject } from "react";

const MARGIN = 8;
// Pointer-downs on controls (and on popups such as the category listbox that
// render inside the header) never start a drag.
const INTERACTIVE = [
  "button",
  "a",
  "input",
  "select",
  "textarea",
  "label",
  "summary",
  "[role=button]",
  "[role=menu]",
  "[role=menuitem]",
  "[role=listbox]",
  "[role=option]",
  "[role=combobox]",
  "[role=tab]",
  "[role=dialog]",
  "[contenteditable=true]",
].join(", ");

/**
 * Header drag for a floating desktop window. The docked CSS position stays the
 * base; the drag only adds a `translate` offset, clamped so the window never
 * leaves its positioned parent (the stage). Disabled when `enabled` is false.
 */
export function useWindowDrag(
  ref: RefObject<HTMLElement | null>,
  enabled: boolean,
  /** Space kept free above the stage bottom (the Dock). */
  bottomReserve = MARGIN,
) {
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const drag = useRef<{
    startX: number;
    startY: number;
    base: { x: number; y: number };
    rect: DOMRect;
    bounds: DOMRect;
  } | null>(null);

  const clamp = useCallback(
    (
      x: number,
      y: number,
      rect: DOMRect,
      bounds: DOMRect,
      base: { x: number; y: number },
    ) => {
      // rect is the window box at offset `base`; compute the allowed offset range.
      const minX = base.x + (bounds.left + MARGIN - rect.left);
      const maxX = base.x + (bounds.right - MARGIN - rect.right);
      const minY = base.y + (bounds.top + MARGIN - rect.top);
      const maxY = base.y + (bounds.bottom - bottomReserve - rect.bottom);
      return {
        x: Math.min(Math.max(x, minX), Math.max(minX, maxX)),
        y: Math.min(Math.max(y, minY), Math.max(minY, maxY)),
      };
    },
    [bottomReserve],
  );

  const boundsOf = (el: HTMLElement) =>
    (el.offsetParent as HTMLElement | null)?.getBoundingClientRect() ??
    new DOMRect(0, 0, window.innerWidth, window.innerHeight);

  const onPointerDown = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      const el = ref.current;
      if (!enabled || !el || event.button !== 0) return;
      if ((event.target as HTMLElement).closest(INTERACTIVE)) return;
      drag.current = {
        startX: event.clientX,
        startY: event.clientY,
        base: offset,
        rect: el.getBoundingClientRect(),
        bounds: boundsOf(el),
      };
      event.currentTarget.setPointerCapture(event.pointerId);
      event.preventDefault();
    },
    [enabled, offset, ref],
  );

  const onPointerMove = useCallback(
    (event: ReactPointerEvent<HTMLElement>) => {
      const d = drag.current;
      if (!d) return;
      setOffset(
        clamp(
          d.base.x + event.clientX - d.startX,
          d.base.y + event.clientY - d.startY,
          d.rect,
          d.bounds,
          d.base,
        ),
      );
    },
    [clamp],
  );

  const onPointerUp = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    if (!drag.current) return;
    drag.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  }, []);

  // Keep a dragged window inside the stage when the viewport shrinks.
  useEffect(() => {
    if (!enabled) return;
    const onResize = () => {
      const el = ref.current;
      if (!el || el.hidden) return;
      setOffset((current) =>
        clamp(
          current.x,
          current.y,
          el.getBoundingClientRect(),
          boundsOf(el),
          current,
        ),
      );
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, [clamp, enabled, ref]);

  useEffect(() => {
    if (!enabled) setOffset({ x: 0, y: 0 });
  }, [enabled]);

  return {
    offset,
    handleProps: enabled
      ? {
          onPointerDown,
          onPointerMove,
          onPointerUp,
          onPointerCancel: onPointerUp,
        }
      : {},
  };
}
