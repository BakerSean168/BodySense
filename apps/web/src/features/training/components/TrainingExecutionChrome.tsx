import { Check, CircleHelp, Minus } from "lucide-react";
import {
  formatClock,
  type TrainerExerciseView,
  type TrainerSessionView,
} from "@/features/body-canvas/model/trainerSession";
import { Chip, GateChip } from "./TrainingExecutionAtoms";

export function ProgressBar({
  exercises,
}: {
  exercises: TrainerExerciseView[];
}) {
  const done = exercises.filter(
    (exercise) => exercise.progress === "done",
  ).length;
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

export function StatusRow({ view }: { view: TrainerSessionView }) {
  return (
    <div className="bc-tr-status">
      <div className="bc-dx-status">
        <Chip
          tone={view.state === "paused" ? "warn" : "ok"}
          dot
          testId="trainer-state"
        >
          {view.state === "paused" ? "已暂停" : "进行中"}
        </Chip>
        <GateChip gate={view.gate} />
      </div>
      <span
        className="bc-tr-clock"
        aria-label={`已训练 ${formatClock(view.elapsedSeconds)}`}
      >
        {view.state === "paused" ? "已计时 " : ""}
        {formatClock(view.elapsedSeconds)}
      </span>
    </div>
  );
}

export function RecordedList({ view }: { view: TrainerSessionView }) {
  return (
    <section className="bc-dx-section" aria-label="已记录">
      <h4 className="bc-dx-label">已记录</h4>
      <ul className="bc-dx-rows" data-testid="trainer-recorded">
        {view.exercises.map((exercise) => (
          <li
            key={exercise.planExerciseId}
            className="bc-dx-row bc-tr-recorded"
            data-progress={exercise.progress}
          >
            <span
              className="bc-dx-mark"
              data-mark={
                exercise.progress === "done"
                  ? "support"
                  : exercise.progress === "partial"
                    ? "gap"
                    : "context"
              }
              aria-hidden="true"
            >
              {exercise.progress === "done" ? (
                <Check size={13} />
              ) : exercise.progress === "partial" ? (
                <CircleHelp size={13} />
              ) : (
                <Minus size={13} />
              )}
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
