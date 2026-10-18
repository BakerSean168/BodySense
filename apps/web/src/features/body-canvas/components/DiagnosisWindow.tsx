import {
  AlertTriangle,
  CheckCircle2,
  CircleSlash,
  LoaderCircle,
  ShieldAlert,
} from "lucide-react";
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

const CHANGE_KIND_LABELS: Record<ProposalRow["changeKind"], string> = {
  create: "新病情",
  update: "更新病情",
  set_disposition: "调整状态",
  update_relation: "调整关联",
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
  const titleId = `${group}-title`;
  return (
    <li
      className="bc-diagnosis-proposal"
      data-proposal-id={row.proposalId}
      data-proposal-state={row.state}
      aria-labelledby={titleId}
    >
      <div className="bc-diagnosis-proposal-meta">
        <span className="bc-diagnosis-proposal-position">
          提案 {row.position}/{row.total} · {CHANGE_KIND_LABELS[row.changeKind]}
        </span>
        <span
          className="bc-diagnosis-badge"
          data-state={row.state}
          data-testid="proposal-state"
        >
          {row.stateLabel}
        </span>
      </div>
      <p className="bc-diagnosis-proposal-title" id={titleId}>
        <strong>{row.title}</strong>
        {row.levelLabel ? (
          <span className="bc-diagnosis-proposal-level">
            AI 判断{row.levelLabel}
          </span>
        ) : null}
      </p>
      {row.summary ? (
        <p className="bc-diagnosis-proposal-summary">{row.summary}</p>
      ) : null}
      {row.state === "rejected" ? (
        <p className="bc-condition-note">
          不采纳只表示这次不写入诊断，不代表医学上已经排除。
        </p>
      ) : null}
      {row.needsReview && controls ? (
        <div className="bc-diagnosis-actions">
          <button
            type="button"
            className="bc-button"
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
            <label key={value} data-choice={value}>
              <input
                type="radio"
                name={group}
                value={value}
                checked={choice === value}
                onChange={() => controls.onSelect(row.proposalId, value)}
              />
              <span>{CHOICE_LABELS[value]}</span>
            </label>
          ))}
          <label data-choice="none">
            <input
              type="radio"
              name={group}
              value=""
              checked={choice === null}
              onChange={() => controls.onSelect(row.proposalId, null)}
            />
            <span>暂不决定</span>
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

/** Receipt / Update receipt / No changes: shown only from a receipt or server state. */
function RunOutcome({ run }: { run: DiagnosisRunView }) {
  if (run.phase === "committed" || run.phase === "updated") {
    const receipt = run.committedReceiptId?.slice(0, 8) ?? "";
    return (
      <div
        role="status"
        className="bc-diagnosis-outcome"
        data-outcome={run.phase}
        data-testid="diagnosis-receipt"
      >
        <CheckCircle2 size={16} aria-hidden="true" />
        <div>
          <p className="bc-diagnosis-outcome-title">
            {run.phase === "updated" ? "综合诊断已更新" : "已写入综合诊断"}
          </p>
          <p className="bc-diagnosis-outcome-text">
            {run.committedCount > 0
              ? `${run.committedCount} 条已采纳的提案已写入，`
              : null}
            写入回执 {receipt}。
          </p>
        </div>
      </div>
    );
  }
  if (run.phase === "closed_without_change") {
    return (
      <div
        role="status"
        className="bc-diagnosis-outcome"
        data-outcome={run.phase}
        data-testid="diagnosis-no-change"
      >
        <CircleSlash size={16} aria-hidden="true" />
        <div>
          <p className="bc-diagnosis-outcome-title">
            本次分析已结束，没有写入诊断
          </p>
          <p className="bc-diagnosis-outcome-text">
            综合诊断保持不变；未采纳的提案不会出现在病情列表里。
          </p>
        </div>
      </div>
    );
  }
  return null;
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
  const deciding = run.phase === "deciding";
  return (
    <section
      className="bc-diagnosis-proposals"
      aria-label="待决定的分析提案"
      data-testid="diagnosis-proposals"
      data-phase={run.phase}
    >
      <div className="bc-diagnosis-proposals-head">
        <h3>分析提案</h3>
        {deciding ? (
          <span className="bc-diagnosis-proposals-count">
            待决定 {run.undecidedCount} / {run.proposals.length}
          </span>
        ) : null}
      </div>
      <p className="bc-window-note">
        以下内容由 AI
        分析生成，只是提案。你决定并确认写入、收到写入回执后，才会成为诊断的一部分。
      </p>
      <RunOutcome run={run} />
      {run.changed && deciding ? (
        <p role="alert" className="bc-window-alert">
          <AlertTriangle size={16} aria-hidden="true" />
          <span>
            诊断在提案生成后已经变化，这些提案不能再直接决定。请请求重新分析。
          </span>
        </p>
      ) : null}
      <ul className="bc-diagnosis-proposal-list">
        {run.proposals.map((row) => (
          <ProposalItem key={row.proposalId} row={row} controls={controls} />
        ))}
      </ul>
      {controls?.error ? (
        <p role="alert" className="bc-error">
          {controls.error}
        </p>
      ) : null}
      {controls && run.state === "open" && deciding ? (
        <div className="bc-diagnosis-actions" data-testid="diagnosis-decide">
          <button
            type="button"
            className="bc-button bc-primary"
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
            className="bc-button"
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
  const { confirmed, run, safetyBlocked, safetyReview } = view;
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
            {safetyBlocked || safetyReview ? (
              <div
                role="alert"
                className="bc-window-alert bc-diagnosis-safety"
                data-testid="diagnosis-safety"
                data-reason={safetyBlocked ? "safety_stopped" : "safety_review"}
              >
                <ShieldAlert size={16} aria-hidden="true" />
                <div>
                  <p className="bc-diagnosis-outcome-title">
                    {safetyBlocked
                      ? "分析已因安全原因停止"
                      : "需要先处理安全提醒"}
                  </p>
                  <p className="bc-diagnosis-outcome-text">
                    {safetyBlocked
                      ? "当前分析因安全原因停止，提案不会写入诊断。请先处理安全提醒，再请求重新分析。"
                      : "安全提醒尚未处理，分析提案现在只能查看，不能决定或写入诊断。"}
                  </p>
                </div>
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
