import type {
  NativeDiagnosisProjection,
  NativeDiagnosisRun,
} from "@/features/consultation/services/nativeDiagnosisService";

/**
 * Penpot Diagnosis / History: what the user decided, newest first, built only
 * from Go records — formal runs with their decision and settlement receipts,
 * and the committed projection's inactive entries. Undecided proposals and
 * open runs are not history; nothing here is inferred from AI output alone.
 */
export type DiagnosisHistoryOutcome =
  | "adopted"
  | "deactivated"
  | "rejected"
  | "stopped";

export interface DiagnosisHistoryEntry {
  key: string;
  at: string;
  dateLabel: string;
  title: string;
  outcome: DiagnosisHistoryOutcome;
  outcomeLabel: string;
  /** AI assessment level of an adopted proposal ("AI 疑似" / "AI 较倾向"). */
  levelLabel: string | null;
  note: string | null;
}

const LEVEL_LABELS: Record<string, string> = {
  suspected: "AI 疑似",
  likely: "AI 较倾向",
};

const DISPOSITION_REASON_LABELS: Record<string, string> = {
  user_withdrawal: "本人撤回",
  superseded_by_new_conclusion: "被新结论替代",
  no_longer_applicable: "不再适用",
};

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function text(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

export function historyDateLabel(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const parts = new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
  }).formatToParts(date);
  const get = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return `${get("month")}月${get("day").padStart(2, "0")}日`;
}

function proposalTitle(
  proposal: NativeDiagnosisRun["proposals"][number],
  names: ReadonlyMap<string, string>,
): string {
  const entry = record(record(proposal.content)?.proposedEntry);
  return (
    text(entry?.medicalName) ??
    (proposal.targetConditionId
      ? names.get(proposal.targetConditionId)
      : undefined) ??
    "病情提案"
  );
}

export function projectDiagnosisHistory(
  diagnosis: NativeDiagnosisProjection | null | undefined,
  runs: readonly NativeDiagnosisRun[],
): DiagnosisHistoryEntry[] {
  const names = new Map(
    (diagnosis?.conditionEntries ?? []).map((entry) => [
      entry.conditionId,
      entry.medicalName,
    ]),
  );
  const entries: DiagnosisHistoryEntry[] = [];
  const deactivatedByRun = new Set<string>();
  for (const run of runs) {
    if (run.state === "open") continue;
    const settlement = run.settlement;
    if (run.outcome === "safety_blocked" || settlement?.outcome === "safety_blocked") {
      const at = settlement?.settledAt ?? run.startedAt;
      entries.push({
        key: `run:${run.runId}`,
        at,
        dateLabel: historyDateLabel(at),
        title: "正式分析",
        outcome: "stopped",
        outcomeLabel: "已暂停",
        levelLabel: null,
        note: "安全提示",
      });
      continue;
    }
    const committed = run.state === "committed" && settlement?.outcome === "committed";
    for (const proposal of run.proposals) {
      const decision = proposal.decision;
      if (!decision) continue;
      const content = record(proposal.content);
      const title = proposalTitle(proposal, names);
      const base = {
        key: `decision:${decision.decisionId}`,
        at: decision.decidedAt,
        dateLabel: historyDateLabel(decision.decidedAt),
        title,
      };
      if (decision.decision === "reject") {
        entries.push({
          ...base,
          outcome: "rejected",
          outcomeLabel: "未采纳",
          levelLabel: null,
          note: "提案已拒绝",
        });
        continue;
      }
      // An accepted choice is history only once its run committed.
      if (!committed) continue;
      if (
        proposal.changeKind === "set_disposition" &&
        text(content?.disposition) === "inactive"
      ) {
        if (proposal.targetConditionId)
          deactivatedByRun.add(proposal.targetConditionId);
        const reason = text(content?.dispositionReason);
        entries.push({
          ...base,
          outcome: "deactivated",
          outcomeLabel: "停用",
          levelLabel: null,
          note: (reason && DISPOSITION_REASON_LABELS[reason]) ?? null,
        });
        continue;
      }
      const level = text(record(record(content?.proposedEntry)?.assessment)?.level);
      entries.push({
        ...base,
        outcome: "adopted",
        outcomeLabel: "采纳",
        levelLabel: level ? (LEVEL_LABELS[level] ?? null) : null,
        note: null,
      });
    }
  }
  // Entries made inactive outside the loaded runs (e.g. the user's direct
  // withdrawal) are still part of the history; their cause is not guessed.
  for (const entry of diagnosis?.conditionEntries ?? []) {
    if (entry.disposition !== "inactive" || deactivatedByRun.has(entry.conditionId))
      continue;
    entries.push({
      key: `inactive:${entry.conditionId}`,
      at: entry.updatedAt,
      dateLabel: historyDateLabel(entry.updatedAt),
      title: entry.medicalName,
      outcome: "deactivated",
      outcomeLabel: "停用",
      levelLabel: null,
      note: null,
    });
  }
  return entries.sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
}
