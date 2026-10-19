import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  CircleHelp,
  FileText,
  LoaderCircle,
  Minus,
  ShieldCheck,
  X,
} from "lucide-react";
import {
  assetErasureService,
  type AssetErasureReceipt,
} from "@/features/profile/services/assetErasureService";
import {
  groupAssetErasure,
  projectAssetErasure,
} from "@/features/profile/components/uploads/assetErasureViewModel";
import { errorMessage } from "@/lib/api-client";
import { useUploadStore } from "@/stores/uploadStore";
import type { ProfileUploadLike } from "../model/profileCategories";
import { formatMediaDate } from "../model/personMedia";

type Scene =
  "scope" | "confirm-source" | "confirm-erase" | "source-removed" | "receipt";

const FILE_KIND: Record<string, string> = {
  report: "体检报告",
  consultation_photo: "问诊照片",
  photo_front: "正面照片",
  photo_side: "侧面照片",
  photo_back: "背面照片",
};

/**
 * Penpot "Assets / Delete scope → Erasure confirm → Erasure retry / completed,
 * Original unavailable" inside the Person window. Two real backend commands:
 *
 * - source removal (DELETE /api/v1/uploads/{id}, PC-D7): only the private
 *   original goes; adopted evidence stays, marked source-unavailable;
 * - AS-002 erasure (POST /api/v1/assets/{id}/erasure): every controlled copy,
 *   reported done only when the receipt verifies every target.
 *
 * Both take a confirm step first. The receipt is the only source of progress:
 * nothing is shown as cleared before the server verified it, and anything a
 * Diagnosis or plan rests on comes back as a review, never changed here.
 */
export function PersonAssetDeletion({
  upload,
  backLabel,
  onBack,
  onErased,
}: {
  upload: ProfileUploadLike;
  backLabel: string;
  onBack: () => void;
  /** Called after the user leaves a fully verified erasure. */
  onErased: (assetId: string) => void;
}) {
  const removeSource = useUploadStore((state) => state.deleteUpload);
  const live = useUploadStore((state) =>
    state.uploads.find((item) => item.id === upload.id),
  );
  const sourceGone =
    (live?.source_availability ?? upload.source_availability) ===
    "source_unavailable";
  const [scene, setScene] = useState<Scene>(
    sourceGone ? "source-removed" : "scope",
  );
  const [scope, setScope] = useState<"source" | "erase">("source");
  const [receipt, setReceipt] = useState<AssetErasureReceipt | null>(null);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // One erasure per asset: retries reuse the key and return the same receipt.
  const key = useRef(crypto.randomUUID());
  const mounted = useRef(true);
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // Each scene change moves focus to its heading for keyboard and AT users.
  useEffect(() => {
    heading.current?.focus();
  }, [scene]);

  const view = useMemo(
    () => (receipt ? projectAssetErasure(receipt) : null),
    [receipt],
  );
  const families = useMemo(
    () => (receipt ? groupAssetErasure(receipt) : []),
    [receipt],
  );

  async function confirmSourceRemoval() {
    setWorking(true);
    setError(null);
    try {
      await removeSource(upload.id);
      if (mounted.current) setScene("source-removed");
    } catch (cause) {
      if (mounted.current)
        setError(errorMessage(cause, "原件没有移除，请重试。"));
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  async function requestErasure() {
    setWorking(true);
    setError(null);
    try {
      const next = await assetErasureService.request(upload.id, key.current);
      if (!mounted.current) return;
      if (next.asset_id !== upload.id)
        throw new Error("清理回执与这个文件不一致。");
      setReceipt(next);
      setScene("receipt");
    } catch (cause) {
      if (mounted.current)
        setError(
          errorMessage(cause, "尚未确认清理结果。重试会继续同一次清理。"),
        );
    } finally {
      if (mounted.current) setWorking(false);
    }
  }

  const fileCard = (
    <div className="bc-asset-file">
      <span className="bc-asset-file-icon" aria-hidden="true">
        <FileText size={20} />
      </span>
      <span className="bc-asset-file-text">
        <strong>{upload.original_name}</strong>
        <span>
          {FILE_KIND[upload.file_type] ?? "文件"} ·{" "}
          {formatMediaDate(upload.created_at)}上传
        </span>
      </span>
    </div>
  );
  const errorLine = error ? (
    <p role="alert" className="bc-error">
      {error}
    </p>
  ) : null;
  const spinner = working ? (
    <LoaderCircle size={15} className="animate-spin" aria-hidden="true" />
  ) : null;

  const shell = (
    title: string,
    body: React.ReactNode,
    footer: React.ReactNode,
    options: { visibleTitle?: boolean; phase: string } = { phase: scene },
  ) => (
    <div
      className="bc-person-category bc-person-detail bc-asset-scene"
      data-testid="person-asset-deletion"
      data-scene={scene}
      data-phase={options.phase}
      onKeyDown={(event) => {
        // Escape leaves the scope choice without deleting anything.
        if (event.key === "Escape" && scene === "scope" && !working) {
          event.preventDefault();
          event.stopPropagation();
          onBack();
        }
      }}
    >
      <div className="bc-person-detail-body">
        <h3
          ref={heading}
          tabIndex={-1}
          className={options.visibleTitle ? "bc-asset-scene-title" : "sr-only"}
        >
          {title}
        </h3>
        {body}
        {errorLine}
      </div>
      <footer className="bc-person-footer">{footer}</footer>
    </div>
  );

  if (scene === "scope") {
    return shell(
      "选择删除范围",
      <>
        {fileCard}
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
      </>,
      // Penpot footer: the other scope as a quiet link on the left
      // ("完整清理" while 仅移除原件 is chosen), the chosen scope as the
      // primary action. Both only open their confirm step.
      <>
        <button
          type="button"
          className="bc-person-footer-link"
          onClick={() => {
            setScope(scope === "source" ? "erase" : "source");
            setScene(scope === "source" ? "confirm-erase" : "confirm-source");
          }}
        >
          {scope === "source" ? "完整清理" : "仅移除原件"}
        </button>
        <button
          type="button"
          className="bc-person-footer-action"
          onClick={() =>
            setScene(scope === "source" ? "confirm-source" : "confirm-erase")
          }
        >
          {scope === "source" ? "仅移除原件" : "完整清理"}
        </button>
      </>,
    );
  }

  if (scene === "confirm-source") {
    return shell(
      "确认移除原件",
      <>
        {fileCard}
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
      </>,
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
      </>,
    );
  }

  if (scene === "confirm-erase") {
    return shell(
      "确认完整清理",
      <>
        {fileCard}
        <AssetList
          label="将清理"
          rows={[
            // One upload is one original file. Derived and checkpoint copies
            // have no pre-erasure count in the API, so they show none.
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
      </>,
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
      </>,
    );
  }

  if (scene === "source-removed") {
    return shell(
      "原件已移除",
      <>
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
      </>,
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
      </>,
      { visibleTitle: false, phase: "source_removed" },
    );
  }

  // Receipt scenes: the server's verification is the only progress shown.
  if (view?.phase === "erased" && receipt) {
    const at = receipt.erased_at ? new Date(receipt.erased_at) : null;
    return shell(
      "清理已完成",
      <>
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
            ? ` · ${at.toLocaleString("zh-CN", {
                month: "long",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
              })}`
            : null}
        </p>
      </>,
      <>
        <span />
        <button
          type="button"
          className="bc-person-footer-action"
          onClick={() => onErased(upload.id)}
        >
          {backLabel}
        </button>
      </>,
      { visibleTitle: false, phase: "erased" },
    );
  }

  const inProgress = view?.phase === "in_progress";
  const needsReview = view?.phase === "review";
  return shell(
    view?.title ?? "正在清理",
    <>
      {fileCard}
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
            <li key={`${label}:${index}`}>{label}</li>
          ))}
        </ul>
      ) : null}
    </>,
    <>
      <button type="button" className="bc-person-footer-link" onClick={onBack}>
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
    </>,
    { visibleTitle: false, phase: view?.phase ?? "in_progress" },
  );
}

