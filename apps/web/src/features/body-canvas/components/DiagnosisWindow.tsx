import { AlertTriangle, LoaderCircle } from "lucide-react";
import {
  CERTAINTY_LABELS,
  type ConfirmedCondition,
} from "../model/confirmedConditions";
import type {
  DiagnosisRunView,
  NativeDiagnosisWindowView,
  ProposalChoice,
  ProposalRow,
} from "../model/diagnosisDecisionViewModel";

export interface DiagnosisDecisionControls {
  selection: Record<string, ProposalChoice>;
  onSelect: (proposalId: string, choice: ProposalChoice | null) => void;
  onReview: (proposalId: string) => void;
  onRecordSelection: () => void;
  onSettle: () => void;
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

function CauseRow({ label, value }: { label: string; value: string | null }) {
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
        <CauseRow label="尚不明确" value={cause.informationGaps.join("；")} />
      ) : null}

      {handling.state === "condition_level" ? (
        <div className="bc-condition-handling">
          <CauseRow label="处理方向" value={handling.summary ?? null} />
          {handling.actions && handling.actions.length > 0 ? (
            <CauseRow label="建议动作" value={handling.actions.join("；")} />
          ) : null}
          {handling.cautionNotes && handling.cautionNotes.length > 0 ? (
            <CauseRow label="注意" value={handling.cautionNotes.join("；")} />
          ) : null}
          <p className="bc-condition-note">
            以上是你采纳的分析里的处理方向，不是医疗处方；训练计划仍需单独确认。
          </p>
        </div>
      ) : handling.state === "plan_level" ? (
        <div className="bc-condition-handling">
          <CauseRow label="处理方向" value={handling.summary ?? null} />
          {handling.actions && handling.actions.length > 0 ? (
            <CauseRow
              label="当前计划动作"
              value={handling.actions.join("；")}
            />
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

const CHOICE_LABELS: Record<ProposalChoice, string> = {
  accept: "采纳",
  reject: "不采纳",
};

function ProposalItem({
  row,
  controls,
}: {
  row: ProposalRow;
  controls?: DiagnosisDecisionControls;
}) {
  const choice = controls?.selection[row.proposalId] ?? null;
  const group = `diagnosis-proposal-${row.proposalId}`;
  return (
    <li
      className="bc-record-item bc-diagnosis-proposal"
      data-proposal-id={row.proposalId}
      data-proposal-state={row.state}
    >
      <p className="bc-record-statement">
        <strong>{row.title}</strong>
        {row.levelLabel ? ` · AI 判断${row.levelLabel}` : null}
      </p>
      {row.summary ? <p className="bc-window-note">{row.summary}</p> : null}
      <p className="bc-window-note" data-testid="proposal-state">
        {row.stateLabel}
      </p>
      {row.needsReview && controls ? (
        <div className="bc-record-actions">
          <button
            type="button"
            className="bc-text-button"
            disabled={controls.busy}
            onClick={() => controls.onReview(row.proposalId)}
          >
            我已重新核对
          </button>
        </div>
      ) : null}
      {row.selectable && controls ? (
        <fieldset className="bc-diagnosis-choice">
          <legend className="sr-only">对“{row.title}”的选择</legend>
          {(["accept", "reject"] as const).map((value) => (
            <label key={value}>
              <input
                type="radio"
                name={group}
                value={value}
                checked={choice === value}
                onChange={() => controls.onSelect(row.proposalId, value)}
              />
              {CHOICE_LABELS[value]}
            </label>
          ))}
          <label>
            <input
              type="radio"
              name={group}
              value=""
              checked={choice === null}
              onChange={() => controls.onSelect(row.proposalId, null)}
            />
            暂不决定
          </label>
        </fieldset>
      ) : null}
    </li>
  );
}

function settleLabel(run: DiagnosisRunView): string {
  if (run.acceptedCount === 0) return "结束本次分析，不写入诊断";
  return run.closesRemaining
    ? `写入已采纳的 ${run.acceptedCount} 条，其余不采纳`
    : `写入已采纳的 ${run.acceptedCount} 条`;
}

function ProposalPanel({
  run,
  controls,
}: {
  run: DiagnosisRunView;
  controls?: DiagnosisDecisionControls;
}) {
  const chosen = run.proposals.filter(
    (row) => row.selectable && controls?.selection[row.proposalId],
  ).length;
  return (
    <section
      className="bc-diagnosis-proposals"
      aria-label="待决定的分析提案"
      data-testid="diagnosis-proposals"
    >
      <h3 className="text-sm font-medium">分析提案</h3>
      <p className="bc-window-note">
        以下内容由 AI
        分析生成，只是提案。你决定并确认写入、收到写入回执后，才会成为诊断的一部分。
      </p>
      {run.committedReceiptId ? (
        <p role="status" className="bc-window-note">
          已写入诊断（回执 {run.committedReceiptId.slice(0, 8)}）。
        </p>
      ) : run.state !== "open" || run.outcome === "closed_without_change" ? (
        <p role="status" className="bc-window-note">
          本次分析已结束，没有写入诊断。
        </p>
      ) : null}
      {run.changed ? (
        <p role="alert" className="bc-window-note">
          诊断在提案生成后已经变化，这些提案不能再直接决定。请请求重新分析。
        </p>
      ) : null}
      <ul className="bc-record-list">
        {run.proposals.map((row) => (
          <ProposalItem key={row.proposalId} row={row} controls={controls} />
        ))}
      </ul>
      {controls?.error ? (
        <p role="alert" className="bc-error">
          {controls.error}
        </p>
      ) : null}
      {controls && run.state === "open" && !run.committedReceiptId ? (
        <div className="bc-record-actions">
          <button
            type="button"
            className="bc-button"
            disabled={!run.canDecide || chosen === 0 || controls.busy}
            onClick={controls.onRecordSelection}
          >
            {controls.busy ? (
              <LoaderCircle size={14} className="animate-spin" />
            ) : null}
            保存所选的 {chosen} 条
          </button>
          <button
            type="button"
            className="bc-text-button"
            disabled={!run.canSettle || chosen > 0 || controls.busy}
            onClick={controls.onSettle}
          >
            {settleLabel(run)}
          </button>
        </div>
      ) : null}
    </section>
  );
}

/**
 * V2 Diagnosis window: one user-owned aggregate (`GET /diagnosis`), one
 * selected adopted ConditionEntry at a time, plus the latest formal run's
 * Agent proposals. Proposals are never rendered as conditions and nothing is
 * shown as committed without a settlement receipt.
 */
export function DiagnosisWindow({
  view,
  selectedConditionId = null,
  loading = false,
  error = null,
  controls,
}: DiagnosisWindowProps) {
  const { confirmed, run, safetyBlocked } = view;
  const selected =
    confirmed.find(
      (condition) => condition.conditionId === selectedConditionId,
    ) ??
    confirmed[0] ??
    null;

  return (
    <section
      className="bc-business-window-body"
      aria-label="综合诊断"
      data-testid="diagnosis-window"
    >
      <div className="bc-window-body" role="region" aria-label="当前诊断内容">
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : loading ? (
          <p role="status" className="bc-window-loading">
            <LoaderCircle size={16} className="animate-spin" />
            正在读取当前诊断
          </p>
        ) : (
          <>
            {safetyBlocked ? (
              <div role="alert" className="bc-window-alert">
                <AlertTriangle size={16} />
                <span>
                  当前分析因安全原因停止，提案不会写入诊断。请先处理安全提醒，再请求重新分析。
                </span>
              </div>
            ) : null}
            {selected ? (
              <div className="bc-condition-list">
                <ConditionCard condition={selected} />
              </div>
            ) : (
              <div className="bc-window-empty">
                <p className="bc-window-empty-title">还没有已确认的病情</p>
                <p className="bc-window-empty-text">
                  只有你决定采纳并收到写入回执的结果才会出现在这里。AI
                  分析提案不会自动写入综合诊断。
                </p>
              </div>
            )}
            {run && run.proposals.length > 0 ? (
              <ProposalPanel run={run} controls={controls} />
            ) : null}
          </>
        )}
      </div>
    </section>
  );
}
