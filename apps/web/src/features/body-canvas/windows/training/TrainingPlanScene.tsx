import {
  CalendarDays,
  Check,
  Clock3,
  Eye,
  Minus,
  ShieldAlert,
  X,
} from "lucide-react";
import type {
  PlanExerciseView,
  PlanTemplateView,
  ProposalView,
  TrainingGateView,
  TrainingPlanView,
  TrainingWindowView,
} from "../../model/trainingPlans";
import type { TrainingPlanControls } from "./types";
import { Banner, Chip } from "./TrainingPrimitives";

function PlanStatusRow({ plan }: { plan: TrainingPlanView }) {
  return (
    <div className="bc-dx-status" data-testid="training-plan-status">
      <Chip tone={plan.statusTone} dot>
        {plan.statusLabel}
      </Chip>
      {plan.frequencyLabel ? (
        <Chip
          tone="neutral"
          icon={<CalendarDays size={12} aria-hidden="true" />}
        >
          {plan.frequencyLabel}
        </Chip>
      ) : null}
      {plan.scheduleLabel ? (
        <Chip tone="neutral">{plan.scheduleLabel}</Chip>
      ) : null}
    </div>
  );
}

function ExerciseRow({ exercise }: { exercise: PlanExerciseView }) {
  return (
    <li
      className="bc-tp-exercise"
      data-exercise-id={exercise.planExerciseId}
      data-change={exercise.change ?? undefined}
    >
      <span className="bc-tp-exercise-text">
        <strong className="bc-tp-exercise-name">{exercise.name}</strong>
        <span className="bc-tp-exercise-dose">
          <span className="bc-tp-exercise-prescription">
            {exercise.prescription}
          </span>
          {exercise.rest ? (
            <span className="bc-tp-exercise-rest">{exercise.rest}</span>
          ) : null}
          {exercise.load ? (
            <span className="bc-tp-exercise-rest">{exercise.load}</span>
          ) : null}
        </span>
      </span>
      {exercise.change ? (
        <Chip tone="ok" dot testId="training-exercise-change">
          {exercise.change === "added" ? "新增" : "已调整"}
        </Chip>
      ) : exercise.side ? (
        <Chip tone="neutral">{exercise.side}</Chip>
      ) : null}
    </li>
  );
}

function TemplateContent({
  plan,
  template,
  onSelectTemplate,
}: {
  plan: TrainingPlanView;
  template: PlanTemplateView;
  onSelectTemplate: (templateId: string) => void;
}) {
  return (
    <section
      className="bc-tp-template"
      aria-label="训练课内容"
      data-testid="training-template"
    >
      <div className="bc-tp-template-head">
        {plan.templates.length > 1 ? (
          <span className="bc-window-select-wrap bc-tp-template-select">
            <select
              className="bc-window-context-select"
              aria-label="选择训练课"
              value={template.templateId}
              onChange={(event) => onSelectTemplate(event.target.value)}
            >
              {plan.templates.map((item) => (
                <option key={item.templateId} value={item.templateId}>
                  {item.title}
                </option>
              ))}
            </select>
          </span>
        ) : (
          <strong className="bc-tp-template-title">{template.title}</strong>
        )}
        <span className="bc-tp-template-meta">
          {template.exerciseCount} 个动作
        </span>
      </div>
      {template.stages.map((stage) => (
        <section
          key={stage.phase}
          className="bc-tp-stage"
          data-stage={stage.phase}
          aria-label={stage.label}
        >
          <h4 className="bc-dx-label">{stage.label}</h4>
          <ol className="bc-tp-exercises">
            {stage.exercises.map((exercise) => (
              <ExerciseRow key={exercise.planExerciseId} exercise={exercise} />
            ))}
          </ol>
        </section>
      ))}
    </section>
  );
}

function GateBanner({ gate, id }: { gate: TrainingGateView; id: string }) {
  if (gate.reasons.length === 0) return null;
  return (
    <Banner
      tone={gate.permits ? "warn" : "bad"}
      icon={<ShieldAlert size={16} aria-hidden="true" />}
      role={gate.permits ? "status" : "alert"}
      testId="training-gate-reason"
    >
      <span id={id}>
        {gate.permits ? "可以开始，但请留意：" : "暂不能开始："}
        {gate.reasons.join("；")}
      </span>
    </Banner>
  );
}

function PendingRow({
  proposal,
  onOpen,
}: {
  proposal: ProposalView;
  onOpen: (proposalId: string) => void;
}) {
  const count = proposal.changes.length;
  return (
    <div className="bc-tp-pending" data-testid="training-pending-proposal">
      {proposal.expiresInDays !== null ? (
        <span className="bc-tp-pending-ttl">
          <Clock3 size={13} aria-hidden="true" />
          {proposal.expiresInDays} 天内
        </span>
      ) : null}
      <span className="bc-tp-pending-text">
        {proposal.kind === "create"
          ? `新计划「${proposal.title}」待确认`
          : `${count} 项调整建议待确认`}
      </span>
      <button
        type="button"
        className="bc-dx-icon-button"
        aria-label={
          proposal.kind === "create" ? "查看新计划建议" : "查看调整建议"
        }
        onClick={() => onOpen(proposal.proposalId)}
      >
        <Eye size={18} aria-hidden="true" />
      </button>
    </div>
  );
}

export function PlanScene({
  view,
  plan,
  controls,
  gateReasonId,
  updated,
}: {
  view: TrainingWindowView;
  plan: TrainingPlanView;
  controls: TrainingPlanControls;
  gateReasonId: string;
  updated: boolean;
}) {
  const template = plan.selectedTemplate;
  return (
    <div
      className="bc-tp-scene"
      data-testid={updated ? "training-updated" : "training-plan"}
    >
      <PlanStatusRow plan={plan} />
      {updated ? (
        <Banner
          tone="info"
          icon={<Check size={16} aria-hidden="true" />}
          role="status"
          testId="training-receipt"
        >
          {view.updated?.created
            ? "已采纳新计划 · 现在生效"
            : "已采纳调整 · 新修订已生效"}
        </Banner>
      ) : null}
      {!updated && controls.notice === "rejected" ? (
        <Banner
          tone="info"
          icon={<Minus size={16} aria-hidden="true" />}
          role="status"
          testId="training-rejected"
          action={
            <button
              type="button"
              className="bc-dx-icon-button"
              aria-label="关闭提示"
              onClick={controls.onDismissNotice}
            >
              <X size={16} aria-hidden="true" />
            </button>
          }
        >
          已拒绝调整 · 计划保持原样
        </Banner>
      ) : null}
      {!updated
        ? [...plan.pendingProposals, ...view.pendingCreates].map((proposal) => (
            <PendingRow
              key={proposal.proposalId}
              proposal={proposal}
              onOpen={controls.onOpenProposal}
            />
          ))
        : null}
      <GateBanner gate={plan.gate} id={gateReasonId} />
      {template ? (
        <TemplateContent
          plan={plan}
          template={template}
          onSelectTemplate={controls.onSelectTemplate}
        />
      ) : null}
    </div>
  );
}
