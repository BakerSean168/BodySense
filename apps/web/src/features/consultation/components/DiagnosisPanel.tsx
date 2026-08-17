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
  高: "bg-green-100 text-green-800",
  中: "bg-yellow-100 text-yellow-800",
  低: "bg-gray-100 text-gray-800",
};

const SEVERITY_COLORS: Record<string, string> = {
  轻度: "bg-green-100 text-green-800",
  中度: "bg-yellow-100 text-yellow-800",
  重度: "bg-red-100 text-red-800",
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
  fresh: "与当前 BodyState 一致",
  potentially_stale: "可能需要复核",
  stale: "已过期，需重新分析",
};

const FRESHNESS_STYLES: Record<string, string> = {
  fresh: "bg-emerald-100 text-emerald-700",
  potentially_stale: "bg-amber-100 text-amber-700",
  stale: "bg-red-100 text-red-700",
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
  bodyStateRevision,
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
      <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
        <p className="text-sm font-semibold text-amber-900">
          {status === "safety_blocked"
            ? "当前分析受安全状态限制"
            : "当前信息不足以形成可靠候选"}
        </p>
        {summary ? (
          <p className="mt-1 text-xs text-amber-800">{summary}</p>
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
          <h3 className="text-sm font-semibold text-gray-700">可能性分析</h3>
          {summary ? (
            <p className="mt-1 text-xs text-gray-500">{summary}</p>
          ) : null}
        </div>
        {bodyStateRevision != null ? (
          <span className="shrink-0 rounded-full bg-slate-100 px-2 py-1 text-[10px] font-semibold text-slate-600">
            BodyState R{bodyStateRevision}
          </span>
        ) : null}
      </div>

      {freshness ? (
        <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span
              className={`rounded-full px-2 py-1 font-semibold ${
                FRESHNESS_STYLES[freshness.state] || "bg-gray-100 text-gray-700"
              }`}
            >
              {FRESHNESS_LABELS[freshness.state] || freshness.state}
            </span>
            <span className="text-gray-500">
              已对照 BodyState R{freshness.evaluated_against_revision}
            </span>
          </div>
          {freshness.reasons.length > 0 ? (
            <ul className="mt-2 space-y-1 text-gray-600">
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
            <h4 className="text-xs font-semibold uppercase tracking-wide text-[#709a83]">
              {concernKey === "general" ? "综合" : concernKey}
            </h4>
          ) : null}

          {candidates.map((diagnosis, index) => {
            const candidateId = diagnosis.candidate_id ?? "";
            return (
              <article
                key={candidateId || `${concernKey}-${index}`}
                className="rounded-lg border border-gray-200 bg-white p-4"
              >
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <h4 className="font-medium text-gray-900">
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
                    <span className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                      证据：{diagnosis.evidence_strength}
                    </span>
                  ) : null}
                </div>

                {diagnosis.basis ? (
                  <p className="mb-2 text-sm text-gray-600">
                    {diagnosis.basis}
                  </p>
                ) : null}
                {diagnosis.reasoning_summary ? (
                  <p className="mb-2 text-xs text-gray-500">
                    分析：{diagnosis.reasoning_summary}
                  </p>
                ) : null}
                {diagnosis.typical_symptoms ? (
                  <p className="text-xs text-gray-500">
                    典型表现：{diagnosis.typical_symptoms}
                  </p>
                ) : null}
                {diagnosis.differential ? (
                  <p className="mt-1 text-xs text-gray-500">
                    区别说明：{diagnosis.differential}
                  </p>
                ) : null}
                {(diagnosis.counterevidence_ids?.length ?? 0) > 0 ? (
                  <p className="mt-2 text-xs text-amber-700">
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
                            ? "border-primary-600 bg-primary-100 text-primary-900"
                            : "border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
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
          className="w-full rounded-lg bg-primary-700 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-primary-800 disabled:cursor-not-allowed disabled:bg-gray-300"
        >
          {isSavingAssessments ? "保存中..." : "保存我的判断"}
        </button>
      ) : null}

      {citations && citations.length > 0 ? (
        <div className="border-t pt-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">
            📚 参考来源
          </h4>
          <div className="space-y-2">
            {citations.map((citation, index) => (
              <div
                key={`${citation.title}-${index}`}
                className="overflow-hidden rounded-lg border border-gray-200 bg-white"
              >
                <button
                  type="button"
                  onClick={() =>
                    setExpandedCitationIndex(
                      expandedCitationIndex === index ? null : index,
                    )
                  }
                  className="flex w-full items-center justify-between p-3 text-left text-xs hover:bg-gray-50"
                >
                  <span className="font-medium text-gray-800">
                    {citation.title}
                  </span>
                  <span className="text-gray-400">
                    {expandedCitationIndex === index ? "收起" : "查看"}
                  </span>
                </button>
                {expandedCitationIndex === index ? (
                  <div className="border-t bg-gray-50 p-3 text-xs leading-relaxed text-gray-600">
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

      <p className="rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800">
        可能性分析仅用于辅助理解和跟踪身体状态，不构成医疗诊断。
      </p>
    </div>
  );
}
