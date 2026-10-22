import { ChevronDown } from "lucide-react";
import { TrainingIcon } from "../../components/TrainingIcon";
import type { TrainingSelection } from "../../model/trainingPlanSelection";

export interface TrainingWindowHeaderProps {
  view: TrainingSelection;
  onSelectPlan: (planId: string) => void;
}

/**
 * Penpot "Window / Title Training": 训练 / <plan picker>. Every plan record is
 * a user-accepted revision, so all of the user's plans are offered; an empty
 * list shows the title only, like Diagnosis / Empty.
 */
export function TrainingWindowHeader({
  view,
  onSelectPlan,
}: TrainingWindowHeaderProps) {
  const title = (separator: boolean) => (
    <>
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      {separator ? (
        <span className="bc-window-context-sep" aria-hidden="true">
          /
        </span>
      ) : null}
    </>
  );
  if (view.plans.length === 0)
    return (
      <div className="bc-window-context bc-training-context">
        {title(false)}
      </div>
    );
  return (
    <div className="bc-window-context bc-training-context">
      {title(true)}
      <span className="bc-window-select-wrap">
        <select
          className="bc-window-context-select"
          aria-label="选择训练计划"
          value={view.selected?.planId ?? ""}
          onChange={(event) => onSelectPlan(event.target.value)}
        >
          {view.selected === null ? <option value="">选择计划</option> : null}
          {view.plans.map((item) => (
            <option key={item.planId} value={item.planId}>
              {item.title}
            </option>
          ))}
        </select>
        <ChevronDown size={15} aria-hidden="true" />
      </span>
    </div>
  );
}

/** Penpot Safety / Restricted: 训练 / 安全限制. */
export function TrainingSafetyHeader() {
  return (
    <div className="bc-window-context bc-training-context">
      <span className="bc-window-context-title">
        <TrainingIcon size={18} aria-hidden="true" />
        训练
      </span>
      <span className="bc-window-context-sep" aria-hidden="true">
        /
      </span>
      <span
        className="bc-window-select-wrap bc-window-context-mode"
        data-testid="training-mode"
      >
        安全限制
      </span>
    </div>
  );
}
