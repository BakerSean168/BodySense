import { useMemo, useState } from "react";
import type {
  Citation,
  DiagnosisCandidate,
  DiagnosisCandidateAssessmentState,
  DiagnosisFreshness,
} from "../types/consultation";

interface DiagnosisPanelProps {
  analysisId?: string;
  bodyStateRevision?: number;
  status?:
    "completed" | "partial" | "insufficient_information" | "safety_blocked";
  summary?: string;
  candidates: DiagnosisCandidate[];
  citations?: Citation[];
  freshness?: DiagnosisFreshness;
  onSaveAssessments?: (
    items: Array<{
      candidate_id: string;
      state: DiagnosisCandidateAssessmentState;
    }>,
  ) => Promise<void> | void;
  isSavingAssessments?: boolean;
}

const CONFIDENCE_COLORS: Record<string, string> = {
  高: "bg-emerald-400/10 text-emerald-200",
  中: "bg-amber-400/10 text-amber-200",
  低: "bg-white/[0.06] text-muted-foreground",
};

const SEVERITY_COLORS: Record<string, string> = {
  轻度: "bg-emerald-400/10 text-emerald-200",
  中度: "bg-amber-400/10 text-amber-200",
  重度: "bg-red-400/10 text-red-200",
};

const ASSESSMENT_OPTIONS: Array<{
  state: DiagnosisCandidateAssessmentState;
  label: string;
}> = [
  { state: "confirmed", label: "✓ 符合我的情况" },
  { state: "unsure", label: "? 不确定" },
  { state: "not_applicable", label: "○ 暂不符合" },
];

const FRESHNESS_LABELS: Record<string, string> = {
  fresh: "与当前身体状态一致",
  potentially_stale: "可能需要复核",
  stale: "已过期，需重新分析",
};

const FRESHNESS_STYLES: Record<string, string> = {
  fresh: "bg-emerald-400/10 text-emerald-200",
  potentially_stale: "bg-amber-400/10 text-amber-200",
  stale: "bg-red-400/10 text-red-200",
};

/**
 * DiagnosisAnalysis is immutable and pinned to one exact BodyState revision.
 *
 * Candidate assessment semantics:
 * - candidates are NOT radio buttons and there is no "pick exactly one" rule;
 * - every candidate may receive an independent user assessment;
 * - unselected/unassessed candidates remain part of the historical analysis;
 * - saving candidate assessments does not automatically generate Treatment.
 */
