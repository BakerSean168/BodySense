import { useState } from "react";
import { EmptyState } from "@/components/ui/EmptyState";
import { HealthDocumentReviewPanel } from "./HealthDocumentReviewPanel";
import type {
  HealthIndicator,
  IndicatorAdmissibilityStatus,
  OCRConfidence,
  OCRResult,
} from "../../types/upload.types";

interface OCRResultViewProps {
  result: OCRResult;
  uploadId?: string;
}

export function OCRResultView({ result, uploadId }: OCRResultViewProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  if (!result.indicators || result.indicators.length === 0) {
    return <EmptyState size="sm" title="未识别到指标候选" />;
  }

  const admissibleCount = result.indicators.filter(
    (indicator) => evidenceStatus(indicator) === "admissible",
  ).length;
  const reviewCount = result.indicators.length - admissibleCount;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium text-foreground/80">
          OCR 识别候选
        </span>
        <ConfidenceBadge confidence={result.confidence} />
        <span className="text-xs text-muted-foreground">
          {admissibleCount} 项可用于评估
          {reviewCount > 0 ? `，${reviewCount} 项待复核` : ""}
        </span>
      </div>

      <div className="rounded-lg border border-border overflow-hidden">
        <table className="min-w-full divide-y divide-border">
          <thead className="bg-muted/50">
            <tr>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                指标名称
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                数值
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                单位
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                参考范围
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                识别置信度
              </th>
              <th className="px-3 py-2 text-left text-xs font-medium text-muted-foreground uppercase">
                评估使用
              </th>
            </tr>
          </thead>
          <tbody className="bg-card divide-y divide-border">
            {result.indicators.map((indicator, index) => (
              <IndicatorRow key={index} indicator={indicator} />
            ))}
          </tbody>
        </table>
      </div>

      <p className="text-xs text-muted-foreground">
        “可用于评估”只表示识别结果通过当前证据准入规则，不代表医学正常/异常或已经由你确认。
      </p>

      {uploadId && <HealthDocumentReviewPanel uploadId={uploadId} />}

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
        <div className="bg-muted/50 rounded-lg p-3">
          <pre className="text-xs text-muted-foreground whitespace-pre-wrap font-mono">
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
    <tr className={requiresReview ? "bg-warning/15" : ""}>
      <td className="px-3 py-2 text-sm text-foreground">{indicator.name}</td>
      <td className="px-3 py-2 text-sm text-foreground font-medium">
        {indicator.value}
      </td>
      <td className="px-3 py-2 text-sm text-muted-foreground">
        {indicator.unit || "-"}
      </td>
      <td className="px-3 py-2 text-sm text-muted-foreground">
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
    high: "bg-success text-success-foreground",
    medium: "bg-warning/30 text-warning-foreground",
    low: "bg-destructive/15 text-destructive",
    unknown: "bg-muted text-foreground/80",
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
    admissible: "bg-success text-success-foreground",
    needs_review: "bg-warning/30 text-warning-foreground",
    rejected: "bg-destructive/15 text-destructive",
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
