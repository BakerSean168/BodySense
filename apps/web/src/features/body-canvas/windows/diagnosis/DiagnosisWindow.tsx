import type { ReactNode } from "react";
import { projectDiagnosisStepper } from "../../model/diagnosisDecisionViewModel";
import { DiagnosisHistoryPanel } from "../../components/DiagnosisHistoryPanel";
import type { DiagnosisWindowProps } from "./types";
import { Banner } from "./DiagnosisPrimitives";
import { ConditionCard } from "./DiagnosisCurrentScene";
import { DecidingFooter, DecidingScene } from "./DiagnosisDecisionScenes";
import {
  EmptyScene,
  LoadingScene,
  NoChangeScene,
  ReceiptScene,
  SafetyScene,
} from "./DiagnosisOutcomeScenes";

/**
 * V2 Diagnosis window: one user-owned aggregate (`GET /diagnosis`), one
 * selected adopted ConditionEntry at a time, plus the latest formal run's
 * Agent proposals, decided one at a time as in the Penpot Diagnosis boards.
 * Proposals are never rendered as conditions and nothing is shown as
 * committed without a settlement receipt.
 */
export function DiagnosisWindow({
  view,
  selectedConditionId = null,
  loading = false,
  error = null,
  partialError = null,
  onRetry,
  controls,
  scene = "current",
}: DiagnosisWindowProps) {
  const { confirmed, run, safetyBlocked, safetyReview } = view;
  const selected =
    confirmed.find(
      (condition) => condition.conditionId === selectedConditionId,
    ) ??
    confirmed[0] ??
    null;
  const activeRun = run && run.proposals.length > 0 ? run : null;
  const stepper =
    activeRun?.phase === "deciding"
      ? projectDiagnosisStepper(activeRun, controls?.selection ?? {})
      : null;

  // A pending workspace safety review only gates an open formal analysis
  // (Penpot Diagnosis / Safety stopped). It never hides the user's own
  // Current/Empty Diagnosis or a settled receipt: those keep their board and
  // carry a short safety note instead.
  const safetyGated =
    safetyBlocked || (safetyReview && activeRun?.phase === "deciding");
  const safetyNote =
    safetyReview && !safetyGated ? (
      <Banner
        tone="bad"
        mark="counter"
        role="alert"
        testId="diagnosis-safety-note"
      >
        需要先处理安全提醒，之后才能请求正式分析
      </Banner>
    ) : null;

  let body: ReactNode;
  let footer: ReactNode = null;
  if (error) {
    body = (
      <Banner tone="bad" mark="counter" role="alert">
        {error}
        {onRetry ? (
          <button type="button" className="bc-text-button" onClick={onRetry}>
            重新读取
          </button>
        ) : null}
      </Banner>
    );
  } else if (loading) {
    body = <LoadingScene />;
  } else if (scene === "history") {
    body = (
      <DiagnosisHistoryPanel
        inactive={view.inactive}
        renderCondition={(condition) => <ConditionCard condition={condition} />}
      />
    );
  } else if (safetyGated) {
    body = <SafetyScene stopped={safetyBlocked} />;
    if (controls?.onOpenSafety)
      footer = (
        <div className="bc-window-footer bc-dx-footer">
          <button
            type="button"
            className="bc-dx-tonal bc-dx-footer-end"
            data-testid="diagnosis-safety-open"
            onClick={controls.onOpenSafety}
          >
            查看限制
          </button>
        </div>
      );
  } else if (activeRun && stepper) {
    body = (
      <DecidingScene run={activeRun} stepper={stepper} controls={controls} />
    );
    if (controls && activeRun.state === "open")
      footer = (
        <DecidingFooter run={activeRun} stepper={stepper} controls={controls} />
      );
  } else if (
    activeRun &&
    (activeRun.phase === "committed" || activeRun.phase === "updated")
  ) {
    body = <ReceiptScene run={activeRun} />;
  } else if (activeRun?.phase === "closed_without_change") {
    body = <NoChangeScene />;
  } else if (selected) {
    body = (
      <div className="bc-condition-list">
        {controls?.error ? (
          <Banner tone="bad" mark="counter" role="alert">
            {controls.error}
          </Banner>
        ) : null}
        <ConditionCard condition={selected} controls={controls} />
      </div>
    );
  } else {
    body = (
      <EmptyScene
        onOpenChat={controls?.onRequestAnalysis ?? controls?.onOpenChat}
      />
    );
  }

  if (
    controls &&
    !error &&
    !loading &&
    scene !== "history" &&
    !safetyGated &&
    activeRun &&
    activeRun.phase !== "deciding"
  ) {
    footer = (
      <div className="bc-window-footer bc-dx-footer">
        <button
          type="button"
          className="bc-dx-tonal"
          onClick={controls.onDismissOutcome}
        >
          {activeRun.phase === "closed_without_change"
            ? "返回当前"
            : "查看当前"}
        </button>
      </div>
    );
  }

  return (
    <section
      className="bc-business-window-body bc-dx"
      aria-label="综合诊断"
      data-testid="diagnosis-window"
    >
      <div className="bc-window-body" role="region" aria-label="当前诊断内容">
        {error || loading || scene === "history" ? null : safetyNote}
        {partialError ? (
          <Banner tone="bad" mark="counter" role="alert">
            {partialError}
            {onRetry ? (
              <button
                type="button"
                className="bc-text-button"
                onClick={onRetry}
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
