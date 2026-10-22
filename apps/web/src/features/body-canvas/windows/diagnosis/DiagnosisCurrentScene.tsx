import { RotateCcw } from "lucide-react";
import {
  CERTAINTY_LABELS,
  type ConfirmedCondition,
} from "../../model/confirmedConditions";
import type { DiagnosisDecisionControls } from "./types";
import { Banner, Chip, MarkRow, Section } from "./DiagnosisPrimitives";

/** Penpot Diagnosis / Current review row: "2 条新记录可能相关" + ↻. */
function ReviewBanner({
  condition,
  controls,
}: {
  condition: ConfirmedCondition;
  controls?: DiagnosisDecisionControls;
}) {
  const review = condition.review;
  if (!review?.prompt) return null;
  const stale = review.state === "stale";
  const requestAnalysis = controls?.onRequestAnalysis ?? controls?.onOpenChat;
  return (
    <Banner
      tone={stale ? "bad" : "warn"}
      mark={stale ? "counter" : "gap"}
      role="status"
      testId="diagnosis-review"
      action={
        <span className="bc-dx-banner-actions">
          {review.canDismiss && controls?.onDismissReview ? (
            <button
              type="button"
              className="bc-dx-text-button"
              disabled={controls.busy}
              onClick={() => controls.onDismissReview?.(condition.conditionId)}
            >
              暂不处理
            </button>
          ) : null}
          {requestAnalysis ? (
            <button
              type="button"
              className="bc-dx-icon-button"
              aria-label="在对话里请求复核分析"
              title="在对话里请求复核分析"
              disabled={controls?.busy}
              onClick={requestAnalysis}
            >
              <RotateCcw size={16} aria-hidden="true" />
            </button>
          ) : null}
        </span>
      }
    >
      <span className="bc-dx-banner-title">{review.title}</span>
      {stale || review.details.length > 1 ? (
        <span className="bc-dx-banner-meta">{review.details.join("；")}</span>
      ) : null}
    </Banner>
  );
}

export function ConditionCard({
  condition,
  controls,
}: {
  condition: ConfirmedCondition;
  controls?: DiagnosisDecisionControls;
}) {
  const { cause, handling } = condition;
  const inactive = condition.disposition === "inactive";
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
    <article
      className="bc-condition"
      data-condition-id={condition.conditionId}
      data-freshness={condition.review?.state}
    >
      <h3 className="sr-only">{condition.name}</h3>
      <div className="bc-dx-status">
        <Chip tone={inactive ? "idle" : "ok"}>
          {inactive ? "已停用" : "已采纳"}
        </Chip>
        {cause.evidenceStrength ? (
          <Chip tone="ai">AI {cause.evidenceStrength}</Chip>
        ) : null}
        <Chip tone="idle">{CERTAINTY_LABELS[condition.certainty]}</Chip>
      </div>
      {inactive ? null : (
        <ReviewBanner condition={condition} controls={controls} />
      )}
      {!inactive && condition.review?.state === "stale" ? (
        <p className="bc-dx-note" data-testid="diagnosis-stale-note">
          重新分析并确认前，训练计划不会基于这条病情生成；你采纳的内容不会被自动修改。
        </p>
      ) : null}

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
          <MarkRow kind="context">还没有已接受的处理方案。</MarkRow>
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
