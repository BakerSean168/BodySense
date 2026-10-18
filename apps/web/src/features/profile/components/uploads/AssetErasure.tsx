import { useEffect, useMemo, useRef, useState } from "react";
import { Check, LoaderCircle, ShieldAlert, Trash2 } from "lucide-react";
import {
  assetErasureService,
  type AssetErasureReceipt,
} from "@/features/profile/services/assetErasureService";
import { errorMessage } from "@/lib/api-client";
import { projectAssetErasure } from "./assetErasureViewModel";

/**
 * AS-002 erasure of one uploaded file and every controlled copy of it. Unlike
 * source removal it goes after derived results too, and it is reported done
 * only once every target verified. Nothing a Diagnosis or plan rests on is
 * changed here: it comes back as a review.
 */
export function AssetErasure({
  assetId,
  onClose,
}: {
  assetId: string;
  onClose: (erased: boolean) => void;
}) {
  const [receipt, setReceipt] = useState<AssetErasureReceipt | null>(null);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const key = useRef(crypto.randomUUID());
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const view = useMemo(
    () => (receipt ? projectAssetErasure(receipt) : null),
    [receipt],
  );

  async function request() {
    setWorking(true);
    setError(null);
    try {
      const next = await assetErasureService.request(assetId, key.current);
      if (!mounted.current) return;
      if (next.asset_id !== assetId)
        throw new Error("删除回执与这个文件不一致。");
      setReceipt(next);
    } catch (cause) {
      if (mounted.current)
        setError(
          errorMessage(cause, "尚未确认删除结果。再次检查会继续同一次删除。"),
        );
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  const erased = view?.phase === "erased";

  return (
    <section
      className="space-y-3 border-t border-gray-200 p-4"
      aria-label="彻底删除这个文件"
      data-testid="asset-erasure"
      data-phase={view?.phase ?? (working ? "in_progress" : "confirm")}
    >
      {!receipt ? (
        <>
          <p className="flex items-center gap-2 text-sm font-medium text-gray-900">
            <Trash2 size={15} /> 彻底删除这个文件？
          </p>
          <p className="text-xs text-gray-600">
            将删除原始文件、识别与分析结果、报告指标复核、相关评估报告副本和对话运行缓存，并逐项验证。仍被诊断条目或训练计划使用的内容需要你先处理。只想删除原始文件、保留已整理的记录时，请改用“移除原件”。
          </p>
        </>
      ) : view ? (
        <>
          <p
            role="status"
            className={`flex items-center gap-2 text-sm ${erased ? "text-green-700" : "text-gray-900"}`}
          >
            {working || view.phase === "in_progress" ? (
              <LoaderCircle size={15} className="animate-spin" />
            ) : erased ? (
              <Check size={15} />
            ) : (
              <ShieldAlert size={15} />
            )}
            <strong>{view.title}</strong>
          </p>
          <p className="text-xs text-gray-600">{view.summary}</p>
          {view.outstanding.length > 0 ? (
            <ul className="space-y-1" aria-label="尚未完成的项目">
              {view.outstanding.map((line) => (
                <li key={line.target} className="text-xs text-gray-700">
                  <span className="font-medium">{line.label}</span>
                  {" · "}
                  {line.reason}
                </li>
              ))}
            </ul>
          ) : null}
          {view.reviews.length > 0 ? (
            <ul className="space-y-1" aria-label="需要你处理的内容">
              {view.reviews.map((label, index) => (
                <li key={`${label}:${index}`} className="text-xs text-gray-700">
                  {label}
                </li>
              ))}
            </ul>
          ) : null}
        </>
      ) : null}
      <div className="flex flex-wrap gap-2">
        {!receipt ? (
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md bg-red-600 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50"
            disabled={working}
            onClick={() => void request()}
          >
            {working ? (
              <LoaderCircle size={14} className="animate-spin" />
            ) : null}
            {working ? "正在删除" : "确认彻底删除"}
          </button>
        ) : null}
        {receipt && !erased ? (
          <button
            type="button"
            className="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 disabled:opacity-50"
            disabled={working}
            onClick={() => void request()}
          >
            再次检查
          </button>
        ) : null}
        <button
          type="button"
          className="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 disabled:opacity-50"
          disabled={working}
          onClick={() => onClose(erased)}
        >
          {erased ? "完成" : receipt ? "稍后再看" : "取消"}
        </button>
      </div>
      {error ? (
        <p role="alert" className="text-xs text-red-600">
          {error}
        </p>
      ) : null}
    </section>
  );
}
