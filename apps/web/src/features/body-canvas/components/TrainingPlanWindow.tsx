import type { ReactNode } from "react";
import { AlertTriangle, LoaderCircle, Play } from "lucide-react";
import {
  formatDosage,
  type PlanExercise,
  type ProjectedTrainingPlans,
  type TrainingStage,
} from "../model/trainingPlans";

/**
 * Penpot V2 TrainingPlan content. The selected plan lives in the shared header
 * next to the contextual Chat action, so this body contains only scrollable
 * stage content and one guarded Start button. Draft plans never run here.
 */

export interface TrainingPlanWindowProps {
  projected: ProjectedTrainingPlans;
  /**
   * Pending plan-change proposals with their accept/reject authorization. The
   * window never mutates plan content itself.
   */
  proposals?: ReactNode;
  loading?: boolean;
  error?: string | null;
  /** True when the backend reported a safety block that prevents execution. */
  executionBlocked: boolean;
  canExecute: boolean;
  onStartTraining: (planId: string) => void;
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
        <section
          key={stage.phase}
          className="bc-plan-stage"
          data-stage={stage.phase}
        >
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

export function TrainingPlanWindow({
  projected,
  proposals,
  loading = false,
  error = null,
  executionBlocked,
  canExecute,
  onStartTraining,
}: TrainingPlanWindowProps) {
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

        {plan && !plan.isAccepted ? (
          <div className="bc-window-alert" role="status">
            <AlertTriangle size={16} />
            <span>
              这份计划尚未被接受。它不能作为当前训练执行；请在对话中审阅并明确确认。
            </span>
          </div>
        ) : null}

        {plan?.isAccepted ? (
          <>
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
          disabled={!plan?.isAccepted || !canExecute || executionBlocked}
          onClick={() => {
            if (plan?.isAccepted && canExecute && !executionBlocked) {
              onStartTraining(plan.planId);
            }
          }}
        >
          <Play size={16} />
          开始训练
        </button>
      </footer>
    </section>
  );
}
