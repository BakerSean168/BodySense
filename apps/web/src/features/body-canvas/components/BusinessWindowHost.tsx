import type { ReactNode } from "react";
import { useBusinessWindowFocus } from "../hooks/useBusinessWindowFocus";
import { X } from "lucide-react";
import {
  BUSINESS_WINDOW_LAYOUT,
  DESKTOP_WINDOW_BOTTOM_RESERVE,
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
 * the same scrollable content boundary, and viewport-clamped positioning:
 * on desktop each docks to its Penpot side (`data-dock`) so the body stays
 * visible beside it.
 */

export interface BusinessWindowHostProps {
  windowId: BusinessWindowId;
  minimized?: boolean;
  /** Responsive presentation changes without remounting a live task. */
  mobile?: boolean;
  /** Mobile-only action footer; the task content stays mounted on resize. */
  footer?: ReactNode;
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
  mobile = false,
  footer,
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
  const hostRef = useBusinessWindowFocus(windowId, isActive, minimized);

  return (
    <div
      ref={hostRef}
      className={mobile ? "bc-business-sheet" : "bc-business-window"}
      data-window={windowId}
      data-dock={mobile ? undefined : layout.dock}
      data-active={isActive ? "true" : "false"}
      data-minimized={minimized ? "true" : "false"}
      data-hidden={mobile ? (minimized ? "true" : "false") : undefined}
      hidden={minimized}
      tabIndex={isActive && !minimized ? -1 : undefined}
      role={mobile ? "dialog" : "region"}
      aria-modal={mobile ? "false" : undefined}
      aria-labelledby={hideTitle ? undefined : headingId}
      aria-label={hideTitle ? meta.title : undefined}
      style={
        {
          "--bc-window-width": `${layout.desktop.width}px`,
          "--bc-window-height": `${layout.desktop.height}px`,
          "--bc-window-max-height": `${layout.maxHeight}px`,
          "--bc-window-left": `${offset.left}px`,
          "--bc-window-top": `${offset.top}px`,
          "--bc-window-bottom": `${DESKTOP_WINDOW_BOTTOM_RESERVE}px`,
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
      <div
        className={
          mobile ? "bc-business-sheet-content" : "bc-business-window-content"
        }
      >
        {children}
      </div>
      {mobile && footer ? (
        <div className="bc-business-sheet-footer">{footer}</div>
      ) : null}
    </div>
  );
}
