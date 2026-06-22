import { useState } from 'react';
import type { OCRResult, HealthIndicator } from '../../types/upload.types';

interface OCRResultViewProps {
  result: OCRResult;
}

export function OCRResultView({ result }: OCRResultViewProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  if (!result.indicators || result.indicators.length === 0) {
    return (
      <div className="text-sm text-gray-500">
        未识别到健康指标
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Confidence badge */}
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium text-gray-700">识别结果</span>
        <ConfidenceBadge confidence={result.confidence} />
      </div>

      {/* Indicators table */}
      <div className="overflow-hidden border border-gray-200 rounded-lg">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                指标名称
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                数值
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                单位
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                参考范围
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                置信度
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {result.indicators.map((indicator, index) => (
              <IndicatorRow key={index} indicator={indicator} />
            ))}
          </tbody>
        </table>
      </div>

      {/* Raw text toggle */}
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="text-sm text-blue-600 hover:text-blue-500 flex items-center gap-1"
      >
        <svg
          className={`w-4 h-4 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
        </svg>
        {isExpanded ? '隐藏原始文本' : '查看原始文本'}
      </button>

      {isExpanded && (
        <div className="bg-gray-50 rounded-lg p-3">
          <pre className="text-xs text-gray-600 whitespace-pre-wrap font-mono">
            {result.raw_text}
          </pre>
        </div>
      )}
    </div>
  );
}

function IndicatorRow({ indicator }: { indicator: HealthIndicator }) {
  const isLowConfidence = indicator.confidence === 'low';

  return (
    <tr className={isLowConfidence ? 'bg-yellow-50' : ''}>
      <td className="px-3 py-2 text-sm text-gray-900">
        {indicator.name}
        {isLowConfidence && (
          <span className="ml-1 text-xs text-yellow-600">(待确认)</span>
        )}
      </td>
      <td className="px-3 py-2 text-sm text-gray-900 font-medium">
        {indicator.value}
      </td>
      <td className="px-3 py-2 text-sm text-gray-500">
        {indicator.unit || '-'}
      </td>
      <td className="px-3 py-2 text-sm text-gray-500">
        {indicator.reference_range || '-'}
      </td>
      <td className="px-3 py-2">
        <ConfidenceBadge confidence={indicator.confidence} />
      </td>
    </tr>
  );
}

function ConfidenceBadge({ confidence }: { confidence: string }) {
  const styles = {
    high: 'bg-green-100 text-green-800',
    medium: 'bg-yellow-100 text-yellow-800',
    low: 'bg-red-100 text-red-800',
  };

  const labels = {
    high: '高',
    medium: '中',
    low: '低',
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
        styles[confidence as keyof typeof styles] || styles.low
      }`}
    >
      {labels[confidence as keyof typeof labels] || confidence}
    </span>
  );
}
