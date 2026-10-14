import { Dumbbell, FileText, MessageCircle, UserRound } from "lucide-react";
import {
  getBusinessWindowMeta,
  type BusinessWindowId,
} from "../model/businessWindows";

/** The four Penpot V2 Dock entrances. Trainer is opened from Training, not a fifth Dock item. */
export const BODY_CANVAS_DOCK_ITEMS = [
  { id: "person", icon: UserRound },
  { id: "training", icon: Dumbbell },
  { id: "diagnosis", icon: FileText },
  { id: "chat", icon: MessageCircle },
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

  return (
    <nav className="bc-dock" aria-label="身体工作区导航">
      {BODY_CANVAS_DOCK_ITEMS.map(({ id, icon: Icon }) => {
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
              <Icon size={21} strokeWidth={1.8} aria-hidden="true" />
              {id === "diagnosis" && diagnosisCount > 0 ? (
                <span className="bc-dock-count" aria-hidden="true">
                  {diagnosisCount > 99 ? "99+" : diagnosisCount}
                </span>
              ) : null}
            </span>
            <span className="bc-dock-label">{dockLabel}</span>
            <kbd aria-hidden="true">{shortcut}</kbd>
          </button>
        );
      })}
    </nav>
  );
}
