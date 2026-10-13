import { AlertTriangle, ArrowRight, LoaderCircle, MessageCircle } from "lucide-react";
import {
  CERTAINTY_LABELS,
  type ConfirmedCondition,
  type DiagnosisView,
} from "../model/confirmedConditions";

/**
 * Diagnosis business window.
 *
 * Design contract (Penpot V5.5 / `BUSINESS_WINDOWS_IMPLEMENTATION.md`):
 * - header: single level-1 title on the left, Close on the right; no decorative
 *   business icon, no second-line subtitle, no repeat of the title as an H1;
 * - body: "已确认病情 · N 项" plus one independent entry per condition showing
 *   name, cause and handling; unconfirmed proposals are explicitly excluded and
 *   only surfaced as a pending count;
 * - no technical version numbers, revision history or version switchers;
 * - a safety block or stale analysis is stated plainly, never hidden.
 */

export interface DiagnosisWindowProps {
  view: DiagnosisView;
  loading?: boolean;
  error?: string | null;
  canRequestReview: boolean;
  onRequestReview: () => void;
  onClose: () => void;
}

function CauseRow({
  label,
  value,
}: {
  label: string;
  value: string | null;
}) {
  if (!value) return null;
  return (
    <p className="bc-condition-row">
      <span className="bc-condition-label">{label}</span>
      <span className="bc-condition-value">{value}</span>
    </p>
  );
}

function ConditionCard({ condition }: { condition: ConfirmedCondition }) {
  const { cause, handling } = condition;
  return (
    <article className="bc-condition" data-condition-id={condition.conditionId}>
      <header className="bc-condition-head">
        <h3>{condition.name}</h3>
        <span className="bc-condition-certainty">
          {CERTAINTY_LABELS[condition.certainty]}
        </span>
      </header>

      <CauseRow label="原因" value={cause.summary} />
      <CauseRow label="依据" value={cause.basis} />
      <CauseRow label="需排除" value={cause.differential} />
      {cause.evidenceStrength ? (
        <CauseRow label="证据强度" value={cause.evidenceStrength} />
      ) : null}
      {cause.counterEvidenceCount > 0 ? (
        <CauseRow
          label="反证"
          value={`${cause.counterEvidenceCount} 条需要继续核对`}
        />
      ) : null}
      {cause.informationGaps.length > 0 ? (
        <CauseRow
          label="尚不明确"
          value={cause.informationGaps.join("；")}
        />
      ) : null}

      {handling.state === "plan_level" ? (
        <div className="bc-condition-handling">
          <CauseRow label="处理方向" value={handling.summary ?? null} />
          {handling.actions && handling.actions.length > 0 ? (
            <CauseRow label="当前计划动作" value={handling.actions.join("；")} />
          ) : null}
          {handling.cautionNotes && handling.cautionNotes.length > 0 ? (
            <CauseRow label="注意" value={handling.cautionNotes.join("；")} />
          ) : null}
          <p className="bc-condition-note">
            以上为当前已接受计划的整体安排，不是针对这一条病情的独立处方。
          </p>
        </div>
      ) : (
        <p className="bc-condition-note">
          还没有针对这条病情的已接受处理方案。可以在对话中一起确定。
        </p>
      )}

      {condition.safetyNotes.length > 0 ? (
        <p className="bc-condition-safety">
          <AlertTriangle size={14} />
          {condition.safetyNotes.join("；")}
        </p>
      ) : null}
    </article>
  );
}

export function DiagnosisWindow({
  view,
  loading = false,
  error = null,
  canRequestReview,
  onRequestReview,
  onClose,
}: DiagnosisWindowProps) {
  const { confirmed, pendingProposals, safetyBlocked, needsReview, reviewReasons } =
    view;

  return (
    <section
      className="bc-business-window-body"
      aria-label="综合诊断"
      data-testid="diagnosis-window"
    >
      <div className="bc-window-body" role="region" aria-label="已确认病情">
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : loading ? (
          <p role="status" className="bc-window-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取当前诊断
          </p>
        ) : safetyBlocked ? (
          <div role="alert" className="bc-window-alert">
            <AlertTriangle size={16} />
            <span>
              当前分析因安全原因没有发布结果。请先处理安全提醒，再请求重新分析。
            </span>
          </div>
        ) : confirmed.length === 0 ? (
          <div className="bc-window-empty">
            <p className="bc-window-empty-title">还没有已确认的病情</p>
            <p className="bc-window-empty-text">
              只有你在对话中核对确认过的结果才会出现在这里。未经确认的分析不会写入综合诊断。
            </p>
          </div>
        ) : (
          <>
            <p className="bc-window-count">已确认病情 · {confirmed.length} 项</p>
            {needsReview ? (
              <p className="bc-window-note">
                已有新的身体记录，建议重新分析后再查看结论。
                {reviewReasons.length > 0 ? ` ${reviewReasons.join("；")}` : ""}
              </p>
            ) : null}
            <div className="bc-condition-list">
              {confirmed.map((condition) => (
                <ConditionCard key={condition.conditionId} condition={condition} />
              ))}
            </div>
          </>
        )}

        {pendingProposals.length > 0 ? (
          <p className="bc-window-pending" role="status">
            另有 {pendingProposals.length} 条分析提案尚未确认，不会显示为病情。
          </p>
        ) : null}
      </div>

      <footer className="bc-window-footer">
        <button
          type="button"
          className="bc-button bc-primary"
          onClick={onRequestReview}
          disabled={!canRequestReview}
        >
          <MessageCircle size={16} />
          在对话中请求复核
          <ArrowRight size={15} />
        </button>
        <button type="button" className="bc-button" onClick={onClose}>
          关闭
        </button>
      </footer>
    </section>
  );
}
