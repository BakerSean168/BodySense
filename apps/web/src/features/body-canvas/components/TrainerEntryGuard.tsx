import { AlertTriangle, ArrowRight } from "lucide-react";
import { TrainingExecutionPanel } from "@/features/training/components/TrainingExecutionPanel";

/**
 * A Trainer window can be opened by deep link or keyboard, but opening a
 * window is NOT equivalent to authorizing an executable training session.
 *
 * All four gates must hold before mounting the legacy execution panel:
 * 1. The selected plan is the plan the user explicitly launched in this page.
 * 2. The authoritative projection marks that revision accepted.
 * 3. The server exposes execution capability.
 * 4. There is no blocking safety hold.
 *
 * This is a UI admission guard, never a substitute for server authorization.
 */
export interface TrainerEntryGuardProps {
  planId: string | null;
  launchedPlanId: string | null;
  accepted: boolean;
  canExecute: boolean;
  safetyBlocked: boolean;
  loading: boolean;
  error: string | null;
  onOpenTraining: () => void;
}

export function TrainerEntryGuard({
  planId,
  launchedPlanId,
  accepted,
  canExecute,
  safetyBlocked,
  loading,
  error,
  onOpenTraining,
}: TrainerEntryGuardProps) {
  const canRenderExecution =
    planId !== null &&
    launchedPlanId === planId &&
    accepted &&
    canExecute &&
    !safetyBlocked &&
    !loading &&
    !error;

  let explanation =
    "请先在训练计划中选择一份已接受的方案，再点击「开始训练」。";
  if (loading) explanation = "正在核对当前训练计划及执行权限。";
  else if (error) explanation = "训练计划暂时无法读取，不能进入训练执行。";
  else if (safetyBlocked)
    explanation = "当前安全状态不允许开始或继续训练。请先处理安全提醒。";
  else if (!planId || !accepted)
    explanation = "当前没有可执行的已接受训练计划。建议先在对话中审阅方案。";
  else if (!canExecute)
    explanation = "当前服务端未授予训练执行权限。请检查计划或安全状态。";

  return (
    <div
      className="bc-window-body bc-trainer-content"
      data-testid="trainer-window"
    >
      {canRenderExecution ? (
        <TrainingExecutionPanel key={planId} planId={planId} />
      ) : (
        <div
          className="bc-window-empty bc-trainer-guard"
          data-testid="trainer-entry-guard"
          role={safetyBlocked ? "alert" : "status"}
        >
          {safetyBlocked || error ? (
            <AlertTriangle size={24} aria-hidden="true" />
          ) : null}
          <p className="bc-window-empty-title">训练执行尚未就绪</p>
          <p className="bc-window-empty-text">{explanation}</p>
          <button
            type="button"
            className="bc-button bc-primary"
            onClick={onOpenTraining}
          >
            查看训练计划 <ArrowRight size={15} aria-hidden="true" />
          </button>
        </div>
      )}
    </div>
  );
}
