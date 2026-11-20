import type { ReactNode } from "react";
import {
  Check,
  Clock3,
  LoaderCircle,
  Minus,
  Pause,
  Plus,
  RotateCcw,
} from "lucide-react";
import {
  formatRestClock,
  type TrainerSessionView,
} from "@/features/body-canvas/model/trainerSession";
import { useTrainerSession } from "../hooks/useTrainerSession";
import { setLabel } from "../model/trainingExecutionLabels";
import { Banner, Notices } from "./TrainingExecutionAtoms";
import { ProgressBar, StatusRow } from "./TrainingExecutionChrome";

type TrainerController = ReturnType<typeof useTrainerSession>;

export function CorrectionConfirm({
  label,
  busy,
  onCancel,
  onConfirm,
}: {
  label: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <div
      className="bc-dx-banner bc-tp-banner bc-tr-correction"
      data-tone="warn"
      role="group"
      aria-label="更正记录"
      data-testid="trainer-correction"
    >
      <RotateCcw size={16} aria-hidden="true" />
      <span className="bc-dx-banner-text">
        撤回「{label}」后重新记录？原记录会保留在训练历史中。
      </span>
      <span className="bc-tr-correction-actions">
        <button
          type="button"
          className="bc-dx-link"
          disabled={busy}
          onClick={onCancel}
        >
          取消
        </button>
        <button
          type="button"
          className="bc-dx-tonal"
          disabled={busy}
          onClick={onConfirm}
        >
          撤回重记
        </button>
      </span>
    </div>
  );
}

