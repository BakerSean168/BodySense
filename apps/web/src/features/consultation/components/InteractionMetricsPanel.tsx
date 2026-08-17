import { useEffect, useState } from "react";
import { consultationApi } from "../services/consultationService";

interface InteractionMetrics {
  total: number;
  answered: number;
  expired: number;
  pending: number;
  answer_rate: number;
  expire_rate: number;
  avg_wait_seconds: number;
}

interface InteractionMetricsPanelProps {
  conversationId: string | null | undefined;
}

/**
 * Compact HITL observability strip (T0-1 Phase C).
 * Renders nothing when there is no conversation or no interactions yet.
 */
export function InteractionMetricsPanel({
  conversationId,
}: InteractionMetricsPanelProps) {
  const [metrics, setMetrics] = useState<InteractionMetrics | null>(null);

  useEffect(() => {
    if (!conversationId || conversationId === "new") {
      setMetrics(null);
      return;
    }
    let cancelled = false;
    consultationApi
      .getInteractionMetrics(conversationId)
      .then((data) => {
        if (!cancelled) setMetrics(data);
      })
      .catch(() => {
        if (!cancelled) setMetrics(null);
      });
    return () => {
      cancelled = true;
    };
  }, [conversationId]);

  if (!metrics || metrics.total === 0) return null;

  const waitLabel =
    metrics.avg_wait_seconds > 0
      ? `${Math.round(metrics.avg_wait_seconds)}s 平均等待`
      : "暂无等待样本";

  return (
    <div className="flex flex-wrap items-center gap-3 rounded-lg border border-[#E4EBE6] bg-[#F7FAF8] px-3 py-2 text-[11px] text-[#5D6B63]">
      <span className="font-semibold text-[#2F3A34]">追问效果</span>
      <span>回答率 {(metrics.answer_rate * 100).toFixed(0)}%</span>
      <span>过期率 {(metrics.expire_rate * 100).toFixed(0)}%</span>
      <span>
        {metrics.answered}/{metrics.total} 已答 · {metrics.pending} 待答 ·{" "}
        {metrics.expired} 过期
      </span>
      <span>{waitLabel}</span>
    </div>
  );
}
