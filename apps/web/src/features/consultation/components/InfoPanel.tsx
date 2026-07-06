import type { HealthFeatureItem, HealthFeatures } from '../types/consultation';
import { BodyVisualization } from './BodyVisualization';

interface InfoPanelProps {
  healthFeatures: HealthFeatures;
  onConfirm?: (category: keyof HealthFeatures, index: number) => void;
  onModify?: (category: keyof HealthFeatures, index: number, item: HealthFeatureItem) => void;
  onDelete?: (category: keyof HealthFeatures, index: number) => void;
}

const CATEGORY_LABELS: Record<keyof HealthFeatures, string> = {
  posture_findings: '姿态观察',
  discomforts: '不适与症状',
  negative_findings: '阴性信息',
  movement_limitations: '活动受限',
  red_flags: '风险提示',
  user_answers: '补充回答',
};

const CATEGORY_ORDER: Array<keyof HealthFeatures> = [
  'posture_findings',
  'discomforts',
  'negative_findings',
  'movement_limitations',
  'red_flags',
  'user_answers',
];

function getTotalCount(healthFeatures: HealthFeatures) {
  return CATEGORY_ORDER.reduce((total, category) => total + healthFeatures[category].length, 0);
}

function getHighlightedParts(healthFeatures: HealthFeatures) {
  const parts = new Set<string>();
  for (const category of CATEGORY_ORDER) {
    for (const item of healthFeatures[category]) {
      if (item.body_part) {
        parts.add(item.body_part);
      }
    }
  }
  return [...parts];
}

export function InfoPanel({ healthFeatures, onConfirm, onModify, onDelete }: InfoPanelProps) {
  const highlightedParts = getHighlightedParts(healthFeatures);
  const totalCount = getTotalCount(healthFeatures);

  return (
    <div className="flex h-full flex-col">
      <div className="mb-4 rounded-lg border bg-white p-3">
        <h3 className="mb-2 text-xs font-semibold text-gray-500">身体可视化</h3>
        <BodyVisualization highlightedParts={highlightedParts} />
      </div>

      <div className="flex-1 overflow-y-auto">
        <h3 className="mb-2 text-xs font-semibold text-gray-500">结构化健康特征</h3>

        {totalCount === 0 ? (
          <p className="text-xs text-gray-400">对话中的体态观察、补充回答和不适信息会在这里沉淀</p>
        ) : (
          <div className="space-y-4">
            {CATEGORY_ORDER.map((category) => {
              const items = healthFeatures[category];
              if (items.length === 0) {
                return null;
              }

              return (
                <section key={category} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-semibold text-[#4A554E]">{CATEGORY_LABELS[category]}</h4>
                    <span className="rounded-full bg-[#F7F5F0] px-2 py-0.5 text-[10px] font-semibold text-[#709a83]">
                      {items.length}
                    </span>
                  </div>

                  {items.map((item, index) => (
                    <div key={`${category}-${index}`} className="rounded-lg border bg-white p-3 shadow-sm">
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <div className="text-sm font-semibold text-primary-800">{item.label}</div>
                          {item.body_part ? (
                            <div className="mt-1 text-xs font-medium text-[#709a83]">{item.body_part}</div>
                          ) : null}
                        </div>
                        {item.confirmed ? (
                          <span className="text-xs font-semibold text-primary-700">已确认</span>
                        ) : null}
                      </div>

                      {item.value ? (
                        <div className="mt-2 text-xs font-medium text-[#4A554E]">结论：{item.value}</div>
                      ) : null}
                      {item.details ? (
                        <div className="mt-1 text-xs text-[#4A554E]">{item.details}</div>
                      ) : null}
                      {item.source ? (
                        <div className="mt-2 text-[11px] text-gray-400">来源：{item.source}</div>
                      ) : null}

                      <div className="mt-3 flex gap-2">
                        {!item.confirmed && onConfirm ? (
                          <button
                            onClick={() => onConfirm(category, index)}
                            className="flex-1 rounded-full border border-primary-200/50 bg-primary-100 py-1.5 text-xs font-semibold text-primary-900 transition-all duration-300 hover:bg-primary-200"
                          >
                            确认
                          </button>
                        ) : null}
                        {onModify ? (
                          <button
                            onClick={() => onModify(category, index, item)}
                            className="flex-1 rounded-full border border-[#E5E3DF] bg-[#F7F5F0] py-1.5 text-xs font-semibold text-[#4A554E] transition-all duration-300 hover:bg-primary-50"
                          >
                            标记
                          </button>
                        ) : null}
                        {onDelete ? (
                          <button
                            onClick={() => onDelete(category, index)}
                            className="rounded-full border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs font-semibold text-rose-700 transition-all duration-300 hover:bg-rose-100"
                          >
                            删除
                          </button>
                        ) : null}
                      </div>
                    </div>
                  ))}
                </section>
              );
            })}
          </div>
        )}
      </div>

      <div className="mt-4 rounded-lg border border-yellow-200 bg-yellow-50 p-3">
        <p className="text-xs text-yellow-800">
          本分析仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
        </p>
      </div>
    </div>
  );
}