export function DiagnosisPanel({
  analysisId,
  status = "completed",
  summary,
  candidates,
  citations,
  freshness,
  onSaveAssessments,
  isSavingAssessments,
}: DiagnosisPanelProps) {
  const [assessments, setAssessments] = useState<
    Record<string, DiagnosisCandidateAssessmentState>
  >({});
  const [expandedCitationIndex, setExpandedCitationIndex] = useState<
    number | null
  >(null);

  const grouped = useMemo(() => {
    const groups = new Map<string, DiagnosisCandidate[]>();
    for (const diagnosis of candidates) {
      const key = diagnosis.concern_key || "general";
      groups.set(key, [...(groups.get(key) ?? []), diagnosis]);
    }
    return [...groups.entries()];
  }, [candidates]);

  if (candidates.length === 0) {
    return (
      <div className="rounded-xl border border-amber-300/12 bg-amber-300/[0.045] p-4">
        <p className="text-sm font-semibold text-amber-100/90">
          {status === "safety_blocked"
            ? "当前分析受安全状态限制"
            : "当前信息不足以形成可靠候选"}
        </p>
        {summary ? (
          <p className="mt-1 text-xs text-amber-100/60">{summary}</p>
        ) : null}
      </div>
    );
  }

  const saveItems = Object.entries(assessments)
    .filter(([candidateId]) => Boolean(candidateId))
    .map(([candidate_id, state]) => ({ candidate_id, state }));

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-foreground">可能性分析</h3>
          {summary ? (
            <p className="mt-1 text-xs text-muted-foreground">{summary}</p>
          ) : null}
        </div>
      </div>

      {freshness && freshness.state !== "fresh" ? (
        <div className="rounded-xl border border-border bg-muted/35 p-3 text-xs">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span
              className={`rounded-full px-2 py-1 font-semibold ${
                FRESHNESS_STYLES[freshness.state] || "bg-white/[0.06] text-muted-foreground"
              }`}
            >
              {FRESHNESS_LABELS[freshness.state] || freshness.state}
            </span>
          </div>
          {freshness.reasons.length > 0 ? (
            <ul className="mt-2 space-y-1 text-muted-foreground">
              {freshness.reasons.map((reason, index) => (
                <li key={`${reason.code}-${index}`}>• {reason.message}</li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}

      {grouped.map(([concernKey, candidates]) => (
        <section key={concernKey} className="space-y-2">
          {grouped.length > 1 ? (
            <h4 className="text-xs font-semibold tracking-wide text-primary">
              {concernKey === "general" ? "综合" : concernKey}
            </h4>
          ) : null}

          {candidates.map((diagnosis, index) => {
            const candidateId = diagnosis.candidate_id ?? "";
            return (
              <article
                key={candidateId || `${concernKey}-${index}`}
                className="rounded-xl border border-border bg-card/70 p-4"
              >
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <h4 className="font-medium text-foreground">
                    {diagnosis.name}
                  </h4>
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs font-medium ${CONFIDENCE_COLORS[diagnosis.confidence] || ""}`}
                  >
                    置信度：{diagnosis.confidence}
                  </span>
                  {diagnosis.severity ? (
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[diagnosis.severity] || ""}`}
                    >
                      {diagnosis.severity}
                    </span>
                  ) : null}
                  {diagnosis.evidence_strength ? (
                    <span className="rounded-full bg-sky-400/10 px-2 py-0.5 text-xs font-medium text-sky-200">
                      证据：{diagnosis.evidence_strength}
                    </span>
                  ) : null}
                </div>

                {diagnosis.basis ? (
                  <p className="mb-2 text-sm text-foreground/78">
                    {diagnosis.basis}
                  </p>
                ) : null}
                {diagnosis.reasoning_summary ? (
                  <p className="mb-2 text-xs text-muted-foreground">
                    分析：{diagnosis.reasoning_summary}
                  </p>
                ) : null}
                {diagnosis.typical_symptoms ? (
                  <p className="text-xs text-muted-foreground">
                    典型表现：{diagnosis.typical_symptoms}
                  </p>
                ) : null}
                {diagnosis.differential ? (
                  <p className="mt-1 text-xs text-muted-foreground">
                    区别说明：{diagnosis.differential}
                  </p>
                ) : null}
                {(diagnosis.counterevidence_ids?.length ?? 0) > 0 ? (
                  <p className="mt-2 text-xs text-amber-200/70">
                    存在 {diagnosis.counterevidence_ids!.length}{" "}
                    项反向/削弱证据，详情将在历史分析中保留。
                  </p>
                ) : null}

                {candidateId && onSaveAssessments ? (
                  <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-3">
                    {ASSESSMENT_OPTIONS.map((option) => (
                      <button
                        key={option.state}
                        type="button"
                        onClick={() =>
                          setAssessments((current) => ({
                            ...current,
                            [candidateId]: option.state,
                          }))
                        }
                        className={`rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors ${
                          assessments[candidateId] === option.state
                            ? "border-primary/45 bg-primary/12 text-primary"
                            : "border-border bg-transparent text-muted-foreground hover:bg-muted/70 hover:text-foreground"
                        }`}
                      >
                        {option.label}
                      </button>
                    ))}
                  </div>
                ) : null}
              </article>
            );
          })}
        </section>
      ))}

      {analysisId && onSaveAssessments ? (
        <button
          type="button"
          disabled={saveItems.length === 0 || isSavingAssessments}
          onClick={() => onSaveAssessments(saveItems)}
          className="w-full rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/85 disabled:cursor-not-allowed disabled:bg-muted disabled:text-muted-foreground"
        >
          {isSavingAssessments ? "保存中..." : "保存我的判断"}
        </button>
      ) : null}

      {citations && citations.length > 0 ? (
        <div className="border-t border-border pt-4">
          <h4 className="mb-2 text-xs font-semibold tracking-wide text-muted-foreground">
            📚 参考来源
          </h4>
          <div className="space-y-2">
            {citations.map((citation, index) => (
              <div
                key={`${citation.title}-${index}`}
                className="overflow-hidden rounded-xl border border-border bg-card/55"
              >
                <button
                  type="button"
                  onClick={() =>
                    setExpandedCitationIndex(
                      expandedCitationIndex === index ? null : index,
                    )
                  }
                  className="flex w-full items-center justify-between p-3 text-left text-xs transition-colors hover:bg-muted/60"
                >
                  <span className="font-medium text-foreground">
                    {citation.title}
                  </span>
                  <span className="text-muted-foreground">
                    {expandedCitationIndex === index ? "收起" : "查看"}
                  </span>
                </button>
                {expandedCitationIndex === index ? (
                  <div className="border-t border-border bg-muted/35 p-3 text-xs leading-relaxed text-muted-foreground">
                    {citation.summary ||
                      citation.body_markdown ||
                      citation.content}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}

    </div>
  );
}
