import type { ReactNode } from "react";
import {
  Check,
  CircleHelp,
  LoaderCircle,
  MessageCircle,
  Minus,
  RotateCcw,
  ShieldCheck,
  Sparkle,
  Stethoscope,
  X,
} from "lucide-react";
import {
  CERTAINTY_LABELS,
  type ConfirmedCondition,
} from "../model/confirmedConditions";
import {
  projectDiagnosisStepper,
  type DiagnosisRunView,
  type DiagnosisStepper,
  type NativeDiagnosisWindowView,
  type ProposalChoice,
  type ProposalRow,
} from "../model/diagnosisDecisionViewModel";

export interface DiagnosisDecisionControls {
  /** Local, not yet recorded choices (the serial stepper's state). */
  selection: Record<string, ProposalChoice>;
  onSelect: (proposalId: string, choice: ProposalChoice | null) => void;
  onReview: (proposalId: string) => void;
  /** Records the local choices (plus `final`) and settles the run. */
  onSubmit: (final?: { proposalId: string; choice: ProposalChoice }) => void;
  /** Receipt / No changes: back to the current Diagnosis. */
  onDismissOutcome: () => void;
  /** Empty state: ask for a formal analysis in Chat. */
  onOpenChat?: () => void;
  busy?: boolean;
  error?: string | null;
}

export interface DiagnosisWindowProps {
  view: NativeDiagnosisWindowView;
  /** The selector is in the shared Penpot V2 toolbar; the body follows that ID. */
  selectedConditionId?: string | null;
  loading?: boolean;
  error?: string | null;
  /** Without controls the window is read-only (proposals are still never facts). */
  controls?: DiagnosisDecisionControls;
}

type Tone = "ok" | "idle" | "warn" | "bad" | "ai" | "neutral";
type MarkKind = "support" | "context" | "gap" | "counter";

function Chip({
  tone,
  children,
  testId,
}: {
  tone: Tone;
  children: ReactNode;
  testId?: string;
}) {
  return (
    <span className="bc-dx-chip" data-tone={tone} data-testid={testId}>
      {tone === "ai" ? (
        <Sparkle size={11} aria-hidden="true" fill="currentColor" />
      ) : tone === "neutral" ? null : (
        <span className="bc-dx-chip-dot" aria-hidden="true" />
      )}
      {children}
    </span>
  );
}

const MARK_ICONS: Record<MarkKind, ReactNode> = {
  support: <Check size={12} strokeWidth={2.6} />,
  context: <Minus size={12} strokeWidth={2.6} />,
  gap: <CircleHelp size={14} strokeWidth={2.2} />,
  counter: <X size={12} strokeWidth={2.6} />,
};

function Mark({ kind }: { kind: MarkKind }) {
  return (
    <span className="bc-dx-mark" data-mark={kind} aria-hidden="true">
      {MARK_ICONS[kind]}
    </span>
  );
}

function MarkRow({
  kind,
  children,
  meta,
  strong,
}: {
  kind: MarkKind;
  children: ReactNode;
  meta?: string | null;
  strong?: boolean;
}) {
  return (
    <li
      className="bc-dx-row"
      data-mark={kind}
      data-strong={strong || undefined}
    >
      <Mark kind={kind} />
      <span className="bc-dx-row-text">
        <span className="bc-dx-row-statement">{children}</span>
        {meta ? <span className="bc-dx-row-meta">{meta}</span> : null}
      </span>
    </li>
  );
}

function Section({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className="bc-dx-section" aria-label={label}>
      <h4 className="bc-dx-label">{label}</h4>
      <ul className="bc-dx-rows">{children}</ul>
    </section>
  );
}

/** Penpot centred state block: halo mark, title, description. */
function StateBlock({
  tone,
  icon,
  title,
  description,
  children,
}: {
  tone: "ok" | "idle" | "bad";
  icon: ReactNode;
  title: string;
  description?: string;
  children?: ReactNode;
}) {
  return (
    <div className="bc-dx-state" data-tone={tone}>
      <span className="bc-dx-state-halo" aria-hidden="true">
        {icon}
      </span>
      <p className="bc-dx-state-title">{title}</p>
      {description ? <p className="bc-dx-state-text">{description}</p> : null}
      {children}
    </div>
  );
}

function Banner({
  tone,
  mark,
  children,
  action,
  role,
  testId,
}: {
  tone: "warn" | "bad" | "info";
  mark: MarkKind;
  children: ReactNode;
  action?: ReactNode;
  role?: "alert" | "status";
  testId?: string;
}) {
  return (
    <div
      className="bc-dx-banner"
      data-tone={tone}
      role={role}
      data-testid={testId}
    >
      <Mark kind={mark} />
      <span className="bc-dx-banner-text">{children}</span>
      {action}
    </div>
  );
}

