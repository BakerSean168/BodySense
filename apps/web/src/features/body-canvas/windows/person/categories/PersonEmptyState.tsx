import type { ReactNode } from "react";

/**
 * Penpot "<Category> · Empty": 64px mint icon disc, title and one short line.
 * Adding lives in the window header "+", never in the empty state.
 */
export function PersonEmptyState({
  icon,
  title,
  message,
}: {
  icon: ReactNode;
  title: string;
  message?: string;
}) {
  return (
    <div className="bc-person-state bc-person-empty">
      <span className="bc-person-state-icon" aria-hidden="true">
        {icon}
      </span>
      <strong>{title}</strong>
      {message ? <span className="bc-person-empty-text">{message}</span> : null}
    </div>
  );
}
