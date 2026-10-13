import { useEffect, useRef, useState, type ReactNode } from "react";
import {
  AlertTriangle,
  ChevronDown,
  LoaderCircle,
  MessageCircle,
  Play,
} from "lucide-react";
import {
  formatDosage,
  type PlanExercise,
  type ProjectedTrainingPlans,
  type TrainingStage,
} from "../model/trainingPlans";

/**
 * TrainingPlan business window.
 *
 * Design contract (Penpot V5.5):
 * - header: single level-1 title on the left, Close on the right; no decorative
 *   icon and no repeated subtitle;
 * - one light single-line plan-selection row at the top showing the current plan
 *   name, its frequency and a compact switch affordance; clicking it opens the
 *   minimal picker (prototype picker height 234px);
 * - the body is one ordered action list per stage; content scrolls vertically;
 * - footer keeps 「开始训练 / 与助手调整」;
 * - switching plans must reuse the same window identity — the parent host owns
 *   that invariant, and this component only ever renders one plan.
 */

export interface TrainingPlanWindowProps {
  projected: ProjectedTrainingPlans;
  /**
   * Pending plan-change proposals with their accept/reject authorization. The
   * window never mutates plan content itself.
   */
  proposals?: ReactNode;
  onSelectPlan: (planId: string) => void;
  loading?: boolean;
  error?: string | null;
  /** True when the backend reported a safety block that prevents execution. */
  executionBlocked: boolean;
  canExecute: boolean;
  onStartTraining: (planId: string) => void;
  onAskAssistant: (planId: string) => void;
  onClose: () => void;
}

function ExerciseRow({ exercise }: { exercise: PlanExercise }) {
  const dosage = formatDosage(exercise);
  return (
    <li className="bc-exercise" data-exercise-id={exercise.exerciseId}>
      <div className="bc-exercise-main">
        <strong>{exercise.name}</strong>
        <span
          className={
            dosage === "剂量未提供"
              ? "bc-exercise-dosage bc-exercise-dosage-missing"
              : "bc-exercise-dosage"
          }
        >
          {dosage}
        </span>
      </div>
      {exercise.description ? (
        <p className="bc-exercise-note">{exercise.description}</p>
      ) : null}
      {exercise.notes ? (
        <p className="bc-exercise-note">{exercise.notes}</p>
      ) : null}
    </li>
  );
}

function StageList({ stages }: { stages: TrainingStage[] }) {
  return (
    <div className="bc-stage-list">
      {stages.map((stage) => (
        <section key={stage.phase} className="bc-plan-stage" data-stage={stage.phase}>
          <h3 className="bc-stage-title">{stage.label}</h3>
          <ol className="bc-exercise-list">
            {stage.exercises.map((exercise) => (
              <ExerciseRow key={exercise.exerciseId} exercise={exercise} />
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}

function PlanPicker({
  plans,
  selectedPlanId,
  onSelect,
  onClose,
}: {
  plans: ProjectedTrainingPlans["plans"];
  selectedPlanId: string | null;
  onSelect: (planId: string) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (!(event.target instanceof Node)) return;
      if (ref.current?.contains(event.target)) return;
      onClose();
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="bc-plan-picker"
      role="listbox"
      aria-label="选择训练计划"
    >
      {plans.map((plan) => (
        <button
          key={plan.planId}
          type="button"
          role="option"
          aria-selected={plan.planId === selectedPlanId}
          className="bc-plan-option"
          data-plan-id={plan.planId}
          onClick={() => {
            onSelect(plan.planId);
            onClose();
          }}
        >
          <span className="bc-plan-option-title">{plan.title}</span>
          {plan.isAccepted ? (
            <span className="bc-plan-option-badge">当前生效</span>
          ) : (
            <span className="bc-plan-option-status">{plan.status}</span>
          )}
        </button>
      ))}
    </div>
  );
}

export function TrainingPlanWindow({
  projected,
  proposals,
  onSelectPlan,
  loading = false,
  error = null,
  executionBlocked,
  canExecute,
  onStartTraining,
  onAskAssistant,
  onClose,
}: TrainingPlanWindowProps) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const plan = projected.selected;

  return (
    <section
      className="bc-business-window-body"
      aria-label="训练计划"
      data-testid="training-window"
    >
      <div className="bc-window-body" role="region" aria-label="当前计划内容">
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : loading ? (
          <p role="status" className="bc-window-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取训练计划
          </p>
        ) : projected.selectedPlanMissing ? (
          <div role="alert" className="bc-window-alert">
            <AlertTriangle size={16} />
            <span>之前打开的计划已经不存在，请在下方重新选择一份计划。</span>
          </div>
        ) : null}

        {projected.plans.length === 0 && !loading && !error ? (
          <div className="bc-window-empty">
            <p className="bc-window-empty-title">还没有训练计划</p>
            <p className="bc-window-empty-text">
              确认分析与方案后，这里会显示该计划完整的热身、训练与放松动作。
            </p>
          </div>
        ) : null}

        {plan ? (
          <>
            <div className="bc-plan-selector">
              <button
                type="button"
                className="bc-plan-selector-trigger"
                aria-expanded={pickerOpen}
                aria-haspopup="listbox"
                onClick={() => setPickerOpen((open) => !open)}
              >
                <span className="bc-plan-selector-name">{plan.title}</span>
                <span className="bc-plan-selector-meta">
                  {plan.frequency ?? "频率未提供"} · {plan.exerciseCount} 个动作
                </span>
                <ChevronDown size={16} />
              </button>
              {pickerOpen ? (
                <PlanPicker
                  plans={projected.plans}
                  selectedPlanId={plan.planId}
                  onSelect={onSelectPlan}
                  onClose={() => setPickerOpen(false)}
                />
              ) : null}
            </div>

            {executionBlocked ? (
              <div role="alert" className="bc-window-alert">
                <AlertTriangle size={16} />
                <span>
                  当前安全状态不允许执行训练。计划内容保持不变，请先处理安全提醒。
                </span>
              </div>
            ) : null}

            {plan.stages.length > 0 ? (
              <StageList stages={plan.stages} />
            ) : (
              <div className="bc-window-empty">
                <p className="bc-window-empty-title">这份计划还没有动作内容</p>
                <p className="bc-window-empty-text">
                  计划已建立，但还没有已接受的训练动作。可以与助手一起补充。
                </p>
              </div>
            )}

            {plan.stageStructureUnavailable && plan.stages.length > 0 ? (
              <p className="bc-window-note">
                阶段按已接受动作的类型归类；后端暂未提供逐计划的热身/放松分段。
              </p>
            ) : null}
            {proposals}
          </>
        ) : null}
      </div>

      <footer className="bc-window-footer">
        <button
          type="button"
          className="bc-button bc-primary"
          disabled={!plan || !canExecute || executionBlocked}
          onClick={() => plan && onStartTraining(plan.planId)}
        >
          <Play size={16} />
          开始训练
        </button>
        <button
          type="button"
          className="bc-button"
          disabled={!plan}
          onClick={() => plan && onAskAssistant(plan.planId)}
        >
          <MessageCircle size={16} />
          与助手调整
        </button>
        <button type="button" className="bc-button" onClick={onClose}>
          关闭
        </button>
      </footer>
    </section>
  );
}
