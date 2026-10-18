import { useId, useMemo, type ReactNode } from "react";
import {
  ArrowRight,
  CalendarDays,
  Check,
  CircleHelp,
  Clock3,
  Dumbbell,
  Eye,
  LoaderCircle,
  MessageCircle,
  Minus,
  Play,
  ShieldAlert,
  ShieldCheck,
  Sparkle,
  X,
} from "lucide-react";
import {
  projectTrainingWindow,
  type ProjectTrainingWindowInput,
  type PlanExerciseView,
  type PlanTemplateView,
  type ProposalChangeView,
  type ProposalView,
  type TrainingGateView,
  type TrainingPlanView,
  type TrainingTone,
  type TrainingWindowView,
} from "../model/trainingPlans";

/**
 * Penpot "03 App" Training boards on the V2 plan records: Empty, Accepted,
 * Proposal, Proposal · Expired and Updated. The plan picker lives in the
 * shared header; this body holds one scene and its footer. Plan content is
 * read-only; only an accepted proposal changes it, and only after its receipt.
 */

export interface TrainingPlanControls {
  busy?: boolean;
  error?: string | null;
  notice?: "rejected" | null;
  onOpenProposal: (proposalId: string) => void;
  onBackToPlan: () => void;
  onAccept: (proposalId: string) => void;
  onReject: (proposalId: string) => void;
  onDismissNotice: () => void;
  onSelectTemplate: (templateId: string) => void;
  onStart: (planId: string, templateId: string) => void;
  onOpenChat: () => void;
}

export interface TrainingPlanWindowProps {
  /** Raw records + local decision state; projected here, inside the lazy chunk. */
  state: ProjectTrainingWindowInput;
  controls: TrainingPlanControls;
  /**
   * Workspace execution capability. The Go gate on the record stays the
   * authority; this only keeps Start closed while the workspace disagrees.
   */
  canExecute: boolean;
}

function Chip({
  tone,
  children,
  dot = false,
  icon,
  testId,
}: {
  tone: TrainingTone;
  children: ReactNode;
  dot?: boolean;
  icon?: ReactNode;
  testId?: string;
}) {
  return (
    <span className="bc-dx-chip" data-tone={tone} data-testid={testId}>
      {dot ? <span className="bc-dx-chip-dot" aria-hidden="true" /> : null}
      {icon}
      {children}
    </span>
  );
}

function Banner({
  tone,
  icon,
  children,
  action,
  role,
  testId,
}: {
  tone: "warn" | "bad" | "info" | "ok";
  icon?: ReactNode;
  children: ReactNode;
  action?: ReactNode;
  role?: "alert" | "status";
  testId?: string;
}) {
  return (
    <div className="bc-dx-banner bc-tp-banner" data-tone={tone} role={role} data-testid={testId}>
      {icon}
      <span className="bc-dx-banner-text">{children}</span>
      {action}
    </div>
  );
}

