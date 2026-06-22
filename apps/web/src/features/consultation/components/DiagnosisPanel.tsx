import { useState } from 'react';

export interface Diagnosis {
  name: string;
  confidence: '高' | '中' | '低';
  severity: '轻度' | '中度' | '重度';
  basis: string;
  typical_symptoms: string;
  differential?: string;
}

export interface TreatmentPlan {
  goal: string;
  duration_weeks: number;
  correction_exercises: {
    name: string;
    description: string;
    sets: string;
    reps: string;
    notes: string;
  }[];
  daily_habits: string[];
  nutrition_advice?: string;
  expected_timeline: string;
  warning_signs: string[];
}

interface DiagnosisPanelProps {
  diagnoses: Diagnosis[];
  treatmentPlan: TreatmentPlan | null;
  onConfirmDiagnosis: (diagnosis: Diagnosis) => void;
  onGenerateTreatment: () => void;
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
  treatmentPlan,
  onConfirmDiagnosis,
  onGenerateTreatment,
  isGeneratingTreatment,
}: DiagnosisPanelProps) {
  const [selectedDiagnosis, setSelectedDiagnosis] = useState<Diagnosis | null>(null);

  if (treatmentPlan) {
    return <TreatmentPlanView plan={treatmentPlan} />;
  }

  if (diagnoses.length === 0) {
    return null;
  }

  return (
    <div className="space-y-4">
      <h3 className="text-sm font-semibold text-gray-700">可能性分析</h3>

      {diagnoses.map((diagnosis, i) => (
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
        </div>
      ))}

      {/* Action buttons */}
      <div className="flex gap-3">
        <button
          onClick={() => selectedDiagnosis && onConfirmDiagnosis(selectedDiagnosis)}
          disabled={!selectedDiagnosis}
          className="flex-1 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white
                     hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed
                     transition-colors"
        >
          确认诊断
        </button>
        <button
          onClick={onGenerateTreatment}
          disabled={isGeneratingTreatment}
          className="flex-1 rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white
                     hover:bg-green-700 disabled:bg-gray-300 disabled:cursor-not-allowed
                     transition-colors"
        >
          {isGeneratingTreatment ? '生成中...' : '生成改善方案'}
        </button>
      </div>
    </div>
  );
}

function TreatmentPlanView({ plan }: { plan: TreatmentPlan }) {
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
            {plan.correction_exercises.map((exercise, i) => (
              <div key={i} className="bg-gray-50 rounded p-3">
                <div className="font-medium text-sm text-gray-800">
                  {exercise.name}
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
            ))}
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

      {/* Disclaimer */}
      <div className="rounded-lg bg-yellow-50 border border-yellow-200 p-3">
        <p className="text-xs text-yellow-800">
          本方案仅供参考，不构成医疗诊断或治疗方案。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
        </p>
      </div>
    </div>
  );
}
