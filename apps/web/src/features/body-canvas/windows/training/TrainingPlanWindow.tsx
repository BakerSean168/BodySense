import { useId, useMemo, type ReactNode } from "react";
import {
  Check,
  LoaderCircle,
  MessageCircle,
  Play,
  ShieldAlert,
  ShieldCheck,
  X,
} from "lucide-react";
import { WindowLoading } from "../../components/WindowLoading";
import { TrainingIcon } from "../../components/TrainingIcon";
import { projectTrainingWindow } from "../../model/trainingPlans";
import type { TrainingPlanWindowProps } from "./types";
import { Chip, Banner } from "./TrainingPrimitives";
import { PlanScene } from "./TrainingPlanScene";
import { ProposalScene } from "./TrainingProposalScene";

export function TrainingPlanWindow({
  state,
  partialError = null,
  controls,
  canExecute,
}: TrainingPlanWindowProps) {
  const gateReasonId = useId();
  const view = useMemo(() => projectTrainingWindow(state), [state]);
  const plan = view.selected;
  let body: ReactNode;
  let footer: ReactNode = null;

  switch (view.scene) {
    case "error":
      body = (
        <Banner
          tone="bad"
          icon={<X size={16} aria-hidden="true" />}
          role="alert"
        >
          训练计划暂时无法读取。
          {controls.onRetry ? (
            <button
              type="button"
              className="bc-text-button"
              onClick={controls.onRetry}
            >
              重新读取
            </button>
          ) : null}
        </Banner>
      );
      break;
    case "loading":
    case "updating":
      body = (
        <WindowLoading
          label={
            view.scene === "updating"
              ? "正在读取新的计划修订"
              : "正在读取训练计划"
          }
        />
      );
      break;
    case "empty":
      body = (
        <div className="bc-dx-outcome bc-tp-empty" data-testid="training-empty">
          <div className="bc-dx-state">
            <span className="bc-dx-state-halo" aria-hidden="true">
              <TrainingIcon size={22} />
            </span>
            <p className="bc-dx-state-title">还没有训练计划</p>
            <p className="bc-dx-state-text">
              确认诊断后，可在对话里生成训练建议
            </p>
            <button
              type="button"
              className="bc-dx-tonal bc-tp-chat"
              onClick={controls.onOpenChat}
            >
              去对话
            </button>
          </div>
        </div>
      );
      break;
    case "proposal":
    case "expired": {
      const proposal = view.proposal!;
      body = <ProposalScene proposal={proposal} controls={controls} />;
      footer =
        view.scene === "proposal" ? (
          <footer
            className="bc-window-footer bc-dx-footer"
            data-testid="training-proposal-decide"
          >
            {plan ? (
              <button
                type="button"
                className="bc-dx-link bc-dx-early"
                onClick={controls.onBackToPlan}
              >
                返回计划
              </button>
            ) : null}
            <button
              type="button"
              className="bc-dx-tonal"
              disabled={controls.busy}
              onClick={() => controls.onReject(proposal.proposalId)}
            >
              拒绝
            </button>
            <button
              type="button"
              className="bc-dx-primary"
              disabled={controls.busy}
              aria-busy={controls.busy || undefined}
              onClick={() => controls.onAccept(proposal.proposalId)}
            >
              {controls.busy ? (
                <LoaderCircle
                  size={16}
                  className="animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <Check size={16} aria-hidden="true" />
              )}
              接受
            </button>
          </footer>
        ) : (
          <footer className="bc-window-footer bc-dx-footer">
            <button
              type="button"
              className="bc-dx-link bc-dx-early"
              onClick={controls.onOpenChat}
            >
              <MessageCircle size={16} aria-hidden="true" />
              重新讨论
            </button>
            <button
              type="button"
              className="bc-dx-tonal"
              onClick={controls.onBackToPlan}
            >
              返回计划
            </button>
          </footer>
        );
      break;
    }
    case "plan":
    case "updated": {
      body = (
        <PlanScene
          view={view}
          plan={plan!}
          controls={controls}
          gateReasonId={gateReasonId}
          updated={view.scene === "updated"}
        />
      );
      const template = plan!.selectedTemplate;
      const startable =
        plan!.canStart && canExecute && template !== null && !partialError;
      footer = (
        <footer
          className="bc-window-footer bc-dx-footer bc-tp-footer"
          data-testid="training-start-footer"
        >
          <Chip
            tone={plan!.status !== "active" ? "idle" : plan!.gate.tone}
            icon={
              plan!.gate.permits ? (
                <ShieldCheck size={13} aria-hidden="true" />
              ) : (
                <ShieldAlert size={13} aria-hidden="true" />
              )
            }
            testId="training-gate"
          >
            {plan!.status !== "active"
              ? `计划${plan!.statusLabel}`
              : plan!.gate.label}
          </Chip>
          <button
            type="button"
            className="bc-dx-primary bc-tp-start"
            aria-label="开始训练"
            aria-describedby={
              plan!.gate.reasons.length ? gateReasonId : undefined
            }
            disabled={!startable}
            onClick={() => {
              if (startable)
                controls.onStart(plan!.planId, template!.templateId);
            }}
          >
            <Play size={16} aria-hidden="true" />
            开始
          </button>
        </footer>
      );
      break;
    }
  }

  return (
    <section
      className="bc-business-window-body bc-dx bc-tp"
      aria-label="训练计划"
      data-testid="training-window"
      data-scene={view.scene}
    >
      <div className="bc-window-body" role="region" aria-label="当前计划内容">
        {partialError ? (
          <Banner tone="bad" icon={<X size={16} />} role="alert">
            {partialError}
            {controls.onRetry ? (
              <button
                type="button"
                className="bc-text-button"
                onClick={controls.onRetry}
              >
                重新读取
              </button>
            ) : null}
          </Banner>
        ) : null}
        {body}
      </div>
      {footer}
    </section>
  );
}
