import { useState } from 'react';
import type { ExtractedInfo } from '../services/consultationService';
import { BodyVisualization } from './BodyVisualization';

interface InfoPanelProps {
  extractedInfo: ExtractedInfo[];
  onConfirm?: (info: ExtractedInfo) => void;
  onModify?: (index: number, info: ExtractedInfo) => void;
}

const SEVERITY_COLORS: Record<string, string> = {
  '轻度': 'bg-green-100 text-green-800',
  '中度': 'bg-yellow-100 text-yellow-800',
  '重度': 'bg-red-100 text-red-800',
};

export function InfoPanel({ extractedInfo, onConfirm, onModify }: InfoPanelProps) {
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editForm, setEditForm] = useState<ExtractedInfo | null>(null);

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
              document.getElementById(`info-card-${index}`)?.scrollIntoView({
                behavior: 'smooth',
                block: 'nearest',
              });
            }
          }}
        />
      </div>

      {/* Extracted Info Cards */}
      <div className="flex-1 overflow-y-auto">
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
                        className="flex-1 text-xs bg-blue-600 text-white rounded py-1 hover:bg-blue-700"
                      >
                        保存
                      </button>
                      <button
                        onClick={handleCancel}
                        className="flex-1 text-xs bg-gray-100 text-gray-600 rounded py-1 hover:bg-gray-200"
                      >
                        取消
                      </button>
                    </div>
                  </div>
                ) : (
                  /* View mode */
                  <>
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-medium text-sm text-blue-700">
                        {info.body_part}
                      </span>
                      {info.severity && (
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[info.severity] || 'bg-gray-100 text-gray-800'}`}
                        >
                          {info.severity}
                        </span>
                      )}
                    </div>
                    {info.symptom_type && (
                      <div className="text-xs text-gray-600">
                        症状：{info.symptom_type}
                      </div>
                    )}
                    {info.duration && (
                      <div className="text-xs text-gray-600">
                        持续时间：{info.duration}
                      </div>
                    )}
                    {info.trigger && (
                      <div className="text-xs text-gray-600">
                        触发场景：{info.trigger}
                      </div>
                    )}
                    {info.relief && (
                      <div className="text-xs text-gray-600">
                        缓解方式：{info.relief}
                      </div>
                    )}

                    {/* Action buttons */}
                    <div className="flex gap-2 mt-2">
                      <button
                        onClick={() => onConfirm?.(info)}
                        className="flex-1 text-xs bg-green-50 text-green-700 rounded py-1 hover:bg-green-100 transition-colors"
                      >
                        确认
                      </button>
                      <button
                        onClick={() => handleEdit(i)}
                        className="flex-1 text-xs bg-gray-50 text-gray-600 rounded py-1 hover:bg-gray-100 transition-colors"
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
