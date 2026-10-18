import { useState, type ReactNode } from "react";
import {
  Check,
  CircleHelp,
  Clock3,
  LoaderCircle,
  Minus,
  Pause,
  Play,
  Plus,
  ShieldAlert,
  ShieldCheck,
  X,
} from "lucide-react";
import {
  formatClock,
  formatRestClock,
  type TrainerExerciseView,
  type TrainerSessionView,
} from "@/features/body-canvas/model/trainerSession";
import type { TrainingGateView } from "@/features/body-canvas/model/trainingPlans";
import { useTrainerSession, type TrainerNotice } from "../hooks/useTrainerSession";

export interface TrainingExecutionPanelProps {
  planId: string;
  /** The template the user started from the TrainingPlan window. */
  templateId: string | null;
  /** Identity of that explicit Start click; one launch starts one session. */
  launchKey: string;
  onOpenPlan?: () => void;
}

/**
 * Penpot Trainer boards (In progress, Set recorded, Recovery, Ended) on the
 * Go runtime. Every set is one explicit user action with a receipt; the
 * TrainerGate decides whether a set can be recorded, never this window.
 */
export function TrainingExecutionPanel({ planId, templateId, launchKey, onOpenPlan }: TrainingExecutionPanelProps) {
  const trainer = useTrainerSession({ planId, templateId, launchKey });
  const view = trainer.view;
  // The stepper starts from the prescription for each new set and travels
  // with that set's Complete command.
  const current = view?.current ?? null;
  const setKey = current ? `${view!.sessionId}:${current.exerciseIndex}:${current.setNumber}` : "none";
  const [actual, setActual] = useState<{ key: string; value: number | null }>({ key: "none", value: null });
  const actualValue = actual.key === setKey ? actual.value : (current?.defaultActual ?? null);
  const changeActual = (value: number) => setActual({ key: setKey, value });

  let body: ReactNode;
  let footer: ReactNode = null;
  if (trainer.loadError) {
    body = (
      <Banner tone="bad" icon={<X size={16} aria-hidden="true" />} role="alert">
        训练记录暂时无法读取。
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
        <Notices notice={trainer.notice} error={trainer.error} gate={trainer.gate} />
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
    body = <EndedScene view={view} />;
    footer = (
      <footer className="bc-window-footer bc-dx-footer">
        <button type="button" className="bc-dx-primary" onClick={onOpenPlan}>
          <Check size={16} aria-hidden="true" />
          完成
        </button>
      </footer>
    );
  } else if (view.state === "paused") {
    body = (
      <RecoveryScene view={view}>
        <Notices notice={trainer.notice} error={trainer.error} gate={view.gate} />
      </RecoveryScene>
    );
    footer = (
      <footer className="bc-window-footer bc-dx-footer" data-testid="trainer-actions">
        <button type="button" className="bc-dx-tonal bc-dx-early" disabled={trainer.busy} onClick={trainer.finish}>
          结束
        </button>
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy || !view.gate.permits}
          aria-describedby={view.gate.permits ? undefined : "trainer-gate-reason"}
          onClick={trainer.resume}
        >
          <Play size={16} aria-hidden="true" />
          继续
        </button>
      </footer>
    );
  } else {
    body = <InProgressScene view={view} trainer={trainer} actual={actualValue} onActual={changeActual} />;
    footer = <InProgressFooter view={view} trainer={trainer} actual={actualValue} />;
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

function Chip({ tone, children, dot, icon, testId }: { tone: string; children: ReactNode; dot?: boolean; icon?: ReactNode; testId?: string }) {
  return (
    <span className="bc-dx-chip" data-tone={tone} data-testid={testId}>
      {dot ? <span className="bc-dx-chip-dot" aria-hidden="true" /> : null}
      {icon}
      {children}
    </span>
  );
}

function Banner({ tone, icon, children, role, testId }: { tone: string; icon?: ReactNode; children: ReactNode; role?: "alert" | "status"; testId?: string }) {
  return (
    <div className="bc-dx-banner bc-tp-banner" data-tone={tone} role={role} data-testid={testId}>
      {icon}
      <span className="bc-dx-banner-text">{children}</span>
    </div>
  );
}

function GateChip({ gate }: { gate: TrainingGateView }) {
  return (
    <Chip
      tone={gate.tone}
      icon={gate.permits ? <ShieldCheck size={12} aria-hidden="true" /> : <ShieldAlert size={12} aria-hidden="true" />}
      testId="trainer-gate"
    >
      {gate.permits ? "可以继续" : "暂不能继续"}
    </Chip>
  );
}

/** Why the last command did not count, and the gate's own reasons. */
function Notices({ notice, error, gate }: { notice: TrainerNotice; error: string | null; gate: TrainingGateView | null }) {
  return (
    <>
      {notice === "safety_changed" ? (
        <Banner tone="warn" icon={<ShieldAlert size={16} aria-hidden="true" />} role="alert" testId="trainer-safety-changed">
          安全状态刚刚更新，这一步没有记录。请看一下当前提醒，再确认一次。
        </Banner>
      ) : null}
      {notice === "changed" ? (
        <Banner tone="info" icon={<CircleHelp size={16} aria-hidden="true" />} role="status" testId="trainer-changed">
          训练记录已在别处更新，已重新读取。
        </Banner>
      ) : null}
      {error ? (
        <Banner tone="bad" icon={<X size={16} aria-hidden="true" />} role="alert">
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

function ProgressBar({ exercises }: { exercises: TrainerExerciseView[] }) {
  const done = exercises.filter((exercise) => exercise.progress === "done").length;
  return (
    <div
      className="bc-tr-progress"
      role="progressbar"
      aria-label="训练进度"
      aria-valuemin={0}
      aria-valuemax={exercises.length}
      aria-valuenow={done}
    >
      {exercises.map((exercise) => (
        <span key={exercise.planExerciseId} data-progress={exercise.progress} />
      ))}
    </div>
  );
}

function StatusRow({ view }: { view: TrainerSessionView }) {
  return (
    <div className="bc-tr-status">
      <div className="bc-dx-status">
        <Chip tone={view.state === "paused" ? "warn" : "ok"} dot testId="trainer-state">
          {view.state === "paused" ? "已暂停" : "进行中"}
        </Chip>
        <GateChip gate={view.gate} />
      </div>
      <span className="bc-tr-clock" aria-label={`已训练 ${formatClock(view.elapsedSeconds)}`}>
        {view.state === "paused" ? "已计时 " : ""}
        {formatClock(view.elapsedSeconds)}
      </span>
    </div>
  );
}

function setLabel(state: string, index: number) {
  return `第 ${index + 1} 组${state === "skipped" ? " · 跳过" : ""}`;
}

function InProgressScene({
  view,
  trainer,
  actual,
  onActual,
}: {
  view: TrainerSessionView;
  trainer: ReturnType<typeof useTrainerSession>;
  actual: number | null;
  onActual: (value: number) => void;
}) {
  const current = view.current;
  const exercise = current ? view.exercises[current.exerciseIndex]! : null;
  const resting = view.restRemainingSeconds > 0;
  const step = current?.actualUnit === "seconds" ? 5 : 1;
  const max = current?.actualUnit === "seconds" ? 86_400 : 1000;

  return (
    <div className="bc-tp-scene" data-testid={resting ? "trainer-set-recorded" : "trainer-in-progress"}>
      <StatusRow view={view} />
      <ProgressBar exercises={view.exercises} />
      <Notices notice={trainer.notice} error={trainer.error} gate={view.gate} />
      {view.last ? (
        <Banner tone="info" icon={<Check size={16} aria-hidden="true" />} role="status" testId="trainer-last-receipt">
          {view.last.setNumber ? `第 ${view.last.setNumber} 组` : view.last.name}
          {view.last.kind === "skipped" ? "已跳过" : "已记录"}
          {view.last.actualReps ? ` · ${view.last.actualReps} 次` : ""}
          {view.last.actualDurationSeconds ? ` · ${view.last.actualDurationSeconds} 秒` : ""}
        </Banner>
      ) : null}
      {exercise && current ? (
        <section className="bc-tr-card" aria-label="当前动作" data-testid="trainer-current">
          <span className="bc-tr-card-stage">
            {exercise.stageLabel} · {exercise.position} / {exercise.stageSize}
          </span>
          <h3 className="bc-tr-card-name">{exercise.name}</h3>
          <ol className="bc-tr-sets" aria-label="组">
            {exercise.sets.map((state, index) => (
              <li
                key={index}
                data-set-state={state}
                data-current={index + 1 === current.setNumber ? "true" : undefined}
                aria-current={index + 1 === current.setNumber ? "step" : undefined}
              >
                {state !== "pending" ? <span className="bc-dx-chip-dot" aria-hidden="true" /> : null}
                {setLabel(state, index)}
              </li>
            ))}
          </ol>
          <p className="bc-tr-target">
            目标 {exercise.prescription.replace(/^\d+ 组 × /, "")}
            {exercise.restSeconds && resting ? ` · 休息 ${exercise.restSeconds} 秒` : ""}
          </p>
          {resting ? (
            <p className="bc-tr-rest" data-testid="trainer-rest" aria-live="off">
              <Clock3 size={14} aria-hidden="true" />
              <span>休息中</span>
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
              <output className="bc-tr-actual" aria-label={current.actualUnit === "seconds" ? "实际秒数" : "实际次数"}>
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
              <span className="bc-tr-actual-label">{current.actualUnit === "seconds" ? "实际秒数" : "实际次数"}</span>
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

function InProgressFooter({
  view,
  trainer,
  actual,
}: {
  view: TrainerSessionView;
  trainer: ReturnType<typeof useTrainerSession>;
  actual: number | null;
}) {
  const resting = view.restRemainingSeconds > 0;
  const blocked = !view.canRecord;
  return (
    <footer className="bc-window-footer bc-dx-footer bc-tr-footer" data-testid="trainer-actions">
      <button type="button" className="bc-dx-icon-button bc-tr-pause" aria-label="暂停训练" disabled={trainer.busy} onClick={trainer.pause}>
        <Pause size={18} aria-hidden="true" />
      </button>
      {view.allRecorded || resting || blocked ? (
        <button type="button" className="bc-dx-link bc-dx-early" disabled={trainer.busy} onClick={trainer.finish}>
          结束
        </button>
      ) : (
        <button type="button" className="bc-dx-link bc-dx-early" disabled={trainer.busy} onClick={trainer.skip}>
          跳过本组
        </button>
      )}
      {view.allRecorded ? (
        <button type="button" className="bc-dx-primary" disabled={trainer.busy} onClick={trainer.finish}>
          <Check size={16} aria-hidden="true" />
          结束训练
        </button>
      ) : (
        <button
          type="button"
          className="bc-dx-primary"
          disabled={trainer.busy || blocked}
          aria-busy={trainer.busy || undefined}
          aria-describedby={view.gate.reasons.length ? "trainer-gate-reason" : undefined}
          onClick={() => trainer.complete(actual)}
        >
          {trainer.busy ? <LoaderCircle size={16} className="animate-spin" aria-hidden="true" /> : <Check size={16} aria-hidden="true" />}
          完成本组
        </button>
      )}
    </footer>
  );
}

function RecordedList({ view }: { view: TrainerSessionView }) {
  return (
    <section className="bc-dx-section" aria-label="已记录">
      <h4 className="bc-dx-label">已记录</h4>
      <ul className="bc-dx-rows" data-testid="trainer-recorded">
        {view.exercises.map((exercise) => (
          <li key={exercise.planExerciseId} className="bc-dx-row bc-tr-recorded" data-progress={exercise.progress}>
            <span
              className="bc-dx-mark"
              data-mark={exercise.progress === "done" ? "support" : exercise.progress === "partial" ? "gap" : "context"}
              aria-hidden="true"
            >
              {exercise.progress === "done" ? <Check size={13} /> : exercise.progress === "partial" ? <CircleHelp size={13} /> : <Minus size={13} />}
            </span>
            <span className="bc-dx-row-text">
              <span className="bc-dx-row-statement">{exercise.name}</span>
            </span>
            <span className="bc-tr-recorded-count">
              {exercise.performedCount} / {exercise.sets.length} 组
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function RecoveryScene({ view, children }: { view: TrainerSessionView; children: ReactNode }) {
  const exercise = view.current ? view.exercises[view.current.exerciseIndex] : null;
  return (
    <div className="bc-tp-scene" data-testid="trainer-recovery">
      <StatusRow view={view} />
      <ProgressBar exercises={view.exercises} />
      {children}
      {exercise && view.current ? (
        <div className="bc-tr-resume">
          <span>上次停在</span>
          <strong>
            {exercise.name} · 第 {view.current.setNumber} 组
          </strong>
        </div>
      ) : null}
      <RecordedList view={view} />
    </div>
  );
}

function EndedScene({ view }: { view: TrainerSessionView }) {
  const minutes = Math.max(1, Math.round(view.elapsedSeconds / 60));
  return (
    <div className="bc-tp-scene" data-testid="trainer-ended">
      <div className="bc-dx-state" data-tone="ok">
        <span className="bc-dx-state-halo" aria-hidden="true">
          <Check size={22} />
        </span>
        <p className="bc-dx-state-title">训练已结束</p>
        <p className="bc-dx-state-text">
          {view.endedReason === "finished" ? "按计划完成" : "提前结束"} · {minutes} 分钟
        </p>
      </div>
      <RecordedList view={view} />
    </div>
  );
}