function PlanStatusRow({ plan }: { plan: TrainingPlanView }) {
  return (
    <div className="bc-dx-status" data-testid="training-plan-status">
      <Chip tone={plan.statusTone} dot>
        {plan.statusLabel}
      </Chip>
      {plan.frequencyLabel ? (
        <Chip tone="neutral" icon={<CalendarDays size={12} aria-hidden="true" />}>
          {plan.frequencyLabel}
        </Chip>
      ) : null}
      {plan.scheduleLabel ? <Chip tone="neutral">{plan.scheduleLabel}</Chip> : null}
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
          <span className="bc-tp-exercise-prescription">{exercise.prescription}</span>
          {exercise.rest ? <span className="bc-tp-exercise-rest">{exercise.rest}</span> : null}
          {exercise.load ? <span className="bc-tp-exercise-rest">{exercise.load}</span> : null}
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
    <section className="bc-tp-template" aria-label="训练课内容" data-testid="training-template">
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
        <span className="bc-tp-template-meta">{template.exerciseCount} 个动作</span>
      </div>
      {template.stages.map((stage) => (
        <section key={stage.phase} className="bc-tp-stage" data-stage={stage.phase} aria-label={stage.label}>
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
        aria-label={proposal.kind === "create" ? "查看新计划建议" : "查看调整建议"}
        onClick={() => onOpen(proposal.proposalId)}
      >
        <Eye size={18} aria-hidden="true" />
      </button>
    </div>
  );
}

function PlanScene({
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
    <div className="bc-tp-scene" data-testid={updated ? "training-updated" : "training-plan"}>
      <PlanStatusRow plan={plan} />
      {updated ? (
        <Banner tone="info" icon={<Check size={16} aria-hidden="true" />} role="status" testId="training-receipt">
          {view.updated?.created ? "已采纳新计划 · 现在生效" : "已采纳调整 · 新修订已生效"}
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
            <PendingRow key={proposal.proposalId} proposal={proposal} onOpen={controls.onOpenProposal} />
          ))
        : null}
      <GateBanner gate={plan.gate} id={gateReasonId} />
      {template ? (
        <TemplateContent plan={plan} template={template} onSelectTemplate={controls.onSelectTemplate} />
      ) : null}
    </div>
  );
}

const CHANGE_LABELS: Record<ProposalChangeView["kind"], { label: string; tone: TrainingTone }> = {
  modified: { label: "修改", tone: "warn" },
  added: { label: "新增", tone: "ok" },
  removed: { label: "移除", tone: "bad" },
};

function ChangeCard({ change, muted }: { change: ProposalChangeView; muted: boolean }) {
  const meta = CHANGE_LABELS[change.kind];
  if (muted)
    return (
      <li className="bc-tp-change-muted" data-change={change.kind}>
        <strong>{change.name}</strong>
        <span>
          {change.kind === "modified" ? (
            <>
              {change.before} <ArrowRight size={12} aria-label="改为" /> {change.after}
            </>
          ) : change.kind === "added" ? (
            `${change.after} · 新增`
          ) : (
            `${change.before} · 移除`
          )}
        </span>
      </li>
    );
  return (
    <li className="bc-tp-change" data-change={change.kind}>
      <span className="bc-tp-change-head">
        <Chip tone={meta.tone} dot>
          {meta.label}
        </Chip>
        <strong>{change.name}</strong>
      </span>
      <span className="bc-tp-change-dose">
        {change.kind === "modified" ? (
          <>
            <s className="bc-tp-change-before">{change.before}</s>
            <ArrowRight size={12} aria-label="改为" />
            <span className="bc-tp-change-after">{change.after}</span>
          </>
        ) : change.kind === "added" ? (
          <span className="bc-tp-change-after">{change.after}</span>
        ) : (
          <s className="bc-tp-change-before">{change.before}</s>
        )}
      </span>
    </li>
  );
}

const EXPIRED_REASON_TEXT: Record<NonNullable<ProposalView["expiredReason"]>, string> = {
  timeout: "未确认，计划保持原样",
  base_revision_changed: "计划已有新的修订，这条建议未生效，计划保持原样",
  relevant_context_changed: "身体、诊断或安全情况已变化，这条建议未生效，计划保持原样",
};

function formatDay(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? ""
    : new Intl.DateTimeFormat("zh-CN", { month: "long", day: "numeric" }).format(date);
}

function ProposalScene({
  proposal,
  controls,
}: {
  proposal: ProposalView;
  controls: TrainingPlanControls;
}) {
  const expired = proposal.phase !== "pending";
  return (
    <div
      className="bc-tp-scene"
      data-testid={expired ? "training-proposal-expired" : "training-proposal"}
      data-proposal-phase={proposal.phase}
    >
      <div className="bc-dx-status">
        {expired ? (
          <>
            <Chip tone="idle" dot>
              {proposal.phase === "accepted" ? "已采纳" : proposal.phase === "rejected" ? "已拒绝" : "已过期"}
            </Chip>
            <Chip tone="neutral" icon={<Clock3 size={12} aria-hidden="true" />}>
              {formatDay(proposal.expiresAt)}
            </Chip>
          </>
        ) : (
          <>
            <Chip tone="warn" dot>
              待确认
            </Chip>
            {proposal.expiresInDays !== null ? (
              <Chip tone="neutral" icon={<Clock3 size={12} aria-hidden="true" />}>
                {proposal.expiresInDays} 天后过期
              </Chip>
            ) : null}
            {proposal.byAgent ? (
              <Chip tone="ai" icon={<Sparkle size={12} aria-hidden="true" />}>
                AI 建议
              </Chip>
            ) : null}
          </>
        )}
      </div>
      {expired && proposal.phase === "expired" && proposal.expiredReason ? (
        <Banner tone="warn" icon={<CircleHelp size={16} aria-hidden="true" />} role="status" testId="training-proposal-stale">
          {EXPIRED_REASON_TEXT[proposal.expiredReason]}
        </Banner>
      ) : null}
      {controls.error ? (
        <Banner tone="bad" icon={<X size={16} aria-hidden="true" />} role="alert">
          {controls.error}
        </Banner>
      ) : null}
      <section className="bc-dx-section" aria-label={expired ? "当时的调整" : "调整"}>
        <h4 className="bc-dx-label">
          {proposal.kind === "create" ? `新计划 · ${proposal.title}` : expired ? "当时的调整" : "调整"}
        </h4>
        <ul className={expired ? "bc-tp-changes-muted" : "bc-tp-changes"}>
          {proposal.changes.map((change) => (
            <ChangeCard key={`${change.kind}:${change.planExerciseId}`} change={change} muted={expired} />
          ))}
        </ul>
        {proposal.changes.length === 0 ? <p className="bc-dx-note">内容与当前计划一致。</p> : null}
      </section>
    </div>
  );
}

export function TrainingPlanWindow({ state, controls, canExecute }: TrainingPlanWindowProps) {
  const gateReasonId = useId();
  const view = useMemo(() => projectTrainingWindow(state), [state]);
  const plan = view.selected;
  let body: ReactNode;
  let footer: ReactNode = null;

  switch (view.scene) {
    case "error":
      body = (
        <Banner tone="bad" icon={<X size={16} aria-hidden="true" />} role="alert">
          训练计划暂时无法读取。
        </Banner>
      );
      break;
    case "loading":
    case "updating":
      body = (
        <p role="status" className="bc-window-loading">
          <LoaderCircle size={16} className="animate-spin" aria-hidden="true" />
          {view.scene === "updating" ? "正在读取新的计划修订" : "正在读取训练计划"}
        </p>
      );
      break;
    case "empty":
      body = (
        <div className="bc-dx-outcome bc-tp-empty" data-testid="training-empty">
          <div className="bc-dx-state">
            <span className="bc-dx-state-halo" aria-hidden="true">
              <Dumbbell size={22} />
            </span>
            <p className="bc-dx-state-title">还没有训练计划</p>
            <p className="bc-dx-state-text">确认诊断后，可在对话里生成训练建议</p>
            <button type="button" className="bc-dx-tonal bc-tp-chat" onClick={controls.onOpenChat}>
              去对话
            </button>
          </div>
        </div>
      );
      break;
    case "proposal":
    case "expired": {
      const proposal = view.proposal!;
      body = <ProposalScene proposal={proposal} controls={controls} />;
      footer =
        view.scene === "proposal" ? (
          <footer className="bc-window-footer bc-dx-footer" data-testid="training-proposal-decide">
            {plan ? (
              <button type="button" className="bc-dx-link bc-dx-early" onClick={controls.onBackToPlan}>
                返回计划
              </button>
            ) : null}
            <button
              type="button"
              className="bc-dx-tonal"
              disabled={controls.busy}
              onClick={() => controls.onReject(proposal.proposalId)}
            >
              拒绝
            </button>
            <button
              type="button"
              className="bc-dx-primary"
              disabled={controls.busy}
              aria-busy={controls.busy || undefined}
              onClick={() => controls.onAccept(proposal.proposalId)}
            >
              {controls.busy ? (
                <LoaderCircle size={16} className="animate-spin" aria-hidden="true" />
              ) : (
                <Check size={16} aria-hidden="true" />
              )}
              接受
            </button>
          </footer>
        ) : (
          <footer className="bc-window-footer bc-dx-footer">
            <button type="button" className="bc-dx-link bc-dx-early" onClick={controls.onOpenChat}>
              <MessageCircle size={16} aria-hidden="true" />
              重新讨论
            </button>
            <button type="button" className="bc-dx-tonal" onClick={controls.onBackToPlan}>
              返回计划
            </button>
          </footer>
        );
      break;
    }
    case "plan":
    case "updated": {
      body = (
        <PlanScene
          view={view}
          plan={plan!}
          controls={controls}
          gateReasonId={gateReasonId}
          updated={view.scene === "updated"}
        />
      );
      const template = plan!.selectedTemplate;
      const startable = plan!.canStart && canExecute && template !== null;
      footer = (
        <footer className="bc-window-footer bc-dx-footer bc-tp-footer" data-testid="training-start-footer">
          <Chip
            tone={plan!.status !== "active" ? "idle" : plan!.gate.tone}
            icon={
              plan!.gate.permits ? (
                <ShieldCheck size={13} aria-hidden="true" />
              ) : (
                <ShieldAlert size={13} aria-hidden="true" />
              )
            }
            testId="training-gate"
          >
            {plan!.status !== "active" ? `计划${plan!.statusLabel}` : plan!.gate.label}
          </Chip>
          <button
            type="button"
            className="bc-dx-primary bc-tp-start"
            aria-label="开始训练"
            aria-describedby={plan!.gate.reasons.length ? gateReasonId : undefined}
            disabled={!startable}
            onClick={() => {
              if (startable) controls.onStart(plan!.planId, template!.templateId);
            }}
          >
            <Play size={16} aria-hidden="true" />
            开始
          </button>
        </footer>
      );
      break;
    }
  }

  return (
    <section
      className="bc-business-window-body bc-dx bc-tp"
      aria-label="训练计划"
      data-testid="training-window"
      data-scene={view.scene}
    >
      <div className="bc-window-body" role="region" aria-label="当前计划内容">
        {body}
      </div>
      {footer}
    </section>
  );
}
