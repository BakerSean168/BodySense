import { useEffect, useRef, type ReactNode } from "react";
import { Check, CircleHelp, FileText, X } from "lucide-react";
import type { ProfileUploadLike } from "../../../../model/profileCategories";
import { formatMediaDate } from "../../../../model/personMedia";
import type { AssetDeletionScene } from "./useAssetDeletionFlow";

const FILE_KIND: Record<string, string> = {
  report: "体检报告",
  consultation_photo: "问诊照片",
  photo_front: "正面照片",
  photo_side: "侧面照片",
  photo_back: "背面照片",
};

export function AssetFileCard({ upload }: { upload: ProfileUploadLike }) {
  return (
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
}

export function AssetDeletionLayout({
  scene,
  error,
  title,
  visibleTitle = false,
  children,
  footer,
}: {
  scene: AssetDeletionScene;
  phase?: string;
  working: boolean;
  error: string | null;
  onBack: () => void;
  title: string;
  visibleTitle?: boolean;
  children: ReactNode;
  footer: ReactNode;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus();
  }, [scene]);

  // The surrounding scene root belongs to PersonAssetDeletion so it survives
  // switching between choice/receipt components and retains keyboard focus.
  return (
    <>
      <div className="bc-person-detail-body">
        <h3
          ref={heading}
          tabIndex={-1}
          className={visibleTitle ? "bc-asset-scene-title" : "sr-only"}
        >
          {title}
        </h3>
        {children}
        {error ? (
          <p role="alert" className="bc-error">
            {error}
          </p>
        ) : null}
      </div>
      <footer className="bc-person-footer">{footer}</footer>
    </>
  );
}

export function ScopeOption({
  checked,
  onSelect,
  icon,
  tone,
  title,
  tags,
}: {
  checked: boolean;
  onSelect: () => void;
  icon: ReactNode;
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

export function AssetList({
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
