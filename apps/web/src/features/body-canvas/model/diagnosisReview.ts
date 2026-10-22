import type {
  NativeDiagnosisCondition,
  NativeDiagnosisRun,
} from "@/features/consultation/services/nativeDiagnosisService";

/**
 * Derived Diagnosis freshness (diagnosis.md §3.6 reviewRecommended), projected
 * for the window. It is a review signal only: it never changes the condition
 * the user adopted. `stale` cannot be put aside; `review_recommended` can be
 * dismissed ("暂不处理") until newer evidence raises it again.
 */
export type DiagnosisFreshnessState =
  NativeDiagnosisCondition["freshness"]["state"];
type FreshnessReason = NativeDiagnosisCondition["freshness"]["reasons"][number];

export interface DiagnosisReviewView {
  state: DiagnosisFreshnessState;
  /** The prompt is on screen: always for stale, unless dismissed otherwise. */
  prompt: boolean;
  dismissed: boolean;
  title: string;
  /** Distinct plain-language causes (ids and revisions stay server-side). */
  details: string[];
  canDismiss: boolean;
  evaluatedBodyStateRevision: number;
}

const REASON_LABELS: Record<FreshnessReason["code"], string> = {
  related_fact_recorded: "新增了相关身体记录",
  related_fact_corrected: "相关记录被更正",
  related_fact_withdrawn: "相关记录被撤回",
  safety_changed: "安全信息有变化",
  cited_episode_changed: "引用的症状过程有变化",
  related_episode_changed: "相关症状过程有变化",
  change_window_exceeded: "之后的变化较多，建议整体复核",
  cited_evidence_corrected: "引用的记录已被更正",
  cited_evidence_withdrawn: "引用的记录已被撤回",
  cited_evidence_unavailable: "引用的记录已不可用",
  source_claim_unavailable: "来源声明已不可用",
  assessment_source_unavailable: "AI 判断所依据的分析已不可用",
  request_source_unavailable: "分析请求的原话已不可用",
  diagnosis_revision_changed: "诊断在提案生成后已变化",
  condition_changed: "相关病情已变化",
};

function details(reasons: readonly FreshnessReason[]): string[] {
  return [...new Set(reasons.map((reason) => REASON_LABELS[reason.code]))];
}

function reviewTitle(reasons: readonly FreshnessReason[]): string {
  const facts = new Set(
    reasons
      .filter((reason) => reason.code === "related_fact_recorded")
      .map((reason) => reason.evidenceId ?? `${reason.bodyStateRevision}`),
  );
  if (facts.size > 0 && facts.size === reasons.length)
    return `${facts.size} 条新记录可能相关`;
  if (reasons.length === 1)
    return `${REASON_LABELS[reasons[0]!.code]}，建议复核`;
  return `${reasons.length} 项新变化可能相关`;
}

export function projectConditionReview(
  condition: Pick<
    NativeDiagnosisCondition,
    "freshness" | "reviewPromptDismissal" | "disposition"
  >,
): DiagnosisReviewView {
  const { freshness } = condition;
  const base = {
    state: freshness.state,
    evaluatedBodyStateRevision: freshness.evaluatedBodyStateRevision,
    details: details(freshness.reasons),
  };
  if (freshness.state === "stale")
    return {
      ...base,
      prompt: true,
      dismissed: false,
      title: "依据有变化，需要重新分析",
      canDismiss: false,
    };
  if (freshness.state === "review_recommended") {
    const dismissed = !!condition.reviewPromptDismissal;
    return {
      ...base,
      prompt: !dismissed,
      dismissed,
      title: reviewTitle(freshness.reasons),
      canDismiss: !dismissed && condition.disposition === "adopted",
    };
  }
  return {
    ...base,
    prompt: false,
    dismissed: false,
    title: "",
    canDismiss: false,
  };
}

/** An open run whose sources no longer qualify cannot be settled. */
export function projectRunReview(
  run: Pick<NativeDiagnosisRun, "freshness"> | null | undefined,
): { state: DiagnosisFreshnessState; title: string; details: string[] } | null {
  const freshness = run?.freshness;
  if (!freshness || freshness.state === "current") return null;
  return {
    state: freshness.state,
    title:
      freshness.state === "stale"
        ? "提案依据已经变化，不能再直接决定"
        : "提案生成后又有新记录，可在对话里重新分析",
    details: details(freshness.reasons),
  };
}
