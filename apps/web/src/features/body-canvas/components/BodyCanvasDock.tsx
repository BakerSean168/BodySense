import { useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  DockChatIcon,
  DockDiagnosisIcon,
  DockPersonIcon,
  DockTrainingIcon,
} from "./DockIcons";
import {
  getBusinessWindowMeta,
  type BusinessWindowId,
} from "../model/businessWindows";

/**
 * The four Penpot V2 Dock entrances in the desktop board order (/ D P M).
 * The mobile board mirrors it (个人 训练 诊断 对话), so the thumb reaches Person
 * first. Trainer is opened from Training, not a fifth Dock item.
 */
export const BODY_CANVAS_DOCK_ITEMS = [
  { id: "chat", icon: DockChatIcon },
  { id: "diagnosis", icon: DockDiagnosisIcon },
  { id: "training", icon: DockTrainingIcon },
  { id: "person", icon: DockPersonIcon },
] as const;

export interface BodyCanvasDockProps {
  activeWindow: BusinessWindowId | null;
  confirmedDiagnosisCount: number;
  onToggle: (window: BusinessWindowId) => void;
}

export function BodyCanvasDock({
  activeWindow,
  confirmedDiagnosisCount,
  onToggle,
}: BodyCanvasDockProps) {
  // Count only adopted/confirmed conditions, never speculative AI proposals.
  const diagnosisCount =
    Number.isFinite(confirmedDiagnosisCount) && confirmedDiagnosisCount > 0
      ? Math.floor(confirmedDiagnosisCount)
      : 0;

  // Penpot "Desktop / Home · Dock collapsed": the chevron folds the Dock to a
  // small pill; window shortcuts keep working. Mobile always shows the Dock.
  const mobile = useMediaQuery("(max-width: 760px)");
  const [collapsedState, setCollapsed] = useState(false);
  const collapsed = collapsedState && !mobile;
  const toggleRef = useRef<HTMLButtonElement>(null);
  const toggled = useRef(false);
  useEffect(() => {
    if (!toggled.current) return;
    toggled.current = false;
    toggleRef.current?.focus();
  }, [collapsed]);
  const toggle = () => {
    toggled.current = true;
    setCollapsed((value) => !value);
  };

  if (collapsed) {
    return (
      <nav
        className="bc-dock"
        data-collapsed="true"
        aria-label="身体工作区导航"
      >
        <button
          ref={toggleRef}
          type="button"
          className="bc-dock-toggle"
          aria-label="展开导航"
          title="展开导航"
          aria-expanded={false}
          onClick={toggle}
        >
          <ChevronUp size={18} strokeWidth={2.2} aria-hidden="true" />
        </button>
      </nav>
    );
  }

  return (
    <nav className="bc-dock" aria-label="身体工作区导航">
      {(mobile
        ? [...BODY_CANVAS_DOCK_ITEMS].reverse()
        : BODY_CANVAS_DOCK_ITEMS
      ).map(({ id, icon: Icon }) => {
        const { title, dockLabel, key: shortcut } = getBusinessWindowMeta(id);
        return (
          <button
            key={id}
            type="button"
            className={`bc-dock-item bc-dock-${id}`}
            aria-label={
              id === "diagnosis" && diagnosisCount > 0
                ? `${title}，${diagnosisCount} 项已确认病情`
                : title
            }
            aria-pressed={activeWindow === id}
            title={`${title} · ${shortcut}`}
            onClick={() => onToggle(id)}
          >
            <span className="bc-dock-icon">
              <Icon size={24} />
              {id === "diagnosis" && diagnosisCount > 0 ? (
                <span className="bc-dock-count" aria-hidden="true">
                  {diagnosisCount > 99 ? "99+" : diagnosisCount}
                </span>
              ) : null}
            </span>
            {/* Desktop shows the Penpot icon-only Dock: the label stays in the
                DOM (visually hidden), with aria-label and the title tooltip. */}
            <span className="bc-dock-label">{dockLabel}</span>
            <kbd aria-hidden="true">{shortcut}</kbd>
          </button>
        );
      })}
      <button
        ref={toggleRef}
        type="button"
        className="bc-dock-toggle"
        aria-label="收起导航"
        title="收起导航"
        aria-expanded={true}
        onClick={toggle}
      >
        <ChevronDown size={18} strokeWidth={2.2} aria-hidden="true" />
      </button>
    </nav>
  );
}
