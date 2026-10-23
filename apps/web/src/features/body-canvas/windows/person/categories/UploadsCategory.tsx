import { useEffect, useRef, useState } from "react";
import {
  FileText,
  Image as ImageIcon,
  LoaderCircle,
  RotateCcw,
  X,
} from "lucide-react";
import { useUploadStore } from "@/stores/uploadStore";
import {
  formatFileSize,
  selectUploadsByCategory,
  type ProfileUploadLike,
} from "../../../model/profileCategories";
import { PersonAssetDeletion } from "./asset-deletion/PersonAssetDeletion";
import { PersonMediaGrid } from "./PersonMediaGrid";
import { PersonEmptyState } from "./PersonEmptyState";
import { ReportCard } from "./ReportCard";
import { IconTip } from "@/components/ui/tooltip";

export function UploadsCategory({
  uploads,
  category,
  mobile = false,
  addOpen = false,
  onAddOpenChange,
}: {
  uploads: readonly ProfileUploadLike[];
  category: "media" | "reports";
  mobile?: boolean;
  /** The window header "+" opens the file picker. */
  addOpen?: boolean;
  onAddOpenChange?: (open: boolean) => void;
}) {
  const items = selectUploadsByCategory(uploads, category);
  const uploadFile = useUploadStore((state) => state.uploadFile);
  const forgetUpload = useUploadStore((state) => state.forgetUpload);
  // Penpot Assets scenes replace the list while one file is being deleted.
  const [deleting, setDeleting] = useState<ProfileUploadLike | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [failed, setFailed] = useState<{
    name: string;
    size: number;
    at: Date;
  } | null>(null);
  // Penpot "Media · Upload failed" keeps the file name and offers a retry.
  const lastFile = useRef<File | null>(null);
  const send = async (file: File) => {
    lastFile.current = file;
    setUploading(true);
    setFailed(null);
    try {
      await uploadFile(
        file,
        category === "reports" ? "report" : "consultation_photo",
      );
      lastFile.current = null;
    } catch {
      setFailed({ name: file.name, size: file.size, at: new Date() });
    } finally {
      setUploading(false);
    }
  };
  const picker = (
    <input
      ref={inputRef}
      type="file"
      className="sr-only"
      tabIndex={-1}
      aria-hidden="true"
      accept={category === "reports" ? "image/*,application/pdf" : "image/*"}
      onChange={(event) => {
        const file = event.target.files?.[0];
        event.target.value = "";
        if (file) void send(file);
      }}
    />
  );
  // The header "+" is a request, not a state: open the picker once, then
  // hand the request back (the click still carries user activation).
  useEffect(() => {
    if (!addOpen) return;
    onAddOpenChange?.(false);
    if (!uploading) inputRef.current?.click();
  }, [addOpen, onAddOpenChange, uploading]);
  const status = uploading ? (
    <p role="status" className="bc-person-upload-status">
      <LoaderCircle size={14} className="animate-spin" aria-hidden="true" />
      正在上传…
    </p>
  ) : failed ? (
    // Penpot "Media · Upload failed": a pink row above the grid with the
    // file, 上传失败, retry and dismiss.
    <div role="alert" className="bc-person-upload-failed">
      <span className="bc-person-upload-failed-thumb" aria-hidden="true" />
      <span className="bc-person-upload-failed-text">
        <strong>{failed.name}</strong>
        <span>
          {formatFileSize(failed.size)} ·{" "}
          {failed.at.toLocaleString("zh-CN", {
            month: "long",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
          })}
        </span>
      </span>
      <span className="bc-person-upload-failed-chip">上传失败</span>
      <IconTip label="重试">
        <button
          type="button"
          className="bc-icon-button"
          aria-label="重试"
          onClick={() => lastFile.current && void send(lastFile.current)}
        >
          <RotateCcw size={17} aria-hidden="true" />
        </button>
      </IconTip>
      <IconTip label="关闭">
        <button
          type="button"
          className="bc-icon-button"
          aria-label="关闭失败提示"
          onClick={() => {
            setFailed(null);
            lastFile.current = null;
          }}
        >
          <X size={17} aria-hidden="true" />
        </button>
      </IconTip>
    </div>
  ) : null;
  if (deleting) {
    return (
      <PersonAssetDeletion
        upload={deleting}
        backLabel={category === "reports" ? "返回报告" : "返回媒体"}
        onBack={() => setDeleting(null)}
        onErased={(id) => {
          forgetUpload(id);
          setDeleting(null);
        }}
      />
    );
  }
  if (items.length === 0) {
    return (
      <>
        {picker}
        <PersonEmptyState
          icon={
            category === "reports" ? (
              <FileText size={24} />
            ) : (
              <ImageIcon size={24} />
            )
          }
          title={category === "reports" ? "还没有报告" : "还没有照片或视频"}
          message={
            category === "reports"
              ? "体检、影像或化验报告"
              : "动作视频或身体照片"
          }
        />
        {status}
      </>
    );
  }
  return (
    <>
      {picker}
      {status}
      {category === "media" ? (
        <PersonMediaGrid uploads={items} initialLimit={mobile ? 6 : 8} />
      ) : (
        <ul className="bc-person-reports">
          {items.map((upload) => (
            <ReportCard
              key={upload.id}
              upload={upload}
              onDelete={() => setDeleting(upload)}
            />
          ))}
        </ul>
      )}
    </>
  );
}
