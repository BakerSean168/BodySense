import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useAuthStore } from "@/stores/authStore";
import {
  appendHealthDocumentReview,
  fetchHealthDocumentReviewContext,
  fetchHealthDocumentSource,
} from "../../services/healthDocumentReview";
import type {
  DocumentIndicatorReviewProjection,
  DocumentIndicatorReviewRecord,
  HealthDocumentReviewAction,
  HealthDocumentReviewContext,
} from "../../types/health-document-review.types";
interface HealthDocumentReviewPanelProps {
  uploadId: string;
}

export function HealthDocumentReviewPanel({
  uploadId,
}: HealthDocumentReviewPanelProps) {
  const accessToken = useAuthStore((state) => state.accessToken);
  const [context, setContext] = useState<HealthDocumentReviewContext | null>();
  const [error, setError] = useState<string | null>(null);
  const [submittingIndex, setSubmittingIndex] = useState<number | null>(null);
  const [sourceIndex, setSourceIndex] = useState<number | null>(null);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [correctedValue, setCorrectedValue] = useState("");
  const [correctedUnit, setCorrectedUnit] = useState("");
  const [correctedRange, setCorrectedRange] = useState("");

  const load = useCallback(async () => {
    if (!accessToken) {
      setContext(null);
      return;
    }
    setError(null);
    try {
      setContext(await fetchHealthDocumentReviewContext(uploadId, accessToken));
    } catch (loadError) {
      setError(
        loadError instanceof Error ? loadError.message : "复核信息加载失败",
      );
    }
  }, [accessToken, uploadId]);

  useEffect(() => {
    let active = true;
    if (!accessToken) {
      setContext(null);
      return;
    }
    setError(null);
    void fetchHealthDocumentReviewContext(uploadId, accessToken)
      .then((value) => {
        if (active) setContext(value);
      })
      .catch((loadError: unknown) => {
        if (active) {
          setError(
            loadError instanceof Error ? loadError.message : "复核信息加载失败",
          );
        }
      });
    return () => {
      active = false;
    };
  }, [accessToken, uploadId]);

  const reviewItems = useMemo(() => {
    if (!context) return [];
    return context.review_candidates.filter(
      (projection) =>
        projection.candidate.evidence_admissibility.status !== "admissible" ||
        (projection.history?.length ?? 0) > 0,
    );
  }, [context]);

  const submit = async (
    projection: DocumentIndicatorReviewProjection,
    action: HealthDocumentReviewAction,
  ) => {
    if (!accessToken || !context) return;
    const sourceRefs = projection.candidate.source_refs ?? [];
    if (sourceRefs.length === 0) {
      setError("该指标缺少可验证来源，不能提交人工复核。");
      return;
    }
    let reviewedPayload: Record<string, unknown> | undefined;
    if (action === "correct") {
      const value = correctedValue.trim();
      if (!value) {
        setError("请输入修正后的指标值。");
        return;
      }
      reviewedPayload = {
        indicator_id: projection.indicator_id,
        name: projection.candidate.name,
        value,
        unit: correctedUnit.trim() || null,
        reference_range: correctedRange.trim() || null,
      };
    }
    setSubmittingIndex(projection.indicator_index);
    setError(null);
    try {
      await appendHealthDocumentReview(
        uploadId,
        context.extraction_run_id,
        {
          indicator_index: projection.indicator_index,
          indicator_id: projection.indicator_id,
          action,
          reviewed_payload: reviewedPayload,
          source_refs: sourceRefs,
          idempotency_key: crypto.randomUUID(),
        },
        accessToken,
      );
      setEditingIndex(null);
      setCorrectedValue("");
      setCorrectedUnit("");
      setCorrectedRange("");
      await load();
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "复核提交失败",
      );
    } finally {
      setSubmittingIndex(null);
    }
  };

  const openSource = async (projection: DocumentIndicatorReviewProjection) => {
    if (!accessToken || !context) return;
    setSourceIndex(projection.indicator_index);
    setError(null);
    try {
      const blob = await fetchHealthDocumentSource(
        uploadId,
        context.extraction_run_id,
        accessToken,
      );
      const objectUrl = URL.createObjectURL(blob);
      const page = projection.candidate.source_regions?.find(
        (region) => region.page_number,
      )?.page_number;
      const sourceUrl =
        page && blob.type === "application/pdf"
          ? `${objectUrl}#page=${page}`
          : objectUrl;
      window.open(sourceUrl, "_blank", "noopener,noreferrer");
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
    } catch (sourceError) {
      setError(
        sourceError instanceof Error ? sourceError.message : "原始报告加载失败",
      );
    } finally {
      setSourceIndex(null);
    }
  };

  if (!accessToken) {
    return (
      <ReviewNotice>
        当前登录状态无法读取私有报告来源；这些候选不会被标记为已人工复核。
      </ReviewNotice>
    );
  }

  if (context === undefined && !error) {
    return (
      <div className="text-xs text-gray-500" role="status">
        正在加载人工复核信息…
      </div>
    );
  }

  if (context === null) {
    return (
      <ReviewNotice>
        这是历史识别结果，缺少可验证的提取运行与来源定位，暂不能人工确认；不会作为已复核证据使用。
      </ReviewNotice>
    );
  }

  return (
    <section
      className="space-y-3"
      aria-labelledby={`health-document-review-${uploadId}`}
    >
      <div>
        <h4
          id={`health-document-review-${uploadId}`}
          className="text-sm font-medium text-gray-800"
        >
          人工复核
        </h4>
        <p className="mt-1 text-xs text-gray-500">
          请对照原始报告确认识别值。确认、修正或排除只影响证据资格，不代表医学诊断。
        </p>
      </div>

      {error && (
        <div
          role="alert"
          className="rounded-md bg-red-50 px-3 py-2 text-xs text-red-700"
        >
          {error.trim()}
        </div>
      )}

      {reviewItems.length === 0 ? (
        <p className="text-xs text-gray-500" role="status">
          当前没有需要人工处理的指标。
        </p>
      ) : (
        <div className="space-y-3">
          {reviewItems.map((projection) => {
            const submitting = submittingIndex === projection.indicator_index;
            const editing = editingIndex === projection.indicator_index;
            return (
              <article
                key={`${projection.indicator_id}-${projection.indicator_index}`}
                className="rounded-lg border border-gray-200 bg-white p-3"
              >
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <div className="text-sm font-medium text-gray-900">
                      {projection.candidate.name || projection.indicator_id}
                    </div>
                    <div className="mt-1 text-sm text-gray-700">
                      机器识别：{String(projection.candidate.value ?? "-")}
                      {projection.candidate.unit
                        ? ` ${projection.candidate.unit}`
                        : ""}
                    </div>
                    {projection.candidate.reference_range && (
                      <div className="mt-1 text-xs text-gray-500">
                        参考范围：{projection.candidate.reference_range}
                      </div>
                    )}
                  </div>
                  <ReviewState record={projection.effective_review} />
                </div>

                <SourceRegions projection={projection} />

                <div className="mt-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => void openSource(projection)}
                    disabled={sourceIndex === projection.indicator_index}
                    className="rounded-md border border-gray-300 px-2.5 py-1.5 text-xs text-gray-700 disabled:opacity-50"
                  >
                    {sourceIndex === projection.indicator_index
                      ? "打开中…"
                      : "查看原始位置"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void submit(projection, "confirm")}
                    disabled={submitting}
                    className="rounded-md bg-emerald-600 px-2.5 py-1.5 text-xs text-white disabled:opacity-50"
                  >
                    确认无误
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setEditingIndex(projection.indicator_index);
                      setCorrectedValue(
                        String(projection.candidate.value ?? ""),
                      );
                      setCorrectedUnit(projection.candidate.unit || "");
                      setCorrectedRange(
                        projection.candidate.reference_range || "",
                      );
                      setError(null);
                    }}
                    disabled={submitting}
                    className="rounded-md border border-gray-300 px-2.5 py-1.5 text-xs text-gray-700 disabled:opacity-50"
                  >
                    修改
                  </button>
                  <button
                    type="button"
                    onClick={() => void submit(projection, "reject")}
                    disabled={submitting}
                    className="rounded-md border border-red-200 px-2.5 py-1.5 text-xs text-red-700 disabled:opacity-50"
                  >
                    不是这个指标
                  </button>
                </div>

                {editing && (
                  <div className="mt-3 grid gap-2 rounded-md bg-gray-50 p-3 sm:grid-cols-3">
                    <label className="text-xs text-gray-700">
                      修正值
                      <input
                        aria-label={`${projection.candidate.name} 修正值`}
                        value={correctedValue}
                        onChange={(event) =>
                          setCorrectedValue(event.target.value)
                        }
                        className="mt-1 w-full rounded border border-gray-300 px-2 py-1.5 text-sm"
                      />
                    </label>
                    <label className="text-xs text-gray-700">
                      单位
                      <input
                        aria-label={`${projection.candidate.name} 修正单位`}
                        value={correctedUnit}
                        onChange={(event) =>
                          setCorrectedUnit(event.target.value)
                        }
                        className="mt-1 w-full rounded border border-gray-300 px-2 py-1.5 text-sm"
                      />
                    </label>
                    <label className="text-xs text-gray-700">
                      参考范围
                      <input
                        aria-label={`${projection.candidate.name} 修正参考范围`}
                        value={correctedRange}
                        onChange={(event) =>
                          setCorrectedRange(event.target.value)
                        }
                        className="mt-1 w-full rounded border border-gray-300 px-2 py-1.5 text-sm"
                      />
                    </label>
                    <div className="flex gap-2 sm:col-span-3">
                      <button
                        type="button"
                        onClick={() => void submit(projection, "correct")}
                        disabled={submitting}
                        className="rounded-md bg-blue-600 px-2.5 py-1.5 text-xs text-white disabled:opacity-50"
                      >
                        保存修正
                      </button>
                      <button
                        type="button"
                        onClick={() => setEditingIndex(null)}
                        disabled={submitting}
                        className="rounded-md border border-gray-300 px-2.5 py-1.5 text-xs text-gray-700 disabled:opacity-50"
                      >
                        取消
                      </button>
                    </div>
                  </div>
                )}

                {(projection.history?.length ?? 0) > 0 && (
                  <details className="mt-3 text-xs text-gray-600">
                    <summary className="cursor-pointer select-none">
                      查看复核历史（{projection.history?.length}）
                    </summary>
                    <ol className="mt-2 space-y-1 pl-4">
                      {projection.history?.map((record) => (
                        <li key={record.id}>
                          {formatReviewAction(record)} ·{" "}
                          {new Date(record.created_at).toLocaleString("zh-CN")}
                        </li>
                      ))}
                    </ol>
                  </details>
                )}
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function ReviewNotice({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      className="rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800"
    >
      {children}
    </div>
  );
}

function ReviewState({ record }: { record?: DocumentIndicatorReviewRecord }) {
  const label = !record
    ? "待确认"
    : record.action === "confirm"
      ? "已确认"
      : record.action === "correct"
        ? "已修正"
        : "已排除";
  return (
    <span
      className="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700"
      role="status"
    >
      状态：{label}
    </span>
  );
}

function SourceRegions({
  projection,
}: {
  projection: DocumentIndicatorReviewProjection;
}) {
  const regions = projection.candidate.source_regions ?? [];
  if (regions.length === 0) {
    return (
      <p className="mt-2 text-xs text-amber-700">
        来源定位缺失：该候选保持待复核状态。
      </p>
    );
  }
  return (
    <div
      className="mt-2 space-y-1 text-xs text-gray-600"
      aria-label="原始报告来源位置"
    >
      {regions.map((region) => (
        <div key={region.source_ref}>
          来源：
          {region.page_number ? `第 ${region.page_number} 页` : "页码未提供"}
          {region.bbox?.length === 4
            ? ` · 区域 ${region.bbox.join(", ")}`
            : " · 区域位置已记录"}
        </div>
      ))}
    </div>
  );
}

function formatReviewAction(record: DocumentIndicatorReviewRecord): string {
  if (record.action === "confirm") return "确认机器识别值";
  if (record.action === "reject") return "排除机器候选";
  const value = record.reviewed_payload?.value;
  const unit = record.reviewed_payload?.unit;
  return `修正为 ${String(value ?? "-")}${unit ? ` ${String(unit)}` : ""}`;
}
