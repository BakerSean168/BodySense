import type { ReactNode } from "react";
import { Plus } from "lucide-react";

/** Stable empty snapshot so the workbench can render its own empty state. */
export function PersonEmptyState({
  icon,
  title,
  message,
  actionLabel,
  onAction,
  disabled,
}: {
  icon: ReactNode;
  title: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
  disabled?: boolean;
}) {
  // Penpot "<Category> · Empty": 64px mint icon disc, title, one quiet line
  // and a round 32px primary action (same anatomy as Records · Empty).
  return (
    <div className="bc-person-state bc-person-empty">
      <span className="bc-person-state-icon" aria-hidden="true">
        {icon}
      </span>
      <strong>{title}</strong>
      <span className="bc-person-empty-text">{message}</span>
      {actionLabel && onAction ? (
        <button
          type="button"
          className="bc-person-add bc-person-empty-action"
          aria-label={actionLabel}
          title={actionLabel}
          onClick={onAction}
          disabled={disabled}
        >
          <Plus size={17} />
        </button>
      ) : null}
    </div>
  );
}
