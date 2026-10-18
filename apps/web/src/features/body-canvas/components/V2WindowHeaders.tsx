import { useEffect, useState } from "react";
import { ChevronDown, Clock3, Stethoscope } from "lucide-react";
import type { ProjectedTrainingPlans } from "../model/trainingPlans";

export interface DiagnosisWindowHeaderProps {
  /** Only adopted ConditionEntries; proposals never appear in the picker. */
  view: { confirmed: readonly { conditionId: string; name: string }[] };
  selectedConditionId: string | null;
  onSelectCondition: (conditionId: string) => void;
  /**
   * Penpot Diagnosis boards: a formal run shows "正式分析" and its settled
   * outcome "提交回执" in the toolbar instead of the condition picker.
   */
  mode?: "analysis" | "receipt" | null;
}

const MODE_LABELS = { analysis: "正式分析", receipt: "提交回执" } as const;

/** Penpot "Window / Title Diagnosis": 🩺 诊断 / <selector>. */
function DiagnosisTitle({ separator = true }: { separator?: boolean }) {
  return (
    <>
      <span className="bc-window-context-title">
        <Stethoscope size={18} aria-hidden="true" />
        诊断
      </span>
      {separator ? (
        <span className="bc-window-context-sep" aria-hidden="true">
          /
        </span>
      ) : null}
    </>
  );
}

/** Penpot V2: condition picker belongs to the window toolbar, not the scroll body. */
export function DiagnosisWindowHeader({
  view,
  selectedConditionId,
  onSelectCondition,
  mode = null,
}: DiagnosisWindowHeaderProps) {
  if (mode)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle />
        <span
          className="bc-window-select-wrap bc-window-context-mode"
          data-testid="diagnosis-mode"
        >
          {MODE_LABELS[mode]}
        </span>
      </div>
    );

  const selected = view.confirmed.some(
    (item) => item.conditionId === selectedConditionId,
  )
    ? selectedConditionId!
    : (view.confirmed[0]?.conditionId ?? "");

  // Penpot Diagnosis / Empty: only the window title, no empty picker.
  if (view.confirmed.length === 0)
    return (
      <div className="bc-window-context bc-diagnosis-context">
        <DiagnosisTitle separator={false} />
      </div>
    );

  return (
    <div className="bc-window-context bc-diagnosis-context">
      <DiagnosisTitle />
      <span className="bc-window-select-wrap">
        <select
          className="bc-window-context-select"
          aria-label="选择已确认病情"
          value={selected}
          disabled={view.confirmed.length === 0}
          onChange={(event) => onSelectCondition(event.target.value)}
        >
          {view.confirmed.length === 0 ? (
            <option value="">暂无已确认病情</option>
          ) : (
            view.confirmed.map((item) => (
              <option key={item.conditionId} value={item.conditionId}>
                {item.name}
              </option>
            ))
          )}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
    </div>
  );
}

export interface TrainingWindowHeaderProps {
  projected: ProjectedTrainingPlans;
  onSelectPlan: (planId: string) => void;
}

/** Same reusable toolbar geometry as Diagnosis; one selected plan, no duplicate list. */
export function TrainingWindowHeader({
  projected,
  onSelectPlan,
}: TrainingWindowHeaderProps) {
  const plans = projected.plans.filter((item) => item.isAccepted);
  const plan = projected.selected?.isAccepted ? projected.selected : null;
  return (
    <div className="bc-window-context bc-training-context">
      <span className="bc-window-context-caption">训练计划</span>
      <span className="bc-window-select-wrap">
        <select
          className="bc-window-context-select"
          aria-label="选择训练计划"
          value={plan?.planId ?? ""}
          disabled={plans.length === 0}
          onChange={(event) => onSelectPlan(event.target.value)}
        >
          {plan === null ? (
            <option value="">暂无选中的已接受计划</option>
          ) : null}
          {plans.map((item) => (
            <option key={item.planId} value={item.planId}>
              {item.title}
            </option>
          ))}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
      {plan?.frequency ? (
        <span className="bc-window-context-meta">{plan.frequency}</span>
      ) : null}
    </div>
  );
}

function localClockLabel(time: Date): string {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "long",
    day: "numeric",
    weekday: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(time);
}

/** The time is labelled as a live local clock, never misrepresented as message creation time. */
export function ChatWindowHeader({ now: override }: { now?: Date }) {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    if (override) return;
    const ticker = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(ticker);
  }, [override]);
  const time = override ?? now;

  return (
    <div className="bc-window-context bc-chat-context">
      <strong className="bc-chat-context-title">对话</strong>
      <time
        className="bc-chat-context-time"
        dateTime={time.toISOString()}
        aria-label={`当前本地时间 ${localClockLabel(time)}`}
      >
        <Clock3 size={13} aria-hidden="true" />
        {localClockLabel(time)}
      </time>
    </div>
  );
}
