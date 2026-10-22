import type { ReactNode } from "react";
import type { TrainingTone } from "../../model/trainingPlans";

export function Chip({
  tone,
  children,
  dot = false,
  icon,
  testId,
}: {
  tone: TrainingTone;
  children: ReactNode;
  dot?: boolean;
  icon?: ReactNode;
  testId?: string;
}) {
  return (
    <span className="bc-dx-chip" data-tone={tone} data-testid={testId}>
      {dot ? <span className="bc-dx-chip-dot" aria-hidden="true" /> : null}
      {icon}
      {children}
    </span>
  );
}

export function Banner({
  tone,
  icon,
  children,
  action,
  role,
  testId,
}: {
  tone: "warn" | "bad" | "info" | "ok";
  icon?: ReactNode;
  children: ReactNode;
  action?: ReactNode;
  role?: "alert" | "status";
  testId?: string;
}) {
  return (
    <div
      className="bc-dx-banner bc-tp-banner"
      data-tone={tone}
      role={role}
      data-testid={testId}
    >
      {icon}
      <span className="bc-dx-banner-text">{children}</span>
      {action}
    </div>
  );
}
