import type { ReactNode } from "react";
import { Check, CircleHelp, Minus, Sparkle, X } from "lucide-react";

type Tone = "ok" | "idle" | "warn" | "bad" | "ai" | "neutral";
type MarkKind = "support" | "context" | "gap" | "counter";

export function Chip({
  tone,
  children,
  testId,
}: {
  tone: Tone;
  children: ReactNode;
  testId?: string;
}) {
  return (
    <span className="bc-dx-chip" data-tone={tone} data-testid={testId}>
      {tone === "ai" ? (
        <Sparkle size={11} aria-hidden="true" fill="currentColor" />
      ) : tone === "neutral" ? null : (
        <span className="bc-dx-chip-dot" aria-hidden="true" />
      )}
      {children}
    </span>
  );
}

const MARK_ICONS: Record<MarkKind, ReactNode> = {
  support: <Check size={12} strokeWidth={2.6} />,
  context: <Minus size={12} strokeWidth={2.6} />,
  gap: <CircleHelp size={14} strokeWidth={2.2} />,
  counter: <X size={12} strokeWidth={2.6} />,
};

export function Mark({ kind }: { kind: MarkKind }) {
  return (
    <span className="bc-dx-mark" data-mark={kind} aria-hidden="true">
      {MARK_ICONS[kind]}
    </span>
  );
}

export function MarkRow({
  kind,
  children,
  meta,
  strong,
}: {
  kind: MarkKind;
  children: ReactNode;
  meta?: string | null;
  strong?: boolean;
}) {
  return (
    <li
      className="bc-dx-row"
      data-mark={kind}
      data-strong={strong || undefined}
    >
      <Mark kind={kind} />
      <span className="bc-dx-row-text">
        <span className="bc-dx-row-statement">{children}</span>
        {meta ? <span className="bc-dx-row-meta">{meta}</span> : null}
      </span>
    </li>
  );
}

export function Section({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <section className="bc-dx-section" aria-label={label}>
      <h4 className="bc-dx-label">{label}</h4>
      <ul className="bc-dx-rows">{children}</ul>
    </section>
  );
}

/** Penpot centred state block: halo mark, title, description. */
export function StateBlock({
  tone,
  icon,
  title,
  description,
  children,
}: {
  tone: "ok" | "idle" | "bad";
  icon: ReactNode;
  title: string;
  description?: string;
  children?: ReactNode;
}) {
  return (
    <div className="bc-dx-state" data-tone={tone}>
      <span className="bc-dx-state-halo" aria-hidden="true">
        {icon}
      </span>
      <p className="bc-dx-state-title">{title}</p>
      {description ? <p className="bc-dx-state-text">{description}</p> : null}
      {children}
    </div>
  );
}

export function Banner({
  tone,
  mark,
  children,
  action,
  role,
  testId,
}: {
  tone: "warn" | "bad" | "info";
  mark: MarkKind;
  children: ReactNode;
  action?: ReactNode;
  role?: "alert" | "status";
  testId?: string;
}) {
  return (
    <div
      className="bc-dx-banner"
      data-tone={tone}
      role={role}
      data-testid={testId}
    >
      <Mark kind={mark} />
      <span className="bc-dx-banner-text">{children}</span>
      {action}
    </div>
  );
}