function ConditionCard({ condition }: { condition: ConfirmedCondition }) {
  const { cause, handling } = condition;
  const basis = [cause.summary, cause.basis].filter(
    (value): value is string => !!value,
  );
  const actions =
    handling.state === "not_confirmed" ? [] : (handling.actions ?? []);
  const cautions = [
    ...(handling.state === "not_confirmed"
      ? []
      : (handling.cautionNotes ?? [])),
    ...condition.safetyNotes,
  ];
  return (
    <article className="bc-condition" data-condition-id={condition.conditionId}>
      <h3 className="sr-only">{condition.name}</h3>
      <div className="bc-dx-status">
        <Chip tone="ok">已采纳</Chip>
        {cause.evidenceStrength ? (
          <Chip tone="ai">AI {cause.evidenceStrength}</Chip>
        ) : null}
        <Chip tone="idle">{CERTAINTY_LABELS[condition.certainty]}</Chip>
      </div>

      {basis.length > 0 ||
      cause.differential ||
      cause.counterEvidenceCount > 0 ? (
        <Section label="依据">
          {basis.map((line) => (
            <MarkRow key={line} kind="support">
              {line}
            </MarkRow>
          ))}
          {cause.differential ? (
            <MarkRow kind="context" meta="需排除">
              {cause.differential}
            </MarkRow>
          ) : null}
          {cause.counterEvidenceCount > 0 ? (
            <MarkRow kind="counter">
              {`${cause.counterEvidenceCount} 条反证需要继续核对`}
            </MarkRow>
          ) : null}
        </Section>
      ) : null}

      {cause.informationGaps.length > 0 ? (
        <Section label="信息缺口">
          {cause.informationGaps.map((gap) => (
            <MarkRow key={gap} kind="gap">
              {gap}
            </MarkRow>
          ))}
        </Section>
      ) : null}

      <Section label="处理方向">
        {handling.state !== "not_confirmed" && handling.summary ? (
          <MarkRow kind="support">{handling.summary}</MarkRow>
        ) : null}
        {actions.map((action) => (
          <MarkRow key={action} kind="support">
            {action}
          </MarkRow>
        ))}
        {cautions.map((note) => (
          <MarkRow key={note} kind="counter" strong>
            {note}
          </MarkRow>
        ))}
        {handling.state === "not_confirmed" ? (
          <MarkRow kind="context">
            还没有针对这条病情的已接受处理方案。可以在对话中一起确定。
          </MarkRow>
        ) : null}
      </Section>
      {handling.state === "condition_level" ? (
        <p className="bc-dx-note">
          以上是你采纳的分析里的处理方向，不是医疗处方；训练计划仍需单独确认。
        </p>
      ) : null}
    </article>
  );
}

/** Confirm first / second: the proposal the user decides now. */
function ProposalCard({ row }: { row: ProposalRow }) {
  const titleId = `diagnosis-proposal-${row.proposalId}-title`;
  return (
    <article
      className="bc-diagnosis-proposal"
      data-proposal-id={row.proposalId}
      data-proposal-state={row.state}
      aria-labelledby={titleId}
    >
      <header className="bc-diagnosis-proposal-head">
        <Chip tone={row.kindTone}>{row.kindLabel}</Chip>
        <p className="bc-diagnosis-proposal-title" id={titleId}>
          {row.title}
        </p>
        <span className="sr-only" data-testid="proposal-state">
          {row.stateLabel}
        </span>
      </header>
      <div className="bc-dx-status">
        {row.levelLabel ? <Chip tone="ai">AI {row.levelLabel}</Chip> : null}
        {row.supportCount > 0 ? (
          <Chip tone="neutral">支持 {row.supportCount}</Chip>
        ) : null}
        {row.counterCount > 0 ? (
          <Chip tone="neutral">反证 {row.counterCount}</Chip>
        ) : null}
        {row.state === "changed" ? <Chip tone="warn">内容已变化</Chip> : null}
      </div>
      <ul className="bc-dx-rows">
        {row.summary ? (
          <MarkRow kind={row.changeKind === "create" ? "support" : "context"}>
            {row.summary}
          </MarkRow>
        ) : null}
        {row.rationale && row.rationale !== row.summary ? (
          <MarkRow kind={row.changeKind === "create" ? "support" : "context"}>
            {row.rationale}
          </MarkRow>
        ) : null}
        {row.gaps.map((gap) => (
          <li key={gap} className="bc-dx-row" data-mark="gap" data-gap="true">
            <Mark kind="gap" />
            <span className="bc-dx-row-text bc-dx-gap">{gap}</span>
          </li>
        ))}
      </ul>
    </article>
  );
}

