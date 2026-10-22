import { Check, LoaderCircle, Minus, ShieldCheck, X } from "lucide-react";
import type { ProfileUploadLike } from "../../../../model/profileCategories";
import {
  AssetDeletionLayout,
  AssetFileCard,
  AssetList,
  ScopeOption,
} from "./AssetDeletionPrimitives";
import type { AssetDeletionFlow } from "./useAssetDeletionFlow";

/** Scope selection and both explicit confirmation steps. No backend call occurs
 * until the final confirmation button is activated.
 */
export function AssetDeletionChoiceScenes({
  upload,
  flow,
  onBack,
}: {
  upload: ProfileUploadLike;
  flow: AssetDeletionFlow;
  onBack: () => void;
}) {
  const {
    scene,
    setScene,
    scope,
    setScope,
    working,
    error,
    sourceGone,
    confirmSourceRemoval,
    requestErasure,
  } = flow;
  const common = { scene, working, error, onBack };
  const spinner = working ? (
    <LoaderCircle size={15} className="animate-spin" aria-hidden="true" />
  ) : null;

  if (scene === "scope") {
    return (
      <AssetDeletionLayout
        {...common}
        title="选择删除范围"
        footer={
          <>
            <button
              type="button"
              className="bc-person-footer-link"
              onClick={() => {
                setScope(scope === "source" ? "erase" : "source");
                setScene(
                  scope === "source" ? "confirm-erase" : "confirm-source",
                );
              }}
            >
              {scope === "source" ? "完整清理" : "仅移除原件"}
            </button>
            <button
              type="button"
              className="bc-person-footer-action"
              onClick={() =>
                setScene(
                  scope === "source" ? "confirm-source" : "confirm-erase",
                )
              }
            >
              {scope === "source" ? "仅移除原件" : "完整清理"}
            </button>
          </>
        }
      >
        <AssetFileCard upload={upload} />
        <div className="bc-asset-label-row">
          <p className="bc-asset-label" id="bc-asset-scope-label">
            删除范围
          </p>
          <button
            type="button"
            className="bc-asset-cancel"
            onClick={onBack}
            title="不删除，返回列表（Esc）"
          >
            取消
          </button>
        </div>
        <div
          role="radiogroup"
          aria-labelledby="bc-asset-scope-label"
          className="bc-asset-scopes"
        >
          <ScopeOption
            checked={scope === "source"}
            onSelect={() => setScope("source")}
            icon={<Minus size={14} />}
            title="仅移除原件"
            tags={[{ label: "原件" }, { label: "证据保留", tone: "keep" }]}
          />
          <ScopeOption
            checked={scope === "erase"}
            onSelect={() => setScope("erase")}
            icon={<X size={14} />}
            tone="danger"
            title="文件及相关健康数据"
            tags={[{ label: "原件" }, { label: "派生" }, { label: "关联证据" }]}
          />
        </div>
        <p className="bc-asset-keep-note">
          <span className="bc-asset-mark" data-tone="keep" aria-hidden="true">
            <Check size={14} strokeWidth={2.6} />
          </span>
          其他独立来源的事实不受影响
        </p>
      </AssetDeletionLayout>
    );
  }

  if (scene === "confirm-source") {
    return (
      <AssetDeletionLayout
        {...common}
        title="确认移除原件"
        footer={
          <>
            <button
              type="button"
              className="bc-person-footer-link"
              disabled={working}
              onClick={() => setScene("scope")}
            >
              取消
            </button>
            <button
              type="button"
              className="bc-person-footer-action bc-asset-confirm"
              disabled={working}
              onClick={() => void confirmSourceRemoval()}
            >
              {spinner ?? <Check size={15} aria-hidden="true" />}
              {working ? "正在移除" : "确认移除原件"}
            </button>
          </>
        }
      >
        <AssetFileCard upload={upload} />
        <AssetList
          label="将移除"
          rows={[{ label: "原件", tone: "remove", chip: "仅原始文件" }]}
        />
        <AssetList
          label="保留"
          rows={[
            { label: "整理出的记录与证据", tone: "keep", chip: "保留" },
            { label: "已作出的决定", tone: "keep", chip: "保留" },
          ]}
        />
        <p className="bc-asset-banner">
          <ShieldCheck size={15} aria-hidden="true" />
          移除后原件不可再查看或核验
        </p>
      </AssetDeletionLayout>
    );
  }

  if (scene === "confirm-erase") {
    return (
      <AssetDeletionLayout
        {...common}
        title="确认完整清理"
        footer={
          <>
            <button
              type="button"
              className="bc-person-footer-link"
              disabled={working}
              onClick={() => setScene(sourceGone ? "source-removed" : "scope")}
            >
              取消
            </button>
            <button
              type="button"
              className="bc-person-footer-action bc-asset-confirm"
              disabled={working}
              onClick={() => void requestErasure()}
            >
              {spinner ?? <Check size={15} aria-hidden="true" />}
              {working ? "正在清理" : "确认清理"}
            </button>
          </>
        }
      >
        <AssetFileCard upload={upload} />
        <AssetList
          label="将清理"
          rows={[
            {
              label: "原件",
              tone: "remove",
              chip: sourceGone ? undefined : "1 份",
            },
            { label: "派生内容", tone: "remove" },
            { label: "检查点副本", tone: "remove" },
          ]}
        />
        <AssetList
          label="保留"
          rows={[
            { label: "独立陈述与事实", tone: "keep", chip: "保留" },
            { label: "最小操作回执", tone: "keep", chip: "保留" },
          ]}
        />
        <p className="bc-asset-banner">
          <ShieldCheck size={15} aria-hidden="true" />
          确认后立即停用该来源
        </p>
      </AssetDeletionLayout>
    );
  }

  return null;
}
