import { useState } from 'react';
import type { Diagnosis, TreatmentPlan, Citation } from '../services/consultationService';

interface DiagnosisPanelProps {
  diagnoses: Diagnosis[];
  citations?: Citation[];
  treatmentPlan: TreatmentPlan | null;
  onConfirmAndGenerateTreatment: (diagnosis: Diagnosis) => void;
  isGeneratingTreatment?: boolean;
}

const CONFIDENCE_COLORS: Record<string, string> = {
  '高': 'bg-green-100 text-green-800',
  '中': 'bg-yellow-100 text-yellow-800',
  '低': 'bg-gray-100 text-gray-800',
};

const SEVERITY_COLORS: Record<string, string> = {
  '轻度': 'bg-green-100 text-green-800',
  '中度': 'bg-yellow-100 text-yellow-800',
  '重度': 'bg-red-100 text-red-800',
};

export function DiagnosisPanel({
  diagnoses,
  citations,
  treatmentPlan,
  onConfirmAndGenerateTreatment,
  isGeneratingTreatment,
}: DiagnosisPanelProps) {
  const [selectedDiagnosis, setSelectedDiagnosis] = useState<Diagnosis | null>(null);
  const [expandedCitationIndex, setExpandedCitationIndex] = useState<number | null>(null);

  if (treatmentPlan) {
    return <TreatmentPlanView plan={treatmentPlan} />;
  }

  if (diagnoses.length === 0) {
    return null;
  }

  const findMatchingCitations = (diagnosisName: string, citationsList?: Citation[]) => {
    if (!citationsList) return [];
    const dName = diagnosisName.toLowerCase();
    return citationsList.filter(c => {
      const cTitle = c.title.toLowerCase();
      const cSlug = c.problem_slug?.toLowerCase() || '';
      return dName.includes(cTitle) || cTitle.includes(dName) || dName.includes(cSlug) || cSlug.includes(dName);
    });
  };

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-700">可能性分析</h3>

      {diagnoses.map((diagnosis, i) => {
        const matches = findMatchingCitations(diagnosis.name, citations);
        return (
          <div
            key={i}
            className={`rounded-lg border p-4 cursor-pointer transition-all ${
              selectedDiagnosis?.name === diagnosis.name
                ? 'border-blue-500 bg-blue-50'
                : 'border-gray-200 hover:border-gray-300'
            }`}
            onClick={() => setSelectedDiagnosis(diagnosis)}
          >
            <div className="flex items-center gap-2 mb-2">
              <h4 className="font-medium text-gray-900">{diagnosis.name}</h4>
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${CONFIDENCE_COLORS[diagnosis.confidence] || ''}`}
              >
                置信度：{diagnosis.confidence}
              </span>
              <span
                className={`rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[diagnosis.severity] || ''}`}
              >
                {diagnosis.severity}
              </span>
            </div>

            <p className="text-sm text-gray-600 mb-2">{diagnosis.basis}</p>

            {diagnosis.typical_symptoms && (
              <p className="text-xs text-gray-500">
                典型表现：{diagnosis.typical_symptoms}
              </p>
            )}

            {diagnosis.differential && (
              <p className="text-xs text-gray-500 mt-1">
                区别说明：{diagnosis.differential}
              </p>
            )}

            {matches.length > 0 && (
              <div className="mt-3 pt-3 border-t border-gray-100 flex flex-wrap gap-2 items-center text-xs text-gray-500">
                <span className="font-medium text-gray-600">📖 知识库来源:</span>
                {matches.map((citation, idx) => (
                  <span
                    key={idx}
                    className="inline-flex items-center bg-gray-100 px-2 py-0.5 rounded text-gray-700 hover:bg-gray-200 transition-colors"
                    title={citation.summary}
                  >
                    {citation.source_title} {citation.source_author ? `(${citation.source_author})` : ''}
                  </span>
                ))}
              </div>
            )}
          </div>
        );
      })}

      {/* Action buttons */}
      <div className="flex gap-3">
        <button
          onClick={() => selectedDiagnosis && onConfirmAndGenerateTreatment(selectedDiagnosis)}
          disabled={!selectedDiagnosis || isGeneratingTreatment}
          className="flex-1 rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white
                     hover:bg-green-700 disabled:bg-gray-300 disabled:cursor-not-allowed
                     transition-colors"
        >
          {isGeneratingTreatment ? '生成中...' : '确认诊断并生成改善方案'}
        </button>
      </div>

      {/* Citations list */}
      {citations && citations.length > 0 && (
        <div className="mt-6 border-t pt-4">
          <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">📚 知识库参考来源</h4>
          <div className="space-y-2">
            {citations.map((citation, idx) => (
              <div key={idx} className="border border-gray-200 rounded-lg overflow-hidden bg-white">
                <button
                  onClick={() => setExpandedCitationIndex(expandedCitationIndex === idx ? null : idx)}
                  className="w-full flex items-center justify-between p-3 text-left hover:bg-gray-50 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-blue-600 text-[10px] font-semibold bg-blue-50 px-1.5 py-0.5 rounded-full border border-blue-100">
                      {citation.category === 'definition' ? '定义' : citation.category === 'exercise' ? '训练' : '自测'}
                    </span>
                    <span className="font-medium text-sm text-gray-800">{citation.title}</span>
                  </div>
                  <div className="flex items-center gap-2 text-xs text-gray-400">
                    <span className="truncate max-w-[200px]">
                      {citation.source_title} {citation.source_author ? `· ${citation.source_author}` : ''}
                    </span>
                    <svg
                      className={`w-4 h-4 transform transition-transform ${expandedCitationIndex === idx ? 'rotate-180' : ''}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </div>
                </button>
                {expandedCitationIndex === idx && (
                  <div className="p-3 bg-gray-50 border-t border-gray-200 text-xs text-gray-600 leading-relaxed max-h-60 overflow-y-auto">
                    {citation.summary && (
                      <p className="font-medium mb-2 text-gray-700 bg-white p-2 rounded border border-gray-100">
                        【内容摘要】{citation.summary}
                      </p>
                    )}
                    <div className="prose prose-sm max-w-none text-gray-600 font-sans whitespace-pre-wrap">
                      {citation.body_markdown || citation.content}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function TreatmentPlanView({ plan }: { plan: TreatmentPlan }) {
  const [expandedCitationIndex, setExpandedCitationIndex] = useState<number | null>(null);
  const citations = plan.citations;

  const findExerciseCitations = (exerciseName: string, citationsList?: Citation[]) => {
    if (!citationsList) return [];
    const eName = exerciseName.toLowerCase();
    return citationsList.filter(c => {
      const cTitle = c.title.toLowerCase();
      const cSummary = c.summary?.toLowerCase() || '';
      const cContent = (c.body_markdown || c.content || '').toLowerCase();
      return eName.includes(cTitle) || cTitle.includes(eName) || cSummary.includes(eName) || cContent.includes(eName);
    });
  };

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-700">改善方案</h3>

      {/* Goal */}
      <div className="rounded-lg border p-4">
        <h4 className="font-medium text-gray-900 mb-1">训练目标</h4>
        <p className="text-sm text-gray-600">{plan.goal}</p>
        <p className="text-xs text-gray-500 mt-1">
          建议周期：{plan.duration_weeks} 周
        </p>
      </div>

      {/* Exercises */}
      {plan.correction_exercises?.length > 0 && (
        <div className="rounded-lg border p-4">
          <h4 className="font-medium text-gray-900 mb-3">矫正动作</h4>
          <div className="space-y-3">
            {plan.correction_exercises.map((exercise, i) => {
              const matches = findExerciseCitations(exercise.name, citations);
              return (
                <div key={i} className="bg-gray-50 rounded p-3">
                  <div className="font-medium text-sm text-gray-800 flex justify-between items-center gap-2 flex-wrap">
                    <span>{exercise.name}</span>
                    {matches.length > 0 && (
                      <span className="text-[10px] text-gray-400 font-normal truncate max-w-[200px]" title={matches[0].source_title}>
                        📚 科学依据: {matches[0].source_title}
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-gray-600 mt-1">
                    {exercise.description}
                  </p>
                  <div className="flex gap-4 mt-2 text-xs text-gray-500">
                    <span>组数：{exercise.sets}</span>
                    <span>次数：{exercise.reps}</span>
                  </div>
                  {exercise.notes && (
                    <p className="text-xs text-orange-600 mt-1">
                      ⚠️ {exercise.notes}
                    </p>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Daily habits */}
      {plan.daily_habits?.length > 0 && (
        <div className="rounded-lg border p-4">
          <h4 className="font-medium text-gray-900 mb-2">日常习惯调整</h4>
          <ul className="space-y-1">
            {plan.daily_habits.map((habit, i) => (
              <li key={i} className="text-sm text-gray-600 flex items-start gap-2">
                <span className="text-blue-500">•</span>
                {habit}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Nutrition */}
      {plan.nutrition_advice && (
        <div className="rounded-lg border p-4">
          <h4 className="font-medium text-gray-900 mb-2">饮食建议</h4>
          <p className="text-sm text-gray-600">{plan.nutrition_advice}</p>
        </div>
      )}

      {/* Timeline */}
      <div className="rounded-lg border p-4">
        <h4 className="font-medium text-gray-900 mb-2">预期改善周期</h4>
        <p className="text-sm text-gray-600">{plan.expected_timeline}</p>
      </div>

      {/* Warning signs */}
      {plan.warning_signs?.length > 0 && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4">
          <h4 className="font-medium text-red-800 mb-2">⚠️ 警示信号</h4>
          <ul className="space-y-1">
            {plan.warning_signs.map((sign, i) => (
              <li key={i} className="text-sm text-red-700 flex items-start gap-2">
                <span>•</span>
                {sign}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Citations / References */}
      {citations && citations.length > 0 && (
        <div className="rounded-lg border p-4 bg-white">
          <h4 className="font-medium text-gray-900 mb-2">📚 方案科学依据</h4>
          <div className="space-y-2">
            {citations.map((citation, idx) => (
              <div key={idx} className="border border-gray-100 rounded overflow-hidden">
                <button
                  type="button"
                  onClick={() => setExpandedCitationIndex(expandedCitationIndex === idx ? null : idx)}
                  className="w-full flex items-center justify-between p-2.5 text-left hover:bg-gray-50 transition-colors text-xs"
                >
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-gray-700">{citation.title}</span>
                  </div>
                  <div className="flex items-center gap-2 text-[11px] text-gray-400">
                    <span className="truncate max-w-[180px]">{citation.source_title} {citation.source_author ? `· ${citation.source_author}` : ''}</span>
                    <svg
                      className={`w-3 h-3 transform transition-transform flex-shrink-0 ${expandedCitationIndex === idx ? 'rotate-180' : ''}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </div>
                </button>
                {expandedCitationIndex === idx && (
                  <div className="p-3 bg-gray-50 border-t border-gray-100 text-[11px] text-gray-600 leading-relaxed max-h-40 overflow-y-auto">
                    {citation.summary && (
                      <p className="font-medium mb-1 text-gray-700 bg-white p-2 rounded border border-gray-100">
                        【摘要】{citation.summary}
                      </p>
                    )}
                    <div className="whitespace-pre-wrap font-sans text-gray-500">
                      {citation.body_markdown || citation.content}
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Disclaimer */}
      <div className="rounded-lg bg-yellow-50 border border-yellow-200 p-3">
        <p className="text-xs text-yellow-800">
          本方案仅供参考，不构成医疗诊断或治疗方案。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
        </p>
      </div>
    </div>
  );
}
