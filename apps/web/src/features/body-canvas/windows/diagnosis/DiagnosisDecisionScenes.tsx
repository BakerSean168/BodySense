import { Check, LoaderCircle, RotateCcw } from "lucide-react";
import type {
  DiagnosisRunView,
  DiagnosisStepper,
  ProposalChoice,
  ProposalRow,
} from "../../model/diagnosisDecisionViewModel";
import type { DiagnosisDecisionControls } from "./types";
import { Banner, Chip, Mark, MarkRow } from "./DiagnosisPrimitives";

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

export function DecidingScene({
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
      {run.review ? (
        <Banner
          tone={run.review.state === "stale" ? "warn" : "info"}
          mark="gap"
          role={run.review.state === "stale" ? "alert" : "status"}
          testId="diagnosis-run-review"
        >
          <span className="bc-dx-banner-title">{run.review.title}</span>
          {run.review.details.length > 0 ? (
            <span className="bc-dx-banner-meta">
              {run.review.details.join("；")}
            </span>
          ) : null}
        </Banner>
      ) : null}
      {run.changed && run.review?.state !== "stale" ? (
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
        以上为 AI 提案，提交后才写入。不采纳不代表医学上已经排除。
      </p>
    </section>
  );
}

export function DecidingFooter({
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
