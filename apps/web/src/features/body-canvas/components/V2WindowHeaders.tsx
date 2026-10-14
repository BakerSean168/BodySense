import { useEffect, useState } from "react";
import { ChevronDown, Clock3 } from "lucide-react";
import type { DiagnosisView } from "../model/confirmedConditions";
import type { ProjectedTrainingPlans } from "../model/trainingPlans";

export interface DiagnosisWindowHeaderProps {
  view: DiagnosisView;
  selectedConditionId: string | null;
  onSelectCondition: (conditionId: string) => void;
}

/** Penpot V2: condition picker belongs to the window toolbar, not the scroll body. */
export function DiagnosisWindowHeader({
  view,
  selectedConditionId,
  onSelectCondition,
}: DiagnosisWindowHeaderProps) {
  const selected = view.confirmed.some(
    (item) => item.conditionId === selectedConditionId,
  )
    ? selectedConditionId!
    : (view.confirmed[0]?.conditionId ?? "");

  return (
    <div className="bc-window-context bc-diagnosis-context">
      <span className="bc-window-context-caption">当前诊断</span>
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