export function InProgressScene({
  view,
  trainer,
  actual,
  onActual,
  onCorrect,
  correction,
}: {
  view: TrainerSessionView;
  trainer: TrainerController;
  actual: number | null;
  onActual: (value: number) => void;
  onCorrect: (actionId: string) => void;
  correction: ReactNode;
}) {
  const current = view.current;
  const exercise = current ? view.exercises[current.exerciseIndex] : null;
  const resting = view.restRemainingSeconds > 0;
  const step = current?.actualUnit === "seconds" ? 5 : 1;
  const max = current?.actualUnit === "seconds" ? 86_400 : 1000;
  // Penpot: the receipt banner belongs to Set recorded (rest running). Once the
  // rest is over the next set is In progress without it; a receipt with no
  // rest stays until the next action, naming its exercise when that moved on.
  const last = view.last;
  const showLast =
    last !== null &&
    (resting || !(last.kind === "performed" && last.restSeconds));
  const lastElsewhere =
    last !== null &&
    exercise !== null &&
    last.planExerciseId !== exercise.planExerciseId;
  const correctable = view.canRecord && !trainer.busy;

  return (
    <div
      className="bc-tp-scene"
      data-testid={resting ? "trainer-set-recorded" : "trainer-in-progress"}
    >
      <StatusRow view={view} />
      <ProgressBar exercises={view.exercises} />
      <Notices notice={trainer.notice} error={trainer.error} gate={view.gate} />
      {correction}
      {showLast && last ? (
        last.kind === "corrected" ? (
          <Banner
            tone="info"
            icon={<RotateCcw size={16} aria-hidden="true" />}
            role="status"
            testId="trainer-last-receipt"
          >
            {last.setNumber
              ? `${lastElsewhere ? `${last.name} · ` : ""}第 ${last.setNumber} 组`
              : last.name}
            已撤回 · 请重新记录
          </Banner>
        ) : (
          <div
            className="bc-dx-banner bc-tp-banner"
            data-tone="info"
            role="status"
            data-testid="trainer-last-receipt"
          >
            <Check size={16} aria-hidden="true" />
            <span className="bc-dx-banner-text">
              {lastElsewhere && last.setNumber ? `${last.name} · ` : ""}
              {last.setNumber ? `第 ${last.setNumber} 组` : last.name}
              {last.kind === "skipped" ? "已跳过" : "已记录"}
              {last.actualReps ? ` · ${last.actualReps} 次` : ""}
              {last.actualDurationSeconds
                ? ` · ${last.actualDurationSeconds} 秒`
                : ""}
            </span>
            {correctable ? (
              <button
                type="button"
                className="bc-dx-link bc-tr-correct"
                onClick={() => onCorrect(last.actionId)}
              >
                更正
              </button>
            ) : null}
          </div>
        )
      ) : null}
      {exercise && current ? (
        <section
          className="bc-tr-card"
          aria-label="当前动作"
          data-testid="trainer-current"
        >
          <div className="bc-tr-card-head">
            <span className="bc-tr-card-stage">
              {exercise.stageLabel} · {exercise.position} / {exercise.stageSize}
            </span>
            {view.canRecord && !resting ? (
              <button
                type="button"
                className="bc-dx-link bc-tr-skip-exercise"
                aria-label={`跳过此动作：${exercise.name}`}
                disabled={trainer.busy}
                onClick={trainer.skipExercise}
              >
                跳过
              </button>
            ) : null}
          </div>
          <h3 className="bc-tr-card-name">{exercise.name}</h3>
          <ol className="bc-tr-sets" aria-label="组">
            {exercise.sets.map((state, index) => {
              const actionId = exercise.setActionIds[index];
              const label = (
                <>
                  {state !== "pending" ? (
                    <span className="bc-dx-chip-dot" aria-hidden="true" />
                  ) : null}
                  {setLabel(state, index)}
                </>
              );
              return (
                <li
                  key={index}
                  data-set-state={state}
                  data-current={
                    index + 1 === current.setNumber ? "true" : undefined
                  }
                  aria-current={
                    index + 1 === current.setNumber ? "step" : undefined
                  }
                >
                  {/* A recorded set opens its correction (confirmed in a second step). */}
                  {actionId && correctable ? (
                    <button
                      type="button"
                      className="bc-tr-set-button"
                      aria-label={`更正第 ${index + 1} 组`}
                      onClick={() => onCorrect(actionId)}
                    >
                      {label}
                    </button>
                  ) : (
                    label
                  )}
                </li>
              );
            })}
          </ol>
          <p className="bc-tr-target">
            目标 {exercise.prescription.replace(/^\d+ 组 × /, "")}
            {exercise.restSeconds && resting
              ? ` · 休息 ${exercise.restSeconds} 秒`
              : ""}
          </p>
          {resting ? (
            <p
              className="bc-tr-rest"
              data-testid="trainer-rest"
              aria-live="off"
            >
              <span className="bc-tr-rest-chip">
                <Clock3 size={12} aria-hidden="true" />
                休息中
              </span>
              <strong>{formatRestClock(view.restRemainingSeconds)}</strong>
            </p>
          ) : actual !== null ? (
            <div className="bc-tr-stepper">
              <button
                type="button"
                className="bc-tr-step"
                aria-label="减少"
                disabled={actual <= 1}
                onClick={() => onActual(Math.max(1, actual - step))}
              >
                <Minus size={18} aria-hidden="true" />
              </button>
              <output
                className="bc-tr-actual"
                aria-label={
                  current.actualUnit === "seconds" ? "实际秒数" : "实际次数"
                }
              >
                {actual}
              </output>
              <button
                type="button"
                className="bc-tr-step"
                aria-label="增加"
                disabled={actual >= max}
                onClick={() => onActual(Math.min(max, actual + step))}
              >
                <Plus size={18} aria-hidden="true" />
              </button>
              <span className="bc-tr-actual-label">
                {current.actualUnit === "seconds" ? "实际秒数" : "实际次数"}
              </span>
            </div>
          ) : null}
        </section>
      ) : view.allRecorded ? (
        <p className="bc-dx-note" data-testid="trainer-all-recorded">
          所有组都已记录，可以结束本次训练。
        </p>
      ) : null}
      {view.next ? (
        <p className="bc-dx-next">
          <span className="bc-dx-next-label">下一项</span>
          <span className="bc-dx-next-name">
            {view.next.name} · {view.next.prescription}
          </span>
        </p>
      ) : null}
    </div>
  );
}

export function InProgressFooter({
  view,
  trainer,
  actual,
}: {
  view: TrainerSessionView;
  trainer: TrainerController;
  actual: number | null;
}) {
  const resting = view.restRemainingSeconds > 0;
  const blocked = !view.canRecord;
  return (
    <footer
      className="bc-window-footer bc-dx-footer bc-tr-footer"
      data-testid="trainer-actions"
    >
      <button
        type="button"
        className="bc-dx-icon-button bc-tr-pause"
        aria-label="暂停训练"
        disabled={trainer.busy}
        onClick={trainer.pause}
      >
        <Pause size={18} aria-hidden="true" />
      </button>
      {view.allRecorded || resting || blocked ? (
        <button
          type="button"
          className="bc-dx-link bc-dx-early"
          disabled={trainer.busy}
          onClick={trainer.finish}
        >
          结束
        </button>
      ) : (
        <button
          type="button"
          className="bc-dx-link bc-dx-early"
          disabled={trainer.busy}
          onClick={trainer.skip}
        >
          跳过本组
        </button>
      )}
      {view.allRecorded ? (
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy}
          onClick={trainer.finish}
        >
          <Check size={16} aria-hidden="true" />
          结束训练
        </button>
      ) : (
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy || blocked}
          aria-busy={trainer.busy || undefined}
          aria-describedby={
            view.gate.reasons.length ? "trainer-gate-reason" : undefined
          }
          onClick={() => trainer.complete(actual)}
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
          完成本组
        </button>
      )}
    </footer>
  );
}
