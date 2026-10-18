import { useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Check, Eye, Minus, Sparkle, X } from "lucide-react";
import { nativeDiagnosisKeys } from "@/features/consultation/hooks/useDiagnosisActions";
import { nativeDiagnosisService as api } from "@/features/consultation/services/nativeDiagnosisService";
import { errorMessage } from "@/lib/api-client";
import type { ConfirmedCondition } from "../model/confirmedConditions";
import {
  projectDiagnosisHistory,
  type DiagnosisHistoryEntry,
  type DiagnosisHistoryOutcome,
} from "../model/diagnosisHistory";

const OUTCOME_MARK: Record<DiagnosisHistoryOutcome, { mark: string; icon: ReactNode; tone: string }> = {
  adopted: { mark: "support", icon: <Check size={12} strokeWidth={2.6} />, tone: "ok" },
  deactivated: { mark: "context", icon: <Minus size={12} strokeWidth={2.6} />, tone: "idle" },
  rejected: { mark: "counter", icon: <X size={12} strokeWidth={2.6} />, tone: "bad" },
  stopped: { mark: "counter", icon: <X size={12} strokeWidth={2.6} />, tone: "bad" },
};

export interface DiagnosisHistoryListProps {
  entries: readonly DiagnosisHistoryEntry[];
  inactive: readonly ConfirmedCondition[];
  loading?: boolean;
  error?: string | null;
  renderCondition: (condition: ConfirmedCondition) => ReactNode;
}

/** Penpot Diagnosis / History: decision timeline plus 已停用 entries. */
export function DiagnosisHistoryList({
  entries,
  inactive,
  loading = false,
  error = null,
  renderCondition,
}: DiagnosisHistoryListProps) {
  const [viewing, setViewing] = useState<string | null>(null);
  const shown = inactive.find((item) => item.conditionId === viewing) ?? null;
  if (shown)
    return (
      <div className="bc-dx-history-detail" data-testid="diagnosis-history-detail">
        <button
          type="button"
          className="bc-dx-text-button bc-dx-history-back"
          onClick={() => setViewing(null)}
        >
          <ArrowLeft size={15} aria-hidden="true" />
          返回历史
        </button>
        {renderCondition(shown)}
      </div>
    );
  return (
    <div className="bc-dx-history" data-testid="diagnosis-history">
      {error ? (
        <p role="alert" className="bc-dx-note">
          {error}
        </p>
      ) : loading ? (
        <p role="status" className="bc-dx-note">
          正在读取诊断历史
        </p>
      ) : entries.length === 0 ? (
        <p className="bc-dx-note" data-testid="diagnosis-history-empty">
          还没有诊断决定。正式分析的每一次采纳、未采纳和停用都会记在这里。
        </p>
      ) : (
        <ol className="bc-dx-timeline" aria-label="诊断决定记录">
          {entries.map((entry) => {
            const mark = OUTCOME_MARK[entry.outcome];
            return (
              <li
                key={entry.key}
                className="bc-dx-timeline-row"
                data-outcome={entry.outcome}
              >
                <time className="bc-dx-timeline-date" dateTime={entry.at}>
                  {entry.dateLabel}
                </time>
                <span className="bc-dx-mark" data-mark={mark.mark} aria-hidden="true">
                  {mark.icon}
                </span>
                <span className="bc-dx-timeline-body">
                  <span className="bc-dx-timeline-title">{entry.title}</span>
                  <span className="bc-dx-status">
                    <span className="bc-dx-chip" data-tone={mark.tone}>
                      <span className="bc-dx-chip-dot" aria-hidden="true" />
                      {entry.outcomeLabel}
                    </span>
                    {entry.levelLabel ? (
                      <span className="bc-dx-chip" data-tone="ai">
                        <Sparkle size={11} aria-hidden="true" fill="currentColor" />
                        {entry.levelLabel}
                      </span>
                    ) : null}
                    {entry.note ? (
                      <span className="bc-dx-chip" data-tone="neutral">
                        {entry.note}
                      </span>
                    ) : null}
                  </span>
                </span>
              </li>
            );
          })}
        </ol>
      )}
      {inactive.length > 0 ? (
        <section className="bc-dx-section bc-dx-history-inactive" aria-label="已停用">
          <h4 className="bc-dx-label">已停用</h4>
          <ul className="bc-dx-rows">
            {inactive.map((item) => (
              <li key={item.conditionId} className="bc-dx-inactive-row">
                <span className="bc-dx-inactive-name">{item.name}</span>
                <span className="bc-dx-chip" data-tone="idle">
                  <span className="bc-dx-chip-dot" aria-hidden="true" />
                  已停用
                </span>
                <button
                  type="button"
                  className="bc-dx-icon-button"
                  aria-label={`查看已停用的“${item.name}”`}
                  onClick={() => setViewing(item.conditionId)}
                >
                  <Eye size={17} aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      <p className="bc-dx-note">
        历史只记录你的决定和回执；未采纳不代表医学上已经排除，停用也不会删除原来的记录。
      </p>
    </div>
  );
}

/** Reads the recent formal runs only while History is open. */
export function DiagnosisHistoryPanel({
  inactive,
  renderCondition,
}: Pick<DiagnosisHistoryListProps, "inactive" | "renderCondition">) {
  const diagnosis = useQuery({
    queryKey: nativeDiagnosisKeys.current(),
    queryFn: api.current,
    retry: false,
  });
  const runs = useQuery({
    queryKey: nativeDiagnosisKeys.history(),
    queryFn: api.history,
    retry: false,
    staleTime: 0,
  });
  const entries = useMemo(
    () => projectDiagnosisHistory(diagnosis.data, runs.data ?? []),
    [diagnosis.data, runs.data],
  );
  const failure = runs.error ?? diagnosis.error;
  return (
    <DiagnosisHistoryList
      entries={entries}
      inactive={inactive}
      loading={runs.isPending}
      error={failure ? errorMessage(failure, "诊断历史暂时无法读取。") : null}
      renderCondition={renderCondition}
    />
  );
}
