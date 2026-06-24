import { useState, useRef } from 'react';
import type { ExtractedInfo } from '../services/consultationService';
import { BodyVisualization } from './BodyVisualization';

interface InfoPanelProps {
  extractedInfo: ExtractedInfo[];
  onConfirm?: (info: ExtractedInfo) => void;
  onModify?: (index: number, info: ExtractedInfo) => void;
}

const SEVERITY_COLORS: Record<string, string> = {
  '轻度': 'bg-[#F1F5F2] text-[#4d7a64] border border-[#c5d7cc]/30',
  '中度': 'bg-[#F7F5F0] text-[#CD7B67] border border-[#E5E3DF]',
  '重度': 'bg-[#B65E49]/10 text-[#B65E49] border border-[#B65E49]/20',
};

export function InfoPanel({ extractedInfo, onConfirm, onModify }: InfoPanelProps) {
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editForm, setEditForm] = useState<ExtractedInfo | null>(null);
  const cardsContainerRef = useRef<HTMLDivElement>(null);

  // Get unique body parts for visualization
  const highlightedParts = [...new Set(extractedInfo.map((info) => info.body_part))];

  const handleEdit = (index: number) => {
    setEditingIndex(index);
    setEditForm({ ...extractedInfo[index] });
  };

  const handleSave = () => {
    if (editingIndex !== null && editForm) {
      onModify?.(editingIndex, editForm);
      setEditingIndex(null);
      setEditForm(null);
    }
  };

  const handleCancel = () => {
    setEditingIndex(null);
    setEditForm(null);
  };

  return (
    <div className="flex flex-col h-full">
      {/* Body Visualization */}
      <div className="mb-4 p-3 bg-white rounded-lg border">
        <h3 className="text-xs font-semibold text-gray-500 mb-2">身体可视化</h3>
        <BodyVisualization
          highlightedParts={highlightedParts}
          onPartClick={(part) => {
            // Find and scroll to the info card for this part
            const index = extractedInfo.findIndex((info) => info.body_part === part);
            if (index >= 0) {
              const container = cardsContainerRef.current;
              if (container) {
                const element = container.querySelector(`#info-card-${index}`) as HTMLElement;
                if (element) {
                  const containerTop = container.getBoundingClientRect().top;
                  const elementTop = element.getBoundingClientRect().top;
                  const scrollOffset = elementTop - containerTop + container.scrollTop;
                  container.scrollTo({
                    top: scrollOffset,
                    behavior: 'smooth',
                  });
                }
              }
            }
          }}
        />
      </div>

      {/* Extracted Info Cards */}
      <div ref={cardsContainerRef} className="flex-1 overflow-y-auto">
        <h3 className="text-xs font-semibold text-gray-500 mb-2">提取的症状信息</h3>

        {extractedInfo.length === 0 ? (
          <p className="text-xs text-gray-400">
            对话中提到的症状信息会在这里显示
          </p>
        ) : (
          <div className="space-y-2">
            {extractedInfo.map((info, i) => (
              <div
                key={i}
                id={`info-card-${i}`}
                className="bg-white rounded-lg p-3 shadow-sm border"
              >
                {editingIndex === i && editForm ? (
                  /* Edit mode */
                  <div className="space-y-2">
                    <div>
                      <label className="text-xs text-gray-500">部位</label>
                      <input
                        type="text"
                        value={editForm.body_part}
                        onChange={(e) =>
                          setEditForm({ ...editForm, body_part: e.target.value })
                        }
                        className="w-full text-sm border rounded px-2 py-1"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">症状类型</label>
                      <input
                        type="text"
                        value={editForm.symptom_type || ''}
                        onChange={(e) =>
                          setEditForm({ ...editForm, symptom_type: e.target.value })
                        }
                        className="w-full text-sm border rounded px-2 py-1"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">持续时间</label>
                      <input
                        type="text"
                        value={editForm.duration || ''}
                        onChange={(e) =>
                          setEditForm({ ...editForm, duration: e.target.value })
                        }
                        className="w-full text-sm border rounded px-2 py-1"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">严重程度</label>
                      <select
                        value={editForm.severity || ''}
                        onChange={(e) =>
                          setEditForm({ ...editForm, severity: e.target.value })
                        }
                        className="w-full text-sm border rounded px-2 py-1"
                      >
                        <option value="">请选择</option>
                        <option value="轻度">轻度</option>
                        <option value="中度">中度</option>
                        <option value="重度">重度</option>
                      </select>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={handleSave}
                        className="flex-1 text-xs bg-[#CD7B67] hover:bg-[#B65E49] text-white rounded-full py-1.5 font-semibold transition-all duration-300 shadow-sm shadow-[#CD7B67]/10 cursor-pointer"
                      >
                        保存
                      </button>
                      <button
                        onClick={handleCancel}
                        className="flex-1 text-xs bg-[#F7F5F0] border border-[#E5E3DF] text-[#4A554E] rounded-full py-1.5 font-semibold hover:bg-primary-50 transition-all duration-300 cursor-pointer"
                      >
                        取消
                      </button>
                    </div>
                  </div>
                ) : (
                  /* View mode */
                  <>
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="font-semibold text-sm text-primary-800">
                        {info.body_part}
                      </span>
                      {info.severity && (
                        <span
                          className={`rounded-full px-2.5 py-0.5 text-[10px] font-bold ${SEVERITY_COLORS[info.severity] || 'bg-gray-100 text-gray-800'}`}
                        >
                          {info.severity}
                        </span>
                      )}
                    </div>
                    {info.symptom_type && (
                      <div className="text-xs text-[#4A554E] font-medium mb-1">
                        症状：{info.symptom_type}
                      </div>
                    )}
                    {info.duration && (
                      <div className="text-xs text-[#4A554E] font-medium mb-1">
                        持续时间：{info.duration}
                      </div>
                    )}
                    {info.trigger && (
                      <div className="text-xs text-[#4A554E] font-medium mb-1">
                        触发场景：{info.trigger}
                      </div>
                    )}
                    {info.relief && (
                      <div className="text-xs text-[#4A554E] font-medium">
                        缓解方式：{info.relief}
                      </div>
                    )}

                    {/* Action buttons */}
                    <div className="flex gap-2 mt-3.5">
                      <button
                        onClick={() => onConfirm?.(info)}
                        className="flex-1 text-xs bg-primary-100 text-primary-900 border border-primary-200/50 rounded-full py-1.5 font-semibold hover:bg-primary-200 transition-all duration-300 cursor-pointer"
                      >
                        确认
                      </button>
                      <button
                        onClick={() => handleEdit(i)}
                        className="flex-1 text-xs bg-[#F7F5F0] border border-[#E5E3DF] text-[#4A554E] rounded-full py-1.5 font-semibold hover:bg-primary-50 transition-all duration-300 cursor-pointer"
                      >
                        修改
                      </button>
                    </div>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Medical disclaimer */}
      <div className="mt-4 p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
        <p className="text-xs text-yellow-800">
          本分析仅供参考，不构成医疗诊断。如存在持续疼痛或严重不适，建议前往专业医疗机构就诊。
        </p>
      </div>
    </div>
  );
}
