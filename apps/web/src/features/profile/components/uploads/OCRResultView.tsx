import { useState } from "react";
import type {
  HealthIndicator,
  IndicatorAdmissibilityStatus,
  OCRConfidence,
  OCRResult,
} from "../../types/upload.types";

interface OCRResultViewProps {
  result: OCRResult;
}

export function OCRResultView({ result }: OCRResultViewProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  if (!result.indicators || result.indicators.length === 0) {
    return <div className="text-sm text-gray-500">未识别到指标候选</div>;
  }

  const admissibleCount = result.indicators.filter(
    (indicator) => evidenceStatus(indicator) === "admissible",
  ).length;
  const reviewCount = result.indicators.length - admissibleCount;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium text-gray-700">OCR 识别候选</span>
        <ConfidenceBadge confidence={result.confidence} />
        <span className="text-xs text-gray-500">
          {admissibleCount} 项可用于评估
          {reviewCount > 0 ? `，${reviewCount} 项待复核` : ""}
        </span>
      </div>

      <div className="rounded-lg border border-gray-200 overflow-hidden">
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
                识别置信度
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase">
                评估使用
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

      <p className="text-xs text-gray-500">
        “可用于评估”只表示识别结果通过当前证据准入规则，不代表医学正常/异常或已经由你确认。
      </p>

      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="text-sm text-blue-600 hover:text-blue-500 flex items-center gap-1"
      >
        <svg
          className={`w-4 h-4 transition-transform ${isExpanded ? "rotate-90" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5l7 7-7 7"
          />
        </svg>
        {isExpanded ? "隐藏原始文本" : "查看原始文本"}
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

function evidenceStatus(
  indicator: HealthIndicator,
): IndicatorAdmissibilityStatus {
  return indicator.evidence_admissibility?.status ?? "needs_review";
}

function IndicatorRow({ indicator }: { indicator: HealthIndicator }) {
  const status = evidenceStatus(indicator);
  const requiresReview = status !== "admissible";

  return (
    <tr className={requiresReview ? "bg-yellow-50" : ""}>
      <td className="px-3 py-2 text-sm text-gray-900">{indicator.name}</td>
      <td className="px-3 py-2 text-sm text-gray-900 font-medium">
        {indicator.value}
      </td>
      <td className="px-3 py-2 text-sm text-gray-500">
        {indicator.unit || "-"}
      </td>
      <td className="px-3 py-2 text-sm text-gray-500">
        {indicator.reference_range || "-"}
      </td>
      <td className="px-3 py-2">
        <ConfidenceBadge confidence={indicator.confidence} />
      </td>
      <td className="px-3 py-2">
        <EvidenceStatusBadge status={status} />
      </td>
    </tr>
  );
}

function ConfidenceBadge({ confidence }: { confidence: OCRConfidence }) {
  const styles: Record<OCRConfidence, string> = {
    high: "bg-green-100 text-green-800",
    medium: "bg-yellow-100 text-yellow-800",
    low: "bg-red-100 text-red-800",
    unknown: "bg-gray-100 text-gray-700",
  };
  const labels: Record<OCRConfidence, string> = {
    high: "高",
    medium: "中",
    low: "低",
    unknown: "未知",
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[confidence]}`}
    >
      {labels[confidence]}
    </span>
  );
}

function EvidenceStatusBadge({
  status,
}: {
  status: IndicatorAdmissibilityStatus;
}) {
  const styles: Record<IndicatorAdmissibilityStatus, string> = {
    admissible: "bg-emerald-100 text-emerald-800",
    needs_review: "bg-yellow-100 text-yellow-800",
    rejected: "bg-red-100 text-red-800",
  };
  const labels: Record<IndicatorAdmissibilityStatus, string> = {
    admissible: "可用于评估",
    needs_review: "待复核",
    rejected: "不可用于评估",
  };

  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status]}`}
    >
      {labels[status]}
    </span>
  );
}
