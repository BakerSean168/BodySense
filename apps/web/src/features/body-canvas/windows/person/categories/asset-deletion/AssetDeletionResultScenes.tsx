import { Check, CircleHelp, LoaderCircle, Minus } from "lucide-react";
import type { ProfileUploadLike } from "../../../../model/profileCategories";
import {
  AssetDeletionLayout,
  AssetFileCard,
  AssetList,
} from "./AssetDeletionPrimitives";
import type { AssetDeletionFlow } from "./useAssetDeletionFlow";

/** Server receipts, never a local timer, decide if erasure is completed. */
export function AssetDeletionResultScenes({
  upload,
  flow,
  backLabel,
  onBack,
  onErased,
}: {
  upload: ProfileUploadLike;
  flow: AssetDeletionFlow;
  backLabel: string;
  onBack: () => void;
  onErased: (assetId: string) => void;
}) {
  const {
    scene,
    setScene,
    receipt,
    working,
    error,
    view,
    families,
    requestErasure,
  } = flow;
  const common = { scene, working, error, onBack };

  if (scene === "source-removed") {
    return (
      <AssetDeletionLayout
        {...common}
        title="原件已移除"
        phase="source_removed"
        footer={
          <>
            <button
              type="button"
              className="bc-person-footer-link"
              onClick={() => setScene("confirm-erase")}
            >
              彻底清理
            </button>
            <button
              type="button"
              className="bc-person-footer-action"
              onClick={onBack}
            >
              {backLabel}
            </button>
          </>
        }
      >
        <div className="bc-asset-result" data-tone="warning">
          <span className="bc-asset-result-disc" aria-hidden="true">
            <Minus size={16} />
          </span>
          <strong aria-hidden="true">原件已移除</strong>
          <span>历史证据保留，原件不可再核验</span>
        </div>
        <dl className="bc-asset-facts">
          <div>
            <dt>文件</dt>
            <dd>
              <span className="bc-asset-chip" data-tone="warning">
                原件不可用
              </span>
            </dd>
          </div>
          <div>
            <dt>证据</dt>
            <dd>
              <span className="bc-asset-chip" data-tone="keep">
                历史准入保留
              </span>
            </dd>
          </div>
        </dl>
      </AssetDeletionLayout>
    );
  }

  if (view?.phase === "erased" && receipt) {
    const at = receipt.erased_at ? new Date(receipt.erased_at) : null;
    return (
      <AssetDeletionLayout
        {...common}
        title="清理已完成"
        phase="erased"
        footer={
          <>
            <span />
            <button
              type="button"
              className="bc-person-footer-action"
              onClick={() => onErased(upload.id)}
            >
              {backLabel}
            </button>
          </>
        }
      >
        <div className="bc-asset-result" data-tone="keep">
          <span className="bc-asset-result-disc" aria-hidden="true">
            <Check size={16} />
          </span>
          <strong aria-hidden="true">清理已完成</strong>
          <span>全部范围已核验</span>
        </div>
        <AssetList
          rows={families.map((family) => ({
            label: family.label,
            tone: "keep" as const,
            chip: "已核验",
          }))}
        />
        <p className="bc-asset-receipt">
          回执 {receipt.request_id.slice(0, 8).toUpperCase()}
          {at && !Number.isNaN(at.getTime())
            ? " · " +
              at.toLocaleString("zh-CN", {
                month: "long",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
              })
            : null}
        </p>
      </AssetDeletionLayout>
    );
  }

  const inProgress = view?.phase === "in_progress";
  const needsReview = view?.phase === "review";
  const spinner = working ? (
    <LoaderCircle size={15} className="animate-spin" aria-hidden="true" />
  ) : null;
  return (
    <AssetDeletionLayout
      {...common}
      title={view?.title ?? "正在清理"}
      phase={view?.phase ?? "in_progress"}
      footer={
        <>
          <button
            type="button"
            className="bc-person-footer-link"
            onClick={onBack}
          >
            返回
          </button>
          <button
            type="button"
            className="bc-person-footer-action"
            disabled={working}
            onClick={() => void requestErasure()}
          >
            {spinner}
            {inProgress ? "再次检查" : "重试未完成"}
          </button>
        </>
      }
    >
      <AssetFileCard upload={upload} />
      <p
        className="bc-asset-banner"
        data-tone={needsReview ? "warning" : undefined}
        role="status"
      >
        {inProgress ? (
          <LoaderCircle size={15} className="animate-spin" aria-hidden="true" />
        ) : (
          <CircleHelp size={15} aria-hidden="true" />
        )}
        {needsReview
          ? "需要你先处理"
          : inProgress
            ? "正在逐项清理并核验"
            : "部分副本仍待清理"}
      </p>
      <AssetList
        rows={[
          { label: "来源停用", tone: "keep", chip: "已生效" },
          ...families.map((family) =>
            family.verified
              ? { label: family.label, tone: "keep" as const, chip: "已核验" }
              : {
                  label: family.label,
                  tone: "pending" as const,
                  chip: inProgress ? "处理中" : "未完成",
                  note: family.reason ?? undefined,
                },
          ),
        ]}
      />
      {view && view.reviews.length > 0 ? (
        <ul className="bc-asset-reviews" aria-label="需要你处理的内容">
          {view.reviews.map((label, index) => (
            <li key={label + ":" + index}>{label}</li>
          ))}
        </ul>
      ) : null}
    </AssetDeletionLayout>
  );
}
