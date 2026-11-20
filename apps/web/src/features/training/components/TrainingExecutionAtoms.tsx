import type { ReactNode } from "react";
import { CircleHelp, ShieldAlert, ShieldCheck, X } from "lucide-react";
import type { TrainingGateView } from "@/features/body-canvas/model/trainingPlans";
import type { TrainerNotice } from "../hooks/useTrainerSession";

export function Chip({
  tone,
  children,
  dot,
  icon,
  testId,
}: {
  tone: string;
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
  role,
  testId,
}: {
  tone: string;
  icon?: ReactNode;
  children: ReactNode;
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
    </div>
  );
}

export function GateChip({ gate }: { gate: TrainingGateView }) {
  return (
    <Chip
      tone={gate.tone}
      icon={
        gate.permits ? (
          <ShieldCheck size={12} aria-hidden="true" />
        ) : (
          <ShieldAlert size={12} aria-hidden="true" />
        )
      }
      testId="trainer-gate"
    >
      {gate.permits ? "可以继续" : "暂不能继续"}
    </Chip>
  );
}

/** Why the last command did not count, and the gate's own reasons. */
export function Notices({
  notice,
  error,
  gate,
}: {
  notice: TrainerNotice;
  error: string | null;
  gate: TrainingGateView | null;
}) {
  return (
    <>
      {notice === "safety_changed" ? (
        <Banner
          tone="warn"
          icon={<ShieldAlert size={16} aria-hidden="true" />}
          role="alert"
          testId="trainer-safety-changed"
        >
          安全状态刚刚更新，这一步没有记录。请看一下当前提醒，再确认一次。
        </Banner>
      ) : null}
      {notice === "changed" ? (
        <Banner
          tone="info"
          icon={<CircleHelp size={16} aria-hidden="true" />}
          role="status"
          testId="trainer-changed"
        >
          训练记录已在别处更新，已重新读取。
        </Banner>
      ) : null}
      {error ? (
        <Banner
          tone="bad"
          icon={<X size={16} aria-hidden="true" />}
          role="alert"
        >
          {error}
        </Banner>
      ) : null}
      {gate && gate.reasons.length > 0 ? (
        <Banner
          tone={gate.permits ? "warn" : "bad"}
          icon={<ShieldAlert size={16} aria-hidden="true" />}
          role={gate.permits ? "status" : "alert"}
          testId="trainer-gate-reason"
        >
          <span id="trainer-gate-reason">
            {gate.permits ? "可以继续，但请留意：" : "安全检查不允许继续："}
            {gate.reasons.join("；")}
          </span>
        </Banner>
      ) : null}
    </>
  );
}