/** Decided / Change decision rows above the current proposal. */
function DecidedRow({
  item,
  controls,
}: {
  item: DiagnosisStepper["decided"][number];
  controls?: DiagnosisDecisionControls;
}) {
  const accepted = item.choice === "accept";
  return (
    <div
      className="bc-dx-decided"
      data-proposal-id={item.row.proposalId}
      data-proposal-state={item.row.state}
      data-choice={item.choice}
    >
      <span className="bc-dx-decided-name">{item.row.title}</span>
      <Chip tone={accepted ? "ok" : "idle"} testId="proposal-choice">
        {accepted ? "已采纳" : "未采纳"}
      </Chip>
      {!accepted ? <Chip tone="neutral">不等于排除</Chip> : null}
      {item.changeable && controls ? (
        <button
          type="button"
          className="bc-dx-icon-button"
          aria-label={`更改对“${item.row.title}”的决定`}
          disabled={controls.busy}
          onClick={() => controls.onSelect(item.row.proposalId, null)}
        >
          <RotateCcw size={16} aria-hidden="true" />
        </button>
      ) : null}
    </div>
  );
}

function DecidingScene({
  run,
  stepper,
  controls,
}: {
  run: DiagnosisRunView;
  stepper: DiagnosisStepper;
  controls?: DiagnosisDecisionControls;
}) {
  const { current, next } = stepper;
  return (
    <section
      className="bc-diagnosis-proposals"
      aria-label="待决定的分析提案"
      data-testid="diagnosis-proposals"
      data-phase={run.phase}
    >
      <div className="bc-dx-status" data-testid="diagnosis-step">
        <Chip tone="neutral">
          {stepper.step} / {stepper.total}
        </Chip>
        <Chip tone="ai">AI 建议</Chip>
        <Chip tone="idle">尚未写入</Chip>
      </div>
      {run.changed ? (
        <Banner tone="warn" mark="gap" role="alert">
          诊断在提案生成后已经变化，这些提案不能再直接决定。请在对话里请求重新分析。
        </Banner>
      ) : null}
      {stepper.decided.map((item) => (
        <DecidedRow key={item.row.proposalId} item={item} controls={controls} />
      ))}
      {current?.needsReview && controls ? (
        <Banner
          tone="warn"
          mark="gap"
          action={
            <button
              type="button"
              className="bc-dx-text-button"
              disabled={controls.busy}
              onClick={() => controls.onReview(current.proposalId)}
            >
              我已重新核对
            </button>
          }
        >
          内容已变化，需要重新核对
        </Banner>
      ) : null}
      {current ? <ProposalCard row={current} /> : null}
      {next ? (
        <p className="bc-dx-next">
          <span className="bc-dx-next-label">下一项</span>
          <span className="bc-dx-next-name">{next.title}</span>
          <Chip tone={next.kindTone}>{next.kindLabel}</Chip>
        </p>
      ) : null}
      {controls?.error ? (
        <Banner tone="bad" mark="counter" role="alert">
          {controls.error}
        </Banner>
      ) : null}
      <p className="bc-dx-note">
        以上由 AI
        分析生成，只是提案；提交并收到回执后才会写入诊断。不采纳不代表医学上已经排除。
      </p>
    </section>
  );
}

function DecidingFooter({
  run,
  stepper,
  controls,
}: {
  run: DiagnosisRunView;
  stepper: DiagnosisStepper;
  controls: DiagnosisDecisionControls;
}) {
  const { current } = stepper;
  const disabled = !run.canDecide || controls.busy;
  const waiting = controls.busy ? (
    <LoaderCircle size={14} className="animate-spin" aria-hidden="true" />
  ) : null;
  const choose = (choice: ProposalChoice) => {
    if (!current) return;
    if (stepper.isLast)
      controls.onSubmit({ proposalId: current.proposalId, choice });
    else controls.onSelect(current.proposalId, choice);
  };
  const reviewing = current?.needsReview ?? false;
  return (
    <div
      className="bc-window-footer bc-dx-footer"
      data-testid="diagnosis-decide"
    >
      {stepper.canSubmitEarly ? (
        <button
          type="button"
          className="bc-dx-link bc-dx-early"
          disabled={disabled}
          onClick={() => controls.onSubmit()}
        >
          其余暂不决定，先提交
        </button>
      ) : null}
      {current ? (
        <>
          <button
            type="button"
            className={
              stepper.isLast && !stepper.anyAccepted
                ? "bc-dx-link"
                : "bc-dx-tonal"
            }
            disabled={disabled || reviewing || !current.selectable}
            onClick={() => choose("reject")}
          >
            {stepper.isLast && !stepper.anyAccepted ? "不更改" : "不采纳"}
          </button>
          <button
            type="button"
            className="bc-dx-primary"
            disabled={disabled || reviewing || !current.selectable}
            onClick={() => choose("accept")}
          >
            {waiting ?? (
              <Check size={16} strokeWidth={2.6} aria-hidden="true" />
            )}
            {stepper.isLast ? "采纳并提交" : "采纳"}
          </button>
        </>
      ) : (
        <button
          type="button"
          className="bc-dx-primary"
          disabled={
            disabled || stepper.decided.every((item) => !item.changeable)
          }
          onClick={() => controls.onSubmit()}
        >
          {waiting ?? <Check size={16} strokeWidth={2.6} aria-hidden="true" />}
          提交
        </button>
      )}
    </div>
  );
}

