import { useRef, useState } from "react";
import {
  FileText,
  Image as ImageIcon,
  LoaderCircle,
  RotateCcw,
  Upload,
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

export function UploadsCategory({
  uploads,
  category,
  mobile = false,
}: {
  uploads: readonly ProfileUploadLike[];
  category: "media" | "reports";
  mobile?: boolean;
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
  const label = category === "reports" ? "上传报告" : "上传照片";
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
  const open = () => inputRef.current?.click();
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
      <button
        type="button"
        className="bc-icon-button"
        aria-label="重试"
        title="重试"
        onClick={() => lastFile.current && void send(lastFile.current)}
      >
        <RotateCcw size={17} aria-hidden="true" />
      </button>
      <button
        type="button"
        className="bc-icon-button"
        aria-label="关闭失败提示"
        title="关闭"
        onClick={() => {
          setFailed(null);
          lastFile.current = null;
        }}
      >
        <X size={17} aria-hidden="true" />
      </button>
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
              ? "上传体检、影像或化验报告，会关联到相关记录"
              : "上传动作视频或身体照片，方便之后对照"
          }
          actionLabel={label}
          onAction={open}
          disabled={uploading}
        />
        {status}
      </>
    );
  }
  return (
    <>
      {picker}
      <div className="bc-person-uploads-head">
        <span>
          {category === "media" ? `${items.length} 个` : `${items.length} 份`}
        </span>
        <button
          type="button"
          className="bc-person-upload"
          aria-label={label}
          title={label}
          onClick={open}
          disabled={uploading}
        >
          <Upload size={17} aria-hidden="true" />
          <span className="bc-person-upload-text" aria-hidden="true">
            上传
          </span>
        </button>
      </div>
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
