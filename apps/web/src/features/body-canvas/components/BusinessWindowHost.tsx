import type { ReactNode } from "react";
import { X } from "lucide-react";
import {
  BUSINESS_WINDOW_LAYOUT,
  desktopWindowOffset,
  getBusinessWindowMeta,
  type BusinessWindowId,
} from "../model/businessWindows";

/**
 * Shared shell for every business window.
 *
 * Penpot V2: the persistent 3D Body Canvas stays mounted behind one active
 * floating window. Diagnosis and Training supply a compact business selector
 * in the header with a separate chat affordance; Person supplies its identity
 * popover trigger, and Chat supplies the local clock. All windows share Close,
 * the same scrollable content boundary, and viewport-clamped positioning.
 */

export interface BusinessWindowHostProps {
  windowId: BusinessWindowId;
  minimized?: boolean;
  /** Index within the open stack, used only for the cascade offset. */
  stackIndex: number;
  onClose: () => void;
  isActive?: boolean;
  /**
   * Replaces the level-1 title on the left of the header. Person uses this for
   * its avatar + nickname trigger; every other window keeps the plain title.
   */
  headerLeft?: ReactNode;
  /** Contextual actions appear before Close, without duplicating footer commands. */
  headerActions?: ReactNode;
  /** The custom headerLeft already conveys identity, so no title is repeated. */
  hideTitle?: boolean;
  children: ReactNode;
}

export function BusinessWindowHost({
  windowId,
  minimized = false,
  stackIndex,
  onClose,
  isActive = false,
  headerLeft,
  headerActions,
  hideTitle = false,
  children,
}: BusinessWindowHostProps) {
  const meta = getBusinessWindowMeta(windowId);
  const layout = BUSINESS_WINDOW_LAYOUT[windowId];
  const offset = desktopWindowOffset(stackIndex);
  const headingId = `bc-window-title-${windowId}`;

  return (
    <div
      className="bc-business-window"
      data-window={windowId}
      data-active={isActive ? "true" : "false"}
      data-minimized={minimized ? "true" : "false"}
      hidden={minimized}
      /*
       * A non-modal, persistent floating window is a landmark rather than a
       * dialog: several can be open, they never trap focus and they stay
       * mounted while folded. Using role="region" also keeps genuine modal
       * overlays (anatomy, shortcuts) unambiguous for assistive technology.
       */
      role="region"
      aria-labelledby={hideTitle ? undefined : headingId}
      aria-label={hideTitle ? meta.title : undefined}
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
        {headerLeft ?? (
          <h2 id={headingId} className="bc-business-window-title">
            {meta.title}
          </h2>
        )}
        <div className="bc-business-window-controls">
          {headerActions}
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
 * Mobile presents the same business semantics as a bounded floating overlay.
 * The bottom Dock remains visible, and a task stays mounted but hidden on close
 * or a Dock switch so its scroll position and incomplete input survive.
 */
export function BusinessSheetHost({
  windowId,
  hidden = false,
  onClose,
  headerLeft,
  headerActions,
  hideTitle = false,
  children,
  footer,
}: {
  windowId: BusinessWindowId;
  /** Kept mounted but off-screen so closing a sheet preserves its task state. */
  hidden?: boolean;
  onClose: () => void;
  headerLeft?: ReactNode;
  headerActions?: ReactNode;
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
      aria-labelledby={hideTitle ? undefined : headingId}
      aria-label={hideTitle ? meta.title : undefined}
    >
      <header className="bc-business-window-header">
        {headerLeft ?? (
          <h2 id={headingId} className="bc-business-window-title">
            {meta.title}
          </h2>
        )}
        <div className="bc-business-window-controls">
          {headerActions}
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
