import { useEffect, useRef, type ReactNode } from "react";
import { Check } from "lucide-react";
import type { TrainerSessionView } from "@/features/body-canvas/model/trainerSession";
import { Textarea } from "@/components/ui/textarea";
import {
  ProgressBar,
  RecordedList,
  StatusRow,
} from "./TrainingExecutionChrome";

export function FeedbackForm({
  text,
  onChange,
  disabled,
}: {
  text: string;
  onChange: (text: string) => void;
  disabled: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    ref.current?.focus();
    ref.current?.scrollIntoView?.({ block: "nearest" });
  }, []);
  return (
    <section
      className="bc-dx-section bc-tr-feedback"
      aria-label="说说感受"
      data-testid="trainer-feedback-form"
    >
      <label className="bc-dx-label" htmlFor="trainer-feedback-text">
        这次训练感觉怎么样？
      </label>
      <Textarea
        ref={ref}
        id="trainer-feedback-text"
        className="bc-tr-feedback-input"
        rows={4}
        maxLength={2000}
        value={text}
        disabled={disabled}
        placeholder="比如哪里吃力、哪里有不适，用你自己的话说就好。"
        onChange={(event) => onChange(event.target.value)}
      />
    </section>
  );
}

export function RecoveryScene({
  view,
  children,
}: {
  view: TrainerSessionView;
  children: ReactNode;
}) {
  const exercise = view.current
    ? view.exercises[view.current.exerciseIndex]
    : null;
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

export function EndedScene({
  view,
  notices,
  form,
}: {
  view: TrainerSessionView;
  notices: ReactNode;
  form: ReactNode;
}) {
  const minutes = Math.max(1, Math.round(view.elapsedSeconds / 60));
  return (
    <div className="bc-tp-scene" data-testid="trainer-ended">
      <div className="bc-dx-state" data-tone="ok">
        <span className="bc-dx-state-halo" aria-hidden="true">
          <Check size={22} />
        </span>
        <p className="bc-dx-state-title">训练已结束</p>
        <p className="bc-dx-state-text">
          {view.endedReason === "finished" ? "按计划完成" : "提前结束"} ·{" "}
          {minutes} 分钟
        </p>
      </div>
      {notices}
      <RecordedList view={view} />
      {view.feedback ? (
        <section
          className="bc-dx-section bc-tr-feedback"
          aria-label="你的感受"
          data-testid="trainer-feedback"
        >
          <h4 className="bc-dx-label">你的感受</h4>
          <blockquote className="bc-tr-feedback-text">
            {view.feedback.text}
          </blockquote>
          <p className="bc-dx-note">已按原话记录。</p>
        </section>
      ) : (
        form
      )}
    </div>
  );
}
