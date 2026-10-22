import { useCallback, useEffect, useRef, useState } from "react";

/** Size the mobile safety offset and desktop Dock reservation from real DOM. */
export function useCanvasStageLayout() {
  // A mobile sheet starts below the safety banner instead of sliding under it,
  // so the banner's real height (it can wrap) is reserved in the sheet layout.
  const [safetyBannerHeight, setSafetyBannerHeight] = useState<number | null>(
    null,
  );
  const safetyBannerObserver = useRef<ResizeObserver | null>(null);
  const measureSafetyBanner = useCallback((node: HTMLDivElement | null) => {
    safetyBannerObserver.current?.disconnect();
    safetyBannerObserver.current = null;
    if (!node) {
      setSafetyBannerHeight(null);
      return;
    }
    const update = () => setSafetyBannerHeight(node.offsetHeight || null);
    update();
    if (typeof ResizeObserver === "undefined") return;
    safetyBannerObserver.current = new ResizeObserver(update);
    safetyBannerObserver.current.observe(node);
  }, []);
  // A desktop window ends above the Dock. The Penpot frame keeps 108px for a
  // 72px Dock, which the desktop Dock matches; a taller Dock (narrow desktop
  // widths, larger text) is measured and its space reserved too.
  const stageRef = useRef<HTMLElement>(null);
  const [dockReserve, setDockReserve] = useState<number | null>(null);
  useEffect(() => {
    const stage = stageRef.current;
    const dock = stage?.querySelector<HTMLElement>(".bc-dock");
    if (!stage || !dock) return;
    const update = () =>
      setDockReserve(Math.max(0, stage.clientHeight - dock.offsetTop) || null);
    update();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(update);
    observer.observe(stage);
    observer.observe(dock);
    return () => observer.disconnect();
  }, []);
  return { stageRef, safetyBannerHeight, dockReserve, measureSafetyBanner };
}