function formatReceiptTime(iso: string | null): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  const parts = new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const get = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return `${get("month")}月${get("day")}日 ${get("hour")}:${get("minute")}`;
}

/** Receipt / Update receipt: only from a committed settlement receipt. */
function ReceiptScene({ run }: { run: DiagnosisRunView }) {
  const decided = run.proposals.filter(
    (row) => row.state === "committed" || row.state === "rejected",
  );
  const time = formatReceiptTime(run.committedAt);
  const receipt = run.committedReceiptId?.slice(0, 8).toUpperCase() ?? "";
  return (
    <div
      role="status"
      className="bc-dx-outcome"
      data-outcome={run.phase}
      data-testid="diagnosis-receipt"
    >
      <StateBlock
        tone="ok"
        icon={<Check size={16} strokeWidth={2.6} />}
        title="诊断已更新"
        description={
          run.phase === "updated"
            ? `${run.committedCount} 项变更已提交`
            : `${decided.length} 项决定已提交`
        }
      />
      <div className="bc-dx-receipt">
        <ul className="bc-dx-receipt-list">
          {decided.map((row) => {
            const committed = row.state === "committed";
            const label = committed
              ? row.kindLabel === "停用"
                ? "已停用"
                : "已采纳"
              : "未采纳";
            return (
              <li
                key={row.proposalId}
                className="bc-dx-receipt-row"
                data-proposal-id={row.proposalId}
                data-proposal-state={row.state}
              >
                <Mark
                  kind={
                    committed
                      ? label === "已停用"
                        ? "context"
                        : "support"
                      : "gap"
                  }
                />
                <span className="bc-dx-receipt-name">{row.title}</span>
                <Chip
                  tone={
                    committed ? (label === "已停用" ? "idle" : "ok") : "neutral"
                  }
                  testId="proposal-state"
                >
                  {label}
                </Chip>
              </li>
            );
          })}
        </ul>
        <p className="bc-dx-receipt-ref">
          回执 {receipt}
          {time ? ` · ${time}` : null}
        </p>
      </div>
    </div>
  );
}

function NoChangeScene() {
  return (
    <div
      role="status"
      className="bc-dx-outcome"
      data-outcome="closed_without_change"
      data-testid="diagnosis-no-change"
    >
      <StateBlock
        tone="idle"
        icon={<Minus size={16} strokeWidth={2.6} />}
        title="诊断未更改"
        description="你的决定已记录"
      />
    </div>
  );
}

function SafetyScene({ stopped }: { stopped: boolean }) {
  return (
    <div
      role="alert"
      className="bc-dx-outcome bc-diagnosis-safety"
      data-testid="diagnosis-safety"
      data-reason={stopped ? "safety_stopped" : "safety_review"}
    >
      <StateBlock
        tone="bad"
        icon={<ShieldCheck size={16} strokeWidth={2.4} />}
        title="分析已暂停"
        description="需要先处理一条安全提示"
      />
      <Banner tone="bad" mark="counter">
        {stopped
          ? "安全提示已停止本次分析，提案不会写入诊断"
          : "安全提示尚未处理，提案现在只能查看，不能决定"}
      </Banner>
    </div>
  );
}

