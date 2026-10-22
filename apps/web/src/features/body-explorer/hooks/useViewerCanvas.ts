import { useEffect, type RefObject } from "react";

export function useResizeObserver(
  ref: RefObject<HTMLElement | null>,
  onWidth: (width: number) => void,
  enabled = true,
) {
  useEffect(() => {
    if (!enabled) return;
    const element = ref.current;
    if (!element || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width;
      if (typeof width === "number") onWidth(width);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [enabled, onWidth, ref]);
}

export function useCanvasContextRestore(
  ref: RefObject<HTMLElement | null>,
  onRestore: () => void,
  enabled = true,
) {
  useEffect(() => {
    if (!enabled) return;
    const root = ref.current;
    if (!root || typeof MutationObserver === "undefined") return;
    let canvas: HTMLCanvasElement | null = null;

    const detach = () => {
      canvas?.removeEventListener("webglcontextrestored", onRestore);
      canvas = null;
    };
    const attach = () => {
      const next = root.querySelector("canvas");
      if (next === canvas) return;
      detach();
      canvas = next;
      canvas?.addEventListener("webglcontextrestored", onRestore);
    };

    attach();
    const observer = new MutationObserver(attach);
    observer.observe(root, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      detach();
    };
  }, [enabled, onRestore, ref]);
}
