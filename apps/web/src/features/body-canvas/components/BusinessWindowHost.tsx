import {
  useCallback,
  useEffect,
  useRef,
  type ReactNode,
} from "react";
import { Minus, X } from "lucide-react";
import {
  BUSINESS_WINDOW_LAYOUT,
  desktopWindowOffset,
  getBusinessWindowMeta,
  type BusinessWindowId,
} from "../model/businessWindows";

/**
 * Shared shell for every business window.
 *
 * Design contract across Diagnosis / TrainingPlan / Chat (Penpot V5.5
 * `BUSINESS_WINDOWS_IMPLEMENTATION.md`):
 * - Desktop: only a level-1 business title on the left and a Close control on
 *   the right. Title starts at x=20 / y=17, 18px 700, divider at y=63.
 * - Mobile: keep the Sheet drag handle; title at x=18 / y=29, 17px 700,
 *   divider at y=74.
 * - No decorative business icon, no second-line subtitle, no duplicated title
 *   in the body.
 *
 * Positioning: the prototype hardcoded x=430/440 so its 878px Viewer had no
 * clipping. Production must adapt to the real stage, so the shell applies the
 * preferred Penpot offset and lets CSS clamp the window inside the stage.
 */

export interface BusinessWindowHostProps {
  windowId: BusinessWindowId;
  minimized?: boolean;
  /** Index within the open stack, used only for the cascade offset. */
  stackIndex: number;
  onClose: () => void;
  onMinimize?: () => void;
  onFocus?: () => void;
  isActive?: boolean;
  /**
   * Replaces the level-1 title on the left of the header. Person uses this for
   * its avatar + nickname trigger; every other window keeps the plain title.
   */
  headerLeft?: ReactNode;
  /** The custom headerLeft already conveys identity, so no title is repeated. */
  hideTitle?: boolean;
  children: ReactNode;
}

export function BusinessWindowHost({
  windowId,
  minimized = false,
  stackIndex,
  onClose,
  onMinimize,
  onFocus,
  isActive = false,
  headerLeft,
  hideTitle = false,
  children,
}: BusinessWindowHostProps) {
  const meta = getBusinessWindowMeta(windowId);
  const layout = BUSINESS_WINDOW_LAYOUT[windowId];
  const offset = desktopWindowOffset(stackIndex);
  const panelRef = useRef<HTMLDivElement>(null);
  const headingId = `bc-window-title-${windowId}`;

  const handlePointerDown = useCallback(() => {
    onFocus?.();
  }, [onFocus]);

  // A minimized window keeps its identity and content mounted so switching back
  // does not lose the user's place; it is only removed from the visual stage.
  useEffect(() => {
    if (minimized) return;
  }, [minimized]);

  return (
    <div
      ref={panelRef}
      className="bc-business-window"
      data-window={windowId}
      data-active={isActive ? "true" : "false"}
      data-minimized={minimized ? "true" : "false"}
      hidden={minimized}
      role="dialog"
      aria-modal="false"
      aria-labelledby={headingId}
      onPointerDown={handlePointerDown}
      style={
        {
          "--bc-window-width": `${layout.desktop.width}px`,
          "--bc-window-height": `${layout.desktop.height}px`,
          "--bc-window-max-height": `${layout.maxHeight}px`,
          "--bc-window-left": `${offset.left}px`,
          "--bc-window-top": `${offset.top}px`,
        } as React.CSSProperties
      }
    >
      <header className="bc-business-window-header">
        <span className="bc-sheet-handle" aria-hidden="true" />
        {headerLeft ?? (
          <h2 id={headingId} className="bc-business-window-title">
            {meta.title}
          </h2>
        )}
        {hideTitle ? (
          <span id={headingId} className="sr-only">
            {meta.title}
          </span>
        ) : null}
        <div className="bc-business-window-controls">
          {onMinimize ? (
            <button
              type="button"
              className="bc-icon-button"
              aria-label={`最小化${meta.title}`}
              onClick={onMinimize}
            >
              <Minus size={18} />
            </button>
          ) : null}
          <button
            type="button"
            className="bc-icon-button"
            aria-label={`关闭${meta.title}`}
            onClick={onClose}
          >
            <X size={18} />
          </button>
        </div>
      </header>
      <div className="bc-business-window-content">{children}</div>
    </div>
  );
}

/**
 * Mobile presents the same business semantics as a bottom sheet. The host keeps
 * the drag handle and a fixed input area outside this component so the task
 * state, focus and scroll position survive an Esc/Close.
 */
export function BusinessSheetHost({
  windowId,
  hidden = false,
  onClose,
  headerLeft,
  hideTitle = false,
  children,
  footer,
}: {
  windowId: BusinessWindowId;
  /** Kept mounted but off-screen so closing a sheet preserves its task state. */
  hidden?: boolean;
  onClose: () => void;
  headerLeft?: ReactNode;
  hideTitle?: boolean;
  children: ReactNode;
  footer?: ReactNode;
}) {
  const meta = getBusinessWindowMeta(windowId);
  const headingId = `bc-sheet-title-${windowId}`;
  return (
    <section
      className="bc-business-sheet"
      data-window={windowId}
      data-hidden={hidden ? "true" : "false"}
      hidden={hidden}
      role="dialog"
      aria-modal="false"
      aria-labelledby={headingId}
    >
      <header className="bc-business-window-header">
        <span className="bc-sheet-handle" aria-hidden="true" />
        {headerLeft ?? (
          <h2 id={headingId} className="bc-business-window-title">
            {meta.title}
          </h2>
        )}
        {hideTitle ? (
          <span id={headingId} className="sr-only">
            {meta.title}
          </span>
        ) : null}
        <div className="bc-business-window-controls">
          <button
            type="button"
            className="bc-icon-button"
            aria-label={`关闭${meta.title}`}
            onClick={onClose}
          >
            <X size={18} />
          </button>
        </div>
      </header>
      <div className="bc-business-sheet-content">{children}</div>
      {footer ? <div className="bc-business-sheet-footer">{footer}</div> : null}
    </section>
  );
}
