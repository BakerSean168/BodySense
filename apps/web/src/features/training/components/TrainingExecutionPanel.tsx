import { useState, type ReactNode } from "react";
import { Check, LoaderCircle, Play, X } from "lucide-react";
import { useTrainerSession } from "../hooks/useTrainerSession";
import { findReceipt } from "../model/trainingExecutionLabels";
import { Banner, Notices } from "./TrainingExecutionAtoms";
import {
  CorrectionConfirm,
  InProgressFooter,
  InProgressScene,
} from "./TrainingExecutionInProgress";
import {
  EndedScene,
  FeedbackForm,
  RecoveryScene,
} from "./TrainingExecutionScenes";

export interface TrainingExecutionPanelProps {
  planId: string;
  /** The template the user started from the TrainingPlan window. */
  templateId: string | null;
  /** Identity of that explicit Start click; one launch starts one session. */
  launchKey: string;
  /** Reopen this live session (the Home training chip) instead of starting one. */
  sessionId?: string | null;
  onOpenPlan?: () => void;
}

/**
 * Penpot Trainer boards (In progress, Set recorded, Recovery, Ended) on the
 * Go runtime. Every set is one explicit user action with a receipt; the
 * TrainerGate decides whether a set can be recorded, never this window.
 * A correction retracts one receipt after the user confirms it (the
 * original stays on record) and reopens its set to be recorded again.
 */
export function TrainingExecutionPanel({
  planId,
  templateId,
  launchKey,
  sessionId,
  onOpenPlan,
}: TrainingExecutionPanelProps) {
  const trainer = useTrainerSession({
    planId,
    templateId,
    launchKey,
    sessionId,
  });
  const view = trainer.view;
  // The stepper starts from the prescription for each new set and travels
  // with that set's Complete command.
  const current = view?.current ?? null;
  const setKey = current
    ? `${view!.sessionId}:${current.exerciseIndex}:${current.setNumber}`
    : "none";
  const [actual, setActual] = useState<{ key: string; value: number | null }>({
    key: "none",
    value: null,
  });
  const actualValue =
    actual.key === setKey ? actual.value : (current?.defaultActual ?? null);
  const changeActual = (value: number) => setActual({ key: setKey, value });
  // A correction is two explicit steps: pick the receipt, confirm the retraction.
  const [correction, setCorrection] = useState<string | null>(null);
  const correctionTarget = correction
    ? (findReceipt(view, correction) ?? null)
    : null;
  const confirmCorrection = () => {
    if (!correction) return;
    trainer.correct(correction);
    setCorrection(null);
  };
  // 「说说感受」: the user's own words on the ended session, sent once.
  const [feedbackOpen, setFeedbackOpen] = useState(false);
  const [feedbackText, setFeedbackText] = useState("");
  const writingFeedback = feedbackOpen && !view?.feedback;

  let body: ReactNode;
  let footer: ReactNode = null;
  if (trainer.loadError) {
    body = (
      <Banner tone="bad" icon={<X size={16} aria-hidden="true" />} role="alert">
        训练记录暂时无法读取。
        <button
          type="button"
          className="bc-text-button"
          onClick={() => void trainer.refresh()}
        >
          重新读取
        </button>
      </Banner>
    );
  } else if (trainer.loading || (!view && trainer.busy)) {
    body = (
      <p role="status" className="bc-window-loading">
        <LoaderCircle size={16} className="animate-spin" aria-hidden="true" />
        正在开始训练
      </p>
    );
  } else if (!view) {
    // Start was refused: say why, nothing was recorded.
    body = (
      <div className="bc-tp-scene" data-testid="trainer-not-started">
        <Notices
          notice={trainer.notice}
          error={trainer.error}
          gate={trainer.gate}
        />
        <p className="bc-dx-note">训练没有开始，也没有记录任何内容。</p>
      </div>
    );
    footer = (
      <footer className="bc-window-footer bc-dx-footer">
        <button type="button" className="bc-dx-tonal" onClick={onOpenPlan}>
          返回计划
        </button>
      </footer>
    );
  } else if (view.state === "ended") {
    body = (
      <EndedScene
        view={view}
        notices={
          <Notices notice={trainer.notice} error={trainer.error} gate={null} />
        }
        form={
          writingFeedback ? (
            <FeedbackForm
              text={feedbackText}
              onChange={setFeedbackText}
              disabled={trainer.busy}
            />
          ) : null
        }
      />
    );
    footer = writingFeedback ? (
      <footer
        className="bc-window-footer bc-dx-footer"
        data-testid="trainer-actions"
      >
        <button
          type="button"
          className="bc-dx-tonal"
          disabled={trainer.busy}
          onClick={() => setFeedbackOpen(false)}
        >
          取消
        </button>
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy || !feedbackText.trim()}
          aria-busy={trainer.busy || undefined}
          onClick={() => trainer.sendFeedback(feedbackText)}
        >
          {trainer.busy ? (
            <LoaderCircle
              size={16}
              className="animate-spin"
              aria-hidden="true"
            />
          ) : (
            <Check size={16} aria-hidden="true" />
          )}
          发送
        </button>
      </footer>
    ) : (
      <footer
        className="bc-window-footer bc-dx-footer"
        data-testid="trainer-actions"
      >
        {view.feedback ? null : (
          <button
            type="button"
            className="bc-dx-tonal"
            onClick={() => setFeedbackOpen(true)}
          >
            说说感受
          </button>
        )}
        <button type="button" className="bc-dx-primary" onClick={onOpenPlan}>
          <Check size={16} aria-hidden="true" />
          完成
        </button>
      </footer>
    );
  } else if (view.state === "paused") {
    body = (
      <RecoveryScene view={view}>
        <Notices
          notice={trainer.notice}
          error={trainer.error}
          gate={view.gate}
        />
      </RecoveryScene>
    );
    footer = (
      <footer
        className="bc-window-footer bc-dx-footer"
        data-testid="trainer-actions"
      >
        <button
          type="button"
          className="bc-dx-tonal bc-dx-early"
          disabled={trainer.busy}
          onClick={trainer.finish}
        >
          结束
        </button>
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy || !view.gate.permits}
          aria-describedby={
            view.gate.permits ? undefined : "trainer-gate-reason"
          }
          onClick={trainer.resume}
        >
          <Play size={16} aria-hidden="true" />
          继续
        </button>
      </footer>
    );
  } else {
    body = (
      <InProgressScene
        view={view}
        trainer={trainer}
        actual={actualValue}
        onActual={changeActual}
        onCorrect={setCorrection}
        correction={
          correctionTarget && view.canRecord ? (
            <CorrectionConfirm
              label={correctionTarget}
              busy={trainer.busy}
              onCancel={() => setCorrection(null)}
              onConfirm={confirmCorrection}
            />
          ) : null
        }
      />
    );
    footer = (
      <InProgressFooter view={view} trainer={trainer} actual={actualValue} />
    );
  }

  return (
    <section
      className="bc-business-window-body bc-dx bc-tp bc-tr"
      aria-label="训练执行"
      data-testid="trainer-session-panel"
      data-state={view?.state ?? "none"}
    >
      <div className="bc-window-body" role="region" aria-label="当前训练">
        {body}
      </div>
      {footer}
    </section>
  );
}