function ScopeOption({
  checked,
  onSelect,
  icon,
  tone,
  title,
  tags,
}: {
  checked: boolean;
  onSelect: () => void;
  icon: React.ReactNode;
  tone?: "danger";
  title: string;
  tags: Array<{ label: string; tone?: "keep" }>;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={checked}
      className="bc-asset-scope"
      data-tone={tone}
      onClick={onSelect}
      onKeyDown={(event) => {
        // Radio group arrows move between the two options.
        if (
          ["ArrowDown", "ArrowUp", "ArrowLeft", "ArrowRight"].includes(
            event.key,
          )
        ) {
          event.preventDefault();
          const group = event.currentTarget.parentElement;
          const options = Array.from(
            group?.querySelectorAll<HTMLButtonElement>('[role="radio"]') ?? [],
          );
          const step =
            event.key === "ArrowUp" || event.key === "ArrowLeft" ? -1 : 1;
          const next =
            options[
              (options.indexOf(event.currentTarget) + step + options.length) %
                options.length
            ];
          next?.focus();
          next?.click();
        }
      }}
      tabIndex={checked ? 0 : -1}
    >
      <span className="bc-asset-scope-title">
        <span
          className="bc-asset-mark"
          data-tone={tone ?? "neutral"}
          aria-hidden="true"
        >
          {icon}
        </span>
        {title}
      </span>
      <span className="bc-asset-scope-tags">
        {tags.map((tag) => (
          <span key={tag.label} className="bc-asset-chip" data-tone={tag.tone}>
            {tag.label}
          </span>
        ))}
      </span>
    </button>
  );
}

function AssetList({
  label,
  rows,
}: {
  label?: string;
  rows: Array<{
    label: string;
    tone: "remove" | "keep" | "pending";
    chip?: string;
    note?: string;
  }>;
}) {
  return (
    <div className="bc-asset-list">
      {label ? <p className="bc-asset-label">{label}</p> : null}
      <ul aria-label={label}>
        {rows.map((row) => (
          <li key={row.label} data-tone={row.tone}>
            <span
              className="bc-asset-mark"
              data-tone={row.tone}
              aria-hidden="true"
            >
              {row.tone === "keep" ? (
                <Check size={14} strokeWidth={2.6} />
              ) : row.tone === "pending" ? (
                <CircleHelp size={14} strokeWidth={2.4} />
              ) : (
                <X size={14} strokeWidth={2.6} />
              )}
            </span>
            <span className="bc-asset-list-text">
              {row.label}
              {row.note ? <small>{row.note}</small> : null}
            </span>
            {row.chip ? (
              <span className="bc-asset-chip" data-tone={row.tone}>
                {row.chip}
              </span>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