function EmptyScene({ onOpenChat }: { onOpenChat?: () => void }) {
  return (
    <div
      className="bc-window-empty bc-dx-outcome"
      data-testid="diagnosis-empty"
    >
      <StateBlock
        tone="idle"
        icon={<Stethoscope size={18} strokeWidth={2.2} />}
        title="还没有诊断"
        description="记录足够后，可以在对话里请求一次正式分析"
      >
        {onOpenChat ? (
          <button
            type="button"
            className="bc-dx-icon-button"
            aria-label="在对话里请求正式分析"
            onClick={onOpenChat}
          >
            <MessageCircle size={16} aria-hidden="true" />
          </button>
        ) : null}
      </StateBlock>
    </div>
  );
}

/** Analyzing-style skeleton while the current Diagnosis loads. */
function LoadingScene() {
  return (
    <div role="status" className="bc-dx-loading" aria-label="正在读取当前诊断">
      <span className="sr-only">正在读取当前诊断</span>
      {["依据", "信息缺口", "处理方向"].map((label) => (
        <div key={label} className="bc-dx-section" aria-hidden="true">
          <span className="bc-dx-label">{label}</span>
          {[0, 1].map((line) => (
            <span key={line} className="bc-dx-skeleton-row" data-line={line}>
              <span className="bc-dx-skeleton-dot" />
              <span className="bc-dx-skeleton-bar" />
            </span>
          ))}
        </div>
      ))}
    </div>
  );
}

/**
 * V2 Diagnosis window: one user-owned aggregate (`GET /diagnosis`), one
 * selected adopted ConditionEntry at a time, plus the latest formal run's
 * Agent proposals, decided one at a time as in the Penpot Diagnosis boards.
 * Proposals are never rendered as conditions and nothing is shown as
 * committed without a settlement receipt.
 */
export function DiagnosisWindow({
  view,
  selectedConditionId = null,
  loading = false,
  error = null,
  controls,
}: DiagnosisWindowProps) {
  const { confirmed, run, safetyBlocked, safetyReview } = view;
  const selected =
    confirmed.find(
      (condition) => condition.conditionId === selectedConditionId,
    ) ??
    confirmed[0] ??
    null;
  const activeRun = run && run.proposals.length > 0 ? run : null;
  const stepper =
    activeRun?.phase === "deciding"
      ? projectDiagnosisStepper(activeRun, controls?.selection ?? {})
      : null;

  // A pending workspace safety review only gates an open formal analysis
  // (Penpot Diagnosis / Safety stopped). It never hides the user's own
  // Current/Empty Diagnosis or a settled receipt: those keep their board and
  // carry a short safety note instead.
  const safetyGated =
    safetyBlocked || (safetyReview && activeRun?.phase === "deciding");
  const safetyNote =
    safetyReview && !safetyGated ? (
      <Banner
        tone="bad"
        mark="counter"
        role="alert"
        testId="diagnosis-safety-note"
      >
        需要先处理安全提醒，之后才能请求正式分析
      </Banner>
    ) : null;

  let body: ReactNode;
  let footer: ReactNode = null;
  if (error) {
    body = (
      <Banner tone="bad" mark="counter" role="alert">
        {error}
      </Banner>
    );
  } else if (loading) {
    body = <LoadingScene />;
  } else if (safetyGated) {
    body = <SafetyScene stopped={safetyBlocked} />;
  } else if (activeRun && stepper) {
    body = (
      <DecidingScene run={activeRun} stepper={stepper} controls={controls} />
    );
    if (controls && activeRun.state === "open")
      footer = (
        <DecidingFooter run={activeRun} stepper={stepper} controls={controls} />
      );
  } else if (
    activeRun &&
    (activeRun.phase === "committed" || activeRun.phase === "updated")
  ) {
    body = <ReceiptScene run={activeRun} />;
  } else if (activeRun?.phase === "closed_without_change") {
    body = <NoChangeScene />;
  } else if (selected) {
    body = (
      <div className="bc-condition-list">
        <ConditionCard condition={selected} />
      </div>
    );
  } else {
    body = <EmptyScene onOpenChat={controls?.onOpenChat} />;
  }

  if (
    controls &&
    !error &&
    !loading &&
    !safetyGated &&
    activeRun &&
    activeRun.phase !== "deciding"
  ) {
    footer = (
      <div className="bc-window-footer bc-dx-footer">
        <button
          type="button"
          className="bc-dx-tonal"
          onClick={controls.onDismissOutcome}
        >
          {activeRun.phase === "closed_without_change"
            ? "返回当前"
            : "查看当前"}
        </button>
      </div>
    );
  }

  return (
    <section
      className="bc-business-window-body bc-dx"
      aria-label="综合诊断"
      data-testid="diagnosis-window"
    >
      <div className="bc-window-body" role="region" aria-label="当前诊断内容">
        {error || loading ? null : safetyNote}
        {body}
      </div>
      {footer}
    </section>
  );
}
